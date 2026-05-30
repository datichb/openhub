#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/api-keys.sh
# Fonctions testées : api_keys_load_cache, _api_keys_get,
#                     get_project_api_model, get_project_api_provider,
#                     get_project_api_key, get_project_api_base_url,
#                     get_project_api_region, api_keys_entry_exists,
#                     remove_api_keys_section

load helpers

setup() {
  common_setup
  
  # Sourcer api-keys.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/api-keys.sh"
  
  # Créer un api-keys.local.md de test
  cp "$BATS_TEST_DIRNAME/fixtures/configs/api-keys-multi-providers.local.md" \
     "$API_KEYS_FILE"
}

teardown() {
  common_teardown
}

# ── _api_keys_get (sans cache) ─────────────────────────────────────────────────

@test "_api_keys_get : lit provider" {
  run _api_keys_get "PROJ-ANTHROPIC" "provider"
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic" ]
}

@test "_api_keys_get : lit model" {
  run _api_keys_get "PROJ-ANTHROPIC" "model"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "_api_keys_get : lit api_key" {
  run _api_keys_get "PROJ-ANTHROPIC" "api_key"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-ant-test123" ]
}

@test "_api_keys_get : lit base_url" {
  run _api_keys_get "PROJ-OPENAI" "base_url"
  [ "$status" -eq 0 ]
  [ "$output" = "https://api.openai.com/v1" ]
}

@test "_api_keys_get : lit region pour Bedrock" {
  run _api_keys_get "PROJ-BEDROCK" "region"
  [ "$status" -eq 0 ]
  [ "$output" = "us-east-1" ]
}

@test "_api_keys_get : retourne vide si clé absente" {
  run _api_keys_get "PROJ-MINIMAL" "provider"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "_api_keys_get : retourne vide si section inexistante" {
  run _api_keys_get "PROJ-NONEXISTENT" "model"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "_api_keys_get : gère clés complexes (agent_models.agents.X)" {
  # Ajouter une clé complexe au fichier
  cat >> "$API_KEYS_FILE" <<'EOF'

[PROJ-COMPLEX]
model=claude-sonnet-4-5
agent_models.agents.orchestrator=claude-opus-4
EOF
  
  run _api_keys_get "PROJ-COMPLEX" "agent_models.agents.orchestrator"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

# ── api_keys_load_cache ────────────────────────────────────────────────────────

@test "api_keys_load_cache : charge toutes les valeurs en mémoire" {
  api_keys_load_cache "PROJ-ANTHROPIC"
  
  # Vérifier que le cache est marqué comme chargé
  [ "$_API_KEYS_CACHE_LOADED" = "1" ]
  [ "$_API_KEYS_CACHE_PROJECT_ID" = "PROJ-ANTHROPIC" ]
  [ "$_API_KEYS_CACHE_PROVIDER" = "anthropic" ]
  [ "$_API_KEYS_CACHE_MODEL" = "claude-opus-4" ]
  [ "$_API_KEYS_CACHE_KEY" = "sk-ant-test123" ]
}

@test "api_keys_load_cache : gère projet avec région (Bedrock)" {
  api_keys_load_cache "PROJ-BEDROCK"
  
  [ "$_API_KEYS_CACHE_REGION" = "us-east-1" ]
  [ "$_API_KEYS_CACHE_PROVIDER" = "bedrock" ]
}

@test "api_keys_load_cache : gère projet avec base_url" {
  api_keys_load_cache "PROJ-OPENAI"
  
  [ "$_API_KEYS_CACHE_BASE_URL" = "https://api.openai.com/v1" ]
  [ "$_API_KEYS_CACHE_PROVIDER" = "openai" ]
}

@test "api_keys_load_cache : gère projet inexistant" {
  api_keys_load_cache "PROJ-NONEXISTENT"
  
  [ "$_API_KEYS_CACHE_LOADED" = "1" ]
  [ "$_API_KEYS_CACHE_PROJECT_ID" = "PROJ-NONEXISTENT" ]
  [ -z "$_API_KEYS_CACHE_PROVIDER" ]
  [ -z "$_API_KEYS_CACHE_MODEL" ]
}

# ── _api_keys_get (avec cache) ─────────────────────────────────────────────────

@test "_api_keys_get : utilise cache si chargé" {
  api_keys_load_cache "PROJ-ANTHROPIC"
  
  # L'appel doit utiliser le cache (pas de lecture fichier)
  run _api_keys_get "PROJ-ANTHROPIC" "provider"
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic" ]
}

@test "_api_keys_get : utilise cache pour model" {
  api_keys_load_cache "PROJ-BEDROCK"
  
  run _api_keys_get "PROJ-BEDROCK" "model"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-sonnet-4-6" ]
}

@test "_api_keys_get : utilise cache pour region" {
  api_keys_load_cache "PROJ-BEDROCK"
  
  run _api_keys_get "PROJ-BEDROCK" "region"
  [ "$status" -eq 0 ]
  [ "$output" = "us-east-1" ]
}

@test "_api_keys_get : fallback awk si clé non supportée par cache" {
  api_keys_load_cache "PROJ-ANTHROPIC"
  
  # Ajouter clé complexe
  cat >> "$API_KEYS_FILE" <<'EOF'

[PROJ-TEST]
agent_models.families.planning=claude-opus-4
EOF
  
  run _api_keys_get "PROJ-TEST" "agent_models.families.planning"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

# ── get_project_api_model ──────────────────────────────────────────────────────

@test "get_project_api_model : retourne model anthropic" {
  run get_project_api_model "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "get_project_api_model : retourne model bedrock" {
  run get_project_api_model "PROJ-BEDROCK"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-sonnet-4-6" ]
}

@test "get_project_api_model : retourne model openai" {
  run get_project_api_model "PROJ-OPENAI"
  [ "$status" -eq 0 ]
  [ "$output" = "gpt-4" ]
}

@test "get_project_api_model : retourne vide si pas de model" {
  # Créer projet sans model
  cat >> "$API_KEYS_FILE" <<'EOF'

[PROJ-NO-MODEL]
provider=anthropic
EOF
  
  run get_project_api_model "PROJ-NO-MODEL"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── get_project_api_provider ───────────────────────────────────────────────────

@test "get_project_api_provider : retourne anthropic" {
  run get_project_api_provider "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic" ]
}

@test "get_project_api_provider : retourne bedrock" {
  run get_project_api_provider "PROJ-BEDROCK"
  [ "$status" -eq 0 ]
  [ "$output" = "bedrock" ]
}

@test "get_project_api_provider : retourne openai" {
  run get_project_api_provider "PROJ-OPENAI"
  [ "$status" -eq 0 ]
  [ "$output" = "openai" ]
}

# ── get_project_api_key ────────────────────────────────────────────────────────

@test "get_project_api_key : retourne clé anthropic" {
  run get_project_api_key "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-ant-test123" ]
}

@test "get_project_api_key : retourne clé bedrock" {
  run get_project_api_key "PROJ-BEDROCK"
  [ "$status" -eq 0 ]
  [ "$output" = "AKIATEST123" ]
}

@test "get_project_api_key : retourne clé openai" {
  run get_project_api_key "PROJ-OPENAI"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-openai-test456" ]
}

# ── get_project_api_base_url ───────────────────────────────────────────────────

@test "get_project_api_base_url : retourne base_url openai" {
  run get_project_api_base_url "PROJ-OPENAI"
  [ "$status" -eq 0 ]
  [ "$output" = "https://api.openai.com/v1" ]
}

@test "get_project_api_base_url : retourne vide si pas de base_url" {
  run get_project_api_base_url "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── get_project_api_region ─────────────────────────────────────────────────────

@test "get_project_api_region : retourne région bedrock" {
  run get_project_api_region "PROJ-BEDROCK"
  [ "$status" -eq 0 ]
  [ "$output" = "us-east-1" ]
}

@test "get_project_api_region : retourne vide si pas de région" {
  run get_project_api_region "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── api_keys_entry_exists ──────────────────────────────────────────────────────

@test "api_keys_entry_exists : retourne 0 si section existe" {
  run api_keys_entry_exists "PROJ-ANTHROPIC"
  [ "$status" -eq 0 ]
}

@test "api_keys_entry_exists : retourne 1 si section n'existe pas" {
  run api_keys_entry_exists "PROJ-NONEXISTENT"
  [ "$status" -eq 1 ]
}

@test "api_keys_entry_exists : ne matche pas préfixe partiel" {
  # Ajouter [PROJ]
  cat >> "$API_KEYS_FILE" <<'EOF'

[PROJ]
model=test
EOF
  
  # PROJ-ANTHROPIC ne doit pas matcher PROJ
  run api_keys_entry_exists "PROJ"
  [ "$status" -eq 0 ]
  
  # Et inversement
  run api_keys_entry_exists "PROJ-ANTHROPIC-EXTRA"
  [ "$status" -eq 1 ]
}

# ── remove_api_keys_section ────────────────────────────────────────────────────

@test "remove_api_keys_section : supprime section complète" {
  # Vérifier que la section existe
  api_keys_entry_exists "PROJ-ANTHROPIC"
  
  # Supprimer
  remove_api_keys_section "PROJ-ANTHROPIC"
  
  # Vérifier qu'elle n'existe plus
  run api_keys_entry_exists "PROJ-ANTHROPIC"
  [ "$status" -eq 1 ]
}

@test "remove_api_keys_section : préserve autres sections" {
  # Supprimer une section
  remove_api_keys_section "PROJ-ANTHROPIC"
  
  # Vérifier que les autres existent toujours
  run api_keys_entry_exists "PROJ-BEDROCK"
  [ "$status" -eq 0 ]
  
  run api_keys_entry_exists "PROJ-OPENAI"
  [ "$status" -eq 0 ]
}

@test "remove_api_keys_section : supprime lignes vides précédentes" {
  remove_api_keys_section "PROJ-BEDROCK"
  
  # Vérifier que le fichier ne contient pas de doubles lignes vides
  blank_count=$(awk 'BEGIN{c=0;max=0} /^$/{c++}; /./{if(c>max)max=c;c=0} END{print max}' "$API_KEYS_FILE")
  [ "$blank_count" -le 1 ]
}

@test "remove_api_keys_section : gère section inexistante" {
  run remove_api_keys_section "PROJ-NONEXISTENT"
  [ "$status" -eq 0 ]
}

@test "remove_api_keys_section : gère fichier absent" {
  rm -f "$API_KEYS_FILE"
  run remove_api_keys_section "PROJ-ANY"
  [ "$status" -eq 0 ]
}

# ── Intégration : workflow complet ─────────────────────────────────────────────

@test "Intégration : lecture avec et sans cache" {
  # Sans cache
  result1=$(get_project_api_model "PROJ-ANTHROPIC")
  [ "$result1" = "claude-opus-4" ]
  
  # Charger cache
  api_keys_load_cache "PROJ-ANTHROPIC"
  
  # Avec cache (doit retourner même résultat)
  result2=$(get_project_api_model "PROJ-ANTHROPIC")
  [ "$result2" = "claude-opus-4" ]
  
  # Les deux résultats doivent être identiques
  [ "$result1" = "$result2" ]
}

@test "Intégration : cache multi-projets" {
  # Charger cache projet 1
  api_keys_load_cache "PROJ-ANTHROPIC"
  result1=$(get_project_api_provider "PROJ-ANTHROPIC")
  [ "$result1" = "anthropic" ]
  
  # Lire projet 2 (cache différent, doit fallback)
  result2=$(get_project_api_provider "PROJ-BEDROCK")
  [ "$result2" = "bedrock" ]
  
  # Recharger cache projet 1
  api_keys_load_cache "PROJ-ANTHROPIC"
  result3=$(get_project_api_provider "PROJ-ANTHROPIC")
  [ "$result3" = "anthropic" ]
}
