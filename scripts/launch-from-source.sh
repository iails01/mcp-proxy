#!/bin/zsh

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(cd -- "$SCRIPT_DIR/.." && pwd)
CONFIG_PATH=${1:-"$HOME/.config/mcp-proxy/config.json"}
GO_BIN=${GO_BIN:-/opt/homebrew/bin/go}
BUILD_OUTPUT="$REPO_ROOT/build/mcp-proxy"

export PATH="/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"

if [[ ! -x "$GO_BIN" ]]; then
  echo "go executable not found at $GO_BIN" >&2
  exit 1
fi

mkdir -p "$REPO_ROOT/build"

BUILD_VERSION="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo local)@$(date +%s)"

cd "$REPO_ROOT"
echo "[launch-from-source] building mcp-proxy from $REPO_ROOT with version $BUILD_VERSION" >&2
rm -f "$BUILD_OUTPUT"
CGO_ENABLED=0 "$GO_BIN" build -ldflags "-X main.BuildVersion=$BUILD_VERSION" -o "$BUILD_OUTPUT" .
echo "[launch-from-source] build complete: $BUILD_OUTPUT" >&2

exec "$BUILD_OUTPUT" --config "$CONFIG_PATH"
