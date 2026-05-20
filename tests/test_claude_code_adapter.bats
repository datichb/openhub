#!/usr/bin/env bats
# Tests pour scripts/adapters/claude-code.adapter.sh
# Fonctions testées : adapter_deploy (génération .claude/agents/*.md)
# Stratégie : sourcer common.sh + prompt-builder.sh + claude-code.adapter.sh
#             avec un agent de test minimal dans CANONICAL_AGENTS_DIR

setup() {
  TEST_DIR="$(mktemp -d)"
  DEPLOY_DIR="$(mktemp -d)"
  AGENTS_DIR="$(mktemp -d)"

  # Fixer HUB_DIR avant le source
  HUB_DIR="$BATS_TEST_DIRNAME/.."

  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Surcharger les chemins après le source
  PROJECTS_FILE="$TEST_DIR/projects.md"
  API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  CANONICAL_AGENTS_DIR="$AGENTS_DIR"

  # Créer un agent de test supportant claude-code
  mkdir -p "$AGENTS_DIR/test"
  cat > "$AGENTS_DIR/test/test-agent.md" <<'AGENTEOF'
---
id: test-agent
label: TestAgent
description: Un agent de test pour bats
targets: [claude-code]
skills: []
---

# Agent de test

Ceci est le corps de l'agent de test.
AGENTEOF

  # Créer un agent qui ne supporte PAS claude-code (seulement opencode)
  cat > "$AGENTS_DIR/test/opencode-only.md" <<'AGENTEOF'
---
id: opencode-only
label: OpencodeOnly
description: Agent opencode uniquement
targets: [opencode]
skills: []
---

# Opencode Only
AGENTEOF

  source "$BATS_TEST_DIRNAME/../scripts/lib/prompt-builder.sh"
  source "$BATS_TEST_DIRNAME/../scripts/adapters/claude-code.adapter.sh"

  # Mocks
  log_info()    { true; }
  log_success() { true; }
  log_warn()    { true; }
  log_error()   { true; }
  get_project_language() { echo ""; }
}

teardown() {
  rm -rf "$TEST_DIR" "$DEPLOY_DIR" "$AGENTS_DIR"
}

# ── adapter_deploy ────────────────────────────────────────────────────────────

@test "claude-code adapter_deploy : crée le dossier .claude/agents/" {
  adapter_deploy "$DEPLOY_DIR" ""
  [ -d "$DEPLOY_DIR/.claude/agents" ]
}

@test "claude-code adapter_deploy : génère le fichier agent avec le bon nom" {
  adapter_deploy "$DEPLOY_DIR" ""
  [ -f "$DEPLOY_DIR/.claude/agents/test-agent.md" ]
}

@test "claude-code adapter_deploy : ne génère pas un agent qui ne supporte pas claude-code" {
  adapter_deploy "$DEPLOY_DIR" ""
  [ ! -f "$DEPLOY_DIR/.claude/agents/opencode-only.md" ]
}

@test "claude-code adapter_deploy : fichier généré contient le frontmatter name" {
  adapter_deploy "$DEPLOY_DIR" ""
  grep -q "^name: TestAgent" "$DEPLOY_DIR/.claude/agents/test-agent.md"
}

@test "claude-code adapter_deploy : fichier généré contient la description" {
  adapter_deploy "$DEPLOY_DIR" ""
  grep -q "Un agent de test pour bats" "$DEPLOY_DIR/.claude/agents/test-agent.md"
}

@test "claude-code adapter_deploy : fichier généré contient le corps de l'agent" {
  adapter_deploy "$DEPLOY_DIR" ""
  grep -q "Ceci est le corps" "$DEPLOY_DIR/.claude/agents/test-agent.md"
}

@test "claude-code adapter_deploy : avec langue, le contenu inclut l'instruction de langue" {
  get_project_language() { echo "english"; }
  adapter_deploy "$DEPLOY_DIR" "PROJ-EN"
  # Le build_agent_content doit injecter une instruction de langue
  grep -qi "english" "$DEPLOY_DIR/.claude/agents/test-agent.md"
}

# ── adapter_deploy_files (Phase 1 directe) ────────────────────────────────────

@test "claude-code adapter_deploy_files : crée le dossier .claude/agents/" {
  adapter_deploy_files "$DEPLOY_DIR" ""
  [ -d "$DEPLOY_DIR/.claude/agents" ]
}

@test "claude-code adapter_deploy_files : génère le fichier agent" {
  adapter_deploy_files "$DEPLOY_DIR" ""
  [ -f "$DEPLOY_DIR/.claude/agents/test-agent.md" ]
}

@test "claude-code adapter_deploy_files : remplit _DEPLOY_FILES_COUNT" {
  adapter_deploy_files "$DEPLOY_DIR" ""
  [ "$_DEPLOY_FILES_COUNT" -eq 1 ]
}

@test "claude-code adapter_deploy_files : remplit _DEPLOY_FILES_COUNT (seule variable de reporting)" {
  # claude-code ne peuple pas _DEPLOY_FILES_AGENT_KEYS/VALS/FILES car sa Phase 2 est un no-op —
  # seul _DEPLOY_FILES_COUNT est utilisé pour le reporting dans cmd-deploy.sh.
  adapter_deploy_files "$DEPLOY_DIR" ""
  [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 0 ]
}

@test "claude-code adapter_deploy_files : ne déploie pas un agent non compatible" {
  adapter_deploy_files "$DEPLOY_DIR" ""
  [ ! -f "$DEPLOY_DIR/.claude/agents/opencode-only.md" ]
}

# ── adapter_deploy_config (Phase 2 — no-op pour claude-code) ──────────────────

@test "claude-code adapter_deploy_config : ne produit aucun fichier de config" {
  adapter_deploy_config "$DEPLOY_DIR" ""
  [ ! -f "$DEPLOY_DIR/opencode.json" ]
  [ ! -f "$DEPLOY_DIR/claude.json" ]
}

@test "claude-code adapter_deploy_config : appelable seul sans Phase 1 (no-op idempotent)" {
  # Aucune Phase 1 préalable — ne doit pas planter
  run adapter_deploy_config "$DEPLOY_DIR" ""
  [ "$status" -eq 0 ]
}

@test "claude-code adapter_deploy_config : appelable après Phase 1 (no-op idempotent)" {
  adapter_deploy_files "$DEPLOY_DIR" ""
  run adapter_deploy_config "$DEPLOY_DIR" ""
  [ "$status" -eq 0 ]
}
