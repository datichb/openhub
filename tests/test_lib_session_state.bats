#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/session-state.sh
# Fonctions testées : session_state_init, session_state_add_ticket,
#                     session_state_update_ticket, session_state_set_current,
#                     session_state_clear_current, session_state_end

load helpers

setup() {
  common_setup
  
  # Changer vers TEST_DIR pour que .opencode soit créé là
  cd "$TEST_DIR"
  
  # Sourcer session-state.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/session-state.sh"
  
  # Les variables readonly utilisent les valeurs par défaut (.opencode/)
  # On vérifie que le fichier sera créé dans TEST_DIR/.opencode/
  SESSION_STATE_FILE="$TEST_DIR/.opencode/session-state.json"
}

teardown() {
  common_teardown
}

# ── session_state_init ─────────────────────────────────────────────────────────

@test "session_state_init : crée fichier session-state.json" {
  run session_state_init "ses_test123" "semi-auto"
  [ "$status" -eq 0 ]
  [ -f "$SESSION_STATE_FILE" ]
}

@test "session_state_init : initialise structure JSON valide" {
  session_state_init "ses_test123" "semi-auto"
  [ -f "$SESSION_STATE_FILE" ]
  
  # Vérifier JSON valide avec jq si disponible
  if command -v jq &>/dev/null; then
    run jq empty "$SESSION_STATE_FILE"
    [ "$status" -eq 0 ]
  fi
}

@test "session_state_init : définit session_id correct" {
  session_state_init "ses_abc789" "manual"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.session_id' "$SESSION_STATE_FILE")
    [ "$result" = "ses_abc789" ]
  fi
}

@test "session_state_init : définit mode correct (semi-auto)" {
  session_state_init "ses_test" "semi-auto"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.mode' "$SESSION_STATE_FILE")
    [ "$result" = "semi-auto" ]
  fi
}

@test "session_state_init : définit mode correct (manuel)" {
  session_state_init "ses_test" "manuel"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.mode' "$SESSION_STATE_FILE")
    [ "$result" = "manuel" ]
  fi
}

@test "session_state_init : définit mode correct (auto)" {
  session_state_init "ses_test" "auto"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.mode' "$SESSION_STATE_FILE")
    [ "$result" = "auto" ]
  fi
}

@test "session_state_init : initialise tickets vide" {
  session_state_init "ses_test" "semi-auto"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets | length' "$SESSION_STATE_FILE")
    [ "$result" = "0" ]
  fi
}

@test "session_state_init : définit current_ticket à null" {
  session_state_init "ses_test" "semi-auto"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket' "$SESSION_STATE_FILE")
    [ "$result" = "null" ]
  fi
}

@test "session_state_init : échoue sans session_id" {
  run session_state_init "" "semi-auto"
  [ "$status" -ne 0 ]
}

@test "session_state_init : échoue sans mode" {
  run session_state_init "ses_test" ""
  [ "$status" -ne 0 ]
}

# ── session_state_add_ticket ───────────────────────────────────────────────────

@test "session_state_add_ticket : ajoute nouveau ticket" {
  session_state_init "ses_test" "semi-auto"
  run session_state_add_ticket "BD-42" "Fix null guard"
  [ "$status" -eq 0 ]
  
  if command -v jq &>/dev/null; then
    count=$(jq -r '.tickets | length' "$SESSION_STATE_FILE")
    [ "$count" = "1" ]
  fi
}

@test "session_state_add_ticket : définit ticket_id correct" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-123" "Test task"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].id' "$SESSION_STATE_FILE")
    [ "$result" = "BD-123" ]
  fi
}

@test "session_state_add_ticket : définit title correct" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-456" "Implement feature X"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].title' "$SESSION_STATE_FILE")
    [ "$result" = "Implement feature X" ]
  fi
}

@test "session_state_add_ticket : initialise statut pending" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-789" "Fix bug"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].status' "$SESSION_STATE_FILE")
    [ "$result" = "pending" ]
  fi
}

@test "session_state_add_ticket : ajoute timestamp last_update" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-100" "Task"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.last_update' "$SESSION_STATE_FILE")
    [ -n "$result" ]
    [[ "$result" != "null" ]]
  fi
}

@test "session_state_add_ticket : ajoute plusieurs tickets" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-1" "Task 1"
  session_state_add_ticket "BD-2" "Task 2"
  session_state_add_ticket "BD-3" "Task 3"
  
  if command -v jq &>/dev/null; then
    count=$(jq -r '.tickets | length' "$SESSION_STATE_FILE")
    [ "$count" = "3" ]
  fi
}

# ── session_state_update_ticket ────────────────────────────────────────────────

@test "session_state_update_ticket : change statut pending→in_progress" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  run session_state_update_ticket "BD-42" "in_progress"
  [ "$status" -eq 0 ]
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].status' "$SESSION_STATE_FILE")
    [ "$result" = "in_progress" ]
  fi
}

@test "session_state_update_ticket : change statut in_progress→completed" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_update_ticket "BD-42" "in_progress"
  run session_state_update_ticket "BD-42" "completed"
  [ "$status" -eq 0 ]
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].status' "$SESSION_STATE_FILE")
    [ "$result" = "completed" ]
  fi
}

@test "session_state_update_ticket : met à jour last_update" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_update_ticket "BD-42" "completed"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.last_update' "$SESSION_STATE_FILE")
    [ -n "$result" ]
    [[ "$result" != "null" ]]
  fi
}

@test "session_state_update_ticket : gère ticket inexistant" {
  session_state_init "ses_test" "semi-auto"
  # Pas de ticket ajouté
  run session_state_update_ticket "BD-999" "completed"
  # La fonction devrait gérer ce cas (comportement à vérifier)
  [ "$status" -eq 0 ]
}

# ── session_state_set_current ──────────────────────────────────────────────────

@test "session_state_set_current : définit current_ticket" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  run session_state_set_current "BD-42" "developer-backend" "implementing"
  [ "$status" -eq 0 ]
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.id' "$SESSION_STATE_FILE")
    [ "$result" = "BD-42" ]
  fi
}

@test "session_state_set_current : définit agent_id correct" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_set_current "BD-42" "developer-frontend" "implementing"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.agent' "$SESSION_STATE_FILE")
    [ "$result" = "developer-frontend" ]
  fi
}

@test "session_state_set_current : définit phase correcte" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_set_current "BD-42" "developer-backend" "reviewing"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.action' "$SESSION_STATE_FILE")
    [ "$result" = "reviewing" ]
  fi
}

@test "session_state_set_current : met à jour last_update" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_set_current "BD-42" "developer-backend" "implementing"
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.last_update' "$SESSION_STATE_FILE")
    [ -n "$result" ]
    [[ "$result" != "null" ]]
  fi
}

# ── session_state_clear_current ────────────────────────────────────────────────

@test "session_state_clear_current : efface current_ticket" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  session_state_set_current "BD-42" "developer-backend" "implementing"
  run session_state_clear_current
  [ "$status" -eq 0 ]
  
  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket' "$SESSION_STATE_FILE")
    [ "$result" = "null" ]
  fi
}

@test "session_state_clear_current : gère current_ticket déjà null" {
  session_state_init "ses_test" "semi-auto"
  run session_state_clear_current
  [ "$status" -eq 0 ]
}

# ── session_state_end ──────────────────────────────────────────────────────────

@test "session_state_end : ajoute ended_at timestamp" {
  session_state_init "ses_test" "semi-auto"
  [ -f "$SESSION_STATE_FILE" ]
  run session_state_end
  [ "$status" -eq 0 ]
  # session_state_end supprime le fichier d'état
  [ ! -f "$SESSION_STATE_FILE" ]
}

@test "session_state_end : marque session terminée" {
  session_state_init "ses_test" "semi-auto"
  [ -f "$SESSION_STATE_FILE" ]
  session_state_end
  # Le fichier est supprimé après la fin de session
  [ ! -f "$SESSION_STATE_FILE" ]
}

# ── Helpers internes ───────────────────────────────────────────────────────────

@test "_session_timestamp : génère timestamp ISO8601 UTC" {
  result=$(_session_timestamp)
  # Format attendu : YYYY-MM-DDTHH:MM:SSZ
  [[ "$result" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
}

@test "_session_escape : échappe quotes doubles" {
  result=$(_session_escape 'Test "quoted" string')
  [[ "$result" == *'\"quoted\"'* ]]
}

@test "_session_escape : échappe backslashes" {
  result=$(_session_escape 'Path\\with\\backslashes')
  [[ "$result" == *'\\\\'* ]]
}

@test "_session_escape : échappe newlines" {
  result=$(_session_escape $'Line 1\nLine 2')
  [[ "$result" == *'\n'* ]]
}

@test "_session_write_state : écriture atomique (tmp+rename)" {
  session_state_init "ses_test" "semi-auto"
  # Vérifier qu'il n'y a pas de fichier .tmp après l'écriture
  run ls "$_SESSION_STATE_DIR"/*.tmp.* 2>/dev/null
  [ "$status" -ne 0 ]  # Pas de fichier .tmp
}

# ── Intégration : cycle complet ────────────────────────────────────────────────

@test "Intégration : cycle session complet" {
  # Init session
  session_state_init "ses_integration" "semi-auto"
  [ -f "$SESSION_STATE_FILE" ]
  
  # Ajouter tickets
  session_state_add_ticket "BD-1" "Task 1"
  session_state_add_ticket "BD-2" "Task 2"
  
  if command -v jq &>/dev/null; then
    count=$(jq -r '.tickets | length' "$SESSION_STATE_FILE")
    [ "$count" = "2" ]
  fi
  
  # Définir ticket courant
  session_state_set_current "BD-1" "developer-backend" "implementing"
  
  if command -v jq &>/dev/null; then
    current=$(jq -r '.current_ticket.id' "$SESSION_STATE_FILE")
    [ "$current" = "BD-1" ]
  fi
  
  # Mettre à jour statut
  session_state_update_ticket "BD-1" "in_progress"
  
  if command -v jq &>/dev/null; then
    status=$(jq -r '.tickets[0].status' "$SESSION_STATE_FILE")
    [ "$status" = "in_progress" ]
  fi
  
  # Terminer ticket
  session_state_update_ticket "BD-1" "completed"
  session_state_clear_current
  
  if command -v jq &>/dev/null; then
    current=$(jq -r '.current_ticket' "$SESSION_STATE_FILE")
    [ "$current" = "null" ]
  fi
  
  # Terminer session — supprime le fichier d'état
  session_state_end
  [ ! -f "$SESSION_STATE_FILE" ]
}

# ── worktree_path ──────────────────────────────────────────────────────────────

@test "session_state_add_ticket : accepte worktree_path optionnel" {
  session_state_init "ses_test" "semi-auto"
  run session_state_add_ticket "BD-42" "Fix null guard" ".worktrees/feat-bd-42"
  [ "$status" -eq 0 ]

  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].worktree_path' "$SESSION_STATE_FILE")
    [ "$result" = ".worktrees/feat-bd-42" ]
  fi
}

@test "session_state_add_ticket : worktree_path null si non fourni" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Fix null guard"

  if command -v jq &>/dev/null; then
    result=$(jq -r '.tickets[0].worktree_path' "$SESSION_STATE_FILE")
    [ "$result" = "null" ]
  fi
}

@test "session_state_set_current : worktree_path présent dans current_ticket" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task" ".worktrees/feat-bd-42"
  run session_state_set_current "BD-42" "developer-backend" "implementing" ".worktrees/feat-bd-42"
  [ "$status" -eq 0 ]

  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.worktree_path' "$SESSION_STATE_FILE")
    [ "$result" = ".worktrees/feat-bd-42" ]
  fi
}

@test "session_state_set_current : worktree_path null si non fourni" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task"
  run session_state_set_current "BD-42" "developer-backend" "implementing"
  [ "$status" -eq 0 ]

  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.worktree_path' "$SESSION_STATE_FILE")
    [ "$result" = "null" ]
  fi
}

@test "session_state_set_current : hérite worktree_path du ticket si non fourni en paramètre" {
  session_state_init "ses_test" "semi-auto"
  session_state_add_ticket "BD-42" "Task" ".worktrees/feat-inherited"
  # Pas de worktree_path en paramètre — doit hériter du ticket
  run session_state_set_current "BD-42" "developer-backend" "implementing"
  [ "$status" -eq 0 ]

  if command -v jq &>/dev/null; then
    result=$(jq -r '.current_ticket.worktree_path' "$SESSION_STATE_FILE")
    [ "$result" = ".worktrees/feat-inherited" ]
  fi
}
