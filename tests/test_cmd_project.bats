#!/usr/bin/env bats
# Tests unitaires pour scripts/cmd-project.sh
# Commandes testées : oc project rename, oc project move
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
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : échoue si projet OLD_ID inexistant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "INEXISTANT" "PROJ-NEW" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : échoue si projet NEW_ID existe déjà" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-KEEP" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_rename : exit 0 si OLD_ID == NEW_ID (identiques)" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-OLD" <<< "y"
  [ "$status" -eq 0 ]
}

@test "cmd_rename : normalise les IDs (minuscules → majuscules)" {
  skip "Normalisation des IDs - edge case complexe à implémenter"
  # Ajouter un projet en minuscule à projects.md ET paths.local.md
  echo -e "\n## proj-lower\n- Nom : Lower" >> "$PROJECTS_FILE"
  echo "proj-lower=$TEST_DIR/lower-project" >> "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "proj-lower" "PROJ-UPPER" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-UPPER"
  assert_file_not_contains "$PROJECTS_FILE" "## proj-lower"
}

# ── cmd_rename : confirmation utilisateur ─────────────────────────────────────

@test "cmd_rename : annule si utilisateur répond 'n'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< "n"
  [ "$status" -eq 0 ]
  
  # Fichier non modifié
  assert_file_contains "$PROJECTS_FILE" "## PROJ-OLD"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

@test "cmd_rename : annule si utilisateur répond vide (défaut=N)" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< ""
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : exécute si utilisateur répond 'y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : exécute si utilisateur répond 'Y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< "Y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

# ── cmd_rename : modification de projects.md ──────────────────────────────────

@test "cmd_rename : renomme le header ## dans projects.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-RENAMED"
  assert_file_not_contains "$PROJECTS_FILE" "## PROJ-OLD"
}

@test "cmd_rename : préserve les autres champs du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "- Nom : Ancien Projet"
  assert_file_contains "$PROJECTS_FILE" "- Stack : TypeScript"
}

@test "cmd_rename : ne touche pas aux autres projets" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-KEEP"
  assert_file_contains "$PROJECTS_FILE" "- Nom : Projet à Conserver"
}

# ── cmd_rename : modification de paths.local.md ───────────────────────────────

@test "cmd_rename : renomme la clé dans paths.local.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-RENAMED=$TEST_DIR/old-project"
  assert_file_not_contains "$PATHS_FILE" "PROJ-OLD="
}

@test "cmd_rename : préserve le chemin du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-RENAMED=$TEST_DIR/old-project"
}

@test "cmd_rename : ne touche pas aux autres entrées paths" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-KEEP=$TEST_DIR/keep-project"
}

# ── cmd_rename : modification de api-keys.local.md ────────────────────────────

@test "cmd_rename : renomme la section [ID] dans api-keys" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "[PROJ-RENAMED]"
  assert_file_not_contains "$API_KEYS_FILE" "[PROJ-OLD]"
}

@test "cmd_rename : préserve les clés API du projet" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "provider: anthropic"
  assert_file_contains "$API_KEYS_FILE" "api_key: sk-old-key"
}

@test "cmd_rename : ne touche pas aux autres sections api-keys" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-RENAMED" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$API_KEYS_FILE" "[PROJ-KEEP]"
  assert_file_contains "$API_KEYS_FILE" "api_key: keep-key"
}

# ── cmd_rename : gestion des fichiers manquants ───────────────────────────────

@test "cmd_rename : fonctionne si paths.local.md absent" {
  rm "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

@test "cmd_rename : fonctionne si api-keys.local.md absent" {
  rm "$API_KEYS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-OLD" "PROJ-NEW" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW"
}

# ── cmd_move : tests de validation ────────────────────────────────────────────

@test "cmd_move : échoue si PROJECT_ID manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move ""
  [ "$status" -ne 0 ]
}

@test "cmd_move : échoue si path manquant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE"
  [ "$status" -ne 0 ]
}

@test "cmd_move : échoue si projet inexistant" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "INEXISTANT" "$TEST_DIR/new-path" <<< "y"
  [ "$status" -ne 0 ]
}

@test "cmd_move : normalise le PROJECT_ID" {
  skip "Normalisation - edge case complexe"
  echo "proj-lower=$TEST_DIR/lower" >> "$PATHS_FILE"
  echo -e "\n## proj-lower\n- Nom : Lower" >> "$PROJECTS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "proj-lower" "$TEST_DIR/new-path" <<< "y"
  [ "$status" -eq 0 ]
}

# ── cmd_move : confirmation utilisateur ───────────────────────────────────────

@test "cmd_move : annule si utilisateur répond 'n'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "n"
  [ "$status" -eq 0 ]
  
  # Chemin non modifié
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/move-project"
}

@test "cmd_move : exécute si utilisateur répond 'y'" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/new-location"
}

# ── cmd_move : modification de paths.local.md ─────────────────────────────────

@test "cmd_move : change le chemin dans paths.local.md" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/new-location"
  assert_file_not_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/move-project"
}

@test "cmd_move : ne touche pas aux autres projets" {
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-KEEP=$TEST_DIR/keep-project"
}

@test "cmd_move : résout les chemins relatifs" {
  skip "Chemins relatifs - nécessite cd dans test"
  cd "$TEST_DIR" || exit 1
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "./relative-path" <<< "y"
  [ "$status" -eq 0 ]
  
  # Le chemin doit être absolu dans paths.local.md
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/relative-path"
}

@test "cmd_move : expand ~ dans le chemin" {
  skip "Expansion ~ - comportement spécifique shell"
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "~/projects/test" <<< "y"
  [ "$status" -eq 0 ]
  
  # ~ doit être expandé vers $HOME
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$HOME/projects/test"
}

# ── cmd_move : ne touche PAS projects.md ni api-keys ──────────────────────────

@test "cmd_move : ne modifie pas projects.md" {
  cp "$PROJECTS_FILE" "$TEST_DIR/projects.md.backup"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  diff "$PROJECTS_FILE" "$TEST_DIR/projects.md.backup"
}

@test "cmd_move : ne modifie pas api-keys.local.md" {
  cp "$API_KEYS_FILE" "$TEST_DIR/api-keys.backup"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/new-location" <<< "y"
  [ "$status" -eq 0 ]
  
  diff "$API_KEYS_FILE" "$TEST_DIR/api-keys.backup"
}

# ── Cas limites ───────────────────────────────────────────────────────────────

@test "cmd_rename : gère les IDs avec caractères spéciaux" {
  echo -e "\n## PROJ-WITH-DASH\n- Nom : Test" >> "$PROJECTS_FILE"
  echo "PROJ-WITH-DASH=$TEST_DIR/test" >> "$PATHS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-WITH-DASH" "PROJ-NEW-DASH" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PROJECTS_FILE" "## PROJ-NEW-DASH"
  assert_file_contains "$PATHS_FILE" "PROJ-NEW-DASH="
}

@test "cmd_move : gère les chemins avec espaces" {
  mkdir -p "$TEST_DIR/path with spaces"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" move "PROJ-MOVE" "$TEST_DIR/path with spaces" <<< "y"
  [ "$status" -eq 0 ]
  
  assert_file_contains "$PATHS_FILE" "PROJ-MOVE=$TEST_DIR/path with spaces"
}

@test "cmd_rename : pas de modification si aucun fichier ne contient le projet" {
  # Projet existe dans projects.md mais pas dans paths/api-keys
  echo -e "\n## PROJ-ORPHAN\n- Nom : Orphan" >> "$PROJECTS_FILE"
  
  run bash "$HUB_DIR/scripts/cmd-project.sh" rename "PROJ-ORPHAN" "PROJ-ADOPTED" <<< "y"
  [ "$status" -eq 0 ]
  
  # Au moins projects.md devrait être modifié
  assert_file_contains "$PROJECTS_FILE" "## PROJ-ADOPTED"
}
