#!/usr/bin/env bats
# Tests unitaires pour scripts/cmd-board.sh
# Fonctions testées : helpers de rendu, _render_column, _render_board, cmd_board

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  source "$SCRIPT_DIR/common.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/cmd-board.sh"
  
  # Mock log functions
  mock_log_functions
  
  # Mock bd command
  export PATH="$TEST_DIR/bin:$PATH"
  mkdir -p "$TEST_DIR/bin"
}

teardown() {
  common_teardown
}

# ── Helpers de rendu ────────────────────────────────────────────────────────

@test "_visible_len : calcule longueur sans codes ANSI" {
  run _visible_len "test"
  [ "$status" -eq 0 ]
  [ "$output" = "4" ]
}

@test "_visible_len : ignore codes ANSI" {
  run _visible_len $'\033[91mtest\033[0m'
  [ "$status" -eq 0 ]
  [ "$output" = "4" ]
}

@test "_trunc : tronque chaîne longue" {
  run _trunc "hello world" 5
  [ "$status" -eq 0 ]
  [ "$output" = "hell…" ]
}

@test "_trunc : garde chaîne courte intacte" {
  run _trunc "hi" 5
  [ "$status" -eq 0 ]
  [ "$output" = "hi" ]
}

@test "_trunc_mid : tronque au milieu avec ellipse" {
  run _trunc_mid "beads-ticket-123456" 12
  [ "$status" -eq 0 ]
  [[ "$output" == *"…"* ]]
  [ "${#output}" -le 13 ]  # 12 + ellipse
}

@test "_trunc_mid : garde chaîne courte" {
  run _trunc_mid "bd-42" 10
  [ "$status" -eq 0 ]
  [ "$output" = "bd-42" ]
}

@test "_repeat : répète caractère N fois" {
  run _repeat '-' 5
  [ "$status" -eq 0 ]
  [ "$output" = "-----" ]
}

@test "_repeat : retourne vide si N=0" {
  run _repeat '-' 0
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "_pad : ajoute padding à droite" {
  run _pad "test" 10
  [ "$status" -eq 0 ]
  [ "${#output}" -eq 10 ]
  [[ "$output" == "test      " ]]
}

@test "_priority_badge : P0 en rouge" {
  run _priority_badge "0"
  [ "$status" -eq 0 ]
  [[ "$output" == *"P0"* ]]
}

@test "_priority_badge : P1 en jaune" {
  run _priority_badge "1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"P1"* ]]
}

@test "_type_badge : tronque type à 7 chars" {
  run _type_badge "feature-very-long"
  [ "$status" -eq 0 ]
  [[ "$output" == *"feature"* ]]
}

@test "_wrap_title : garde titre court sur 1 ligne" {
  _wrap_title "Short title" 50
  
  [ "$_WRAP_L1" = "Short title" ]
  [ -z "$_WRAP_L2" ]
}

@test "_wrap_title : wrappe titre long sur 2 lignes" {
  _wrap_title "This is a very long title that should be wrapped" 20
  
  [ -n "$_WRAP_L1" ]
  [ -n "$_WRAP_L2" ]
  [ "${#_WRAP_L1}" -le 20 ]
}

@test "_wrap_title : coupe au dernier espace" {
  _wrap_title "Hello world test" 12
  
  # Devrait couper entre "Hello world" et "test"
  [[ "$_WRAP_L1" == *"world"* ]] || [[ "$_WRAP_L1" == "Hello" ]]
  [ -n "$_WRAP_L2" ]
}

# ── _render_column ──────────────────────────────────────────────────────────

@test "_render_column : rend colonne vide" {
  _render_column "OPEN" "$DIM" "[]" 20
  
  [ "${#_COL_LINES[@]}" -gt 0 ]
  # Devrait contenir header, message vide, footer
  [[ "${_COL_LINES[0]}" == *"OPEN"* ]]
}

@test "_render_column : rend colonne avec 1 ticket" {
  local ticket_json='[{"id":"bd-42","title":"Test ticket","priority":"1","type":"feature"}]'
  
  _render_column "OPEN" "$DIM" "$ticket_json" 30
  
  [ "${#_COL_LINES[@]}" -gt 3 ]
  # Vérifier que le ticket apparaît
  local all_lines="${_COL_LINES[*]}"
  [[ "$all_lines" == *"bd-42"* ]]
  [[ "$all_lines" == *"Test ticket"* ]]
}

@test "_render_column : rend colonne avec multiples tickets" {
  local tickets_json='[
    {"id":"bd-1","title":"First","priority":"0","type":"bug"},
    {"id":"bd-2","title":"Second","priority":"1","type":"feature"}
  ]'
  
  _render_column "IN PROGRESS" "$BLUE" "$tickets_json" 30
  
  [ "${#_COL_LINES[@]}" -gt 6 ]
  local all_lines="${_COL_LINES[*]}"
  [[ "$all_lines" == *"bd-1"* ]]
  [[ "$all_lines" == *"bd-2"* ]]
}

@test "_render_column : header contient label" {
  _render_column "REVIEW" "$YELLOW" "[]" 25
  
  [[ "${_COL_LINES[0]}" == *"REVIEW"* ]]
}

@test "_render_column : footer avec bordure" {
  _render_column "BLOCKED" "$RED" "[]" 20
  
  local last_line="${_COL_LINES[-1]}"
  [[ "$last_line" == *"└"* ]]
}

# ── _render_board ───────────────────────────────────────────────────────────

@test "_render_board : nécessite bd disponible" {
  # Mock _require_bd qui échoue
  _require_bd() {
    return 1
  }
  export -f _require_bd
  
  run _render_board "TEST-PROJECT" "$TEST_DIR"
  [ "$status" -ne 0 ]
}

@test "_render_board : nécessite .beads init" {
  # Mock _require_bd OK
  _require_bd() {
    return 0
  }
  export -f _require_bd
  
  # Mock _require_beads_init qui échoue
  _require_beads_init() {
    return 1
  }
  export -f _require_beads_init
  
  run _render_board "TEST-PROJECT" "$TEST_DIR"
  [ "$status" -ne 0 ]
}

@test "_render_board : affiche board vide" {
  # Mock dependencies
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  export -f _require_bd _require_beads_init
  
  # Mock bd command qui retourne vide
  cat > "$TEST_DIR/bin/bd" <<'EOF'
#!/bin/bash
echo "[]"
EOF
  chmod +x "$TEST_DIR/bin/bd"
  
  # Mock tput
  tput() {
    echo "100"
  }
  export -f tput
  
  run _render_board "TEST-PROJECT" "$TEST_DIR"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OPEN"* ]]
  [[ "$output" == *"IN PROGRESS"* ]]
  [[ "$output" == *"REVIEW"* ]]
  [[ "$output" == *"BLOCKED"* ]]
}

# ── cmd_board ───────────────────────────────────────────────────────────────

@test "cmd_board : nécessite PROJECT_ID ou auto-discovery" {
  # Pas de .beads dans PWD
  run cmd_board ""
  [ "$status" -ne 0 ]
}

@test "cmd_board : auto-discovery trouve .beads" {
  mkdir -p "$TEST_DIR/project/.beads"
  cd "$TEST_DIR/project"
  
  # Mock dependencies
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  _render_board() { echo "Board rendered"; }
  export -f _require_bd _require_beads_init _render_board
  
  run cmd_board
  [ "$status" -eq 0 ]
  [[ "$output" == *"Board rendered"* ]]
}

@test "cmd_board : accepte PROJECT_ID explicite" {
  mkdir -p "$TEST_DIR/test-project"
  
  # Mock functions
  normalize_project_id() { echo "TEST-PROJECT"; }
  resolve_project_path() { echo "$TEST_DIR/test-project"; }
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  _render_board() { echo "Board for $1 at $2"; }
  export -f normalize_project_id resolve_project_path
  export -f _require_bd _require_beads_init _render_board
  
  run cmd_board "test-project"
  [ "$status" -eq 0 ]
  [[ "$output" == *"TEST-PROJECT"* ]]
}

@test "cmd_board : option --watch active le mode watch" {
  mkdir -p "$TEST_DIR/project/.beads"
  
  # Mock dependencies
  normalize_project_id() { echo "PROJECT"; }
  resolve_project_path() { echo "$TEST_DIR/project"; }
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  _render_board() { echo "Render"; return 0; }
  export -f normalize_project_id resolve_project_path
  export -f _require_bd _require_beads_init _render_board
  
  # Mock sleep pour exit immédiatement
  sleep() {
    exit 0
  }
  export -f sleep
  
  # Mock clear
  clear() {
    :
  }
  export -f clear
  
  run cmd_board "project" --watch
  # Should exit via mocked sleep
  [[ "$output" == *"Render"* ]]
}

@test "cmd_board : option --interval change l'intervalle" {
  mkdir -p "$TEST_DIR/project/.beads"
  
  # Mock dependencies
  normalize_project_id() { echo "PROJECT"; }
  resolve_project_path() { echo "$TEST_DIR/project"; }
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  _render_board() { return 0; }
  export -f normalize_project_id resolve_project_path
  export -f _require_bd _require_beads_init _render_board
  
  # Test que l'interval est parsé (pas de vrai test du sleep)
  # On vérifie juste que ça ne crash pas
  run cmd_board "project" --interval 10
  [ "$status" -eq 0 ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : workflow complet avec tickets" {
  mkdir -p "$TEST_DIR/project/.beads"
  
  # Mock bd pour retourner des tickets
  cat > "$TEST_DIR/bin/bd" <<'EOF'
#!/bin/bash
case "$3" in
  open)
    echo '[{"id":"bd-1","title":"Open task","priority":"1","type":"feature"}]'
    ;;
  in_progress)
    echo '[{"id":"bd-2","title":"In progress","priority":"0","type":"bug"}]'
    ;;
  review)
    echo '[]'
    ;;
  blocked)
    echo '[]'
    ;;
esac
EOF
  chmod +x "$TEST_DIR/bin/bd"
  
  # Mock dependencies
  normalize_project_id() { echo "PROJECT"; }
  resolve_project_path() { echo "$TEST_DIR/project"; }
  _require_bd() { return 0; }
  _require_beads_init() { return 0; }
  export -f normalize_project_id resolve_project_path
  export -f _require_bd _require_beads_init
  
  # Mock tput
  tput() { echo "120"; }
  export -f tput
  
  run cmd_board "project"
  [ "$status" -eq 0 ]
  [[ "$output" == *"bd-1"* ]]
  [[ "$output" == *"bd-2"* ]]
  [[ "$output" == *"OPEN"* ]]
  [[ "$output" == *"IN PROGRESS"* ]]
}
