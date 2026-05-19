#!/usr/bin/env bats
# Tests de validation du schéma agent_models dans hub.json
# Critères d'acceptance : rétrocompatibilité, parsing, robustesse

setup() {
  TEST_DIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── Rétrocompatibilité ────────────────────────────────────────────────────────

@test "hub.json sans agent_models est parsable par jq" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": {
    "model": "claude-sonnet-4-5"
  },
  "cli": {
    "language": "fr"
  }
}
EOF
  run jq '.' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
}

@test "hub.json sans agent_models retourne null pour .agent_models" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
EOF
  run jq -r '.agent_models // empty' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "hub.json sans agent_models retourne empty pour .agent_models.families" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
EOF
  run jq -r '.agent_models.families // empty' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "hub.json sans agent_models retourne empty pour .agent_models.agents" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
EOF
  run jq -r '.agent_models.agents // empty' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── Parsing avec agent_models bien formé ──────────────────────────────────────

@test "hub.json avec agent_models vide est parsable" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": {},
    "agents": {}
  }
}
EOF
  run jq '.' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
}

@test "hub.json avec agent_models.families peuplé retourne la bonne valeur" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": {
      "planning": "claude-opus-4",
      "auditor": "claude-haiku-4-5"
    },
    "agents": {}
  }
}
EOF
  run jq -r '.agent_models.families.planning' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "hub.json avec agent_models.agents peuplé retourne la bonne valeur" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": {},
    "agents": {
      "orchestrator-dev": "claude-opus-4",
      "debugger": "claude-sonnet-4-5"
    }
  }
}
EOF
  run jq -r '.agent_models.agents."orchestrator-dev"' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "hub.json avec families et agents peuplés est valide" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "opencode": { "model": "claude-sonnet-4-5" },
  "agent_models": {
    "families": {
      "planning": "claude-opus-4",
      "auditor": "claude-haiku-4-5"
    },
    "agents": {
      "orchestrator-dev": "claude-opus-4",
      "debugger": "claude-sonnet-4-5"
    }
  }
}
EOF
  run jq '.' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
}

@test "famille inexistante retourne null" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "agent_models": {
    "families": { "planning": "claude-opus-4" },
    "agents": {}
  }
}
EOF
  run jq -r '.agent_models.families.inexistant // empty' "$TEST_DIR/hub.json"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── Robustesse : agent_models mal formé ───────────────────────────────────────

@test "hub.json avec agent_models non-objet est détecté par jq type check" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "agent_models": "invalid"
}
EOF
  run jq -e '.agent_models | type == "object"' "$TEST_DIR/hub.json"
  [ "$status" -ne 0 ]
}

@test "hub.json avec families non-objet est détecté par jq type check" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "agent_models": {
    "families": "not-an-object",
    "agents": {}
  }
}
EOF
  run jq -e '.agent_models.families | type == "object"' "$TEST_DIR/hub.json"
  [ "$status" -ne 0 ]
}

@test "hub.json avec agents non-objet est détecté par jq type check" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "version": "1.4.0",
  "agent_models": {
    "families": {},
    "agents": ["not", "an", "object"]
  }
}
EOF
  run jq -e '.agent_models.agents | type == "object"' "$TEST_DIR/hub.json"
  [ "$status" -ne 0 ]
}

@test "hub.json avec JSON invalide est rejeté par jq" {
  cat > "$TEST_DIR/hub.json" <<'EOF'
{
  "agent_models": {
    "families": { "planning": "claude-opus-4", }
  }
}
EOF
  run jq '.' "$TEST_DIR/hub.json"
  [ "$status" -ne 0 ]
}

# ── hub.json.example contient le bloc agent_models ────────────────────────────

@test "hub.json.example contient le bloc agent_models" {
  run jq -e '.agent_models' "$BATS_TEST_DIRNAME/../config/hub.json.example"
  [ "$status" -eq 0 ]
}

@test "hub.json.example agent_models a les clés families et agents" {
  run jq -e '.agent_models | has("families") and has("agents")' "$BATS_TEST_DIRNAME/../config/hub.json.example"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}
