# analyze.sh — pcap analysis tool

`scripts/analyze.sh` reads a Temporal pcap capture and writes two output files to `captures/reports/`:

| Output file | Contents |
|---|---|
| `<name>_flow.html` | Data-flow diagram, all-protocol traffic sequence diagram, and gRPC sequence diagram (open in any browser) |
| `<name>_stats.md` | Protocol breakdown, connection matrix, gRPC method call counts, Temporal-specific insights |

All diagrams support zoom (scroll or buttons) and pan (drag).

## Prerequisites (one-time)

```bash
brew install wireshark   # provides tshark
pip install pyyaml       # optional — enables auto-loading container names from docker-compose.yml
```

Without `pyyaml` the script falls back to a hardcoded IP → container name map and works normally.

## Usage

```bash
# All protocols, all hosts
./scripts/analyze.sh captures/temporal_00001.pcap

# gRPC traffic only
./scripts/analyze.sh captures/temporal_00001.pcap --only grpc

# Multiple protocols
./scripts/analyze.sh captures/temporal_00001.pcap --only grpc,http

# Hide noisy protocols
./scripts/analyze.sh captures/temporal_00001.pcap --exclude pgsql,tcp

# Single worker only
./scripts/analyze.sh captures/temporal_00001.pcap --only-host hello-world-worker

# Hide infrastructure noise
./scripts/analyze.sh captures/temporal_00001.pcap --exclude-host wireshark,temporal-ui

# Combine protocol and host filters (AND semantics)
./scripts/analyze.sh captures/temporal_00001.pcap --only grpc --only-host hello-world-worker

# Hide Temporal inter-service traffic (history/matching/internal-worker)
./scripts/analyze.sh captures/temporal_00001.pcap --no-interservice

# SDK↔frontend gRPC only — no inter-service, no DB noise
./scripts/analyze.sh captures/temporal_00001.pcap --no-interservice --only grpc
```

## Flags

### Protocol filters (mutually exclusive)

| Flag | Description |
|---|---|
| `--only PROTOS` | Show only the listed protocols; all others are hidden |
| `--exclude PROTOS` | Hide the listed protocols |

Named protocols: `grpc` / `http2` (ports 7233/7234/7235/7239), `pgsql` / `postgresql` (port 5432), `http` (port 8080), `tcp`, `arp`. Anything else is matched against tshark's Protocol column (e.g. `ICMPv6`).

### Host filters (mutually exclusive)

| Flag | Description |
|---|---|
| `--only-host HOSTS` | Show only packets where the host is source or destination |
| `--exclude-host HOSTS` | Hide packets where the host is source or destination |

Host specs accept container names (e.g. `temporal-frontend`, `hello-world-worker`) or raw IPs (e.g. `172.20.0.40`). Container name → IP mappings are read automatically from `docker-compose.yml` when `pyyaml` is installed, so new services are picked up without touching the script.

### Convenience flags

| Flag | Description |
|---|---|
| `--no-interservice` | Exclude traffic to/from `temporal-history`, `temporal-matching`, and `temporal-internal-worker`. Keeps SDK workers, starters, `temporal-frontend`, and `temporal-ui`. Can be combined with any other filter. |

Protocol and host filters are ANDed when both are specified.

## Output descriptions

**Data-flow diagram** — shows which containers communicated and how much traffic each connection carried.

**Traffic sequence diagram** — all protocol events in chronological order; gRPC arrows show the RPC method name, others show the protocol label. Consecutive identical events are compressed (xN).

**gRPC sequence diagram** — every gRPC method call in chronological order, with consecutive identical calls from the same source compressed (e.g. `PollWorkflowTaskQueue (x1,234)`).
