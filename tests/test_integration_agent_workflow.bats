#!/usr/bin/env bats
# Tests d'intégration - Workflow agent complet
# Workflow : List agents → Pick agent → Build prompt → Résoudre modèle → Start

load helpers

setup() {
  common_setup
  
  # Sourcer modules nécessaires
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export CANONICAL_AGENTS_DIR="$BATS_TEST_DIRNAME/fixtures/agents"
  
  source "$SCRIPT_DIR/common.sh"
  
  # Mock log functions
  mock_log_functions
  
  # Créer projet de test
  export TEST_PROJECT_ID="AGENT-TEST"
  export TEST_PROJECT_PATH="$TEST_DIR/project"
  mkdir -p "$TEST_PROJECT_PATH"
  
  # Créer projects.md
  cat > "$PROJECTS_FILE" <<EOF
## $TEST_PROJECT_ID
- Nom : Agent Test Project
- Stack : TypeScript
- Agents : orchestrator
EOF
  
  cat > "$PATHS_FILE" <<EOF
$TEST_PROJECT_ID|$TEST_PROJECT_PATH
EOF
}

teardown() {
  common_teardown
}

# ── Phase 1 : List agents ───────────────────────────────────────────────────

@test "Agent workflow : list_available_agents liste les agents" {
  source "$LIB_DIR/agent-picker.sh"
  
  # Créer quelques agents de test
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"
  cat > "$CANONICAL_AGENTS_DIR/planning/orchestrator.md" <<'EOF'
---
id: orchestrator
description: Orchestrateur principal
---
# Orchestrator
EOF
  
  _list_all_agents_grouped
  
  [ "${#_pick_items[@]}" -gt 0 ]
  
  # Vérifier qu'orchestrator est présent
  local found=0
  for agent in "${_pick_items[@]}"; do
    if [ "$agent" = "orchestrator" ]; then
      found=1
      break
    fi
  done
  [ "$found" = "1" ]
}

@test "Agent workflow : agents sont groupés par famille" {
  source "$LIB_DIR/agent-picker.sh"
  
  # Créer agents dans différentes familles
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"
  mkdir -p "$CANONICAL_AGENTS_DIR/dev"
  
  cat > "$CANONICAL_AGENTS_DIR/planning/orchestrator.md" <<'EOF'
---
id: orchestrator
---
# Orchestrator
EOF
  
  cat > "$CANONICAL_AGENTS_DIR/dev/developer.md" <<'EOF'
---
id: developer
---
# Developer
EOF
  
  _list_all_agents_grouped
  
  # Vérifier qu'on a plusieurs familles
  local unique_families
  unique_families=$(printf '%s\n' "${_pick_families[@]}" | sort -u | wc -l | tr -d ' ')
  [ "$unique_families" -ge 2 ]
}

# ── Phase 2 : Pick agent ────────────────────────────────────────────────────

@test "Agent workflow : get_project_agents retourne agents configurés" {
  run get_project_agents "$TEST_PROJECT_ID"
  [ "$status" -eq 0 ]
  [[ "$output" == *"orchestrator"* ]]
}

@test "Agent workflow : _set_project_agents modifie la liste" {
  source "$LIB_DIR/agent-picker.sh"
  
  _set_project_agents "$TEST_PROJECT_ID" "orchestrator,developer-backend"
  
  run get_project_agents "$TEST_PROJECT_ID"
  [[ "$output" == *"orchestrator"* ]]
  [[ "$output" == *"developer-backend"* ]]
}

# ── Phase 3 : Build prompt ──────────────────────────────────────────────────

@test "Agent workflow : read_agent_frontmatter extrait métadonnées" {
  source "$LIB_DIR/prompt-builder.sh"
  
  # Créer agent avec frontmatter
  local agent_file="$TEST_DIR/test-agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test-agent
description: Test agent
mode: primary
model_floor: claude-sonnet-4
---
# Test Agent
Content here
EOF
  
  # read_agent_frontmatter expose des variables, ne retourne pas de valeur
  read_agent_frontmatter "$agent_file"
  [ "$_fm_id" = "test-agent" ]
  
  # model_floor n'est pas extrait par read_agent_frontmatter (seulement id, skills, model)
  # Utiliser extract_frontmatter_value pour d'autres champs
  run extract_frontmatter_value "$agent_file" "model_floor"
  [ "$output" = "claude-sonnet-4" ]
}

@test "Agent workflow : strip_frontmatter supprime le YAML" {
  source "$LIB_DIR/prompt-builder.sh"
  
  local agent_file="$TEST_DIR/test-agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test
---
# Content
Body text
EOF
  
  run strip_frontmatter "$agent_file"
  [ "$status" -eq 0 ]
  [[ "$output" == *"# Content"* ]]
  [[ "$output" != *"id: test"* ]]
}

@test "Agent workflow : build_agent_content compose le prompt" {
  source "$LIB_DIR/prompt-builder.sh"
  
  local agent_file="$TEST_DIR/agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test-agent
---
# Test Agent
Instructions here
EOF
  
  # Mock get_hub_version
  get_hub_version() {
    echo "2.0.0"
  }
  export -f get_hub_version
  
  run build_agent_content "$agent_file" "" "$TEST_PROJECT_PATH"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Test Agent"* ]]
}

# ── Phase 4 : Résoudre modèle ───────────────────────────────────────────────

@test "Agent workflow : resolve_agent_model cascade projet → hub" {
  source "$LIB_DIR/prompt-builder.sh"
  
  # Créer hub.json
  mkdir -p "$HUB_DIR/config"
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "opencode": {
    "model": "claude-sonnet-4"
  }
}
EOF
  
  local agent_file="$TEST_DIR/agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test
---
EOF
  
  # Sans config projet, devrait fallback sur hub
  run resolve_agent_model "$agent_file" "$TEST_PROJECT_ID" ""
  [ "$status" -eq 0 ]
  [[ "$output" == *"claude-sonnet-4"* ]]
}

@test "Agent workflow : clamp_model force modèle si model_floor" {
  source "$LIB_DIR/prompt-builder.sh"
  
  # Créer agent avec model_floor
  local agent_file="$TEST_DIR/agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test
model_floor: claude-sonnet-4
---
EOF
  
  # Créer hub.json avec modèle plus bas
  mkdir -p "$HUB_DIR/config"
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "opencode": {
    "model": "claude-haiku-4"
  }
}
EOF
  
  run resolve_agent_model "$agent_file" "$TEST_PROJECT_ID" ""
  [ "$status" -eq 0 ]
  # Devrait forcer sonnet-4 à cause du model_floor
  [[ "$output" == *"sonnet"* ]] || [[ "$output" == *"claude-sonnet-4"* ]]
}

# ── Phase 5 : Start (simulation) ────────────────────────────────────────────

@test "Agent workflow : metrics_ticket_start log démarrage" {
  source "$LIB_DIR/metrics.sh"
  
  export _METRICS_DIR="$TEST_DIR/.opencode"
  export _METRICS_FILE="$_METRICS_DIR/metrics.jsonl"
  
  metrics_ticket_start "test-ticket" "orchestrator"
  
  [ -f "$_METRICS_FILE" ]
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"ticket_start"'* ]]
  [[ "$output" == *'"agent":"orchestrator"'* ]]
}

@test "Agent workflow : session_state_init crée état session" {
  source "$LIB_DIR/session-state.sh"
  
  # session-state.sh utilise des chemins relatifs au répertoire courant
  cd "$TEST_DIR"
  
  # session_state_init requiert 2 paramètres : session_id et mode
  session_state_init "test-session" "auto"
  
  [ -f ".opencode/session-state.json" ]
  run cat ".opencode/session-state.json"
  # Vérifier avec espace après les deux-points
  [[ "$output" == *'"mode": "auto"'* ]]
}

# ── Workflow complet ────────────────────────────────────────────────────────

@test "Intégration : workflow agent end-to-end" {
  # 1. Lister les agents disponibles
  source "$LIB_DIR/agent-picker.sh"
  
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"
  cat > "$CANONICAL_AGENTS_DIR/planning/orchestrator.md" <<'EOF'
---
id: orchestrator
description: Orchestrateur principal
model_floor: claude-sonnet-4
---
# Orchestrator
Vous êtes l'orchestrateur principal
EOF
  
  _list_all_agents_grouped
  [ "${#_pick_items[@]}" -gt 0 ]
  
  # 2. Configurer agents du projet
  _set_project_agents "$TEST_PROJECT_ID" "orchestrator"
  
  run get_project_agents "$TEST_PROJECT_ID"
  [[ "$output" == *"orchestrator"* ]]
  
  # 3. Build prompt
  source "$LIB_DIR/prompt-builder.sh"
  
  get_hub_version() { echo "2.0.0"; }
  export -f get_hub_version
  
  local agent_file="$CANONICAL_AGENTS_DIR/planning/orchestrator.md"
  run build_agent_content "$agent_file" "" "$TEST_PROJECT_PATH"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Orchestrator"* ]]
  
  # 4. Résoudre modèle
  mkdir -p "$HUB_DIR/config"
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "opencode": {
    "model": "claude-sonnet-4"
  }
}
EOF
  
  run resolve_agent_model "$agent_file" "$TEST_PROJECT_ID" ""
  [ "$status" -eq 0 ]
  [[ "$output" == *"claude"* ]]
  
  # 5. Init session
  source "$LIB_DIR/session-state.sh"
  
  cd "$TEST_DIR"
  session_state_init "test-session" "auto"
  [ -f ".opencode/session-state.json" ]
  
  # 6. Log métriques
  source "$LIB_DIR/metrics.sh"
  export _METRICS_DIR="$TEST_DIR/.opencode"
  export _METRICS_FILE="$_METRICS_DIR/metrics.jsonl"
  
  metrics_ticket_start "test-ticket" "orchestrator"
  [ -f "$_METRICS_FILE" ]
}

@test "Intégration : switch entre agents" {
  source "$LIB_DIR/agent-picker.sh"
  source "$LIB_DIR/prompt-builder.sh"
  
  # Créer 2 agents
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"
  mkdir -p "$CANONICAL_AGENTS_DIR/dev"
  
  cat > "$CANONICAL_AGENTS_DIR/planning/orchestrator.md" <<'EOF'
---
id: orchestrator
---
# Orchestrator
EOF
  
  cat > "$CANONICAL_AGENTS_DIR/dev/developer.md" <<'EOF'
---
id: developer-backend
---
# Developer Backend
EOF
  
  # Configurer avec orchestrator
  _set_project_agents "$TEST_PROJECT_ID" "orchestrator"
  run get_project_agents "$TEST_PROJECT_ID"
  [[ "$output" == *"orchestrator"* ]]
  
  # Switch vers developer
  _set_project_agents "$TEST_PROJECT_ID" "developer-backend"
  run get_project_agents "$TEST_PROJECT_ID"
  [[ "$output" == *"developer-backend"* ]]
  [[ "$output" != *"orchestrator"* ]]
}

@test "Intégration : prompt avec skills injectés" {
  source "$LIB_DIR/prompt-builder.sh"
  
  # Créer agent
  local agent_file="$TEST_DIR/agent.md"
  cat > "$agent_file" <<'EOF'
---
id: test
skills: [websearch]
---
# Test Agent
Base instructions
EOF
  
  # Créer skill
  mkdir -p "$HUB_DIR/skills"
  cat > "$HUB_DIR/skills/websearch.md" <<'EOF'
# WebSearch Skill
Search capability
EOF
  
  get_hub_version() { echo "2.0.0"; }
  export -f get_hub_version
  
  export SKILLS_DIR="$HUB_DIR/skills"
  
  run build_agent_content "$agent_file" "" "" ""
  [ "$status" -eq 0 ]
  [[ "$output" == *"Test Agent"* ]]
  # Le skill devrait être injecté via strip_frontmatter
  [[ "$output" == *"WebSearch"* ]] || [[ "$output" == *"Search capability"* ]]
}
