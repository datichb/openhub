#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-metrics.sh — Affiche les métriques agrégées de vélocité du workflow
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   ./oc.sh metrics          → affiche les métriques
#
# Affiche :
# - Nombre total de tickets complétés
# - Temps moyen par ticket
# - Cycles review moyens
# - Top 3 des raisons de correction
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/metrics.sh"

# ─────────────────────────────────────────
# DISPLAY HELPERS
# ─────────────────────────────────────────

_metrics_header() {
  echo ""
  echo -e "${BOLD}${CYAN}📊 Métriques de vélocité OpenCode${RESET}"
  echo -e "${DIM}═══════════════════════════════════════${RESET}"
  echo ""
}

_metrics_section() {
  echo -e "${BOLD}${BLUE}$1${RESET}"
}

_metrics_item() {
  printf "  ${DIM}•${RESET}  %-28s " "$1"
  echo -e "$2"
}

# ─────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────

main() {
  local metrics_file
  metrics_file=$(metrics_get_file_path)

  # Vérifier si le fichier existe
  if ! metrics_file_exists; then
    _metrics_header
    echo -e "${YELLOW}⚠${RESET}  Fichier de métriques non trouvé : ${DIM}${metrics_file}${RESET}"
    echo ""
    echo -e "  Les métriques seront collectées automatiquement lors de l'exécution"
    echo -e "  des workflows via l'orchestrator."
    echo ""
    echo -e "  ${DIM}Conseil : utilisez ${RESET}oc start${DIM} pour lancer un workflow et générer des métriques.${RESET}"
    echo ""
    exit 0
  fi

  # Agréger les données
  metrics_aggregate

  # Afficher l'en-tête
  _metrics_header

  # Section : Statistiques générales
  _metrics_section "📈 Statistiques générales"
  echo ""

  if [ "$METRICS_TOTAL_TICKETS" -eq 0 ]; then
    _metrics_item "Tickets complétés" "${DIM}0${RESET}"
    _metrics_item "Temps moyen" "${DIM}—${RESET}"
    _metrics_item "Cycles review moyens" "${DIM}—${RESET}"
  else
    _metrics_item "Tickets complétés" "${GREEN}${METRICS_TOTAL_TICKETS}${RESET}"

    if [ "$METRICS_AVG_DURATION" -gt 0 ]; then
      _metrics_item "Temps moyen par ticket" "${CYAN}${METRICS_AVG_DURATION_FMT}${RESET}"
    else
      _metrics_item "Temps moyen par ticket" "${DIM}—${RESET}"
    fi

    _metrics_item "Cycles review moyens" "${CYAN}${METRICS_AVG_CYCLES}${RESET}"
  fi

  echo ""

  # Section : Top raisons de correction
  _metrics_section "🔧 Top 3 raisons de correction"
  echo ""

  if [ ${#METRICS_TOP_CORRECTIONS[@]} -eq 0 ]; then
    echo -e "  ${DIM}Aucune correction enregistrée${RESET}"
  else
    local rank=1
    for entry in "${METRICS_TOP_CORRECTIONS[@]}"; do
      local reason="${entry%|*}"
      local count="${entry#*|}"

      # Coloration selon le rang
      local color=""
      case $rank in
        1) color="${YELLOW}" ;;
        2) color="${CYAN}" ;;
        3) color="${DIM}" ;;
      esac

      printf "  ${color}%d.${RESET} %-35s ${DIM}— %s fois${RESET}\n" "$rank" "$reason" "$count"
      rank=$((rank + 1))
    done
  fi

  # Section : WebSearch Usage
  if [ "$(metrics_count_websearch)" -gt 0 ]; then
    echo ""
    _metrics_section "🔍 WebSearch Usage"
    echo ""
    
    local ws_count
    ws_count=$(metrics_count_websearch)
    _metrics_item "Total queries" "${CYAN}${ws_count}${RESET}"
    
    # Top query types
    if [ ${#METRICS_TOP_WEBSEARCH_TYPES[@]} -gt 0 ]; then
      echo ""
      echo -e "  ${DIM}Top query types:${RESET}"
      for entry in "${METRICS_TOP_WEBSEARCH_TYPES[@]}"; do
        local type="${entry%|*}"
        local count="${entry#*|}"
        printf "    ${DIM}•${RESET} %-25s ${CYAN}%s${RESET}\n" "$type" "$count"
      done
    fi
  fi

  echo ""
  echo -e "${DIM}───────────────────────────────────────${RESET}"
  echo -e "${DIM}Source : ${metrics_file}${RESET}"
  echo ""
}

main "$@"
