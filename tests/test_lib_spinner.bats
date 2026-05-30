#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/spinner.sh
# Fonctions testées : _spinner_start, _spinner_stop

load helpers

setup() {
  common_setup
  
  # Sourcer colors.sh pour les couleurs
  source "$BATS_TEST_DIRNAME/../scripts/lib/colors.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/spinner.sh"
}

teardown() {
  # S'assurer que le spinner est arrêté
  _spinner_stop "" 0 2>/dev/null || true
  common_teardown
}

# ── _spinner_start ──────────────────────────────────────────────────────────

@test "_spinner_start : démarre spinner en background" {
  _spinner_start "Test message"
  
  # Vérifier que le PID est défini
  [ -n "$_SPINNER_PID" ]
  
  # Vérifier que le process existe
  run kill -0 "$_SPINNER_PID"
  [ "$status" -eq 0 ]
  
  # Cleanup
  _spinner_stop "Done" 0
}

@test "_spinner_start : utilise message par défaut" {
  _spinner_start
  
  [ -n "$_SPINNER_PID" ]
  
  _spinner_stop "" 0
}

@test "_spinner_start : accepte message custom" {
  _spinner_start "Custom loading..."
  
  [ -n "$_SPINNER_PID" ]
  [ "$_SPINNER_MSG" = "Custom loading..." ]
  
  _spinner_stop "" 0
}

# ── _spinner_stop ───────────────────────────────────────────────────────────

@test "_spinner_stop : arrête le spinner" {
  _spinner_start "Test"
  local pid=$_SPINNER_PID
  
  _spinner_stop "Finished" 0
  
  # Vérifier que le PID est vidé
  [ -z "$_SPINNER_PID" ]
  
  # Vérifier que le process n'existe plus
  run kill -0 "$pid"
  [ "$status" -ne 0 ]
}

@test "_spinner_stop : affiche message de succès si code 0" {
  _spinner_start "Test"
  
  run _spinner_stop "Success" 0
  [ "$status" -eq 0 ]
  [[ "$output" == *"Success"* ]]
}

@test "_spinner_stop : affiche message d'erreur si code non-0" {
  _spinner_start "Test"
  
  run _spinner_stop "Failed" 1
  [ "$status" -eq 0 ]
  [[ "$output" == *"Failed"* ]]
}

@test "_spinner_stop : ne fait rien si pas de spinner actif" {
  run _spinner_stop "Message" 0
  [ "$status" -eq 0 ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : cycle complet start/stop" {
  # Start
  _spinner_start "Processing..."
  [ -n "$_SPINNER_PID" ]
  
  # Attendre un peu
  sleep 0.2
  
  # Stop
  _spinner_stop "Completed" 0
  [ -z "$_SPINNER_PID" ]
}

@test "Intégration : multiple cycles" {
  # Cycle 1
  _spinner_start "Step 1"
  sleep 0.1
  _spinner_stop "Step 1 done" 0
  
  # Cycle 2
  _spinner_start "Step 2"
  sleep 0.1
  _spinner_stop "Step 2 done" 0
}
