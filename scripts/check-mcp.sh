#!/bin/bash
# Vérifie si les MCP servers ont besoin d'être buildés.
# Retourne le texte "need to be built" si au moins un serveur est absent ou périmé.
# Usage : bash check-mcp.sh
# Exit 0 si tout est à jour, exit 1 si au moins un serveur est à rebuilder.

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HUB_DIR="$(cd "$SCRIPTS_DIR/.." && pwd)"
SERVERS_DIR="$HUB_DIR/servers"

needs_build=()

for server_dir in "$SERVERS_DIR"/*/; do
  [ ! -d "$server_dir" ] && continue
  server_name=$(basename "$server_dir")

  # Ignorer les dossiers sans package.json (pas un vrai serveur)
  [ ! -f "$server_dir/package.json" ] && continue

  dist_dir="$server_dir/dist"

  # Cas 1 : dist/ absent
  if [ ! -d "$dist_dir" ] || [ -z "$(ls -A "$dist_dir" 2>/dev/null)" ]; then
    echo "  ✗ $server_name: dist/ absent — need to be built"
    needs_build+=("$server_name")
    continue
  fi

  # Cas 2 : un fichier source plus récent que n'importe quel fichier dist
  src_dir="$server_dir/src"
  if [ -d "$src_dir" ]; then
    # Trouver le fichier src le plus récent
    newest_src=$(find "$src_dir" -type f \( -name "*.ts" -o -name "*.js" \) -newer "$dist_dir" 2>/dev/null | head -1)
    if [ -n "$newest_src" ]; then
      echo "  ✗ $server_name: sources modifiées depuis le dernier build — need to be built"
      needs_build+=("$server_name")
      continue
    fi
  fi

  echo "  ✓ $server_name: à jour"
done

if [ ${#needs_build[@]} -gt 0 ]; then
  exit 1
fi

exit 0
