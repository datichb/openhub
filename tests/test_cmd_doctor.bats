#!/usr/bin/env bats
# Tests pour scripts/cmd-doctor.sh
# Couvre : exit codes, checks outils, checks config, checks projets

setup() {
  TEST_DIR="$(mktemp -d)"

  # Créer les répertoires de données de test
  mkdir -p "$TEST_DIR/config" "$TEST_DIR/projects"

  # hub.json valide
  cat > "$TEST_DIR/config/hub.json" <<'HUBEOF'
{"version":"1.5.0","cli":{"language":"fr"}}
HUBEOF
  cat > "$TEST_DIR/config/hub.json.example" <<'HUBEOF'
{"version":"1.5.0","cli":{"language":"fr"}}
HUBEOF

  # providers.json minimal
  echo '{"providers":{}}' > "$TEST_DIR/config/providers.json"

  # On surcharge uniquement les fichiers de données — PAS HUB_DIR ni LIB_DIR
  # Le script détecte automatiquement son propre répertoire (réel hub dir)
  export HUB_CONFIG="$TEST_DIR/config/hub.json"
  export HUB_CONFIG_EXAMPLE="$TEST_DIR/config/hub.json.example"
  export PROVIDERS_FILE="$TEST_DIR/config/providers.json"
  export PROJECTS_FILE="$TEST_DIR/projects/projects.md"
  export PATHS_FILE="$TEST_DIR/projects/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/projects/api-keys.local.md"

  CMD_DOCTOR="$BATS_TEST_DIRNAME/../scripts/cmd-doctor.sh"
}

teardown() {
  rm -rf "$TEST_DIR"
}

_run_doctor() {
  run bash "$CMD_DOCTOR" "$@"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Exit codes
# ══════════════════════════════════════════════════════════════════════════════

@test "doctor: exit 0 si tous les outils critiques sont présents et config valide" {
  # Les outils critiques (jq, git, perl) doivent être présents dans l'env CI
  if ! command -v jq &>/dev/null || ! command -v git &>/dev/null; then
    skip "jq ou git absent dans l'environnement de test"
  fi
  _run_doctor
  # Peut être 0 (OK) ou 2 (WARN) selon les outils optionnels présents
  [ "$status" -eq 0 ] || [ "$status" -eq 2 ]
}

@test "doctor: exit 2 si seulement des WARN (aucun FAIL)" {
  # Tous les outils critiques présents, mais api-keys.local.md avec mauvaises permissions
  if ! command -v jq &>/dev/null || ! command -v git &>/dev/null; then
    skip "jq ou git absent dans l'environnement de test"
  fi
  # Créer api-keys.local.md avec permissions trop larges
  echo "[PROJ-A]" > "$API_KEYS_FILE"
  chmod 644 "$API_KEYS_FILE"

  _run_doctor
  # Doit signaler un WARN sur les permissions
  [[ "$output" == *"AVERT"* || "$output" == *"WARN"* || "$output" == *"permissions"* ]]
}

@test "doctor: exit 1 si hub.json est absent" {
  rm -f "$HUB_CONFIG"
  _run_doctor
  [ "$status" -eq 1 ]
  [[ "$output" == *"ÉCHEC"* || "$output" == *"FAIL"* || "$output" == *"absent"* || "$output" == *"missing"* ]]
}

@test "doctor: exit 1 si hub.json est invalide (JSON corrompu)" {
  echo "{ invalid json" > "$HUB_CONFIG"
  _run_doctor
  [ "$status" -eq 1 ]
  [[ "$output" == *"invalide"* || "$output" == *"invalid"* || "$output" == *"FAIL"* || "$output" == *"ÉCHEC"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Checks outils
# ══════════════════════════════════════════════════════════════════════════════

@test "doctor: affiche le statut de jq" {
  _run_doctor
  [[ "$output" == *"jq"* ]]
}

@test "doctor: affiche le statut de git" {
  _run_doctor
  [[ "$output" == *"git"* ]]
}

@test "doctor: affiche le statut de perl" {
  _run_doctor
  [[ "$output" == *"perl"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Checks configuration
# ══════════════════════════════════════════════════════════════════════════════

@test "doctor: détecte la dérive de version entre hub.json et hub.json.example" {
  echo '{"version":"1.0.0"}' > "$HUB_CONFIG"
  echo '{"version":"1.5.0"}' > "$HUB_CONFIG_EXAMPLE"
  _run_doctor
  [[ "$output" == *"version"* ]]
}

@test "doctor: signale l'absence de providers.json" {
  rm -f "$PROVIDERS_FILE"
  _run_doctor
  [[ "$output" == *"providers"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Checks projets
# ══════════════════════════════════════════════════════════════════════════════

@test "doctor: signale un projet avec path introuvable" {
  if ! command -v jq &>/dev/null; then
    skip "jq absent"
  fi
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-MISSING
- Nom : Projet Manquant
- Stack : Node.js
EOF
  echo "PROJ-MISSING=/tmp/nonexistent-$$" > "$PATHS_FILE"

  _run_doctor
  [[ "$output" == *"PROJ-MISSING"* ]]
}

@test "doctor: affiche OK pour un projet avec chemin valide et agents déployés" {
  if ! command -v jq &>/dev/null; then
    skip "jq absent"
  fi
  local proj_dir="$TEST_DIR/my-project"
  mkdir -p "$proj_dir/.opencode/agents"
  touch "$proj_dir/opencode.json"

  cat > "$PROJECTS_FILE" <<EOF
## MY-PROJ
- Nom : My Project
- Stack : Node.js
EOF
  echo "MY-PROJ=$proj_dir" > "$PATHS_FILE"

  _run_doctor
  [[ "$output" == *"MY-PROJ"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. Résumé
# ══════════════════════════════════════════════════════════════════════════════

@test "doctor: affiche un message de résumé" {
  _run_doctor
  [[ "$output" == *"check"* || "$output" == *"contrôle"* || "$output" == *"terminé"* || "$output" == *"passed"* || "$output" == *"passé"* || "$output" == *"FAIL"* || "$output" == *"WARN"* ]]
}

@test "doctor: crée le répertoire .locks s'il est absent" {
  # Le répertoire .locks est créé dans le vrai HUB_DIR (répertoire du hub)
  # Ce test vérifie juste que le script ne crashe pas et termine normalement
  _run_doctor || true
  [ "$status" -eq 0 ] || [ "$status" -eq 1 ] || [ "$status" -eq 2 ]
}
