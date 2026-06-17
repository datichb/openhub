#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

# Lit la version depuis hub.json.example (tracké git, source de vérité)
HUB_VERSION=""
if [ -f "$HUB_DIR/config/hub.json.example" ]; then
  if command -v jq &>/dev/null; then
    HUB_VERSION=$(jq -r '.version // empty' "$HUB_DIR/config/hub.json.example" 2>/dev/null)
  else
    HUB_VERSION=$(grep '"version"' "$HUB_DIR/config/hub.json.example" | sed 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  fi
fi

HUB_VERSION="${HUB_VERSION:-$(t version.unknown)}"

echo -e "${BOLD}openhub${RESET} v${HUB_VERSION}"
