#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
resolve_oc_lang

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

# Supprime une section [PROJECT_ID] complète du fichier (délègue à common.sh)
_remove_section() {
  remove_api_keys_section "$1"
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

cmd_set() {
  local id="${1:-}"
  shift || true

  # Special case: oc config set language <en|fr>
  if [ "$id" = "language" ]; then
    cmd_set_language "$@"
    return
  fi

  # Flags optionnels
  local flag_model="" flag_provider="" flag_api_key="" flag_base_url=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --model)    [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_model="$2";    shift 2 ;;
      --provider) [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_provider="$2"; shift 2 ;;
      --api-key)  [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_api_key="$2";  shift 2 ;;
      --base-url) [ $# -ge 2 ] || { log_error "Option $1 requiert une valeur"; exit 1; }; flag_base_url="$2"; shift 2 ;;
      *) log_error "Option inconnue : $1"; exit 1 ;;
    esac
  done

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

  # Clé API (saisie masquée)
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
      PROJECT_ID="$id" bash "$SCRIPTS_DIR/cmd-deploy.sh" all "$id"
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

# ── Dispatcher ─────────────────────────────────────────────────────

case "$SUBCOMMAND" in
  set)   cmd_set "$@" ;;
  get)   cmd_get "$@" ;;
  list)  cmd_list ;;
  unset) cmd_unset "$@" ;;
  "")
    echo -e "${BOLD}$(t config.title)${RESET}"
    echo ""
    echo "  $(t help.config_set)"
    echo "  $(t help.config_set_desc)"
    echo "  $(t help.config_language)"
    echo "  $(t help.config_get)"
    echo "  $(t help.config_list)"
    echo "  $(t help.config_unset)"
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
    exit 1
    ;;
esac
