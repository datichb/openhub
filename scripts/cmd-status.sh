#!/bin/bash
# Affiche le statut de tous les projets enregistrés.
# Usage :
#   ./oc.sh status          → vue détaillée (Beads, API, agents déployés)
#   ./oc.sh status --short  → tableau compact (id, chemin, statut)
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"

# ── Parse flags ──────────────────────────────────────────────────────────────

SHORT_MODE=false
for _arg in "$@"; do
  case "$_arg" in
    --short|-s) SHORT_MODE=true ;;
  esac
done

# ── Helpers d'affichage ───────────────────────────────────────────────────────

_status_ok()   { printf "    ${GREEN}✔${RESET}  %s\n" "$*"; }
_status_warn() { printf "    ${YELLOW}⚠${RESET}  %s\n" "$*"; }
_status_info() { printf "    ${BLUE}·${RESET}  %s\n" "$*"; }

# ── Lecture de tous les PROJECT_ID depuis projects.md ────────────────────────

_list_project_ids() {
  [ -f "$PROJECTS_FILE" ] || return 0
  grep '^## ' "$PROJECTS_FILE" | sed 's/^## //' | grep -v '^$'
}

# ── Vue courte : tableau compact ─────────────────────────────────────────────

_show_short() {
  ensure_projects_file
  resolve_oc_lang

  log_title "$(t list.title)"

  local ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_warn "$(t list.no_projects)"
    exit 0
  fi

  echo ""
  printf "  ${BOLD}%-20s %-30s %-15s${RESET}\n" "$(t list.col_id)" "$(t list.col_path)" "$(t list.col_status)"
  printf "  %s\n" "────────────────────────────────────────────────────────────"

  for id in "${ids[@]}"; do
    local_path=$(get_project_path "$id" 2>/dev/null || true)

    if [ -z "$local_path" ]; then
      status="${YELLOW}$(t list.status_no_path)${RESET}"
      display_path="$(t list.path_undefined)"
      display_color="$YELLOW"
    elif [ -d "${local_path/#\~/$HOME}" ]; then
      status="${GREEN}$(t list.status_ok)${RESET}"
      display_path="$local_path"
      display_color=""
    else
      status="${RED}$(t list.status_missing)${RESET}"
      display_path="$local_path"
      display_color=""
    fi

    if [ -n "$display_color" ]; then
      printf "  %-20s ${display_color}%-30s${RESET} " "$id" "$display_path"
    else
      printf "  %-20s %-30s " "$id" "$display_path"
    fi
    echo -e "$status"
  done

  echo ""
}

# ── Statut d'un projet ───────────────────────────────────────────────────────

_show_project_status() {
  local id="$1"
  echo ""
  echo -e "  ${BOLD}${id}${RESET}"

  # ── Chemin local ──────────────────────────────────────────────────────────
  local path=""
  path=$(get_project_path "$id" 2>/dev/null || true)
  path="${path/#\~/$HOME}"

  if [ -z "$path" ]; then
    _status_warn "$(t status.no_path)"
  elif [ ! -d "$path" ]; then
    _status_warn "$(t status.dir_missing)$path"
    path=""
  else
    _status_info "$(t status.path_label)$path"
  fi

  # ── Beads initialisé ──────────────────────────────────────────────────────
  if [ -n "$path" ] && [ -d "$path/.beads" ]; then
    _status_ok "$(t status.beads_ok)"
  else
    _status_warn "$(t status.beads_not_init)  (./oc.sh beads init $id)"
  fi

  # ── Clé API configurée ────────────────────────────────────────────────────
  if api_keys_entry_exists "$id"; then
    local provider model
    provider=$(get_project_api_provider "$id")
    model=$(get_project_api_model "$id")
    local detail=""
    [ -n "$provider" ] && detail="${provider}"
    [ -n "$model" ]    && detail="${detail:+${detail} / }${model}"
    _status_ok "API configurée${detail:+ (${detail})}"
  else
    _status_warn "$(t status.api_not_set)  (./oc.sh config $id)"
  fi

  # ── Tracker ───────────────────────────────────────────────────────────────
  local tracker
  tracker=$(get_project_tracker "$id")
  case "$tracker" in
    none|"") _status_info "$(t status.tracker_none)" ;;
    *)       _status_ok   "Tracker : $tracker" ;;
  esac

  # ── Agents déployés (cible par défaut) ────────────────────────────────────
  if [ -n "$path" ]; then
    local default_target
    default_target=$(get_default_target)
    local agents_dir=""
    case "$default_target" in
      opencode)    agents_dir="$path/.opencode/agents" ;;
      claude-code) agents_dir="$path/.claude/agents" ;;
    esac

    if [ -n "$agents_dir" ] && [ -d "$agents_dir" ]; then
      local count
      count=$(find "$agents_dir" -name "*.md" -o -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
      _status_ok "$(t status.agents_deployed) (${default_target}) : ${count} fichier(s)"
    else
      _status_warn "$(t status.agents_missing) ${default_target}  (./oc.sh deploy all $id)"
    fi
  fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

if [ "$SHORT_MODE" = true ]; then
  _show_short
  exit 0
fi

ensure_projects_file

log_title "$(t status.title)"

project_ids=$(_list_project_ids)

if [ -z "$project_ids" ]; then
  echo ""
  log_warn "$(t status.no_projects)"
  echo ""
  exit 0
fi

while IFS= read -r pid; do
  _show_project_status "$pid"
done <<< "$project_ids"

echo ""
