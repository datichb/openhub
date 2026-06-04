#!/usr/bin/env bats
# Tests pour scripts/cmd-worktree.sh
# Fonctions testées : _cmd_list, _cmd_create, _cmd_remove, _cmd_cleanup, _cmd_status
# cmd-worktree.sh est un script top-level testé via exécution directe.

setup() {
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  echo '{"cli":{"language":"fr"}}' > "$HUB_CONFIG"

  export OC_NON_INTERACTIVE=1

  CMD_WORKTREE="$BATS_TEST_DIRNAME/../scripts/cmd-worktree.sh"

  # Projet de test
  cat > "$PROJECTS_FILE" <<'PROJEOF'
## WT-PROJ
- Nom : Projet Worktree
- Stack : TypeScript
- Tracker : none
- Worktree : enabled
- Worktree auto cleanup : false
- Worktree base branch : main
PROJEOF

  FAKE_PROJECT="$TEST_DIR/fake-project"
  mkdir -p "$FAKE_PROJECT/.git/info"
  touch "$FAKE_PROJECT/.git/info/exclude"
  mkdir -p "$FAKE_PROJECT/.git/refs/heads"

  cat > "$PATHS_FILE" <<EOF
WT-PROJ=$FAKE_PROJECT
EOF

  : > "$API_KEYS_FILE"

  # Logs
  GIT_CALLS_LOG="$TEST_DIR/git_calls.log"
  export GIT_CALLS_LOG
  : > "$GIT_CALLS_LOG"

  mkdir -p "$TEST_DIR/bin"

  # Mock git
  REAL_GIT="$(command -v git)"
  export REAL_GIT
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
case "${1:-}" in
  "worktree")
    case "${2:-}" in
      "add")
        # Créer le répertoire simulant le worktree
        TARGET="${5:-${4:-${3:-}}}"
        if [ -n "$TARGET" ]; then
          mkdir -p "$TARGET"
        fi
        exit 0 ;;
      "remove")
        TARGET="${4:-${3:-}}"
        rm -rf "$TARGET" 2>/dev/null || true
        exit 0 ;;
      "list"|"prune") exit 0 ;;
    esac ;;
  "rev-parse")
    [ "${2:-}" = "--verify" ] && exit 1
    [ "${2:-}" = "--abbrev-ref" ] && echo "main" && exit 0
    exit 0 ;;
  "branch")
    [ "${2:-}" = "--merged" ] && echo "  main" && exit 0
    exit 0 ;;
esac
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  unset HUB_CONFIG
  rm -rf "$TEST_DIR"
}

# ── oc worktree help ───────────────────────────────────────────────────────────

@test "cmd-worktree : affiche l'aide sans arguments" {
  run bash "$CMD_WORKTREE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc worktree"* ]]
}

@test "cmd-worktree : affiche l'aide avec help" {
  run bash "$CMD_WORKTREE" help
  [ "$status" -eq 0 ]
  [[ "$output" == *"list"* ]]
  [[ "$output" == *"create"* ]]
  [[ "$output" == *"remove"* ]]
}

@test "cmd-worktree : sous-commande inconnue → exit 1" {
  run bash "$CMD_WORKTREE" unknowncmd WT-PROJ
  [ "$status" -ne 0 ]
}

# ── oc worktree list ───────────────────────────────────────────────────────────

@test "cmd-worktree list : affiche message si aucun worktree" {
  run bash "$CMD_WORKTREE" list WT-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"aucun worktree"* ]]
}

@test "cmd-worktree list : liste les worktrees existants" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42/.git"
  # Mock git rev-parse pour le worktree
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
case "${1:-}" in
  "rev-parse")
    [ "${2:-}" = "--abbrev-ref" ] && echo "feat/bd-42" && exit 0
    exit 0 ;;
  "branch")
    [ "${2:-}" = "--merged" ] && echo "  main" && exit 0
    exit 0 ;;
esac
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  run bash "$CMD_WORKTREE" list WT-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"feat-bd-42"* ]]
}

@test "cmd-worktree list : erreur si projet inconnu" {
  run bash "$CMD_WORKTREE" list INEXISTANT
  [ "$status" -ne 0 ]
}

# ── oc worktree create ─────────────────────────────────────────────────────────

@test "cmd-worktree create : crée le worktree pour la branche donnée" {
  run bash "$CMD_WORKTREE" create feat/test WT-PROJ
  [ "$status" -eq 0 ]
  # Le répertoire worktree a été créé (via le mock git qui fait mkdir -p)
  [ -d "$FAKE_PROJECT/.worktrees/feat-test" ]
}

@test "cmd-worktree create : erreur si BRANCH manquant" {
  run bash "$CMD_WORKTREE" create
  [ "$status" -ne 0 ]
  [[ "$output" == *"Usage"* ]] || [[ "$output" == *"BRANCH"* ]]
}

@test "cmd-worktree create : erreur si worktree déjà existant" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-existing"
  run bash "$CMD_WORKTREE" create feat/existing WT-PROJ
  [ "$status" -ne 0 ]
}

@test "cmd-worktree create : erreur si projet inconnu" {
  run bash "$CMD_WORKTREE" create feat/test INEXISTANT
  [ "$status" -ne 0 ]
}

# ── oc worktree remove ─────────────────────────────────────────────────────────

@test "cmd-worktree remove : supprime le worktree existant (non-interactif)" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-to-remove"
  run bash "$CMD_WORKTREE" remove feat/to-remove WT-PROJ
  [ "$status" -eq 0 ]
  # En mode non-interactif, OC_NON_INTERACTIVE=1 → _prompt retourne vide
  # ${_confirm:-Y} → "Y" → confirmation acceptée → worktree supprimé (git mock le rm)
  [ ! -d "$FAKE_PROJECT/.worktrees/feat-to-remove" ]
}

@test "cmd-worktree remove : erreur si BRANCH manquant" {
  run bash "$CMD_WORKTREE" remove
  [ "$status" -ne 0 ]
}

@test "cmd-worktree remove : réussit (avertissement) si worktree inexistant" {
  run bash "$CMD_WORKTREE" remove feat/inexistant WT-PROJ
  [ "$status" -eq 0 ]
}

# ── oc worktree cleanup ────────────────────────────────────────────────────────

@test "cmd-worktree cleanup : ne fait rien si aucun worktree" {
  run bash "$CMD_WORKTREE" cleanup WT-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"nettoyer"* ]] || [[ "$output" == *"Nettoyage"* ]]
}

@test "cmd-worktree cleanup : erreur si projet inconnu" {
  run bash "$CMD_WORKTREE" cleanup INEXISTANT
  [ "$status" -ne 0 ]
}

# ── oc worktree status ─────────────────────────────────────────────────────────

@test "cmd-worktree status : affiche le résumé du projet" {
  run bash "$CMD_WORKTREE" status WT-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"Worktree"* ]]
}

@test "cmd-worktree status : affiche worktree enabled" {
  run bash "$CMD_WORKTREE" status WT-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"enabled"* ]]
}

@test "cmd-worktree status : erreur si projet inconnu" {
  run bash "$CMD_WORKTREE" status INEXISTANT
  [ "$status" -ne 0 ]
}
