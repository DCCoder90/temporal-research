# temporal-analyze — User Guide

**temporal-analyze** turns a `.pcap` capture from a running Temporal cluster into interactive diagrams, statistics, and a SQL-queryable dataset. It works as a native desktop GUI or a headless CLI.

---

## Table of contents

1. [Requirements](#requirements)
2. [Configuration files](#configuration-files)
3. [GUI mode](#gui-mode)
   - [Opening a capture](#opening-a-capture)
   - [Filter panel](#filter-panel)
   - [Running analysis](#running-analysis)
   - [Diagrams tab](#diagrams-tab)
   - [Statistics tab](#statistics-tab)
   - [Query tab](#query-tab)
   - [Exporting reports](#exporting-reports)
4. [CLI mode](#cli-mode)
   - [Basic usage](#basic-usage)
   - [Flags reference](#flags-reference)
   - [Protocol names](#protocol-names)
   - [Host filtering](#host-filtering)
   - [JSON output](#json-output)
4. [Understanding the output](#understanding-the-output)
   - [Data Flow Diagram](#data-flow-diagram)
   - [Traffic Sequence Diagram](#traffic-sequence-diagram)
   - [gRPC Sequence Diagram](#grpc-sequence-diagram)
   - [Statistics sections](#statistics-sections)
5. [SQL query reference](#sql-query-reference)
   - [packets table](#packets-table)
   - [grpc_calls table](#grpc_calls-table)
   - [Example queries](#example-queries)
6. [Troubleshooting](#troubleshooting)

---

## Requirements

- **tshark** must be in your `PATH`. Install via:
  - macOS: `brew install wireshark`
  - Ubuntu/Debian: `sudo apt-get install tshark`
  - Windows: install [Wireshark](https://www.wireshark.org/download.html) and ensure `tshark.exe` is on `PATH`
- Capture files must be in standard `.pcap` format (as written by tshark's ring-buffer)

---

## Configuration files

`temporal-analyze` reads `config.json` at startup to know how to resolve container IPs to names and how to label protocol ports. **The file must be present** — the application will refuse to start if it is missing.

### File locations

The tool searches for `config.json` in the following locations, in order:

1. **The directory containing the binary** — the most convenient location for CLI users. Extract the release archive and keep both files together.
2. **`~/.config/temporal-analyze/`** — the preferred location for the GUI, since the binary lives inside a `.app` bundle on macOS.

### config.json

A single JSON file with two top-level maps:

```json
{
  "hosts": {
    "172.20.0.21": "temporal-frontend",
    "172.20.0.22": "temporal-history",
    "172.20.0.10": "postgresql",
    "10.0.0.10": "order-worker",
    "10.0.0.11": "payment-worker"
  },
  "ports": {
    "7233": "Temporal gRPC (frontend)",
    "5432": "PostgreSQL",
    "8080": "HTTP (Temporal UI)",
    "9090": "My custom service"
  }
}
```

- **`hosts`** — maps container IP addresses to human-readable names. These names appear in diagrams, statistics, and SQL query results. Any IP not listed is displayed as its raw address.
- **`ports`** — maps TCP port numbers to human-readable labels used in the Protocol Breakdown table and connection matrix.

> **Note:** the gRPC ports used for packet decoding (`7233`, `7234`, `7235`, `7239`) are fixed Temporal protocol constants and cannot be changed via `config.json`. Only the display labels are configurable here.

### Customising for your own cluster

If your Temporal cluster uses different IPs or service names than the defaults:

1. Open `config.json` in any text editor
2. Replace the IP addresses and names in the `"hosts"` map to match your setup
3. Add entries for any custom workers or services you want named in diagrams
4. Update `"ports"` if you use non-standard port numbers
5. Restart `temporal-analyze`

A default `config.json` for the standard `temporalcoms` Docker Compose setup is included in every release archive. It is a good starting point for customisation.

### Error if file is missing

If `config.json` is not found at startup, the application prints an error listing where it looked:

```
Configuration error: config.json not found

Searched:
  /usr/local/bin
  /Users/yourname/.config/temporal-analyze

Place config.json in one of these directories.
A default config.json is included in each release archive.
```

Download the default `config.json` from the [Releases page](https://github.com/DCCoder90/temporal-research/releases) (it is bundled inside each release `.zip`) and place it in one of the listed directories.

---

## GUI mode

### Opening a capture

There are two ways to load a file:

- **Click** the file zone in the sidebar to open a native file picker
- **Drag and drop** a `.pcap` file from Finder/Explorer onto the sidebar

The file zone shows the filename once a file is selected. The **Analyze** button becomes active.

### Filter panel

The sidebar filter panel lets you narrow the analysis before it runs. Filters are applied to both diagrams and statistics.

#### Protocol Filter

| Option | Behaviour |
|--------|-----------|
| All protocols | Everything in the capture is shown (default) |
| Only | Only packets matching the listed protocols are included |
| Exclude | Matching packets are removed; everything else is shown |

Enter comma-separated protocol names in the text field when using Only or Exclude. See [Protocol names](#protocol-names) for valid values.

#### Host Filter

| Option | Behaviour |
|--------|-----------|
| All hosts | No host filtering (default) |
| Only | Only packets where the source **or** destination matches are included |
| Exclude | Packets involving any listed host are removed |

Enter comma-separated container names (e.g. `hello-world-worker`) or raw IPs. Protocol and host filters are ANDed together.

#### Hide internal services

Checking **Hide internal services** excludes all traffic involving `temporal-history`, `temporal-matching`, and `temporal-internal-worker`. Useful for focusing on client-facing gRPC and PostgreSQL traffic.

### Running analysis

Click **Analyze**. The status bar shows progress. When complete:

- The metadata bar at the top of the main area shows file, duration, packet count, bytes, gRPC call count, and the active filter (if any)
- The view tabs become visible
- The **Diagrams** tab is shown automatically

### Diagrams tab

Three diagrams are shown (subject to filters):

| Diagram | When shown |
|---------|-----------|
| Data Flow | Always |
| Traffic Sequence | Hidden when the only filter is `grpc` / `http2` (would be identical to the gRPC diagram) |
| gRPC Sequence | Always; paginated when there are more than 150 compressed entries |

**Zoom and pan controls** appear in each diagram's toolbar:

- `+` / `−` buttons zoom in/out
- `⟳` resets to fit
- Scroll the mouse wheel to zoom
- Click and drag to pan

**gRPC Sequence pagination**: when a capture has many gRPC calls, the diagram is split across pages. Use the `‹` and `›` buttons in the toolbar to navigate. The current page is shown as `Page N of M`.

### Statistics tab

Switch to the **Statistics** tab to see a rendered Markdown report covering:

- Summary (file, duration, packet count, bytes, gRPC calls, active filter)
- Protocol Breakdown
- Connection Matrix (top 30 directed connections)
- Network Health (TCP retransmissions, RTT min/avg/p95/max)
- gRPC Method Calls (frequency table, status codes, Temporal-specific insights)
- Top Talkers

### Query tab

The **Query** tab provides a full SQL engine over the packet and gRPC call data loaded from the last analysis. See [SQL query reference](#sql-query-reference) for the full schema.

#### Editor

The editor supports SQL syntax highlighting. Use **Ctrl+Enter** (or **Cmd+Enter** on Mac) to run a query without clicking the button.

#### Query history

Previous queries are saved in browser `localStorage` (up to 10 entries, most recent first). Navigate history with:

- **Ctrl+Up** — go back to an older query
- **Ctrl+Down** — go forward to a newer query (or back to what you were typing)

Your draft is preserved while navigating history, so pressing Ctrl+Down all the way forward restores what you had typed.

#### Sample queries

The **— sample queries —** dropdown preloads common queries into the editor. Selecting a sample does not run it automatically — edit it first if you like, then press Run.

#### Results

Results appear in a scrollable table below the editor. Click any column header to sort ascending; click again to sort descending. A `↑` or `↓` indicator shows the active sort column and direction.

Results are capped at **10,000 rows**. A warning appears if the result was truncated — add `LIMIT` or a `WHERE` clause to narrow it down.

#### CSV export

After a successful query, the **Export CSV** button becomes active. Click it to download the full result (up to the 10,000-row cap) as a CSV file named `query_result.csv`.

### Exporting reports

Click **Export** to write two files alongside the original pcap:

| File | Contents |
|------|----------|
| `<pcapname>_flow.html` | Self-contained interactive HTML with all three diagrams |
| `<pcapname>_stats.md` | Markdown statistics report |

Export reuses the last analysis result — it does **not** re-run tshark.

The status bar shows the names of the exported files when done.

---

## CLI mode

### Basic usage

```bash
temporal-analyze <pcap-file> [flags]
```

Output files are written to the same directory as the input pcap:

- `<name>_flow.html` — interactive HTML diagrams
- `<name>_stats.md` — Markdown statistics report

Progress is written to stderr. Stdout is used only when `--json` is set.

### Flags reference

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--only` | `-o` | string | Comma-separated protocols to include; all others hidden |
| `--exclude` | `-x` | string | Comma-separated protocols to exclude |
| `--only-host` | | string | Comma-separated container names or IPs to include |
| `--exclude-host` | | string | Comma-separated hosts to exclude |
| `--no-interservice` | | bool | Exclude temporal-history, temporal-matching, temporal-internal-worker |
| `--json` | | bool | Write JSON analysis result to stdout instead of files |
| `--quiet` | `-q` | bool | Suppress all progress output to stderr |
| `--version` | | | Print version string and exit |
| `--help` | `-h` | | Print usage |

`--only` and `--exclude` are mutually exclusive. `--only-host` and `--exclude-host` are mutually exclusive.

### Protocol names

Protocol names are case-insensitive. Named aliases map to specific ports:

| Name | Alias | Ports covered |
|------|-------|---------------|
| `grpc` | `http2` | 7233, 7234, 7235, 7239 (all Temporal gRPC) |
| `pgsql` | `postgresql` | 5432 |
| `http` | | 8080 (Temporal UI) |
| `tcp` | | All TCP packets |
| `arp` | | ARP packets |
| any other value | | Matched against tshark's Protocol column value |

### Host filtering

Host specs can be container names or raw IPs. Either form resolves to both directions (src OR dst):

```bash
# Include only traffic from/to the hello-world-worker
temporal-analyze capture.pcap --only-host hello-world-worker

# Include multiple hosts
temporal-analyze capture.pcap --only-host hello-world-worker,hello-world-starter

# Exclude Wireshark and the Temporal UI from diagrams
temporal-analyze capture.pcap --exclude-host wireshark,temporal-ui

# Combine protocol and host filters (ANDed)
temporal-analyze capture.pcap --only grpc --only-host hello-world-worker
```

Known container names:

| Container | IP |
|-----------|----|
| `postgresql` | 172.20.0.10 |
| `temporal-frontend` | 172.20.0.21 |
| `temporal-history` | 172.20.0.22 |
| `temporal-matching` | 172.20.0.23 |
| `temporal-internal-worker` | 172.20.0.24 |
| `temporal-ui` | 172.20.0.30 |
| `hello-world-worker` | 172.20.0.40 |
| `hello-world-starter` | 172.20.0.41 |
| `scheduled-worker` | 172.20.0.42 |
| `signals-worker` | 172.20.0.43 |
| `child-workflows-worker` | 172.20.0.44 |
| `retries-worker` | 172.20.0.45 |
| `saga-worker` | 172.20.0.46 |
| `wireshark` | 172.20.0.50 |
| `signals-starter` / `signals-approve` / `signals-reject` | 172.20.0.52–54 |
| `child-workflows-starter` | 172.20.0.55 |
| `retries-starter` | 172.20.0.56 |
| `saga-starter` / `saga-fail-starter` | 172.20.0.57–58 |

### JSON output

`--json` writes a structured JSON document to stdout and skips writing HTML/Markdown files. Combine with `--quiet` to suppress stderr progress so stdout is clean for piping.

```bash
# Pipe to jq
temporal-analyze capture.pcap --json --quiet | jq '.grpc_calls | map(.method) | unique'

# Save to file
temporal-analyze capture.pcap --json --quiet > analysis.json
```

**JSON structure:**

```json
{
  "pcap": "temporal_00001.pcap",
  "duration_s": 12.34,
  "packet_count": 5678,
  "grpc_count": 120,
  "filter": "only: grpc",
  "packets": [
    {
      "time": 1234567890.123,
      "src_ip": "172.20.0.41",
      "dst_ip": "172.20.0.21",
      "src_port": "54321",
      "dst_port": "7233",
      "bytes": 512,
      "protocol": "HTTP2",
      "tcp_stream": 3,
      "tcp_len": 480,
      "tcp_flags": 24,
      "retransmit": false,
      "rtt": 0.000312
    }
  ],
  "grpc_calls": [
    {
      "time": 1234567890.456,
      "src": "hello-world-starter",
      "dst": "temporal-frontend",
      "full_path": "/temporal.api.workflowservice.v1.WorkflowService/StartWorkflowExecution",
      "service": "WorkflowService",
      "method": "StartWorkflowExecution",
      "tcp_stream": 3,
      "stream_id": 1,
      "status_code": 0
    }
  ]
}
```

`filter` is omitted when no filter is active. `status_code` is `-1` if no gRPC response trailer was captured; `0` is OK.

---

## Understanding the output

### Data Flow Diagram

A Mermaid `flowchart LR` showing every unique directed connection between hosts. Each arrow is labelled with the protocol (or port label), the total packet count, and total bytes in that direction. PostgreSQL nodes use a cylinder shape (`[( )]`). Useful for a quick topology overview.

### Traffic Sequence Diagram

A `sequenceDiagram` showing all packets in chronological order. gRPC packets are replaced by their decoded method name. Consecutive identical events between the same pair of hosts are compressed to `(xN)` to keep the diagram readable. This diagram is hidden when the only filter is `grpc`/`http2` (it would be identical to the gRPC diagram).

Compressed up to 150 entries. If the capture has more events, the diagram is truncated with a note at the bottom.

### gRPC Sequence Diagram

A `sequenceDiagram` showing only decoded gRPC calls, in chronological order. Sequence numbers are shown. Consecutive identical calls from the same source are compressed to `(xN)`.

When there are more than 150 compressed entries the diagram is paginated. Navigate with the `‹` / `›` buttons in the toolbar. Each page is a complete standalone diagram.

### Statistics sections

#### Summary

Quick stats: file name, timestamp, capture duration, active filter, total packets, total bytes, decoded gRPC call count.

#### Protocol Breakdown

All protocols present in the capture, sorted by packet count. The `% of Traffic` column is relative to total packet count.

#### Connection Matrix

Top 30 directed source → destination pairs by packet count, labelled with protocol and port. Uses container names wherever possible.

#### Network Health

**TCP Retransmissions** — total retransmitted packets and percentage of total traffic. If any retransmissions were detected, the top 5 source → destination paths are shown.

**TCP Round-Trip Times** — min, average, p95, and max ACK RTT across all TCP packets where tshark captured a valid RTT measurement. RTTs are in milliseconds.

#### gRPC Method Calls

**Method frequency table** — each unique gRPC method seen in the capture, call count, and which services called it.

**gRPC Status Codes** — only shown when more than one distinct status code was observed. `0` = OK, `-1` = no response trailer captured. Non-zero codes include their standard gRPC meaning.

**Temporal-Specific Insights** — groups methods into:
- Workflow Lifecycle (start, signal, query, cancel, terminate, history fetch)
- Task Queue Activity (poll and respond calls; worker efficiency ratio)
- Activity Retry Analysis (failure count and failure rate, when failures were observed)
- Schedule Management
- Namespace and Cluster operations

#### Top Talkers

Top 10 hosts by total packet involvement (sent + received combined).

---

## SQL query reference

The **Query** tab runs SQLite SQL against two tables populated from the last analysis. Full SQLite syntax is supported including `WHERE`, `GROUP BY`, `HAVING`, `ORDER BY`, `JOIN`, `LIMIT`, window functions, and all built-in functions.

### packets table

| Column | Type | Description |
|--------|------|-------------|
| `time` | REAL | Unix epoch timestamp |
| `src_ip` | TEXT | Source IP address |
| `dst_ip` | TEXT | Destination IP address |
| `src` | TEXT | Source container name (resolved from IP) |
| `dst` | TEXT | Destination container name (resolved from IP) |
| `src_port` | TEXT | Source TCP port |
| `dst_port` | TEXT | Destination TCP port |
| `protocol` | TEXT | Protocol label from tshark (TCP, HTTP2, PGSQL, …) |
| `bytes` | INTEGER | Total frame size in bytes (including headers) |
| `tcp_stream` | INTEGER | TCP stream index; `-1` if not TCP |
| `tcp_len` | INTEGER | TCP payload bytes; `0` if not TCP |
| `tcp_flags` | INTEGER | TCP flags as integer — use bitwise `&` (e.g. `tcp_flags & 2` = SYN) |
| `retransmit` | INTEGER | `1` if tshark detected a retransmission, `0` otherwise |
| `rtt` | REAL | TCP ACK round-trip time in seconds; `0` if unavailable |

### grpc_calls table

| Column | Type | Description |
|--------|------|-------------|
| `time` | REAL | Unix epoch timestamp |
| `src` | TEXT | Calling service name |
| `dst` | TEXT | Target service name |
| `full_path` | TEXT | Full gRPC `:path` header, e.g. `/temporal.api.workflowservice.v1.WorkflowService/PollWorkflowTaskQueue` |
| `service` | TEXT | Service name portion of the path (second-to-last segment) |
| `method` | TEXT | Method name (last segment) |
| `tcp_stream` | INTEGER | TCP stream index (links to `packets.tcp_stream`) |
| `stream_id` | INTEGER | HTTP/2 stream ID within the TCP connection |
| `status_code` | INTEGER | gRPC status code (`0` = OK, `-1` = not captured) |

### Example queries

```sql
-- Protocol breakdown
SELECT protocol, COUNT(*) AS packets, SUM(bytes) AS total_bytes
FROM packets
GROUP BY protocol
ORDER BY total_bytes DESC;

-- Top talkers by bytes sent
SELECT src, COUNT(*) AS packets, SUM(bytes) AS bytes
FROM packets
GROUP BY src
ORDER BY bytes DESC
LIMIT 20;

-- gRPC call frequency
SELECT method, COUNT(*) AS calls
FROM grpc_calls
GROUP BY method
ORDER BY calls DESC;

-- Failed gRPC calls (non-zero, non-unknown status)
SELECT src, dst, method, status_code, COUNT(*) AS n
FROM grpc_calls
WHERE status_code > 0
GROUP BY src, dst, method, status_code
ORDER BY n DESC;

-- Average RTT per source/destination pair (in ms)
SELECT src, dst,
       ROUND(AVG(rtt) * 1000, 3) AS avg_rtt_ms,
       COUNT(*) AS samples
FROM packets
WHERE rtt > 0
GROUP BY src, dst
ORDER BY avg_rtt_ms DESC;

-- Retransmitted packets by path
SELECT src, dst, protocol, COUNT(*) AS retransmits
FROM packets
WHERE retransmit = 1
GROUP BY src, dst, protocol
ORDER BY retransmits DESC;

-- Traffic volume per minute
SELECT CAST(time / 60 AS INTEGER) * 60 AS minute_epoch,
       COUNT(*) AS packets,
       SUM(bytes) AS bytes
FROM packets
GROUP BY minute_epoch
ORDER BY minute_epoch;

-- Join: gRPC calls with their packets (by TCP stream)
SELECT p.time, p.src, p.dst, g.method, p.bytes
FROM packets p
JOIN grpc_calls g ON p.tcp_stream = g.tcp_stream
ORDER BY p.time
LIMIT 100;

-- Identify SYN packets (new connections)
SELECT src, dst, COUNT(*) AS syn_count
FROM packets
WHERE tcp_flags & 2 = 2       -- SYN bit set
  AND tcp_flags & 16 = 0      -- ACK bit not set
GROUP BY src, dst
ORDER BY syn_count DESC;
```

---

## Troubleshooting

### "Configuration error: config.json not found"

The application requires `config.json` to be present before it can start. It searches in the binary's directory first, then `~/.config/temporal-analyze/`. See [Configuration files](#configuration-files) for full details.

Download the default `config.json` from the [Releases page](https://github.com/DCCoder90/temporal-research/releases) (it is bundled inside each release `.zip`) and place it in the appropriate directory.

### "tshark not found in PATH"

Install tshark and ensure the directory containing it is on your `PATH`. On macOS with Homebrew: `brew install wireshark`. Verify with `tshark --version`.

### No gRPC calls decoded

This happens when tshark's HTTP/2 dissector did not recognise traffic on port 7233. Possible causes:

- The capture was taken before any Temporal gRPC traffic occurred
- tshark version is too old to decode HTTP/2 on non-standard ports (try upgrading)

Verify manually:
```bash
tshark -r capture.pcap \
  -d tcp.port==7233,http2 \
  -d tcp.port==7234,http2 \
  -d tcp.port==7235,http2 \
  -d tcp.port==7239,http2 \
  -Y http2.headers.path \
  -T fields -e http2.headers.path | head
```

### "no packets matched your filter"

The combination of protocol and host filters removed all packets. Try:
- Removing the host filter and rerunning to check the protocol filter alone
- Using `--exclude` instead of `--only`
- Checking container names with `docker ps` — the name must exactly match an entry in `config.json`

### Statistics show 0 gRPC calls but packets exist

gRPC decoding is a best-effort second tshark pass. If the capture only has TCP/HTTP2 packets but tshark could not parse HTTP/2 headers (e.g. mid-stream capture without the initial handshake), gRPC calls will be empty while raw packets are still shown in all other sections.

### Diagrams render blank in the exported HTML

The exported HTML loads Mermaid.js and svg-pan-zoom from CDN. If you are offline, open the file in a browser with network access, or host the CDN scripts locally.
