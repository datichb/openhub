#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/progress-bar.sh — Barres de progression modernes avec récapitulatifs
# ─────────────────────────────────────────────────────────────────────────────
# Compatible bash 3.2+ (macOS)
#
# Usage :
#   source "$LIB_DIR/progress-bar.sh"
#
#   # Afficher une progression
#   _progress_bar 5 10 "item-name"
#
#   # Finaliser la progression
#   _progress_done
#
#   # Afficher un récapitulatif
#   _progress_summary "Phase terminée" \
#     "30 items traités" \
#     "Details supplémentaires" \
#     "  - Sous-item indenté"

_PROGRESS_ENABLED=false

# Détection automatique : stdout est un TTY
if [ -t 1 ]; then
  _PROGRESS_ENABLED=true
fi

# Désactive la progression (pour --no-progress)
_progress_disable() {
  _PROGRESS_ENABLED=false
}

# Affiche/met à jour une barre de progression sur une seule ligne
# $1 = current (numéro actuel, 1-based)
# $2 = total (nombre total d'items)
# $3 = label (texte à afficher, optionnel)
# $4 = status (optionnel: "error" pour affichage en rouge avec ✗)
_progress_bar() {
  # Skip si progression désactivée
  [ "$_PROGRESS_ENABLED" != true ] && return 0
  
  local current="$1"
  local total="$2"
  local label="${3:-}"
  local status="${4:-}"
  
  # Calculer le pourcentage
  local percent=0
  [ "$total" -gt 0 ] && percent=$(( current * 100 / total ))
  
  # Largeur de la barre (20 caractères)
  local bar_width=20
  local filled=$(( percent * bar_width / 100 ))
  local empty=$(( bar_width - filled ))
  
  # Construire la barre avec caractères Unicode
  local bar=""
  local i=0
  while [ "$i" -lt "$filled" ]; do 
    bar="${bar}█"
    i=$((i + 1))
  done
  i=0
  while [ "$i" -lt "$empty" ]; do 
    bar="${bar}░"
    i=$((i + 1))
  done
  
  # Coloration selon le status
  local color="${CYAN}"
  local suffix=""
  if [ "$status" = "error" ]; then
    color="${RED}"
    suffix=" ${RED}✗${RESET}"
  fi
  
  # Afficher avec \r pour écraser la ligne précédente
  # Format: "    [████████████░░░░░░░░] 75% (15/20) agent-name"
  printf "\r\033[2K    ${color}[${bar}]${RESET} ${BOLD}%3d%%${RESET} (%d/%d) ${DIM}%s${RESET}%s" \
    "$percent" "$current" "$total" "$label" "$suffix"
}

# Finalise la barre de progression (nouvelle ligne)
_progress_done() {
  [ "$_PROGRESS_ENABLED" != true ] && return 0
  echo ""  # Nouvelle ligne
}

# Affiche un récapitulatif structuré après une phase
# $1 = title (titre du récapitulatif, ex: "Phase 1 terminée")
# $@ = lignes du récapitulatif (une par argument)
#
# Format de sortie :
#   ✅ Phase 1 terminée
#      · Ligne 1
#      · Ligne 2
#        - Sous-item (commence par espace)
_progress_summary() {
  local title="$1"
  shift
  
  echo ""
  echo -e "    ${GREEN}✅${RESET} ${BOLD}${title}${RESET}"
  
  # Afficher chaque ligne du résumé
  while [ $# -gt 0 ]; do
    local line="$1"
    
    # Si la ligne commence par un espace, c'est un sous-item (indenté)
    if [[ "$line" =~ ^[[:space:]] ]]; then
      echo -e "      ${DIM}${line}${RESET}"
    else
      echo -e "       ${BLUE}·${RESET} ${line}"
    fi
    shift
  done
}
