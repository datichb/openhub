#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/i18n.sh
# Fonctions testées : t(), t_en() - échantillon représentatif de clés

load helpers

setup() {
  common_setup
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/i18n.sh"
}

teardown() {
  common_teardown
}

# ── Fonction t() - EN ───────────────────────────────────────────────────────

@test "t : retourne texte EN par défaut" {
  export OC_LANG=en
  
  run t "cmd.unknown"
  [ "$status" -eq 0 ]
  [ -n "$output" ]
}

@test "t : clés core EN" {
  export OC_LANG=en
  
  run t "cancelled"
  [[ "$output" == *"cancel"* ]] || [[ "$output" == "Cancelled" ]]
  
  run t "project_id.required"
  [ -n "$output" ]
}

@test "t : clés help EN" {
  export OC_LANG=en
  
  run t "help.title"
  [[ "$output" == *"opencode"* ]]
  
  run t "help.usage"
  [[ "$output" == *"sage"* ]]
}

@test "t : clés init EN" {
  export OC_LANG=en
  
  run t "init.detect.existing"
  [ -n "$output" ]
  
  run t "init.adopt.prompt"
  [ -n "$output" ]
}

# ── Fonction t() - FR ───────────────────────────────────────────────────────

@test "t : retourne texte FR si OC_LANG=fr" {
  export OC_LANG=fr
  
  run t "cmd.unknown"
  [[ "$output" == *"Commande inconnue"* ]]
}

@test "t : clés core FR" {
  export OC_LANG=fr
  
  run t "cancelled"
  [[ "$output" == *"Annulé"* ]]
  
  run t "project_id.required"
  [[ "$output" == *"PROJECT_ID requis"* ]]
}

@test "t : clés help FR" {
  export OC_LANG=fr
  
  run t "help.title"
  [[ "$output" == *"opencode"* ]]
  
  run t "help.section.setup"
  [ -n "$output" ]
}

@test "t : clés beads FR" {
  export OC_LANG=fr
  
  run t "beads.not_installed"
  [ -n "$output" ]
}

# ── Fonction t_en() ─────────────────────────────────────────────────────────

@test "t_en : force EN même si OC_LANG=fr" {
  export OC_LANG=fr
  
  # t() retourne FR
  run t "cancelled"
  local output_fr="$output"
  
  # t_en() force EN
  run t_en "cancelled"
  [ "$output" != "$output_fr" ]
}

# ── Fallback ────────────────────────────────────────────────────────────────

@test "t : retourne clé elle-même si inexistante" {
  export OC_LANG=en
  
  run t "nonexistent.key.test"
  [ "$output" = "nonexistent.key.test" ]
}

@test "t : fallback FR -> EN si clé FR absente" {
  export OC_LANG=fr
  
  # Tester une clé qui pourrait n'exister qu'en EN
  run t "some.english.only.key"
  [ -n "$output" ]
}

# ── Échantillon clés par catégorie ──────────────────────────────────────────

@test "t : échantillon clés cmd-project" {
  export OC_LANG=en
  
  run t "project.list.title"
  [ -n "$output" ]
}

@test "t : échantillon clés cmd-deploy" {
  export OC_LANG=en
  
  run t "deploy.title"
  [ -n "$output" ]
}

@test "t : échantillon clés cmd-beads" {
  export OC_LANG=en
  
  run t "beads.status.title"
  [ -n "$output" ]
}

@test "t : échantillon clés cmd-config" {
  export OC_LANG=en
  
  run t "config.show.title"
  [ -n "$output" ]
}

# ── Tests variantes linguistiques ───────────────────────────────────────────

@test "t : gère OC_LANG vide (défaut EN)" {
  unset OC_LANG
  
  run t "help.title"
  [ -n "$output" ]
}

@test "t : gère OC_LANG invalide (défaut EN)" {
  export OC_LANG=de
  
  run t "help.title"
  [ -n "$output" ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : switch langue EN -> FR" {
  export OC_LANG=en
  run t "cancelled"
  local output_en="$output"
  
  export OC_LANG=fr
  run t "cancelled"
  local output_fr="$output"
  
  [ "$output_en" != "$output_fr" ]
  [[ "$output_fr" == *"Annulé"* ]]
}

@test "Intégration : multiple clés FR" {
  export OC_LANG=fr
  
  run t "cmd.unknown"
  [[ "$output" == *"Commande inconnue"* ]]
  
  run t "invalid_choice"
  [[ "$output" == *"Choix invalide"* ]]
  
  run t "no_modification"
  [[ "$output" == *"Aucune modification"* ]]
}
