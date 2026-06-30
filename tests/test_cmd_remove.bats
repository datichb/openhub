#!/usr/bin/env bats
# Tests pour scripts/cmd-remove.sh
# cmd-remove.sh est un script top-level (non sourceable) — testé via exécution directe.
# La confirmation interactive est bypassée en pipant "y" sur stdin.

setup() {
  TEST_DIR="$(mktemp -d)"

  # Surcharger les fichiers de données vers le répertoire de test
  # Les variables exportées seront héritées par le sous-processus cmd-remove.sh
  # et respectées grâce au pattern ${VAR:-default} dans common.sh
  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # hub.json de test avec langue française — resolve_oc_lang lit ce fichier
  # dans le subprocess cmd-remove.sh et fixe OC_LANG=fr pour les messages d'erreur
  mkdir -p "$TEST_DIR/config"
  cat > "$TEST_DIR/config/hub.json" <<'HUBEOF'
{"version":"1.0.0","cli":{"language":"fr"}}
HUBEOF
  export HUB_CONFIG="$TEST_DIR/config/hub.json"

  # Script sous test
  CMD_REMOVE="$BATS_TEST_DIRNAME/../scripts/cmd-remove.sh"

  # Écrire un projects.md avec 2 projets
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test

## PROJ-A
- Nom : Projet Alpha
- Stack : Node.js
- Board Beads : PROJ-A
- Tracker : jira
- Labels : test

## PROJ-B
- Nom : Projet Bravo
- Stack : Python
- Board Beads : PROJ-B
- Tracker : gitlab
- Labels : back
PROJEOF

  # Écrire paths.local.md
  cat > "$PATHS_FILE" <<'PATHEOF'
PROJ-A=/home/user/alpha
PROJ-B=/home/user/bravo
PATHEOF

  # Écrire api-keys.local.md
  cat > "$API_KEYS_FILE" <<'APIEOF'
[PROJ-A]
model=claude-sonnet-4-5
provider=anthropic
api_key=sk-ant-aaa

[PROJ-B]
model=claude-opus-4-5
provider=litellm
api_key=sk-bbb
base_url=https://api.example.com/v1
APIEOF
}

teardown() {
  rm -rf "$TEST_DIR"
}

# Exécute cmd-remove.sh en surchargeant les fichiers de données via env
# @param $1 — PROJECT_ID à supprimer
# Pipe "y" pour confirmer la suppression
_run_remove() {
  run bash -c 'echo "y" | bash "$1" "$2"' _ "$CMD_REMOVE" "$1"
}

# ── Suppression complète ──────────────────────────────────────────────────────

@test "cmd-remove : supprime le projet de projects.md" {
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]

  # PROJ-A ne doit plus exister
  ! grep -q "^## PROJ-A" "$PROJECTS_FILE"
  # PROJ-B doit toujours exister
  grep -q "^## PROJ-B" "$PROJECTS_FILE"
}

@test "cmd-remove : supprime le chemin de paths.local.md" {
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]

  # PROJ-A ne doit plus être dans paths.local.md
  ! grep -q "^PROJ-A=" "$PATHS_FILE"
  # PROJ-B doit toujours être présent
  grep -q "^PROJ-B=" "$PATHS_FILE"
}

@test "cmd-remove : supprime la section de api-keys.local.md" {
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]

  # PROJ-A ne doit plus être dans api-keys.local.md
  ! grep -q "^\[PROJ-A\]" "$API_KEYS_FILE"
  # PROJ-B doit toujours être présent
  grep -q "^\[PROJ-B\]" "$API_KEYS_FILE"
}

@test "cmd-remove : ne laisse pas de fichier .bak" {
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]

  [ ! -f "${PROJECTS_FILE}.bak" ]
  [ ! -f "${PATHS_FILE}.bak" ]
}

# ── Cas limites ───────────────────────────────────────────────────────────────

@test "cmd-remove : exit 1 si le projet est absent" {
  run bash -c 'echo "y" | bash "$1" "$2"' _ "$CMD_REMOVE" "INEXISTANT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"introuvable"* ]]
}

@test "cmd-remove : exit 1 si aucun PROJECT_ID fourni" {
  run bash "$CMD_REMOVE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"requis"* ]]
}

@test "cmd-remove : tolère paths.local.md absent" {
  rm -f "$PATHS_FILE"
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]
}

@test "cmd-remove : tolère api-keys.local.md absent" {
  rm -f "$API_KEYS_FILE"
  _run_remove "PROJ-A"
  [ "$status" -eq 0 ]
}

@test "cmd-remove : ne supprime pas un projet dont l'ID est un préfixe" {
  # Ajouter un projet PROJ qui est un préfixe de PROJ-A et PROJ-B
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ
- Nom : Projet Préfixe
- Stack : Go
- Board Beads : PROJ
EOF
  printf 'PROJ=/home/user/proj\n' >> "$PATHS_FILE"

  _run_remove "PROJ"
  [ "$status" -eq 0 ]

  # PROJ-A et PROJ-B doivent toujours exister
  grep -q "^## PROJ-A" "$PROJECTS_FILE"
  grep -q "^## PROJ-B" "$PROJECTS_FILE"
  grep -q "^PROJ-A=" "$PATHS_FILE"
  grep -q "^PROJ-B=" "$PATHS_FILE"
}

# ── Mode --dry-run ────────────────────────────────────────────────────────────

@test "cmd-remove --dry-run : n'efface rien dans projects.md" {
  run bash "$CMD_REMOVE" --project PROJ-A --dry-run
  [ "$status" -eq 0 ]
  grep -q "^## PROJ-A" "$PROJECTS_FILE"
}

@test "cmd-remove --dry-run : n'efface rien dans paths.local.md" {
  run bash "$CMD_REMOVE" --project PROJ-A --dry-run
  [ "$status" -eq 0 ]
  grep -q "^PROJ-A=" "$PATHS_FILE"
}

@test "cmd-remove --dry-run : n'efface rien dans api-keys.local.md" {
  run bash "$CMD_REMOVE" --project PROJ-A --dry-run
  [ "$status" -eq 0 ]
  grep -q "^\[PROJ-A\]" "$API_KEYS_FILE"
}

@test "cmd-remove --dry-run : affiche les actions simulées" {
  run bash "$CMD_REMOVE" --project PROJ-A --dry-run
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]
}

@test "cmd-remove --dry-run (-n) : syntaxe courte fonctionne" {
  run bash "$CMD_REMOVE" --project PROJ-A -n
  [ "$status" -eq 0 ]
  grep -q "^## PROJ-A" "$PROJECTS_FILE"
}

@test "cmd-remove --dry-run : ne demande pas de confirmation" {
  # Sans pipe "y" — doit quand même réussir en dry-run
  run bash "$CMD_REMOVE" --project PROJ-A --dry-run
  [ "$status" -eq 0 ]
}

@test "cmd-remove --dry-run --clean : affiche les fichiers qui seraient supprimés" {
  # Créer un faux dossier projet avec agents
  local proj_dir="$TEST_DIR/alpha"
  mkdir -p "$proj_dir/.opencode/agents"
  touch "$proj_dir/opencode.json"
  # Mettre à jour le path
  printf 'PROJ-A=%s\n' "$proj_dir" >> "$PATHS_FILE"

  run bash "$CMD_REMOVE" --project PROJ-A --clean --dry-run
  [ "$status" -eq 0 ]
  grep -q "^## PROJ-A" "$PROJECTS_FILE"
  [ -d "$proj_dir/.opencode/agents" ]
  [ -f "$proj_dir/opencode.json" ]
}
