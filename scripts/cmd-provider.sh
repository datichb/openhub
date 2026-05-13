#!/bin/bash
# Gestion des fournisseurs LLM — configuration hub et par-projet

set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
resolve_oc_lang

# _build_provider_menu et _collect_provider_credentials sont définis dans lib/providers.sh
# sourcé via common.sh

# ────────────────────────────────────────────────────────────────────────────────
# Subcommande : oc provider list
# Affiche tous les fournisseurs du catalogue avec leur statut
# ────────────────────────────────────────────────────────────────────────────────
cmd_list() {
  log_title "$(t provider.title)"
  echo ""

  [ ! -f "$PROVIDERS_FILE" ] && { log_error "$(t provider.no_catalog)"; exit 1; }

  local hub_provider; hub_provider=$(get_hub_default_provider)
  local hub_api_key; hub_api_key=$(get_hub_default_api_key)

  jq -r '.providers | keys[]' "$PROVIDERS_FILE" | while read -r pname; do
    local label; label=$(get_provider_info "$pname" "label")
    local desc; desc=$(get_provider_info "$pname" "description")
    local targets_raw; targets_raw=$(jq -r --arg n "$pname" '.providers[$n].supported_targets[]' "$PROVIDERS_FILE" 2>/dev/null | paste -sd ',' -)

    # Statut hub
    local status=""
    if [ "$pname" = "$hub_provider" ]; then
      if [ -n "$hub_api_key" ]; then
        status=" ${GREEN}◆ fournisseur du hub${RESET}"
      else
        status=" ${YELLOW}◆ fournisseur du hub (clé non configurée)${RESET}"
      fi
    fi

    printf "  ${BOLD}%s${RESET}%b\n" "$label" "$status"
    printf "    %s\n" "$desc"
    [ -n "$targets_raw" ] && printf "    Cibles : %s\n" "$targets_raw"
    echo ""
  done
}

# ────────────────────────────────────────────────────────────────────────────────
# Subcommande : oc provider set-default
# Configure le fournisseur par défaut au niveau hub
# ────────────────────────────────────────────────────────────────────────────────
cmd_set_default() {
  log_title "$(t provider.default_title)"

  [ ! -f "$PROVIDERS_FILE" ] && { log_error "$(t provider.no_catalog)"; exit 1; }
  [ ! -f "$HUB_CONFIG" ] && { log_error "$(t provider.hub_json_missing)"; exit 1; }

  # Afficher le fournisseur actuel comme contexte
  local current_provider; current_provider=$(get_hub_default_provider)
  local current_api_key; current_api_key=$(get_hub_default_api_key)
  echo ""
  if [ -n "$current_provider" ]; then
    local current_label; current_label=$(get_provider_info "$current_provider" "label" 2>/dev/null || echo "$current_provider")
    if [ -n "$current_api_key" ]; then
      local masked="${current_api_key:0:4}***"
      echo -e "  Fournisseur actuel : ${BOLD}${current_label}${RESET} (clé : ${masked})"
    else
      echo -e "  Fournisseur actuel : ${BOLD}${current_label}${RESET} ${YELLOW}(clé non configurée)${RESET}"
    fi
    echo ""
  fi

  log_info "$(t provider.choose_default)"
  echo ""

  local providers_array=()
  _build_provider_menu providers_array
  echo ""

  read -rp "  Numéro (1-${#providers_array[@]}) : " choice

  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#providers_array[@]}" ]; then
    log_error "Choix invalide : '$choice'"
    exit 1
  fi

  local selected_provider="${providers_array[$((choice - 1))]}"
  local selected_label; selected_label=$(get_provider_info "$selected_provider" "label")
  local requires_api_key; requires_api_key=$(get_provider_info "$selected_provider" "requires_api_key")

  echo ""
  log_info "$(t provider.selected) ${BOLD}${selected_label}${RESET}"

  _cred_api_key=""
  _cred_base_url=""
  _collect_provider_credentials "$selected_provider" "$selected_label"

  # Vérification clé si requise
  if [ "$requires_api_key" = "true" ] && [ -z "$_cred_api_key" ]; then
    log_warn "$(t provider.api_key_empty_warn)"
  fi

  # Mettre à jour hub.json (avec région si bedrock)
  local tmp; tmp=$(mktemp)
  if [ -n "${_cred_region:-}" ]; then
    jq \
      --arg name   "$selected_provider" \
      --arg key    "$_cred_api_key" \
      --arg url    "$_cred_base_url" \
      --arg region "${_cred_region}" \
      '.default_provider.name = $name | .default_provider.api_key = $key | .default_provider.base_url = $url | .default_provider.region = $region' \
      "$HUB_CONFIG" > "$tmp"
  else
    jq \
      --arg name "$selected_provider" \
      --arg key  "$_cred_api_key" \
      --arg url  "$_cred_base_url" \
      '.default_provider.name = $name | .default_provider.api_key = $key | .default_provider.base_url = $url | del(.default_provider.region)' \
      "$HUB_CONFIG" > "$tmp"
  fi
  mv "$tmp" "$HUB_CONFIG"

  # Protéger hub.json si clé présente
  if [ -n "$_cred_api_key" ]; then
    local gitignore="$HUB_DIR/.gitignore"
    if [ ! -f "$gitignore" ] || ! grep -qx "config/hub.json" "$gitignore"; then
      echo "config/hub.json" >> "$gitignore"
      log_info "$(t provider.hub_json_added_gitignore)"
    fi
  fi

  echo ""
  log_success "$(t provider.saved) ${selected_label}"
  [ -n "$_cred_base_url" ] && log_info "URL de base : ${_cred_base_url}"

  # Régénérer opencode.json du hub immédiatement pour que la config soit active
  source "$HUB_DIR/scripts/lib/adapter-manager.sh"
  local active_targets
  active_targets=$(get_active_targets)
  local _synced=false
  while IFS= read -r target; do
    [ -z "$target" ] && continue
    load_adapter "$target"
    if declare -F adapter_deploy &>/dev/null; then
      log_info "Mise à jour de ${target} (opencode.json)..."
      adapter_deploy "$HUB_DIR" ""
      _synced=true
    fi
  done <<< "$active_targets"
  [ "$_synced" = false ] && log_info "$(t provider.apply_hint)"
}

# ────────────────────────────────────────────────────────────────────────────────
# Main dispatcher
# ────────────────────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-list}"
case "$SUBCOMMAND" in
  list)        cmd_list "${@:2}" ;;
  set-default) cmd_set_default "${@:2}" ;;
  set|get)
    log_error "provider ${SUBCOMMAND} est supprimé — utilisez : oc config ${SUBCOMMAND} <PROJECT_ID>"
    exit 1
    ;;
  *)
    log_error "$(t subcmd.unknown) : $SUBCOMMAND"
    echo ""
    echo "$(t provider.usage)"
    echo ""
    echo "  $(t provider.list_cmd)"
    echo "  $(t provider.set_default_cmd)"
    exit 1
    ;;
esac
