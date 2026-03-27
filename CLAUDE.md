# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

A local Temporal cluster environment for demonstrating workflow patterns, with network packet capture and analysis. It runs Temporal server, PostgreSQL, UI, and example workflows in Docker containers, and includes a Go tool (`temporal-analyze`) to capture and visualize gRPC traffic between components.

## Common Commands

```bash
# Start everything (first run needs --build to compile Go examples)
docker compose up --build

# Subsequent runs (images already built)
docker compose up

# Run specific example workflows (one-shot containers)
docker compose run --rm hello-world-starter
docker compose run --rm signals-starter       # Start an approval workflow
docker compose run --rm signals-approve        # Send approval signal
docker compose run --rm child-workflows-starter
docker compose run --rm retries-starter
docker compose run --rm saga-starter           # Happy path
docker compose run --rm saga-fail-starter      # Failure path (triggers compensation)

# Analyze captured packets (CLI)
cd tools/temporal-analyze
./temporal-analyze captures/temporal_00001.pcap
./temporal-analyze captures/temporal_00001.pcap --only grpc
./temporal-analyze captures/temporal_00001.pcap --exclude pgsql,tcp
./temporal-analyze captures/temporal_00001.pcap --only-host worker-name
./temporal-analyze captures/temporal_00001.pcap --no-interservice
./temporal-analyze captures/temporal_00001.pcap --json --quiet | jq '.grpc_calls | map(.method) | unique'

# Build temporal-analyze (CLI only)
cd tools/temporal-analyze && go build -tags nogui -o temporal-analyze .

# Build temporal-analyze (GUI — requires Wails v2)
cd tools/temporal-analyze && wails build

# Teardown (wipes all data — PostgreSQL uses tmpfs)
docker compose down
```

## Architecture

**Network topology** (`temporal-net` bridge, 172.20.0.0/16):
- PostgreSQL (172.20.0.10) — ephemeral, tmpfs only
- temporal-frontend (172.20.0.21) — external gRPC :7233, membership :6933
- temporal-history (172.20.0.22) — gRPC :7234, membership :6934
- temporal-matching (172.20.0.23) — gRPC :7235, membership :6935
- temporal-internal-worker (172.20.0.24) — gRPC :7239, membership :6939
- Temporal UI (172.20.0.30) — web UI on port 8080
- tshark (host networking) — rolling ring-buffer capture (5×50MB files) to `/captures`
- Wireshark web GUI (172.20.0.50) — port 3000 for pcap visualization

**Startup sequence and race condition avoidance**: `temporalio/admin-tools` runs as a one-shot init container (`temporal-setup`) that creates DB schema via `temporal-sql-tool` and exits 0. The four `temporalio/server` containers all depend on `temporal-setup: service_completed_successfully`, so they never race to initialize `cluster_metadata_info`. A second one-shot container (`temporal-default-namespace`) retries until the frontend is ready, then registers the `default` namespace.

**Service discovery**: Each service sets `TEMPORAL_BROADCAST_ADDRESS` to its own static IP, which gets registered in PostgreSQL's RingPop membership table. Services discover each other by querying that table — no explicit peer hostnames are needed (except `PUBLIC_FRONTEND_ADDRESS` on the internal worker).

**Example workflow pattern** — all 6 examples follow identical layout:
```
example-name/
├── go.mod                    # Go 1.22, temporal SDK v1.26.1
├── workflow/workflow.go      # Workflow + Activity definitions
├── worker/main.go            # Registers workflow/activity, polls task queue
└── starter/main.go           # Connects as client, starts workflow, waits for result
```

**Analysis pipeline**: `tshark` captures raw pcap → `temporal-analyze` decodes IP packets and gRPC calls, resolves container IPs to names, builds interactive Mermaid diagrams, and generates HTML + Markdown reports. The tool is available as a native desktop GUI (Wails) or a headless CLI.

## Key Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Defines all services, static IPs, profiles, dependencies |
| `temporal-config/scripts/setup.sh` | DB schema init script run by `temporal-setup` container |
| `temporal-config/dynamicconfig/docker.yaml` | Disables client version check, sets max ID length |
| `tshark/Dockerfile` | Alpine + tshark; ring-buffer capture on `temporal-net` |
| `wireshark/hosts` | Static IP → container name mappings for Wireshark pcap display |
| `tools/temporal-analyze/` | Go-based pcap analysis tool (GUI + CLI); see its own README |
| `tools/temporal-analyze/config.json` | Maps container IPs to names and ports to labels; bundled in releases |
| `tools/temporal-analyze/docs/user-guide.md` | Full end-user documentation for temporal-analyze |

## temporal-analyze Tool

The analysis tool lives in `tools/temporal-analyze/`. Full documentation is in `tools/temporal-analyze/README.md` and `tools/temporal-analyze/docs/user-guide.md`.

**Build tags:**
- `nogui` — CLI-only binary (no Wails dependency); used by GitHub Actions releases
- *(no tag)* — GUI binary (requires Wails v2 installed)

**CLI flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--only <protocols>` | `-o` | Comma-separated protocols to include |
| `--exclude <protocols>` | `-x` | Comma-separated protocols to exclude |
| `--only-host <hosts>` | | Comma-separated container names or IPs to include |
| `--exclude-host <hosts>` | | Comma-separated hosts to exclude |
| `--no-interservice` | | Exclude temporal-history, temporal-matching, temporal-internal-worker |
| `--json` | | Write JSON result to stdout instead of files |
| `--quiet` | `-q` | Suppress progress output to stderr |
| `--version` | | Print version and exit |

**Configuration**: requires `config.json` alongside the binary (or in `~/.config/temporal-analyze/`). The file maps container IPs to names and ports to labels. The application refuses to start without it — defaults matching this repo's Docker Compose setup are bundled in every GitHub release archive.

**GUI note**: `config.Load()` is called in `app.startup()` (not in `main()`) so that Wails binding generation (`wails build`) does not fail when run from a temp directory that has no `config.json`. Wails-internal flags (prefixed `-wails`) bypass the CLI path entirely.

## Go Module Notes

Each example has its own `go.mod`. The Dockerfiles run `go mod tidy` during build to bootstrap dependencies — this is intentional since `go.sum` files are not committed.

`tools/temporal-analyze` has its own `go.mod` (module `temporal-analyze`) and is built separately from the examples.
