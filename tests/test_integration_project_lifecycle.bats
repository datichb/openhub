#!/usr/bin/env bats
# Tests d'intégration - Lifecycle complet d'un projet
# Workflow : Init → Config → Beads init → Deploy → Status → Remove

load helpers

setup() {
  common_setup
  
  # Sourcer oc.sh principal
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  source "$SCRIPT_DIR/common.sh"
  
  # Mock log functions
  mock_log_functions
  
  # Variables pour le projet de test
  export TEST_PROJECT_ID="INTEGRATION-TEST"
  export TEST_PROJECT_PATH="$TEST_DIR/integration-project"
  
  # Créer projects.md et paths.local.md
  mkdir -p "$(dirname "$PROJECTS_FILE")"
  mkdir -p "$(dirname "$PATHS_FILE")"
  touch "$PROJECTS_FILE"
  touch "$PATHS_FILE"
}

teardown() {
  common_teardown
}

# ── Phase 1 : Init projet ───────────────────────────────────────────────────

@test "Lifecycle : init nouveau projet" {
  # Sourcer cmd-init
  source "$SCRIPT_DIR/cmd-init.sh"
  
  # Mock user input
  read() {
    case "${!#}" in
      *"Nom"*) eval "$2='Integration Test Project'" ;;
      *"Stack"*) eval "$2='TypeScript'" ;;
      *"Labels"*) eval "$2='test,integration'" ;;
      *) eval "$2=''" ;;
    esac
    return 0
  }
  export -f read
  
  mkdir -p "$TEST_PROJECT_PATH"
  cd "$TEST_PROJECT_PATH"
  
  run cmd_init "$TEST_PROJECT_ID" "$TEST_PROJECT_PATH"
  [ "$status" -eq 0 ]
  
  # Vérifier que le projet est dans projects.md
  run grep "$TEST_PROJECT_ID" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "Lifecycle : projet existe après init" {
  # Ajouter projet manuellement
  cat >> "$PROJECTS_FILE" <<EOF

## $TEST_PROJECT_ID
- Nom : Test Project
- Stack : TypeScript
- Labels : test
EOF
  
  cat >> "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  # Vérifier existence
  run project_exists "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
}

# ── Phase 2 : Configuration ─────────────────────────────────────────────────

@test "Lifecycle : configurer API key" {
  # Préparer projet
  cat >> "$PROJECTS_FILE" <<EOF

## $TEST_PROJECT_ID
- Nom : Test Project
EOF
  
  cat >> "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  # Ajouter clé API dans api-keys.local.md
  mkdir -p "$(dirname "$API_KEYS_FILE")"
  cat >> "$API_KEYS_FILE" <<EOF

## $TEST_PROJECT_ID
provider = anthropic
model = claude-sonnet-4
api_key = sk-test-key-123
EOF
  
  # Vérifier lecture
  source "$LIB_DIR/api-keys.sh"
  run get_project_api_provider "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic" ]
}

@test "Lifecycle : configurer agents du projet" {
  # Projet existe
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Test Project
- Stack : TypeScript
EOF
  
  # Ajouter agents
  source "$LIB_DIR/agent-picker.sh"
  _set_project_agents "$TEST_PROJECT_ID" "orchestrator,developer-backend"
  
  # Vérifier
  run grep "Agents : orchestrator,developer-backend" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

# ── Phase 3 : Beads init ────────────────────────────────────────────────────

@test "Lifecycle : beads init crée .beads/" {
  mkdir -p "$TEST_PROJECT_PATH"
  cd "$TEST_PROJECT_PATH"
  
  # Mock bd command
  export PATH="$TEST_DIR/bin:$PATH"
  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/bd" <<'EOF'
#!/bin/bash
case "$1" in
  init)
    mkdir -p .beads
    echo "Beads initialized"
    ;;
  status)
    echo "Beads OK"
    ;;
esac
exit 0
EOF
  chmod +x "$TEST_DIR/bin/bd"
  
  # Init beads
  run bd init
  [ "$status" -eq 0 ]
  [ -d ".beads" ]
}

@test "Lifecycle : beads sync récupère tickets" {
  mkdir -p "$TEST_PROJECT_PATH/.beads"
  cd "$TEST_PROJECT_PATH"
  
  # Mock bd command
  export PATH="$TEST_DIR/bin:$PATH"
  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/bd" <<'EOF'
#!/bin/bash
case "$1" in
  sync)
    echo "Synced 5 tickets"
    exit 0
    ;;
  list)
    echo '[{"id":"bd-1","title":"Test"}]'
    exit 0
    ;;
esac
EOF
  chmod +x "$TEST_DIR/bin/bd"
  
  run bd sync
  [ "$status" -eq 0 ]
  [[ "$output" == *"Synced"* ]]
}

# ── Phase 4 : Deploy ────────────────────────────────────────────────────────

@test "Lifecycle : deploy crée .opencode/" {
  mkdir -p "$TEST_PROJECT_PATH"
  
  # Mock deploy
  source "$SCRIPT_DIR/cmd-deploy.sh"
  
  # Mock functions
  _require_project_config() { return 0; }
  _deploy_agents() { mkdir -p "$1/.opencode/agents"; }
  _deploy_prompts() { return 0; }
  _deploy_env() { return 0; }
  export -f _require_project_config _deploy_agents _deploy_prompts _deploy_env
  
  run _deploy_agents "$TEST_PROJECT_PATH"
  [ "$status" -eq 0 ]
  [ -d "$TEST_PROJECT_PATH/.opencode/agents" ]
}

@test "Lifecycle : deploy copie agents natifs" {
  mkdir -p "$TEST_PROJECT_PATH"
  mkdir -p "$HUB_DIR/agents/orchestrator"
  
  # Créer agent de test
  cat > "$HUB_DIR/agents/orchestrator/orchestrator.md" <<'EOF'
---
id: orchestrator
---
# Orchestrator agent
EOF
  
  # Mock deploy function
  _deploy_agents() {
    local target="$1"
    mkdir -p "$target/.opencode/agents"
    cp -r "$HUB_DIR/agents/orchestrator" "$target/.opencode/agents/" 2>/dev/null || true
  }
  export -f _deploy_agents
  
  _deploy_agents "$TEST_PROJECT_PATH"
  
  [ -d "$TEST_PROJECT_PATH/.opencode/agents/orchestrator" ]
}

# ── Phase 5 : Status ────────────────────────────────────────────────────────

@test "Lifecycle : status affiche info projet" {
  # Préparer projet complet
  mkdir -p "$TEST_PROJECT_PATH/.beads"
  mkdir -p "$TEST_PROJECT_PATH/.opencode/agents"
  
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Test Project
- Stack : TypeScript
- Agents : orchestrator
EOF
  
  cat > "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  # Vérifier qu'on peut lire les infos
  run get_project_path "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  [ "$output" = "$TEST_PROJECT_PATH" ]
  
  run get_project_agents "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  [[ "$output" == *"orchestrator"* ]]
}

# ── Phase 6 : Remove ────────────────────────────────────────────────────────

@test "Lifecycle : remove supprime de projects.md" {
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Test Project
EOF
  
  cat > "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  # Sourcer cmd-remove
  source "$SCRIPT_DIR/cmd-remove.sh"
  
  # Mock confirmation
  read() {
    eval "$2='Y'"
    return 0
  }
  export -f read
  
  run cmd_remove "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  
  # Vérifier suppression
  run grep "$TEST_PROJECT_ID" "$PROJECTS_FILE"
  [ "$status" -ne 0 ]
}

@test "Lifecycle : remove --clean supprime aussi .opencode/" {
  mkdir -p "$TEST_PROJECT_PATH/.opencode/agents"
  touch "$TEST_PROJECT_PATH/.opencode/agents/test.md"
  
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Test Project
EOF
  
  cat > "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  source "$SCRIPT_DIR/cmd-remove.sh"
  
  # Mock confirmation
  read() {
    eval "$2='Y'"
    return 0
  }
  export -f read
  
  run cmd_remove "$TEST_PROJECT_ID" --clean
  [ "$status" -eq 0 ]
  
  # Vérifier cleanup
  [ ! -d "$TEST_PROJECT_PATH/.opencode" ]
}

# ── Workflow complet ────────────────────────────────────────────────────────

@test "Intégration : workflow complet end-to-end" {
  # 1. Init projet
  mkdir -p "$TEST_PROJECT_PATH"
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Integration Test
- Stack : TypeScript
- Labels : test
EOF
  
  cat > "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
  
  # Vérifier projet créé
  run project_exists "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  
  # 2. Config API
  mkdir -p "$(dirname "$API_KEYS_FILE")"
  cat > "$API_KEYS_FILE" <<EOF
## $TEST_PROJECT_ID
provider = anthropic
model = claude-sonnet-4
api_key = sk-test-123
EOF
  
  # 3. Beads init
  mkdir -p "$TEST_PROJECT_PATH/.beads"
  [ -d "$TEST_PROJECT_PATH/.beads" ]
  
  # 4. Deploy
  mkdir -p "$TEST_PROJECT_PATH/.opencode/agents"
  [ -d "$TEST_PROJECT_PATH/.opencode" ]
  
  # 5. Status
  run get_project_path "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  [ "$output" = "$TEST_PROJECT_PATH" ]
  
  # 6. Remove
  source "$SCRIPT_DIR/cmd-remove.sh"
  read() { eval "$2='Y'"; return 0; }
  export -f read
  
  run cmd_remove "$TEST_PROJECT_ID" --clean
  [ "$status" -eq 0 ]
  
  # Vérifier nettoyage complet
  run project_exists "$TEST_PROJECT_ID"
  [ "$status" -ne 0 ]
}

@test "Intégration : multi-projets isolation" {
  # Créer 2 projets
  local proj1="TEST-PROJ-1"
  local proj2="TEST-PROJ-2"
  local path1="$TEST_DIR/proj1"
  local path2="$TEST_DIR/proj2"
  
  mkdir -p "$path1" "$path2"
  
  cat > "$PROJECTS_FILE" <<EOF
## $proj1
- Nom : Project 1
- Stack : TypeScript

## $proj2
- Nom : Project 2
- Stack : Python
EOF
  
  cat > "$PATHS_FILE" <<EOF
$proj1|$path1
$proj2|$path2
EOF
  
  # Vérifier isolation
  run get_project_path "$proj1"
  [ "$output" = "$path1" ]
  
  run get_project_path "$proj2"
  [ "$output" = "$path2" ]
  
  # Modifier proj1 ne doit pas affecter proj2
  source "$LIB_DIR/agent-picker.sh"
  _set_project_agents "$proj1" "orchestrator"
  
  run get_project_agents "$proj1"
  [[ "$output" == *"orchestrator"* ]]
  
  run get_project_agents "$proj2"
  [[ "$output" != *"orchestrator"* ]]
}

@test "Intégration : renommage projet préserve cohérence" {
  # Créer projet initial
  local old_id="OLD-PROJECT"
  local new_id="NEW-PROJECT"
  local project_path="$TEST_DIR/project"
  
  mkdir -p "$project_path"
  
  cat > "$PROJECTS_FILE" <<EOF
## $old_id
- Nom : Old Name
- Stack : TypeScript
EOF
  
  cat > "$PATHS_FILE" <<EOF
$old_id|$project_path
EOF
  
  mkdir -p "$(dirname "$API_KEYS_FILE")"
  cat > "$API_KEYS_FILE" <<EOF
## $old_id
provider = anthropic
EOF
  
  # Sourcer cmd-project pour rename
  source "$SCRIPT_DIR/cmd-project.sh"
  
  # Mock confirmation
  read() { eval "$2='Y'"; return 0; }
  export -f read
  
  run cmd_rename "$old_id" "$new_id"
  [ "$status" -eq 0 ]
  
  # Vérifier cohérence
  run grep "$new_id" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
  
  run grep "$new_id" "$PATHS_FILE"
  [ "$status" -eq 0 ]
  
  run grep "$old_id" "$PROJECTS_FILE"
  [ "$status" -ne 0 ]
}
