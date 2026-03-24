# temporalcoms

A local Temporal cluster running as individual Docker containers, with a Go "hello world" workflow and two packet-inspection tools for observing network traffic.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Mac/Windows) or Docker + Docker Compose (Linux)

## Start everything

```bash
docker compose up --build
```

The first run downloads images and compiles the Go binaries — give it a few minutes. On subsequent runs `--build` can be omitted.

**Startup order:**
1. PostgreSQL becomes healthy
2. Schema migrations run (one-shot container exits)
3. All four Temporal services start (frontend, history, matching, worker)
4. Default namespace is created (one-shot container exits)
5. Your Go worker registers on the task queue
6. The starter fires one workflow, prints the result, and exits

## Watching the workflow run

```bash
docker compose logs -f hello-world-starter
```

You should see:
```
✓ Workflow result: Hello, World! (from Temporal activity)
```

To trigger it again:
```bash
docker compose restart hello-world-starter
docker compose logs -f hello-world-starter
```

## Web UIs

| URL | What it is |
|---|---|
| http://localhost:8080 | Temporal UI — browse workflows, task queues, namespaces |
| http://localhost:3000 | Wireshark — full packet capture GUI |
| http://localhost:3001 | ntopng — real-time traffic analytics dashboard |

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

## Using ntopng

1. Open http://localhost:3001 — no login required
2. The dashboard shows live flows between containers on the Temporal network
3. Use **Flows** or **Hosts** in the top nav to drill into specific traffic

## Tear down

```bash
docker compose down
```

All data is ephemeral — PostgreSQL uses a tmpfs mount and is wiped on shutdown.

## Project layout

```
docker-compose.yml          # All containers
temporal-config/            # Temporal server dynamic config
hello-world/
  workflow/workflow.go      # Workflow + Activity definitions
  worker/                   # Go worker (polls task queue)
  starter/                  # Go starter (fires one workflow run)
tshark/Dockerfile           # Minimal tshark capture image
captures/                   # Rolling pcap files written by tshark
wireshark/hosts             # Static IP → container name mappings
```
