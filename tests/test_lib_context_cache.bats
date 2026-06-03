#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/context-cache.sh
# Fonctions testées : cache_exists, validate_context_cache, cache_invalidate,
#                     cache_get_generated_at, cache_get_stack,
#                     _inject_context_instructions

load helpers

setup() {
  common_setup

  # Sourcer common.sh (nécessaire pour les helpers partagés)
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Sourcer context-cache.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/context-cache.sh"

  # Répertoire projet de test
  PROJECT_PATH="$TEST_DIR/fake-project"
  mkdir -p "$PROJECT_PATH"

  # opencode.json minimal pour les tests _inject_context_instructions
  echo '{"$schema":"https://opencode.ai/config.json","model":"claude-sonnet-4"}' \
    > "$PROJECT_PATH/opencode.json"
}

teardown() {
  common_teardown
}

# ── cache_exists ──────────────────────────────────────────────────────────────

@test "cache_exists : retourne 1 si context.json absent" {
  run cache_exists "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "cache_exists : retourne 0 si context.json présent" {
  mkdir -p "$PROJECT_PATH/.opencode"
  echo '{}' > "$PROJECT_PATH/.opencode/context.json"

  run cache_exists "$PROJECT_PATH"
  [ "$status" -eq 0 ]
}

# ── validate_context_cache ────────────────────────────────────────────────────

@test "validate_context_cache : retourne 1 si cache absent" {
  run validate_context_cache "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "validate_context_cache : retourne 1 si JSON invalide" {
  mkdir -p "$PROJECT_PATH/.opencode"
  echo 'NOT JSON' > "$PROJECT_PATH/.opencode/context.json"

  run validate_context_cache "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "validate_context_cache : retourne 0 si key_files vide et JSON valide" {
  mkdir -p "$PROJECT_PATH/.opencode"
  cat > "$PROJECT_PATH/.opencode/context.json" <<'EOF'
{
  "version": "1.0",
  "generated_at": "2026-01-01T00:00:00Z",
  "stack": {"languages": ["typescript"]},
  "conventions": {"source": "CONVENTIONS.md", "hash": "sha256:abc"},
  "key_files": {}
}
EOF

  run validate_context_cache "$PROJECT_PATH"
  [ "$status" -eq 0 ]
}

@test "validate_context_cache : retourne 1 si un fichier hashé a changé" {
  command -v shasum &>/dev/null || command -v sha256sum &>/dev/null || skip "sha256 non disponible"

  mkdir -p "$PROJECT_PATH/.opencode"
  echo "original" > "$PROJECT_PATH/CONVENTIONS.md"

  # Hash incorrect (simulant un fichier modifié depuis la génération du cache)
  cat > "$PROJECT_PATH/.opencode/context.json" <<'EOF'
{
  "version": "1.0",
  "generated_at": "2026-01-01T00:00:00Z",
  "stack": {},
  "conventions": {},
  "key_files": {
    "CONVENTIONS.md": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}
EOF

  run validate_context_cache "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "validate_context_cache : retourne 1 si un fichier hashé est supprimé" {
  mkdir -p "$PROJECT_PATH/.opencode"
  cat > "$PROJECT_PATH/.opencode/context.json" <<'EOF'
{
  "version": "1.0",
  "generated_at": "2026-01-01T00:00:00Z",
  "stack": {},
  "conventions": {},
  "key_files": {
    "fichier-inexistant.json": "sha256:abc"
  }
}
EOF

  run validate_context_cache "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

# ── cache_invalidate ──────────────────────────────────────────────────────────

@test "cache_invalidate : supprime context.json" {
  mkdir -p "$PROJECT_PATH/.opencode"
  echo '{}' > "$PROJECT_PATH/.opencode/context.json"

  run cache_invalidate "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  [ ! -f "$PROJECT_PATH/.opencode/context.json" ]
}

@test "cache_invalidate : retourne 1 si context.json absent" {
  run cache_invalidate "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

# ── cache_get_generated_at ────────────────────────────────────────────────────

@test "cache_get_generated_at : retourne la date de génération" {
  command -v jq &>/dev/null || skip "jq non disponible"
  mkdir -p "$PROJECT_PATH/.opencode"
  cat > "$PROJECT_PATH/.opencode/context.json" <<'EOF'
{"version":"1.0","generated_at":"2026-05-28T10:30:00Z","stack":{},"conventions":{},"key_files":{}}
EOF

  run cache_get_generated_at "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  [ "$output" = "2026-05-28T10:30:00Z" ]
}

# ── _inject_context_instructions ─────────────────────────────────────────────

@test "_inject_context_instructions : pas d'instructions si aucun contexte" {
  command -v jq &>/dev/null || skip "jq non disponible"
  # Aucun ONBOARDING.md, CONVENTIONS.md ni cache

  _inject_context_instructions "$PROJECT_PATH"

  run jq 'has("instructions")' "$PROJECT_PATH/opencode.json"
  [ "$output" = "false" ]
}

@test "_inject_context_instructions : injecte ONBOARDING.md si présent (sans cache)" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/ONBOARDING.md"

  _inject_context_instructions "$PROJECT_PATH"

  run jq -r '.instructions[]' "$PROJECT_PATH/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ONBOARDING.md"* ]]
}

@test "_inject_context_instructions : injecte CONVENTIONS.md si présent (sans cache)" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/CONVENTIONS.md"

  _inject_context_instructions "$PROJECT_PATH"

  run jq -r '.instructions[]' "$PROJECT_PATH/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONVENTIONS.md"* ]]
}

@test "_inject_context_instructions : injecte les deux fichiers si présents" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/ONBOARDING.md"
  touch "$PROJECT_PATH/CONVENTIONS.md"

  _inject_context_instructions "$PROJECT_PATH"

  run jq -r '.instructions | length' "$PROJECT_PATH/opencode.json"
  [ "$output" = "2" ]
}

@test "_inject_context_instructions : préfère context.json valide à ONBOARDING.md" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/ONBOARDING.md"
  mkdir -p "$PROJECT_PATH/.opencode"
  cat > "$PROJECT_PATH/.opencode/context.json" <<'EOF'
{"version":"1.0","generated_at":"2026-01-01T00:00:00Z","stack":{},"conventions":{},"key_files":{}}
EOF
  # Mock validate_context_cache : retourne toujours succès
  validate_context_cache() { return 0; }

  _inject_context_instructions "$PROJECT_PATH"

  run jq -r '.instructions[]' "$PROJECT_PATH/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *".opencode/context.json"* ]]
  [[ "$output" != *"ONBOARDING.md"* ]]
}

@test "_inject_context_instructions : fallback ONBOARDING.md si cache invalide" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/ONBOARDING.md"
  mkdir -p "$PROJECT_PATH/.opencode"
  echo '{}' > "$PROJECT_PATH/.opencode/context.json"
  # Mock validate_context_cache : retourne toujours échec
  validate_context_cache() { return 1; }

  _inject_context_instructions "$PROJECT_PATH"

  run jq -r '.instructions[]' "$PROJECT_PATH/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ONBOARDING.md"* ]]
  [[ "$output" != *"context.json"* ]]
}

@test "_inject_context_instructions : supprime instructions si contexte disparaît" {
  command -v jq &>/dev/null || skip "jq non disponible"
  # opencode.json avec instructions préexistantes
  echo '{"$schema":"https://opencode.ai/config.json","model":"claude-sonnet-4","instructions":["OLD.md"]}' \
    > "$PROJECT_PATH/opencode.json"
  # Aucun fichier contexte, pas de cache

  _inject_context_instructions "$PROJECT_PATH"

  run jq 'has("instructions")' "$PROJECT_PATH/opencode.json"
  [ "$output" = "false" ]
}

@test "_inject_context_instructions : opencode.json reste du JSON valide après injection" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$PROJECT_PATH/ONBOARDING.md"
  touch "$PROJECT_PATH/CONVENTIONS.md"

  _inject_context_instructions "$PROJECT_PATH"

  run jq . "$PROJECT_PATH/opencode.json"
  [ "$status" -eq 0 ]
}

@test "_inject_context_instructions : sans effet si opencode.json absent" {
  # Ne doit pas planter si opencode.json n'existe pas
  local proj="$TEST_DIR/no-config-project"
  mkdir -p "$proj"
  touch "$proj/ONBOARDING.md"

  run _inject_context_instructions "$proj"
  # Doit retourner 0 (silencieux)
  [ "$status" -eq 0 ]
}
