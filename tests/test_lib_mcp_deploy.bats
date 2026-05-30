#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/mcp-deploy.sh
# Fonctions testées : check_and_build_mcp, deploy_mcp_servers, configure_mcp_in_project

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh pour avoir les variables
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  source "$SCRIPT_DIR/common.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/mcp-deploy.sh"
  
  # Mock des fonctions log
  mock_log_functions
  
  # Créer un faux hub avec servers
  export TEST_HUB_DIR="$TEST_DIR/hub"
  export HUB_DIR="$TEST_HUB_DIR"
  mkdir -p "$TEST_HUB_DIR/servers/figma-mcp"
  mkdir -p "$TEST_HUB_DIR/scripts"
}

teardown() {
  common_teardown
}

# ── check_and_build_mcp ─────────────────────────────────────────────────────

@test "check_and_build_mcp : retourne 0 si MCP à jour" {
  # Mock check-mcp.sh qui dit que tout est OK
  cat > "$HUB_DIR/scripts/check-mcp.sh" <<'EOF'
#!/bin/bash
echo "All MCP servers are up to date"
exit 0
EOF
  chmod +x "$HUB_DIR/scripts/check-mcp.sh"
  
  run check_and_build_mcp
  [ "$status" -eq 0 ]
}

@test "check_and_build_mcp : propose build si MCP obsolète" {
  # Mock check-mcp.sh qui dit qu'il faut builder
  cat > "$HUB_DIR/scripts/check-mcp.sh" <<'EOF'
#!/bin/bash
echo "Some servers need to be built"
exit 0
EOF
  chmod +x "$HUB_DIR/scripts/check-mcp.sh"
  
  # Mock build-mcp.sh
  cat > "$HUB_DIR/scripts/build-mcp.sh" <<'EOF'
#!/bin/bash
echo "Building MCP servers..."
exit 0
EOF
  chmod +x "$HUB_DIR/scripts/build-mcp.sh"
  
  # Mock _prompt pour accepter automatiquement
  _prompt() {
    eval "$1='Y'"
  }
  export -f _prompt
  
  run check_and_build_mcp
  [ "$status" -eq 0 ]
}

@test "check_and_build_mcp : skip build si utilisateur refuse" {
  # Mock check-mcp.sh
  cat > "$HUB_DIR/scripts/check-mcp.sh" <<'EOF'
#!/bin/bash
echo "Some servers need to be built"
exit 0
EOF
  chmod +x "$HUB_DIR/scripts/check-mcp.sh"
  
  # Mock _prompt pour refuser
  _prompt() {
    eval "$1='N'"
  }
  export -f _prompt
  
  run check_and_build_mcp
  [ "$status" -ne 0 ]
}

# ── deploy_mcp_servers ──────────────────────────────────────────────────────

@test "deploy_mcp_servers : crée dossier .opencode/servers" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  deploy_mcp_servers "$deploy_dir"
  
  [ -d "$deploy_dir/.opencode/servers" ]
}

@test "deploy_mcp_servers : copie dist et package.json" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Créer un serveur de test avec dist
  mkdir -p "$HUB_DIR/servers/test-server/dist"
  echo "console.log('test');" > "$HUB_DIR/servers/test-server/dist/index.js"
  cat > "$HUB_DIR/servers/test-server/package.json" <<'EOF'
{
  "name": "test-server",
  "version": "1.0.0"
}
EOF
  
  # Mock npm install
  npm() {
    return 0
  }
  export -f npm
  
  deploy_mcp_servers "$deploy_dir"
  
  [ -f "$deploy_dir/.opencode/servers/test-server/dist/index.js" ]
  [ -f "$deploy_dir/.opencode/servers/test-server/package.json" ]
}

@test "deploy_mcp_servers : skip serveur non buildé" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Créer un serveur sans dist
  mkdir -p "$HUB_DIR/servers/unbuild-server"
  cat > "$HUB_DIR/servers/unbuild-server/package.json" <<'EOF'
{
  "name": "unbuild-server",
  "version": "1.0.0"
}
EOF
  
  deploy_mcp_servers "$deploy_dir"
  
  [ ! -d "$deploy_dir/.opencode/servers/unbuild-server" ]
}

@test "deploy_mcp_servers : déploie plusieurs serveurs" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Créer 2 serveurs
  for srv in server1 server2; do
    mkdir -p "$HUB_DIR/servers/$srv/dist"
    echo "console.log('$srv');" > "$HUB_DIR/servers/$srv/dist/index.js"
    cat > "$HUB_DIR/servers/$srv/package.json" <<EOF
{
  "name": "$srv",
  "version": "1.0.0"
}
EOF
  done
  
  # Mock npm
  npm() {
    return 0
  }
  export -f npm
  
  deploy_mcp_servers "$deploy_dir"
  
  [ -d "$deploy_dir/.opencode/servers/server1" ]
  [ -d "$deploy_dir/.opencode/servers/server2" ]
}

@test "deploy_mcp_servers : gère erreur copie dist" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Créer serveur mais dist en lecture seule
  mkdir -p "$HUB_DIR/servers/readonly-server/dist"
  echo "test" > "$HUB_DIR/servers/readonly-server/dist/index.js"
  cat > "$HUB_DIR/servers/readonly-server/package.json" <<'EOF'
{
  "name": "readonly-server"
}
EOF
  
  # Mock cp qui échoue
  cp() {
    if [[ "$*" == *"dist"* ]]; then
      return 1
    fi
    command cp "$@"
  }
  export -f cp
  
  run deploy_mcp_servers "$deploy_dir"
  [ "$status" -eq 0 ]  # Continue malgré l'erreur
}

@test "deploy_mcp_servers : retourne 0 si aucun serveur" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Pas de serveurs dans hub
  rm -rf "$HUB_DIR/servers"/*
  
  run deploy_mcp_servers "$deploy_dir"
  [ "$status" -eq 0 ]
}

# ── configure_mcp_in_project ────────────────────────────────────────────────

@test "configure_mcp_in_project : skip si opencode.json absent" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  run configure_mcp_in_project "$deploy_dir"
  [ "$status" -eq 0 ]
}

@test "configure_mcp_in_project : configure figma-mcp" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir/.opencode/servers/figma-mcp/dist"
  
  # Créer opencode.json minimal
  cat > "$deploy_dir/opencode.json" <<'EOF'
{
  "mcpServers": {}
}
EOF
  
  # Mock jq
  which jq >/dev/null 2>&1 || skip "jq non disponible"
  
  configure_mcp_in_project "$deploy_dir"
  
  # Vérifier que figma est configuré
  run jq -r '.mcpServers.figma.command' "$deploy_dir/opencode.json"
  [ "$output" = "node" ]
  
  run jq -r '.mcpServers.figma.args[0]' "$deploy_dir/opencode.json"
  [ "$output" = ".opencode/servers/figma-mcp/dist/index.js" ]
}

@test "configure_mcp_in_project : sauvegarde backup" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir/.opencode/servers/figma-mcp/dist"
  
  cat > "$deploy_dir/opencode.json" <<'EOF'
{
  "mcpServers": {}
}
EOF
  
  which jq >/dev/null 2>&1 || skip "jq non disponible"
  
  configure_mcp_in_project "$deploy_dir"
  
  # Le backup est supprimé si succès
  [ ! -f "$deploy_dir/opencode.json.bak" ]
}

@test "configure_mcp_in_project : restaure backup si erreur jq" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir/.opencode/servers/figma-mcp/dist"
  
  local original_content='{"mcpServers":{},"original":true}'
  echo "$original_content" > "$deploy_dir/opencode.json"
  
  # Mock jq qui échoue
  jq() {
    return 1
  }
  export -f jq
  
  configure_mcp_in_project "$deploy_dir"
  
  # Le fichier original devrait être restauré avec son contenu original
  [ -f "$deploy_dir/opencode.json" ]
  run cat "$deploy_dir/opencode.json"
  [[ "$output" == *'"original":true'* ]]
}

@test "configure_mcp_in_project : ne configure pas serveurs non déployés" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  cat > "$deploy_dir/opencode.json" <<'EOF'
{
  "mcpServers": {}
}
EOF
  
  # Pas de serveurs déployés
  
  which jq >/dev/null 2>&1 || skip "jq non disponible"
  
  configure_mcp_in_project "$deploy_dir"
  
  # mcpServers devrait rester vide
  run jq -r '.mcpServers | keys | length' "$deploy_dir/opencode.json"
  [ "$output" = "0" ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : workflow complet déploiement MCP" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Préparer serveur
  mkdir -p "$HUB_DIR/servers/figma-mcp/dist"
  echo "console.log('figma');" > "$HUB_DIR/servers/figma-mcp/dist/index.js"
  cat > "$HUB_DIR/servers/figma-mcp/package.json" <<'EOF'
{
  "name": "figma-mcp",
  "version": "1.0.0"
}
EOF
  
  # Créer opencode.json
  cat > "$deploy_dir/opencode.json" <<'EOF'
{
  "mcpServers": {}
}
EOF
  
  # Mock npm
  npm() {
    return 0
  }
  export -f npm
  
  which jq >/dev/null 2>&1 || skip "jq non disponible"
  
  # Déployer
  deploy_mcp_servers "$deploy_dir"
  
  # Configurer
  configure_mcp_in_project "$deploy_dir"
  
  # Vérifier déploiement
  [ -f "$deploy_dir/.opencode/servers/figma-mcp/dist/index.js" ]
  
  # Vérifier configuration
  run jq -r '.mcpServers.figma.command' "$deploy_dir/opencode.json"
  [ "$output" = "node" ]
}

@test "Intégration : déploiement sans opencode.json existant" {
  local deploy_dir="$TEST_DIR/project"
  mkdir -p "$deploy_dir"
  
  # Préparer serveur
  mkdir -p "$HUB_DIR/servers/test-mcp/dist"
  echo "test" > "$HUB_DIR/servers/test-mcp/dist/index.js"
  cat > "$HUB_DIR/servers/test-mcp/package.json" <<'EOF'
{
  "name": "test-mcp"
}
EOF
  
  # Mock npm
  npm() {
    return 0
  }
  export -f npm
  
  # Déployer sans opencode.json
  deploy_mcp_servers "$deploy_dir"
  configure_mcp_in_project "$deploy_dir"
  
  # Le serveur devrait être déployé même sans config
  [ -f "$deploy_dir/.opencode/servers/test-mcp/dist/index.js" ]
}
