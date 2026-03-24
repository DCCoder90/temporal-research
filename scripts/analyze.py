#!/usr/bin/env python3
"""
analyze.py — Analyze Temporal .pcap captures.

Usage (via wrapper):
    ./scripts/analyze.sh captures/temporal_00001.pcap

Direct usage:
    python3 scripts/analyze.py captures/temporal_00001.pcap

Outputs to <project-root>/captures/reports/:
    <name>_flow.html   — Mermaid data-flow diagram + gRPC sequence diagram
    <name>_stats.md    — Protocol breakdown, connection matrix, Temporal insights

Requirements:
    - tshark (brew install wireshark)
    - Python 3.9+
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path

# ── Configuration ──────────────────────────────────────────────────────────────

SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent
REPORTS_DIR = PROJECT_ROOT / "captures" / "reports"

# Static IP → container name map (matches docker-compose.yml)
IP_TO_NAME: dict[str, str] = {
    "172.20.0.10": "postgresql",
    "172.20.0.20": "temporal",
    "172.20.0.30": "temporal-ui",
    "172.20.0.40": "hello-world-worker",
    "172.20.0.41": "hello-world-starter",
    "172.20.0.42": "scheduled-worker",
    "172.20.0.43": "signals-worker",
    "172.20.0.44": "child-workflows-worker",
    "172.20.0.45": "retries-worker",
    "172.20.0.46": "saga-worker",
    "172.20.0.50": "wireshark",
}

# Well-known ports → human labels
PORT_LABELS: dict[str, str] = {
    "7233": "Temporal gRPC",
    "5432": "PostgreSQL",
    "8080": "HTTP (Temporal UI)",
}

# Maximum compressed rows shown in sequence diagram before truncating
MAX_SEQ_ENTRIES = 150

# User-friendly protocol name → set of ports that define it in this project.
# Anything not listed here is matched case-insensitively against the
# _ws.col.Protocol field that tshark fills in (e.g. "ARP", "ICMPv6").
_PROTO_PORTS: dict[str, set[str]] = {
    "pgsql":      {"5432"},
    "postgresql": {"5432"},
    "grpc":       {"7233"},
    "http2":      {"7233"},
    "http":       {"8080"},
}

# Protocols considered "gRPC" for the purpose of the sequence diagram.
_GRPC_PROTO_NAMES = {"grpc", "http2"}


# ── Helpers ────────────────────────────────────────────────────────────────────

def resolve(ip: str) -> str:
    """Map a container IP to its name, or return the raw IP if unknown."""
    return IP_TO_NAME.get(ip, ip)


def mermaid_id(name: str) -> str:
    """Sanitize a string for use as a Mermaid node/participant identifier."""
    return re.sub(r"[^a-zA-Z0-9]", "_", name)


def fmt_bytes(n: int | float) -> str:
    for unit in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} TB"


def fmt_num(n: int) -> str:
    return f"{n:,}"


def matches_protocol(packet: dict, proto: str) -> bool:
    """Return True if packet belongs to the named protocol.

    Named aliases (grpc, pgsql, http2, http, postgresql) are matched by port.
    Anything else is matched case-insensitively against tshark's Protocol column.
    """
    key = proto.lower()
    if key in _PROTO_PORTS:
        ports = _PROTO_PORTS[key]
        return packet["dport"] in ports or packet["sport"] in ports
    return packet["proto"].lower() == key


def apply_filter(
    packets: list[dict],
    grpc_calls: list[tuple],
    only: list[str] | None,
    exclude: list[str] | None,
) -> tuple[list[dict], list[tuple]]:
    """Filter packets and grpc_calls according to --only / --exclude flags."""
    if not only and not exclude:
        return packets, grpc_calls

    if only:
        protos = [p.lower() for p in only]
        packets = [p for p in packets if any(matches_protocol(p, pr) for pr in protos)]
        grpc_included = bool(set(protos) & _GRPC_PROTO_NAMES)
        grpc_calls = grpc_calls if grpc_included else []
    else:  # exclude
        protos = [p.lower() for p in exclude]
        packets = [p for p in packets if not any(matches_protocol(p, pr) for pr in protos)]
        grpc_excluded = bool(set(protos) & _GRPC_PROTO_NAMES)
        grpc_calls = [] if grpc_excluded else grpc_calls

    return packets, grpc_calls


# ── tshark extraction ──────────────────────────────────────────────────────────

def _run_tshark(
    pcap: Path,
    fields: list[str],
    extra_args: list[str] | None = None,
    filter_expr: str | None = None,
) -> list[list[str]]:
    """Run tshark and return tab-separated field rows (one list per packet)."""
    cmd = ["tshark", "-r", str(pcap), "-T", "fields"]
    if extra_args:
        cmd.extend(extra_args)
    for f in fields:
        cmd.extend(["-e", f])
    cmd.extend(["-E", "separator=\t", "-E", "header=n"])
    if filter_expr:
        cmd.extend(["-Y", filter_expr])

    result = subprocess.run(cmd, capture_output=True, text=True)
    rows: list[list[str]] = []
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        # Pad short rows so callers can always unpack len(fields) items.
        while len(parts) < len(fields):
            parts.append("")
        rows.append(parts[: len(fields)])
    return rows


def extract_packets(pcap: Path) -> tuple[list[dict], float]:
    """Return (packets, capture_duration_seconds) for all IP packets."""
    fields = [
        "frame.time_epoch",
        "ip.src",
        "ip.dst",
        "tcp.srcport",
        "tcp.dstport",
        "frame.len",
        "_ws.col.Protocol",
    ]
    packets: list[dict] = []
    for row in _run_tshark(pcap, fields):
        time_s, src, dst, sport, dport, length, proto = row
        if not src or not dst:
            continue
        try:
            packets.append(
                {
                    "t":     float(time_s),
                    "src":   src,
                    "dst":   dst,
                    "sport": sport,
                    "dport": dport,
                    "len":   int(length) if length else 0,
                    "proto": proto,
                }
            )
        except ValueError:
            continue

    if not packets:
        return [], 0.0
    times = [p["t"] for p in packets]
    return packets, max(times) - min(times)


def extract_grpc_calls(pcap: Path) -> list[tuple[float, str, str, str]]:
    """Return (time, src_name, dst_name, method_name) for every gRPC request.

    Temporal uses unencrypted HTTP/2 (h2c) on port 7233. We tell tshark to
    treat that port as HTTP/2, then extract the :path pseudo-header which
    carries the full gRPC method path, e.g.:
        /temporal.api.workflowservice.v1.WorkflowService/PollWorkflowTaskQueue
    """
    fields = ["frame.time_epoch", "ip.src", "ip.dst", "http2.headers.path"]
    rows = _run_tshark(
        pcap,
        fields,
        extra_args=["-d", "tcp.port==7233,http2"],
        filter_expr="http2.headers.path",
    )

    calls: list[tuple[float, str, str, str]] = []
    for time_s, src, dst, path in rows:
        if not path:
            continue
        method = path.rsplit("/", 1)[-1]  # last segment = method name
        try:
            calls.append((float(time_s), resolve(src), resolve(dst), method))
        except ValueError:
            continue
    return calls


# ── Diagram builders ───────────────────────────────────────────────────────────

def build_flow_diagram(packets: list[dict]) -> str:
    """Mermaid flowchart LR showing directed traffic between containers."""
    # (src_name, dst_name, port_label) -> [packet_count, byte_count]
    edges: dict[tuple[str, str, str], list[int]] = defaultdict(lambda: [0, 0])
    nodes: set[str] = set()

    for p in packets:
        src = resolve(p["src"])
        dst = resolve(p["dst"])
        label = PORT_LABELS.get(p["dport"], f"TCP:{p['dport']}" if p["dport"] else "TCP")
        edges[(src, dst, label)][0] += 1
        edges[(src, dst, label)][1] += p["len"]
        nodes.add(src)
        nodes.add(dst)

    lines = ["flowchart LR"]

    for node in sorted(nodes):
        nid = mermaid_id(node)
        # Use a cylinder shape for the database, stadium for everything else.
        if node == "postgresql":
            lines.append(f'    {nid}[("{node}")]')
        else:
            lines.append(f'    {nid}(["{node}"])')

    lines.append("")

    for (src, dst, label), (pkts, nbytes) in sorted(
        edges.items(), key=lambda x: -x[1][0]
    ):
        sid, did = mermaid_id(src), mermaid_id(dst)
        edge_label = f"{label} — {fmt_num(pkts)} pkts / {fmt_bytes(nbytes)}"
        lines.append(f'    {sid} -->|"{edge_label}"| {did}')

    return "\n".join(lines)


def build_sequence_diagram(calls: list[tuple[float, str, str, str]]) -> str:
    """Mermaid sequenceDiagram with consecutive identical calls compressed."""
    if not calls:
        return (
            "sequenceDiagram\n"
            "    Note over temporal: No gRPC calls decoded in this capture\n"
            "    Note over temporal: Run tshark -r file.pcap -d tcp.port==7233,http2 -Y http2.headers.path"
        )

    # Collect participants in first-appearance order.
    seen: dict[str, None] = {}
    for _, src, dst, _ in calls:
        seen.setdefault(src, None)
        seen.setdefault(dst, None)

    lines = ["sequenceDiagram"]
    for p in seen:
        lines.append(f"    participant {mermaid_id(p)} as {p}")
    lines.append("")

    # Compress consecutive identical (src, dst, method) runs into a single row.
    compressed: list[list] = []  # [src, dst, method, count]
    for _, src, dst, method in calls:
        if compressed and compressed[-1][:3] == [src, dst, method]:
            compressed[-1][3] += 1
        else:
            compressed.append([src, dst, method, 1])

    total = len(compressed)
    for src, dst, method, count in compressed[:MAX_SEQ_ENTRIES]:
        sid, did = mermaid_id(src), mermaid_id(dst)
        label = f"{method} (x{fmt_num(count)})" if count > 1 else method
        lines.append(f"    {sid}->>{did}: {label}")

    if total > MAX_SEQ_ENTRIES:
        omitted = total - MAX_SEQ_ENTRIES
        lines.append(
            f"    Note over temporal: ... {fmt_num(omitted)} more call type(s) not shown (cap: {MAX_SEQ_ENTRIES})"
        )

    return "\n".join(lines)


# ── HTML output ────────────────────────────────────────────────────────────────

_HTML_TEMPLATE = """\
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Temporal Analysis — {filename}</title>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
  <script>
    mermaid.initialize({{
      startOnLoad: true,
      theme: "default",
      sequence: {{ showSequenceNumbers: true, mirrorActors: false, useMaxWidth: false }},
      flowchart: {{ curve: "basis", useMaxWidth: false }}
    }});
  </script>
  <style>
    * {{ box-sizing: border-box; }}
    body {{
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, sans-serif;
      max-width: 1600px; margin: 0 auto; padding: 2rem 3rem;
      background: #f4f6fb; color: #1a1a2e;
    }}
    h1 {{ border-bottom: 3px solid #6c63ff; padding-bottom: .5rem; margin-bottom: .75rem; }}
    h2 {{ color: #4a4a8a; margin-top: 2.5rem; margin-bottom: .25rem; }}
    .meta {{
      display: flex; flex-wrap: wrap; gap: .75rem;
      color: #555; font-size: .9rem; margin-bottom: 2rem;
    }}
    .meta span {{
      background: white; padding: .3rem .8rem;
      border-radius: 4px; box-shadow: 0 1px 3px rgba(0,0,0,.1);
    }}
    .hint {{ color: #888; font-size: .85rem; margin: .25rem 0 .75rem; }}
    .card {{
      background: white; border-radius: 10px; padding: 1.5rem 2rem;
      box-shadow: 0 2px 10px rgba(0,0,0,.08); margin: .5rem 0 1.5rem;
      overflow-x: auto;
    }}
    footer {{
      margin-top: 3rem; color: #bbb; font-size: .8rem; text-align: center;
      border-top: 1px solid #e0e0e0; padding-top: 1rem;
    }}
  </style>
</head>
<body>

  <h1>Temporal Traffic Analysis</h1>
  <div class="meta">
    <span><strong>File:</strong> {filename}</span>
    <span><strong>Generated:</strong> {generated}</span>
    <span><strong>Duration:</strong> {duration}</span>
    <span><strong>Packets:</strong> {total_packets}</span>
    <span><strong>Bytes:</strong> {total_bytes}</span>
    <span><strong>gRPC Calls:</strong> {grpc_calls}</span>
    {filter_badge}
  </div>

  <h2>Data Flow Diagram</h2>
  <p class="hint">
    Each arrow shows protocol, total packet count, and bytes transferred in that direction.
  </p>
  <div class="card">
    <div class="mermaid">
{flow_diagram}
    </div>
  </div>

  <h2>gRPC Sequence Diagram</h2>
  <p class="hint">
    Consecutive identical calls from the same source are compressed (xN).
    Sequence numbers show call order. Temporal uses HTTP/2 on port 7233.
  </p>
  <div class="card">
    <div class="mermaid">
{sequence_diagram}
    </div>
  </div>

  <footer>Generated by scripts/analyze.py &mdash; temporalcoms</footer>
</body>
</html>
"""


def generate_html(
    pcap_name: str,
    flow: str,
    seq: str,
    duration: float,
    total_pkts: int,
    total_bytes: int,
    grpc_count: int,
    filter_desc: str | None = None,
) -> str:
    if filter_desc:
        badge = f'<span style="background:#fff3cd;color:#856404;"><strong>Filter:</strong> {filter_desc}</span>'
    else:
        badge = ""
    return _HTML_TEMPLATE.format(
        filename=pcap_name,
        generated=datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        duration=f"{duration:.1f}s",
        total_packets=fmt_num(total_pkts),
        total_bytes=fmt_bytes(total_bytes),
        grpc_calls=fmt_num(grpc_count),
        flow_diagram=flow,
        sequence_diagram=seq,
        filter_badge=badge,
    )


# ── Stats markdown ─────────────────────────────────────────────────────────────

def generate_stats(
    pcap_name: str,
    packets: list[dict],
    grpc_calls: list[tuple],
    duration: float,
    filter_desc: str | None = None,
) -> str:
    total_pkts = len(packets)
    total_bytes = sum(p["len"] for p in packets)

    out: list[str] = []

    def heading(level: int, text: str) -> None:
        out.append("")
        out.append("#" * level + " " + text)
        out.append("")

    def table(headers: list[str], rows: list[list]) -> None:
        out.append("| " + " | ".join(headers) + " |")
        out.append("|" + "|".join(["---"] * len(headers)) + "|")
        for row in rows:
            out.append("| " + " | ".join(str(c) for c in row) + " |")
        out.append("")

    # ── Header ────────────────────────────────────────────────────────────────
    out.append("# Temporal Traffic Analysis")
    out.append("")
    table(
        ["", ""],
        [
            ["**File**",               f"`{pcap_name}`"],
            ["**Generated**",          datetime.now().strftime("%Y-%m-%d %H:%M:%S")],
            ["**Capture Duration**",   f"{duration:.1f} seconds"],
            ["**Filter**",             f"`{filter_desc}`" if filter_desc else "_none (all protocols)_"],
            ["**Total Packets**",      fmt_num(total_pkts)],
            ["**Total Bytes**",        fmt_bytes(total_bytes)],
            ["**gRPC Calls Decoded**", fmt_num(len(grpc_calls))],
        ],
    )
    out.append("---")

    # ── Protocol breakdown ─────────────────────────────────────────────────────
    heading(2, "Protocol Breakdown")

    proto_buckets: dict[str, list[int]] = defaultdict(lambda: [0, 0])
    for p in packets:
        label = PORT_LABELS.get(p["dport"], p["proto"] or "Other")
        proto_buckets[label][0] += 1
        proto_buckets[label][1] += p["len"]

    table(
        ["Protocol", "Packets", "Bytes", "% of Traffic"],
        [
            [lbl, fmt_num(pkts), fmt_bytes(nb), f"{100 * pkts / total_pkts:.1f}%"]
            for lbl, (pkts, nb) in sorted(proto_buckets.items(), key=lambda x: -x[1][0])
        ],
    )
    out.append("---")

    # ── Connection matrix ──────────────────────────────────────────────────────
    heading(2, "Connection Matrix")
    out.append("_Top 30 directed connections by packet count._")
    out.append("")

    conn: dict[tuple[str, str, str], list[int]] = defaultdict(lambda: [0, 0])
    for p in packets:
        src  = resolve(p["src"])
        dst  = resolve(p["dst"])
        lbl  = PORT_LABELS.get(p["dport"], p["dport"] or "?")
        conn[(src, dst, lbl)][0] += 1
        conn[(src, dst, lbl)][1] += p["len"]

    table(
        ["Source", "Destination", "Protocol / Port", "Packets", "Bytes"],
        [
            [src, dst, lbl, fmt_num(pkts), fmt_bytes(nb)]
            for (src, dst, lbl), (pkts, nb)
            in sorted(conn.items(), key=lambda x: -x[1][0])[:30]
        ],
    )
    out.append("---")

    # ── gRPC method calls ──────────────────────────────────────────────────────
    heading(2, "gRPC Method Calls")

    if not grpc_calls:
        out.append(
            "> **No gRPC calls were decoded.** This can happen when tshark's HTTP/2 dissector\n"
            "> did not recognise traffic on port 7233.  Verify with:\n"
            "> ```\n"
            "> tshark -r <file> -d tcp.port==7233,http2 -Y http2.headers.path\n"
            "> ```"
        )
    else:
        out.append("_All methods called on port 7233 (Temporal Frontend), decoded from HTTP/2 HEADERS._")
        out.append("")

        method_data: dict[str, dict] = defaultdict(lambda: {"count": 0, "sources": set()})
        for _, src, dst, method in grpc_calls:
            method_data[method]["count"] += 1
            method_data[method]["sources"].add(src)

        table(
            ["Method", "Calls", "Called By"],
            [
                [f"`{m}`", fmt_num(d["count"]), ", ".join(sorted(d["sources"]))]
                for m, d in sorted(method_data.items(), key=lambda x: -x[1]["count"])
            ],
        )
        out.append("---")

        mc = {m: d["count"] for m, d in method_data.items()}

        # ── Temporal-specific insights ─────────────────────────────────────────
        heading(2, "Temporal-Specific Insights")

        heading(3, "Workflow Lifecycle")
        table(
            ["Event", "Count", "Notes"],
            [
                ["`StartWorkflowExecution`",         fmt_num(mc.get("StartWorkflowExecution", 0)),         "new workflow runs initiated"],
                ["`SignalWorkflowExecution`",         fmt_num(mc.get("SignalWorkflowExecution", 0)),         "signals delivered to running workflows"],
                ["`QueryWorkflow`",                   fmt_num(mc.get("QueryWorkflow", 0)),                   "synchronous query calls"],
                ["`RequestCancelWorkflowExecution`",  fmt_num(mc.get("RequestCancelWorkflowExecution", 0)),  "graceful cancellation requests"],
                ["`TerminateWorkflowExecution`",      fmt_num(mc.get("TerminateWorkflowExecution", 0)),      "forced termination"],
                ["`GetWorkflowExecutionHistory`",     fmt_num(mc.get("GetWorkflowExecutionHistory", 0)),     "history fetches (UI or SDK)"],
            ],
        )

        heading(3, "Task Queue Activity")
        wf_polls    = mc.get("PollWorkflowTaskQueue", 0)
        wf_done     = mc.get("RespondWorkflowTaskCompleted", 0)
        act_polls   = mc.get("PollActivityTaskQueue", 0)
        act_done    = mc.get("RespondActivityTaskCompleted", 0)
        act_failed  = mc.get("RespondActivityTaskFailed", 0)
        act_hb      = mc.get("RecordActivityTaskHeartbeat", 0)

        table(
            ["Metric", "Count", "Notes"],
            [
                ["`PollWorkflowTaskQueue`",      fmt_num(wf_polls),   "long-poll; returns when a workflow task is ready"],
                ["`RespondWorkflowTaskCompleted`",fmt_num(wf_done),    "worker finished executing a workflow task"],
                ["`PollActivityTaskQueue`",       fmt_num(act_polls),  "long-poll; returns when an activity task is ready"],
                ["`RespondActivityTaskCompleted`",fmt_num(act_done),   "activity returned successfully"],
                ["`RespondActivityTaskFailed`",   fmt_num(act_failed), "activity returned an error — Temporal will retry"],
                ["`RecordActivityTaskHeartbeat`", fmt_num(act_hb),     "long-running activity progress ping"],
            ],
        )

        total_polls = wf_polls + act_polls
        total_done  = wf_done  + act_done
        if total_polls > 0 and total_done > 0:
            heading(3, "Worker Efficiency")
            ratio = total_polls / total_done
            note  = "expected for idle long-poll workers" if ratio > 50 else "workers are executing at high throughput"
            out.append(f"- Total poll calls: **{fmt_num(total_polls)}**")
            out.append(f"- Total task completions: **{fmt_num(total_done)}**")
            out.append(f"- Poll-to-completion ratio: **{ratio:.0f}:1** — {note}")
            out.append("")

        if act_failed > 0:
            heading(3, "Activity Retry Analysis")
            out.append(f"- **{fmt_num(act_failed)}** activity failure(s) observed (RespondActivityTaskFailed).")
            out.append(f"  Temporal will schedule retries automatically per each workflow's RetryPolicy.")
            if act_done > 0:
                fail_rate = 100 * act_failed / (act_failed + act_done)
                out.append(f"- **{fmt_num(act_done)}** eventual completion(s) recorded.")
                out.append(f"- Observed failure rate: **{fail_rate:.1f}%** of activity attempts.")
            out.append("")

        heading(3, "Schedule Management")
        table(
            ["Operation", "Count"],
            [
                ["`CreateSchedule`", fmt_num(mc.get("CreateSchedule", 0))],
                ["`UpdateSchedule`", fmt_num(mc.get("UpdateSchedule", 0))],
                ["`DeleteSchedule`", fmt_num(mc.get("DeleteSchedule", 0))],
                ["`ListSchedules`",  fmt_num(mc.get("ListSchedules", 0))],
            ],
        )

        heading(3, "Namespace and Cluster")
        table(
            ["Operation", "Count"],
            [
                ["`RegisterNamespace`",  fmt_num(mc.get("RegisterNamespace", 0))],
                ["`DescribeNamespace`",  fmt_num(mc.get("DescribeNamespace", 0))],
                ["`GetClusterInfo`",     fmt_num(mc.get("GetClusterInfo", 0))],
                ["`GetSystemInfo`",      fmt_num(mc.get("GetSystemInfo", 0))],
            ],
        )
        out.append("---")

    # ── Top talkers ────────────────────────────────────────────────────────────
    heading(2, "Top Talkers")

    sent = Counter(resolve(p["src"]) for p in packets)
    recv = Counter(resolve(p["dst"]) for p in packets)
    all_hosts = set(sent) | set(recv)

    table(
        ["Host", "Sent (packets)", "Received (packets)", "Total"],
        [
            [host, fmt_num(sent[host]), fmt_num(recv[host]), fmt_num(sent[host] + recv[host])]
            for host in sorted(all_hosts, key=lambda h: -(sent[h] + recv[h]))[:10]
        ],
    )

    return "\n".join(out) + "\n"


# ── Entry point ────────────────────────────────────────────────────────────────

def _build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="analyze.py",
        description="Analyze a Temporal .pcap capture and produce flow diagrams and statistics.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Protocol names (case-insensitive):
  grpc / http2    Temporal gRPC traffic (port 7233)
  pgsql / postgresql  PostgreSQL traffic (port 5432)
  http            Temporal UI HTTP traffic (port 8080)
  tcp             Raw TCP packets (no higher-level protocol decoded)
  arp             ARP packets
  <other>         Matched against tshark's Protocol column (e.g. ICMPv6, DNS)

Examples:
  analyze.sh capture.pcap                         # all protocols
  analyze.sh capture.pcap --only grpc             # gRPC traffic only
  analyze.sh capture.pcap --only grpc,http        # gRPC + Temporal UI
  analyze.sh capture.pcap --exclude pgsql,tcp     # hide PostgreSQL and raw TCP
""",
    )
    parser.add_argument("pcap", help="Path to the .pcap file to analyze")

    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--only", "-o",
        metavar="PROTOS",
        help="Comma-separated list of protocols to include (all others are hidden)",
    )
    group.add_argument(
        "--exclude", "-x",
        metavar="PROTOS",
        help="Comma-separated list of protocols to exclude",
    )
    return parser


def main() -> None:
    args = _build_arg_parser().parse_args()

    pcap = Path(args.pcap).expanduser().resolve()
    if not pcap.exists():
        print(f"Error: file not found: {args.pcap}", file=sys.stderr)
        sys.exit(1)

    # Parse filter lists and build a human-readable description.
    only    = [p.strip() for p in args.only.split(",")]    if args.only    else None
    exclude = [p.strip() for p in args.exclude.split(",")]  if args.exclude else None

    if only:
        filter_desc = "only: " + ", ".join(only)
    elif exclude:
        filter_desc = "exclude: " + ", ".join(exclude)
    else:
        filter_desc = None

    REPORTS_DIR.mkdir(parents=True, exist_ok=True)
    html_path  = REPORTS_DIR / f"{pcap.stem}_flow.html"
    stats_path = REPORTS_DIR / f"{pcap.stem}_stats.md"

    print(f"Analyzing {pcap.name} ...")
    if filter_desc:
        print(f"  Filter: {filter_desc}")

    print("  [1/4] Extracting packet data ...")
    packets, duration = extract_packets(pcap)
    if not packets:
        print("Error: no IP packets found in capture.", file=sys.stderr)
        sys.exit(1)

    print("  [2/4] Decoding gRPC calls (HTTP/2 on port 7233) ...")
    grpc = extract_grpc_calls(pcap)

    print("  [3/4] Applying filter ..." if filter_desc else "  [3/4] No filter applied.")
    packets, grpc = apply_filter(packets, grpc, only, exclude)
    if not packets:
        print("Error: no packets remain after filtering.", file=sys.stderr)
        sys.exit(1)
    total_bytes = sum(p["len"] for p in packets)
    print(f"        {fmt_num(len(packets))} packets  |  {fmt_bytes(total_bytes)}  |  {duration:.1f}s window  |  {fmt_num(len(grpc))} gRPC calls")

    print("  [4/4] Building diagrams and statistics ...")
    flow  = build_flow_diagram(packets)
    seq   = build_sequence_diagram(grpc)
    html  = generate_html(pcap.name, flow, seq, duration, len(packets), total_bytes, len(grpc), filter_desc)
    stats = generate_stats(pcap.name, packets, grpc, duration, filter_desc)
    html_path.write_text(html, encoding="utf-8")
    stats_path.write_text(stats, encoding="utf-8")

    print(f"\n✓  Done.")
    print(f"   Flow diagram : {html_path}")
    print(f"   Statistics   : {stats_path}")
    print()


if __name__ == "__main__":
    main()
