#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-service.sh — Gestion générique des services/intégrations MCP
#
# Usage :
#   oc service [setup|status|list|remove|help] [service-id]
#
# Alias supportés (via oc.sh) :
#   oc figma <cmd>   → oc service <cmd> figma
#   oc gitlab <cmd>  → oc service <cmd> gitlab
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/services.sh"
resolve_oc_lang

# ── Helper wizard ─────────────────────────────────────────────────────────────

_svc_step() {
  local num="$1" total="$2" label="$3"
  echo ""
  echo -e "${DIM}│${RESET}"
  echo -e "${CYAN}◇${RESET}  ${BOLD}$(t service.step) ${num}/${total} — ${label}${RESET}"
  echo -e "${DIM}│${RESET}"
}

_svc_ok() {
  echo -e "  ${GREEN}✓${RESET}  $*"
}

_svc_fail() {
  echo -e "  ${RED}✗${RESET}  $*"
}

_svc_info() {
  echo -e "  ${DIM}→${RESET}  $*"
}

# Masque un secret : affiche les 4 derniers caractères, remplace le reste par *
_svc_mask() {
  local value="$1"
  local len="${#value}"
  if [ "$len" -le 4 ]; then
    printf '%s' "****"
  else
    local tail="${value: -4}"
    printf '%s' "****${tail}"
  fi
}

# ── Sous-commande : list ──────────────────────────────────────────────────────

cmd_service_list() {
  _intro "$(t service.list.title)"

  local services
  services=$(svc_list_available 2>/dev/null) || services=""
  if [ -z "$services" ]; then
    log_warn "$(t service.catalogue.empty)"
    _outro ""
    return 0
  fi

  # En-tête du tableau
  printf "  ${BOLD}%-12s  %-35s  %s${RESET}\n" "$(t service.col.name)" "$(t service.col.description)" "$(t service.col.status)"
  printf "  %-12s  %-35s  %s\n" "────────────" "───────────────────────────────────" "──────────────"

  while IFS= read -r svc_id; do
    local label description status_str
    label=$(svc_get_field "$svc_id" "label" 2>/dev/null || echo "$svc_id")
    description=$(svc_localized "$svc_id" "description" 2>/dev/null || echo "")
    # Tronquer si nécessaire
    [ "${#description}" -gt 35 ] && description="${description:0:32}..."

    if svc_is_configured "$svc_id" 2>/dev/null; then
      status_str="${GREEN}$(t service.status.configured)${RESET}"
    else
      status_str="${DIM}$(t service.status.not_configured)${RESET}"
    fi

    printf "  ${CYAN}%-12s${RESET}  %-35s  " "$label" "$description"
    echo -e "$status_str"
  done <<< "$services"

  echo ""
  _outro "$(t service.list.hint)"
}

# ── Sous-commande : status ────────────────────────────────────────────────────

cmd_service_status() {
  local filter_id="${1:-}"
  _intro "$(t service.status.title)"

    local services
    if [ -n "$filter_id" ]; then
      if ! svc_exists "$filter_id"; then
        log_error "$(t service.unknown) : $filter_id"
        _outro ""
        return 1
      fi
      services="$filter_id"
    else
      services=$(svc_list_available 2>/dev/null) || services=""
      if [ -z "$services" ]; then
        log_warn "$(t service.catalogue.empty)"
        _outro ""
        return 0
      fi
    fi

  while IFS= read -r svc_id; do
    local label
    label=$(svc_get_field "$svc_id" "label" 2>/dev/null || echo "$svc_id")
    echo -e "${DIM}│${RESET}"
    echo -e "  ${BOLD}${CYAN}${label}${RESET}"

    # Credentials
    local count
    count=$(svc_credential_count "$svc_id" 2>/dev/null || echo 0)
    for (( i=0; i<count; i++ )); do
      local cred_key cred_label value
      cred_key=$(svc_get_credential "$svc_id" "$i" "key")
      cred_label=$(svc_localized_credential "$svc_id" "$i" "label")
      value=$(svc_get_env_value "$cred_key" 2>/dev/null || echo "")
      if [ -n "$value" ]; then
        local secret
        secret=$(svc_get_credential_bool "$svc_id" "$i" "secret")
        if [ "$secret" = "true" ]; then
          _svc_ok "${cred_label} : $(_svc_mask "$value")"
        else
          _svc_ok "${cred_label} : ${value}"
        fi
      else
        _svc_fail "${cred_label} : $(t service.status.not_configured)"
      fi
    done

    # Validation token (si endpoint défini)
    local endpoint
    endpoint=$(jq -r --arg s "$svc_id" '.services[$s].validation.endpoint // empty' "$SERVICES_FILE" 2>/dev/null || echo "")
    if [ -n "$endpoint" ]; then
      local handle
      if handle=$(svc_validate_token "$svc_id" 2>/dev/null); then
        if [ -n "$handle" ]; then
          _svc_ok "$(t service.status.valid) (${handle})"
        else
          _svc_ok "$(t service.status.valid)"
        fi
      else
        _svc_fail "$(t service.status.invalid)"
      fi
    fi

    # MCP build
    local mcp_server
    mcp_server=$(svc_get_field "$svc_id" "mcp_server" 2>/dev/null || echo "")
    if [ -n "$mcp_server" ]; then
      if svc_is_mcp_built "$svc_id" 2>/dev/null; then
        _svc_ok "MCP : $(t service.status.built) (${mcp_server})"
      else
        _svc_fail "MCP : $(t service.status.not_built) (${mcp_server})"
        _svc_info "$(t service.build.hint) : oc service setup ${svc_id}"
      fi
    fi
  done <<< "$services"

  echo ""
  _outro ""
}

# ── Sous-commande : remove ────────────────────────────────────────────────────

cmd_service_remove() {
  local service_id="${1:-}"
  if [ -z "$service_id" ]; then
    log_error "$(t service.id.required)"
    exit 1
  fi

  if ! svc_exists "$service_id"; then
    log_error "$(t service.unknown) : $service_id"
    exit 1
  fi

  local label
  label=$(svc_get_field "$service_id" "label")

  _intro "$(t service.remove.title)"

  if ! svc_is_configured "$service_id" 2>/dev/null; then
    log_warn "$(t service.remove.not_configured) : $label"
    _outro ""
    return 0
  fi

  # Confirmation
  _prompt confirm "$(t service.remove.confirm) ${label} ? [y/N] : "
  if [[ ! "${confirm:-N}" =~ ^[Yy]$ ]]; then
    _outro "$(t cancelled)"
    return 0
  fi

  svc_remove_env_values "$service_id"
  _svc_ok "$(t service.remove.done) : $label"
  _outro ""
}

# ── Sous-commande : setup ─────────────────────────────────────────────────────

cmd_service_setup() {
  local service_id="${1:-}"

  # Si pas de service fourni → menu de sélection interactif
  if [ -z "$service_id" ]; then
    _intro "$(t service.setup.title)"
    echo -e "${DIM}│${RESET}"
    echo -e "  $(t service.setup.select)"
    echo ""

    local services
    services=$(svc_list_available 2>/dev/null)
    if [ -z "$services" ]; then
      log_error "$(t service.catalogue.empty)"
      exit 1
    fi

    # Afficher le menu numéroté
    local menu_items=()
    while IFS= read -r svc_id; do
      menu_items+=("$svc_id")
    done <<< "$services"

    local idx=1
    for svc_id in "${menu_items[@]}"; do
      local label desc
      label=$(svc_get_field "$svc_id" "label" 2>/dev/null || echo "$svc_id")
      desc=$(svc_localized "$svc_id" "description" 2>/dev/null || echo "")
      printf "    ${BLUE}%d${RESET}.  ${CYAN}%-12s${RESET}  %s\n" "$idx" "$label" "$desc"
      idx=$((idx + 1))
    done
    echo ""

    _prompt choice "$(t service.setup.choose) [1] : "
    local chosen_idx="${choice:-1}"
    # Valider le choix
    if ! [[ "$chosen_idx" =~ ^[0-9]+$ ]] || \
       [ "$chosen_idx" -lt 1 ] || \
       [ "$chosen_idx" -gt "${#menu_items[@]}" ]; then
      log_error "$(t invalid_choice) : $chosen_idx"
      exit 1
    fi
    service_id="${menu_items[$((chosen_idx - 1))]}"
    echo ""
  fi

  # Vérifier que le service existe
  if ! svc_exists "$service_id"; then
    log_error "$(t service.unknown) : $service_id"
    exit 1
  fi

  local label
  label=$(svc_get_field "$service_id" "label")
  local total_creds
  total_creds=$(svc_credential_count "$service_id")
  # Étapes = credentials + validation + sauvegarde
  local total_steps=$(( total_creds + 2 ))

  _intro "$(t service.setup.title) — ${label}"

  # ── Collecte des credentials ──────────────────────────────────────────────
  local collected_keys=()
  local collected_values=()

  for (( i=0; i<total_creds; i++ )); do
    local cred_key cred_label cred_secret cred_required cred_default cred_pattern cred_help
    cred_key=$(svc_get_credential "$service_id" "$i" "key")
    cred_label=$(svc_localized_credential "$service_id" "$i" "label")
    cred_secret=$(svc_get_credential_bool "$service_id" "$i" "secret")
    cred_required=$(svc_get_credential_bool "$service_id" "$i" "required")
    cred_default=$(svc_get_credential "$service_id" "$i" "default")
    cred_pattern=$(svc_get_credential "$service_id" "$i" "validation_pattern")
    cred_help=$(svc_localized_credential "$service_id" "$i" "help")

    _svc_step "$((i + 1))" "$total_steps" "$cred_label"

    # Afficher l'aide si disponible
    if [ -n "$cred_help" ]; then
      echo -e "  ${DIM}$(t service.setup.help)${RESET}"
      while IFS= read -r help_line; do
        echo -e "    ${DIM}${help_line}${RESET}"
      done <<< "$cred_help"
      echo -e "${DIM}│${RESET}"
    fi

    # Valeur existante
    local existing_value
    existing_value=$(svc_get_env_value "$cred_key" 2>/dev/null || echo "")
    if [ -n "$existing_value" ]; then
      local existing_display
      if [ "$cred_secret" = "true" ]; then
        existing_display=$(_svc_mask "$existing_value")
      else
        existing_display="$existing_value"
      fi
      echo -e "  ${DIM}$(t service.setup.existing) : ${existing_display}${RESET}"
      _prompt keep_existing "$(t service.setup.keep) [Y/n] : "
      if [[ "${keep_existing:-Y}" =~ ^[Yy]$ ]]; then
        collected_keys+=("$cred_key")
        collected_values+=("$existing_value")
        _svc_ok "$(t service.setup.kept)"
        continue
      fi
    fi

    # Saisie de la valeur
    local input_value=""
    while true; do
      if [ "$cred_secret" = "true" ]; then
        if [ "${OC_NON_INTERACTIVE:-0}" = "1" ]; then
          input_value="${!cred_key:-}"
        else
          echo -e "${DIM}│${RESET}"
          trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
          read -rsp "  ${cred_label} : " input_value
          stty echo 2>/dev/null
          trap - INT TERM
          echo ""
        fi
      else
        if [ "${OC_NON_INTERACTIVE:-0}" = "1" ]; then
          # En mode non-interactif, utiliser la variable d'env si disponible
          local env_val="${!cred_key:-}"
          input_value="${env_val:-${cred_default:-}}"
        elif [ -n "$cred_default" ]; then
          _prompt input_value "${cred_label} [${cred_default}] : "
          input_value="${input_value:-$cred_default}"
        else
          _prompt input_value "${cred_label} : "
        fi
      fi

      # Valeur vide sur champ requis
      if [ -z "$input_value" ] && [ "$cred_required" = "true" ]; then
        # En mode non-interactif, on accepte la valeur vide (ne pas boucler)
        if [ "${OC_NON_INTERACTIVE:-0}" = "1" ]; then
          log_warn "$(t service.setup.required) : $cred_key"
          break
        fi
        log_warn "$(t service.setup.required)"
        continue
      fi

      # Validation du format si pattern défini
      if [ -n "$cred_pattern" ] && [ -n "$input_value" ]; then
        if ! echo "$input_value" | grep -qE "$cred_pattern"; then
          log_warn "$(t service.setup.invalid_format) ($(t service.setup.expected) : ${cred_pattern})"
          # En mode non-interactif, on ne redemande pas
          if [ "${OC_NON_INTERACTIVE:-0}" = "1" ]; then
            break
          fi
          _prompt retry "$(t service.setup.retry) [Y/n] : "
          if [[ "${retry:-Y}" =~ ^[Yy]$ ]]; then
            continue
          fi
        fi
      fi

      break
    done

    collected_keys+=("$cred_key")
    collected_values+=("$input_value")
  done

  # ── Validation API ────────────────────────────────────────────────────────
  _svc_step "$((total_creds + 1))" "$total_steps" "$(t service.validation.title)"

  local endpoint
  endpoint=$(jq -r --arg s "$service_id" '.services[$s].validation.endpoint // empty' \
    "$SERVICES_FILE" 2>/dev/null || echo "")

  if [ -n "$endpoint" ]; then
    # Écrire temporairement les valeurs pour pouvoir valider
    for (( i=0; i<${#collected_keys[@]}; i++ )); do
      svc_set_env_value "${collected_keys[$i]}" "${collected_values[$i]}" 2>/dev/null || true
    done

    local handle
    local validation_ok=false
    local retry_count=0
    while [ $retry_count -lt 3 ]; do
      _svc_info "$(t service.validation.testing)..."
      if handle=$(svc_validate_token "$service_id" 2>/dev/null); then
        validation_ok=true
        break
      fi
      retry_count=$((retry_count + 1))
      if [ $retry_count -lt 3 ]; then
        log_warn "$(t service.validation.ko)"
        _prompt retry_val "$(t service.setup.retry) [Y/n] : "
        if [[ ! "${retry_val:-Y}" =~ ^[Yy]$ ]]; then
          break
        fi
      fi
    done

    if $validation_ok; then
      if [ -n "${handle:-}" ]; then
        _svc_ok "$(t service.validation.ok) (${handle})"
      else
        _svc_ok "$(t service.validation.ok)"
      fi
    else
      log_warn "$(t service.validation.ko)"
      _prompt continue_anyway "$(t service.setup.continue_anyway) [y/N] : "
      if [[ ! "${continue_anyway:-N}" =~ ^[Yy]$ ]]; then
        # Nettoyer les valeurs temporaires
        svc_remove_env_values "$service_id" 2>/dev/null || true
        _outro "$(t cancelled)"
        exit 1
      fi
    fi
  else
    _svc_info "$(t service.validation.skip)"
  fi

  # ── Sauvegarde & Build ────────────────────────────────────────────────────
  _svc_step "$total_steps" "$total_steps" "$(t service.save.title)"

  # Sauvegarder toutes les valeurs collectées
  for (( i=0; i<${#collected_keys[@]}; i++ )); do
    if [ -n "${collected_values[$i]}" ]; then
      svc_set_env_value "${collected_keys[$i]}" "${collected_values[$i]}"
      local cred_label_save
      cred_label_save=$(svc_localized_credential "$service_id" "$i" "label" 2>/dev/null || echo "${collected_keys[$i]}")
      _svc_ok "${cred_label_save} $(t service.saved)"
    fi
  done

  # Build MCP si nécessaire
  local mcp_server
  mcp_server=$(svc_get_field "$service_id" "mcp_server" 2>/dev/null || echo "")
  if [ -n "$mcp_server" ]; then
    if svc_is_mcp_built "$service_id" 2>/dev/null; then
      _svc_ok "MCP ${mcp_server} : $(t service.status.built)"
    else
      _svc_info "$(t service.build.start) (${mcp_server})..."
      if svc_build_mcp "$service_id" 2>/dev/null; then
        _svc_ok "$(t service.build.done)"
      else
        log_warn "$(t service.build.failed)"
        _svc_info "$(t service.build.manual) : bash scripts/build-mcp.sh ${mcp_server}"
      fi
    fi
  fi

  # ── Récapitulatif ─────────────────────────────────────────────────────────
  echo ""
  echo -e "${DIM}│${RESET}"
  local width=50
  local bar=""
  local i=0
  while [ "$i" -lt "$width" ]; do bar="${bar}─"; i=$(( i + 1 )); done
  echo -e "${GREEN}┌─ ${BOLD}${label} $(t service.setup.done)${RESET}${GREEN} ────────────┐${RESET}"
  for (( i=0; i<${#collected_keys[@]}; i++ )); do
    local display_value="${collected_values[$i]}"
    local is_secret
    is_secret=$(svc_get_credential_bool "$service_id" "$i" "secret" 2>/dev/null || echo "false")
    [ "$is_secret" = "true" ] && display_value=$(_svc_mask "$display_value")
    local cred_lbl
    cred_lbl=$(svc_localized_credential "$service_id" "$i" "label" 2>/dev/null || echo "${collected_keys[$i]}")
    printf "${GREEN}│${RESET}  %-20s %-26s ${GREEN}│${RESET}\n" "${cred_lbl}" "${display_value:0:26}"
  done
  echo -e "${GREEN}│${RESET}"
  printf "${GREEN}│${RESET}  %-48s ${GREEN}│${RESET}\n" "$(t service.setup.status_hint) : oc service status ${service_id}"
  echo -e "${GREEN}└─${bar:0:48}──┘${RESET}"

  _outro "$(t service.setup.outro)"
}

# ── Sous-commande : help ──────────────────────────────────────────────────────

cmd_service_help() {
  echo -e "${BOLD}$(t service.help.title)${RESET}"
  echo ""
  echo -e "  $(t service.help.usage)"
  echo ""
  echo -e "  ${CYAN}$(t service.help.setup_cmd)${RESET}    $(t service.help.setup_desc)"
  echo -e "  ${CYAN}$(t service.help.status_cmd)${RESET}   $(t service.help.status_desc)"
  echo -e "  ${CYAN}$(t service.help.list_cmd)${RESET}     $(t service.help.list_desc)"
  echo -e "  ${CYAN}$(t service.help.remove_cmd)${RESET}   $(t service.help.remove_desc)"
  echo ""
  echo -e "  ${DIM}$(t service.help.aliases)${RESET}"
  echo -e "    ${CYAN}oc figma${RESET}  →  oc service ... figma"
  echo -e "    ${CYAN}oc gitlab${RESET} →  oc service ... gitlab"
  echo ""
}

# ── Dispatcher ────────────────────────────────────────────────────────────────
# Guard : permet de sourcer sans exécuter le dispatcher (pour les tests)
[ -n "${_CMD_SERVICE_SOURCE_ONLY:-}" ] && return 0

SUBCOMMAND="${1:-}"
shift || true

case "$SUBCOMMAND" in
  setup)    cmd_service_setup "$@" ;;
  status)   cmd_service_status "$@" ;;
  list)     cmd_service_list ;;
  remove)   cmd_service_remove "$@" ;;
  help|--help|-h) cmd_service_help ;;
  "")       cmd_service_list ;;
  *)
    log_error "$(t subcmd.unknown) : $SUBCOMMAND"
    cmd_service_help
    exit 1
    ;;
esac
