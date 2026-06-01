#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-dashboard.sh — Dashboard TUI pour suivre l'avancement des sessions
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   ./oc.sh dashboard          → affiche le dashboard de session
#
# Affiche :
# - État de la session (active ou idle)
# - Liste des tickets avec leur statut emoji
# - Agent actif et action en cours
# - Informations de session (démarrage, mode)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/session-state.sh"

# ─────────────────────────────────────────
# TUI HELPERS (tput based)
# ─────────────────────────────────────────

# Largeur de la boîte
BOX_WIDTH=50

# Dessine une ligne horizontale
_draw_line() {
  local char="${1:-─}"
  local width="${2:-$BOX_WIDTH}"
  printf '%*s' "$width" '' | tr ' ' "$char"
}

# Dessine le header de la boîte
_draw_header() {
  local title="$1"
  local title_len=${#title}
  local padding=$(( (BOX_WIDTH - title_len - 4) / 2 ))
  local padding_right=$(( BOX_WIDTH - title_len - 4 - padding ))

  echo -e "${CYAN}╭$(_draw_line "─" $((BOX_WIDTH - 2)))╮${RESET}"
  printf "${CYAN}│${RESET}%*s${BOLD}%s${RESET}%*s${CYAN}│${RESET}\n" "$padding" "" "$title" "$padding_right" ""
  echo -e "${CYAN}╰$(_draw_line "─" $((BOX_WIDTH - 2)))╯${RESET}"
}

# Dessine une section avec titre
_draw_section() {
  local title="$1"
  echo ""
  echo -e "${BOLD}${BLUE}${title}${RESET}"
}

# Emoji pour le statut
_status_emoji() {
  local status="$1"
  case "$status" in
    pending)     echo "⏳" ;;
    in_progress) echo "🔄" ;;
    review)      echo "👁️" ;;
    completed)   echo "✅" ;;
    blocked)     echo "🚫" ;;
    *)           echo "❓" ;;
  esac
}

# Label pour l'action
_action_label() {
  local action="$1"
  case "$action" in
    implementing) echo "Implémentation" ;;
    testing)      echo "Tests" ;;
    reviewing)    echo "Review" ;;
    waiting_cp2)  echo "Attente CP-2" ;;
    idle)         echo "En attente" ;;
    *)            echo "$action" ;;
  esac
}

# Label pour le mode
_mode_label() {
  local mode="$1"
  case "$mode" in
    manuel)    echo "Manuel" ;;
    semi-auto) echo "Semi-auto" ;;
    auto)      echo "Auto" ;;
    *)         echo "$mode" ;;
  esac
}

# Formate un timestamp ISO8601 en heure lisible (HH:MM UTC)
_format_time() {
  local ts="$1"
  # Extraire l'heure et les minutes du timestamp ISO8601
  # Format: 2024-01-15T10:30:00Z
  if [[ "$ts" =~ T([0-9]{2}):([0-9]{2}) ]]; then
    echo "${BASH_REMATCH[1]}:${BASH_REMATCH[2]} UTC"
  else
    echo "--:--"
  fi
}

# ─────────────────────────────────────────
# DASHBOARD VIEWS
# ─────────────────────────────────────────

# Affiche le dashboard quand aucune session n'est active
_show_idle_dashboard() {
  echo ""
  _draw_header "OpenCode Dashboard"
  echo ""
  echo -e "  ${DIM}Aucune session active${RESET}"
  echo ""
  echo -e "  ${DIM}Lancez une session avec :${RESET}"
  echo -e "  ${CYAN}oc start <project>${RESET}"
  echo ""
}

# Affiche le dashboard avec une session active
_show_active_dashboard() {
  local state_json="$1"

  # Parser les données JSON avec jq
  local started_at mode
  started_at=$(echo "$state_json" | jq -r '.started_at // ""')
  mode=$(echo "$state_json" | jq -r '.mode // ""')

  # Current ticket
  local current_id current_agent current_action
  current_id=$(echo "$state_json" | jq -r '.current_ticket.id // ""')
  current_agent=$(echo "$state_json" | jq -r '.current_ticket.agent // ""')
  current_action=$(echo "$state_json" | jq -r '.current_ticket.action // ""')

  # Header
  echo ""
  _draw_header "OpenCode Dashboard — Session Active"

  # Section : Tickets
  _draw_section "📋 Tickets"
  echo ""

  # Lire les tickets
  local ticket_count
  ticket_count=$(echo "$state_json" | jq '.tickets | length')

  if [ "$ticket_count" -eq 0 ]; then
    echo -e "  ${DIM}Aucun ticket dans la session${RESET}"
  else
    # Itérer sur les tickets
    local i=0
    while [ "$i" -lt "$ticket_count" ]; do
      local t_id t_status t_title emoji
      t_id=$(echo "$state_json" | jq -r ".tickets[$i].id // \"\"")
      t_status=$(echo "$state_json" | jq -r ".tickets[$i].status // \"pending\"")
      t_title=$(echo "$state_json" | jq -r ".tickets[$i].title // \"\"")
      emoji=$(_status_emoji "$t_status")

      # Mettre en évidence le ticket courant
      if [ "$t_id" = "$current_id" ]; then
        echo -e "  ${emoji} ${BOLD}${t_id}${RESET} — ${t_title} ${CYAN}◀${RESET}"
      else
        echo -e "  ${emoji} ${t_id} — ${t_title}"
      fi

      i=$((i + 1))
    done
  fi

  # Section : Agent actif
  if [ -n "$current_agent" ]; then
    echo ""
    echo -e "${BOLD}${BLUE}🤖 Agent actif${RESET} : ${GREEN}${current_agent}${RESET}"
  fi

  # Section : Action en cours
  if [ -n "$current_action" ]; then
    local action_label
    action_label=$(_action_label "$current_action")
    echo -e "${BOLD}${BLUE}🎬 Action${RESET} : ${CYAN}${action_label}${RESET}"
  fi

  # Section : Info session
  echo ""
  local started_time mode_label
  started_time=$(_format_time "$started_at")
  mode_label=$(_mode_label "$mode")
  echo -e "${DIM}⏱️  Session démarrée à ${started_time} — Mode: ${mode_label}${RESET}"

  echo ""
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  # Vérifier que jq est disponible
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq est requis pour le dashboard. Installez-le avec : brew install jq"
    exit 1
  fi

  # Lire l'état de session
  local state_json
  state_json=$(session_state_read)

  # Valider que le JSON est bien formé
  if [ -n "$state_json" ] && ! echo "$state_json" | jq empty 2>/dev/null; then
    log_error "État de session corrompu — fichier JSON invalide"
    _show_idle_dashboard
    exit 0
  fi

  # Afficher le dashboard approprié
  if [ -z "$state_json" ]; then
    _show_idle_dashboard
  else
    _show_active_dashboard "$state_json"
  fi
}

main "$@"
