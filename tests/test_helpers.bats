#!/usr/bin/env bats
# Tests de validation pour tests/helpers.bash
# Vérifie que les helpers fonctionnent correctement

load helpers

setup() {
  common_setup
}

teardown() {
  common_teardown
}

# ── Tests des fonctions de création de fixtures ──────────────────────────────

@test "make_test_project : crée un projet minimal" {
  make_test_project "TEST-PROJ" "Mon Projet" "TypeScript"
  
  assert_file_contains "$PROJECTS_FILE" "## TEST-PROJ"
  assert_file_contains "$PROJECTS_FILE" "- Nom : Mon Projet"
  assert_file_contains "$PROJECTS_FILE" "- Stack : TypeScript"
}

@test "make_full_test_project : crée un projet complet" {
  make_full_test_project "TEST-FULL" "Projet Complet" "Python" "gitlab" "user/repo"
  
  assert_file_contains "$PROJECTS_FILE" "## TEST-FULL"
  assert_file_contains "$PROJECTS_FILE" "- Tracker : gitlab"
  assert_file_contains "$PROJECTS_FILE" "- Repo : user/repo"
}

@test "make_test_agent : crée un agent minimal" {
  make_test_agent "test-agent" "planning"
  
  [ -f "$TEST_DIR/agents/planning/test-agent.md" ]
  assert_file_contains "$TEST_DIR/agents/planning/test-agent.md" "id: test-agent"
  assert_file_contains "$TEST_DIR/agents/planning/test-agent.md" "targets: [opencode]"
}

@test "make_test_agent : crée un agent avec skills" {
  make_test_agent "agent-with-skills" "developer" "skill1" "skill2"
  
  assert_file_contains "$TEST_DIR/agents/developer/agent-with-skills.md" 'skills: ["skill1", "skill2"]'
}

@test "make_test_skill : crée une skill" {
  make_test_skill "test-skill" "shared"
  
  [ -f "$TEST_DIR/skills/shared/test-skill.md" ]
  assert_file_contains "$TEST_DIR/skills/shared/test-skill.md" "id: test-skill"
}

@test "make_test_hub_config : crée un hub.json valide" {
  make_test_hub_config "claude-opus-4"
  
  [ -f "$HUB_CONFIG" ]
  assert_json_valid "$HUB_CONFIG"
  assert_json_field "$HUB_CONFIG" ".opencode.model" "claude-opus-4"
}

@test "make_test_api_keys : crée un fichier api-keys" {
  make_test_api_keys "anthropic" "sk-test-123"
  
  [ -f "$API_KEYS_FILE" ]
  assert_file_contains "$API_KEYS_FILE" "provider: anthropic"
  assert_file_contains "$API_KEYS_FILE" "api_key: sk-test-123"
}

# ── Tests des assertions ──────────────────────────────────────────────────────

@test "assert_file_contains : succès quand pattern présent" {
  echo "test content" > "$TEST_DIR/test.txt"
  assert_file_contains "$TEST_DIR/test.txt" "test"
}

@test "assert_file_contains : échec quand pattern absent" {
  echo "test content" > "$TEST_DIR/test.txt"
  run assert_file_contains "$TEST_DIR/test.txt" "absent"
  [ "$status" -ne 0 ]
}

@test "assert_file_not_contains : succès quand pattern absent" {
  echo "test content" > "$TEST_DIR/test.txt"
  assert_file_not_contains "$TEST_DIR/test.txt" "absent"
}

@test "assert_json_valid : valide du JSON correct" {
  require_command "jq"
  echo '{"key": "value"}' > "$TEST_DIR/test.json"
  assert_json_valid "$TEST_DIR/test.json"
}

@test "assert_json_valid : détecte du JSON invalide" {
  require_command "jq"
  echo '{invalid json}' > "$TEST_DIR/test.json"
  run assert_json_valid "$TEST_DIR/test.json"
  [ "$status" -ne 0 ]
}

@test "count_occurrences : compte correctement les occurrences" {
  echo -e "line1\nline2\nline1" > "$TEST_DIR/test.txt"
  count=$(count_occurrences "$TEST_DIR/test.txt" "line1")
  [ "$count" -eq 2 ]
}

# ── Tests des mocks ───────────────────────────────────────────────────────────

@test "mock_bd_with_log : capture les appels à bd" {
  BD_LOG="$TEST_DIR/bd.log"
  mock_bd_with_log "$BD_LOG"
  
  bd list
  bd create "task"
  
  assert_file_contains "$BD_LOG" "bd list"
  assert_file_contains "$BD_LOG" "bd create task"
}

@test "mock_git_with_log : capture les appels à git" {
  GIT_LOG="$TEST_DIR/git.log"
  mock_git_with_log "$GIT_LOG"
  
  git status
  git add .
  
  assert_file_contains "$GIT_LOG" "git status"
  assert_file_contains "$GIT_LOG" "git add ."
}

@test "mock_get_hub_version : retourne la version mockée" {
  # Recréer le mock avec une nouvelle version
  get_hub_version() {
    echo "1.2.3"
  }
  export -f get_hub_version
  
  result=$(get_hub_version)
  [ "$result" = "1.2.3" ]
}

# ── Tests des utilitaires ─────────────────────────────────────────────────────

@test "count_lines : compte les lignes correctement" {
  echo -e "line1\nline2\nline3" > "$TEST_DIR/test.txt"
  count=$(count_lines "$TEST_DIR/test.txt")
  [ "$count" -eq 3 ]
}

@test "command_exists : détecte les commandes présentes" {
  command_exists "bash"
}

@test "command_exists : détecte les commandes absentes" {
  ! command_exists "commande-inexistante-xyz"
}

@test "trim : supprime les espaces en début et fin" {
  result=$(trim "  texte  ")
  [ "$result" = "texte" ]
}

# ── Test d'intégration ────────────────────────────────────────────────────────

@test "intégration : créer un environnement de test complet" {
  # Créer des fixtures
  make_test_hub_config "claude-sonnet-4-5"
  make_test_project "PROJ-1" "Projet Test" "TypeScript"
  make_test_agent "agent-1" "planning" "skill-1"
  make_test_skill "skill-1" "shared"
  
  # Vérifier que tout existe
  [ -f "$HUB_CONFIG" ]
  [ -f "$PROJECTS_FILE" ]
  [ -f "$TEST_DIR/agents/planning/agent-1.md" ]
  [ -f "$TEST_DIR/skills/shared/skill-1.md" ]
  
  # Vérifier le contenu
  assert_json_valid "$HUB_CONFIG"
  assert_file_contains "$PROJECTS_FILE" "## PROJ-1"
  assert_file_contains "$TEST_DIR/agents/planning/agent-1.md" "id: agent-1"
  assert_file_contains "$TEST_DIR/skills/shared/skill-1.md" "id: skill-1"
}
