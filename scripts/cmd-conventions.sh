#!/bin/bash
# oc conventions [PROJECT_ID]
# Détecte et documente les conventions d'un projet dans CONVENTIONS.md
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/prompt-builder.sh"

ensure_projects_file

# ── Parsing des arguments ──────────────────────────────────────────────────────
FORCE=false
PROJECT_ID=""
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --project|-p) PROJECT_ID="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --force|-f)   FORCE=true ;;
    --project|-p) _prev="$arg" ;;
  esac
done

# ── Sélection interactive si pas d'ID ─────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_error "Aucun projet enregistré → ./oc.sh init"
    exit 1
  fi

  _intro "Conventions — choisir un projet"
  echo ""
  for i in "${!ids[@]}"; do
    printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
  done
  echo ""
  read -rp "  Numéro : " choice
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
    log_error "Choix invalide : $choice (attendu 1-${#ids[@]})"
    exit 1
  fi
  PROJECT_ID="${ids[$((choice-1))]}"
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Validation opencode ────────────────────────────────────────────────────────
load_adapter
adapter_validate || {
  log_error "opencode non disponible → oc install"
  exit 1
}

# ── Vérifier si CONVENTIONS.md existe déjà ────────────────────────────────────
CONVENTIONS_FILE="$PROJECT_PATH/CONVENTIONS.md"
_intro "Conventions — ${PROJECT_ID}"
printf "${DIM}│${RESET}  %-10s %s\n" "Chemin" "$PROJECT_PATH"
echo -e "${DIM}│${RESET}"

if [ -f "$CONVENTIONS_FILE" ] && [ "$FORCE" = false ]; then
  # Lire la date de génération depuis le fichier existant
  _existing_date=$(grep -m1 "^> Généré le" "$CONVENTIONS_FILE" 2>/dev/null | sed 's/^> Généré le //' | cut -d' ' -f1 || echo "date inconnue")
  log_warn "CONVENTIONS.md existe déjà (généré le ${_existing_date})"
  echo -e "${DIM}│${RESET}"
  read -rp "  Écraser et regénérer ? [y/N] : " _overwrite
  if [[ ! "${_overwrite:-N}" =~ ^[Yy]$ ]]; then
    log_info "Opération annulée — CONVENTIONS.md conservé"
    log_info "Pour forcer : ./oc.sh conventions ${PROJECT_ID} --force"
    echo ""
    exit 0
  fi
fi

# ── Construire le prompt de bootstrap ─────────────────────────────────────────
PROMPT=$(build_conventions_bootstrap_prompt "$PROJECT_PATH" "$PROJECT_ID" "$HUB_DIR")
AGENT_NAME="onboarder"

log_info "Détection des conventions du projet via l'agent ${AGENT_NAME}…"
echo -e "${DIM}│${RESET}"
echo -e "${DIM}│${RESET}  L'agent va :"
echo -e "${DIM}│${RESET}    1. Explorer la config linting/formatting, tsconfig, package.json"
echo -e "${DIM}│${RESET}    2. Analyser le nommage et la structure depuis la codebase"
echo -e "${DIM}│${RESET}    3. Lire les configs Git et de test"
echo -e "${DIM}│${RESET}    4. Écrire CONVENTIONS.md à la racine du projet"
echo -e "${DIM}│${RESET}"

_outro "Lancement de opencode…"
_prompt _ ""

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "$AGENT_NAME"
