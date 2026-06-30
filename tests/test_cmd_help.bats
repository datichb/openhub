#!/usr/bin/env bats
# Tests pour scripts/cmd-help.sh
# Vérifie : sections affichées, commandes listées, code de sortie, i18n
# Optimisation : cmd-help.sh est exécuté UNE SEULE FOIS dans setup_file,
# son output est mis en cache dans BATS_FILE_TMPDIR — chaque test lit le cache.

load helpers

setup_file() {
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"

  mkdir -p "$BATS_FILE_TMPDIR"
    > "$HUB_CONFIG"

  # Exécuter cmd-help.sh une seule fois et mettre l'output en cache
  bash "$BATS_TEST_DIRNAME/../scripts/cmd-help.sh" > "$BATS_FILE_TMPDIR/help_output" 2>&1
  export HELP_STATUS=$?
  export HELP_OUTPUT_FILE="$BATS_FILE_TMPDIR/help_output"
}

setup() {
  # Rien à initialiser per-test : toutes les fixtures sont dans setup_file
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export HUB_CONFIG="$BATS_FILE_TMPDIR/hub.json"
}

teardown() {
  true
}

# Helper : charge l'output mis en cache
_help_output() {
  cat "$HELP_OUTPUT_FILE"
}

# ── Comportement général ──────────────────────────────────────────────────────

@test "help : s'exécute sans erreur" {
  [ "$HELP_STATUS" -eq 0 ]
}

@test "help : output non vide" {
  [ "$HELP_STATUS" -eq 0 ]
  [ -s "$HELP_OUTPUT_FILE" ]
}

@test "help : affiche plusieurs lignes" {
  local lines
  lines=$(wc -l < "$HELP_OUTPUT_FILE" | tr -d ' ')
  [ "$lines" -gt 10 ]
}

# ── Sections ──────────────────────────────────────────────────────────────────

@test "help : contient une section setup/installation" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Ss]etup ]] || [[ "$out" =~ [Ii]nstall ]] || [[ "$out" =~ [Cc]onfiguration ]]
}

@test "help : contient une section projets/projects" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Pp]rojet ]] || [[ "$out" =~ [Pp]roject ]]
}

@test "help : contient une section lancement/launch" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Ll]ancement ]] || [[ "$out" =~ [Ll]aunch ]] || [[ "$out" =~ [Ss]tart ]]
}

@test "help : contient une section analyse/analysis" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Aa]nalyse ]] || [[ "$out" =~ [Aa]nalysis ]] || [[ "$out" =~ [Aa]udit ]]
}

@test "help : contient une section updates/mise à jour" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Uu]pdate ]] || [[ "$out" =~ [Mm]ise.à.jour ]] || [[ "$out" =~ [Uu]pgrade ]]
}

@test "help : contient une section config" {
  local out; out=$(_help_output)
  [[ "$out" =~ [Cc]onfig ]]
}

# ── Commandes listées ─────────────────────────────────────────────────────────

@test "help : mentionne la commande start" {
  grep -q "start" "$HELP_OUTPUT_FILE"
}

@test "help : mentionne la commande deploy" {
  grep -q "deploy" "$HELP_OUTPUT_FILE"
}

@test "help : mentionne la commande audit" {
  grep -q "audit" "$HELP_OUTPUT_FILE"
}

@test "help : mentionne des exemples d'utilisation" {
  local out; out=$(_help_output)
  [[ "$out" =~ "oc.sh" ]] || [[ "$out" =~ "oc " ]]
}
