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
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  # Les labels ont été propagés
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
}

@test "cmd-start : respecte le refus de bd init (n)" {
  # Pas de .beads → question → n, puis Enter gate
  run bash -c '
    printf "n\n\n" | bash "$1" TEST-PROJ
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]

  # bd init ne doit PAS avoir été appelé
  ! grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
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
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
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
adapter_deploy_skills() { echo "adapter_deploy_skills called" >> "$DEPLOY_CALLS_LOG"; }
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

# ── _inject_context_instructions : mise à jour de opencode.json ──────────────

@test "cmd-start : _inject_context_instructions injecte ONBOARDING.md si présent" {
  command -v jq &>/dev/null || skip "jq non disponible"

  # Préparer un projet avec opencode.json existant et ONBOARDING.md
  local proj="$TEST_DIR/fake-project"
  mkdir -p "$proj/.beads"
  echo '{"$schema":"https://opencode.ai/config.json","model":"claude-sonnet-4"}' > "$proj/opencode.json"
  touch "$proj/ONBOARDING.md"

  # Sourcer context-cache.sh et appeler la fonction directement
  source "$BATS_TEST_DIRNAME/../scripts/lib/context-cache.sh"
  cache_exists() { return 1; }  # Pas de cache
  _inject_context_instructions "$proj"

  run jq -r '.instructions[]' "$proj/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ONBOARDING.md"* ]]
}

@test "cmd-start : _inject_context_instructions injecte context.json si cache valide" {
  command -v jq &>/dev/null || skip "jq non disponible"

  local proj="$TEST_DIR/fake-project"
  mkdir -p "$proj/.opencode" "$proj/.beads"
  echo '{"$schema":"https://opencode.ai/config.json","model":"claude-sonnet-4"}' > "$proj/opencode.json"
  echo '{"version":"1.0","generated_at":"2026-01-01T00:00:00Z","stack":{},"conventions":{},"key_files":{}}' > "$proj/.opencode/context.json"
  touch "$proj/ONBOARDING.md"  # présent mais le cache doit avoir la priorité

  source "$BATS_TEST_DIRNAME/../scripts/lib/context-cache.sh"
  # Mock validate_context_cache pour retourner succès
  validate_context_cache() { return 0; }
  _inject_context_instructions "$proj"

  run jq -r '.instructions[]' "$proj/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *".opencode/context.json"* ]]
  [[ "$output" != *"ONBOARDING.md"* ]]
}

@test "cmd-start : _inject_context_instructions supprime instructions si aucun contexte" {
  command -v jq &>/dev/null || skip "jq non disponible"

  local proj="$TEST_DIR/fake-project"
  mkdir -p "$proj/.beads"
  # opencode.json avec instructions préexistantes
  echo '{"$schema":"https://opencode.ai/config.json","model":"claude-sonnet-4","instructions":["OLD.md"]}' > "$proj/opencode.json"
  # Aucun fichier contexte, pas de cache

  source "$BATS_TEST_DIRNAME/../scripts/lib/context-cache.sh"
  cache_exists() { return 1; }
  _inject_context_instructions "$proj"

  run jq 'has("instructions")' "$proj/opencode.json"
  [ "$output" = "false" ]
}

# ── Mode --worktree ───────────────────────────────────────────────────────────

@test "cmd-start : --parallel et --onboard sont mutuellement exclusifs" {
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ --parallel --onboard
  ' _ "$CMD_START"
  [ "$status" -ne 0 ]
  [[ "$output" == *"mutuellement exclusifs"* ]]
}

@test "cmd-start : --parallel et --worktree sont mutuellement exclusifs" {
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ --parallel --worktree feat/test
  ' _ "$CMD_START"
  [ "$status" -ne 0 ]
  [[ "$output" == *"mutuellement exclusifs"* ]]
}

@test "cmd-start : --dev et --parallel sont mutuellement exclusifs" {
  mkdir -p "$TEST_DIR/fake-project/.beads"
  run bash -c '
    printf "\n" | bash "$1" TEST-PROJ --dev --parallel
  ' _ "$CMD_START"
  [ "$status" -ne 0 ]
  [[ "$output" == *"mutuellement exclusifs"* ]]
}

@test "cmd-start : --worktree échoue si BRANCH vide en mode non-interactif" {
  mkdir -p "$TEST_DIR/fake-project/.beads"
  mkdir -p "$TEST_DIR/fake-project/.opencode/agents"
  # OC_NON_INTERACTIVE=1 → _prompt retourne vide → WORKTREE_BRANCH reste vide
  run bash -c '
    export OC_NON_INTERACTIVE=1
    printf "\n" | bash "$1" TEST-PROJ --worktree
  ' _ "$CMD_START"
  # Doit échouer car pas de branche fournie et non-interactif
  [ "$status" -ne 0 ]
  [[ "$output" == *"requis"* ]] || [[ "$output" == *"branche"* ]]
}

@test "cmd-start : --worktree avec branche crée le worktree et lance opencode" {
  mkdir -p "$TEST_DIR/fake-project/.beads"
  mkdir -p "$TEST_DIR/fake-project/.opencode/agents"
  # Requis par _worktree_require_git qui vérifie l'existence de .git/
  mkdir -p "$TEST_DIR/fake-project/.git/info"
  touch "$TEST_DIR/fake-project/.git/info/exclude"

  # Mock git pour worktree add
  cat > "$TEST_DIR/bin/git" <<'GITEOF'
#!/bin/bash
echo "git $*" >> "$GIT_CALLS_LOG"
case "${1:-}" in
  "worktree")
    case "${2:-}" in
      "add")
        TARGET="${5:-${4:-${3:-}}}"
        [ -n "$TARGET" ] && mkdir -p "$TARGET"
        exit 0 ;;
      *) exit 0 ;;
    esac ;;
  "rev-parse")
    [ "${2:-}" = "--verify" ] && exit 1
    [ "${2:-}" = "--abbrev-ref" ] && echo "main" && exit 0
    exit 0 ;;
  "branch") echo "  main"; exit 0 ;;
  "remote") exit 1 ;;
esac
exec "$REAL_GIT" "$@"
GITEOF
  chmod +x "$TEST_DIR/bin/git"

  run bash -c '
    export OC_NON_INTERACTIVE=1
    printf "\n" | bash "$1" TEST-PROJ --worktree feat/test-feature
  ' _ "$CMD_START"
  [ "$status" -eq 0 ]
  [[ "$output" == *"worktree"* ]]
}

# ── _build_session_title ──────────────────────────────────────────────────────
# _build_session_title est définie dans scripts/lib/session-title.sh (sourceable).

_bt() {
  # Helper : exécute _build_session_title dans un sous-shell propre
  bash -c "
    export HUB_CONFIG='$HUB_CONFIG'
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    resolve_oc_lang
    source '$BATS_TEST_DIRNAME/../scripts/lib/session-title.sh'
    _build_session_title $*
  "
}

@test "_build_session_title : prompt simple extrait les mots significatifs" {
  run _bt "false false false 'je veux ajouter le nommage des sessions' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"nommage"* ]]
  [[ "$output" == *"sessions"* ]]
  [[ "$output" != *" je "* ]]
  [[ "$output" != *" veux "* ]]
}

@test "_build_session_title : stop-words FR supprimés" {
  run _bt "false false false 'je veux que tu fasses une feature de cache' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" != *" je "* ]]
  [[ "$output" != *" veux "* ]]
  [[ "$output" != *" une "* ]]
  [[ "$output" == *"feature"* ]]
  [[ "$output" == *"cache"* ]]
}

@test "_build_session_title : stop-words EN supprimés" {
  run _bt "false false false 'fix the broken login page with a new handler' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" != *" the "* ]]
  [[ "$output" != *" a "* ]]
  [[ "$output" != *" with "* ]]
  [[ "$output" == *"fix"* ]]
  [[ "$output" == *"broken"* ]]
  [[ "$output" == *"login"* ]]
}

@test "_build_session_title : référence ticket préfixée en premier" {
  run _bt "false false false 'fix CP-3 crash on login' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CP-3"* ]]
  local pos_ticket pos_other
  pos_ticket=$(echo "$output" | awk '{print index($0,"CP-3")}')
  pos_other=$(echo "$output" | awk '{print index($0,"crash")}')
  [ "$pos_ticket" -lt "$pos_other" ]
}

@test "_build_session_title : mode onboard retourne onboard: PROJECT_ID" {
  run _bt "false true false '' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"onboard"*"MYPROJ"* ]]
}

@test "_build_session_title : mode parallel retourne parallel: BRANCH" {
  run _bt "false false true '' 'MYPROJ' 'feat/my-feature'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"parallel"*"feat/my-feature"* ]]
}

@test "_build_session_title : mode dev avec contexte ticket" {
  run _bt "true false false '' 'MYPROJ' '' 'CP-5' 'Fix auth bug on login'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"dev"* ]]
  [[ "$output" == *"CP-5"* ]]
}

@test "_build_session_title : prompt vide retourne PROJECT_ID et date" {
  run _bt "false false false '' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MYPROJ"* ]]
  [[ "$output" =~ [0-9]{4}-[0-9]{2}-[0-9]{2} ]]
}

@test "_build_session_title : titre tronqué à 50 caractères max" {
  run _bt "false false false 'implement complex authentication system with oauth tokens refresh mechanism' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [ "${#output}" -le 52 ]
}

@test "_build_session_title : prompt mixte FR/EN filtre les deux langues" {
  run _bt "false false false 'je want to fix the bug de cache' 'MYPROJ'"
  [ "$status" -eq 0 ]
  [[ "$output" != *" je "* ]]
  [[ "$output" != *" the "* ]]
  [[ "$output" == *"fix"* ]]
  [[ "$output" == *"bug"* ]]
  [[ "$output" == *"cache"* ]]
}
# ── --resume ──────────────────────────────────────────────────────────────────

@test "cmd-start --resume : sans PROJECT_ID affiche erreur et exit 1" {
  run bash "$CMD_START" --resume
  [ "$status" -eq 1 ]
  [[ "$output" == *"PROJECT_ID"* ]] || [[ "$output" == *"requis"* ]] || [[ "$output" == *"requires"* ]]
}

@test "cmd-start --resume : incompatible avec --dev exit 1" {
  run bash "$CMD_START" --resume --dev -p TEST-PROJ
  [ "$status" -eq 1 ]
  [[ "$output" == *"incompatible"* ]] || [[ "$output" == *"exclusive"* ]]
}

@test "cmd-start --resume : incompatible avec --onboard exit 1" {
  run bash "$CMD_START" --resume --onboard -p TEST-PROJ
  [ "$status" -eq 1 ]
  [[ "$output" == *"incompatible"* ]] || [[ "$output" == *"exclusive"* ]]
}

@test "cmd-start --resume : aucune session trouvée affiche suggestion" {
  # Base SQLite vide
  export _OCDB_FILE="$TEST_DIR/empty.db"
  sqlite3 "$TEST_DIR/empty.db" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY, project_id TEXT NOT NULL DEFAULT 'p',
      parent_id TEXT, slug TEXT NOT NULL DEFAULT 's',
      directory TEXT NOT NULL DEFAULT '/d', title TEXT NOT NULL DEFAULT 't',
      version TEXT NOT NULL DEFAULT '1', share_url TEXT,
      cost REAL DEFAULT 0, tokens_input INTEGER DEFAULT 0,
      tokens_output INTEGER DEFAULT 0, tokens_reasoning INTEGER DEFAULT 0,
      tokens_cache_read INTEGER DEFAULT 0, tokens_cache_write INTEGER DEFAULT 0,
      agent TEXT, model TEXT, metadata TEXT,
      time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL
    );
    CREATE TABLE part (
      id TEXT PRIMARY KEY, message_id TEXT NOT NULL DEFAULT 'msg',
      session_id TEXT NOT NULL, time_created INTEGER NOT NULL,
      time_updated INTEGER NOT NULL, data TEXT NOT NULL
    );
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DIR/empty.db'
    export OC_NON_INTERACTIVE=1
    export PROJECTS_FILE='$PROJECTS_FILE'
    export PATHS_FILE='$PATHS_FILE'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export PATH='$TEST_DIR/bin:$PATH'
    bash '$CMD_START' --resume -p TEST-PROJ
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"session"* ]]
}

@test "cmd-start --resume : sélection valide lance opencode -s <id>" {
  export _OCDB_FILE="$TEST_DIR/resume.db"
  # Créer .beads pour éviter le prompt d'initialisation beads
  mkdir -p "$TEST_DIR/fake-project/.beads"
  local NOW_MS=$(( $(date +%s) * 1000 ))
  sqlite3 "$TEST_DIR/resume.db" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY, project_id TEXT NOT NULL DEFAULT 'p',
      parent_id TEXT, slug TEXT NOT NULL DEFAULT 's',
      directory TEXT NOT NULL DEFAULT '/d', title TEXT NOT NULL DEFAULT 't',
      version TEXT NOT NULL DEFAULT '1', share_url TEXT,
      cost REAL DEFAULT 0, tokens_input INTEGER DEFAULT 0,
      tokens_output INTEGER DEFAULT 0, tokens_reasoning INTEGER DEFAULT 0,
      tokens_cache_read INTEGER DEFAULT 0, tokens_cache_write INTEGER DEFAULT 0,
      agent TEXT, model TEXT, metadata TEXT,
      time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL
    );
    CREATE TABLE part (
      id TEXT PRIMARY KEY, message_id TEXT NOT NULL DEFAULT 'msg',
      session_id TEXT NOT NULL, time_created INTEGER NOT NULL,
      time_updated INTEGER NOT NULL, data TEXT NOT NULL
    );
    INSERT INTO session (id, slug, directory, title, agent, cost, time_created, time_updated)
    VALUES ('session-abc', 'my-slug', '$TEST_DIR/fake-project', 'Test session', 'explore', 0.05, $NOW_MS, $NOW_MS);
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DIR/resume.db'
    export OC_NON_INTERACTIVE=0
    export PROJECTS_FILE='$PROJECTS_FILE'
    export PATHS_FILE='$PATHS_FILE'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export OPENCODE_LOG='$OPENCODE_LOG'
    export PATH='$TEST_DIR/bin:$PATH'
    printf '1\n\n' | bash '$CMD_START' --resume -p TEST-PROJ
  "
  [ "$status" -eq 0 ]
  grep -q '\-s session-abc' "$OPENCODE_LOG"
}

@test "cmd-start --resume : sélection hors plage affiche erreur" {
  export _OCDB_FILE="$TEST_DIR/resume2.db"
  local NOW_MS=$(( $(date +%s) * 1000 ))
  sqlite3 "$TEST_DIR/resume2.db" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY, project_id TEXT NOT NULL DEFAULT 'p',
      parent_id TEXT, slug TEXT NOT NULL DEFAULT 's',
      directory TEXT NOT NULL DEFAULT '/d', title TEXT NOT NULL DEFAULT 't',
      version TEXT NOT NULL DEFAULT '1', share_url TEXT,
      cost REAL DEFAULT 0, tokens_input INTEGER DEFAULT 0,
      tokens_output INTEGER DEFAULT 0, tokens_reasoning INTEGER DEFAULT 0,
      tokens_cache_read INTEGER DEFAULT 0, tokens_cache_write INTEGER DEFAULT 0,
      agent TEXT, model TEXT, metadata TEXT,
      time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL
    );
    CREATE TABLE part (
      id TEXT PRIMARY KEY, message_id TEXT NOT NULL DEFAULT 'msg',
      session_id TEXT NOT NULL, time_created INTEGER NOT NULL,
      time_updated INTEGER NOT NULL, data TEXT NOT NULL
    );
    INSERT INTO session (id, slug, directory, title, agent, cost, time_created, time_updated)
    VALUES ('session-xyz', 'slug-xyz', '$TEST_DIR/fake-project', 'Test', 'explore', 0.01, $NOW_MS, $NOW_MS);
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DIR/resume2.db'
    export OC_NON_INTERACTIVE=0
    export PROJECTS_FILE='$PROJECTS_FILE'
    export PATHS_FILE='$PATHS_FILE'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export PATH='$TEST_DIR/bin:$PATH'
    printf '99\n' | bash '$CMD_START' --resume -p TEST-PROJ
  "
  [ "$status" -eq 1 ]
  [[ "$output" == *"invalide"* ]] || [[ "$output" == *"Invalid"* ]]
}
