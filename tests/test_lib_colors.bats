#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/colors.sh
# Fonctions testées : log_info, log_success, log_warn, log_error, log_title, _intro, _outro, _prompt

load helpers

setup() {
  common_setup
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/colors.sh"
}

teardown() {
  common_teardown
}

# ── Constantes couleurs ─────────────────────────────────────────────────────

@test "Constantes : variables ANSI définies" {
  [ -n "$RED" ]
  [ -n "$GREEN" ]
  [ -n "$YELLOW" ]
  [ -n "$BLUE" ]
  [ -n "$CYAN" ]
  [ -n "$BOLD" ]
  [ -n "$DIM" ]
  [ -n "$RESET" ]
}

# ── Loggers ─────────────────────────────────────────────────────────────────

@test "log_info : affiche message avec symbole bleu" {
  run log_info "Test message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Test message"* ]]
  [[ "$output" == *"◆"* ]]
}

@test "log_success : affiche message avec symbole vert" {
  run log_success "Success message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Success message"* ]]
  [[ "$output" == *"◆"* ]]
}

@test "log_warn : affiche message sur stderr" {
  run log_warn "Warning message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Warning message"* ]]
}

@test "log_error : affiche message sur stderr" {
  run log_error "Error message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Error message"* ]]
}

@test "log_title : affiche titre en gras" {
  run log_title "My Title"
  [ "$status" -eq 0 ]
  [[ "$output" == *"My Title"* ]]
}

@test "log_info : gère messages multilignes" {
  run log_info "Line 1\nLine 2"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Line 1"* ]]
}

@test "log_info : gère caractères spéciaux" {
  run log_info "Test: \$VAR & special"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Test:"* ]]
}

# ── TUI Helpers ─────────────────────────────────────────────────────────────

@test "_intro : affiche titre avec gouttière" {
  run _intro "Command Title"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Command Title"* ]]
  [[ "$output" == *"◆"* ]]
  [[ "$output" == *"│"* ]]
}

@test "_outro : affiche message de clôture" {
  run _outro "Done message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Done message"* ]]
  [[ "$output" == *"└"* ]]
}

@test "_prompt : affiche gouttière et prompt" {
  # Mock read qui écrit dans la variable
  read() {
    eval "$5='test_value'"
    return 0
  }
  export -f read
  
  _prompt my_var "Enter value: "
  
  [ "$my_var" = "test_value" ]
}

@test "_prompt : gère EOF sans échouer" {
  # Mock read qui simule EOF
  read() {
    return 1
  }
  export -f read
  
  run _prompt my_var "Enter value: "
  [ "$status" -eq 0 ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : workflow TUI complet" {
  # Intro
  output_intro=$(_intro "Test Command")
  [[ "$output_intro" == *"Test Command"* ]]
  
  # Log info
  output_info=$(log_info "Processing...")
  [[ "$output_info" == *"Processing"* ]]
  
  # Outro
  output_outro=$(_outro "Completed")
  [[ "$output_outro" == *"Completed"* ]]
}

@test "Intégration : tous les loggers utilisables" {
  run log_info "Info"
  [ "$status" -eq 0 ]
  
  run log_success "Success"
  [ "$status" -eq 0 ]
  
  run log_warn "Warning"
  [ "$status" -eq 0 ]
  
  run log_error "Error"
  [ "$status" -eq 0 ]
  
  run log_title "Title"
  [ "$status" -eq 0 ]
}
