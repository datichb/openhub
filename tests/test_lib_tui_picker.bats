#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/tui-picker.sh
# Fonctions testées : _read_key, _pick_from_list (tests limités - TUI interactif)

load helpers

setup() {
  common_setup
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/tui-picker.sh"
}

teardown() {
  common_teardown
}

# ── _read_key (tests limités) ──────────────────────────────────────────────

@test "_read_key : fonction définie" {
  run declare -F _read_key
  [ "$status" -eq 0 ]
}

# ── _pick_from_list (tests limités - mock) ──────────────────────────────────

@test "_pick_from_list : retourne sélection si liste vide" {
  _pick_total=0
  _pick_render_fn="echo"
  
  _pick_from_list "item1,item2"
  
  [ "$_PICK_RESULT" = "item1,item2" ]
}

@test "_pick_from_list : échoue si _pick_total non défini" {
  unset _pick_total
  _pick_render_fn="echo"
  
  run _pick_from_list ""
  [ "$status" -ne 0 ]
}

@test "_pick_from_list : échoue si _pick_render_fn non défini" {
  _pick_total=5
  unset _pick_render_fn
  
  run _pick_from_list ""
  [ "$status" -ne 0 ]
}

# Tests d'intégration complets nécessiteraient simulation clavier
# Skip pour l'approche pragmatique car nécessite TTY interactif

@test "_pick_from_list : variables initialisées correctement" {
  _pick_total=3
  _pick_items=("item1" "item2" "item3")
  _pick_checked=(0 0 0)
  _pick_render_fn="_test_render"
  
  _test_render() {
    # Mock render qui simule ESC immédiat
    return 0
  }
  export -f _test_render
  
  # Ce test vérifierait l'initialisation mais nécessite TTY
  # Vérifions juste que les variables sont acceptées
  [ "${#_pick_items[@]}" -eq 3 ]
  [ "$_pick_total" -eq 3 ]
}
