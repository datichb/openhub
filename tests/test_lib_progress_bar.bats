#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/progress-bar.sh
# Fonctions testées : _progress_bar, _progress_done, _progress_summary, _progress_disable

load helpers

setup() {
  common_setup
  
  # Sourcer colors.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/colors.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/progress-bar.sh"
  
  # Forcer l'activation de la progression pour les tests
  export _PROGRESS_ENABLED=true
}

teardown() {
  common_teardown
}

# ── _progress_disable ───────────────────────────────────────────────────────

@test "_progress_disable : désactive la progression" {
  _progress_disable
  
  [ "$_PROGRESS_ENABLED" = false ]
}

# ── _progress_bar ───────────────────────────────────────────────────────────

@test "_progress_bar : affiche barre de progression" {
  run _progress_bar 5 10 "test-item"
  [ "$status" -eq 0 ]
}

@test "_progress_bar : calcule pourcentage correct" {
  run _progress_bar 50 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"50%"* ]]
}

@test "_progress_bar : affiche 0% au début" {
  run _progress_bar 0 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"0%"* ]]
}

@test "_progress_bar : affiche 100% à la fin" {
  run _progress_bar 100 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"100%"* ]]
}

@test "_progress_bar : gère total=0 sans crash" {
  run _progress_bar 0 0
  [ "$status" -eq 0 ]
}

@test "_progress_bar : affiche label" {
  run _progress_bar 5 10 "my-agent"
  [ "$status" -eq 0 ]
  [[ "$output" == *"my-agent"* ]]
}

@test "_progress_bar : status error affiche en rouge" {
  run _progress_bar 5 10 "failed-item" "error"
  [ "$status" -eq 0 ]
  [[ "$output" == *"✗"* ]]
}

@test "_progress_bar : skip si progression désactivée" {
  _progress_disable
  
  run _progress_bar 5 10
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── _progress_done ──────────────────────────────────────────────────────────

@test "_progress_done : finalise la progression" {
  run _progress_done
  [ "$status" -eq 0 ]
}

@test "_progress_done : skip si progression désactivée" {
  _progress_disable
  
  run _progress_done
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── _progress_summary ───────────────────────────────────────────────────────

@test "_progress_summary : affiche récapitulatif avec titre" {
  run _progress_summary "Phase completed"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Phase completed"* ]]
}

@test "_progress_summary : affiche lignes de résumé" {
  run _progress_summary "Done" "Line 1" "Line 2"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Line 1"* ]]
  [[ "$output" == *"Line 2"* ]]
}

@test "_progress_summary : gère sous-items indentés" {
  run _progress_summary "Done" "Main item" "  Sub-item"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Sub-item"* ]]
}

@test "_progress_summary : gère récapitulatif vide" {
  run _progress_summary "Done"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Done"* ]]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : séquence progress complète" {
  # Progression de 0 à 100%
  _progress_bar 0 10 "agent-1"
  _progress_bar 5 10 "agent-5"
  _progress_bar 10 10 "agent-10"
  _progress_done
  
  # Résumé
  run _progress_summary "All agents deployed" "10 agents total"
  [ "$status" -eq 0 ]
}

@test "Intégration : progression avec erreur" {
  _progress_bar 1 3 "agent-1"
  _progress_bar 2 3 "agent-2"
  _progress_bar 3 3 "agent-3-failed" "error"
  _progress_done
}
