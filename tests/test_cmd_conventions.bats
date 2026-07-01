#!/usr/bin/env bats
# Tests pour scripts/cmd-conventions.sh
# Vérifie : validation args, --force/-f, CONVENTIONS.md existant, lancement adapter

load helpers

setup() {
  common_setup

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
    > "$HUB_CONFIG"

  CMD_CONVENTIONS="$BATS_TEST_DIRNAME/../scripts/cmd-conventions.sh"

  mkdir -p "$TEST_DIR/proj-conv"

  cat > "$PROJECTS_FILE" <<'EOF'
# Registre de test

## CONV-PROJ
- Nom : Conv Project
- Stack : Node.js
- Board Beads : CONV-PROJ
- Tracker : none
- Labels : test
- Agents : all
EOF

  cat > "$PATHS_FILE" <<EOF
CONV-PROJ=$TEST_DIR/proj-conv
EOF

  : > "$API_KEYS_FILE"

  # Mock opencode
  OPENCODE_LOG="$TEST_DIR/opencode_calls.log"
  export OPENCODE_LOG
  : > "$OPENCODE_LOG"
  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*" >> "$OPENCODE_LOG"
exit 0
OCEOF
  chmod +x "$TEST_DIR/bin/opencode"
  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  common_teardown
}

# ── Validation ────────────────────────────────────────────────────────────────

@test "conventions : projet inexistant → erreur" {
  run bash "$CMD_CONVENTIONS" -p "INEXISTANT"
  [ "$status" -ne 0 ]
}

# ── CONVENTIONS.md absent — lancement direct ──────────────────────────────────

@test "conventions : CONVENTIONS.md absent → lance opencode" {
  run bash -c 'printf "\n" | bash "$1" -p CONV-PROJ' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
}

@test "conventions : affiche le chemin du projet" {
  run bash -c 'printf "\n" | bash "$1" -p CONV-PROJ' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  [[ "$output" =~ "$TEST_DIR/proj-conv" ]]
}

# ── CONVENTIONS.md existant — demande confirmation ────────────────────────────

@test "conventions : CONVENTIONS.md existant + réponse N → annule" {
  echo "# Conventions existantes" > "$TEST_DIR/proj-conv/CONVENTIONS.md"
  # Réponse N pour ne pas écraser
  run bash -c 'printf "N\n" | bash "$1" -p CONV-PROJ' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  # opencode ne doit pas avoir été appelé
  [ ! -s "$OPENCODE_LOG" ]
}

@test "conventions : CONVENTIONS.md existant + réponse Y → lance opencode" {
  echo "# Conventions existantes" > "$TEST_DIR/proj-conv/CONVENTIONS.md"
  # Réponse Y pour écraser, puis Enter pour confirmation lancement
  run bash -c 'printf "y\n\n" | bash "$1" -p CONV-PROJ' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
}

# ── Flag --force ──────────────────────────────────────────────────────────────

@test "conventions : --force bypass la confirmation d'écrasement" {
  echo "# Conventions existantes" > "$TEST_DIR/proj-conv/CONVENTIONS.md"
  # Avec --force, pas de question → Enter pour lancement
  run bash -c 'printf "\n" | bash "$1" -p CONV-PROJ --force' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
}

@test "conventions : -f est alias de --force" {
  echo "# Conventions existantes" > "$TEST_DIR/proj-conv/CONVENTIONS.md"
  run bash -c 'printf "\n" | bash "$1" -p CONV-PROJ -f' _ "$CMD_CONVENTIONS"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
}
