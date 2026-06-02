#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/metrics.sh
# Fonctions testées : helpers, logging, timers, events, aggregation

load helpers

setup() {
  common_setup
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/metrics.sh"
  
  # Override des variables pour tester
  export _METRICS_DIR="$TEST_DIR/.opencode"
  export _METRICS_FILE="$_METRICS_DIR/metrics.jsonl"
  export _METRICS_TIMER_DIR="$TEST_DIR/timers"
}

teardown() {
  common_teardown
}

# ── Helpers ─────────────────────────────────────────────────────────────────

@test "_metrics_timestamp : retourne timestamp ISO8601 UTC" {
  run _metrics_timestamp
  [ "$status" -eq 0 ]
  # Format : YYYY-MM-DDTHH:MM:SSZ
  [[ "$output" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
}

@test "_metrics_escape : échappe les guillemets doubles" {
  run _metrics_escape 'Hello "world"'
  [ "$status" -eq 0 ]
  [ "$output" = 'Hello \"world\"' ]
}

@test "_metrics_escape : échappe les backslashes" {
  run _metrics_escape 'path\to\file'
  [ "$status" -eq 0 ]
  [ "$output" = 'path\\to\\file' ]
}

@test "_metrics_escape : échappe les newlines" {
  run _metrics_escape $'line1\nline2'
  [ "$status" -eq 0 ]
  [[ "$output" == *'\n'* ]]
}

@test "_metrics_escape : échappe les tabs" {
  run _metrics_escape $'col1\tcol2'
  [ "$status" -eq 0 ]
  [[ "$output" == *'\t'* ]]
}

@test "_metrics_ensure_file : crée le dossier et fichier" {
  [ ! -d "$_METRICS_DIR" ]
  [ ! -f "$_METRICS_FILE" ]
  
  _metrics_ensure_file
  
  [ -d "$_METRICS_DIR" ]
  [ -f "$_METRICS_FILE" ]
}

@test "_metrics_ensure_file : ne fait rien si déjà existants" {
  mkdir -p "$_METRICS_DIR"
  echo "existing" > "$_METRICS_FILE"
  
  _metrics_ensure_file
  
  run cat "$_METRICS_FILE"
  [ "$output" = "existing" ]
}

# ── Core logging ────────────────────────────────────────────────────────────

@test "metrics_log_event : log événement simple" {
  metrics_log_event "test_event" "bd-42"
  
  [ -f "$_METRICS_FILE" ]
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"test_event"'* ]]
  [[ "$output" == *'"ticket_id":"bd-42"'* ]]
  [[ "$output" == *'"timestamp":'* ]]
}

@test "metrics_log_event : log avec agent" {
  metrics_log_event "test_event" "bd-42" "developer-backend"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"agent":"developer-backend"'* ]]
}

@test "metrics_log_event : log avec duration" {
  metrics_log_event "test_event" "bd-42" "" "900"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"duration_seconds":900'* ]]
}

@test "metrics_log_event : log avec extra fields" {
  metrics_log_event "test_event" "bd-42" "" "" '"reason":"test","count":3'
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"reason":"test"'* ]]
  [[ "$output" == *'"count":3'* ]]
}

@test "metrics_log_event : échoue si event_type manquant" {
  run metrics_log_event "" "bd-42"
  [ "$status" -ne 0 ]
  [ ! -f "$_METRICS_FILE" ]
}

@test "metrics_log_event : échoue si ticket_id manquant" {
  run metrics_log_event "test_event" ""
  [ "$status" -ne 0 ]
  [ ! -f "$_METRICS_FILE" ]
}

@test "metrics_log_event : crée le fichier au premier appel" {
  [ ! -f "$_METRICS_FILE" ]
  
  metrics_log_event "test_event" "bd-42"
  
  [ -f "$_METRICS_FILE" ]
}

@test "metrics_log_event : append plusieurs événements" {
  metrics_log_event "event1" "bd-42"
  metrics_log_event "event2" "bd-43"
  
  run wc -l < "$_METRICS_FILE"
  output=$(echo "$output" | tr -d ' ')
  [ "$output" = "2" ]
}

# ── Timers ──────────────────────────────────────────────────────────────────

@test "metrics_start_timer : crée fichier timer" {
  metrics_start_timer "bd-42"
  
  [ -f "$_METRICS_TIMER_DIR/bd-42.timer" ]
}

@test "metrics_start_timer : stocke timestamp epoch" {
  metrics_start_timer "bd-42"
  
  run cat "$_METRICS_TIMER_DIR/bd-42.timer"
  [[ "$output" =~ ^[0-9]+$ ]]
}

@test "metrics_start_timer : échoue si ticket_id manquant" {
  run metrics_start_timer ""
  [ "$status" -ne 0 ]
}

@test "metrics_get_duration : retourne durée en secondes" {
  metrics_start_timer "bd-42"
  sleep 0.1
  
  run metrics_get_duration "bd-42"
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^[0-9]+$ ]]
  [ "$output" -ge 0 ]
}

@test "metrics_get_duration : retourne vide si timer inexistant" {
  run metrics_get_duration "bd-nonexistent"
  [ "$status" -ne 0 ]
  [ -z "$output" ]
}

@test "metrics_get_duration : échoue si ticket_id manquant" {
  run metrics_get_duration ""
  [ "$status" -ne 0 ]
}

@test "metrics_clear_timer : supprime fichier timer" {
  metrics_start_timer "bd-42"
  [ -f "$_METRICS_TIMER_DIR/bd-42.timer" ]
  
  metrics_clear_timer "bd-42"
  
  [ ! -f "$_METRICS_TIMER_DIR/bd-42.timer" ]
}

@test "metrics_clear_timer : ne fait rien si timer inexistant" {
  run metrics_clear_timer "bd-nonexistent"
  [ "$status" -eq 0 ]
}

# ── Event-specific functions ────────────────────────────────────────────────

@test "metrics_ticket_start : log ticket_start" {
  metrics_ticket_start "bd-42" "developer-backend"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"ticket_start"'* ]]
  [[ "$output" == *'"ticket_id":"bd-42"'* ]]
  [[ "$output" == *'"agent":"developer-backend"'* ]]
}

@test "metrics_ticket_start : fonctionne sans agent" {
  metrics_ticket_start "bd-42"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"ticket_start"'* ]]
}

@test "metrics_ticket_complete : log ticket_complete" {
  metrics_ticket_complete "bd-42" "developer-backend" "900"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"ticket_complete"'* ]]
  [[ "$output" == *'"duration_seconds":900'* ]]
}

@test "metrics_review_cycle : log review_cycle avec cycle number" {
  metrics_review_cycle "bd-42" "1"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"review_cycle"'* ]]
  [[ "$output" == *'"cycle":1'* ]]
}

@test "metrics_review_cycle : fonctionne sans cycle number" {
  metrics_review_cycle "bd-42"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"review_cycle"'* ]]
}

@test "metrics_correction : log correction avec raison" {
  metrics_correction "bd-42" "lint errors"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"correction"'* ]]
  [[ "$output" == *'"reason":"lint errors"'* ]]
}

@test "metrics_websearch : log websearch avec tool et query_type" {
  metrics_websearch "bd-42" "websearch" "CVE lookup"
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"websearch"'* ]]
  [[ "$output" == *'"tool":"websearch"'* ]]
  [[ "$output" == *'"query_type":"CVE lookup"'* ]]
}

# ── Aggregation functions ───────────────────────────────────────────────────

@test "metrics_file_exists : retourne faux si fichier absent" {
  run metrics_file_exists
  [ "$status" -ne 0 ]
}

@test "metrics_file_exists : retourne vrai si fichier existe et non vide" {
  mkdir -p "$_METRICS_DIR"
  echo "test" > "$_METRICS_FILE"
  
  run metrics_file_exists
  [ "$status" -eq 0 ]
}

@test "metrics_get_file_path : retourne chemin du fichier" {
  run metrics_get_file_path
  [ "$status" -eq 0 ]
  [ "$output" = "$_METRICS_FILE" ]
}

@test "metrics_count_completed : retourne 0 si fichier absent" {
  run metrics_count_completed
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_count_completed : compte les tickets complétés" {
  metrics_ticket_complete "bd-42" "developer-backend" "900"
  metrics_ticket_complete "bd-43" "developer-backend" "600"
  
  run metrics_count_completed
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

@test "metrics_avg_duration : retourne 0 si fichier absent" {
  run metrics_avg_duration
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_avg_duration : calcule durée moyenne" {
  metrics_ticket_complete "bd-42" "" "900"
  metrics_ticket_complete "bd-43" "" "600"
  
  run metrics_avg_duration
  [ "$status" -eq 0 ]
  [ "$output" = "750" ]
}

@test "metrics_avg_review_cycles : retourne 0 si fichier absent" {
  run metrics_avg_review_cycles
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_avg_review_cycles : calcule moyenne cycles" {
  metrics_ticket_complete "bd-42" "" "900"
  metrics_ticket_complete "bd-43" "" "600"
  metrics_review_cycle "bd-42" "1"
  metrics_review_cycle "bd-43" "1"
  metrics_review_cycle "bd-43" "2"
  
  run metrics_avg_review_cycles
  [ "$status" -eq 0 ]
  [ "$output" = "1.5" ]
}

@test "metrics_format_duration : formate secondes en heures/minutes/secondes" {
  run metrics_format_duration 3665
  [ "$status" -eq 0 ]
  [ "$output" = "1h 1m 5s" ]
}

@test "metrics_format_duration : formate minutes/secondes" {
  run metrics_format_duration 930
  [ "$status" -eq 0 ]
  [ "$output" = "15m 30s" ]
}

@test "metrics_format_duration : formate secondes uniquement" {
  run metrics_format_duration 45
  [ "$status" -eq 0 ]
  [ "$output" = "45s" ]
}

@test "metrics_count_websearch : compte les websearch calls" {
  metrics_websearch "bd-42" "websearch" "CVE lookup"
  metrics_websearch "bd-43" "webfetch" "Doc check"
  
  run metrics_count_websearch
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : cycle complet avec timer" {
  # Start timer
  metrics_start_timer "bd-42"
  [ -f "$_METRICS_TIMER_DIR/bd-42.timer" ]
  
  # Log start
  metrics_ticket_start "bd-42" "developer-backend"
  
  # Attendre 1 seconde
  sleep 0.1
  
  # Get duration
  duration=$(metrics_get_duration "bd-42")
  [ -n "$duration" ]
  [ "$duration" -ge 0 ]
  
  # Log complete avec duration
  metrics_ticket_complete "bd-42" "developer-backend" "$duration"
  
  # Clear timer
  metrics_clear_timer "bd-42"
  [ ! -f "$_METRICS_TIMER_DIR/bd-42.timer" ]
  
  # Vérifier contenu fichier
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"ticket_start"'* ]]
  [[ "$output" == *'"event":"ticket_complete"'* ]]
  [[ "$output" == *'"duration_seconds":'* ]]
}

@test "Intégration : workflow avec review et correction" {
  metrics_ticket_start "bd-42" "developer-backend"
  metrics_review_cycle "bd-42" "1"
  metrics_correction "bd-42" "lint errors"
  metrics_review_cycle "bd-42" "2"
  metrics_ticket_complete "bd-42" "developer-backend" "1800"
  
  run metrics_count_completed
  [ "$output" = "1" ]
  
  run cat "$_METRICS_FILE"
  [[ "$output" == *'"event":"review_cycle"'* ]]
  [[ "$output" == *'"event":"correction"'* ]]
}

@test "Intégration : agrégation metrics" {
  # Créer des données
  metrics_ticket_complete "bd-42" "" "900"
  metrics_ticket_complete "bd-43" "" "600"
  metrics_review_cycle "bd-42" "1"
  metrics_correction "bd-42" "lint errors"
  metrics_websearch "bd-42" "websearch" "CVE"
  
  # Agréger
  metrics_aggregate
  
  [ "$METRICS_TOTAL_TICKETS" = "2" ]
  [ "$METRICS_AVG_DURATION" = "750" ]
  [ "$METRICS_AVG_DURATION_FMT" = "12m 30s" ]
  [ "$METRICS_AVG_CYCLES" = "0.5" ]
  [ "$METRICS_WEBSEARCH_COUNT" = "1" ]
}
