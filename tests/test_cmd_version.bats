#!/usr/bin/env bats
# Tests pour scripts/cmd-version.sh
# Vérifie : affichage version, format, fallback sans jq
# Optimisation : cmd-version.sh est exécuté UNE SEULE FOIS dans setup_file,
# son output est mis en cache dans BATS_FILE_TMPDIR.

load helpers

setup_file() {
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"
  mkdir -p "$BATS_FILE_TMPDIR"

    > "$HUB_CONFIG"

  # Exécuter cmd-version.sh une seule fois et mettre l'output en cache
  bash "$BATS_TEST_DIRNAME/../scripts/cmd-version.sh" > "$BATS_FILE_TMPDIR/version_output" 2>&1
  export VERSION_STATUS=$?
  export VERSION_OUTPUT_FILE="$BATS_FILE_TMPDIR/version_output"
}

setup() {
  # Rien à initialiser per-test : toutes les fixtures sont dans setup_file
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"
}

teardown() {
  true
}

# ── Affichage ─────────────────────────────────────────────────────────────────

@test "version : s'exécute sans erreur" {
  [ "$VERSION_STATUS" -eq 0 ]
}

@test "version : affiche 'opencode-hub'" {
  grep -q "opencode-hub" "$VERSION_OUTPUT_FILE"
}

@test "version : affiche un numéro de version avec format vX.Y.Z" {
  local out; out=$(cat "$VERSION_OUTPUT_FILE")
  [[ "$out" =~ v[0-9]+\.[0-9]+\.[0-9]+ ]]
}

@test "version : la version lue correspond à hub.json.example" {
  command -v jq &>/dev/null || skip "jq non disponible"
  local expected
  expected=$(jq -r '.version' "$HUB_DIR/config/hub.json.example" 2>/dev/null)
  local out; out=$(cat "$VERSION_OUTPUT_FILE")
  [[ "$out" =~ "$expected" ]]
}

@test "version : fallback si hub.json.example absent — version inconnue affichée" {
  local example_file="$HUB_DIR/config/hub.json.example"
  [ -f "$example_file" ] || skip "hub.json.example introuvable dans le vrai hub"
  local out; out=$(cat "$VERSION_OUTPUT_FILE")
  [[ "$out" =~ v[0-9] ]]
}

@test "version : output sur une seule ligne" {
  local lines
  lines=$(wc -l < "$VERSION_OUTPUT_FILE" | tr -d ' ')
  [ "$lines" -eq 1 ]
}
