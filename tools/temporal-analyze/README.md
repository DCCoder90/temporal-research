# temporal-analyze

A tool for analyzing Temporal `.pcap` captures. Available as a **native desktop GUI** or a **CLI** that writes self-contained HTML and Markdown reports.

> Full end-user documentation is in **[docs/user-guide.md](docs/user-guide.md)**.

---

## What it does

Given a `.pcap` file captured from a running Temporal cluster, `temporal-analyze`:

- Decodes all IP packets and gRPC calls from HTTP/2 traffic on Temporal ports
- Resolves container IPs to human-readable names
- Builds interactive **Mermaid diagrams** — data flow, traffic sequence, and gRPC sequence (paginated)
- Generates a **statistics report** — protocol breakdown, connection matrix, network health (RTT, retransmissions), and Temporal-specific insights
- Exposes a **SQL query engine** against the packet and gRPC call data (GUI only)

---

## Requirements

- [tshark](https://www.wireshark.org/docs/man-pages/tshark.html) must be in `PATH`
- Go 1.22+ (to build from source)
- [Wails v2](https://wails.io) (GUI builds only)

---

## Building

### CLI binary

```bash
go build -tags nogui -o temporal-analyze .
```

### GUI (macOS .app)

```bash
wails build
# Output: build/bin/temporal-analyze.app
```

### GUI (dev mode with hot-reload)

```bash
wails dev
```

---

## Quick start

### GUI

Launch the app, drop a `.pcap` file onto the sidebar, optionally set filters, then click **Analyze**. Use the **Diagrams**, **Statistics**, and **Query** tabs to explore the capture. Click **Export** to write `_flow.html` and `_stats.md` alongside the pcap.

### CLI

```bash
# Analyze a capture — writes _flow.html and _stats.md next to the pcap
./temporal-analyze captures/temporal_00001.pcap

# gRPC traffic only
./temporal-analyze captures/temporal_00001.pcap --only grpc

# Hide internal Temporal services (history, matching, internal-worker)
./temporal-analyze captures/temporal_00001.pcap --no-interservice

# Output raw JSON (packets + gRPC calls) to stdout
./temporal-analyze captures/temporal_00001.pcap --json --quiet | jq '.grpc_calls | map(.method) | unique'

# Show version
./temporal-analyze --version
```

### All CLI flags

| Flag | Short | Description |
|------|-------|-------------|
| `--only <protocols>` | `-o` | Comma-separated protocols to include; all others hidden |
| `--exclude <protocols>` | `-x` | Comma-separated protocols to exclude |
| `--only-host <hosts>` | | Comma-separated container names or IPs to include |
| `--exclude-host <hosts>` | | Comma-separated hosts to exclude |
| `--no-interservice` | | Exclude history, matching, and internal-worker |
| `--json` | | Write analysis result as JSON to stdout instead of files |
| `--quiet` | `-q` | Suppress progress output to stderr |
| `--version` | | Print version and exit |
| `--help` | `-h` | Print usage |

Protocol names: `grpc`, `http2`, `pgsql`, `postgresql`, `http`, `tcp`, `arp`, or any tshark protocol column value.

---

## Output files (CLI / Export)

| File | Description |
|------|-------------|
| `<name>_flow.html` | Self-contained interactive HTML with all three diagrams |
| `<name>_stats.md` | Markdown statistics report |

---

## Project layout

```
app.go                    Wails App struct — Analyze, Export, QueryDB, ResolveIP
query.go                  In-memory SQLite engine (GUI only)
main.go                   GUI entry point (!nogui)
main_nogui.go             CLI-only entry point (nogui build tag)
cli/cli.go                cobra command — flags, --json, --quiet, --version
internal/
  analysis/analysis.go    Pipeline: extract → filter → diagrams → stats
  config/config.go        IP map, port labels, gRPC ports, filter name aliases
  diagram/diagram.go      Mermaid diagram builders (flow, sequence, traffic)
  filter/filter.go        Protocol and host filter logic
  report/report.go        HTML and Markdown report generators
  tshark/tshark.go        tshark invocation, Packet and GRPCCall types
frontend/
  index.html              GUI shell
  src/main.js             All GUI logic (tabs, diagrams, query, history, sort)
  src/style.css           GUI styles
```
