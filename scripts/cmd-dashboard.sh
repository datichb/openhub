#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-dashboard.sh — Dashboard multi-projet du hub OpenCode
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc dashboard          → vue synthétique (check rapide quotidien)
#
# Sections (dans l'ordre) :
#   1. Budget sessions     — coût aujourd'hui / semaine / mois + cache hit rate
#   2. Économies IA        — context-mode + RTK (si installés)
#   3. Projets             — tickets par projet (via bd)
#   4. Sessions récentes   — 5 dernières sessions (via SQLite)
#   5. Session active      — orchestrateur en cours (rétrocompat)
#
# Sources de données (lecture seule) :
#   - ~/.local/share/opencode/opencode.db
#   - bd list (par projet)
#   - session-state.json (rétrocompatiblité)
#   - ~/.claude/context-mode/sessions/ (context-mode)
#   - rtk gain (RTK)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"
source "$LIB_DIR/session-state.sh"
source "$LIB_DIR/ai-savings.sh"

# ─────────────────────────────────────────
# TUI HELPERS
# ─────────────────────────────────────────

BOX_WIDTH=62

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

# ─────────────────────────────────────────
# SECTION : BUDGET (SQLite)
# Affiche coût + sessions par période, cache hit rate inline
# ─────────────────────────────────────────

_show_budget_section() {
  _draw_section "💰 Budget"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${DIM}sqlite3 ou base OpenCode non disponible${RESET}"
    echo -e "  ${DIM}Installer sqlite3 pour voir les coûts.${RESET}"
    return
  fi

  local cost_today cost_week cost_month lifetime hit_rate
  local active_today created_today active_week active_month

  # Coûts exacts par steps
  cost_today=$(ocdb_exact_cost 1)
  cost_week=$(ocdb_exact_cost 7)
  cost_month=$(ocdb_exact_cost 30)
  lifetime=$(ocdb_total_cost_all_time)
  hit_rate=$(ocdb_cache_hit_rate 7)

  # Sessions exactes par steps
  ocdb_exact_sessions 1 || true
  active_today="${OCDB_SESSIONS_ACTIVE:-0}"
  created_today="${OCDB_SESSIONS_CREATED:-0}"

  ocdb_exact_sessions 7 || true
  active_week="${OCDB_SESSIONS_ACTIVE:-0}"

  ocdb_exact_sessions 30 || true
  active_month="${OCDB_SESSIONS_ACTIVE:-0}"

  # Couleur du cache hit rate
  local hit_color="${DIM}"
  if awk "BEGIN { exit !($hit_rate >= 80) }" 2>/dev/null; then
    hit_color="${GREEN}"
  elif awk "BEGIN { exit !($hit_rate >= 50) }" 2>/dev/null; then
    hit_color="${CYAN}"
  elif awk "BEGIN { exit !($hit_rate > 0) }" 2>/dev/null; then
    hit_color="${YELLOW}"
  fi

  # Ligne aujourd'hui : sessions avec mention "(dont X créées)" si multi-jours
  local sessions_today_label="${active_today} actives"
  if [ "${active_today}" -gt "${created_today}" ] 2>/dev/null; then
    sessions_today_label="${active_today} actives  ${DIM}(dont ${created_today} créées)${RESET}"
  fi

  printf "  ${DIM}•${RESET}  %-14s  ${GREEN}\$%-10s${RESET}  ${DIM}%s${RESET}" \
    "Aujourd'hui" "$cost_today" "$sessions_today_label"
  if awk "BEGIN { exit !($hit_rate > 0) }" 2>/dev/null; then
    printf "  ${DIM}cache ${RESET}${hit_color}%s%%${RESET}" "$hit_rate"
  fi
  echo ""

  printf "  ${DIM}•${RESET}  %-14s  ${CYAN}\$%-10s${RESET}  ${DIM}%s actives${RESET}\n" \
    "Cette semaine" "$cost_week" "$active_week"
  printf "  ${DIM}•${RESET}  %-14s  ${DIM}\$%-10s${RESET}  ${DIM}%s actives${RESET}\n" \
    "Ce mois" "$cost_month" "$active_month"
  echo ""
  printf "  ${DIM}•${RESET}  %-14s  ${BOLD}${GREEN}\$%s${RESET}\n" \
    "Total lifetime" "$lifetime"
}

# ─────────────────────────────────────────
# SECTION : ÉCONOMIES IA (context-mode + RTK)
# ─────────────────────────────────────────

_show_ai_savings_section() {
  local ctx_ok=0 rtk_ok=0
  aisavings_load_ctx_stats 0 && ctx_ok=1 || true
  aisavings_load_rtk_stats && rtk_ok=1 || true

  [ "$ctx_ok" -eq 0 ] && [ "$rtk_ok" -eq 0 ] && return

  _draw_section "🔋 Économies IA  ${DIM}(lifetime)${RESET}"
  echo ""

  if [ "$ctx_ok" -eq 1 ]; then
    local fmt_tokens
    fmt_tokens=$(aisavings_format_tokens "$CTX_TOKENS_SAVED")
    printf "  ${DIM}•${RESET}  %-14s  ${CYAN}%s tokens${RESET}  ${DIM}·${RESET}  ${GREEN}\$%s${RESET}  ${DIM}·${RESET}  réduction ${CYAN}%s%%${RESET}\n" \
      "context-mode" "$fmt_tokens" "$CTX_DOLLARS_SAVED" "$CTX_REDUCTION_PCT"
  fi

  if [ "$rtk_ok" -eq 1 ]; then
    local fmt_rtk
    fmt_rtk=$(aisavings_format_tokens "$RTK_TOTAL_SAVED")
    printf "  ${DIM}•${RESET}  %-14s  ${CYAN}%s tokens${RESET}  ${DIM}·${RESET}  ${GREEN}%s%%${RESET}  ${DIM}·${RESET}  %s cmds\n" \
      "RTK" "$fmt_rtk" "$RTK_AVG_SAVINGS_PCT" "$RTK_TOTAL_COMMANDS"
  fi
}

# ─────────────────────────────────────────
# SECTION : PROJETS (bd)
# ─────────────────────────────────────────

_show_projects_section() {
  _draw_section "📁 Projets"

  if ! command -v bd &>/dev/null; then
    echo -e "  ${DIM}bd non disponible — installer Beads pour voir les tickets${RESET}"
    return
  fi

  local projects_file="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
  local paths_file="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"

  if [ ! -f "$projects_file" ] || [ ! -f "$paths_file" ]; then
    echo -e "  ${DIM}Registre de projets introuvable${RESET}"
    return
  fi

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

    [ "$proj_path" = "." ] && proj_path="$HUB_DIR"
    [ -z "$proj_path" ] && continue
    [ ! -d "$proj_path" ] && continue
    [ ! -d "$proj_path/.beads" ] && continue

    local done_count=0 inprogress_count=0 todo_count=0 blocked_count=0
    local current_ticket_title=""

    local bd_output
    if bd_output=$(bd -C "$proj_path" list --json --no-tree 2>/dev/null); then
      if command -v jq &>/dev/null && echo "$bd_output" | jq empty 2>/dev/null; then
        done_count=$(echo "$bd_output" | jq '[.[] | select(.status == "done")] | length' 2>/dev/null || echo "0")
        inprogress_count=$(echo "$bd_output" | jq '[.[] | select(.status == "in_progress")] | length' 2>/dev/null || echo "0")
        todo_count=$(echo "$bd_output" | jq '[.[] | select(.status == "todo" or .status == "pending")] | length' 2>/dev/null || echo "0")
        blocked_count=$(echo "$bd_output" | jq '[.[] | select(.status == "blocked")] | length' 2>/dev/null || echo "0")
        current_ticket_title=$(echo "$bd_output" | jq -r '[.[] | select(.status == "in_progress")] | first | .title // ""' 2>/dev/null || echo "")
      fi
    fi

    any_shown=true
    echo ""

    # Ligne principale : [ID]  ✅ N  🔄 N  ⏳ N  [🚫 N]
    printf "  ${BOLD}${CYAN}[%-12s${RESET}${BOLD}${CYAN}]${RESET}  " "$proj_id"
    printf "${GREEN}✅ %-3s${RESET}" "$done_count"
    printf "  ${CYAN}🔄 %-3s${RESET}" "$inprogress_count"
    printf "  ${DIM}⏳ %-3s${RESET}" "$todo_count"
    [ "$blocked_count" -gt 0 ] && printf "  ${YELLOW}🚫 %-3s${RESET}" "$blocked_count"
    echo ""

    # Ticket en cours (si existe)
    if [ -n "$current_ticket_title" ]; then
      [ ${#current_ticket_title} -gt 44 ] && current_ticket_title="${current_ticket_title:0:42}…"
      echo -e "  ${DIM}  └─ En cours${RESET} : ${current_ticket_title}"
    fi
  done

  if [ "$any_shown" = "false" ]; then
    echo -e "  ${DIM}Aucun projet avec Beads initialisé${RESET}"
    echo -e "  ${DIM}Utiliser : oc beads init <PROJECT_ID>${RESET}"
  fi
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
    local title agent cost ts_ms slug
    slug=$(echo "$entry"  | cut -d'|' -f1)
    title=$(echo "$entry" | cut -d'|' -f2)
    agent=$(echo "$entry" | cut -d'|' -f3)
    cost=$(echo "$entry"  | cut -d'|' -f4)
    ts_ms=$(echo "$entry" | cut -d'|' -f5)

    local date_str
    date_str=$(ocdb_format_date "$ts_ms")

    [ ${#title} -gt 34 ] && title="${title:0:32}…"

    printf "  ${DIM}•${RESET}  %-34s  ${DIM}%-20s${RESET}  ${GREEN}\$%s${RESET}  ${DIM}%s  %s${RESET}\n" \
      "$title" "${agent:-—}" "$cost" "$date_str" "$slug"
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

  _draw_section "⚡ Session active"
  echo ""

  [ -n "$current_agent" ] && echo -e "  ${DIM}Agent${RESET}   ${GREEN}${current_agent}${RESET}"
  [ -n "$current_id" ]    && echo -e "  ${DIM}Ticket${RESET}  ${CYAN}${current_id}${RESET}"

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
    echo -e "  ${DIM}Action${RESET}  ${CYAN}${action_label}${RESET}"
  fi

  if [ -n "$started_at" ]; then
    local started_time="--:--"
    [[ "$started_at" =~ T([0-9]{2}):([0-9]{2}) ]] && started_time="${BASH_REMATCH[1]}:${BASH_REMATCH[2]} UTC"
    echo -e "  ${DIM}Depuis ${started_time} — Mode: ${mode}${RESET}"
  fi
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  echo ""
  _draw_header "OpenCode Hub — Dashboard"
  echo ""

  # 1. Budget (coût + cache hit rate inline)
  _show_budget_section

  # 2. Économies IA (si plugins installés)
  _show_ai_savings_section

  # 3. Session orchestrateur active (rétrocompat — affiché seulement si actif)
  local state_json=""
  if command -v jq &>/dev/null; then
    state_json=$(session_state_read 2>/dev/null || echo "")
    if [ -n "$state_json" ] && ! echo "$state_json" | jq empty 2>/dev/null; then
      state_json=""
    fi
  fi
  if [ -n "$state_json" ]; then
    local has_current
    has_current=$(echo "$state_json" | jq -r '.current_ticket.id // ""' 2>/dev/null || echo "")
    [ -n "$has_current" ] && _show_active_session_compat "$state_json"
  fi

  # 4. Projets (bd)
  _show_projects_section

  # 5. Sessions récentes
  _show_recent_sessions_section

  echo ""
  _draw_separator
  echo ""
  echo -e "  ${DIM}Détails : oc metrics [--period today|week|month]${RESET}"
  echo ""
}

main "$@"
