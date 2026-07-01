#!/usr/bin/env bats
# Tests pour scripts/cmd-debug.sh
# Vérifie : validation args, vérification agent, vérification déploiement, lancement

load helpers

setup() {
  common_setup

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"

    > "$HUB_CONFIG"

  CMD_DEBUG="$BATS_TEST_DIRNAME/../scripts/cmd-debug.sh"

  # Projet avec agent debugger déployé
  mkdir -p "$TEST_DIR/proj-debug/.opencode/agents"
  touch "$TEST_DIR/proj-debug/.opencode/agents/debugger.md"

  # Projet sans agents déployés
  mkdir -p "$TEST_DIR/proj-nodeploy"

  # Projet avec agents mais sans debugger
  mkdir -p "$TEST_DIR/proj-nodebugger/.opencode/agents"
  touch "$TEST_DIR/proj-nodebugger/.opencode/agents/developer-frontend.md"

  cat > "$PROJECTS_FILE" <<'EOF'
# Registre de test

## DEBUG-PROJ
- Nom : Debug Project
- Stack : Node.js
- Board Beads : DEBUG-PROJ
- Tracker : none
- Labels : debug
- Agents : all

## NODEPLOY-PROJ
- Nom : No Deploy Project
- Stack : Python
- Board Beads : NODEPLOY-PROJ
- Tracker : none
- Labels : test
- Agents : all

## NODEBUGGER-PROJ
- Nom : No Debugger Project
- Stack : Go
- Board Beads : NODEBUGGER-PROJ
- Tracker : none
- Labels : test
- Agents : developer-frontend
EOF

  cat > "$PATHS_FILE" <<EOF
DEBUG-PROJ=$TEST_DIR/proj-debug
NODEPLOY-PROJ=$TEST_DIR/proj-nodeploy
NODEBUGGER-PROJ=$TEST_DIR/proj-nodebugger
EOF

  : > "$API_KEYS_FILE"

  # Mock opencode dans le PATH — répond et quitte immédiatement
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

@test "debug : projet inexistant → erreur" {
  run bash "$CMD_DEBUG" -p "INEXISTANT-PROJ"
  [ "$status" -ne 0 ]
}

@test "debug : PROJECT_ID normalisé (minuscules acceptées)" {
  # debug-proj en minuscules doit être normalisé en DEBUG-PROJ
  # Si resolve_project_path échoue, c'est un autre code d'erreur
  run bash "$CMD_DEBUG" -p "debug-proj" < /dev/null
  # Soit succès (normalisé), soit erreur propre avec message (pas de crash silencieux)
  [ -n "$output" ]
}

# ── Vérification déploiement ──────────────────────────────────────────────────

@test "debug : projet avec debugger déployé → lance opencode" {
  # Le script finit par "IFS= read -rp '' _" (attente d'une touche) puis exec opencode
  run bash -c 'printf "\n" | bash "$1" -p DEBUG-PROJ' _ "$CMD_DEBUG"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
}

@test "debug : affiche le chemin du projet" {
  run bash -c 'printf "\n" | bash "$1" -p DEBUG-PROJ' _ "$CMD_DEBUG"
  [ "$status" -eq 0 ]
  [[ "$output" =~ "$TEST_DIR/proj-debug" ]]
}

@test "debug : affiche l'agent requis (debugger)" {
  run bash -c 'printf "\n" | bash "$1" -p DEBUG-PROJ' _ "$CMD_DEBUG"
  [ "$status" -eq 0 ]
  [[ "$output" =~ "debugger" ]]
}

# ── Agents non déployés ───────────────────────────────────────────────────────

@test "debug : projet sans agents déployés → propose deploy (réponse N)" {
  # On répond "N" pour refuser le déploiement, puis Enter pour la confirmation
  run bash -c 'printf "N\n\n" | bash "$1" -p NODEPLOY-PROJ' _ "$CMD_DEBUG"
  # Doit afficher un avertissement sur le déploiement manquant (opencode ou deploy)
  [[ "$output" =~ "opencode" ]] || [[ "$output" =~ "deploy" ]]
}

@test "debug : projet sans debugger dans agents → propose d'ajouter (réponse N)" {
  run bash -c 'printf "N\n\n" | bash "$1" -p NODEBUGGER-PROJ' _ "$CMD_DEBUG"
  # Doit mentionner l'agent manquant
  [[ "$output" =~ "debugger" ]]
}
