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
# Sources de données :
#   - ~/.local/share/opencode/opencode.db (sessions, tokens, coûts)
#   - bd list/show (tickets par projet)
#   - .opencode/metrics.jsonl (vélocité workflow — rétrocompat.)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"
source "$LIB_DIR/metrics.sh"

# ─────────────────────────────────────────
# PARSE ARGS
# ─────────────────────────────────────────

_PERIOD="week"      # défaut : 7 jours
_PERIOD_DAYS=7
_PERIOD_LABEL="7 derniers jours"

_parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --period)
        shift
        case "${1:-}" in
          today)
            _PERIOD="today"
            _PERIOD_DAYS=1
            _PERIOD_LABEL="Aujourd'hui"
            ;;
          week)
            _PERIOD="week"
            _PERIOD_DAYS=7
            _PERIOD_LABEL="7 derniers jours"
            ;;
          month)
            _PERIOD="month"
            _PERIOD_DAYS=30
            _PERIOD_LABEL="30 derniers jours"
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
  echo -e "${BOLD}${CYAN}📊 Métriques OpenCode Hub${RESET}"
  echo -e "${DIM}══════════════════════════════════════════${RESET}"
  echo -e "  ${DIM}Période : ${RESET}${CYAN}${_PERIOD_LABEL}${RESET}"
  echo ""
}

_metrics_section() {
  echo ""
  echo -e "${BOLD}${BLUE}$1${RESET}"
}

_metrics_item() {
  printf "  ${DIM}•${RESET}  %-30s " "$1"
  echo -e "$2"
}

_metrics_subsection() {
  echo -e "  ${DIM}$1${RESET}"
}

# Barre de progression ASCII proportionnelle à un maximum
# Usage : _metrics_bar valeur max largeur
_metrics_bar() {
  local val="${1:-0}"
  local max="${2:-1}"
  local width="${3:-20}"
  if [ "$max" -eq 0 ] 2>/dev/null; then max=1; fi
  local filled
  filled=$(awk "BEGIN { printf \"%d\", ($val / $max) * $width }")
  local empty=$(( width - filled ))
  printf "["
  printf '%0.s█' $(seq 1 "$filled" 2>/dev/null) 2>/dev/null || true
  printf '%0.s░' $(seq 1 "$empty" 2>/dev/null) 2>/dev/null || true
  printf "]"
}

# Formate un coût USD en affichage coloré
_fmt_cost() {
  local cost="${1:-0}"
  if awk "BEGIN { exit !($cost > 0) }" 2>/dev/null; then
    echo -e "${GREEN}\$${cost}${RESET}"
  else
    echo -e "${DIM}\$0.00${RESET}"
  fi
}

# ─────────────────────────────────────────
# SECTION : TICKETS (bd)
# ─────────────────────────────────────────

_show_tickets_section() {
  _metrics_section "🎯 Tickets par projet"

  if ! command -v bd &>/dev/null; then
    echo -e "  ${DIM}bd non disponible — installer Beads pour voir les tickets${RESET}"
    return
  fi

  # Lire les projets depuis la configuration hub
  local projects_file="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
  local paths_file="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"

  if [ ! -f "$projects_file" ] || [ ! -f "$paths_file" ]; then
    echo -e "  ${DIM}Registre de projets introuvable${RESET}"
    return
  fi

  # Extraire les IDs de projet depuis projects.md (lignes ## ID)
  local project_ids=()
  while IFS= read -r line; do
    if [[ "$line" =~ ^##[[:space:]]+([A-Z0-9_-]+)$ ]]; then
      project_ids+=("${BASH_REMATCH[1]}")
    fi
  done < "$projects_file"

  if [ ${#project_ids[@]} -eq 0 ]; then
    echo -e "  ${DIM}Aucun projet trouvé${RESET}"
    return
  fi

  local any_shown=false

  for proj_id in "${project_ids[@]}"; do
    # Résoudre le chemin du projet
    local proj_path=""
    while IFS='=' read -r key val; do
      # Ignorer lignes vides et commentaires
      [[ "$key" =~ ^[[:space:]]*# ]] && continue
      [[ -z "$key" ]] && continue
      local k="${key// /}"
      if [ "$k" = "$proj_id" ]; then
        proj_path="$val"
        break
      fi
    done < "$paths_file"

    # Chemin relatif "." = hub lui-même
    if [ "$proj_path" = "." ]; then
      proj_path="$HUB_DIR"
    fi

    [ -z "$proj_path" ] && continue
    [ ! -d "$proj_path" ] && continue

    # Vérifier que beads est initialisé pour ce projet
    if [ ! -d "$proj_path/.beads" ]; then
      continue
    fi

    # Compter les tickets par statut via bd list
    local done_count=0 inprogress_count=0 todo_count=0 blocked_count=0 total_count=0

    local bd_output
    if bd_output=$(bd list --format json 2>/dev/null); then
      # Parser le JSON si jq disponible
      if command -v jq &>/dev/null && echo "$bd_output" | jq empty 2>/dev/null; then
        done_count=$(echo "$bd_output" | jq '[.[] | select(.status == "done")] | length' 2>/dev/null || echo "0")
        inprogress_count=$(echo "$bd_output" | jq '[.[] | select(.status == "in_progress")] | length' 2>/dev/null || echo "0")
        todo_count=$(echo "$bd_output" | jq '[.[] | select(.status == "todo" or .status == "pending")] | length' 2>/dev/null || echo "0")
        blocked_count=$(echo "$bd_output" | jq '[.[] | select(.status == "blocked")] | length' 2>/dev/null || echo "0")
        total_count=$(echo "$bd_output" | jq 'length' 2>/dev/null || echo "0")
      fi
    fi

    if [ "$total_count" -eq 0 ]; then
      continue
    fi

    any_shown=true
    echo ""
    echo -e "  ${BOLD}[${proj_id}]${RESET}"
    _metrics_item "✅ Complétés"    "${GREEN}${done_count}${RESET}"
    _metrics_item "🔄 En cours"     "${CYAN}${inprogress_count}${RESET}"
    _metrics_item "⏳ En attente"   "${DIM}${todo_count}${RESET}"
    [ "$blocked_count" -gt 0 ] && _metrics_item "🚫 Bloqués" "${YELLOW}${blocked_count}${RESET}"
  done

  if [ "$any_shown" = "false" ]; then
    echo -e "  ${DIM}Aucun ticket disponible (bd non initialisé ou aucun ticket)${RESET}"
  fi
}

# ─────────────────────────────────────────
# SECTION : WORKFLOW VÉLOCITÉ (rétrocompat)
# ─────────────────────────────────────────

_show_velocity_section() {
  local metrics_file
  metrics_file=$(metrics_get_file_path 2>/dev/null || echo "")

  if [ -z "$metrics_file" ] || ! metrics_file_exists 2>/dev/null; then
    return
  fi

  metrics_aggregate 2>/dev/null || return

  if [ "${METRICS_TOTAL_TICKETS:-0}" -eq 0 ]; then
    return
  fi

  _metrics_section "🔧 Vélocité workflow"
  echo ""

  _metrics_item "Tickets complétés" "${GREEN}${METRICS_TOTAL_TICKETS}${RESET}"

  if [ "${METRICS_AVG_DURATION:-0}" -gt 0 ]; then
    _metrics_item "Temps moyen / ticket" "${CYAN}${METRICS_AVG_DURATION_FMT}${RESET}"
  fi

  _metrics_item "Cycles review moyens" "${CYAN}${METRICS_AVG_CYCLES:-0}${RESET}"

  if [ "${#METRICS_TOP_CORRECTIONS[@]}" -gt 0 ]; then
    echo ""
    _metrics_subsection "Top raisons de correction :"
    local rank=1
    for entry in "${METRICS_TOP_CORRECTIONS[@]}"; do
      local reason="${entry%|*}"
      local count="${entry#*|}"
      local color=""
      case $rank in
        1) color="${YELLOW}" ;;
        2) color="${CYAN}" ;;
        *) color="${DIM}" ;;
      esac
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

  # ── Section SQLite ───────────────────────
  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${YELLOW}⚠${RESET}  sqlite3 ou base OpenCode non disponible"
    echo -e "  ${DIM}Pour activer les métriques de coûts et tokens :${RESET}"
    echo -e "  ${DIM}  macOS  : sqlite3 est natif (/usr/bin/sqlite3)${RESET}"
    echo -e "  ${DIM}  Linux  : sudo apt-get install sqlite3${RESET}"
    echo -e "  ${DIM}  Puis lancer OpenCode au moins une fois.${RESET}"
    echo ""
  else
    # Agréger les données SQLite
    ocdb_aggregate "$_PERIOD_DAYS" || true

    # ── Statistiques globales ────────────────
    _metrics_section "📈 Vue globale"
    echo ""
    _metrics_item "Sessions totales"   "${GREEN}${OCDB_TOTAL_SESSIONS}${RESET}"
    _metrics_item "Coût total"         "${GREEN}\$${OCDB_TOTAL_COST}${RESET}"
    echo ""

    # Tokens
    local fmt_input fmt_output fmt_cache_read fmt_cache_write
    fmt_input=$(ocdb_format_tokens "$OCDB_TOKENS_INPUT")
    fmt_output=$(ocdb_format_tokens "$OCDB_TOKENS_OUTPUT")
    fmt_cache_read=$(ocdb_format_tokens "$OCDB_TOKENS_CACHE_READ")
    fmt_cache_write=$(ocdb_format_tokens "$OCDB_TOKENS_CACHE_WRITE")

    _metrics_item "Tokens input"         "${CYAN}${fmt_input}${RESET}"
    _metrics_item "Tokens output"        "${CYAN}${fmt_output}${RESET}"
    _metrics_item "Cache write"          "${DIM}${fmt_cache_write}${RESET}"
    _metrics_item "Cache read"           "${DIM}${fmt_cache_read}${RESET}"
    echo ""

    # Cache hit rate
    local hit_color="${DIM}"
    local hit_rate="${OCDB_CACHE_HIT_RATE:-0.0}"
    if awk "BEGIN { exit !($hit_rate >= 80) }" 2>/dev/null; then
      hit_color="${GREEN}"
    elif awk "BEGIN { exit !($hit_rate >= 50) }" 2>/dev/null; then
      hit_color="${CYAN}"
    elif awk "BEGIN { exit !($hit_rate > 0) }" 2>/dev/null; then
      hit_color="${YELLOW}"
    fi
    _metrics_item "Cache hit rate"  "${hit_color}${hit_rate}%${RESET}  ${DIM}(économies estimées : \$${OCDB_CACHE_SAVINGS})${RESET}"

    # ── Coût par projet ───────────────────────
    if [ "${#OCDB_TOP_PROJECTS[@]}" -gt 0 ]; then
      _metrics_section "💰 Coût par projet"
      echo ""

      # Trouver le max pour la barre de progression
      local max_cost=0
      for entry in "${OCDB_TOP_PROJECTS[@]}"; do
        local cost="${entry##*|}"
        if awk "BEGIN { exit !($cost > $max_cost) }" 2>/dev/null; then
          max_cost="$cost"
        fi
      done

      for entry in "${OCDB_TOP_PROJECTS[@]}"; do
        local dir="${entry%|*}"
        local cost="${entry##*|}"
        # Raccourcir le chemin : garder seulement le nom du dossier
        local short_dir
        short_dir=$(basename "$dir" 2>/dev/null || echo "$dir")
        printf "  ${DIM}•${RESET}  %-28s ${GREEN}\$%-8s${RESET}" "$short_dir" "$cost"
        echo ""
      done
    fi

    # ── Top agents ────────────────────────────
    if [ "${#OCDB_TOP_AGENTS[@]}" -gt 0 ]; then
      _metrics_section "🤖 Top agents"
      echo ""
      for entry in "${OCDB_TOP_AGENTS[@]}"; do
        local agent="${entry%|*}"
        local cost="${entry##*|}"
        [ -z "$agent" ] || [ "$agent" = "unknown" ] && continue
        _metrics_item "$agent" "${CYAN}\$${cost}${RESET}"
      done
    fi

    # ── Top modèles ───────────────────────────
    if [ "${#OCDB_TOP_MODELS[@]}" -gt 0 ]; then
      _metrics_section "🧠 Top modèles"
      echo ""
      for entry in "${OCDB_TOP_MODELS[@]}"; do
        local model="${entry%|*}"
        local cost="${entry##*|}"
        [ -z "$model" ] || [ "$model" = "unknown" ] && continue
        _metrics_item "$model" "${DIM}\$${cost}${RESET}"
      done
    fi

    # ── Sessions récentes ─────────────────────
    if [ "${#OCDB_RECENT_SESSIONS[@]}" -gt 0 ]; then
      _metrics_section "🕐 Sessions récentes"
      echo ""
      for entry in "${OCDB_RECENT_SESSIONS[@]}"; do
        local slug title agent cost ts_ms
        slug=$(echo "$entry" | cut -d'|' -f1)
        title=$(echo "$entry" | cut -d'|' -f2)
        agent=$(echo "$entry" | cut -d'|' -f3)
        cost=$(echo "$entry" | cut -d'|' -f4)
        ts_ms=$(echo "$entry" | cut -d'|' -f5)

        local date_str
        date_str=$(ocdb_format_date "$ts_ms")

        # Tronquer le titre à 35 chars
        if [ ${#title} -gt 35 ]; then
          title="${title:0:33}…"
        fi

        printf "  ${DIM}•${RESET}  %-35s  %-20s  ${GREEN}\$%-8s${RESET}  ${DIM}%s${RESET}\n" \
          "$title" "${agent:-—}" "$cost" "$date_str"
      done
    fi
  fi

  # ── Tickets bd ───────────────────────────
  _show_tickets_section

  # ── Vélocité workflow (rétrocompat) ─────
  _show_velocity_section

  echo ""
  echo -e "${DIM}──────────────────────────────────────────${RESET}"
  local db_path
  db_path=$(ocdb_get_db_path 2>/dev/null || echo "n/a")
  echo -e "${DIM}Sources : ${db_path}${RESET}"
  echo ""
}

main "$@"
