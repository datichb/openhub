#!/usr/bin/env bats
# Tests pour scripts/cmd-start.sh — gate exec + bd init interactif
# cmd-start.sh est un script top-level (non sourceable) — testé via exécution directe.
# adapter_start fait exec → on mock opencode comme un script PATH.

setup() {
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # Isoler HUB_CONFIG pour éviter que le hub.json local (ex: default_provider bedrock)
  # n'influence le comportement de adapter_start — langue française pour OC_LANG=fr
  export HUB_CONFIG="$TEST_DIR/hub.json"
  echo '{"cli":{"language":"fr"}}' > "$HUB_CONFIG"

  # OC_NON_INTERACTIVE=0 : les prompts lisent stdin (pipe) au lieu de retourner vide.
  # Nécessaire pour les tests 7 et 8 qui vérifient la lecture de l'URL upstream.
  export OC_NON_INTERACTIVE=0

  CMD_START="$BATS_TEST_DIRNAME/../scripts/cmd-start.sh"

  # ── Données de test ──────────────────────────────────────────────────────────
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test

## TEST-PROJ
- Nom : Projet Test
- Stack : Node.js
- Board Beads : TEST-PROJ
- Tracker : none
- Labels : feature,fix
- Agents : all
PROJEOF

  mkdir -p "$TEST_DIR/fake-project"
  cat > "$PATHS_FILE" <<EOF
TEST-PROJ=$TEST_DIR/fake-project
EOF

  : > "$API_KEYS_FILE"

  # ── Mock opencode dans le PATH ────────────────────────────────────────────────
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

  # ── Mock bd dans le PATH ─────────────────────────────────────────────────────
  BD_CALLS_LOG="$TEST_DIR/bd_calls.log"
  export BD_CALLS_LOG
  : > "$BD_CALLS_LOG"

  cat > "$TEST_DIR/bin/bd" <<'BDEOF'
#!/bin/bash
echo "bd $*" >> "$BD_CALLS_LOG"
if [ "${1:-}" = "init" ]; then
  mkdir -p .beads
fi
exit 0
BDEOF
  chmod +x "$TEST_DIR/bin/bd"

  # ── Mock git — intercepte remote, délègue le reste au vrai git ───────────────
  GIT_CALLS_LOG="$TEST_DIR/git_calls.log"
  export GIT_CALLS_LOG
  : > "$GIT_CALLS_LOG"

  REAL_GIT="$(command -v git)"
  export REAL_GIT
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
if [ "${1:-}" = "remote" ]; then
  if [ "${2:-}" = "get-url" ]; then
    exit 1
  elif [ "${2:-}" = "add" ]; then
    exit 0
  fi
  exit 0
fi
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  # Mock jq : common.sh utilise "opencode" si jq est absent ou hub.json absent
  # On s'assure que le mock opencode est trouvé par adapter_validate
  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  unset HUB_CONFIG
  rm -rf "$TEST_DIR"
}

# ── Gate : confirmation avant lancement ──────────────────────────────────────

@test "cmd-start : affiche la confirmation avant lancement" {
  # .beads existe → pas de question bd init ; juste Enter pour le gate
  mkdir -p "$TEST_DIR/fake-project/.beads"
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # Le message de gate doit apparaître
  [[ "$output" == *"Appuyer sur Entrée pour lancer"* ]]
  # opencode a été appelé
  [ -s "$OPENCODE_LOG" ]
}

@test "cmd-start : lance opencode après la confirmation" {
  mkdir -p "$TEST_DIR/fake-project/.beads"
  mkdir -p "$TEST_DIR/fake-project/.opencode/agents"
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # opencode a été lancé (via exec → notre mock)
  grep -q "opencode" "$OPENCODE_LOG"
}

# ── Beads init interactif ────────────────────────────────────────────────────

@test "cmd-start : propose bd init si .beads absent et bd disponible" {
  # Pas de .beads → Y(bd init), n(upstream), Enter(gate)
  run bash -c '
    printf "Y\nn\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # bd init a été appelé
  grep -q "bd init" "$BD_CALLS_LOG"
  # Les labels ont été propagés
  grep -q "bd label create feature" "$BD_CALLS_LOG"
  grep -q "bd label create fix" "$BD_CALLS_LOG"
}

@test "cmd-start : respecte le refus de bd init (n)" {
  # Pas de .beads → question → n, puis Enter gate
  run bash -c '
    printf "n\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # bd init ne doit PAS avoir été appelé
  ! grep -q "bd init" "$BD_CALLS_LOG"
  # Mais opencode doit quand même être lancé
  [ -s "$OPENCODE_LOG" ]
}

@test "cmd-start : avertissement passif si .beads absent et bd indisponible" {
  # Retirer le mock bd et restreindre le PATH pour exclure le vrai bd (/opt/homebrew/bin)
  rm -f "$TEST_DIR/bin/bd"
  run bash -c '
    export PATH="'"$TEST_DIR/bin"':/usr/bin:/bin"
    printf "\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # Message d'avertissement passif
  [[ "$output" == *"Beads non initialisé"* ]]
  # Pas de question bd init
  [[ "$output" != *"Initialiser Beads maintenant"* ]]
}

@test "cmd-start : propage les labels après bd init" {
  # Pas de .beads → Y(bd init), n(upstream), Enter(gate)
  run bash -c '
    printf "Y\nn\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # Vérifier les appels bd
  grep -q "bd init" "$BD_CALLS_LOG"
  grep -q "bd label create feature" "$BD_CALLS_LOG"
  grep -q "bd label create fix" "$BD_CALLS_LOG"
}

# ── Proposition upstream git ──────────────────────────────────────────────────

@test "cmd-start : propose git remote add upstream après bd init" {
  # Pas de .beads → Y(bd init), Y(upstream), URL, Enter(gate)
  run bash -c '
    printf "Y\nY\nhttps://github.com/test/repo.git\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # git remote add upstream a été appelé avec l'URL
  grep -q "git remote add upstream https://github.com/test/repo.git" "$GIT_CALLS_LOG"
  [[ "$output" == *"Remote upstream configuré"* ]]
}

@test "cmd-start : respecte le refus de configurer upstream (n)" {
  # Pas de .beads → Y(bd init), n(upstream), Enter(gate)
  : > "$GIT_CALLS_LOG"
  run bash -c '
    printf "Y\nn\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # git remote add ne doit PAS avoir été appelé
  ! grep -q "git remote add upstream" "$GIT_CALLS_LOG"
  [[ "$output" == *"Configurer plus tard"* ]]
}

# ── Mode --dev ───────────────────────────────────────────────────────────────

@test "cmd-start : --dev exit si .beads absent" {
  # --dev requiert .beads — doit exit 1
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ --dev
  ' _ "$CMD_START"
  [ "$status" -ne 0 ]
  [[ "$output" == *"requiert Beads"* ]]
}

# ── Flag --provider avec agents déjà déployés ────────────────────────────────
# Vérifie que seule adapter_deploy_config est appelée (pas adapter_deploy entier)

@test "cmd-start --provider : appelle adapter_deploy_config si agents déjà déployés" {
  # Préparer un projet avec agents déjà déployés
  mkdir -p "$TEST_DIR/fake-project/.opencode/agents"
  mkdir -p "$TEST_DIR/fake-project/.beads"

  DEPLOY_CALLS_LOG="$TEST_DIR/deploy_calls.log"
  : > "$DEPLOY_CALLS_LOG"

  # Créer un adapter mock qui trace les appels dans le log
  # cmd-start source l'adapter via load_adapter — on remplace le fichier source
  ADAPTERS_DIR_MOCK="$TEST_DIR/adapters"
  mkdir -p "$ADAPTERS_DIR_MOCK"
  cat > "$ADAPTERS_DIR_MOCK/opencode.adapter.sh" << ADAPTEREOF
adapter_validate()      { return 0; }
adapter_needs_node()    { return 0; }
adapter_deploy_files()  { echo "adapter_deploy_files called"  >> "$DEPLOY_CALLS_LOG"; }
adapter_deploy_config() { echo "adapter_deploy_config called" >> "$DEPLOY_CALLS_LOG"; }
adapter_deploy()        { echo "adapter_deploy called"        >> "$DEPLOY_CALLS_LOG"; }
adapter_install()       { true; }
adapter_update()        { true; }
adapter_start()         { true; }
ADAPTEREOF

  run bash -c '
    export PATH="'"$TEST_DIR/bin"':$PATH"
    export HUB_CONFIG="'"$HUB_CONFIG"'"
    export PROJECTS_FILE="'"$PROJECTS_FILE"'"
    export PATHS_FILE="'"$PATHS_FILE"'"
    export API_KEYS_FILE="'"$API_KEYS_FILE"'"
    export ADAPTERS_DIR="'"$ADAPTERS_DIR_MOCK"'"
    printf "\n" | bash "'"$CMD_START"'" TEST-PROJ --provider anthropic 2>/dev/null || true
  '

  # adapter_deploy_config doit avoir été appelé (Phase 2 seule)
  grep -q "adapter_deploy_config called" "$DEPLOY_CALLS_LOG"

  # adapter_deploy (complet) ne doit PAS avoir été appelé
  ! grep -q "adapter_deploy called" "$DEPLOY_CALLS_LOG"
}
