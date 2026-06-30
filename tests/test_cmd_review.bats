#!/usr/bin/env bats
# Tests pour scripts/cmd-review.sh
# cmd-review.sh est un script top-level (non sourceable) — testé via exécution directe.
# adapter_start fait exec → on mock opencode comme un script PATH.

setup() {
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # Isoler HUB_CONFIG — langue française pour que resolve_oc_lang → OC_LANG=fr
  export HUB_CONFIG="$TEST_DIR/hub.json"
  echo '{"cli":{"language":"fr"}}' > "$HUB_CONFIG"

  CMD_REVIEW="$BATS_TEST_DIRNAME/../scripts/cmd-review.sh"

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

  # Projet avec sélection d'agents restrictive
  cat >> "$PROJECTS_FILE" <<'PROJEOF'

## TEST-RESTRICTED
- Nom : Projet Restricted
- Stack : Node.js
- Tracker : none
- Agents : orchestrator,planner
PROJEOF

  # Projet avec branche de base "develop"
  cat >> "$PROJECTS_FILE" <<'PROJEOF'

## TEST-DEVELOP-BASE
- Nom : Projet Develop Base
- Stack : Node.js
- Tracker : none
- Agents : all
- Worktree base branch : develop
PROJEOF

  mkdir -p "$TEST_DIR/fake-project"
  mkdir -p "$TEST_DIR/fake-project/.opencode/agents"
  # Créer un reviewer.md factice pour éviter le prompt de déploiement
  touch "$TEST_DIR/fake-project/.opencode/agents/reviewer.md"
  mkdir -p "$TEST_DIR/fake-project-restricted"
  mkdir -p "$TEST_DIR/fake-project-restricted/.opencode/agents"
  touch "$TEST_DIR/fake-project-restricted/.opencode/agents/reviewer.md"
  # Projet avec branche de base "develop" (pour tester la lecture de projects.md)
  mkdir -p "$TEST_DIR/fake-project-develop"
  mkdir -p "$TEST_DIR/fake-project-develop/.opencode/agents"
  touch "$TEST_DIR/fake-project-develop/.opencode/agents/reviewer.md"
  cat > "$PATHS_FILE" <<EOF
TEST-PROJ=$TEST_DIR/fake-project
TEST-RESTRICTED=$TEST_DIR/fake-project-restricted
TEST-DEVELOP-BASE=$TEST_DIR/fake-project-develop
EOF

  : > "$API_KEYS_FILE"

  # ── Mock git dans le PATH ─────────────────────────────────────────────────────
  GIT_CALLS_LOG="$TEST_DIR/git_calls.log"
  export GIT_CALLS_LOG
  : > "$GIT_CALLS_LOG"

  REAL_GIT="$(command -v git)"
  export REAL_GIT

  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
# Simuler "branch --show-current" → retourner "feature/my-branch"
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "branch" ] && [ "${4:-}" = "--show-current" ]; then
  echo "feature/my-branch"
  exit 0
fi
# Simuler "fetch" → succès par défaut
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "fetch" ]; then
  exit 0
fi
# Simuler "pull" → succès par défaut
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "pull" ]; then
  exit 0
fi
# Simuler "diff main...feature/my-branch" → diff non vide
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "diff" ]; then
  echo "+ added line"
  exit 0
fi
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  # ── Mock opencode dans le PATH ────────────────────────────────────────────────
  OPENCODE_LOG="$TEST_DIR/opencode_calls.log"
  export OPENCODE_LOG
  : > "$OPENCODE_LOG"

  cat > "$TEST_DIR/bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*" >> "$OPENCODE_LOG"
exit 0
OCEOF
  chmod +x "$TEST_DIR/bin/opencode"

  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  unset HUB_CONFIG
  rm -rf "$TEST_DIR"
}

# ── Détection automatique de la branche courante ──────────────────────────────

@test "cmd-review : détecte automatiquement la branche courante et affiche le bloc intro" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"feature/my-branch"* ]]
}

@test "cmd-review : accepte --branch et utilise la branche fournie" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch my-feature
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"my-feature"* ]]
}

@test "cmd-review : affiche le bloc intro avec Chemin, Branche, Agent" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Chemin"* ]]
  [[ "$output" == *"Branche"* ]]
  [[ "$output" == *"Agent"* ]]
}

@test "cmd-review : lance opencode avec --agent reviewer" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "opencode" "$OPENCODE_LOG"
  grep -q "\-\-agent reviewer" "$OPENCODE_LOG"
}

@test "cmd-review : affiche la confirmation avant lancement" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Lancement de la review"* ]]
}

# ── Sélection interactive si pas d'ID ────────────────────────────────────────

@test "cmd-review : exit si PROJECT_ID invalide" {
  run bash -c '
    printf "\n" | bash "$1" --project INEXISTANT --branch main
  ' _ "$CMD_REVIEW"
  [ "$status" -ne 0 ]
}

# ── Vérification sélection restrictive ───────────────────────────────────────

@test "cmd-review : avertit si reviewer absent de la sélection projet" {
  run bash -c '
    # Y = ajouter reviewer, n = ne pas redéployer, Enter = gate
    printf "Y\nn\n\n" | bash "$1" --project TEST-RESTRICTED --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"reviewer"* ]]
}

@test "cmd-review : continue même si refus d'ajout de reviewer dans la sélection" {
  # Répondre n = refus d'ajout, Enter = gate
  run bash -c '
    printf "n\n\n" | bash "$1" --project TEST-RESTRICTED --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  # opencode doit quand même être appelé
  [ -s "$OPENCODE_LOG" ]
}

# ── Argument --branch avec PROJECT_ID ────────────────────────────────────────

@test "cmd-review : --branch avant PROJECT_ID est accepté" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch release/v2.0
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  [[ "$output" == *"release/v2.0"* ]]
}

@test "cmd-review : passe --prompt à opencode avec les instructions de review" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "\-\-prompt" "$OPENCODE_LOG"
}

@test "cmd-review : le prompt contient la commande git diff exacte" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "git diff main...feat/test" "$OPENCODE_LOG"
}

@test "cmd-review : le prompt utilise la branche de base du projet (develop)" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-DEVELOP-BASE --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "git diff develop...feat/test" "$OPENCODE_LOG"
}

# ── Synchronisation git (fetch + pull) ────────────────────────────────────────

@test "cmd-review : exécute git fetch avant le diff" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "fetch" "$GIT_CALLS_LOG"
}

@test "cmd-review : exécute git pull --ff-only origin main (branche de base par défaut)" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "pull --ff-only origin main" "$GIT_CALLS_LOG"
}

@test "cmd-review : utilise la branche de base depuis projects.md (develop)" {
  run bash -c '
    printf "\n" | bash "$1" --project TEST-DEVELOP-BASE --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  grep -q "pull --ff-only origin develop" "$GIT_CALLS_LOG"
}

@test "cmd-review : propose confirmation si fetch échoue" {
  # Remplacer le mock git par un qui fait échouer fetch
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "branch" ] && [ "${4:-}" = "--show-current" ]; then
  echo "feature/my-branch"; exit 0
fi
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "fetch" ]; then
  exit 1
fi
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "diff" ]; then
  echo "+ added line"; exit 0
fi
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  # Répondre Y au prompt de confirmation, puis Enter pour le gate de lancement
  run bash -c '
    printf "Y\n\n" | bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  # L'avertissement de fetch échoué doit apparaître
  [[ "$output" == *"Fetch"* ]]
  # opencode doit quand même avoir été lancé (Y = continuer)
  [ -s "$OPENCODE_LOG" ]
}

@test "cmd-review : annule si sync échoue et utilisateur refuse" {
  # Mock git avec fetch qui échoue
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "branch" ] && [ "${4:-}" = "--show-current" ]; then
  echo "feature/my-branch"; exit 0
fi
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "fetch" ]; then
  exit 1
fi
if [ "${1:-}" = "-C" ] && [ "${3:-}" = "diff" ]; then
  echo "+ added line"; exit 0
fi
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  # Répondre n au prompt → annulation
  # OC_NON_INTERACTIVE doit être 0 pour que _prompt lise le stdin pipe (read -t 1)
  # Sinon _prompt retourne "" et on continue par défaut (comportement non-interactif)
  run bash -c '
    OC_NON_INTERACTIVE=0 printf "n\n" | OC_NON_INTERACTIVE=0 bash "$1" --project TEST-PROJ --branch feat/test
  ' _ "$CMD_REVIEW"
  [ "$status" -eq 0 ]
  # opencode NE doit PAS avoir été appelé
  [ ! -s "$OPENCODE_LOG" ]
}
