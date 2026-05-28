#!/bin/bash
# Build tous les MCP servers ou un seul si spécifié
# Usage: 
#   bash scripts/build-mcp.sh          # Build tous
#   bash scripts/build-mcp.sh figma-mcp    # Build figma-mcp uniquement

set -e

HUB_PATH="${HUB_PATH:-$(cd "$(dirname "$0")/.." && pwd)}"
SERVER_NAME="${1:-}"

build_server() {
  local server_dir=$1
  local server_name=$(basename "$server_dir")
  
  if [ ! -f "$server_dir/package.json" ]; then
    echo "⚠️  No package.json found in $server_name, skipping"
    return 0
  fi
  
  echo "Building MCP Server: $server_name"
  cd "$server_dir"
  
  # Installer les dépendances si node_modules absent
  if [ ! -d "node_modules" ]; then
    echo "  → Installing dependencies..."
    npm install --silent
  fi
  
  # Compiler
  echo "  → Compiling TypeScript..."
  npm run build
  
  if [ $? -eq 0 ]; then
    echo "✓ $server_name built successfully"
  else
    echo "✗ $server_name build failed"
    exit 1
  fi
}

if [ -n "$SERVER_NAME" ]; then
  # Build un seul serveur
  if [ -d "$HUB_PATH/servers/$SERVER_NAME" ]; then
    build_server "$HUB_PATH/servers/$SERVER_NAME"
  else
    echo "✗ Server '$SERVER_NAME' not found in servers/"
    exit 1
  fi
else
  # Build tous les serveurs
  echo "Building all MCP servers..."
  echo ""
  
  found=0
  for server_dir in "$HUB_PATH/servers/"*/; do
    if [ -d "$server_dir" ] && [ -f "$server_dir/package.json" ]; then
      build_server "$server_dir"
      echo ""
      found=1
    fi
  done
  
  if [ $found -eq 0 ]; then
    echo "No MCP servers found in servers/"
    exit 0
  fi
fi

echo "✓ All builds completed"
