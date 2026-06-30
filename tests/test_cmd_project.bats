#!/usr/bin/env bats
# Tests unitaires pour scripts/cmd-project.sh
# Commandes testées : oc project rename, oc project move, oc project configure
# Note : cmd-project.sh est exécuté directement (pas sourcé) car il utilise set -euo pipefail

load helpers

setup() {
  common_setup

  # Les tests rename/move pipent des réponses via <<<. Pour que _prompt lise
  # stdin (au lieu de court-circuiter avec OC_NON_INTERACTIVE=1), on le désactive.
  export OC_NON_INTERACTIVE=0

  # Variables d'environnement pour cmd-project.sh
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  
  # Créer un registre de projets de test
  cat > "$PROJECTS_FILE" <<'EOF'
# Projets de test

## PROJ-OLD
- Nom : Ancien Projet
- Stack : TypeScript

## PROJ-KEEP
- Nom : Projet à Conserver
- Stack : Python

## PROJ-MOVE
- Nom : Projet à Déplacer
- Stack : Test
EOF

  # Créer paths.local.md
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  cat > "$PATHS_FILE" <<EOF
PROJ-OLD=$TEST_DIR/old-project
PROJ-KEEP=$TEST_DIR/keep-project
PROJ-MOVE=$TEST_DIR/move-project
EOF

  # Créer api-keys.local.md
  cat > "$API_KEYS_FILE" <<'EOF'
# API Keys de test

[PROJ-OLD]
provider: anthropic
api_key: sk-old-key

[PROJ-KEEP]
provider: bedrock
api_key: keep-key
EOF

  # Créer les répertoires de projets
  mkdir -p "$TEST_DIR/old-project"
  mkdir -p "$TEST_DIR/keep-project"
  mkdir -p "$TEST_DIR/move-project"
  mkdir -p "$TEST_DIR/new-location"
}

teardown() {
  common_teardown
}

# ── cmd_rename : tests de validation ──────────────────────────────────────────

@test "cmd_rename : échoue si OLD_ID manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename
  [ "$status" -ne 0 ]
}

@test "cmd_rename : échoue si NEW_ID manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : échoue si projet OLD_ID inexistant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "INEXISTANT" --to "PROJ-NEW" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : échoue si projet NEW_ID existe déjà" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-KEEP" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : exit 0 si OLD_ID == NEW_ID (identiques)" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-OLD" <<< "y"
  [ "$status" -eq 0 ]
}

@test "cmd_rename : normalise les IDs (minuscules → majuscules)" {
  skip "Normalisation des IDs - edge case complexe à implémenter"
  # Ajouter un projet en minuscule à projects.md ET paths.local.md
  echo -e "\n## proj-lower\n- Nom : Lower" >> "$PROJECTS_FILE"
  echo "proj-lower=$TEST_DIR/lower-project" >> "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "proj-lower" --to "PROJ-UPPER" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-UPPER"
  assert_file_not_contains "$PROJECTS_FILE" "## proj-lower"
}

# ── cmd_rename : confirmation utilisateur ─────────────────────────────────────

@test "cmd_rename : annule si utilisateur répond 'n'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< "n"
  [ "$status" -eq 0 ]
  
  # Fichier non modifié
  assert_file_contains "$PROJECTS_FILE" "## PROJ-OLD"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

@test "cmd_rename : annule si utilisateur répond vide (défaut=N)" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< ""
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : exécute si utilisateur répond 'y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : exécute si utilisateur répond 'Y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< "Y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

# ── cmd_rename : modification de projects.md ──────────────────────────────────

@test "cmd_rename : renomme le header ## dans projects.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-RENAMED"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : préserve les autres champs du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "- Nom : Ancien Projet"
  assert_file_contains "$PROJECTS_FILE" "- Stack : TypeScript"
}

@test "cmd_rename : ne touche pas aux autres projets" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-KEEP"
  assert_file_contains "$PROJECTS_FILE" "- Nom : Projet à Conserver"
}

# ── cmd_rename : modification de paths.local.md ───────────────────────────────

@test "cmd_rename : renomme la clé dans paths.local.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-RENAMED=$TEST_DIR/old-project"
  assert_file_not_contains "$PATHS_FILE" "PROJ-OLD="
}

@test "cmd_rename : préserve le chemin du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-RENAMED=$TEST_DIR/old-project"
}

@test "cmd_rename : ne touche pas aux autres entrées paths" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-KEEP=$TEST_DIR/keep-project"
}

# ── cmd_rename : modification de api-keys.local.md ────────────────────────────

@test "cmd_rename : renomme la section [ID] dans api-keys" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "[PROJ-RENAMED]"
  assert_file_not_contains "$API_KEYS_FILE" "[PROJ-OLD]"
}

@test "cmd_rename : préserve les clés API du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "provider: anthropic"
  assert_file_contains "$API_KEYS_FILE" "api_key: sk-old-key"
}

@test "cmd_rename : ne touche pas aux autres sections api-keys" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "[PROJ-KEEP]"
  assert_file_contains "$API_KEYS_FILE" "api_key: keep-key"
}

# ── cmd_rename : gestion des fichiers manquants ───────────────────────────────

@test "cmd_rename : fonctionne si paths.local.md absent" {
  rm "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

@test "cmd_rename : fonctionne si api-keys.local.md absent" {
  rm "$API_KEYS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-OLD" --to "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

# ── cmd_move : tests de validation ────────────────────────────────────────────

@test "cmd_move : échoue si PROJECT_ID manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p ""
  [ "$status" -ne 0 ]
}

@test "cmd_move : échoue si path manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE"
  [ "$status" -ne 0 ]
}

@test "cmd_move : échoue si projet inexistant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "INEXISTANT" "$TEST_DIR/new-path" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_move : normalise le PROJECT_ID" {
  skip "Normalisation - edge case complexe"
  echo "proj-lower=$TEST_DIR/lower" >> "$PATHS_FILE"
  echo -e "\n## proj-lower\n- Nom : Lower" >> "$PROJECTS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "proj-lower" "$TEST_DIR/new-path" <<< "y"
  [ "$status" -eq 0 ]
}

# ── cmd_move : confirmation utilisateur ───────────────────────────────────────

@test "cmd_move : annule si utilisateur répond 'n'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "n"
  [ "$status" -eq 0 ]
  
  # Chemin non modifié
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/move-project"
}

@test "cmd_move : exécute si utilisateur répond 'y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/new-location"
}

# ── cmd_move : modification de paths.local.md ─────────────────────────────────

@test "cmd_move : change le chemin dans paths.local.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/new-location"
  assert_file_not_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/move-project"
}

@test "cmd_move : ne touche pas aux autres projets" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-KEEP=$TEST_DIR/keep-project"
}

@test "cmd_move : résout les chemins relatifs" {
  skip "Chemins relatifs - nécessite cd dans test"
  cd "$TEST_DIR" || exit 1
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "./relative-path" <<< "y"
  [ "$status" -eq 0 ]
  
  # Le chemin doit être absolu dans paths.local.md
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/relative-path"
}

@test "cmd_move : expand ~ dans le chemin" {
  skip "Expansion ~ - comportement spécifique shell"
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "~/projects/test" <<< "y"
  [ "$status" -eq 0 ]
  
  # ~ doit être expandé vers $HOME
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$HOME/projects/test"
}

# ── cmd_move : ne touche PAS projects.md ni api-keys ──────────────────────────

@test "cmd_move : ne modifie pas projects.md" {
  cp "$PROJECTS_FILE" "$TEST_DIR/projects.md.backup"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  diff "$PROJECTS_FILE" "$TEST_DIR/projects.md.backup"
}

@test "cmd_move : ne modifie pas api-keys.local.md" {
  cp "$API_KEYS_FILE" "$TEST_DIR/api-keys.backup"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  diff "$API_KEYS_FILE" "$TEST_DIR/api-keys.backup"
}

# ── Cas limites ───────────────────────────────────────────────────────────────

@test "cmd_rename : gère les IDs avec caractères spéciaux" {
  echo -e "\n## PROJ-WITH-DASH\n- Nom : Test" >> "$PROJECTS_FILE"
  echo "PROJ-WITH-DASH=$TEST_DIR/test" >> "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-WITH-DASH" --to "PROJ-NEW-DASH" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW-DASH"
  assert_file_contains "$PATHS_FILE" "PROJ-NEW-DASH="
}

@test "cmd_move : gère les chemins avec espaces" {
  mkdir -p "$TEST_DIR/path with spaces"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move -p "PROJ-MOVE" "$TEST_DIR/path with spaces" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/path with spaces"
}

@test "cmd_rename : pas de modification si aucun fichier ne contient le projet" {
  # Projet existe dans projects.md mais pas dans paths/api-keys
  echo -e "\n## PROJ-ORPHAN\n- Nom : Orphan" >> "$PROJECTS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename --from "PROJ-ORPHAN" --to "PROJ-ADOPTED" <<< "y"
  [ "$status" -eq 0 ]
  
  # Au moins projects.md devrait être modifié
  assert_file_contains "$PROJECTS_FILE" "## PROJ-ADOPTED"
}

# ── cmd_configure : validation ────────────────────────────────────────────────

@test "cmd_configure : échoue si projet inexistant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "INEXISTANT" <<< ""
  [ "$status" -ne 0 ]
}

@test "cmd_configure : exit 0 si projet valide et Entrée pour tout" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < /dev/null
  [ "$status" -eq 0 ]
}

@test "cmd_configure : normalise le PROJECT_ID en majuscules" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "proj-old" < /dev/null
  [ "$status" -eq 0 ]
}

# ── cmd_configure : modification Stack ────────────────────────────────────────

@test "cmd_configure : met à jour le champ Stack" {
  # Séquence stdin dans l'ordre des prompts :
  # 1. Stack (modifié), 2. Tracker, 3. Labels, 4. Langue, 5. Disable, 6. MCP, 7. Worktree
  local input_file="$TEST_DIR/input.txt"
  printf 'Go + Gin\n\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Stack : Go + Gin"
}

@test "cmd_configure : préserve les autres champs si Stack modifié" {
  local input_file="$TEST_DIR/input.txt"
  printf 'Ruby on Rails\n\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "## PROJ-OLD"
  assert_file_contains "$PROJECTS_FILE" "- Nom : Ancien Projet"
}

# ── cmd_configure : modification Tracker ──────────────────────────────────────

@test "cmd_configure : met à jour le Tracker vers jira" {
  # 1. Stack (conserver), 2. Tracker=2 (jira), reste conserver
  local input_file="$TEST_DIR/input.txt"
  printf '\n2\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Tracker : jira"
}

@test "cmd_configure : met à jour le Tracker vers gitlab" {
  local input_file="$TEST_DIR/input.txt"
  printf '\n3\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Tracker : gitlab"
}

@test "cmd_configure : met à jour le Tracker vers none" {
  local input_file="$TEST_DIR/input.txt"
  # D'abord forcer jira
  printf '\n2\n\n\n\n\n\n' > "$input_file"
  bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file" >/dev/null 2>&1 || true
  # Remettre à none
  printf '\n1\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Tracker : none"
}

# ── cmd_configure : modification Labels ───────────────────────────────────────

@test "cmd_configure : met à jour les Labels" {
  # 1. Stack, 2. Tracker, 3. Labels=feature,fix,api, reste conserver
  local input_file="$TEST_DIR/input.txt"
  printf '\n\nfeature,fix,api\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Labels : feature,fix,api"
}

# ── cmd_configure : modification Langue ───────────────────────────────────────

@test "cmd_configure : met à jour la Langue" {
  # 1. Stack, 2. Tracker, 3. Labels, 4. Langue=english, reste conserver
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\nenglish\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Langue : english"
}

@test "cmd_configure : normalise la langue en minuscules" {
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\nENGLISH\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Langue : english"
}

# ── cmd_configure : modification Disable agents ───────────────────────────────

@test "cmd_configure : met à jour Disable agents" {
  # 1. Stack, 2. Tracker, 3. Labels, 4. Langue, 5. Disable=build,plan, reste conserver
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\n\nbuild,plan\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Disable agents : build,plan"
}

@test "cmd_configure : vide Disable agents avec 'none'" {
  local input_file="$TEST_DIR/input.txt"
  # D'abord ajouter disable agents
  printf '\n\n\n\nbuild\n\n\n' > "$input_file"
  bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file" >/dev/null 2>&1 || true
  # Puis le vider
  printf '\n\n\n\nnone\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_not_contains "$PROJECTS_FILE" "- Disable agents : build"
}

# ── cmd_configure : modification MCP ──────────────────────────────────────────

@test "cmd_configure : met à jour le champ MCP" {
  # 1. Stack, 2. Tracker, 3. Labels, 4. Langue, 5. Disable, 6. MCP=all, 7. Worktree
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\n\n\nall\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- MCP : all"
}

# ── cmd_configure : modification Worktree ─────────────────────────────────────

@test "cmd_configure : active les worktrees" {
  # 1. Stack, 2. Tracker, 3. Labels, 4. Langue, 5. Disable, 6. MCP, 7. Worktree=enabled
  # Quand worktree=enabled, 2 prompts supplémentaires : auto-cleanup, base-branch
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\n\n\n\nenabled\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Worktree : enabled"
}

@test "cmd_configure : active les worktrees avec 'y'" {
  local input_file="$TEST_DIR/input.txt"
  printf '\n\n\n\n\n\ny\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Worktree : enabled"
}

@test "cmd_configure : désactive les worktrees" {
  local input_file="$TEST_DIR/input.txt"
  # D'abord activer
  printf '\n\n\n\n\n\nenabled\n\n\n' > "$input_file"
  bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file" >/dev/null 2>&1 || true
  # Puis désactiver
  printf '\n\n\n\n\n\ndisabled\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Worktree : disabled"
}

@test "cmd_configure : configure auto-cleanup quand worktrees enabled" {
  local input_file="$TEST_DIR/input.txt"
  # Worktree=enabled, auto-cleanup=true, base-branch conserver
  printf '\n\n\n\n\n\nenabled\ntrue\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Worktree : enabled"
  assert_file_contains "$PROJECTS_FILE" "- Worktree auto cleanup : true"
}

@test "cmd_configure : configure base branch non-main" {
  local input_file="$TEST_DIR/input.txt"
  # Worktree=enabled, auto-cleanup conserver, base-branch=develop
  printf '\n\n\n\n\n\nenabled\n\ndevelop\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "- Worktree base branch : develop"
}

# ── cmd_configure : ne touche pas aux autres projets ──────────────────────────

@test "cmd_configure : ne modifie pas les autres projets" {
  local input_file="$TEST_DIR/input.txt"
  printf 'New Stack\n\n\n\n\n\n\n' > "$input_file"
  run bash "$HUB_DIR/scripts/cmd-project.sh" configure -p "PROJ-OLD" < "$input_file"
  [ "$status" -eq 0 ]

  assert_file_contains "$PROJECTS_FILE" "## PROJ-KEEP"
  assert_file_contains "$PROJECTS_FILE" "- Nom : Projet à Conserver"
}

# ── cmd_configure : help / sous-commande ──────────────────────────────────────

@test "cmd_configure : affiche dans le help" {
  run bash "$HUB_DIR/scripts/cmd-project.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "configure"
}

@test "sous-commande inconnue : erreur avec exit non-0" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" unknown-cmd
  [ "$status" -ne 0 ]
}
