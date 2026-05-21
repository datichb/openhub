#!/usr/bin/env bats
# Tests unitaires pour _get_agent_model (cascade 7 niveaux + clamp)
# et les fonctions sous-jacentes : resolve_agent_model, clamp_model, _get_agent_family

setup() {
  TEST_DIR="$(mktemp -d)"

  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  source "$BATS_TEST_DIRNAME/../scripts/lib/api-keys.sh"
  source "$BATS_TEST_DIRNAME/../scripts/lib/prompt-builder.sh"
  source "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"

  # Créer un agent minimal dans une famille
  mkdir -p "$TEST_DIR/agents/planning"
  cat > "$TEST_DIR/agents/planning/orchestrator-dev.md" <<'EOF'
---
id: orchestrator-dev
targets: [opencode]
skills: []
---

# Orchestrator Dev
EOF

  # Agent avec plancher model dans le frontmatter.
  # ⚠ model: DOIT apparaître avant skills: car read_agent_frontmatter fait un
  #   early exit dès que id+targets+skills sont lus (model est optionnel et
  #   n'est pas inclus dans la condition de sortie anticipée).
  cat > "$TEST_DIR/agents/planning/high-floor.md" <<'EOF'
---
id: high-floor
model: claude-opus-4
targets: [opencode]
skills: []
---

# High Floor Agent
EOF

  # Agent dans la famille developer
  mkdir -p "$TEST_DIR/agents/developer"
  cat > "$TEST_DIR/agents/developer/dev-backend.md" <<'EOF'
---
id: dev-backend
targets: [opencode]
skills: []
---

# Dev Backend
EOF

  # Mocker get_hub_version
  get_hub_version() { echo "test"; }
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── _get_agent_family ─────────────────────────────────────────────────────────

@test "_get_agent_family déduit la famille depuis le chemin" {
  run _get_agent_family "$TEST_DIR/agents/planning/orchestrator-dev.md"
  [ "$status" -eq 0 ]
  [ "$output" = "planning" ]
}

@test "_get_agent_family déduit 'developer' pour agents/developer/xxx.md" {
  run _get_agent_family "$TEST_DIR/agents/developer/dev-backend.md"
  [ "$status" -eq 0 ]
  [ "$output" = "developer" ]
}

# ── clamp_model ───────────────────────────────────────────────────────────────

@test "clamp_model retourne le modèle résolu quand supérieur au plancher" {
  run clamp_model "claude-opus-4" "claude-sonnet-4-5" "test-agent"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "clamp_model retourne le plancher quand résolu est inférieur" {
  local result
  result=$(clamp_model "claude-haiku-4-5" "claude-sonnet-4-5" "test-agent" 2>/dev/null)
  [ "$result" = "claude-sonnet-4-5" ]
}

@test "clamp_model émet un log_warn quand le plancher est appliqué" {
  # Capturer stderr pour le warning
  result=$(clamp_model "claude-haiku-4-5" "claude-opus-4" "test-agent" 2>&1)
  echo "$result" | grep -q "plancher appliqué"
}

@test "clamp_model retourne le résolu quand pas de plancher" {
  run clamp_model "claude-haiku-4-5" "" "test-agent"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-haiku-4-5" ]
}

@test "clamp_model retourne le résolu quand égal au plancher" {
  run clamp_model "claude-sonnet-4-5" "claude-sonnet-4-5" "test-agent"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-sonnet-4-5" ]
}

# ── resolve_agent_model : cascade ─────────────────────────────────────────────

@test "resolve_agent_model niveau 7 : fallback hardcodé claude-sonnet-4-5" {
  # Pas de project_id, pas de hub.json → fallback
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-sonnet-4-5" ]
}

@test "resolve_agent_model niveau 6 : hub.json opencode.model" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-opus-4" }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 5 : hub.json families override" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": { "planning": "claude-opus-4" },
    "agents": {}
  }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 4 : hub.json agents override" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": { "planning": "claude-haiku-4-5" },
    "agents": { "orchestrator-dev": "claude-opus-4" }
  }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 4 prime sur niveau 5" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-haiku-4-5" },
  "agent_models": {
    "families": { "planning": "claude-sonnet-4-5" },
    "agents": { "orchestrator-dev": "claude-opus-4" }
  }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 3 : projet model via _api_keys_get" {
  # Mock _api_keys_get pour le projet
  _api_keys_get() {
    case "$2" in
      model) echo "claude-opus-4" ;;
      *) echo "" ;;
    esac
  }
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" "my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 2 : projet families override" {
  _api_keys_get() {
    case "$2" in
      agent_models.families.planning) echo "claude-opus-4" ;;
      *) echo "" ;;
    esac
  }
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" "my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 1 : projet agents override" {
  _api_keys_get() {
    case "$2" in
      agent_models.agents.orchestrator-dev) echo "claude-opus-4" ;;
      *) echo "" ;;
    esac
  }
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" "my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "resolve_agent_model niveau 1 prime sur niveaux 2-7" {
  _api_keys_get() {
    case "$2" in
      agent_models.agents.orchestrator-dev) echo "claude-haiku-4-5" ;;
      agent_models.families.planning) echo "claude-sonnet-4-5" ;;
      model) echo "claude-opus-4" ;;
      *) echo "" ;;
    esac
  }
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-opus-4" },
  "agent_models": {
    "families": { "planning": "claude-opus-4" },
    "agents": { "orchestrator-dev": "claude-opus-4" }
  }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  run resolve_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" "my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-haiku-4-5" ]
}

# ── resolve_agent_model + clamp ───────────────────────────────────────────────

@test "resolve_agent_model applique le clamp quand résolu < plancher frontmatter" {
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  local result
  result=$(resolve_agent_model "$TEST_DIR/agents/planning/high-floor.md" "" 2>/dev/null)
  [ "$result" = "claude-opus-4" ]
}

@test "resolve_agent_model clamp ne descend pas en dessous du plancher" {
  _api_keys_get() {
    case "$2" in
      agent_models.agents.high-floor) echo "claude-haiku-4-5" ;;
      *) echo "" ;;
    esac
  }
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  local result
  result=$(resolve_agent_model "$TEST_DIR/agents/planning/high-floor.md" "my-project" 2>/dev/null)
  [ "$result" = "claude-opus-4" ]
}

# ── _get_agent_model ──────────────────────────────────────────────────────────

@test "_get_agent_model retourne vide quand modèle résolu == modèle global" {
  # Pas de hub.json → fallback = claude-sonnet-4-5 = DEFAULT_MODEL
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  unset OPENCODE_MODEL
  run _get_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_get_agent_model retourne le modèle quand différent du global" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": {},
    "agents": { "orchestrator-dev": "claude-opus-4" }
  }
}
EOF
  HUB_CONFIG="$TEST_DIR/hub.json"
  unset OPENCODE_MODEL
  run _get_agent_model "$TEST_DIR/agents/planning/orchestrator-dev.md" ""
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic/claude-opus-4" ]
}

@test "_get_agent_model retourne vide quand agent_file est vide" {
  run _get_agent_model "" ""
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_get_agent_model avec clamp retourne le plancher quand global < plancher" {
  HUB_CONFIG="$TEST_DIR/nonexistent.json"
  unset OPENCODE_MODEL
  local result
  result=$(_get_agent_model "$TEST_DIR/agents/planning/high-floor.md" "" 2>/dev/null)
  [ "$result" = "anthropic/claude-opus-4" ]
}
