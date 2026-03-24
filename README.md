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

1. Open http://localhost:3000 in your browser
2. Double-click the **eth0** interface to start a capture
3. To see *all* inter-container traffic (not just packets to/from Wireshark itself), go to **Capture → Options**, select eth0, and tick **Enable promiscuous mode**
4. Use display filters as normal, e.g. `tcp.port == 7233` to isolate Temporal gRPC traffic

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
docker-compose.yml          # All 12 containers
temporal-config/            # Temporal server config + dynamic config
scripts/                    # One-shot setup scripts (schema, namespace)
hello-world/
  workflow/workflow.go      # Workflow + Activity definitions
  worker/                   # Go worker (polls task queue)
  starter/                  # Go starter (fires one workflow run)
```
