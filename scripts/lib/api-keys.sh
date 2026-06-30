#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# api-keys.sh — Parser INI pour projects/api-keys.local.md
# Sourcé par common.sh — ne pas sourcer directement.
#
# Format attendu dans api-keys.local.md :
#   [PROJECT_ID]
#   model=claude-opus-4-5
#   provider=anthropic
#   api_key=sk-ant-...
#   base_url=https://...    # optionnel
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_API_KEYS_LOADED:-}" ] && return 0
_API_KEYS_LOADED=1

# Variables globales de cache (lecture unique pour performances)
_API_KEYS_CACHE_LOADED=0
_API_KEYS_CACHE_PROJECT_ID=""
_API_KEYS_CACHE_PROVIDER=""
_API_KEYS_CACHE_MODEL=""
_API_KEYS_CACHE_KEY=""
_API_KEYS_CACHE_BASE_URL=""
_API_KEYS_CACHE_REGION=""

# Charge toutes les valeurs d'un projet en une seule lecture du fichier
# Usage : api_keys_load_cache <PROJECT_ID>
# Optimise les appels multiples à _api_keys_get pour le même projet
api_keys_load_cache() {
  local id="$1"
  [ -f "$API_KEYS_FILE" ] || return 0
  
  _API_KEYS_CACHE_LOADED=1
  _API_KEYS_CACHE_PROJECT_ID="$id"
  _API_KEYS_CACHE_PROVIDER=""
  _API_KEYS_CACHE_MODEL=""
  _API_KEYS_CACHE_KEY=""
  _API_KEYS_CACHE_BASE_URL=""
  _API_KEYS_CACHE_REGION=""
  
  local in_section=0
  while IFS= read -r line; do
    # Détection section (ligne exacte "[PROJECT_ID]")
    if [ "$line" = "[$id]" ]; then
      in_section=1
      continue
    elif [[ "$line" =~ ^\[.*\]$ ]]; then
      # Autre section, sortir
      [ "$in_section" = "1" ] && break
      in_section=0
    fi
    
    # Extraction valeurs dans la section active
    if [ "$in_section" = "1" ]; then
      case "$line" in
        provider=*)  _API_KEYS_CACHE_PROVIDER="${line#provider=}" ;;
        model=*)     _API_KEYS_CACHE_MODEL="${line#model=}" ;;
        api_key=*)   _API_KEYS_CACHE_KEY="${line#api_key=}" ;;
        base_url=*)  _API_KEYS_CACHE_BASE_URL="${line#base_url=}" ;;
        region=*)    _API_KEYS_CACHE_REGION="${line#region=}" ;;
      esac
    fi
  done < "$API_KEYS_FILE"

  # Si la clé API est un marqueur keychain, la résoudre depuis le backend sécurisé
  if [ "${_API_KEYS_CACHE_KEY:-}" = "__KEYCHAIN__" ] && command -v _secret_get &>/dev/null; then
    local _resolved; _resolved=$(_secret_get "$id" "api_key" 2>/dev/null || true)
    _API_KEYS_CACHE_KEY="${_resolved:-}"
  fi
}

# Lit une clé INI pour une section donnée
# Usage : _api_keys_get <PROJECT_ID> <key>
# Si le cache est chargé pour ce PROJECT_ID, utilise le cache (zéro I/O)
# Sinon, utilise l'ancienne méthode awk (rétrocompatibilité)
_api_keys_get() {
  local id="$1" key="$2"
  
  # Si le cache est chargé pour ce projet, l'utiliser
  if [ "$_API_KEYS_CACHE_LOADED" = "1" ] && [ "$_API_KEYS_CACHE_PROJECT_ID" = "$id" ]; then
    case "$key" in
      provider)  echo "$_API_KEYS_CACHE_PROVIDER" ;;
      model)     echo "$_API_KEYS_CACHE_MODEL" ;;
      api_key)   echo "$_API_KEYS_CACHE_KEY" ;;
      base_url)  echo "$_API_KEYS_CACHE_BASE_URL" ;;
      region)    echo "$_API_KEYS_CACHE_REGION" ;;
      *)         # Clés non supportées par le cache (ex: agent_models.agents.X)
                 # Fallback sur l'ancienne méthode
                 [ -f "$API_KEYS_FILE" ] || return 0
                 awk -v section="[${id}]" -v key="${key}" '
                   $0 == section { found=1; next }
                   found && /^\[/ { found=0 }
                   found && $0 ~ "^" key "=" { sub(/^[^=]+=/, ""); print; exit }
                 ' "$API_KEYS_FILE" ;;
    esac
    return 0
  fi
  
  # Cache non chargé ou autre projet : utiliser l'ancienne méthode
  [ -f "$API_KEYS_FILE" ] || return 0
  awk -v section="[${id}]" -v key="${key}" '
    $0 == section { found=1; next }
    found && /^\[/ { found=0 }
    found && $0 ~ "^" key "=" { sub(/^[^=]+=/, ""); print; exit }
  ' "$API_KEYS_FILE"
}

# Retourne le modèle configuré pour un projet (vide si absent)
get_project_api_model() {
  _api_keys_get "$1" "model"
}

# Retourne le provider configuré pour un projet (vide si absent)
get_project_api_provider() {
  _api_keys_get "$1" "provider"
}

# Retourne la clé API configurée pour un projet (vide si absent)
get_project_api_key() {
  _api_keys_get "$1" "api_key"
}

# Retourne la base URL configurée pour un projet (vide si absent)
get_project_api_base_url() {
  _api_keys_get "$1" "base_url"
}

# Retourne la région AWS configurée pour un projet (vide si absent)
get_project_api_region() {
  _api_keys_get "$1" "region"
}

# Vérifie si une section [PROJECT_ID] existe dans api-keys.local.md
# Utilise une comparaison de ligne exacte pour éviter les faux positifs
# (ex: "[PROJ]" ne doit pas matcher "[PROJ-FULL]")
api_keys_entry_exists() {
  local id="$1"
  [ -f "$API_KEYS_FILE" ] || return 1
  awk -v section="[${id}]" '$0 == section { found=1; exit } END { exit !found }' "$API_KEYS_FILE"
}

# Supprime une section [PROJECT_ID] complète de api-keys.local.md
# (ligne vide précédente incluse)
remove_api_keys_section() {
  local id="$1"
  [ -f "$API_KEYS_FILE" ] || return 0
  api_keys_entry_exists "$id" || return 0
  local tmp; tmp=$(mktemp)
  awk -v section="[${id}]" '
    BEGIN { skip=0; pending_blank=0 }
    /^$/ { if (!skip) { pending_blank=1 }; next }
    $0 == section { pending_blank=0; skip=1; next }
    skip && /^\[/ { skip=0 }
    !skip {
      if (pending_blank) { print ""; pending_blank=0 }
      print
    }
    skip { next }
  ' "$API_KEYS_FILE" > "$tmp"
  mv "$tmp" "$API_KEYS_FILE"
}
