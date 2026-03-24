#!/usr/bin/env bash
# analyze.sh — wrapper that checks deps then runs analyze.py
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Dependency checks ──────────────────────────────────────────────────────────

if ! command -v tshark &>/dev/null; then
    echo "Error: tshark not found."
    echo "  Install with: brew install wireshark"
    echo "  (Choose 'Install command-line tools' when prompted)"
    exit 1
fi

if ! command -v python3 &>/dev/null; then
    echo "Error: python3 not found."
    echo "  Install with: brew install python"
    exit 1
fi

exec python3 "$SCRIPT_DIR/analyze.py" "$@"
