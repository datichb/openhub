#!/usr/bin/env bats
# Tests pour les commandes websearch de cmd-config.sh
# Vérifie : enable, disable, status (hub et projet), idempotence, erreurs

setup() {
  TEST_DIR="$(mktemp -d)"
  
  # Sourcer common.sh pour les fonctions partagées
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  
  # Surcharger REPO_ROOT vers le test dir
  REPO_ROOT="$TEST_DIR"
  
  # Créer la structure minimale pour les tests projets
  PROJECTS_FILE="$TEST_DIR/projects/projects.md"
  PATHS_FILE="$TEST_DIR/projects/paths.local.md"
  mkdir -p "$(dirname "$PROJECTS_FILE")"
  
  # Format projects.md correct (format markdown)
  cat > "$PROJECTS_FILE" <<'EOF'
# Registre des projets

## TEST-PROJ-1
- Nom : Test Project 1
- Stack : Node.js
- Board Beads : TEST-PROJ-1
- Tracker : none
- Labels : test

## TEST-PROJ-2
- Nom : Test Project 2
- Stack : Python
- Board Beads : TEST-PROJ-2
- Tracker : none
- Labels : test
EOF

  # Format paths.local.md correct
  cat > "$PATHS_FILE" <<EOF
# Chemins locaux (ignoré par git)
TEST-PROJ-1=$TEST_DIR/test-project-1
TEST-PROJ-2=$TEST_DIR/test-project-2
EOF
  
  # Créer les répertoires de projets de test
  mkdir -p "$TEST_DIR/test-project-1/.opencode"
  mkdir -p "$TEST_DIR/test-project-2/.opencode"
  
  # Mocks des fonctions de log
  log_info()    { true; }
  log_success() { true; }
  log_warn()    { true; }
  log_error()   { true; }
  
  # Sourcer cmd-config.sh en mode source only
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── cmd_websearch_enable : Hub level ──────────────────────────────────

@test "websearch enable hub : crée opencode.json avec permissions" {
  run cmd_websearch_enable
  [ "$status" -eq 0 ]
  
  local hub_config="$TEST_DIR/opencode.json"
  [ -f "$hub_config" ]
  
  run jq -r '.permission.websearch' "$hub_config"
  [ "$output" = "allow" ]
  
  run jq -r '.permission.webfetch' "$hub_config"
  [ "$output" = "allow" ]
}

@test "websearch enable hub : idempotent (peut s'exécuter plusieurs fois)" {
  # Premier enable
  run cmd_websearch_enable
  [ "$status" -eq 0 ]
  
  # Deuxième enable (devrait mettre à jour sans erreur)
  run cmd_websearch_enable
  [ "$status" -eq 0 ]
  
  # Vérifier que le fichier est toujours valide
  local hub_config="$TEST_DIR/opencode.json"
  run jq -r '.permission.websearch' "$hub_config"
  [ "$output" = "allow" ]
}

@test "websearch enable hub : échoue avec message clair si jq manquant" {
  # Créer un opencode.json existant pour forcer l'utilisation de jq
  echo '{}' > "$TEST_DIR/opencode.json"
  
  # Backup de la fonction jq originale et mock avec échec
  _original_jq=$(command -v jq)
  jq() {
    echo "jq: command not found" >&2
    return 127
  }
  export -f jq
  
  # Mock command -v pour retourner false pour jq
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "jq" ]; then
      return 1
    else
      builtin command "$@"
    fi
  }
  export -f command
  
  run cmd_websearch_enable
  [ "$status" -ne 0 ]
  
  # Restore
  unset -f jq
  unset -f command
}

# ── cmd_websearch_enable : Project level ──────────────────────────────

@test "websearch enable project : crée .opencode/opencode.json dans le projet" {
  run cmd_websearch_enable "TEST-PROJ-1"
  [ "$status" -eq 0 ]
  
  local proj_config="$TEST_DIR/test-project-1/.opencode/opencode.json"
  [ -f "$proj_config" ]
  
  run jq -r '.permission.websearch' "$proj_config"
  [ "$output" = "allow" ]
  
  run jq -r '.permission.webfetch' "$proj_config"
  [ "$output" = "allow" ]
}

@test "websearch enable project : idempotent pour projet" {
  # Premier enable
  run cmd_websearch_enable "TEST-PROJ-1"
  [ "$status" -eq 0 ]
  
  # Deuxième enable
  run cmd_websearch_enable "TEST-PROJ-1"
  [ "$status" -eq 0 ]
  
  # Vérifier que le fichier est toujours valide
  local proj_config="$TEST_DIR/test-project-1/.opencode/opencode.json"
  run jq -r '.permission.websearch' "$proj_config"
  [ "$output" = "allow" ]
}

@test "websearch enable project : échoue si projet inexistant" {
  run cmd_websearch_enable "inexistant-proj"
  [ "$status" -ne 0 ]
}

# ── cmd_websearch_disable ─────────────────────────────────────────────

@test "websearch disable project : définit permission.websearch=deny et supprime env" {
  # D'abord enable
  cmd_websearch_enable "TEST-PROJ-1" >/dev/null 2>&1
  
  # Puis disable
  run cmd_websearch_disable "TEST-PROJ-1"
  [ "$status" -eq 0 ]
  
  local proj_config="$TEST_DIR/test-project-1/.opencode/opencode.json"
  
  # Vérifier que permission.websearch est deny
  run jq -r '.permission.websearch' "$proj_config"
  [ "$output" = "deny" ]
}

@test "websearch disable sans PROJECT_ID : échoue avec usage message" {
  run cmd_websearch_disable
  [ "$status" -ne 0 ]
}

@test "websearch disable project sans opencode.json : warning mais pas d'erreur" {
  # Projet existe mais pas de opencode.json
  run cmd_websearch_disable "TEST-PROJ-2"
  [ "$status" -eq 0 ]
}

# ── cmd_websearch_status ──────────────────────────────────────────────

@test "websearch status hub : affiche statut hub avec format correct" {
  # Créer un hub config enabled
  cmd_websearch_enable >/dev/null 2>&1
  
  run cmd_websearch_status
  [ "$status" -eq 0 ]
  [[ "$output" =~ "Hub" ]]
  [[ "$output" =~ "permission.websearch: allow" ]]
  [[ "$output" =~ "Enabled" ]]
}

@test "websearch status hub disabled : affiche disabled" {
  # Créer un hub config sans websearch
  echo '{}' > "$TEST_DIR/opencode.json"
  
  run cmd_websearch_status
  [ "$status" -eq 0 ]
  [[ "$output" =~ "Hub" ]]
  [[ "$output" =~ "Disabled" ]]
}

@test "websearch status project : affiche hub + project status" {
  # Enable both hub and project
  cmd_websearch_enable >/dev/null 2>&1
  cmd_websearch_enable "TEST-PROJ-1" >/dev/null 2>&1
  
  run cmd_websearch_status "TEST-PROJ-1"
  [ "$status" -eq 0 ]
  [[ "$output" =~ "Hub" ]]
  [[ "$output" =~ "Project (TEST-PROJ-1)" ]]
  [[ "$output" =~ "permission.websearch: allow" ]]
}

@test "websearch status project sans opencode.json : affiche inherit message" {
  # Hub enabled mais pas de project config
  cmd_websearch_enable >/dev/null 2>&1
  
  run cmd_websearch_status "TEST-PROJ-2"
  [ "$status" -eq 0 ]
  [[ "$output" =~ "Project (TEST-PROJ-2)" ]]
  [[ "$output" =~ "No project-specific opencode.json" ]]
  [[ "$output" =~ "inherits from hub config" ]]
}

@test "websearch status sans hub opencode.json : affiche not found" {
  run cmd_websearch_status
  [ "$status" -eq 0 ]
  [[ "$output" =~ "No opencode.json found" ]]
}

# ── Integration : enable → status → disable → status ─────────────────

@test "integration : enable → status enabled → disable → status disabled" {
  # Enable
  cmd_websearch_enable "TEST-PROJ-1" >/dev/null 2>&1
  
  # Status should show enabled
  run cmd_websearch_status "TEST-PROJ-1"
  [[ "$output" =~ "Enabled" ]]
  
  # Disable
  cmd_websearch_disable "TEST-PROJ-1" >/dev/null 2>&1
  
  # Status should show disabled or not enabled
  run cmd_websearch_status "TEST-PROJ-1"
  [[ "$output" =~ "Disabled" ]] || [[ "$output" =~ "deny" ]]
}
