#!/bin/bash
# Gestion des cibles de déploiement par projet.
# Usage : oc target <sous-commande> [args]
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/target-picker.sh"
resolve_oc_lang

##
# Affiche les cibles configurées pour un projet.
# @param {string} $1 — PROJECT_ID
##
cmd_info() {
  local raw_id="${1:-}"
  if [ -z "$raw_id" ]; then
    log_error "$(t target.usage.info)"
    exit 1
  fi

  local id
  id=$(normalize_project_id "$raw_id")
  if ! project_exists "$id"; then
    log_error "$(t project_id.required) : $id → ./oc.sh list"
    exit 1
  fi

  local current
  current=$(get_project_targets "$id")
  echo ""
  if [ -z "$current" ]; then
    echo -e "  $(t target.targets_label) ${BOLD}$id${RESET} : $(t target.all_active)"
  else
    echo -e "  $(t target.targets_label) ${BOLD}$id${RESET} : $current"
  fi
  echo ""
}

##
# Sélectionne les cibles de déploiement pour un projet donné.
# Lance le picker interactif et écrit le résultat dans projects.md.
# @param {string} $1 — PROJECT_ID
##
cmd_select() {
  local raw_id="${1:-}"
  if [ -z "$raw_id" ]; then
    log_error "$(t target.usage.select)"
    exit 1
  fi

  local id
  id=$(normalize_project_id "$raw_id")
  if ! project_exists "$id"; then
    log_error "$(t project_id.required) : $id → ./oc.sh list"
    exit 1
  fi

  local current
  current=$(get_project_targets "$id")
  log_title "$(t target.select.title) $id"
  if [ -z "$current" ]; then
    log_info "$(t target.current_all)"
  else
    log_info "Sélection actuelle : ${current}"
  fi
  echo ""

  PICKED_TARGETS=""
  _pick_project_targets "${current:-all}"

  # Normaliser : "all" → vide (= fallback hub.json)
  local new_targets="$PICKED_TARGETS"
  [ "$new_targets" = "all" ] && new_targets=""

  if [ "$new_targets" = "$current" ]; then
    log_info "$(t no_modification)"
    return
  fi

  if [ -z "$new_targets" ]; then
    # Supprimer le champ Targets → retour au comportement global
    _set_project_targets "$id" "all"
    echo ""
    log_success "$(t target.reset_done) $id — $(t target.reset_suffix)"
  else
    _set_project_targets "$id" "$new_targets"
    echo ""
    local count
    count=$(echo "$new_targets" | tr ',' '\n' | grep -v '^$' | wc -l | tr -d ' ')
    log_success "$count $(t target.selected) $id : $new_targets"
  fi

  # Proposer un redéploiement immédiat
  echo ""
  read -rp "$(t start.deploy_now)" redeploy </dev/tty
  redeploy="${redeploy:-Y}"
  if [[ "$redeploy" =~ ^[Yy]$ ]]; then
    exec "$HUB_DIR/oc.sh" deploy all "$id"
  else
    log_info "$(t deploy_later) $id"
  fi
}

# ── DISPATCH ─────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-}"
shift 2>/dev/null || true

case "$SUBCOMMAND" in
  info)    cmd_info "$@" ;;
  select)  cmd_select "$@" ;;
  *)
    echo -e "${BOLD}$(t target.title)${RESET}"
    echo ""
    echo "  $(t target.info_cmd)"
    echo "  $(t target.select_cmd)"
    echo ""
    echo -e "${BOLD}$(t target.examples)${RESET}"
    echo "  ./oc.sh target info MY-PROJECT"
    echo "  ./oc.sh target select MY-PROJECT"
    echo ""
    echo -e "${BOLD}$(t target.available)${RESET}"
    echo "  opencode     → .opencode/agents/"
    echo ""
    echo "  $(t target.default_hint)"
    echo ""
    ;;
esac
