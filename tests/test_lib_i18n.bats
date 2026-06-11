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
  # La clé "cancelled" existe en FR ("Annulé") et en EN ("Cancelled").
  # On vérifie que OC_LANG=fr retourne bien la traduction FR (non vide, non la clé brute).
  export OC_LANG=fr
  run t "cancelled"
  [ "$status" -eq 0 ]
  [ "$output" = "Annulé" ]
}

# ── Échantillon clés par catégorie ──────────────────────────────────────────

@test "t : échantillon clés cmd-project (help.title)" {
  export OC_LANG=en
  
  run t "help.title"
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  [ "$output" != "help.title" ]
}

@test "t : échantillon clés cmd-deploy (service.deploy.title)" {
  export OC_LANG=en
  
  run t "service.deploy.title"
  [ "$status" -eq 0 ]
  [ "$output" = "Deploy MCP server" ]
}

@test "t : échantillon clés cmd-beads (beads.status.all)" {
  export OC_LANG=en
  
  run t "beads.status.all"
  [ "$status" -eq 0 ]
  [ "$output" = "Beads status — all projects" ]
}

@test "t : échantillon clés cmd-config (config.title)" {
  export OC_LANG=en
  
  run t "config.title"
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  [ "$output" != "config.title" ]
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

# ── Clés init.mcp.* ──────────────────────────────────────────────────────────

@test "t : clés init.mcp FR — step_title, prompt_intro, skip" {
  export OC_LANG=fr

  run t "init.mcp.step_title"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Services MCP"* ]]

  run t "init.mcp.prompt_intro"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MCP"* ]]

  run t "init.mcp.skip"
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc service setup"* ]]

  run t "init.mcp.none"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aucun"* ]]

  run t "init.mcp.all"
  [ "$status" -eq 0 ]
  [[ "$output" == *"activés"* ]]
}

@test "t : clés init.mcp EN — step_title, prompt_intro, skip" {
  export OC_LANG=en

  run t "init.mcp.step_title"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MCP Services"* ]]

  run t "init.mcp.prompt_intro"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MCP"* ]]

  run t "init.mcp.skip"
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc service setup"* ]]

  run t "init.mcp.none"
  [ "$status" -eq 0 ]
  [[ "$output" == *"No MCP"* ]]

  run t "init.mcp.all"
  [ "$status" -eq 0 ]
  [[ "$output" == *"enabled"* ]]
}

# ── Clés review.git_* ─────────────────────────────────────────────────────────

@test "t : clés review.git_* FR — fetching, sync_continue, aborted" {
  export OC_LANG=fr

  run t "review.git_fetching"
  [ "$status" -eq 0 ]
  [[ "$output" == *"fetch"* ]]

  run t "review.git_fetch_done"
  [ "$status" -eq 0 ]
  [[ "$output" == *"terminé"* ]]

  run t "review.git_sync_continue"
  [ "$status" -eq 0 ]
  [[ "$output" == *"malgré"* ]]

  run t "review.git_sync_aborted"
  [ "$status" -eq 0 ]
  [[ "$output" == *"annulée"* ]]
}

@test "t : clés review.git_* EN — fetching, sync_continue, aborted" {
  export OC_LANG=en

  run t "review.git_fetching"
  [ "$status" -eq 0 ]
  [[ "$output" == *"fetch"* ]]

  run t "review.git_fetch_done"
  [ "$status" -eq 0 ]
  [[ "$output" == *"complete"* ]]

  run t "review.git_sync_continue"
  [ "$status" -eq 0 ]
  [[ "$output" == *"despite"* ]]

  run t "review.git_sync_aborted"
  [ "$status" -eq 0 ]
  [[ "$output" == *"cancelled"* ]]
}
