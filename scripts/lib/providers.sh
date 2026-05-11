#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# providers.sh — Résolution des providers LLM (hub.json + providers.json)
# Sourcé par common.sh — ne pas sourcer directement.
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_PROVIDERS_LOADED:-}" ] && return 0
_PROVIDERS_LOADED=1

# Chemin du catalogue des providers
PROVIDERS_FILE="$HUB_DIR/config/providers.json"

# Lire le catalogue des providers
get_provider_info() {
  local provider_name="$1" field="$2"
  [ -f "$PROVIDERS_FILE" ] || return 1
  jq -r --arg n "$provider_name" --arg f "$field" '.providers[$n][$f] // empty' "$PROVIDERS_FILE" 2>/dev/null
}

# Vérifier si un provider existe dans le catalogue
provider_exists() {
  local provider_name="$1"
  [ -f "$PROVIDERS_FILE" ] || return 1
  jq -e --arg n "$provider_name" '.providers | has($n)' "$PROVIDERS_FILE" &>/dev/null
}

# Lister tous les providers du catalogue
list_all_providers() {
  [ -f "$PROVIDERS_FILE" ] || return 1
  jq -r '.providers | keys[]' "$PROVIDERS_FILE" 2>/dev/null
}

# Hub-level default provider (lecture depuis hub.json)
get_hub_default_provider() {
  [ -f "$HUB_CONFIG" ] || return 1
  jq -r '.default_provider.name // empty' "$HUB_CONFIG" 2>/dev/null
}

get_hub_default_api_key() {
  [ -f "$HUB_CONFIG" ] || return 1
  jq -r '.default_provider.api_key // empty' "$HUB_CONFIG" 2>/dev/null
}

get_hub_default_base_url() {
  [ -f "$HUB_CONFIG" ] || return 1
  jq -r '.default_provider.base_url // empty' "$HUB_CONFIG" 2>/dev/null
}

get_hub_default_model() {
  [ -f "$HUB_CONFIG" ] || return 1
  jq -r '.default_provider.model // empty' "$HUB_CONFIG" 2>/dev/null
}

# Résout le provider effectif pour un projet
# Priorité : api-keys.local.md projet > hub default > anthropic (fallback)
get_effective_provider() {
  local project_id="$1"
  local project_provider=""
  if [ -n "$project_id" ]; then
    project_provider=$(get_project_api_provider "$project_id")
  fi
  if [ -n "$project_provider" ]; then
    echo "$project_provider"
  else
    local hub_provider; hub_provider=$(get_hub_default_provider)
    echo "${hub_provider:-anthropic}"
  fi
}

# Résout le modèle effectif pour un projet (chaîne de priorité complète)
# Priorité : 1) api-keys projet  2) hub default  3) opencode.model de hub.json  4) DEFAULT_MODEL
get_effective_llm_model() {
  local project_id="${1:-}"
  local model=""

  # 1. api-keys.local.md du projet
  if [ -n "$project_id" ]; then
    model=$(get_project_api_model "$project_id")
    [ -n "$model" ] && echo "$model" && return 0
  fi

  # 2. hub default provider model
  local hub_model; hub_model=$(get_hub_default_model)
  [ -n "$hub_model" ] && echo "$hub_model" && return 0

  # 3. opencode.model de hub.json (comportement actuel)
  if command -v jq &>/dev/null && [ -f "$HUB_CONFIG" ]; then
    model=$(jq -r '.opencode.model // empty' "$HUB_CONFIG" 2>/dev/null)
    [ -n "$model" ] && echo "$model" && return 0
  fi

  # 4. Fallback : DEFAULT_MODEL
  echo "$DEFAULT_MODEL"
}
