#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-optimize.sh — Analyse les patterns de gaspillage de tokens du hub
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc optimize                      → 30 derniers jours (défaut)
#   oc optimize --period week        → 7 derniers jours
#   oc optimize --period today       → aujourd'hui
#   oc optimize --project T-SRU      → filtré sur un projet
#
# Sources de données :
#   - ~/.local/share/opencode/opencode.db (sessions + tool calls)
#   - agents/, skills/, servers/ du hub (agents déployés)
#
# Grade global : A (0 critique) → F (5+ critiques)
# Findings : critique / warning / info
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"

# ─────────────────────────────────────────
# PARSE ARGS
# ─────────────────────────────────────────

_PERIOD_DAYS=30
_PERIOD_LABEL="30 derniers jours"
_PROJECT_FILTER=""

_parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --period)
        shift
        case "${1:-}" in
          today) _PERIOD_DAYS=1;  _PERIOD_LABEL="Aujourd'hui" ;;
          week)  _PERIOD_DAYS=7;  _PERIOD_LABEL="7 derniers jours" ;;
          month) _PERIOD_DAYS=30; _PERIOD_LABEL="30 derniers jours" ;;
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
      --project)
        shift
        _PROJECT_FILTER="${1:-}"
        ;;
      --project=*)
        _PROJECT_FILTER="${1#--project=}"
        ;;
    esac
    shift
  done
}

# ─────────────────────────────────────────
# FINDINGS — structure
# ─────────────────────────────────────────

# Tableaux de findings : "level|titre|description|fix"
_FINDINGS_CRITICAL=()
_FINDINGS_WARNING=()
_FINDINGS_INFO=()

_add_finding() {
  local level="$1"
  local title="$2"
  local desc="$3"
  local fix="${4:-}"
  local entry="${title}|${desc}|${fix}"
  case "$level" in
    critical) _FINDINGS_CRITICAL+=("$entry") ;;
    warning)  _FINDINGS_WARNING+=("$entry") ;;
    info)     _FINDINGS_INFO+=("$entry") ;;
  esac
}

# ─────────────────────────────────────────
# GRADE
# ─────────────────────────────────────────

_compute_grade() {
  local score=$(( ${#_FINDINGS_CRITICAL[@]} * 3 + ${#_FINDINGS_WARNING[@]} ))
  if   [ "$score" -eq 0 ];   then echo "A"
  elif [ "$score" -le 2 ];   then echo "B"
  elif [ "$score" -le 5 ];   then echo "C"
  elif [ "$score" -le 9 ];   then echo "D"
  elif [ "$score" -le 14 ];  then echo "E"
  else                             echo "F"
  fi
}

_grade_color() {
  case "$1" in
    A) echo "${GREEN}" ;;
    B) echo "${CYAN}" ;;
    C) echo "${YELLOW}" ;;
    D|E) echo "${RED}" ;;
    F) echo "${RED}${BOLD}" ;;
    *) echo "${DIM}" ;;
  esac
}

# ─────────────────────────────────────────
# ANALYSES
# ─────────────────────────────────────────

# Analyse 1 : MCP servers déployés mais inutilisés
_analyze_unused_mcp() {
  local unused
  while IFS= read -r server; do
    [ -n "$server" ] && unused="$server"
    [ -n "$server" ] && _add_finding "critical" \
      "MCP inutilisé : ${server}" \
      "Aucun appel ${server}_* sur les ${_PERIOD_LABEL}" \
      "oc service remove ${server%%-mcp} [PROJECT_ID]"
  done < <(ocdb_unused_mcp "$_PERIOD_DAYS" 2>/dev/null || true)
}

# Analyse 2 : Sessions coûteuses sans edit
_analyze_sessions_no_edit() {
  local sessions=()
  while IFS= read -r line; do
    [ -n "$line" ] && sessions+=("$line")
  done < <(ocdb_sessions_no_edit "$_PERIOD_DAYS" "1.0" 2>/dev/null || true)

  local count=${#sessions[@]}
  if [ "$count" -eq 0 ]; then return; fi

  local total_cost=0
  for entry in "${sessions[@]}"; do
    local cost
    cost=$(echo "$entry" | cut -d'|' -f4)
    total_cost=$(LC_ALL=C awk "BEGIN { printf \"%.2f\", $total_cost + ${cost:-0} }")
  done

  local level="warning"
  [ "$count" -ge 5 ] && level="critical"

  _add_finding "$level" \
    "Sessions sans modification : ${count} sessions (\$${total_cost})" \
    "Ces sessions ont dépensé \$${total_cost} sans aucun edit/write de fichier" \
    "Vérifier si ce sont des sessions d'exploration intentionnelles"
}

# Analyse 3 : Ratio Read/Edit faible
_analyze_read_edit_ratio() {
  local ratio
  ratio=$(ocdb_avg_read_edit_ratio "$_PERIOD_DAYS" 2>/dev/null || echo "0.0")

  local ratio_num
  ratio_num=$(LC_ALL=C awk "BEGIN { printf \"%d\", $ratio * 10 }")

  if awk "BEGIN { exit !($ratio < 1.0 && $ratio > 0) }" 2>/dev/null; then
    _add_finding "critical" \
      "Ratio Read/Edit très faible : ${ratio}" \
      "Les agents éditent sans assez lire (idéal >= 2.0) — risque de retries coûteux" \
      "Ajouter des instructions de lecture préalable dans les agents developer"
  elif awk "BEGIN { exit !($ratio < 2.0 && $ratio >= 1.0) }" 2>/dev/null; then
    _add_finding "warning" \
      "Ratio Read/Edit faible : ${ratio}" \
      "Les agents lisent peu avant d'éditer (idéal >= 2.0)" \
      "Encourager la lecture du contexte avant modification dans les prompts agents"
  else
    _add_finding "info" \
      "Ratio Read/Edit correct : ${ratio}" \
      "Les agents lisent suffisamment avant d'éditer" \
      ""
  fi
}

# Analyse 4 : Taux d'erreurs tool calls
_analyze_tool_errors() {
  local error_rate
  error_rate=$(ocdb_tool_error_rate "$_PERIOD_DAYS" 2>/dev/null || echo "0.0")

  if awk "BEGIN { exit !($error_rate > 25) }" 2>/dev/null; then
    _add_finding "critical" \
      "Taux d'erreurs élevé : ${error_rate}% des tool calls en erreur" \
      "Plus d'1 tool call sur 4 échoue — coûts inutiles en retries" \
      "Inspecter les erreurs bash récurrentes : oc metrics --period month"
  elif awk "BEGIN { exit !($error_rate > 10) }" 2>/dev/null; then
    _add_finding "warning" \
      "Taux d'erreurs notable : ${error_rate}% des tool calls en erreur" \
      "Des tool calls échouent régulièrement (normal < 10%)" \
      ""
  fi
}

# Analyse 5 : Re-lectures de fichiers dans une même session
_analyze_repeated_reads() {
  local repeated=()
  while IFS= read -r line; do
    [ -n "$line" ] && repeated+=("$line")
  done < <(ocdb_repeated_reads "$_PERIOD_DAYS" 5 2>/dev/null || true)

  local count=${#repeated[@]}
  [ "$count" -eq 0 ] && return

  local examples=""
  local i=0
  for entry in "${repeated[@]}"; do
    [ $i -ge 3 ] && break
    local fname
    fname=$(echo "$entry" | cut -d'|' -f1)
    local cnt
    cnt=$(echo "$entry" | cut -d'|' -f2)
    examples="${examples}${fname} (${cnt}×) "
    i=$((i + 1))
  done

  _add_finding "warning" \
    "Fichiers re-lus excessivement : ${count} cas détectés" \
    "Ex: ${examples}" \
    "Ajouter ces fichiers en contexte initial de session plutôt qu'en lecture répétée"
}

# Analyse 6 : Délégations task excessives
_analyze_heavy_delegation() {
  local count
  count=$(ocdb_sessions_heavy_delegation "$_PERIOD_DAYS" 2>/dev/null || echo "0")

  [ "$count" -eq 0 ] && return

  _add_finding "info" \
    "Délégation intensive : ${count} sessions avec >40% de tool calls = task" \
    "Fan-out sous-agents élevé — peut être intentionnel (orchestrateur) ou excessif" \
    ""
}

# Analyse 7 : Skills/agents jamais invoqués via l'outil skill
_analyze_unused_skills() {
  local skill_count
  skill_count=$(ocdb_tool_count "skill" "$_PERIOD_DAYS" 2>/dev/null || echo "0")

  local skills_dir="${HUB_DIR}/skills"
  local total_skills=0
  if [ -d "$skills_dir" ]; then
    total_skills=$(find "$skills_dir" -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
  fi

  if [ "$skill_count" -eq 0 ] && [ "$total_skills" -gt 0 ]; then
    _add_finding "warning" \
      "Aucune skill chargée via l'outil skill sur la période" \
      "${total_skills} skills définies dans le hub — aucune invoquée (Bucket B)" \
      "Vérifier que les agents ont accès aux skills Bucket B dans leurs frontmatters"
  elif [ "$skill_count" -gt 0 ]; then
    _add_finding "info" \
      "Skills Bucket B actives : ${skill_count} chargements" \
      "Les agents utilisent bien les skills à la demande" \
      ""
  fi
}

# Analyse 8 : Sessions de pure conversation (sans aucun tool)
_analyze_conversation_sessions() {
  local rows=()
  while IFS= read -r line; do
    [ -n "$line" ] && rows+=("$line")
  done < <(ocdb_activity_breakdown "$_PERIOD_DAYS" 2>/dev/null | grep "^conversation|" || true)

  [ ${#rows[@]} -eq 0 ] && return

  local conv_count conv_cost
  conv_count=$(echo "${rows[0]}" | cut -d'|' -f2)
  conv_cost=$(echo "${rows[0]}" | cut -d'|' -f3)

  if [ "${conv_count:-0}" -gt 3 ]; then
    _add_finding "info" \
      "Sessions conversation : ${conv_count} (\$${conv_cost})" \
      "Sessions sans aucun tool call — peut indiquer de l'usage non-productif" \
      ""
  fi
}

# Analyse 9 : Absence de cache (hit rate très bas)
_analyze_cache_usage() {
  local hit_rate
  hit_rate=$(ocdb_cache_hit_rate "$_PERIOD_DAYS" 2>/dev/null || echo "0.0")

  if awk "BEGIN { exit !($hit_rate < 30 && $hit_rate > 0) }" 2>/dev/null; then
    _add_finding "warning" \
      "Cache hit rate faible : ${hit_rate}%" \
      "Le cache prompt est peu utilisé (idéal > 80%) — les tokens input sont re-chargés inutilement" \
      "Stabiliser le system prompt et les instructions agents pour maximiser le cache"
  elif awk "BEGIN { exit !($hit_rate == 0) }" 2>/dev/null; then
    _add_finding "info" \
      "Cache hit rate : 0% (données insuffisantes ou cache non activé)" \
      "" \
      ""
  fi
}

# ─────────────────────────────────────────
# AFFICHAGE
# ─────────────────────────────────────────

_print_findings() {
  local level="$1"
  local label="$2"
  local color="$3"
  local -n arr="$4"

  [ ${#arr[@]} -eq 0 ] && return

  echo ""
  echo -e "${color}${BOLD}${label}${RESET}"
  echo ""

  for entry in "${arr[@]}"; do
    local title desc fix
    title=$(echo "$entry" | cut -d'|' -f1)
    desc=$(echo "$entry"  | cut -d'|' -f2)
    fix=$(echo "$entry"   | cut -d'|' -f3)

    echo -e "  ${color}▸${RESET} ${BOLD}${title}${RESET}"
    [ -n "$desc" ] && echo -e "    ${DIM}${desc}${RESET}"
    [ -n "$fix"  ] && echo -e "    ${CYAN}→ ${fix}${RESET}"
    echo ""
  done
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  _parse_args "$@"

  echo ""
  echo -e "${BOLD}${CYAN}🔍 Analyse d'optimisation OpenCode Hub${RESET}"
  echo -e "${DIM}══════════════════════════════════════════${RESET}"
  echo -e "  ${DIM}Période : ${RESET}${CYAN}${_PERIOD_LABEL}${RESET}"
  [ -n "$_PROJECT_FILTER" ] && echo -e "  ${DIM}Projet  : ${RESET}${CYAN}${_PROJECT_FILTER}${RESET}"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${YELLOW}⚠${RESET}  sqlite3 ou base OpenCode non disponible"
    echo -e "  ${DIM}  macOS  : sqlite3 est natif (/usr/bin/sqlite3)${RESET}"
    echo -e "  ${DIM}  Linux  : sudo apt-get install sqlite3${RESET}"
    echo ""
    exit 0
  fi

  echo -e "  ${DIM}Analyse en cours...${RESET}"

  # Lancer toutes les analyses
  _analyze_unused_mcp
  _analyze_sessions_no_edit
  _analyze_read_edit_ratio
  _analyze_tool_errors
  _analyze_repeated_reads
  _analyze_heavy_delegation
  _analyze_unused_skills
  _analyze_conversation_sessions
  _analyze_cache_usage

  # Calcul du grade
  local grade
  grade=$(_compute_grade)
  local grade_color
  grade_color=$(_grade_color "$grade")
  local total_findings=$(( ${#_FINDINGS_CRITICAL[@]} + ${#_FINDINGS_WARNING[@]} + ${#_FINDINGS_INFO[@]} ))

  # Header résultat
  echo ""
  echo -e "  ${grade_color}${BOLD}Grade : ${grade}${RESET}   ${DIM}${total_findings} finding(s) — ${#_FINDINGS_CRITICAL[@]} critique(s), ${#_FINDINGS_WARNING[@]} warning(s), ${#_FINDINGS_INFO[@]} info(s)${RESET}"
  echo -e "${DIM}──────────────────────────────────────────${RESET}"

  if [ "$total_findings" -eq 0 ]; then
    echo ""
    echo -e "  ${GREEN}✓ Aucun problème détecté sur la période.${RESET}"
    echo ""
  else
    _print_findings "critical" "🚨 Critique" "${RED}"  _FINDINGS_CRITICAL
    _print_findings "warning"  "⚠  Warning"  "${YELLOW}" _FINDINGS_WARNING
    _print_findings "info"     "ℹ  Info"     "${DIM}"   _FINDINGS_INFO
  fi

  echo -e "${DIM}──────────────────────────────────────────${RESET}"
  echo -e "  ${DIM}Pour les détails de coûts : oc metrics --period month${RESET}"
  echo ""
}

main "$@"
