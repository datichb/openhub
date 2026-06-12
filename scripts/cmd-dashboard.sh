#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-dashboard.sh — Dashboard multi-projet du hub OpenCode
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc dashboard          → dashboard multi-projet (tous les projets)
#
# Affiche :
# - Vue multi-projet : tickets actifs, bloqués, complétés (via bd)
# - Budget sessions : aujourd'hui / semaine / mois (via SQLite OpenCode)
# - Sessions récentes (via SQLite OpenCode)
# - Top agents actifs (via SQLite OpenCode)
#
# Sources de données (lecture seule — aucune écriture requise) :
#   - ~/.local/share/opencode/opencode.db
#   - bd list (par projet)
#   - session-state.json (rétrocompatiblité)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"
source "$LIB_DIR/session-state.sh"

# ─────────────────────────────────────────
# TUI HELPERS
# ─────────────────────────────────────────

BOX_WIDTH=56

_draw_line() {
  local char="${1:-─}"
  local width="${2:-$BOX_WIDTH}"
  printf '%*s' "$width" '' | tr ' ' "$char"
}

_draw_header() {
  local title="$1"
  local title_len=${#title}
  local inner=$(( BOX_WIDTH - 2 ))
  local padding=$(( (inner - title_len) / 2 ))
  local padding_right=$(( inner - title_len - padding ))

  echo -e "${CYAN}╭$(_draw_line "─" $inner)╮${RESET}"
  printf "${CYAN}│${RESET}%*s${BOLD}%s${RESET}%*s${CYAN}│${RESET}\n" \
    "$padding" "" "$title" "$padding_right" ""
  echo -e "${CYAN}╰$(_draw_line "─" $inner)╯${RESET}"
}

_draw_separator() {
  echo -e "${DIM}$(_draw_line "─" $BOX_WIDTH)${RESET}"
}

_draw_section() {
  echo ""
  echo -e "${BOLD}${BLUE}$1${RESET}"
}

_draw_item() {
  printf "  ${DIM}•${RESET}  %-28s " "$1"
  echo -e "$2"
}

# ─────────────────────────────────────────
# SECTION : PROJETS (bd)
# ─────────────────────────────────────────

_show_projects_section() {
  _draw_section "📁 Projets"

  if ! command -v bd &>/dev/null; then
    echo -e "  ${DIM}bd non disponible — installer Beads pour voir les tickets${RESET}"
    echo -e "  ${DIM}brew install beads  ou  https://beads.sh${RESET}"
    return
  fi

  local projects_file="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
  local paths_file="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"

  if [ ! -f "$projects_file" ] || [ ! -f "$paths_file" ]; then
    echo -e "  ${DIM}Registre de projets introuvable${RESET}"
    return
  fi

  # Extraire les IDs de projet
  local project_ids=()
  while IFS= read -r line; do
    if [[ "$line" =~ ^##[[:space:]]+([A-Z0-9_-]+)$ ]]; then
      project_ids+=("${BASH_REMATCH[1]}")
    fi
  done < "$projects_file"

  if [ ${#project_ids[@]} -eq 0 ]; then
    echo -e "  ${DIM}Aucun projet configuré${RESET}"
    return
  fi

  local any_shown=false

  for proj_id in "${project_ids[@]}"; do
    # Résoudre le chemin
    local proj_path=""
    while IFS='=' read -r key val; do
      [[ "$key" =~ ^[[:space:]]*# ]] && continue
      [[ -z "$key" ]] && continue
      local k="${key// /}"
      if [ "$k" = "$proj_id" ]; then
        proj_path="$val"
        break
      fi
    done < "$paths_file"

    if [ "$proj_path" = "." ]; then
      proj_path="$HUB_DIR"
    fi

    [ -z "$proj_path" ] && continue
    [ ! -d "$proj_path" ] && continue
    [ ! -d "$proj_path/.beads" ] && continue

    # Compter tickets par statut
    local done_count=0 inprogress_count=0 todo_count=0 blocked_count=0 total_count=0
    local current_ticket_title="" current_ticket_agent=""

    local bd_output
    if bd_output=$(cd "$proj_path" && bd list --format json 2>/dev/null); then
      if command -v jq &>/dev/null && echo "$bd_output" | jq empty 2>/dev/null; then
        done_count=$(echo "$bd_output" | jq '[.[] | select(.status == "done")] | length' 2>/dev/null || echo "0")
        inprogress_count=$(echo "$bd_output" | jq '[.[] | select(.status == "in_progress")] | length' 2>/dev/null || echo "0")
        todo_count=$(echo "$bd_output" | jq '[.[] | select(.status == "todo" or .status == "pending")] | length' 2>/dev/null || echo "0")
        blocked_count=$(echo "$bd_output" | jq '[.[] | select(.status == "blocked")] | length' 2>/dev/null || echo "0")
        total_count=$(echo "$bd_output" | jq 'length' 2>/dev/null || echo "0")
        # Ticket en cours
        current_ticket_title=$(echo "$bd_output" | jq -r '[.[] | select(.status == "in_progress")] | first | .title // ""' 2>/dev/null || echo "")
      fi
    fi

    any_shown=true
    echo ""
    echo -e "  ${BOLD}${CYAN}[${proj_id}]${RESET}"

    if [ -n "$current_ticket_title" ]; then
      # Tronquer à 38 chars
      if [ ${#current_ticket_title} -gt 38 ]; then
        current_ticket_title="${current_ticket_title:0:36}…"
      fi
      echo -e "  ${DIM}En cours${RESET} : ${current_ticket_title}"
    fi

    printf "  Tickets   : "
    printf "${GREEN}✅ %-4s${RESET}" "$done_count"
    printf "${CYAN}🔄 %-4s${RESET}" "$inprogress_count"
    printf "${DIM}⏳ %-4s${RESET}" "$todo_count"
    [ "$blocked_count" -gt 0 ] && printf "${YELLOW}🚫 %-4s${RESET}" "$blocked_count"
    echo ""
  done

  if [ "$any_shown" = "false" ]; then
    echo -e "  ${DIM}Aucun projet avec Beads initialisé${RESET}"
    echo -e "  ${DIM}Utiliser : oc beads init <PROJECT_ID>${RESET}"
  fi
}

# ─────────────────────────────────────────
# SECTION : BUDGET SESSIONS (SQLite)
# ─────────────────────────────────────────

_show_budget_section() {
  _draw_section "💰 Budget sessions"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${DIM}sqlite3 ou base OpenCode non disponible${RESET}"
    echo -e "  ${DIM}Installer sqlite3 pour voir les coûts.${RESET}"
    return
  fi

  local cost_today cost_week cost_month
  local sessions_today sessions_week sessions_month

  cost_today=$(ocdb_total_cost 1)
  sessions_today=$(ocdb_sessions_count 1)
  cost_week=$(ocdb_total_cost 7)
  sessions_week=$(ocdb_sessions_count 7)
  cost_month=$(ocdb_total_cost 30)
  sessions_month=$(ocdb_sessions_count 30)

  # Cache hit rate semaine
  local hit_rate
  hit_rate=$(ocdb_cache_hit_rate 7)

  printf "  ${DIM}•${RESET}  %-14s  ${GREEN}\$%-10s${RESET}  ${DIM}%s sessions${RESET}\n" \
    "Aujourd'hui" "$cost_today" "$sessions_today"
  printf "  ${DIM}•${RESET}  %-14s  ${CYAN}\$%-10s${RESET}  ${DIM}%s sessions${RESET}\n" \
    "Cette semaine" "$cost_week" "$sessions_week"
  printf "  ${DIM}•${RESET}  %-14s  ${DIM}\$%-10s${RESET}  ${DIM}%s sessions${RESET}\n" \
    "Ce mois" "$cost_month" "$sessions_month"
  echo ""

  # Cache hit rate
  local hit_color="${DIM}"
  if awk "BEGIN { exit !($hit_rate >= 80) }" 2>/dev/null; then
    hit_color="${GREEN}"
  elif awk "BEGIN { exit !($hit_rate >= 50) }" 2>/dev/null; then
    hit_color="${CYAN}"
  elif awk "BEGIN { exit !($hit_rate > 0) }" 2>/dev/null; then
    hit_color="${YELLOW}"
  fi
  printf "  ${DIM}•${RESET}  %-14s  ${hit_color}%s%%${RESET}  ${DIM}(7 derniers jours)${RESET}\n" \
    "Cache hit rate" "$hit_rate"
}

# ─────────────────────────────────────────
# SECTION : SESSIONS RÉCENTES (SQLite)
# ─────────────────────────────────────────

_show_recent_sessions_section() {
  _draw_section "🕐 Sessions récentes"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${DIM}sqlite3 non disponible${RESET}"
    return
  fi

  local sessions=()
  while IFS= read -r line; do
    [ -n "$line" ] && sessions+=("$line")
  done < <(ocdb_recent_sessions 5 7)

  if [ ${#sessions[@]} -eq 0 ]; then
    echo -e "  ${DIM}Aucune session ces 7 derniers jours${RESET}"
    return
  fi

  for entry in "${sessions[@]}"; do
    local slug title agent cost ts_ms
    slug=$(echo "$entry" | cut -d'|' -f1)
    title=$(echo "$entry" | cut -d'|' -f2)
    agent=$(echo "$entry" | cut -d'|' -f3)
    cost=$(echo "$entry" | cut -d'|' -f4)
    ts_ms=$(echo "$entry" | cut -d'|' -f5)

    local date_str
    date_str=$(ocdb_format_date "$ts_ms")

    # Tronquer le titre
    if [ ${#title} -gt 32 ]; then
      title="${title:0:30}…"
    fi

    printf "  ${DIM}•${RESET}  %-32s  %-18s  ${GREEN}\$%-7s${RESET}  ${DIM}%s${RESET}\n" \
      "$title" "${agent:-—}" "$cost" "$date_str"
  done
}

# ─────────────────────────────────────────
# SECTION : TOP AGENTS (SQLite)
# ─────────────────────────────────────────

_show_agents_section() {
  _draw_section "🤖 Top agents (7j)"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${DIM}sqlite3 non disponible${RESET}"
    return
  fi

  local agents=()
  while IFS= read -r line; do
    [ -n "$line" ] && agents+=("$line")
  done < <(ocdb_cost_by_agent 7 5)

  if [ ${#agents[@]} -eq 0 ]; then
    echo -e "  ${DIM}Aucun agent ces 7 derniers jours${RESET}"
    return
  fi

  # Trouver le max pour pourcentage
  local max_cost=0
  for entry in "${agents[@]}"; do
    local cost="${entry##*|}"
    if awk "BEGIN { exit !($cost > $max_cost) }" 2>/dev/null; then
      max_cost="$cost"
    fi
  done

  for entry in "${agents[@]}"; do
    local agent="${entry%|*}"
    local cost="${entry##*|}"
    [ -z "$agent" ] || [ "$agent" = "unknown" ] && continue

    local pct="0"
    if awk "BEGIN { exit !($max_cost > 0) }" 2>/dev/null; then
      pct=$(awk "BEGIN { printf \"%d\", ($cost/$max_cost)*100 }")
    fi

    printf "  ${DIM}•${RESET}  %-22s  ${CYAN}\$%-8s${RESET}  ${DIM}%3s%%${RESET}\n" \
      "$agent" "$cost" "$pct"
  done
}

# ─────────────────────────────────────────
# SECTION : SESSION ACTIVE (rétrocompat)
# ─────────────────────────────────────────

_show_active_session_compat() {
  local state_json="$1"

  local current_id current_agent current_action mode started_at
  current_id=$(echo "$state_json" | jq -r '.current_ticket.id // ""' 2>/dev/null || echo "")
  current_agent=$(echo "$state_json" | jq -r '.current_ticket.agent // ""' 2>/dev/null || echo "")
  current_action=$(echo "$state_json" | jq -r '.current_ticket.action // ""' 2>/dev/null || echo "")
  mode=$(echo "$state_json" | jq -r '.mode // ""' 2>/dev/null || echo "")
  started_at=$(echo "$state_json" | jq -r '.started_at // ""' 2>/dev/null || echo "")

  [ -z "$current_agent" ] && [ -z "$current_id" ] && return

  _draw_section "⚡ Session orchestrateur active"
  echo ""

  if [ -n "$current_agent" ]; then
    echo -e "  ${BOLD}🤖 Agent actif${RESET} : ${GREEN}${current_agent}${RESET}"
  fi
  if [ -n "$current_id" ]; then
    echo -e "  ${BOLD}🎫 Ticket${RESET}     : ${CYAN}${current_id}${RESET}"
  fi
  if [ -n "$current_action" ]; then
    local action_label
    case "$current_action" in
      implementing) action_label="Implémentation" ;;
      testing)      action_label="Tests" ;;
      reviewing)    action_label="Review" ;;
      waiting_cp2)  action_label="Attente CP-2" ;;
      idle)         action_label="En attente" ;;
      *)            action_label="$current_action" ;;
    esac
    echo -e "  ${BOLD}🎬 Action${RESET}     : ${CYAN}${action_label}${RESET}"
  fi
  if [ -n "$started_at" ]; then
    local started_time="--:--"
    if [[ "$started_at" =~ T([0-9]{2}):([0-9]{2}) ]]; then
      started_time="${BASH_REMATCH[1]}:${BASH_REMATCH[2]} UTC"
    fi
    echo -e "  ${DIM}⏱️  Démarrée à ${started_time} — Mode: ${mode}${RESET}"
  fi
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  echo ""
  _draw_header "OpenCode Hub — Dashboard"
  echo ""

  # ── Section projets (bd) ─────────────────
  _show_projects_section

  # ── Session orchestrateur active (rétrocompat) ──
  local state_json=""
  if command -v jq &>/dev/null; then
    state_json=$(session_state_read 2>/dev/null || echo "")
    if [ -n "$state_json" ] && ! echo "$state_json" | jq empty 2>/dev/null; then
      state_json=""
    fi
  fi
  if [ -n "$state_json" ]; then
    # Vérifier qu'il y a bien un ticket actif (session non vide)
    local has_current
    has_current=$(echo "$state_json" | jq -r '.current_ticket.id // ""' 2>/dev/null || echo "")
    if [ -n "$has_current" ]; then
      _show_active_session_compat "$state_json"
    fi
  fi

  echo ""
  _draw_separator

  # ── Budget sessions (SQLite) ─────────────
  _show_budget_section

  echo ""
  _draw_separator

  # ── Sessions récentes (SQLite) ───────────
  _show_recent_sessions_section

  echo ""
  _draw_separator

  # ── Top agents (SQLite) ──────────────────
  _show_agents_section

  echo ""
  _draw_separator
  echo ""
  echo -e "  ${DIM}Détails : oc metrics [--period today|week|month]${RESET}"
  echo ""
}

main "$@"
