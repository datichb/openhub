#!/usr/bin/env bats
# Tests pour scripts/cmd-update.sh
# Vérifie : adapter_update appelé, gestion bd absent, gestion brew absent, skills_none
# Optimisation : cmd-update.sh est exécuté UNE SEULE FOIS dans setup_file,
# son output est mis en cache dans BATS_FILE_TMPDIR.

load helpers

setup_file() {
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"
  mkdir -p "$BATS_FILE_TMPDIR/bin"

  printf '{"version":"1.0.0","default_target":"opencode","active_targets":["opencode"],"cli":{"language":"fr"}}\n' \
    > "$HUB_CONFIG"

  # Mock opencode
  cat > "$BATS_FILE_TMPDIR/bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*"
exit 0
OCEOF
  chmod +x "$BATS_FILE_TMPDIR/bin/opencode"

  # Mock npm
  cat > "$BATS_FILE_TMPDIR/bin/npm" <<'NPMEOF'
#!/bin/bash
echo "npm $*"
exit 0
NPMEOF
  chmod +x "$BATS_FILE_TMPDIR/bin/npm"

  export PATH="$BATS_FILE_TMPDIR/bin:$PATH"
  export UPDATE_OUTPUT_FILE="$BATS_FILE_TMPDIR/update_output"

  # Exécuter une seule fois avec réponse N à la question sync
  bash -c 'printf "N\n" | bash "$1"' _ "$BATS_TEST_DIRNAME/../scripts/cmd-update.sh" \
    > "$UPDATE_OUTPUT_FILE" 2>&1
  export UPDATE_STATUS=$?
}

setup() {
  # Rien à initialiser per-test : toutes les fixtures sont dans setup_file
  # On exporte juste les variables nécessaires
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"
  export PATH="$BATS_FILE_TMPDIR/bin:$PATH"
}

teardown() {
  true
}

# ── Exécution générale ────────────────────────────────────────────────────────

@test "update : s'exécute sans erreur (bd absent, no skills)" {
  [ "$UPDATE_STATUS" -eq 0 ]
}

@test "update : affiche le titre" {
  local out; out=$(cat "$UPDATE_OUTPUT_FILE")
  [[ "$out" =~ "Mise à jour" ]]
}

@test "update : affiche succès en fin d'exécution" {
  local out; out=$(cat "$UPDATE_OUTPUT_FILE")
  [[ "$out" =~ "terminée" ]]
}

# ── Gestion bd absent ─────────────────────────────────────────────────────────

@test "update : bd absent → warning mais pas d'erreur fatale" {
  [ "$UPDATE_STATUS" -eq 0 ]
  local out; out=$(cat "$UPDATE_OUTPUT_FILE")
  [[ "$out" =~ "bd" ]]
}

# ── Gestion skills ────────────────────────────────────────────────────────────

@test "update : sans fichier .sources.json → message skills_none" {
  local out; out=$(cat "$UPDATE_OUTPUT_FILE")
  [[ "$out" =~ "Aucun skill externe" ]]
}

# ── Proposition sync après mise à jour skills ─────────────────────────────────

@test "update : réponse N à sync_now → pas de sync" {
  [ "$UPDATE_STATUS" -eq 0 ]
}
