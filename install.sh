#!/usr/bin/env bash
# mnemo installer — builds the binary, then launches the interactive setup wizard.
# No environment variables to export; the TUI collects everything.
set -euo pipefail

REPO="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="${MNEMO_BIN_DIR:-$HOME/.local/bin}"
BIN="$BIN_DIR/mnemo"

command -v go >/dev/null 2>&1 || { echo "error: Go is required to build mnemo (https://go.dev/dl)"; exit 1; }

echo "Building mnemo…"
mkdir -p "$BIN_DIR"
( cd "$REPO" && go build -o "$BIN" ./cmd/mnemo )
echo "✓ installed $BIN"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "! note: $BIN_DIR is not on your PATH — add: export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

echo
# Launch the interactive setup wizard (vault, MCP scope, skills, graph).
"$BIN" setup --plugin-src "$REPO/plugin" "$@"
