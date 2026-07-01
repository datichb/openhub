#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/providers.sh
# Fonctions testées : get_provider_info, get_provider_bool, provider_exists,
#                     get_effective_provider, get_hub_default_provider,
#                     get_provider_default_model, list_all_providers

load helpers

setup() {
  common_setup
  
  # Sourcer providers.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/providers.sh"
  
  # Créer un providers.json de test
  mkdir -p "$TEST_DIR/config"
  export PROVIDERS_FILE="$TEST_DIR/config/providers.json"
  cat > "$PROVIDERS_FILE" <<'EOF'
{
  "providers": {
    "anthropic": {
      "label": "Anthropic (Claude)",
      "default_model": "claude-sonnet-4-6",
      "prefix": "anthropic",
      "requires_api_key": true
    },
    "bedrock": {
      "label": "Amazon Bedrock",
      "default_model": "claude-sonnet-4-6",
      "prefix": "amazon-bedrock",
      "requires_api_key": true,
      "requires_aws_region": true
    },
    "github-copilot": {
      "label": "GitHub Copilot",
      "default_model": "claude-sonnet-4.5",
      "prefix": "github-copilot",
      "requires_api_key": false
    },
    "ollama": {
      "label": "Ollama (Local)",
      "default_model": "llama3.1",
      "prefix": null,
      "requires_api_key": false
    },
    "test-provider": {
      "label": "Test Provider",
      "default_model": "test-model",
      "prefix": "test"
    }
  }
}
EOF
  
  # Créer un hub.json de test
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "default_provider": {
    "name": "anthropic",
    "api_key": "sk-test-123",
    "base_url": "https://api.anthropic.com",
    "model": "claude-opus-4"
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
EOF

  # Créer un api-keys.local.md de test
  cat > "$API_KEYS_FILE" <<'EOF'
# API Keys de test

## TEST-PROJ
provider: bedrock
api_key: test-bedrock-key
model: claude-haiku-4-5
EOF
}

teardown() {
  common_teardown
}

# ── get_provider_info ─────────────────────────────────────────────────────────

@test "get_provider_info : retourne le label d'un provider" {
  result=$(get_provider_info "anthropic" "label")
  [ "$result" = "Anthropic (Claude)" ]
}

@test "get_provider_info : retourne le default_model" {
  result=$(get_provider_info "bedrock" "default_model")
  [ "$result" = "claude-sonnet-4-6" ]
}

@test "get_provider_info : retourne le prefix" {
  result=$(get_provider_info "anthropic" "prefix")
  [ "$result" = "anthropic" ]
}

@test "get_provider_info : retourne vide si provider inexistant" {
  result=$(get_provider_info "inexistant" "label")
  [ -z "$result" ]
}

@test "get_provider_info : retourne vide si champ inexistant" {
  result=$(get_provider_info "anthropic" "champ_inexistant")
  [ -z "$result" ]
}

# ── get_provider_bool ─────────────────────────────────────────────────────────

@test "get_provider_bool : retourne true pour requires_api_key=true" {
  result=$(get_provider_bool "anthropic" "requires_api_key")
  [ "$result" = "true" ]
}

@test "get_provider_bool : retourne false pour requires_api_key=false" {
  result=$(get_provider_bool "github-copilot" "requires_api_key")
  [ "$result" = "false" ]
}

@test "get_provider_bool : retourne true par défaut si champ absent (null)" {
  result=$(get_provider_bool "test-provider" "requires_api_key")
  [ "$result" = "true" ]
}

@test "get_provider_bool : gère requires_aws_region" {
  result=$(get_provider_bool "bedrock" "requires_aws_region")
  [ "$result" = "true" ]
}

@test "get_provider_bool : retourne true pour provider inexistant" {
  result=$(get_provider_bool "inexistant" "requires_api_key")
  [ "$result" = "true" ]
}

# ── provider_exists ───────────────────────────────────────────────────────────

@test "provider_exists : retourne 0 si provider existe" {
  provider_exists "anthropic"
}

@test "provider_exists : retourne 1 si provider n'existe pas" {
  ! provider_exists "inexistant"
}

@test "provider_exists : gère tous les providers du catalogue" {
  provider_exists "bedrock"
  provider_exists "github-copilot"
  provider_exists "ollama"
  provider_exists "test-provider"
}

# ── list_all_providers ────────────────────────────────────────────────────────

@test "list_all_providers : liste tous les providers" {
  require_command "jq"
  
  result=$(list_all_providers)
  
  echo "$result" | grep -q "anthropic"
  echo "$result" | grep -q "bedrock"
  echo "$result" | grep -q "github-copilot"
  echo "$result" | grep -q "ollama"
  echo "$result" | grep -q "test-provider"
}

@test "list_all_providers : retourne 5 providers" {
  require_command "jq"
  
  count=$(list_all_providers | wc -l | tr -d ' ')
  [ "$count" -eq 5 ]
}

# ── get_provider_default_model ────────────────────────────────────────────────

@test "get_provider_default_model : retourne le modèle par défaut" {
  result=$(get_provider_default_model "anthropic")
  [ "$result" = "claude-sonnet-4-6" ]
}

@test "get_provider_default_model : gère bedrock" {
  result=$(get_provider_default_model "bedrock")
  [ "$result" = "claude-sonnet-4-6" ]
}

@test "get_provider_default_model : gère github-copilot" {
  result=$(get_provider_default_model "github-copilot")
  [ "$result" = "claude-sonnet-4.5" ]
}

@test "get_provider_default_model : gère ollama" {
  result=$(get_provider_default_model "ollama")
  [ "$result" = "llama3.1" ]
}

@test "get_provider_default_model : retourne vide si provider inexistant" {
  result=$(get_provider_default_model "inexistant")
  [ -z "$result" ]
}

# ── get_hub_default_provider ──────────────────────────────────────────────────

@test "get_hub_default_provider : lit le provider depuis hub.json" {
  result=$(get_hub_default_provider)
  [ "$result" = "anthropic" ]
}

@test "get_hub_default_provider : utilise un cache pour éviter lectures multiples" {
  # Le cache utilise des variables shell globales — doit être testé dans un
  # seul shell continu pour que le cache persiste entre les appels.
  run bash -c '
    source "'"$BATS_TEST_DIRNAME"'/../scripts/common.sh"
    export HUB_CONFIG="'"$HUB_CONFIG"'"
    export PROVIDERS_FILE="'"$PROVIDERS_FILE"'"
    _HUB_DEFAULT_PROVIDER_CACHE=""
    _HUB_DEFAULT_PROVIDER_CACHE_LOADED=0

    # Premier appel direct (pas en subshell) pour charger le cache
    get_hub_default_provider >/dev/null

    # Modifier hub.json après le premier appel
    echo "{\"default_provider\":{\"name\":\"bedrock\"}}" > "$HUB_CONFIG"

    # Deuxième appel : doit retourner la valeur en cache (pas le fichier modifié)
    get_hub_default_provider
  '
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic" ]
}

@test "get_hub_default_provider : retourne vide si hub.json inexistant" {
  rm "$HUB_CONFIG"
  _HUB_DEFAULT_PROVIDER_CACHE_LOADED=0
  
  run get_hub_default_provider
  [ "$status" -ne 0 ]
}

# ── get_hub_default_api_key ───────────────────────────────────────────────────

@test "get_hub_default_api_key : lit la clé API depuis hub.json" {
  result=$(get_hub_default_api_key)
  [ "$result" = "sk-test-123" ]
}

@test "get_hub_default_api_key : retourne vide si clé absente" {
  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {
    "name": "anthropic"
  }
}
EOF
  
  result=$(get_hub_default_api_key)
  [ -z "$result" ]
}

# ── get_hub_default_base_url ──────────────────────────────────────────────────

@test "get_hub_default_base_url : lit la base_url depuis hub.json" {
  result=$(get_hub_default_base_url)
  [ "$result" = "https://api.anthropic.com" ]
}

# ── get_hub_default_model ─────────────────────────────────────────────────────

@test "get_hub_default_model : lit le modèle depuis hub.json" {
  result=$(get_hub_default_model)
  [ "$result" = "claude-opus-4" ]
}

# ── get_effective_provider ────────────────────────────────────────────────────

@test "get_effective_provider : priorité 0 - override explicite" {
  result=$(get_effective_provider "" "ollama")
  [ "$result" = "ollama" ]
}

@test "get_effective_provider : priorité 1 - provider du projet" {
  # Mock get_project_api_provider
  get_project_api_provider() {
    [ "$1" = "TEST-PROJ" ] && echo "bedrock"
  }
  export -f get_project_api_provider
  
  result=$(get_effective_provider "TEST-PROJ" "")
  [ "$result" = "bedrock" ]
}

@test "get_effective_provider : priorité 2 - hub default provider" {
  # Mock get_project_api_provider retourne vide
  get_project_api_provider() {
    return 0
  }
  export -f get_project_api_provider
  
  result=$(get_effective_provider "PROJ-SANS-PROVIDER" "")
  [ "$result" = "anthropic" ]
}

@test "get_effective_provider : retourne vide si hub.json n'a pas de default_provider" {
  # Mock get_project_api_provider retourne vide
  get_project_api_provider() {
    return 0
  }
  export -f get_project_api_provider

  # Hub sans default_provider — ne doit plus fallback sur anthropic
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test"
}
EOF
  _HUB_DEFAULT_PROVIDER_CACHE_LOADED=0

  result=$(get_effective_provider "" "")
  [ -z "$result" ]
}

@test "get_effective_provider : override prime sur projet et hub" {
  get_project_api_provider() {
    echo "bedrock"
  }
  export -f get_project_api_provider
  
  result=$(get_effective_provider "TEST-PROJ" "ollama")
  [ "$result" = "ollama" ]
}

# ── get_effective_llm_model ───────────────────────────────────────────────────

@test "get_effective_llm_model : priorité 1 - model du projet" {
  # Mock get_project_api_model
  get_project_api_model() {
    [ "$1" = "TEST-PROJ" ] && echo "claude-haiku-4-5"
  }
  export -f get_project_api_model
  
  result=$(get_effective_llm_model "TEST-PROJ")
  [ "$result" = "claude-haiku-4-5" ]
}

@test "get_effective_llm_model : priorité 2 - hub default model" {
  # Mock get_project_api_model retourne vide
  get_project_api_model() {
    return 0
  }
  export -f get_project_api_model
  
  result=$(get_effective_llm_model "PROJ-SANS-MODEL")
  [ "$result" = "claude-opus-4" ]
}

@test "get_effective_llm_model : priorité 3 - opencode.model de hub.json" {
  # Mock get_project_api_model retourne vide
  get_project_api_model() {
    return 0
  }
  export -f get_project_api_model
  
  # Hub sans default_provider.model
  cat > "$HUB_CONFIG" <<'EOF'
{
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
EOF
  
  result=$(get_effective_llm_model "")
  [ "$result" = "claude-sonnet-4-5" ]
}

@test "get_effective_llm_model : priorité 4 - fallback DEFAULT_MODEL" {
  # Mock get_project_api_model retourne vide
  get_project_api_model() {
    return 0
  }
  export -f get_project_api_model
  
  # Hub vide
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test"
}
EOF
  
  result=$(get_effective_llm_model "")
  [ "$result" = "$DEFAULT_MODEL" ]
}

# ── Cas limites ───────────────────────────────────────────────────────────────

@test "get_provider_info : gère prefix null (ollama)" {
  require_command "jq"
  result=$(get_provider_info "ollama" "prefix")
  # jq retourne la chaîne "null" pour les valeurs null
  [ "$result" = "null" ] || [ -z "$result" ]
}

@test "provider_exists : retourne 1 si PROVIDERS_FILE inexistant" {
  rm "$PROVIDERS_FILE"
  ! provider_exists "anthropic"
}

@test "get_provider_default_model : retourne vide si PROVIDERS_FILE inexistant" {
  rm "$PROVIDERS_FILE"
  run get_provider_default_model "anthropic"
  # La fonction retourne 1 si le fichier n'existe pas
  [ "$status" -ne 0 ]
  [ -z "$output" ]
}
