#!/usr/bin/env bats
# Tests unitaires pour scripts/common.sh
# Fonctions testées : _get_project_field, get_project_language, get_project_tracker,
#                     get_project_labels, project_exists, normalize_project_id,
#                     resolve_project_path, api_keys_entry_exists,
#                     get_project_api_*, get_project_path, path_exists

setup() {
  TEST_DIR="$(mktemp -d)"

  # Sourcer common.sh — PROJECTS_FILE sera recalculé depuis BASH_SOURCE
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Surcharger PROJECTS_FILE après le source (écrase la valeur calculée)
  PROJECTS_FILE="$TEST_DIR/projects.md"
  # Surcharger API_KEYS_FILE pour les tests de clés API
  API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  # Surcharger PATHS_FILE pour les tests de chemins
  PATHS_FILE="$TEST_DIR/paths.local.md"

  # Écrire un projects.md minimal pour les tests
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test

## PROJ-FR
- Nom : Projet Français
- Stack : Test
- Board Beads : PROJ-FR
- Tracker : gitlab
- Labels : test

## PROJ-EN
- Nom : Projet Anglais
- Stack : Test
- Board Beads : PROJ-EN
- Tracker : jira
- Labels : test
- Langue : english

## PROJ-NO-TRACKER
- Nom : Sans Tracker
- Stack : Test
- Board Beads : PROJ-NO-TRACKER
- Labels : test

## PROJ-MULTI-LABELS
- Nom : Labels multiples
- Stack : Test
- Board Beads : PROJ-MULTI-LABELS
- Labels : feature,fix,front,back

## PROJ-NO-LABELS
- Nom : Sans Labels
- Stack : Test
- Board Beads : PROJ-NO-LABELS
PROJEOF
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── get_project_language ──────────────────────────────────────────────────────

@test "get_project_language : retourne la langue quand le champ Langue est présent" {
  run get_project_language "PROJ-EN"
  [ "$status" -eq 0 ]
  [ "$output" = "english" ]
}

@test "get_project_language : retourne une chaîne vide quand le champ Langue est absent" {
  local tmp_projects="$TEST_DIR/projects-no-lang.md"
  cat > "$tmp_projects" <<'EOF'
## PROJ-NO-LANG
- Nom : Sans Langue
- Stack : Test
- Board Beads : PROJ-NO-LANG
- Tracker : none
- Labels : test
EOF
  PROJECTS_FILE="$tmp_projects"
  run get_project_language "PROJ-NO-LANG"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "get_project_language : retourne une chaîne vide pour un PROJECT_ID inexistant" {
  run get_project_language "INEXISTANT"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── get_project_tracker ───────────────────────────────────────────────────────

@test "get_project_tracker : retourne jira quand Tracker est jira" {
  run get_project_tracker "PROJ-EN"
  [ "$status" -eq 0 ]
  [ "$output" = "jira" ]
}

@test "get_project_tracker : retourne gitlab quand Tracker est gitlab" {
  run get_project_tracker "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "gitlab" ]
}

@test "get_project_tracker : retourne none quand le champ Tracker est absent" {
  run get_project_tracker "PROJ-NO-TRACKER"
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

# ── get_project_labels ────────────────────────────────────────────────────────

@test "get_project_labels : retourne le label unique d'un projet" {
  run get_project_labels "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "test" ]
}

@test "get_project_labels : retourne la liste séparée par virgules" {
  run get_project_labels "PROJ-MULTI-LABELS"
  [ "$status" -eq 0 ]
  [ "$output" = "feature,fix,front,back" ]
}

@test "get_project_labels : retourne une chaîne vide quand le champ est absent" {
  run get_project_labels "PROJ-NO-LABELS"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "get_project_labels : retourne une chaîne vide pour un projet inexistant" {
  run get_project_labels "INEXISTANT"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── project_exists ────────────────────────────────────────────────────────────

@test "project_exists : retourne 0 pour un projet présent dans projects.md" {
  run project_exists "PROJ-FR"
  [ "$status" -eq 0 ]
}

@test "project_exists : retourne non-zero pour un projet absent" {
  run project_exists "INEXISTANT"
  [ "$status" -ne 0 ]
}

@test "project_exists : ne matche pas un préfixe de PROJECT_ID (sous-chaîne)" {
  # PROJ-FR existe, mais PROJ ne doit pas matcher
  run project_exists "PROJ"
  [ "$status" -ne 0 ]
}

# ── normalize_project_id ──────────────────────────────────────────────────────

@test "normalize_project_id : convertit en majuscules" {
  run normalize_project_id "mon-app"
  [ "$status" -eq 0 ]
  [ "$output" = "MON-APP" ]
}

# ── api_keys_entry_exists ─────────────────────────────────────────────────────

@test "api_keys_entry_exists : retourne 0 quand la section existe" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FR]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey
EOF
  run api_keys_entry_exists "PROJ-FR"
  [ "$status" -eq 0 ]
}

@test "api_keys_entry_exists : retourne non-zero quand la section est absente" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FR]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey
EOF
  run api_keys_entry_exists "INEXISTANT"
  [ "$status" -ne 0 ]
}

@test "api_keys_entry_exists : ne matche pas un préfixe de section (sous-chaîne)" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FULL]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey
EOF
  # [PROJ] ne doit pas matcher [PROJ-FULL]
  run api_keys_entry_exists "PROJ"
  [ "$status" -ne 0 ]
}

@test "api_keys_entry_exists : retourne non-zero quand le fichier est absent" {
  rm -f "$API_KEYS_FILE"
  run api_keys_entry_exists "PROJ-FR"
  [ "$status" -ne 0 ]
}

# ── get_project_api_model ─────────────────────────────────────────────────────

@test "get_project_api_model : retourne le modèle configuré" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FR]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey
EOF
  run get_project_api_model "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4-5" ]
}

@test "get_project_api_model : retourne une chaîne vide si le projet est absent" {
  cat > "$API_KEYS_FILE" <<'EOF'
[AUTRE]
model=claude-haiku-4-5
provider=anthropic
api_key=sk-ant-other
EOF
  run get_project_api_model "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── get_project_api_provider ──────────────────────────────────────────────────

@test "get_project_api_provider : retourne le provider configuré" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-LITELLM]
model=claude-sonnet-4-5
provider=litellm
api_key=sk-bRf-testkey
base_url=https://api.mammouth.ai/v1
EOF
  run get_project_api_provider "PROJ-LITELLM"
  [ "$status" -eq 0 ]
  [ "$output" = "litellm" ]
}

# ── get_project_api_key ───────────────────────────────────────────────────────

@test "get_project_api_key : retourne la clé API configurée" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FR]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey123
EOF
  run get_project_api_key "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-ant-testkey123" ]
}

@test "get_project_api_key : retourne une chaîne vide si le projet est absent" {
  cat > "$API_KEYS_FILE" <<'EOF'
[AUTRE]
api_key=sk-ant-other
EOF
  run get_project_api_key "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── get_project_api_base_url ──────────────────────────────────────────────────

@test "get_project_api_base_url : retourne la base_url quand elle est présente" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-LITELLM]
model=claude-sonnet-4-5
provider=litellm
api_key=sk-bRf-testkey
base_url=https://api.mammouth.ai/v1
EOF
  run get_project_api_base_url "PROJ-LITELLM"
  [ "$status" -eq 0 ]
  [ "$output" = "https://api.mammouth.ai/v1" ]
}

@test "get_project_api_base_url : retourne une chaîne vide si base_url est absente" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-FR]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-testkey
EOF
  run get_project_api_base_url "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── parser INI : isolation entre sections ─────────────────────────────────────

@test "_api_keys_get : ne retourne pas les valeurs d'une autre section" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-A]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-aaa

[PROJ-B]
model=claude-haiku-4-5
provider=litellm
api_key=sk-bbb
EOF
  run get_project_api_key "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-ant-aaa" ]

  run get_project_api_key "PROJ-B"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-bbb" ]
}

@test "_api_keys_get : supporte les valeurs avec signe = (ex: URL avec query string)" {
  cat > "$API_KEYS_FILE" <<'EOF'
[PROJ-URL]
model=claude-sonnet-4-5
provider=litellm
api_key=sk-test
base_url=https://api.example.com/v1?foo=bar&baz=qux
EOF
  run get_project_api_base_url "PROJ-URL"
  [ "$status" -eq 0 ]
  [ "$output" = "https://api.example.com/v1?foo=bar&baz=qux" ]
}

# ── get_project_path ──────────────────────────────────────────────────────────

@test "get_project_path : retourne le chemin quand paths.local.md est présent" {
  printf 'PROJ-FR=/home/user/projets/proj-fr\nPROJ-EN=/home/user/projets/proj-en\n' > "$PATHS_FILE"
  run get_project_path "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "/home/user/projets/proj-fr" ]
}

@test "get_project_path : retourne 1 si paths.local.md est absent" {
  # PATHS_FILE pointe vers un fichier inexistant (pas créé dans ce test)
  run get_project_path "PROJ-FR"
  [ "$status" -ne 0 ]
}

@test "get_project_path : ne matche pas un préfixe de PROJECT_ID (sous-chaîne)" {
  printf 'PROJ-FULL=/home/user/projets/proj-full\n' > "$PATHS_FILE"
  run get_project_path "PROJ"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── path_exists ───────────────────────────────────────────────────────────────

@test "path_exists : retourne 0 si l'entrée existe" {
  printf 'PROJ-FR=/home/user/projets/proj-fr\n' > "$PATHS_FILE"
  run path_exists "PROJ-FR"
  [ "$status" -eq 0 ]
}

@test "path_exists : retourne non-zero si l'entrée est absente" {
  printf 'PROJ-FR=/home/user/projets/proj-fr\n' > "$PATHS_FILE"
  run path_exists "PROJ-ABSENT"
  [ "$status" -ne 0 ]
}

@test "path_exists : ne matche pas un préfixe de PROJECT_ID (sous-chaîne)" {
  printf 'PROJ-FULL=/home/user/projets/proj-full\n' > "$PATHS_FILE"
  run path_exists "PROJ"
  [ "$status" -ne 0 ]
}

# ── _get_project_field (parser interne) ───────────────────────────────────────

@test "_get_project_field : retourne la valeur brute d'un champ existant" {
  run _get_project_field "PROJ-EN" "Langue"
  [ "$status" -eq 0 ]
  [ "$output" = "english" ]
}

@test "_get_project_field : retourne une chaîne vide pour un champ absent" {
  run _get_project_field "PROJ-FR" "Langue"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_get_project_field : retourne une chaîne vide pour un projet inexistant" {
  run _get_project_field "INEXISTANT" "Tracker"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_get_project_field : ne lit pas un champ d'un autre projet" {
  # PROJ-EN a Langue, PROJ-FR n'en a pas — vérifier l'isolation
  run _get_project_field "PROJ-FR" "Langue"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_get_project_field : retourne Labels avec virgules intactes" {
  run _get_project_field "PROJ-MULTI-LABELS" "Labels"
  [ "$status" -eq 0 ]
  [ "$output" = "feature,fix,front,back" ]
}

# ── get_project_agents ────────────────────────────────────────────────────────

@test "get_project_agents : retourne all quand le champ Agents est absent" {
  run get_project_agents "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "all" ]
}

@test "get_project_agents : retourne all quand le champ Agents vaut all" {
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-ALL
- Nom : Avec All
- Stack : Test
- Labels : test
- Agents : all
EOF
  run get_project_agents "PROJ-ALL"
  [ "$status" -eq 0 ]
  [ "$output" = "all" ]
}

@test "get_project_agents : retourne le CSV quand des agents sont listés" {
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-SOME
- Nom : Agents partiels
- Stack : Test
- Labels : test
- Agents : reviewer,debugger,planner
EOF
  run get_project_agents "PROJ-SOME"
  [ "$status" -eq 0 ]
  [ "$output" = "reviewer,debugger,planner" ]
}

@test "get_project_agents : retourne all pour un projet inexistant" {
  run get_project_agents "INEXISTANT"
  [ "$status" -eq 0 ]
  [ "$output" = "all" ]
}

# ── should_deploy_agent ──────────────────────────────────────────────────────

@test "should_deploy_agent : retourne 0 si project_id vide" {
  run should_deploy_agent "" "reviewer"
  [ "$status" -eq 0 ]
}

@test "should_deploy_agent : retourne 0 si agents=all" {
  run should_deploy_agent "PROJ-FR" "reviewer"
  [ "$status" -eq 0 ]
}

@test "should_deploy_agent : retourne 0 si l'agent est dans le CSV" {
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-FILTER
- Nom : Filtré
- Stack : Test
- Labels : test
- Agents : reviewer,debugger,planner
EOF
  run should_deploy_agent "PROJ-FILTER" "debugger"
  [ "$status" -eq 0 ]
}

@test "should_deploy_agent : retourne non-zero si l'agent n'est pas dans le CSV" {
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-FILTER
- Nom : Filtré
- Stack : Test
- Labels : test
- Agents : reviewer,debugger,planner
EOF
  run should_deploy_agent "PROJ-FILTER" "orchestrator"
  [ "$status" -ne 0 ]
}

@test "should_deploy_agent : pas de faux positif sur sous-chaîne (review vs reviewer)" {
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-SUBSTR
- Nom : Sous-chaîne
- Stack : Test
- Labels : test
- Agents : reviewer,debugger
EOF
  run should_deploy_agent "PROJ-SUBSTR" "review"
  [ "$status" -ne 0 ]
}

# ── resolve_project_path ──────────────────────────────────────────────────────

@test "resolve_project_path : retourne le chemin d'un projet valide" {
  local project_dir="$TEST_DIR/my-project"
  mkdir -p "$project_dir"
  printf 'PROJ-FR=%s\n' "$project_dir" > "$PATHS_FILE"
  run resolve_project_path "PROJ-FR"
  [ "$status" -eq 0 ]
  [ "$output" = "$project_dir" ]
}

@test "resolve_project_path : normalise l'ID en majuscules" {
  local project_dir="$TEST_DIR/my-project"
  mkdir -p "$project_dir"
  printf 'PROJ-FR=%s\n' "$project_dir" > "$PATHS_FILE"
  run resolve_project_path "proj-fr"
  [ "$status" -eq 0 ]
  [ "$output" = "$project_dir" ]
}

@test "resolve_project_path : exit 1 si le projet n'existe pas" {
  run bash -c "source \"$BATS_TEST_DIRNAME/../scripts/common.sh\" 2>/dev/null; PATHS_FILE=\"$PATHS_FILE\" resolve_project_path INEXISTANT 2>&1"
  [ "$status" -eq 1 ]
  [[ "$output" == *"introuvable"* ]]
}

@test "resolve_project_path : exit 1 si le chemin est vide" {
  # PROJ-NO-LABELS existe dans projects.md mais n'a pas d'entrée dans paths.local.md
  printf '' > "$PATHS_FILE"
  run bash -c "source \"$BATS_TEST_DIRNAME/../scripts/common.sh\" 2>/dev/null; PATHS_FILE=\"$PATHS_FILE\" resolve_project_path PROJ-NO-LABELS 2>&1"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Aucun chemin"* ]]
}

@test "resolve_project_path : exit 1 si le dossier n'existe pas sur le disque" {
  printf 'PROJ-FR=/tmp/dossier-inexistant-%s\n' "$$" > "$PATHS_FILE"
  run bash -c "source \"$BATS_TEST_DIRNAME/../scripts/common.sh\" 2>/dev/null; PATHS_FILE=\"$PATHS_FILE\" resolve_project_path PROJ-FR 2>&1"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Dossier introuvable"* ]]
}

# ── _set_project_agents (lib/agent-picker.sh) ─────────────────────────────────

@test "_set_project_agents : remplace un champ Agents existant" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/agent-picker.sh"
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-SET
- Nom : Test Set
- Stack : Test
- Labels : test
- Agents : all
EOF
  run _set_project_agents "PROJ-SET" "reviewer,debugger"
  [ "$status" -eq 0 ]
  run grep -- "- Agents : reviewer,debugger" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "_set_project_agents : ajoute après Labels si Agents absent" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/agent-picker.sh"
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-NO-AGENTS
- Nom : Sans Agents
- Stack : Test
- Labels : test
EOF
  run _set_project_agents "PROJ-NO-AGENTS" "planner,orchestrator"
  [ "$status" -eq 0 ]
  run grep -- "- Agents : planner,orchestrator" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "_set_project_agents : ne modifie pas les autres projets" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/agent-picker.sh"
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-A
- Nom : Projet A
- Stack : Test
- Labels : test
- Agents : all

## PROJ-B
- Nom : Projet B
- Stack : Test
- Labels : test
- Agents : all
EOF
  _set_project_agents "PROJ-A" "reviewer"
  # PROJ-A modifié
  run bash -c "awk '/^## PROJ-A$/,/^$/{print}' '$PROJECTS_FILE' | grep 'Agents'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"reviewer"* ]]
  # PROJ-B inchangé
  run bash -c "awk '/^## PROJ-B$/,/^$/{print}' '$PROJECTS_FILE' | grep 'Agents'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"all"* ]]
}
