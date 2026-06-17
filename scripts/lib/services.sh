#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# services.sh — Gestion des services/intégrations MCP (catalog + config)
# Sourcé par cmd-service.sh et les scripts qui en ont besoin.
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_SERVICES_LOADED:-}" ] && return 0
_SERVICES_LOADED=1

# Chemin du catalogue des services
SERVICES_FILE="${SERVICES_FILE:-$HUB_DIR/config/services.json}"

# Chemin du fichier de stockage des credentials de services
# Fichier séparé de opencode.json pour ne pas invalider la config opencode
OPENCODE_GLOBAL_CONFIG="${OPENCODE_GLOBAL_CONFIG:-$HOME/.config/opencode/services-env.json}"

# ─────────────────────────────────────────────────────────────────────────────
# Helpers catalogue (lecture JSON)
# ─────────────────────────────────────────────────────────────────────────────

# Retourne la liste des IDs de services disponibles
# Usage : svc_list_available
svc_list_available() {
  [ -f "$SERVICES_FILE" ] || return 1
  jq -r '.services | keys[]' "$SERVICES_FILE" 2>/dev/null
}

# Vérifie qu'un service existe dans le catalogue
# Usage : svc_exists "figma"
svc_exists() {
  local service_id="$1"
  [ -f "$SERVICES_FILE" ] || return 1
  jq -e --arg s "$service_id" '.services | has($s)' "$SERVICES_FILE" &>/dev/null
}

# Lit un champ de premier niveau d'un service dans le catalogue
# Usage : svc_get_field "figma" "label"
svc_get_field() {
  local service_id="$1" field="$2"
  [ -f "$SERVICES_FILE" ] || return 1
  jq -r --arg s "$service_id" --arg f "$field" '.services[$s][$f] // empty' "$SERVICES_FILE" 2>/dev/null
}

# Retourne le nombre de credentials pour un service
# Usage : svc_credential_count "figma"
svc_credential_count() {
  local service_id="$1"
  [ -f "$SERVICES_FILE" ] || return 1
  jq -r --arg s "$service_id" '.services[$s].credentials | length' "$SERVICES_FILE" 2>/dev/null
}

# Lit un champ d'un credential par index (0-based)
# Usage : svc_get_credential "figma" 0 "key"
svc_get_credential() {
  local service_id="$1" index="$2" field="$3"
  [ -f "$SERVICES_FILE" ] || return 1
  jq -r --arg s "$service_id" --argjson i "$index" --arg f "$field" \
    '.services[$s].credentials[$i][$f] // empty' "$SERVICES_FILE" 2>/dev/null
}

# Lit un champ booléen d'un credential (retourne "true" ou "false" en toute sécurité)
# Usage : svc_get_credential_bool "figma" 0 "secret"
svc_get_credential_bool() {
  local service_id="$1" index="$2" field="$3"
  [ -f "$SERVICES_FILE" ] || { printf '%s' "true"; return 0; }
  jq -r --arg s "$service_id" --argjson i "$index" --arg f "$field" \
    '.services[$s].credentials[$i][$f] | if . == null then "true" elif . == false then "false" else tostring end' \
    "$SERVICES_FILE" 2>/dev/null
}

# Retourne un champ localisé (field_fr ou field_en selon OC_LANG)
# Usage : svc_localized "figma" "description"
# Lit description_fr ou description_en selon OC_LANG
svc_localized() {
  local service_id="$1" base_field="$2"
  local lang="${OC_LANG:-en}"
  local value

  # Essai avec suffixe langue
  value=$(jq -r --arg s "$service_id" --arg f "${base_field}_${lang}" \
    '.services[$s][$f] // empty' "$SERVICES_FILE" 2>/dev/null)

  # Fallback sur l'autre langue si vide
  if [ -z "$value" ]; then
    local fallback_lang="en"
    [ "$lang" = "en" ] && fallback_lang="fr"
    value=$(jq -r --arg s "$service_id" --arg f "${base_field}_${fallback_lang}" \
      '.services[$s][$f] // empty' "$SERVICES_FILE" 2>/dev/null)
  fi

  printf '%s' "$value"
}

# Retourne un champ localisé d'un credential
# Usage : svc_localized_credential "figma" 0 "label"
svc_localized_credential() {
  local service_id="$1" index="$2" base_field="$3"
  local lang="${OC_LANG:-en}"
  local value

  value=$(jq -r --arg s "$service_id" --argjson i "$index" --arg f "${base_field}_${lang}" \
    '.services[$s].credentials[$i][$f] // empty' "$SERVICES_FILE" 2>/dev/null)

  if [ -z "$value" ]; then
    local fallback_lang="en"
    [ "$lang" = "en" ] && fallback_lang="fr"
    value=$(jq -r --arg s "$service_id" --argjson i "$index" --arg f "${base_field}_${fallback_lang}" \
      '.services[$s].credentials[$i][$f] // empty' "$SERVICES_FILE" 2>/dev/null)
  fi

  printf '%s' "$value"
}

# ─────────────────────────────────────────────────────────────────────────────
# Helpers config (~/.config/opencode/services-env.json)
# ─────────────────────────────────────────────────────────────────────────────

# S'assure que le fichier de credentials global existe avec une structure minimale
# Ce fichier est intentionnellement séparé de opencode.json pour ne pas invalider
# la config opencode (qui a additionalProperties: false et ne connaît pas "env")
_svc_ensure_config_file() {
  local config_file="${1:-$OPENCODE_GLOBAL_CONFIG}"
  if [ ! -f "$config_file" ]; then
    mkdir -p "$(dirname "$config_file")"
    printf '{\n  "env": {}\n}\n' > "$config_file"
  fi
  # S'assurer que la clé "env" existe
  if ! jq -e '.env' "$config_file" &>/dev/null; then
    local tmp
    tmp=$(mktemp)
    jq '. + {"env": {}}' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
  fi
}

# Lit une valeur env depuis la config globale
# Usage : svc_get_env_value "FIGMA_PERSONAL_ACCESS_TOKEN"
svc_get_env_value() {
  local key="$1"
  local config_file="${OPENCODE_GLOBAL_CONFIG}"
  [ -f "$config_file" ] || return 1
  jq -r --arg k "$key" '.env[$k] // empty' "$config_file" 2>/dev/null
}

# Écrit/met à jour une valeur env dans la config globale (atomic write)
# Usage : svc_set_env_value "FIGMA_PERSONAL_ACCESS_TOKEN" "figd_xxx"
svc_set_env_value() {
  local key="$1" value="$2"
  local config_file="${OPENCODE_GLOBAL_CONFIG}"
  _svc_ensure_config_file "$config_file"
  local tmp
  tmp=$(mktemp)
  if jq --arg k "$key" --arg v "$value" '.env[$k] = $v' "$config_file" > "$tmp"; then
    mv "$tmp" "$config_file"
  else
    rm -f "$tmp"
    return 1
  fi
}

# Supprime toutes les clés env d'un service
# Usage : svc_remove_env_values "figma"
svc_remove_env_values() {
  local service_id="$1"
  local config_file="${OPENCODE_GLOBAL_CONFIG}"
  [ -f "$config_file" ] || return 0

  local count
  count=$(svc_credential_count "$service_id")
  [ -z "$count" ] || [ "$count" -eq 0 ] && return 0

  local tmp
  tmp=$(mktemp)
  local jq_filter=". "
  for (( i=0; i<count; i++ )); do
    local cred_key
    cred_key=$(svc_get_credential "$service_id" "$i" "key")
    [ -n "$cred_key" ] && jq_filter+="| del(.env[\"$cred_key\"]) "
  done
  if jq "$jq_filter" "$config_file" > "$tmp"; then
    mv "$tmp" "$config_file"
  else
    rm -f "$tmp"
    return 1
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Helpers config par projet (mcp.<server>.environment dans opencode.json)
# Les credentials projet ont priorité sur les credentials globaux.
# ─────────────────────────────────────────────────────────────────────────────

# Lit un credential depuis mcp.<server>.environment dans opencode.json d'un projet
# Usage : svc_get_project_env_value "<project_path>" "<server_name>" "<KEY>"
svc_get_project_env_value() {
  local project_path="$1" server_name="$2" key="$3"
  local opencode_json="$project_path/opencode.json"
  [ -f "$opencode_json" ] || return 1
  jq -r --arg s "$server_name" --arg k "$key" \
    '.mcp[$s].environment[$k] // empty' "$opencode_json" 2>/dev/null
}

# Écrit un credential dans mcp.<server>.environment de opencode.json d'un projet (atomic)
# Crée le bloc mcp.<server> s'il n'existe pas encore.
# Usage : svc_set_project_env_value "<project_path>" "<server_name>" "<KEY>" "<value>"
svc_set_project_env_value() {
  local project_path="$1" server_name="$2" key="$3" value="$4"
  local opencode_json="$project_path/opencode.json"
  [ -f "$opencode_json" ] || return 1
  local tmp
  tmp=$(mktemp)
  if jq --arg s "$server_name" --arg k "$key" --arg v "$value" \
    '.mcp[$s].environment[$k] = $v' "$opencode_json" > "$tmp"; then
    mv "$tmp" "$opencode_json"
  else
    rm -f "$tmp"
    return 1
  fi
}

# Supprime toutes les clés env d'un service dans opencode.json d'un projet
# Usage : svc_remove_project_env_values "<project_path>" "<service_id>"
svc_remove_project_env_values() {
  local project_path="$1" service_id="$2"
  local opencode_json="$project_path/opencode.json"
  [ -f "$opencode_json" ] || return 0

  local mcp_server
  mcp_server=$(svc_get_field "$service_id" "mcp_server" 2>/dev/null || echo "")
  [ -z "$mcp_server" ] && return 0

  local count
  count=$(svc_credential_count "$service_id")
  [ -z "$count" ] || [ "$count" -eq 0 ] && return 0

  local tmp
  tmp=$(mktemp)
  local jq_filter=". "
  for (( i=0; i<count; i++ )); do
    local cred_key
    cred_key=$(svc_get_credential "$service_id" "$i" "key")
    [ -n "$cred_key" ] && jq_filter+="| del(.mcp[\"$mcp_server\"].environment[\"$cred_key\"]) "
  done
  if jq "$jq_filter" "$opencode_json" > "$tmp"; then
    mv "$tmp" "$opencode_json"
  else
    rm -f "$tmp"
    return 1
  fi
}

# Retourne toutes les valeurs env d'un service sous forme d'objet JSON
# Usage : svc_get_all_env_for_service "figma"  → {"FIGMA_PERSONAL_ACCESS_TOKEN":"...","FIGMA_TEAM_ID":"..."}
svc_get_all_env_for_service() {
  local service_id="$1"
  local config_file="${OPENCODE_GLOBAL_CONFIG}"
  [ -f "$config_file" ] || { printf '{}'; return 0; }

  local count
  count=$(svc_credential_count "$service_id")
  [ -z "$count" ] || [ "$count" -eq 0 ] && { printf '{}'; return 0; }

  local result="{}"
  for (( i=0; i<count; i++ )); do
    local cred_key
    cred_key=$(svc_get_credential "$service_id" "$i" "key")
    [ -z "$cred_key" ] && continue
    local val
    val=$(svc_get_env_value "$cred_key")
    if [ -n "$val" ]; then
      result=$(printf '%s' "$result" | jq --arg k "$cred_key" --arg v "$val" '. + {($k): $v}')
    fi
  done
  printf '%s' "$result"
}

# Vérifie si toutes les credentials requises d'un service sont configurées
# Usage : svc_is_configured "figma"  → 0 si oui, 1 si non
svc_is_configured() {
  local service_id="$1"
  local count
  count=$(svc_credential_count "$service_id")
  [ -z "$count" ] && return 1

  for (( i=0; i<count; i++ )); do
    # Lecture booléenne correcte : false ne doit pas être converti par // empty
    local required
    required=$(jq -r --arg s "$service_id" --argjson i "$i" \
      '.services[$s].credentials[$i].required | if . == null then "true" elif . == false then "false" else tostring end' \
      "$SERVICES_FILE" 2>/dev/null)
    [ "$required" = "false" ] && continue

    local cred_key
    cred_key=$(svc_get_credential "$service_id" "$i" "key")
    local value
    value=$(svc_get_env_value "$cred_key")
    [ -z "$value" ] && return 1
  done
  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# Validation API
# ─────────────────────────────────────────────────────────────────────────────

# Valide le token d'un service via son endpoint de validation
# Affiche le handle/username retourné si succès
# Usage : svc_validate_token "figma"  → 0 si OK, 1 si KO
svc_validate_token() {
  local service_id="$1"
  local endpoint
  endpoint=$(jq -r --arg s "$service_id" '.services[$s].validation.endpoint // empty' "$SERVICES_FILE" 2>/dev/null)
  [ -z "$endpoint" ] && return 0  # Pas d'endpoint de validation → skip

  local header_name
  header_name=$(jq -r --arg s "$service_id" '.services[$s].validation.header // empty' "$SERVICES_FILE" 2>/dev/null)
  local token_field
  token_field=$(jq -r --arg s "$service_id" '.services[$s].validation.token_field // empty' "$SERVICES_FILE" 2>/dev/null)
  local success_field
  success_field=$(jq -r --arg s "$service_id" '.services[$s].validation.success_field // empty' "$SERVICES_FILE" 2>/dev/null)
  local base_url_field
  base_url_field=$(jq -r --arg s "$service_id" '.services[$s].validation.base_url_field // empty' "$SERVICES_FILE" 2>/dev/null)

  local token
  token=$(svc_get_env_value "$token_field")
  [ -z "$token" ] && return 1

  # Si un base_url_field est défini, utiliser sa valeur pour construire l'endpoint
  if [ -n "$base_url_field" ]; then
    local base_url
    base_url=$(svc_get_env_value "$base_url_field")
    if [ -n "$base_url" ]; then
      # Remplacer le domaine par défaut (gitlab.com) par l'instance configurée
      local path
      path=$(echo "$endpoint" | sed 's|https://gitlab.com||')
      endpoint="${base_url%/}${path}"
    fi
  fi

  local response
  response=$(curl -sf --max-time 10 -H "${header_name}: ${token}" "$endpoint" 2>/dev/null)
  local exit_code=$?

  if [ $exit_code -ne 0 ] || [ -z "$response" ]; then
    return 1
  fi

  # Extraire le champ de succès si défini
  if [ -n "$success_field" ] && command -v jq &>/dev/null; then
    local handle
    handle=$(echo "$response" | jq -r "$success_field // empty" 2>/dev/null)
    [ -n "$handle" ] && printf '%s' "$handle"
  fi
  return 0
}

# Valide le team_id d'un service via son endpoint team_validation
# Affiche le nom de la team retourné si succès
# Usage : svc_validate_team "figma"  → 0 si OK, 1 si KO
svc_validate_team() {
  local service_id="$1"
  local endpoint_tpl
  endpoint_tpl=$(jq -r --arg s "$service_id" '.services[$s].team_validation.endpoint // empty' "$SERVICES_FILE" 2>/dev/null)
  [ -z "$endpoint_tpl" ] && return 0  # Pas de team_validation → skip

  local header_name
  header_name=$(jq -r --arg s "$service_id" '.services[$s].team_validation.header // empty' "$SERVICES_FILE" 2>/dev/null)
  local token_field
  token_field=$(jq -r --arg s "$service_id" '.services[$s].team_validation.token_field // empty' "$SERVICES_FILE" 2>/dev/null)
  local team_field
  team_field=$(jq -r --arg s "$service_id" '.services[$s].team_validation.team_field // empty' "$SERVICES_FILE" 2>/dev/null)
  local success_field
  success_field=$(jq -r --arg s "$service_id" '.services[$s].team_validation.success_field // empty' "$SERVICES_FILE" 2>/dev/null)

  local token team_id
  token=$(svc_get_env_value "$token_field")
  team_id=$(svc_get_env_value "$team_field")
  [ -z "$token" ] && return 1
  [ -z "$team_id" ] && return 1

  # Interpoler le team_id dans l'URL template
  local endpoint="${endpoint_tpl/\{$team_field\}/$team_id}"

  local response
  response=$(curl -sf --max-time 10 -H "${header_name}: ${token}" "$endpoint" 2>/dev/null)
  local exit_code=$?

  if [ $exit_code -ne 0 ] || [ -z "$response" ]; then
    return 1
  fi

  if [ -n "$success_field" ] && command -v jq &>/dev/null; then
    local team_name
    team_name=$(echo "$response" | jq -r "$success_field // empty" 2>/dev/null)
    [ -n "$team_name" ] && printf '%s' "$team_name"
  fi
  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# MCP server helpers
# ─────────────────────────────────────────────────────────────────────────────

# Vérifie si le MCP server d'un service est buildé
# Usage : svc_is_mcp_built "figma"  → 0 si oui, 1 si non
svc_is_mcp_built() {
  local service_id="$1"
  local mcp_server
  mcp_server=$(svc_get_field "$service_id" "mcp_server")
  [ -z "$mcp_server" ] && return 0  # Pas de MCP server → skip

  local dist_dir="$HUB_DIR/servers/$mcp_server/dist"
  [ -d "$dist_dir" ] && [ -f "$dist_dir/index.js" ]
}

# Lance le build du MCP server d'un service
# Usage : svc_build_mcp "figma"
svc_build_mcp() {
  local service_id="$1"
  local mcp_server
  mcp_server=$(svc_get_field "$service_id" "mcp_server")
  [ -z "$mcp_server" ] && return 0

  if [ ! -d "$HUB_DIR/servers/$mcp_server" ]; then
    return 1
  fi

  bash "$HUB_DIR/scripts/build-mcp.sh" "$mcp_server"
}
