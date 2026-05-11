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

# Lit une clé INI pour une section donnée
# Usage : _api_keys_get <PROJECT_ID> <key>
_api_keys_get() {
  local id="$1" key="$2"
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
