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

# Retourne "true" ou "false" pour un champ booléen d'un provider dans providers.json
# Usage : get_provider_bool <provider_name> <field_name>
# Exemple :
#   get_provider_bool "github-copilot" "requires_api_key"  # → "false"
#   get_provider_bool "anthropic" "requires_api_key"       # → "true"
#   get_provider_bool "new-provider" "requires_api_key"    # → "true" (null par défaut)
# Note : ne pas utiliser `// empty` ni `// true` car false est falsy en jq
get_provider_bool() {
  local provider="$1"
  local field="$2"
  jq -r --arg p "$provider" --arg f "$field" \
    '.providers[$p][$f] | if . == null then "true" elif . == false then "false" else tostring end' \
    "$PROVIDERS_FILE" 2>/dev/null
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

# ─────────────────────────────────────────────────────────────────────────────
# Helpers interactifs (partagés par cmd-provider.sh et cmd-config.sh)
# ─────────────────────────────────────────────────────────────────────────────

# Affiche le menu numéroté des providers et remplit un tableau.
# Usage : _build_provider_menu <array_name>
# @param {nameref} $1 — nom du tableau Bash à remplir
_build_provider_menu() {
  local -n _menu_array=$1
  _menu_array=()
  if [ -f "$PROVIDERS_FILE" ] && command -v jq &>/dev/null; then
    while IFS= read -r pname; do
      _menu_array+=("$pname")
    done < <(jq -r '.providers | keys[]' "$PROVIDERS_FILE")
  else
    _menu_array=("anthropic" "mammouth" "github-models" "bedrock" "ollama")
  fi

  local _i=1
  for pname in "${_menu_array[@]}"; do
    local _label; _label=$(get_provider_info "$pname" "label" 2>/dev/null || echo "$pname")
    printf "  ${BLUE}%d${RESET}. %s\n" "$_i" "$_label"
    _i=$((_i + 1))
  done
}

# Collecte les credentials pour un provider sélectionné de façon interactive.
# Sortie : variables globales dans le contexte appelant :
#   _cred_api_key   — clé API saisie
#   _cred_base_url  — URL de base (si applicable)
#   _cred_region    — région AWS (si applicable)
# @param $1 — nom interne du provider
# @param $2 — label affiché
_collect_provider_credentials() {
  local provider_name="$1"
  local provider_label="$2"
  local requires_api_key; requires_api_key=$(get_provider_bool "$provider_name" "requires_api_key")
  local requires_base_url; requires_base_url=$(get_provider_bool "$provider_name" "requires_base_url")
  local requires_region; requires_region=$(get_provider_bool "$provider_name" "requires_region")
  local default_base_url; default_base_url=$(get_provider_info "$provider_name" "default_base_url" 2>/dev/null || echo "")
  local default_region; default_region=$(get_provider_info "$provider_name" "default_region" 2>/dev/null || echo "")

  _cred_api_key=""
  _cred_base_url="$default_base_url"
  _cred_region=""

  if [ "$requires_api_key" = "true" ]; then
    echo ""
    trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
    read -rsp "  Clé API ${provider_label} : " _cred_api_key
    stty echo 2>/dev/null
    trap - INT TERM
    echo ""
  fi

  if [ "$requires_region" = "true" ]; then
    echo ""
    if [ -n "$default_region" ]; then
      read -rp "  Région AWS [${default_region}] : " _input_region
      _cred_region="${_input_region:-$default_region}"
    else
      read -rp "  Région AWS (ex: us-east-1) : " _cred_region
    fi
  fi

  if [ "$requires_base_url" = "true" ] && [ -n "$default_base_url" ]; then
    echo ""
    read -rp "  URL de base [${default_base_url}] : " _input_url
    _cred_base_url="${_input_url:-$default_base_url}"
  fi
}
