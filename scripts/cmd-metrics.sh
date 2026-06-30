#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-metrics.sh — Métriques de vélocité, coûts et usage du hub OpenCode
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc metrics                  → 7 derniers jours (défaut)
#   oc metrics --period today   → aujourd'hui seulement
#   oc metrics --period week    → 7 derniers jours
#   oc metrics --period month   → 30 derniers jours
#
# Sections (dans l'ordre) :
#   1. Vue globale    — sessions, coût, tokens, cache hit rate + économies plugins
#   2. Coût           — par projet, par agent, par modèle (fusionné)
#   3. Activité       — tool-use patterns par catégorie
#   4. Sessions récentes
#   5. Tickets        — par projet (bd)
#   6. Vélocité workflow (si metrics.jsonl)
#
# Sources de données :
#   - ~/.local/share/opencode/opencode.db (sessions, tokens, coûts)
#   - bd list (tickets par projet)
#   - .opencode/metrics.jsonl (vélocité workflow — rétrocompat.)
#   - ~/.claude/context-mode/sessions/ (context-mode)
#   - rtk gain (RTK)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"
source "$LIB_DIR/metrics.sh"
source "$LIB_DIR/ai-savings.sh"

# ─────────────────────────────────────────
# PARSE ARGS
# ─────────────────────────────────────────

_PERIOD="week"
_PERIOD_DAYS=7
_PERIOD_LABEL="7 derniers jours"

_parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --period|-d)
        shift
        case "${1:-}" in
          today)
            _PERIOD="today"; _PERIOD_DAYS=1; _PERIOD_LABEL="Aujourd'hui"
            ;;
          week)
            _PERIOD="week";  _PERIOD_DAYS=7; _PERIOD_LABEL="7 derniers jours"
            ;;
          month)
            _PERIOD="month"; _PERIOD_DAYS=30; _PERIOD_LABEL="30 derniers jours"
            ;;
          *)
            echo "Période inconnue : '${1:-}'. Options : today, week, month" >&2
            exit 1
            ;;
        esac
        ;;
      --period=*)
        local val="${1#--period=}"
        set -- "--period" "$val" "${@:2}"
        continue
        ;;
    esac
    shift
  done
}

# ─────────────────────────────────────────
# DISPLAY HELPERS
# ─────────────────────────────────────────

_metrics_header() {
  echo ""
  echo -e "${BOLD}${CYAN}📊 Métriques OpenCode Hub${RESET}  ${DIM}·  ${_PERIOD_LABEL}${RESET}"
  echo -e "${DIM}──────────────────────────────────────────────${RESET}"
  echo ""
}

_metrics_section() {
  echo ""
  echo -e "${BOLD}${BLUE}$1${RESET}"
}

_metrics_item() {
  printf "  ${DIM}•${RESET}  %-28s " "$1"
  echo -e "$2"
}

_metrics_subsection() {
  echo ""
  echo -e "  ${CYAN}$1${RESET}"
}

_metrics_subitem() {
  printf "    ${DIM}·${RESET}  %-26s " "$1"
  echo -e "$2"
}

# Barre de progression ASCII proportionnelle à un maximum
_metrics_bar() {
  local val="${1:-0}" max="${2:-1}" width="${3:-20}"
  [ "$max" -eq 0 ] 2>/dev/null && max=1
  local filled
  filled=$(awk "BEGIN { printf \"%d\", ($val / $max) * $width }")
  local empty=$(( width - filled ))
  printf "["
  printf '%0.s█' $(seq 1 "$filled" 2>/dev/null) 2>/dev/null || true
  printf '%0.s░' $(seq 1 "$empty" 2>/dev/null) 2>/dev/null || true
  printf "]"
}

# ─────────────────────────────────────────
# SECTION : VUE GLOBALE (SQLite + plugins)
# ─────────────────────────────────────────

_show_global_section() {
  _metrics_section "📈 Vue globale"
  echo ""

  # ── Sessions : actives (steps) + créées ──
  ocdb_exact_sessions "$_PERIOD_DAYS" || true
  local _active="${OCDB_SESSIONS_ACTIVE:-0}"
  local _created="${OCDB_SESSIONS_CREATED:-0}"
  local _sessions_label
  if [ "$_active" -gt "$_created" ]; then
    _sessions_label="${GREEN}${_active} actives${RESET}  ${DIM}(dont ${_created} créées)${RESET}"
  else
    _sessions_label="${GREEN}${_active}${RESET}"
  fi

  # ── Coût exact (steps dans la période) ──
  local _exact_cost
  _exact_cost=$(ocdb_exact_cost "$_PERIOD_DAYS")

  _metrics_item "Sessions"   "$_sessions_label"
  _metrics_item "Coût total" "${GREEN}\$${_exact_cost}${RESET}  ${DIM}(steps dans la période)${RESET}"
  echo ""

  # ── Tokens ──
  local fmt_input fmt_output fmt_cache_read fmt_cache_write
  fmt_input=$(ocdb_format_tokens "$OCDB_TOKENS_INPUT")
  fmt_output=$(ocdb_format_tokens "$OCDB_TOKENS_OUTPUT")
  fmt_cache_read=$(ocdb_format_tokens "$OCDB_TOKENS_CACHE_READ")
  fmt_cache_write=$(ocdb_format_tokens "$OCDB_TOKENS_CACHE_WRITE")

  _metrics_item "Tokens input"       "${CYAN}${fmt_input}${RESET}"
  _metrics_item "Tokens output"      "${CYAN}${fmt_output}${RESET}"
  _metrics_item "Cache write / read" "${DIM}${fmt_cache_write}${RESET}  ${DIM}/  ${fmt_cache_read}${RESET}"
  echo ""

  # ── Cache hit rate ──
  local hit_color="${DIM}"
  local hit_rate="${OCDB_CACHE_HIT_RATE:-0.0}"
  if awk "BEGIN { exit !($hit_rate >= 80) }" 2>/dev/null; then
    hit_color="${GREEN}"
  elif awk "BEGIN { exit !($hit_rate >= 50) }" 2>/dev/null; then
    hit_color="${CYAN}"
  elif awk "BEGIN { exit !($hit_rate > 0) }" 2>/dev/null; then
    hit_color="${YELLOW}"
  fi
  _metrics_item "Cache hit rate" "${hit_color}${hit_rate}%${RESET}  ${DIM}(économies estimées : \$${OCDB_CACHE_SAVINGS})${RESET}"

  # ── Économies plugins ──
  local _ctx_ok=0 _rtk_ok=0
  aisavings_load_ctx_stats "$_PERIOD_DAYS" && _ctx_ok=1 || true
  aisavings_load_rtk_stats && _rtk_ok=1 || true

  if [ "$_ctx_ok" -eq 1 ] || [ "$_rtk_ok" -eq 1 ]; then
    _metrics_subsection "Économies plugins"

    if [ "$_ctx_ok" -eq 1 ]; then
      local _ctx_fmt
      _ctx_fmt=$(aisavings_format_tokens "$CTX_TOKENS_SAVED")
      _metrics_subitem "context-mode" \
        "${DIM}${CTX_PERIOD_LABEL}${RESET}  ${CYAN}${_ctx_fmt} tokens${RESET}  ${DIM}·${RESET}  ${GREEN}\$${CTX_DOLLARS_SAVED}${RESET}  ${DIM}· -${CTX_REDUCTION_PCT}%${RESET}"
    fi

    if [ "$_rtk_ok" -eq 1 ]; then
      local _rtk_fmt
      _rtk_fmt=$(aisavings_format_tokens "$RTK_TOTAL_SAVED")
      _metrics_subitem "RTK" \
        "${DIM}(global)${RESET}  ${CYAN}${_rtk_fmt} tokens${RESET}  ${DIM}·${RESET}  ${GREEN}${RTK_AVG_SAVINGS_PCT}%${RESET}  ${DIM}· ${RTK_TOTAL_COMMANDS} cmds${RESET}"
    fi
  fi
}

# ─────────────────────────────────────────
# SECTION : COÛT TOTAL (lifetime + breakdown par période)
# Toujours affiché avec les 3 fenêtres fixes (today/week/month)
# La ligne correspondant au --period actif est mise en évidence
# ─────────────────────────────────────────

_show_total_cost_section() {
  # Vérifier qu'il y a des données (au moins un step)
  local _has_steps
  _has_steps=$(_ocdb_query "
    SELECT COUNT(*) FROM part
    WHERE json_extract(data,'$.type') = 'step-finish'
    LIMIT 1;
  " 2>/dev/null || echo "0")
  [ "${_has_steps:-0}" -eq 0 ] && return

  _metrics_section "💳 Coût total"
  echo ""

  # Lifetime (depuis session.cost — agrégat complet)
  local _lifetime
  _lifetime=$(ocdb_total_cost_all_time)
  _metrics_item "Lifetime" "${BOLD}${GREEN}\$${_lifetime}${RESET}"
  echo ""
  echo -e "  ${DIM}──────────────────────────────────────────${RESET}"

  # Les 3 périodes fixes — toujours affichées indépendamment du --period
  local _cost_today _cost_week _cost_month
  _cost_today=$(ocdb_exact_cost 1)
  _cost_week=$(ocdb_exact_cost 7)
  _cost_month=$(ocdb_exact_cost 30)

  # Couleur selon si la ligne correspond au --period actif
  local _col_today="${CYAN}" _col_week="${CYAN}" _col_month="${CYAN}"
  local _mark_today="" _mark_week="" _mark_month=""
  case "$_PERIOD" in
    today) _col_today="${GREEN}";  _mark_today="  ${DIM}← période active${RESET}" ;;
    week)  _col_week="${GREEN}";   _mark_week="  ${DIM}← période active${RESET}" ;;
    month) _col_month="${GREEN}";  _mark_month="  ${DIM}← période active${RESET}" ;;
  esac

  printf "  ${DIM}•${RESET}  %-18s  ${_col_today}\$%-10s${RESET}%s\n" \
    "Aujourd'hui  (steps)" "$_cost_today" "$_mark_today"
  printf "  ${DIM}•${RESET}  %-18s  ${_col_week}\$%-10s${RESET}%s\n" \
    "7 jours      (steps)" "$_cost_week" "$_mark_week"
  printf "  ${DIM}•${RESET}  %-18s  ${_col_month}\$%-10s${RESET}%s\n" \
    "30 jours     (steps)" "$_cost_month" "$_mark_month"
}

# ─────────────────────────────────────────
# SECTION : COÛT (par projet / agent / modèle)
# Fusionné en une section avec sous-groupes
# ─────────────────────────────────────────

_show_cost_section() {
  local has_data=false
  [ "${#OCDB_TOP_PROJECTS[@]}" -gt 0 ] && has_data=true
  [ "${#OCDB_TOP_AGENTS[@]}" -gt 0 ] && has_data=true
  [ "${#OCDB_TOP_MODELS[@]}" -gt 0 ] && has_data=true
  [ "$has_data" = "false" ] && return

  _metrics_section "💰 Coût"

  # ── Par projet ──
  if [ "${#OCDB_TOP_PROJECTS[@]}" -gt 0 ]; then
    _metrics_subsection "Par projet"
    local max_cost=0
    for entry in "${OCDB_TOP_PROJECTS[@]}"; do
      local cost="${entry##*|}"
      awk "BEGIN { exit !($cost > $max_cost) }" 2>/dev/null && max_cost="$cost" || true
    done
    for entry in "${OCDB_TOP_PROJECTS[@]}"; do
      local dir="${entry%|*}" cost="${entry##*|}"
      local short_dir
      short_dir=$(basename "$dir" 2>/dev/null || echo "$dir")
      local pct=0
      awk "BEGIN { exit !($max_cost > 0) }" 2>/dev/null && \
        pct=$(awk "BEGIN { printf \"%d\", ($cost/$max_cost)*100 }") || true
      _metrics_subitem "$short_dir" "${GREEN}\$${cost}${RESET}  ${DIM}${pct}%${RESET}"
    done
  fi

  # ── Par agent ──
  if [ "${#OCDB_TOP_AGENTS[@]}" -gt 0 ]; then
    _metrics_subsection "Par agent"
    local max_agent_cost=0
    for entry in "${OCDB_TOP_AGENTS[@]}"; do
      local cost="${entry##*|}"
      awk "BEGIN { exit !($cost > $max_agent_cost) }" 2>/dev/null && max_agent_cost="$cost" || true
    done
    for entry in "${OCDB_TOP_AGENTS[@]}"; do
      local agent="${entry%|*}" cost="${entry##*|}"
      [ -z "$agent" ] || [ "$agent" = "unknown" ] && continue
      local pct=0
      awk "BEGIN { exit !($max_agent_cost > 0) }" 2>/dev/null && \
        pct=$(awk "BEGIN { printf \"%d\", ($cost/$max_agent_cost)*100 }") || true
      _metrics_subitem "$agent" "${CYAN}\$${cost}${RESET}  ${DIM}${pct}%${RESET}"
    done
  fi

  # ── Par modèle ──
  if [ "${#OCDB_TOP_MODELS[@]}" -gt 0 ]; then
    _metrics_subsection "Par modèle"
    for entry in "${OCDB_TOP_MODELS[@]}"; do
      local model="${entry%|*}" cost="${entry##*|}"
      [ -z "$model" ] || [ "$model" = "unknown" ] && continue
      _metrics_subitem "$model" "${DIM}\$${cost}${RESET}"
    done
  fi
}

# ─────────────────────────────────────────
# SECTION : ACTIVITÉ (tool-use patterns)
# ─────────────────────────────────────────

_show_activity_section() {
  local days="$1"
  local rows=()
  while IFS= read -r line; do
    [ -n "$line" ] && rows+=("$line")
  done < <(ocdb_activity_breakdown "$days" 2>/dev/null || true)
  [ ${#rows[@]} -eq 0 ] && return

  _metrics_section "🎯 Activité"
  echo ""

  local total_cost=0
  for row in "${rows[@]}"; do
    local cost; cost=$(echo "$row" | cut -d'|' -f3)
    total_cost=$(LC_ALL=C awk "BEGIN { printf \"%.4f\", $total_cost + ${cost:-0} }")
  done

  for row in "${rows[@]}"; do
    local category count cost
    category=$(echo "$row" | cut -d'|' -f1)
    count=$(echo "$row"    | cut -d'|' -f2)
    cost=$(echo "$row"     | cut -d'|' -f3)

    local label emoji color
    case "$category" in
      code)          label="Code";          emoji="💻"; color="${GREEN}" ;;
      planification) label="Planification"; emoji="🗺️"; color="${CYAN}" ;;
      exploration)   label="Exploration";   emoji="🔍"; color="${CYAN}" ;;
      review)        label="Review";        emoji="👁️"; color="${YELLOW}" ;;
      debug)         label="Debug";         emoji="🐛"; color="${YELLOW}" ;;
      conversation)  label="Conversation";  emoji="💬"; color="${DIM}" ;;
      *)             label="$category";     emoji="•";  color="${DIM}" ;;
    esac

    local pct=0
    awk "BEGIN { exit !($total_cost > 0) }" 2>/dev/null && \
      pct=$(LC_ALL=C awk "BEGIN { printf \"%d\", $cost/$total_cost*100 }") || true

    printf "  ${DIM}•${RESET}  %s %-16s  ${color}%-3s sessions${RESET}  ${DIM}\$%-10s (%s%%)${RESET}\n" \
      "$emoji" "$label" "$count" "$cost" "$pct"
  done
}

# ─────────────────────────────────────────
# SECTION : SESSIONS RÉCENTES
# ─────────────────────────────────────────

_show_recent_sessions_section() {
  [ "${#OCDB_RECENT_SESSIONS[@]}" -eq 0 ] && return

  _metrics_section "🕐 Sessions récentes"
  echo ""

  for entry in "${OCDB_RECENT_SESSIONS[@]}"; do
    local title agent cost ts_ms
    title=$(echo "$entry" | cut -d'|' -f2)
    agent=$(echo "$entry" | cut -d'|' -f3)
    cost=$(echo "$entry"  | cut -d'|' -f4)
    ts_ms=$(echo "$entry" | cut -d'|' -f5)

    local date_str
    date_str=$(ocdb_format_date "$ts_ms")

    [ ${#title} -gt 34 ] && title="${title:0:32}…"

    printf "  ${DIM}•${RESET}  %-34s  ${DIM}%-20s${RESET}  ${GREEN}\$%-8s${RESET}  ${DIM}%s${RESET}\n" \
      "$title" "${agent:-—}" "$cost" "$date_str"
  done
}

# ─────────────────────────────────────────
# SECTION : TICKETS (bd)
# ─────────────────────────────────────────

_show_tickets_section() {
  if ! command -v bd &>/dev/null; then
    return
  fi

  local projects_file="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
  local paths_file="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"

  [ ! -f "$projects_file" ] || [ ! -f "$paths_file" ] && return

  local project_ids=()
  while IFS= read -r line; do
    [[ "$line" =~ ^##[[:space:]]+([A-Z0-9_-]+)$ ]] && project_ids+=("${BASH_REMATCH[1]}")
  done < "$projects_file"

  [ ${#project_ids[@]} -eq 0 ] && return

  local any_shown=false

  for proj_id in "${project_ids[@]}"; do
    local proj_path=""
    while IFS='=' read -r key val; do
      [[ "$key" =~ ^[[:space:]]*# ]] && continue
      [[ -z "$key" ]] && continue
      local k="${key// /}"
      if [ "$k" = "$proj_id" ]; then proj_path="$val"; break; fi
    done < "$paths_file"

    [ "$proj_path" = "." ] && proj_path="$HUB_DIR"
    [ -z "$proj_path" ] || [ ! -d "$proj_path" ] || [ ! -d "$proj_path/.beads" ] && continue

    local done_count=0 inprogress_count=0 todo_count=0 blocked_count=0 total_count=0
    local bd_output
    if bd_output=$(bd -C "$proj_path" list --json --no-tree 2>/dev/null); then
      if command -v jq &>/dev/null && echo "$bd_output" | jq empty 2>/dev/null; then
        done_count=$(echo "$bd_output" | jq '[.[] | select(.status == "done")] | length' 2>/dev/null || echo "0")
        inprogress_count=$(echo "$bd_output" | jq '[.[] | select(.status == "in_progress")] | length' 2>/dev/null || echo "0")
        todo_count=$(echo "$bd_output" | jq '[.[] | select(.status == "todo" or .status == "pending")] | length' 2>/dev/null || echo "0")
        blocked_count=$(echo "$bd_output" | jq '[.[] | select(.status == "blocked")] | length' 2>/dev/null || echo "0")
        total_count=$(echo "$bd_output" | jq 'length' 2>/dev/null || echo "0")
      fi
    fi

    [ "$total_count" -eq 0 ] && continue

    if [ "$any_shown" = "false" ]; then
      _metrics_section "📁 Tickets par projet"
      any_shown=true
    fi

    echo ""
    printf "  ${BOLD}[%-12s${RESET}${BOLD}]${RESET}  " "$proj_id"
    printf "${GREEN}✅ %-3s${RESET}" "$done_count"
    printf "  ${CYAN}🔄 %-3s${RESET}" "$inprogress_count"
    printf "  ${DIM}⏳ %-3s${RESET}" "$todo_count"
    [ "$blocked_count" -gt 0 ] && printf "  ${YELLOW}🚫 %-3s${RESET}" "$blocked_count"
    echo ""
  done
}

# ─────────────────────────────────────────
# SECTION : WORKFLOW VÉLOCITÉ (rétrocompat)
# ─────────────────────────────────────────

_show_velocity_section() {
  local metrics_file
  metrics_file=$(metrics_get_file_path 2>/dev/null || echo "")
  [ -z "$metrics_file" ] || ! metrics_file_exists 2>/dev/null && return

  metrics_aggregate 2>/dev/null || return
  [ "${METRICS_TOTAL_TICKETS:-0}" -eq 0 ] && return

  _metrics_section "🔧 Vélocité workflow"
  echo ""

  _metrics_item "Tickets complétés"   "${GREEN}${METRICS_TOTAL_TICKETS}${RESET}"
  [ "${METRICS_AVG_DURATION:-0}" -gt 0 ] && \
    _metrics_item "Temps moyen / ticket" "${CYAN}${METRICS_AVG_DURATION_FMT}${RESET}"
  _metrics_item "Cycles review moyens" "${CYAN}${METRICS_AVG_CYCLES:-0}${RESET}"

  if [ "${#METRICS_TOP_CORRECTIONS[@]}" -gt 0 ]; then
    echo ""
    echo -e "  ${DIM}Top raisons de correction :${RESET}"
    local rank=1
    for entry in "${METRICS_TOP_CORRECTIONS[@]}"; do
      local reason="${entry%|*}" count="${entry#*|}" color="${DIM}"
      [ $rank -eq 1 ] && color="${YELLOW}"
      [ $rank -eq 2 ] && color="${CYAN}"
      printf "    ${color}%d.${RESET} %-30s ${DIM}— %s fois${RESET}\n" "$rank" "$reason" "$count"
      rank=$((rank + 1))
    done
  fi
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  _parse_args "$@"

  _metrics_header

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${YELLOW}⚠${RESET}  sqlite3 ou base OpenCode non disponible"
    echo -e "  ${DIM}  macOS  : sqlite3 est natif (/usr/bin/sqlite3)${RESET}"
    echo -e "  ${DIM}  Linux  : sudo apt-get install sqlite3${RESET}"
    echo ""
  else
    ocdb_aggregate "$_PERIOD_DAYS" || true

    # 1. Vue globale (sessions exactes, coût exact période, tokens, cache, économies plugins)
    _show_global_section

    # 2. Coût total (lifetime + breakdown today/week/month par steps)
    _show_total_cost_section

    # 3. Coût fusionné (projet / agent / modèle)
    _show_cost_section

    # 3. Activité (tool-use patterns)
    _show_activity_section "$_PERIOD_DAYS"

    # 4. Sessions récentes
    _show_recent_sessions_section
  fi

  # 5. Tickets (bd — indépendant de SQLite)
  _show_tickets_section

  # 6. Vélocité workflow (rétrocompat)
  _show_velocity_section

  echo ""
  echo -e "${DIM}──────────────────────────────────────────────${RESET}"
  local db_path
  db_path=$(ocdb_get_db_path 2>/dev/null || echo "n/a")
  echo -e "${DIM}Sources : ${db_path}${RESET}"
  echo ""
}

main "$@"
