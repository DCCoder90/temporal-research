# Planned Improvements

## 1. Rewrite `analyze.py` as a cross-platform Go program

### Goal

Replace `scripts/analyze.py` + `scripts/analyze.sh` with a single Go binary that works as both a terminal CLI and a desktop GUI — without the user needing to know or care which mode they're in.

### User experience

| How launched | Behaviour |
|---|---|
| Double-click the binary (Mac or Windows) | GUI window opens |
| `temporal-analyze` from terminal (no args) | GUI window opens |
| `temporal-analyze capture.pcap --only grpc` | Terminal output, no window |
| `temporal-analyze --help` | Prints help to terminal, no window |

### Architecture

```
CORE     pure Go packages — analysis logic, knows nothing about UI
  └─ CLI   parses flags, calls CORE, prints results to terminal
      └─ GUI  Wails window, calls same CORE, renders results inline
```

All three layers live in a single binary. Routing happens at startup:

```go
func main() {
    if len(os.Args) > 1 {
        cli.Run()  // flags → core → print to terminal
    } else {
        gui.Run()  // Wails window → same core → render in app
    }
}
```

### Internal package layout

```
analyze/
  cmd/
    temporal-analyze/
      main.go              # startup router (CLI vs GUI)
  internal/
    tshark/                # tshark subprocess calls, field extraction
    filter/                # --only, --exclude, --no-interservice logic
    diagram/               # Mermaid diagram string builders
    report/                # HTML + Markdown generation
    config/                # port labels, IP maps, interservice hosts
  gui/
    app.go                 # Wails app setup
    frontend/              # HTML/CSS/JS — reuses existing Mermaid diagrams
```

### GUI framework

[Wails](https://wails.io) — Go backend with an embedded native webview (WebKit on Mac, WebView2 on Windows). The existing HTML + Mermaid output maps directly onto this: the GUI renders the same diagrams the CLI writes to disk, just displayed in a native app window instead. Adds drag-and-drop for pcap files and a filter panel without changing any core logic.

### Windows console behaviour

Windows distinguishes between "console applications" (always spawn a terminal window on double-click) and "Windows applications" (no terminal window). To support both modes from one binary:

- Build with `-H windowsgui` (suppresses the terminal window on double-click)
- In CLI mode, call `AttachConsole(ATTACH_PARENT_PROCESS)` at startup to attach to the parent terminal session

This is a standard pattern — running the binary from PowerShell or cmd.exe behaves exactly like any other CLI tool. The user never notices the difference.

### What stays the same

- `docker-compose.yml` and all services — unchanged
- All Go workflow examples — unchanged
- `scripts/ANALYZE.md` — updated to document the new install method
- `scripts/analyze.sh` — can be retired once the binary ships; kept temporarily for backward compatibility

### Distribution

GitHub Actions builds on tag push and uploads artifacts to GitHub Releases.

**CLI** (cross-compiled from a single Linux runner):
```
temporal-analyze-macos-arm64
temporal-analyze-macos-x86_64
temporal-analyze-windows.exe
temporal-analyze-linux-amd64
```

**GUI** (requires platform runners — Wails uses native webview):
```
temporal-analyze-ui-macos.app.zip   # built on macos-latest
temporal-analyze-ui-windows.exe     # built on windows-latest
```

The CLI and GUI are the same binary built with different Wails flags — no separate codebase to maintain.

### Install

```bash
# Mac
curl -L https://github.com/you/temporalcoms/releases/latest/download/temporal-analyze-macos-arm64 \
  -o /usr/local/bin/temporal-analyze && chmod +x /usr/local/bin/temporal-analyze

# Windows (PowerShell)
irm https://github.com/you/temporalcoms/releases/latest/download/temporal-analyze-windows.exe \
  -OutFile temporal-analyze.exe
```

### Benefits over current Python script

| | Python (`analyze.py`) | Go binary |
|---|---|---|
| Requires Python | Yes | No |
| `pyyaml` optional dep | Yes | Bundled |
| `analyze.sh` wrapper needed | Yes | No |
| Startup time | ~0.5–1s | Instant |
| Binary size | N/A (script) | ~8–12 MB |
| GUI support | No | Yes (Wails) |
| Cross-platform install | Manual | Single download |
