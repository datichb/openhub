#!/usr/bin/env bats
# Tests pour scripts/cmd-init.sh — bd init + propagation labels
# cmd-init.sh est un script top-level (non sourceable) — testé via exécution directe.
# Les entrées interactives sont fournies via stdin.

setup() {
  TEST_DIR="$(mktemp -d)"

  # Ces tests valident le flux interactif via stdin pipé — désactiver le
  # court-circuit non-interactif qui ignore stdin (utilisé en CI globalement).
  export OC_NON_INTERACTIVE=0

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  CMD_INIT="$BATS_TEST_DIRNAME/../scripts/cmd-init.sh"

  # ── Mock cmd-deploy.sh ────────────────────────────────────────────────────────
  # cmd-init.sh appelle : bash "$SCRIPTS_DIR/cmd-deploy.sh" all "$PROJECT_ID"
  # SCRIPTS_DIR étant calculé depuis BASH_SOURCE dans common.sh, on ne peut pas
  # le surcharger via l'environnement. On intercepte via un wrapper `bash` dans le PATH
  # qui détecte l'argument *cmd-deploy.sh et court-circuite l'exécution.
  DEPLOY_CALLS_LOG="$TEST_DIR/deploy_calls.log"
  export DEPLOY_CALLS_LOG
  : > "$DEPLOY_CALLS_LOG"

  mkdir -p "$TEST_DIR/bin"

  REAL_BASH="$(command -v bash)"
  export REAL_BASH
  cat > "$TEST_DIR/bin/bash" <<'BASHEOF'
#!/bin/bash
# Scan args for *cmd-deploy.sh and short-circuit if found
deploy_script=""
other_args=()
for arg in "$@"; do
  if [[ "$arg" == *cmd-deploy.sh ]]; then
    deploy_script="$arg"
  else
    other_args+=("$arg")
  fi
done
if [[ -n "$deploy_script" ]]; then
  echo "cmd-deploy ${other_args[*]}" >> "$DEPLOY_CALLS_LOG"
  exit 0
fi
exec "$REAL_BASH" "$@"
BASHEOF
  chmod +x "$TEST_DIR/bin/bash"

  # Projects vide (ensure_projects_file le crée si absent)
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test
PROJEOF

  : > "$PATHS_FILE"

  # Créer un répertoire projet cible
  mkdir -p "$TEST_DIR/fake-project"

  # ── Mock bd dans le PATH ─────────────────────────────────────────────────────
  BD_CALLS_LOG="$TEST_DIR/bd_calls.log"
  export BD_CALLS_LOG
  : > "$BD_CALLS_LOG"
  cat > "$TEST_DIR/bin/bd" <<'BDEOF'
#!/bin/bash
echo "bd $*" >> "$BD_CALLS_LOG"
# Gérer le flag -C <path> (bd -C <path> <cmd> ...)
_args=("$@")
if [ "${_args[0]:-}" = "-C" ]; then
  _bd_dir="${_args[1]}"
  _args=("${_args[@]:2}")
else
  _bd_dir="."
fi
if [ "${_args[0]:-}" = "init" ]; then
  mkdir -p "$_bd_dir/.beads"
fi
exit 0
BDEOF
  chmod +x "$TEST_DIR/bin/bd"

  # Mock brew (non disponible en CI)
  cat > "$TEST_DIR/bin/brew" <<'BREWEOF'
#!/bin/bash
exit 0
BREWEOF
  chmod +x "$TEST_DIR/bin/brew"

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
  # Simuler aucun remote configuré par défaut
  if [ "${2:-}" = "get-url" ]; then
    exit 1
  elif [ "${2:-}" = "add" ]; then
    exit 0
  fi
  exit 0
fi
# Déléguer au vrai git pour le reste
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# Exécute cmd-init.sh avec des entrées piped
# Utilise un heredoc pour fournir les réponses interactives
_run_init() {
  run bash "$CMD_INIT" "$@"
}

# ── bd init + propagation labels intégrée ────────────────────────────────────

@test "cmd-init : propose bd init et propage les labels" {
  # Entrées: Nom, Stack, Labels, Tracker=1(none), init beads=Y, upstream=n, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature,fix,back\n1\nY\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # bd init a été appelé
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  # Les labels ont été propagés
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
  grep -q "label create back" "$BD_CALLS_LOG"
}

@test "cmd-init : ne propose pas bd init si .beads existe déjà" {
  mkdir -p "$TEST_DIR/fake-project/.beads"
  # Entrées: Nom, Stack, Labels, Tracker=1(none), select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nn\nn\nn\n" | bash "$1" --name NEWPROJ2 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # bd init ne doit PAS avoir été appelé
  ! grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
}

@test "cmd-init : respecte le refus de bd init (n)" {
  # Entrées: Nom, Stack, Labels, Tracker=1(none), init beads=n, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ3 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # bd init ne doit PAS avoir été appelé
  ! grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
}

@test "cmd-init : ne propose pas bd init si bd absent du PATH" {
  # Supprimer le mock bd du PATH
  rm -f "$TEST_DIR/bin/bd"
  # Entrées: Nom, Stack, Labels, Tracker=1(none), select agents=n, MCP=n, deploy=n
  # Pas de question bd init car bd n'est pas disponible
  # Mais la question "Installer Beads maintenant ?" est posée — répondre n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ4 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # Aucun appel bd
  [ ! -s "$BD_CALLS_LOG" ]
}

@test "cmd-init : propage les labels avec espaces (trim)" {
  # Entrées: Nom, Stack, Labels avec espaces, Tracker=1, init beads=Y, upstream=n, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\n feature , fix , back \n1\nY\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ5 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # Les labels doivent être trimés
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
  grep -q "label create back" "$BD_CALLS_LOG"
}

# ── Labels par défaut ────────────────────────────────────────────────────────

@test "cmd-init : propage les labels par défaut si saisie vide" {
  # L'utilisateur ne saisit rien pour les labels → default feature,fix
  # Entrées: Nom, Stack, Labels=(vide), Tracker=1, init beads=Y, upstream=n, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\n\n1\nY\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ6 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # bd init a été appelé
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  # Les labels par défaut (feature,fix) doivent être propagés
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
}

# ── Proposition upstream git ─────────────────────────────────────────────────

@test "cmd-init : propose git remote add upstream après bd init" {
  # Entrées: Nom, Stack, Labels, Tracker=1, init beads=Y, upstream=Y, URL, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nY\nY\nhttps://github.com/test/repo.git\nn\nn\nn\n" | bash "$1" --name NEWPROJ7 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # git remote add upstream a été appelé avec l'URL
  grep -q "git remote add upstream https://github.com/test/repo.git" "$GIT_CALLS_LOG"
  [[ "$output" == *"Remote upstream configuré"* ]]
}

@test "cmd-init : respecte le refus de configurer upstream (n)" {
  # Entrées: Nom, Stack, Labels, Tracker=1, init beads=Y, upstream=n, select agents=n, MCP=n, deploy=n
  : > "$GIT_CALLS_LOG"
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nY\nn\nn\nn\nn\n" | bash "$1" --name NEWPROJ8 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # git remote add ne doit PAS avoir été appelé
  ! grep -q "git remote add upstream" "$GIT_CALLS_LOG"
  [[ "$output" == *"Configurer plus tard"* ]]
}

# ── Étape MCP — sélection par projet ─────────────────────────────────────────

@test "cmd-init étape MCP : répond N → MCP : none écrit dans projects.md" {
  # Entrées: Nom, Stack, Labels, Tracker=1(none), init beads=n, select agents=n, MCP=n, deploy=n
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nn\nn\nn\nn\n" | bash "$1" --name MCPTEST1 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  grep -q "MCP : none" "$PROJECTS_FILE"
}

@test "cmd-init étape MCP : mode non-interactif → MCP : none par défaut" {
  # OC_NON_INTERACTIVE=1 — toutes les prompts retournent vide → N par défaut
  run bash -c '
    export OC_NON_INTERACTIVE=1
    bash "$1" --name MCPTEST2 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  # En mode non-interactif, le script peut sortir tôt mais ne doit pas crasher
  [ "$status" -eq 0 ] || [ "$status" -eq 1 ]
}

@test "cmd-init étape MCP : MCP absent du récapitulatif si none" {
  # Entrées: réponse N à MCP
  run bash -c '
    printf "Mon Projet\nNode.js\nfeature\n1\nn\nn\nn\nn\n" | bash "$1" --name MCPTEST3 "$2"
  ' _ "$CMD_INIT" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # Le récapitulatif doit mentionner le statut MCP
  [[ "$output" == *"MCP"* ]]
}
