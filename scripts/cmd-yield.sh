#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-yield.sh — Corrélation sessions OpenCode ↔ commits git
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   oc yield                     → 7 derniers jours (défaut)
#   oc yield --period today      → aujourd'hui
#   oc yield --period month      → 30 derniers jours
#   oc yield --project T-SRU     → filtré sur un projet
#
# Classifie chaque session en :
#   Productive  : au moins un commit git dans les 24h suivant la session
#   Revertée    : commit trouvé mais suivi d'un revert
#   Abandonnée  : aucun commit dans la fenêtre de 24h
#
# Sources de données :
#   - ~/.local/share/opencode/opencode.db (sessions)
#   - git log (par répertoire de projet)
#   - projects.md + paths.local.md (résolution projet → chemin)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/opencode-db.sh"

# ─────────────────────────────────────────
# PARSE ARGS
# ─────────────────────────────────────────

_PERIOD_DAYS=7
_PERIOD_LABEL="7 derniers jours"
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
# HELPERS GIT
# ─────────────────────────────────────────

# Résoudre le dépôt git principal à partir d'un chemin (supporte worktrees)
_git_root() {
  local path="$1"
  [ -d "$path" ] || return 1
  git -C "$path" rev-parse --show-toplevel 2>/dev/null || return 1
}

# Vérifie si un commit est un revert (message commence par "Revert")
_is_revert_commit() {
  local repo="$1"
  local hash="$2"
  local msg
  msg=$(git -C "$repo" log --format="%s" -1 "$hash" 2>/dev/null || echo "")
  [[ "$msg" == Revert* ]]
}

# Cherche des commits dans une fenêtre de temps [start_s, end_s] dans un dépôt git
# Retourne les hashes (un par ligne), vide si aucun
_git_commits_in_window() {
  local repo="$1"
  local start_s="$2"  # timestamp Unix (secondes)
  local end_s="$3"

  [ -d "$repo/.git" ] || [ -f "$repo/.git" ] || return 0

  # Formater les dates selon l'OS
  local after_date before_date
  if date -r "$start_s" "+%Y-%m-%dT%H:%M:%S" >/dev/null 2>&1; then
    # macOS
    after_date=$(date -r "$start_s" "+%Y-%m-%dT%H:%M:%S")
    before_date=$(date -r "$end_s"  "+%Y-%m-%dT%H:%M:%S")
  else
    # Linux
    after_date=$(date -d "@${start_s}" "+%Y-%m-%dT%H:%M:%S" 2>/dev/null || echo "")
    before_date=$(date -d "@${end_s}"  "+%Y-%m-%dT%H:%M:%S" 2>/dev/null || echo "")
  fi

  [ -z "$after_date" ] && return 0

  # Limiter à 20 commits max par fenêtre pour la perf
  git -C "$repo" log \
    --format="%H" \
    --after="$after_date" \
    --before="$before_date" \
    --max-count=20 \
    2>/dev/null || true
}

# ─────────────────────────────────────────
# RÉSOLUTION PROJETS
# ─────────────────────────────────────────

# Retourne tous les projets configurés sous forme "PROJECT_ID|path"
_get_configured_projects() {
  local projects_file="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
  local paths_file="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"

  [ -f "$projects_file" ] || return
  [ -f "$paths_file" ] || return

  local project_ids=()
  while IFS= read -r line; do
    if [[ "$line" =~ ^##[[:space:]]+([A-Z0-9_-]+)$ ]]; then
      project_ids+=("${BASH_REMATCH[1]}")
    fi
  done < "$projects_file"

  for proj_id in "${project_ids[@]}"; do
    # Filtre optionnel
    if [ -n "$_PROJECT_FILTER" ] && [ "$proj_id" != "$_PROJECT_FILTER" ]; then
      continue
    fi

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

    echo "${proj_id}|${proj_path}"
  done
}

# Trouve le répertoire git correspondant à un path de session
# Essaie une correspondance directe, puis cherche dans les projets configurés
_find_project_for_dir() {
  local session_dir="$1"

  while IFS='|' read -r proj_id proj_path; do
    # Correspondance exacte ou préfixe (worktrees)
    if [[ "$session_dir" == "$proj_path"* ]] || [[ "$session_dir" == "${proj_path%/}"* ]]; then
      echo "$proj_id|$proj_path"
      return 0
    fi
    # Correspondance via git root (worktrees partagent le même dépôt)
    local proj_root session_root
    proj_root=$(_git_root "$proj_path" 2>/dev/null || echo "")
    session_root=$(_git_root "$session_dir" 2>/dev/null || echo "")
    if [ -n "$proj_root" ] && [ -n "$session_root" ] && [ "$proj_root" = "$session_root" ]; then
      echo "$proj_id|$proj_path"
      return 0
    fi
  done < <(_get_configured_projects)

  return 1
}

# ─────────────────────────────────────────
# ANALYSE PAR PROJET
# ─────────────────────────────────────────

_analyze_project_yield() {
  local proj_id="$1"
  local proj_path="$2"

  local since_ts
  since_ts=$(_ocdb_since_ts "$_PERIOD_DAYS")

  # Récupérer les sessions du projet depuis la DB
  local sessions=()
  while IFS= read -r line; do
    [ -n "$line" ] && sessions+=("$line")
  done < <(_ocdb_query "
    SELECT
      id,
      ROUND(cost, 4),
      time_created,
      time_updated
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts}
      AND (
        directory = '${proj_path}'
        OR directory LIKE '${proj_path}%'
      )
    ORDER BY time_created DESC;
  " 2>/dev/null || true)

  local total=${#sessions[@]}
  [ "$total" -eq 0 ] && return

  # Résoudre le dépôt git du projet
  local git_repo
  git_repo=$(_git_root "$proj_path" 2>/dev/null || echo "")
  if [ -z "$git_repo" ]; then
    echo ""
    echo -e "  ${BOLD}${CYAN}[${proj_id}]${RESET}  ${DIM}(pas de dépôt git — yield non disponible)${RESET}"
    return
  fi

  local productive=0 abandoned=0 reverted=0
  local cost_productive=0 cost_abandoned=0 cost_reverted=0

  for entry in "${sessions[@]}"; do
    local sess_id cost start_ts end_ts
    sess_id=$(echo "$entry" | cut -d'|' -f1)
    cost=$(echo "$entry"    | cut -d'|' -f2)
    start_ts=$(echo "$entry" | cut -d'|' -f3)
    end_ts=$(echo "$entry"  | cut -d'|' -f4)

    # Convertir ms → s
    local start_s end_s window_end
    start_s=$(( start_ts / 1000 ))
    end_s=$(( end_ts / 1000 ))
    # Fenêtre : fin de session + 24h
    window_end=$(( end_s + 86400 ))

    local commits
    commits=$(_git_commits_in_window "$git_repo" "$start_s" "$window_end" 2>/dev/null || echo "")

    if [ -z "$commits" ]; then
      abandoned=$(( abandoned + 1 ))
      cost_abandoned=$(LC_ALL=C awk "BEGIN { printf \"%.4f\", $cost_abandoned + ${cost:-0} }")
    else
      # Vérifier si un commit est un revert
      local has_revert=false
      while IFS= read -r hash; do
        [ -z "$hash" ] && continue
        if _is_revert_commit "$git_repo" "$hash" 2>/dev/null; then
          has_revert=true
          break
        fi
      done <<< "$commits"

      if [ "$has_revert" = "true" ]; then
        reverted=$(( reverted + 1 ))
        cost_reverted=$(LC_ALL=C awk "BEGIN { printf \"%.4f\", $cost_reverted + ${cost:-0} }")
      else
        productive=$(( productive + 1 ))
        cost_productive=$(LC_ALL=C awk "BEGIN { printf \"%.4f\", $cost_productive + ${cost:-0} }")
      fi
    fi
  done

  # Affichage
  echo ""
  echo -e "  ${BOLD}${CYAN}[${proj_id}]${RESET}  ${DIM}${total} sessions — dépôt : $(basename "$git_repo")${RESET}"

  local pct_productive=0 pct_abandoned=0 pct_reverted=0
  if [ "$total" -gt 0 ]; then
    pct_productive=$(LC_ALL=C awk "BEGIN { printf \"%d\", $productive/$total*100 }")
    pct_abandoned=$(LC_ALL=C  awk "BEGIN { printf \"%d\", $abandoned/$total*100 }")
    pct_reverted=$(LC_ALL=C   awk "BEGIN { printf \"%d\", $reverted/$total*100 }")
  fi

  printf "  ${DIM}•${RESET}  %-14s  ${GREEN}%-3s${RESET}  ${DIM}(%3s%%)${RESET}  ${GREEN}\$%s${RESET}\n" \
    "Productives" "$productive" "$pct_productive" "$cost_productive"
  printf "  ${DIM}•${RESET}  %-14s  ${YELLOW}%-3s${RESET}  ${DIM}(%3s%%)${RESET}  ${YELLOW}\$%s${RESET}\n" \
    "Abandonnées" "$abandoned" "$pct_abandoned" "$cost_abandoned"
  printf "  ${DIM}•${RESET}  %-14s  ${RED}%-3s${RESET}  ${DIM}(%3s%%)${RESET}  ${RED}\$%s${RESET}\n" \
    "Revertées" "$reverted" "$pct_reverted" "$cost_reverted"

  # Warning si coût abandonné élevé
  if awk "BEGIN { exit !($cost_abandoned > 10) }" 2>/dev/null; then
    echo ""
    echo -e "  ${YELLOW}⚠${RESET}  ${DIM}\$${cost_abandoned} de sessions sans commit — utiliser ${RESET}oc optimize${DIM} pour investiguer${RESET}"
  fi
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  _parse_args "$@"

  echo ""
  echo -e "${BOLD}${CYAN}🌾 Yield — Sessions ↔ Commits git${RESET}"
  echo -e "${DIM}══════════════════════════════════════════${RESET}"
  echo -e "  ${DIM}Période  : ${RESET}${CYAN}${_PERIOD_LABEL}${RESET}"
  echo -e "  ${DIM}Fenêtre  : ${RESET}${DIM}commits dans les 24h suivant chaque session${RESET}"
  [ -n "$_PROJECT_FILTER" ] && echo -e "  ${DIM}Projet   : ${RESET}${CYAN}${_PROJECT_FILTER}${RESET}"
  echo ""

  if ! ocdb_check_available 2>/dev/null; then
    echo -e "  ${YELLOW}⚠${RESET}  sqlite3 ou base OpenCode non disponible"
    echo ""
    exit 0
  fi

  if ! command -v git &>/dev/null; then
    echo -e "  ${YELLOW}⚠${RESET}  git non disponible — yield nécessite git"
    echo ""
    exit 0
  fi

  local any_shown=false

  while IFS='|' read -r proj_id proj_path; do
    _analyze_project_yield "$proj_id" "$proj_path"
    any_shown=true
  done < <(_get_configured_projects)

  if [ "$any_shown" = "false" ]; then
    echo -e "  ${DIM}Aucun projet configuré ou aucune session trouvée.${RESET}"
    echo -e "  ${DIM}Vérifier projects.md et paths.local.md${RESET}"
  fi

  echo ""
  echo -e "${DIM}──────────────────────────────────────────${RESET}"
  echo -e "  ${DIM}Sessions coûteuses sans commit → oc optimize${RESET}"
  echo ""
}

main "$@"
