#!/bin/bash
# Vérifie l'état de build des MCP servers
# Retourne 0 si tous sont buildés, 1 sinon

HUB_PATH="${HUB_PATH:-$(cd "$(dirname "$0")/.." && pwd)}"
NEEDS_BUILD=0

echo "Checking MCP servers build status..."
echo ""

for server_dir in "$HUB_PATH/servers/"*/; do
  if [ ! -d "$server_dir" ]; then
    continue
  fi
  
  server_name=$(basename "$server_dir")
  
  if [ ! -f "$server_dir/package.json" ]; then
    continue
  fi
  
  if [ ! -d "$server_dir/dist" ]; then
    echo "⚠️  $server_name: NOT BUILT"
    NEEDS_BUILD=1
  else
    # Vérifier si src/ a été modifié après dist/
    if [ "$server_dir/src" -nt "$server_dir/dist" ]; then
      echo "⚠️  $server_name: OUT OF DATE"
      NEEDS_BUILD=1
    else
      echo "✓ $server_name: UP TO DATE"
    fi
  fi
done

echo ""

if [ $NEEDS_BUILD -eq 1 ]; then
  echo "Some MCP servers need to be built."
  echo "Run: bash scripts/build-mcp.sh"
fi

exit $NEEDS_BUILD
