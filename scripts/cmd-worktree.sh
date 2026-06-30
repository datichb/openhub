#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-worktree.sh — Gestion des git worktrees pour le travail en parallèle
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc worktree list [PROJECT_ID]
#   oc worktree create BRANCH [PROJECT_ID]
#   oc worktree remove BRANCH [PROJECT_ID]
#   oc worktree cleanup [PROJECT_ID]
#   oc worktree status [PROJECT_ID]
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/worktree.sh"

ensure_projects_file

# ── Sous-commande et arguments ────────────────────────────────────────────────
SUBCOMMAND="${1:-}"
shift || true

# ── Aide ─────────────────────────────────────────────────────────────────────
_worktree_help() {
  _h_section "oc worktree <subcommand> [options]"

  _h_cmd "list [PROJECT_ID]"              "$(t help.worktree_list.desc)"
  _h_cmd "create <BRANCH> [PROJECT_ID]"   "$(t help.worktree_create.desc)"
  _h_cmd "remove <BRANCH> [PROJECT_ID]"   "$(t help.worktree_remove.desc)"
  _h_cmd "cleanup [PROJECT_ID]"           "$(t help.worktree_cleanup.desc)"
  _h_cmd "status [PROJECT_ID]"            "$(t help.worktree_status.desc)"

  _h_section "Examples"
  _h_note "oc worktree create feat/bd-42 MY-APP"
  _h_note "oc worktree list MY-APP"
  _h_note "oc worktree remove feat/bd-42 MY-APP"
  _h_note "oc worktree cleanup MY-APP"

  echo ""
}

# ── Résolution interactive du projet ─────────────────────────────────────────
# Usage : _resolve_project_interactive [PROJECT_ID]
# Retourne PROJECT_ID dans $PROJECT_ID (variable parente)
_resolve_project() {
  local id="${1:-}"

  if [ -z "$id" ]; then
    local ids=()
    while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

    if [ ${#ids[@]} -eq 0 ]; then
      log_error "Aucun projet enregistré → oc init"
      exit 1
    fi

    echo -e "${BOLD}Sélectionner un projet :${RESET}"
    echo ""
    for i in "${!ids[@]}"; do
      printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
    done
    echo ""
    read -rp "  Numéro : " choice || true
    if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
      log_error "Choix invalide : $choice"
      exit 1
    fi
    id="${ids[$((choice-1))]}"
  fi

  echo "$(normalize_project_id "$id")"
}

# ── oc worktree list ──────────────────────────────────────────────────────────
_cmd_list() {
  local project_id
  project_id=$(_resolve_project "${1:-}")
  local project_path
  project_path=$(resolve_project_path "$project_id")
  local base_branch
  base_branch=$(get_project_worktree_base_branch "$project_id")

  _intro "oc worktree list  ${project_id}"
  printf "${DIM}│${RESET}  %-12s %s\n" "Chemin"  "$project_path"
  printf "${DIM}│${RESET}  %-12s %s\n" "Base"    "$base_branch"
  echo -e "${DIM}│${RESET}"

  worktree_list "$project_path" "$base_branch"

  echo ""
  _outro "Pour nettoyer les mergés : oc worktree cleanup ${project_id}"
}

# ── oc worktree create ────────────────────────────────────────────────────────
_cmd_create() {
  local branch="${1:-}"
  local project_id

  if [ -z "$branch" ]; then
    log_error "Usage : oc worktree create BRANCH [PROJECT_ID]"
    exit 1
  fi
  shift || true
  project_id=$(_resolve_project "${1:-}")
  local project_path
  project_path=$(resolve_project_path "$project_id")

  _intro "oc worktree create  ${project_id}"
  printf "${DIM}│${RESET}  %-12s %s\n" "Chemin"   "$project_path"
  printf "${DIM}│${RESET}  %-12s %s\n" "Branche"  "$branch"
  echo -e "${DIM}│${RESET}"

  local wt_path
  wt_path=$(worktree_create "$project_path" "$branch")

  echo -e "${DIM}│${RESET}"
  _outro "Worktree prêt : $wt_path"
}

# ── oc worktree remove ────────────────────────────────────────────────────────
_cmd_remove() {
  local branch="${1:-}"
  local project_id

  if [ -z "$branch" ]; then
    log_error "Usage : oc worktree remove BRANCH [PROJECT_ID]"
    exit 1
  fi
  shift || true
  project_id=$(_resolve_project "${1:-}")
  local project_path
  project_path=$(resolve_project_path "$project_id")

  _intro "oc worktree remove  ${project_id}"
  printf "${DIM}│${RESET}  %-12s %s\n" "Chemin"   "$project_path"
  printf "${DIM}│${RESET}  %-12s %s\n" "Branche"  "$branch"
  echo -e "${DIM}│${RESET}"

  # Confirmation interactive
  local wt_path
  wt_path=$(worktree_get_path "$project_path" "$branch")

  if worktree_exists "$project_path" "$branch"; then
    _prompt _confirm "Supprimer le worktree '${branch}' ? [Y/n] "
    if [[ "${_confirm:-Y}" =~ ^[Nn]$ ]]; then
      log_info "Annulé."
      exit 0
    fi
  fi

  worktree_remove "$project_path" "$branch"

  echo -e "${DIM}│${RESET}"
  _outro "Worktree supprimé."
}

# ── oc worktree cleanup ───────────────────────────────────────────────────────
_cmd_cleanup() {
  local project_id
  project_id=$(_resolve_project "${1:-}")
  local project_path
  project_path=$(resolve_project_path "$project_id")
  local base_branch
  base_branch=$(get_project_worktree_base_branch "$project_id")

  _intro "oc worktree cleanup  ${project_id}"
  printf "${DIM}│${RESET}  %-12s %s\n" "Chemin"  "$project_path"
  printf "${DIM}│${RESET}  %-12s %s\n" "Base"    "$base_branch"
  echo -e "${DIM}│${RESET}"

  worktree_cleanup_merged "$project_path" "$base_branch"

  echo -e "${DIM}│${RESET}"
  _outro "Nettoyage terminé."
}

# ── oc worktree status ────────────────────────────────────────────────────────
_cmd_status() {
  local project_id
  project_id=$(_resolve_project "${1:-}")
  local project_path
  project_path=$(resolve_project_path "$project_id")
  local base_branch
  base_branch=$(get_project_worktree_base_branch "$project_id")
  local worktree_enabled
  worktree_enabled=$(get_project_worktree_enabled "$project_id")
  local auto_cleanup
  auto_cleanup=$(get_project_worktree_auto_cleanup "$project_id")

  _intro "oc worktree status  ${project_id}"
  printf "${DIM}│${RESET}  %-20s %s\n" "Chemin"            "$project_path"
  printf "${DIM}│${RESET}  %-20s %s\n" "Worktree activé"   "$worktree_enabled"
  printf "${DIM}│${RESET}  %-20s %s\n" "Auto-cleanup"      "$auto_cleanup"
  printf "${DIM}│${RESET}  %-20s %s\n" "Base branch"       "$base_branch"
  echo -e "${DIM}│${RESET}"

  worktree_status "$project_path" "$base_branch"

  echo -e "${DIM}│${RESET}"
  _outro "oc worktree list ${project_id}  — détail complet"
}

# ── Dispatch ──────────────────────────────────────────────────────────────────
case "$SUBCOMMAND" in
  list)    _cmd_list    "$@" ;;
  create)  _cmd_create  "$@" ;;
  remove)  _cmd_remove  "$@" ;;
  cleanup) _cmd_cleanup "$@" ;;
  status)  _cmd_status  "$@" ;;
  help|--help|-h|"") _worktree_help ;;
  *)
    log_error "Sous-commande inconnue : $SUBCOMMAND"
    _worktree_help
    exit 1
    ;;
esac
