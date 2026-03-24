# temporalcoms

A local Temporal cluster running as individual Docker containers, with a Go "hello world" workflow and Wireshark for observing network traffic.

> Vibe-coded with [Claude Code](https://github.com/anthropics/claude-code).

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Mac/Windows) or Docker + Docker Compose (Linux)

## Start everything

```bash
docker compose up --build
```

The first run downloads images and compiles the Go binaries — give it a few minutes. On subsequent runs `--build` can be omitted.

**Startup order:**
1. tshark starts capturing on `temporal-net`
2. PostgreSQL becomes healthy
3. Temporal server initialises (schema, metadata, default namespace)
4. Temporal UI becomes available

Example workflow workers and starters are **not** started automatically — see [Example workflows](#example-workflows) below.

## Running the hello-world workflow

```bash
docker compose run --rm hello-world-starter
```

This automatically starts `hello-world-worker` as a dependency, runs the workflow, prints the result, and exits. You should see:

```
✓ Workflow result: Hello, World! (from Temporal activity)
```

To run it again just re-run the same command. The worker stays running until you stop it:

```bash
docker compose stop hello-world-worker
```

## Web UIs

| URL | What it is |
|---|---|
| http://localhost:8080 | Temporal UI — browse workflows, task queues, namespaces |
| http://localhost:3000 | Wireshark — full packet capture GUI |

## Using Wireshark

Packet capture is split across two containers:

- **tshark** runs with host networking, captures all traffic on the `temporal-net` bridge, and writes a rolling ring-buffer of pcap files (5 × 50 MB) to `./captures/`
- **wireshark** provides the web GUI at http://localhost:3000 and mounts `./captures` read-only

To open a capture:
1. Open http://localhost:3000 in your browser
2. In the Wireshark GUI, go to **File → Open** and navigate to `/captures/`
3. Open the latest `temporal_*.pcap` file
4. Enable **View → Name Resolution → Resolve Network Addresses** to see container names instead of IPs

### Useful display filters

| Filter | What it shows |
|---|---|
| `tcp.port == 7233` | All Temporal gRPC traffic |
| `ip.src_host != "wireshark" && ip.dst_host != "wireshark" && !pgsql` | Everything except Wireshark's own traffic and PostgreSQL chatter — good starting point |
| `tcp.port == 7233 && ip.src_host != "wireshark" && ip.dst_host != "wireshark"` | Only Temporal gRPC, no Wireshark noise |
| `(ip.src_host == "hello-world-worker" \|\| ip.dst_host == "hello-world-worker")` | All traffic to/from the worker |
| `(ip.src_host == "hello-world-starter" \|\| ip.dst_host == "hello-world-starter")` | All traffic to/from the starter (workflow submission) |
| `ip.src_host == "temporal" \|\| ip.dst_host == "temporal"` | All traffic in and out of the Temporal server |
| `tcp.port == 5432` | PostgreSQL only — useful for watching schema/persistence activity |
| `tcp.port == 8080` | Temporal UI HTTP traffic |


## Analyzing captures

`scripts/analyze.sh` reads a pcap file and writes two output files to `captures/reports/`:

| Output file | Contents |
|---|---|
| `<name>_flow.html` | Data-flow diagram + gRPC sequence diagram (open in any browser) |
| `<name>_stats.md` | Protocol breakdown, connection matrix, gRPC method call counts, Temporal-specific insights |

**Prerequisites** (one-time):
```bash
brew install wireshark   # provides tshark
```

**Usage:**
```bash
# All protocols
./scripts/analyze.sh captures/temporal_00001.pcap

# gRPC traffic only
./scripts/analyze.sh captures/temporal_00001.pcap --only grpc

# Multiple protocols
./scripts/analyze.sh captures/temporal_00001.pcap --only grpc,http

# Hide noisy protocols
./scripts/analyze.sh captures/temporal_00001.pcap --exclude pgsql,tcp
```

Named protocols: `grpc` / `http2` (port 7233), `pgsql` / `postgresql` (port 5432), `http` (port 8080), `tcp`, `arp`. Anything else is matched against tshark's Protocol column (e.g. `ICMPv6`).

The flow diagram shows which containers communicated and how much traffic each connection carried. The sequence diagram shows every gRPC method call in order, with consecutive identical calls from the same source compressed (e.g., `PollWorkflowTaskQueue (x1,234)`).

---

## Example workflows

Five additional workflows demonstrate different Temporal features. **Nothing starts automatically** — every worker and starter requires an explicit command. Running a starter automatically brings up its worker as a dependency, so you only need one command per workflow.

### Scheduled — periodic workflows via Temporal Schedules

```bash
docker compose run --rm scheduled-starter
```

Creates a schedule that triggers `ScheduledReportWorkflow` every 30 seconds. Watch repeated runs appear in the Temporal UI under **Schedules**.

### Signals — long-running workflows with signals and queries

```bash
# 1. Start a workflow that blocks waiting for a signal
docker compose run --rm signals-starter

# 2a. Approve it (runs ProcessOrderActivity)
docker compose run --rm signals-approve

# 2b. Or reject it (runs CancelOrderActivity)
docker compose run --rm signals-reject
```

The workflow exposes a `status` query you can inspect in the Temporal UI while it is pending.

### Child workflows — parallel fan-out

```bash
docker compose run --rm child-workflows-starter
```

`DataPipelineWorkflow` spawns five child workflows in parallel, one per item. Observe the parent-child coordination traffic in Wireshark.

### Retries — activity retry with exponential backoff

```bash
docker compose run --rm retries-starter
```

`UnreliableActivity` deliberately fails on attempts 1 and 2, then succeeds on attempt 3. The retry policy uses exponential backoff (2 s → 4 s → success). Watch `RespondActivityTaskFailed` RPCs in Wireshark followed by a final `RespondActivityTaskCompleted`.

### Saga — distributed transactions with compensation

```bash
# Happy path: flight + hotel + car all succeed
docker compose run --rm saga-starter

# Failure path: hotel booking fails → flight is automatically cancelled
docker compose run --rm saga-fail-starter
```

`BookingWorkflow` books three resources in sequence, registering a compensation action after each. If any step fails, completed bookings are rolled back in reverse order.

## Tear down

```bash
docker compose down
```

All data is ephemeral — PostgreSQL uses a tmpfs mount and is wiped on shutdown.

## Project layout

```
docker-compose.yml          # All containers
temporal-config/            # Temporal server dynamic config
examples/
  hello-world/              # Simple hello-world workflow
  scheduled/                # Temporal Schedules (periodic trigger)
  signals/                  # Signals + queries (approval workflow)
  child-workflows/          # Parallel child workflow fan-out
  retries/                  # Activity retries with exponential backoff
  saga/                     # Saga pattern with compensation
  Each example follows the same layout:
    workflow/workflow.go    # Workflow + Activity definitions
    worker/                 # Worker (manual: docker compose run --rm)
    starter/                # Starter (manual: docker compose run --rm)
tshark/Dockerfile           # Minimal tshark capture image
captures/                   # Rolling pcap files written by tshark
  reports/                  # Generated reports (gitignored)
wireshark/hosts             # Static IP → container name mappings
scripts/
  analyze.sh                # Entry point — checks deps, calls analyze.py
  analyze.py                # Extracts packets + gRPC calls, generates HTML + Markdown
```
