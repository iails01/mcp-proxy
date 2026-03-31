#!/bin/zsh

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(cd -- "$SCRIPT_DIR/.." && pwd)
LABEL="com.luliu.mcp-proxy"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOG_PATH="$REPO_ROOT/logs/launchd.stderr.log"
MODE="${1:-kickstart}"

case "$MODE" in
  kickstart)
    launchctl kickstart -k "gui/$(id -u)/$LABEL"
    ;;
  reload)
    launchctl bootout "gui/$(id -u)" "$PLIST" 2>/dev/null || true
    launchctl bootstrap "gui/$(id -u)" "$PLIST"
    ;;
  *)
    echo "Usage: $0 [kickstart|reload]" >&2
    exit 1
    ;;
esac

sleep 3

echo "== launchctl =="
launchctl print "gui/$(id -u)/$LABEL" | sed -n '1,25p'

if [[ -f "$LOG_PATH" ]]; then
  echo
  echo "== recent log =="
  tail -n 30 "$LOG_PATH"
fi
