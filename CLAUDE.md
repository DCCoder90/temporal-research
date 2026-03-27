# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

A local Temporal cluster environment for demonstrating workflow patterns, with network packet capture and analysis. It runs Temporal server, PostgreSQL, UI, and example workflows in Docker containers, and includes tools to capture and visualize gRPC traffic between components.

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

# Analyze captured packets
./scripts/analyze.sh captures/temporal_*.pcap
./scripts/analyze.sh captures/temporal_*.pcap --only grpc
./scripts/analyze.sh captures/temporal_*.pcap --exclude pgsql,tcp
./scripts/analyze.sh captures/temporal_*.pcap --only-host worker-name

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

**Analysis pipeline**: `tshark` captures raw pcap → `analyze.py` decodes gRPC frames, reconstructs message flows, and generates interactive HTML diagrams (data-flow, sequence diagrams) + JSON reports.

## Key Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Defines all services, static IPs, profiles, dependencies |
| `temporal-config/scripts/setup.sh` | DB schema init script run by `temporal-setup` container |
| `temporal-config/dynamicconfig/docker.yaml` | Disables client version check, sets max ID length |
| `tshark/Dockerfile` | Alpine + tshark; ring-buffer capture on `temporal-net` |
| `wireshark/hosts` | Static IP → container name mappings for pcap display |
| `scripts/analyze.py` | Main pcap analysis engine; auto-loads container names from docker-compose.yml if `pyyaml` is installed |

## Go Module Notes

Each example has its own `go.mod`. The Dockerfiles run `go mod tidy` during build to bootstrap dependencies — this is intentional since `go.sum` files are not committed.
