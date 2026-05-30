#!/bin/bash
set -euo pipefail

# Guard : si _CMD_CONFIG_SOURCE_ONLY=1, ne sourcer que les fonctions sans exécuter
# common.sh doit être déjà sourcé par l'appelant dans ce cas
if [ "${_CMD_CONFIG_SOURCE_ONLY:-}" != "1" ]; then
  source "$(cd "$(dirname "$0")" && pwd)/common.sh"
  resolve_oc_lang
fi

# ─────────────────────────────────────────────────────────────────
# oc config — gestion des clés API et modèles par projet
# Stockage : projects/api-keys.local.md (non versionné)
# ─────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-}"
shift || true

# ── Helpers internes ──────────────────────────────────────────────

# Assure que api-keys.local.md existe
_ensure_api_keys_file() {
  if [ ! -f "$API_KEYS_FILE" ]; then
    mkdir -p "$(dirname "$API_KEYS_FILE")"
    cat > "$API_KEYS_FILE" <<'EOF'
# Clés API et modèles par projet — NE PAS COMMITTER
# Format :
#   [PROJECT_ID]
#   model=claude-opus-4-5
#   provider=anthropic
#   api_key=sk-ant-...
#   base_url=https://...   (optionnel — litellm uniquement)
EOF
    log_info "api-keys.local.md créé"
  fi
}

# Vérifie qu'un nom de famille existe dans agents/
# Retourne 0 si valide, 1 sinon
_validate_family_name() {
  local name="$1"
  [ -d "$CANONICAL_AGENTS_DIR/$name" ]
}

# Vérifie qu'un agent id existe (fichier .md dans agents/*/)
# Retourne 0 si valide, 1 sinon
_validate_agent_name() {
  local name="$1"
  local f
  for f in "$CANONICAL_AGENTS_DIR"/*/"${name}.md"; do
    [ -f "$f" ] && return 0
  done
  return 1
}

# Émet un warning si le modèle n'est pas dans la liste connue
_warn_unknown_model() {
  local model="$1"
  case "$model" in
    claude-opus-4|claude-opus-4-*|claude-sonnet-4-5|claude-sonnet-4-5-*|claude-haiku-4-5|claude-haiku-4-5-*) ;;
    *) log_warn "Unknown model '$model' — known models: claude-opus-4, claude-sonnet-4-5, claude-haiku-4-5" ;;
  esac
}

# Supprime une section [PROJECT_ID] complète du fichier (délègue à common.sh)
_remove_section() {
  remove_api_keys_section "$1"
}

# Retourne "true" si le provider requiert une clé API, "false" sinon
_resolve_requires_api_key() {
  local provider="$1"
  get_provider_bool "$provider" "requires_api_key"
}

# Écrit ou remplace une section complète (atomique via tmpfile + mv)
_write_section() {
  local id="$1" model="$2" provider="$3" api_key="$4" base_url="$5"
  _ensure_api_keys_file
  # Construire le contenu complet (fichier existant sans la section + nouvelle entrée)
  local tmp; tmp=$(mktemp)
  # Retirer l'ancienne section si elle existe (via awk → tmpfile)
  if api_keys_entry_exists "$id"; then
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
  else
    cp "$API_KEYS_FILE" "$tmp"
  fi
  # Ajouter la nouvelle entrée
  {
    echo ""
    echo "[${id}]"
    echo "model=${model}"
    echo "provider=${provider}"
    echo "api_key=${api_key}"
    [ -n "$base_url" ] && echo "base_url=${base_url}"
  } >> "$tmp"
  # Remplacement atomique
  mv "$tmp" "$API_KEYS_FILE"
}

# Affiche la configuration d'un projet (masque la clé)
_display_entry() {
  local id="$1"
  local model provider api_key base_url masked
  model=$(get_project_api_model "$id")
  provider=$(get_project_api_provider "$id")
  api_key=$(get_project_api_key "$id")
  base_url=$(get_project_api_base_url "$id")
  # Masquer la clé API : conserver les 8 premiers caractères + ***
  if [ -n "$api_key" ] && [ "${#api_key}" -gt 8 ]; then
    masked="${api_key:0:8}***"
  elif [ -n "$api_key" ]; then
    masked="***"
  else
    masked="(non définie)"
  fi
  echo -e "  ${BOLD}${id}${RESET}"
  echo "    model    : ${model:-(défaut hub)}"
  echo "    provider : ${provider:-(non défini)}"
  echo "    api_key  : ${masked}"
  [ -n "$base_url" ] && echo "    base_url : ${base_url}"
}

# ── Sous-commandes ─────────────────────────────────────────────────

cmd_set_language() {
  local value="${1:-}"
  if [ -z "$value" ]; then
    log_error "Usage: oc config set language <en|fr>"
    exit 1
  fi
  case "$value" in
    en|fr) ;;
    *) log_error "Invalid language value (accepted: en, fr)"; exit 1 ;;
  esac
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is required to set the language"
    exit 1
  fi
  if [ ! -f "$HUB_CONFIG" ]; then
    log_error "hub.json not found — run first: ./oc.sh install"
    exit 1
  fi
  local tmp; tmp=$(mktemp)
  jq --arg lang "$value" '.cli.language = $lang' "$HUB_CONFIG" > "$tmp" && mv "$tmp" "$HUB_CONFIG"
  log_success "CLI language set to $value"
}

cmd_get_language() {
  local lang; lang=$(get_hub_language)
  log_info "Current CLI language: ${lang:-en}"
}

_cmd_set_hub() {
  # Parse --family-model et --agent-model au niveau hub (pas de PROJECT_ID)
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is required for hub-level config"
    exit 1
  fi
  if [ ! -f "$HUB_CONFIG" ]; then
    log_error "hub.json not found — run first: ./oc.sh install"
    exit 1
  fi
  while [ $# -gt 0 ]; do
    case "$1" in
      --family-model)
        [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur (format: key=value)"; exit 1; }
        local fm_key="${2%%=*}" fm_value="${2#*=}"
        [ "$fm_key" = "$2" ] && { log_error "--family-model requiert le format key=value"; exit 1; }
        _validate_family_name "$fm_key" || { log_error "Unknown family '$fm_key' — available: $(cd "$CANONICAL_AGENTS_DIR" 2>/dev/null && printf '%s, ' */ | sed 's/, $//')"; exit 1; }
        _warn_unknown_model "$fm_value"
        local tmp; tmp=$(mktemp)
        jq --arg k "$fm_key" --arg v "$fm_value" '.agent_models.families[$k] = $v' "$HUB_CONFIG" > "$tmp" && mv "$tmp" "$HUB_CONFIG"
        log_success "Hub family model set: $fm_key=$fm_value"
        shift 2 ;;
      --agent-model)
        [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur (format: key=value)"; exit 1; }
        local am_key="${2%%=*}" am_value="${2#*=}"
        [ "$am_key" = "$2" ] && { log_error "--agent-model requiert le format key=value"; exit 1; }
        _validate_agent_name "$am_key" || { log_error "Unknown agent '$am_key' — no matching file in $CANONICAL_AGENTS_DIR/*/"; exit 1; }
        _warn_unknown_model "$am_value"
        local tmp; tmp=$(mktemp)
        jq --arg k "$am_key" --arg v "$am_value" '.agent_models.agents[$k] = $v' "$HUB_CONFIG" > "$tmp" && mv "$tmp" "$HUB_CONFIG"
        log_success "Hub agent model set: $am_key=$am_value"
        shift 2 ;;
      *) log_error "Option inconnue : $1"; exit 1 ;;
    esac
  done
}

# Ajoute ou remplace une ligne key=value dans une section [ID] de api-keys.local.md
_upsert_api_keys_field() {
  local id="$1" field="$2" value="$3"
  _ensure_api_keys_file
  if ! api_keys_entry_exists "$id"; then
    printf '\n[%s]\n' "$id" >> "$API_KEYS_FILE"
  fi
  local tmp; tmp=$(mktemp)
  awk -v section="[${id}]" -v field="$field" -v value="$value" '
    BEGIN { found_section=0; replaced=0 }
    $0 == section { found_section=1; print; next }
    found_section && /^\[/ {
      if (!replaced) { print field "=" value }
      found_section=0; replaced=1; print; next
    }
    found_section && substr($0, 1, length(field)+1) == field "=" { print field "=" value; replaced=1; next }
    { print }
    END { if (found_section && !replaced) print field "=" value }
  ' "$API_KEYS_FILE" > "$tmp"
  mv "$tmp" "$API_KEYS_FILE"
}

cmd_set() {
  local id="${1:-}"

  # Special case: oc config set language <en|fr>
  if [ "$id" = "language" ]; then
    shift; cmd_set_language "$@"
    return
  fi

  # Si le premier arg est un flag → pas de project_id, opération sur le hub
  if [ -z "$id" ] || [[ "$id" == --* ]]; then
    _cmd_set_hub "$@"
    return
  fi

  shift || true

  # Flags optionnels
  local flag_model="" flag_provider="" flag_api_key="" flag_base_url=""
  local flag_family_models=() flag_agent_models=()
  while [ $# -gt 0 ]; do
    case "$1" in
      --model)    [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_model="$2";    shift 2 ;;
      --provider) [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_provider="$2"; shift 2 ;;
      --api-key)  [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_api_key="$2";  shift 2 ;;
      --base-url) [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_base_url="$2"; shift 2 ;;
      --family-model) [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_family_models+=("$2"); shift 2 ;;
      --agent-model)  [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_agent_models+=("$2");  shift 2 ;;
      *) log_error "Option inconnue : $1"; exit 1 ;;
    esac
  done

  # Si --family-model ou --agent-model utilisés → écriture autonome, pas de flow interactif
  if [ ${#flag_family_models[@]} -gt 0 ] || [ ${#flag_agent_models[@]} -gt 0 ]; then
    id=$(normalize_project_id "$id")
    require_project_id "$id"
    for entry in "${flag_family_models[@]}"; do
      local fk="${entry%%=*}" fv="${entry#*=}"
      [ "$fk" = "$entry" ] && { log_error "--family-model requiert le format key=value"; exit 1; }
      _validate_family_name "$fk" || { log_error "Unknown family '$fk' — available: $(cd "$CANONICAL_AGENTS_DIR" 2>/dev/null && printf '%s, ' */ | sed 's/, $//')"; exit 1; }
      _warn_unknown_model "$fv"
      _upsert_api_keys_field "$id" "agent_models.families.${fk}" "$fv"
      log_success "Project $id family model set: $fk=$fv"
    done
    for entry in "${flag_agent_models[@]}"; do
      local ak="${entry%%=*}" av="${entry#*=}"
      [ "$ak" = "$entry" ] && { log_error "--agent-model requiert le format key=value"; exit 1; }
      _validate_agent_name "$ak" || { log_error "Unknown agent '$ak' — no matching file in $CANONICAL_AGENTS_DIR/*/"; exit 1; }
      _warn_unknown_model "$av"
      _upsert_api_keys_field "$id" "agent_models.agents.${ak}" "$av"
      log_success "Project $id agent model set: $ak=$av"
    done
    return
  fi

  [ -z "$id" ] && { read -rp "  PROJECT_ID : " id; }
  id=$(normalize_project_id "$id")
  require_project_id "$id"

  # Valeurs actuelles (si entrée existe déjà)
  local cur_model cur_provider cur_api_key cur_base_url
  cur_model=$(get_project_api_model "$id")
  cur_provider=$(get_project_api_provider "$id")
  cur_api_key=$(get_project_api_key "$id")
  cur_base_url=$(get_project_api_base_url "$id")

  echo -e "\n${BOLD}Configuration API — $id${RESET}\n"

  # Modèle
  if [ -z "$flag_model" ]; then
    local default_model="${cur_model:-$DEFAULT_MODEL}"
    read -rp "  Modèle [${default_model}] : " flag_model
    flag_model="${flag_model:-$default_model}"
  fi

  # Provider
  if [ -z "$flag_provider" ]; then
    local default_provider="${cur_provider:-anthropic}"
    if [ -f "$PROVIDERS_FILE" ] && command -v jq &>/dev/null; then
      # Menu dynamique depuis providers.json
      echo ""
      echo "  Choisir le provider :"
      echo ""
      local _providers_array=()
      _build_provider_menu _providers_array
      echo ""
      read -rp "  Numéro (1-${#_providers_array[@]}) ou Entrée pour conserver [${default_provider}] : " _choice
      if [ -z "$_choice" ]; then
        flag_provider="$default_provider"
      elif [[ "$_choice" =~ ^[0-9]+$ ]] && [ "$_choice" -ge 1 ] && [ "$_choice" -le "${#_providers_array[@]}" ]; then
        flag_provider="${_providers_array[$((_choice - 1))]}"
      else
        log_error "Choix invalide : $_choice"
        exit 1
      fi
    else
      # Fallback texte libre si providers.json absent
      echo "  Providers disponibles : anthropic / mammouth / github-models / bedrock / ollama / litellm"
      read -rp "  Provider [${default_provider}] : " flag_provider
      flag_provider="${flag_provider:-$default_provider}"
    fi
  fi
  # Normaliser
  flag_provider=$(echo "$flag_provider" | tr '[:upper:]' '[:lower:]')
  # Valider : accepter tous les providers du catalogue + litellm
  local valid_providers="anthropic mammouth github-models bedrock ollama litellm"
  if [ -f "$PROVIDERS_FILE" ] && command -v jq &>/dev/null; then
    valid_providers=$(jq -r '.providers | keys | join(" ")' "$PROVIDERS_FILE" 2>/dev/null || echo "$valid_providers")
  fi
  local _found=false
  for _p in $valid_providers; do
    [ "$_p" = "$flag_provider" ] && _found=true && break
  done
  if [ "$_found" = false ]; then
    log_error "Provider non supporté : $flag_provider"
    log_info  "Providers disponibles : $valid_providers"
    exit 1
  fi

  # Vérifier si le provider nécessite une clé API
  local requires_api_key
  requires_api_key=$(_resolve_requires_api_key "$flag_provider")

  # Clé API (saisie masquée + validation — uniquement si le provider la requiert)
  if [ "$requires_api_key" = "true" ]; then
    if [ -z "$flag_api_key" ]; then
      local masked_cur=""
      [ -n "$cur_api_key" ] && masked_cur=" [actuelle : ${cur_api_key:0:8}***]"
      # Restaurer l'écho terminal si l'utilisateur interrompt (Ctrl+C)
      trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
      read -rsp "  Clé API${masked_cur} : " flag_api_key
      stty echo 2>/dev/null
      trap - INT TERM
      echo ""
      # Si aucune nouvelle saisie et qu'une ancienne existe, conserver l'ancienne
      if [ -z "$flag_api_key" ] && [ -n "$cur_api_key" ]; then
        flag_api_key="$cur_api_key"
        log_info "Clé API inchangée"
      fi
    fi
    if [ -z "$flag_api_key" ]; then
      log_error "Clé API requise"
      exit 1
    fi
  fi

  # Base URL (optionnel pour tous les providers, mais particulièrement utile pour litellm et autres)
  if [ -z "$flag_base_url" ] && { [ "$flag_provider" = "litellm" ] || [ "$flag_provider" = "mammouth" ] || [ "$flag_provider" = "github-models" ] || [ "$flag_provider" = "bedrock" ] || [ "$flag_provider" = "ollama" ]; }; then
    local default_url="${cur_base_url:-}"
    local prompt_url="  Base URL${default_url:+ [${default_url}]} : "
    read -rp "$prompt_url" flag_base_url
    flag_base_url="${flag_base_url:-$default_url}"
  fi

  # Écriture
  _ensure_api_keys_file
  _write_section "$id" "$flag_model" "$flag_provider" "$flag_api_key" "$flag_base_url"
  log_success "$(t config.written) $id"

  # Proposer un re-déploiement uniquement si le chemin du projet est connu
  echo ""
  if path_exists "$id"; then
    read -rp "  $(t config.apply_now)" apply_now
    if [[ "${apply_now:-Y}" =~ ^[Yy]$ ]]; then
      PROJECT_ID="$id" bash "$SCRIPTS_DIR/cmd-deploy.sh" "$id"
    else
      log_info "$(t config.apply_later)"
    fi
  else
    log_info "$(t config.no_path) $id $(t config.apply_via) $id"
  fi
}

cmd_get() {
  local id="${1:-}"
  [ -z "$id" ] && { log_error "Usage : oc config get <PROJECT_ID>"; exit 1; }

  # Special case: oc config get language
  if [ "$id" = "language" ]; then
    cmd_get_language
    return
  fi

  id=$(normalize_project_id "$id")
  if ! api_keys_entry_exists "$id"; then
    log_warn "$(t config.no_entry) $id"
    exit 0
  fi
  echo ""
  _display_entry "$id"
  echo ""
}

cmd_list() {
  if [ ! -f "$API_KEYS_FILE" ]; then
    log_info "$(t config.no_file)"
    exit 0
  fi
  local sections
  sections=$(grep -E '^\[.+\]$' "$API_KEYS_FILE" | tr -d '[]' || true)
  if [ -z "$sections" ]; then
    log_info "$(t config.no_entries)"
    exit 0
  fi
  echo -e "\n${BOLD}$(t config.saved)${RESET}\n"
  while IFS= read -r id; do
    _display_entry "$id"
    echo ""
  done <<< "$sections"
}

cmd_unset() {
  local id="${1:-}"
  [ -z "$id" ] && { log_error "Usage : oc config unset <PROJECT_ID>"; exit 1; }
  id=$(normalize_project_id "$id")
  if ! api_keys_entry_exists "$id"; then
    log_warn "$(t config.no_entry) $id"
    exit 0
  fi
  read -rp "  $(t config.delete_confirm) $id ? [y/N] : " confirm
  if [[ "${confirm:-N}" =~ ^[Yy]$ ]]; then
    _remove_section "$id"
    log_success "$(t config.deleted) $id"
  else
    log_info "$(t cancelled)"
  fi
}

# ── WebSearch management ───────────────────────────────────────────────

cmd_websearch() {
  local subcmd="${1:-}"
  shift || true
  
  case "$subcmd" in
    enable)  cmd_websearch_enable "$@" ;;
    disable) cmd_websearch_disable "$@" ;;
    status)  cmd_websearch_status "$@" ;;
    "")
      echo -e "${BOLD}WebSearch Configuration${RESET}"
      echo ""
      echo "  oc config websearch enable [PROJECT_ID]   — Enable WebSearch (Exa AI) for a project or hub"
      echo "  oc config websearch disable [PROJECT_ID]  — Disable WebSearch for a project"
      echo "  oc config websearch status [PROJECT_ID]   — Show WebSearch status"
      echo ""
      echo "WebSearch enables agents to search the web using Exa AI (hosted by OpenCode)."
      echo "No API key required. Use this for CVE lookup, documentation research, design patterns, etc."
      exit 0
      ;;
    *)
      log_error "Unknown websearch subcommand: $subcmd"
      echo "Available: enable, disable, status"
      exit 1
      ;;
  esac
}

cmd_websearch_enable() {
  local id="${1:-}"
  
  # Si aucun ID fourni → activer au niveau hub
  if [ -z "$id" ]; then
    log_info "Enabling WebSearch at hub level (opencode-hub/opencode.json)..."
    
    local hub_opencode_config="$REPO_ROOT/opencode.json"
    
    if [ ! -f "$hub_opencode_config" ]; then
      log_warn "opencode.json not found, creating it..."
      cat > "$hub_opencode_config" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "OPENCODE_ENABLE_EXA": "1"
  },
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
EOF
      log_success "Created opencode.json with WebSearch enabled"
      return 0
    fi
    
    # Utiliser jq pour ajouter les champs si disponible, sinon erreur
    if ! command -v jq >/dev/null 2>&1; then
      log_error "jq is required to modify opencode.json"
      log_info "Please install jq or manually add:"
      echo ""
      echo '  "env": { "OPENCODE_ENABLE_EXA": "1" },'
      echo '  "permission": { "websearch": "allow", "webfetch": "allow" }'
      exit 1
    fi
    
    local tmp; tmp=$(mktemp)
    jq '.env.OPENCODE_ENABLE_EXA = "1" | .permission.websearch = "allow" | .permission.webfetch = "allow"' \
      "$hub_opencode_config" > "$tmp" && mv "$tmp" "$hub_opencode_config"
    
    log_success "WebSearch enabled at hub level"
    log_info "All deployed projects will inherit this configuration"
    echo ""
    echo "  → Agents can now use WebSearch for:"
    echo "     - CVE and security advisory lookup"
    echo "     - Documentation and best practices research"
    echo "     - Stack discovery and ecosystem exploration"
    echo "     - Design patterns and UI trends"
    echo ""
    echo "  → Run './oc.sh deploy all' to apply to all projects"
    return 0
  fi
  
  # Sinon → activer au niveau projet
  id=$(normalize_project_id "$id")
  require_project_id "$id"
  
  local project_path
  project_path=$(get_project_path "$id")
  
  if [ -z "$project_path" ]; then
    log_error "Project path not found for $id"
    log_info "Register it with: ./oc.sh project add $id /path/to/project"
    exit 1
  fi
  
  local project_opencode_config="${project_path}/.opencode/opencode.json"
  
  log_info "Enabling WebSearch for project: $id"
  
  if [ ! -f "$project_opencode_config" ]; then
    mkdir -p "$(dirname "$project_opencode_config")"
    cat > "$project_opencode_config" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "OPENCODE_ENABLE_EXA": "1"
  },
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
EOF
    log_success "Created .opencode/opencode.json with WebSearch enabled"
    return 0
  fi
  
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is required to modify opencode.json"
    exit 1
  fi
  
  local tmp; tmp=$(mktemp)
  jq '.env.OPENCODE_ENABLE_EXA = "1" | .permission.websearch = "allow" | .permission.webfetch = "allow"' \
    "$project_opencode_config" > "$tmp" && mv "$tmp" "$project_opencode_config"
  
  log_success "WebSearch enabled for project: $id"
  echo ""
  echo "  Config: ${project_opencode_config}"
}

cmd_websearch_disable() {
  local id="${1:-}"
  
  if [ -z "$id" ]; then
    log_error "Usage: oc config websearch disable <PROJECT_ID>"
    log_info "To disable at hub level, manually edit opencode.json"
    exit 1
  fi
  
  id=$(normalize_project_id "$id")
  require_project_id "$id"
  
  local project_path
  project_path=$(get_project_path "$id")
  
  if [ -z "$project_path" ]; then
    log_error "Project path not found for $id"
    exit 1
  fi
  
  local project_opencode_config="${project_path}/.opencode/opencode.json"
  
  if [ ! -f "$project_opencode_config" ]; then
    log_warn "No opencode.json found for project $id"
    exit 0
  fi
  
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is required to modify opencode.json"
    exit 1
  fi
  
  local tmp; tmp=$(mktemp)
  jq 'del(.env.OPENCODE_ENABLE_EXA) | .permission.websearch = "deny"' \
    "$project_opencode_config" > "$tmp" && mv "$tmp" "$project_opencode_config"
  
  log_success "WebSearch disabled for project: $id"
}

cmd_websearch_status() {
  local id="${1:-}"
  
  # Hub level status
  echo -e "\n${BOLD}WebSearch Status${RESET}\n"
  
  local hub_opencode_config="$REPO_ROOT/opencode.json"
  
  if [ -f "$hub_opencode_config" ]; then
    echo "  Hub (opencode-hub):"
    if command -v jq >/dev/null 2>&1; then
      local hub_exa hub_perm
      hub_exa=$(jq -r '.env.OPENCODE_ENABLE_EXA // "not set"' "$hub_opencode_config" 2>/dev/null)
      hub_perm=$(jq -r '.permission.websearch // "not set"' "$hub_opencode_config" 2>/dev/null)
      echo "    OPENCODE_ENABLE_EXA: $hub_exa"
      echo "    permission.websearch: $hub_perm"
      if [ "$hub_exa" = "1" ] && [ "$hub_perm" = "allow" ]; then
        echo -e "    Status: ${GREEN}✓ Enabled${RESET}"
      else
        echo -e "    Status: ${YELLOW}○ Disabled${RESET}"
      fi
    else
      echo "    (jq required to read config)"
    fi
  else
    echo "  Hub: No opencode.json found"
  fi
  
  # Project level status
  if [ -n "$id" ]; then
    id=$(normalize_project_id "$id")
    require_project_id "$id"
    
    local project_path
    project_path=$(get_project_path "$id")
    
    if [ -z "$project_path" ]; then
      log_warn "Project path not found for $id"
      exit 1
    fi
    
    local project_opencode_config="${project_path}/.opencode/opencode.json"
    
    echo ""
    echo "  Project ($id):"
    if [ -f "$project_opencode_config" ]; then
      if command -v jq >/dev/null 2>&1; then
        local proj_exa proj_perm
        proj_exa=$(jq -r '.env.OPENCODE_ENABLE_EXA // "not set"' "$project_opencode_config" 2>/dev/null)
        proj_perm=$(jq -r '.permission.websearch // "not set"' "$project_opencode_config" 2>/dev/null)
        echo "    OPENCODE_ENABLE_EXA: $proj_exa"
        echo "    permission.websearch: $proj_perm"
        if [ "$proj_exa" = "1" ] && [ "$proj_perm" = "allow" ]; then
          echo -e "    Status: ${GREEN}✓ Enabled${RESET}"
        else
          echo -e "    Status: ${YELLOW}○ Disabled${RESET}"
        fi
      else
        echo "    (jq required to read config)"
      fi
    else
      echo "    No project-specific opencode.json"
      echo "    → inherits from hub config"
    fi
  fi
  
  echo ""
}

# ── Dispatcher ─────────────────────────────────────────────────────

# Si sourcé pour les fonctions uniquement, ne pas exécuter le dispatcher
if [ "${_CMD_CONFIG_SOURCE_ONLY:-}" = "1" ]; then return 0 2>/dev/null || exit 0; fi

case "$SUBCOMMAND" in
  set)   cmd_set "$@" ;;
  get)   cmd_get "$@" ;;
  list)  cmd_list ;;
  unset) cmd_unset "$@" ;;
  websearch) cmd_websearch "$@" ;;
  "")
    echo -e "${BOLD}$(t config.title)${RESET}"
    echo ""
    echo "  $(t help.config_set)"
    echo "  $(t help.config_set_desc)"
    echo "  $(t help.config_language)"
    echo "  $(t help.config_get)"
    echo "  $(t help.config_list)"
    echo "  $(t help.config_unset)"
    echo ""
    echo "  oc config websearch <enable|disable|status> [PROJECT_ID]"
    echo "  Manage WebSearch (Exa AI) integration"
    exit 0
    ;;
  *)
    log_error "$(t subcmd.unknown) : $SUBCOMMAND"
    echo ""
    echo -e "${BOLD}$(t config.title)${RESET}"
    echo ""
    echo "  $(t help.config_set)"
    echo "  $(t help.config_set_desc)"
    echo "  $(t help.config_language)"
    echo "  $(t help.config_get)"
    echo "  $(t help.config_list)"
    echo "  $(t help.config_unset)"
    echo ""
    echo "  oc config websearch <enable|disable|status> [PROJECT_ID]"
    exit 1
    ;;
esac
