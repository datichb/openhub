#!/bin/bash
# Gestion des projets enregistrés : rename, move
# Usage :
#   oc project rename <OLD_ID> <NEW_ID>   — renomme un projet dans tous les fichiers registre
#   oc project move <PROJECT_ID> <path>   — change le chemin local d'un projet
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
resolve_oc_lang

# ────────────────────────────────────────────────────────────────────────────────
# Subcommande : oc project rename <OLD_ID> <NEW_ID>
# Renomme un projet dans projects.md, paths.local.md, api-keys.local.md
# ────────────────────────────────────────────────────────────────────────────────
cmd_rename() {
  local old_id="${1:-}" new_id="${2:-}"

  [ -z "$old_id" ] && { log_error "Usage : oc project rename <OLD_ID> <NEW_ID>"; exit 1; }
  [ -z "$new_id" ] && { log_error "Usage : oc project rename <OLD_ID> <NEW_ID>"; exit 1; }

  old_id=$(normalize_project_id "$old_id")
  new_id=$(normalize_project_id "$new_id")

  if [ "$old_id" = "$new_id" ]; then
    log_warn "Les deux identifiants sont identiques : $old_id"
    exit 0
  fi

  if ! project_exists "$old_id"; then
    log_error "Projet introuvable : $old_id"
    exit 1
  fi

  if project_exists "$new_id"; then
    log_error "Un projet avec l'identifiant $new_id existe déjà"
    exit 1
  fi

  command -v perl &>/dev/null || { log_error "perl requis pour cette opération"; exit 1; }

  echo ""
  log_title "Renommer un projet"
  echo ""
  printf "  %-16s %s\n" "Ancien ID :" "$old_id"
  printf "  %-16s %s\n" "Nouvel ID :" "$new_id"
  echo ""
  read -rp "  Confirmer le renommage ? [y/N] : " confirm
  [[ "${confirm:-N}" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)"; exit 0; }

  local changed=0

  # ── projects.md : renommer l'en-tête de section ──────────────────────────
  if [ -f "$PROJECTS_FILE" ] && grep -q "^## ${old_id}$" "$PROJECTS_FILE"; then
    perl -i -0777pe "s{^## \Q${old_id}\E$}{## ${new_id}}mg" "$PROJECTS_FILE"
    log_success "projects.md : $old_id → $new_id"
    changed=$((changed + 1))
  fi

  # ── paths.local.md : renommer la clé ────────────────────────────────────
  if [ -f "$PATHS_FILE" ] && grep -q "^${old_id}=" "$PATHS_FILE"; then
    perl -i -pe "s{^\Q${old_id}\E=}{${new_id}=}" "$PATHS_FILE"
    log_success "paths.local.md : $old_id → $new_id"
    changed=$((changed + 1))
  fi

  # ── api-keys.local.md : renommer la section ──────────────────────────────
  if [ -f "$API_KEYS_FILE" ] && grep -q "^\[${old_id}\]" "$API_KEYS_FILE"; then
    perl -i -pe "s{^\[${old_id}\]}{[${new_id}]}" "$API_KEYS_FILE"
    log_success "api-keys.local.md : [$old_id] → [$new_id]"
    changed=$((changed + 1))
  fi

  echo ""
  if [ "$changed" -gt 0 ]; then
    log_success "Projet renommé : $old_id → $new_id"
    echo ""
    log_info "Si des agents sont déployés dans le projet, relancer : ./oc.sh deploy all ${new_id}"
  else
    log_warn "Aucun fichier modifié (projet non trouvé dans les registres)"
  fi
  echo ""
}

# ────────────────────────────────────────────────────────────────────────────────
# Subcommande : oc project move <PROJECT_ID> <path>
# Change le chemin local d'un projet dans paths.local.md
# ────────────────────────────────────────────────────────────────────────────────
cmd_move() {
  local project_id="${1:-}" new_path="${2:-}"

  [ -z "$project_id" ] && { log_error "Usage : oc project move <PROJECT_ID> <path>"; exit 1; }
  [ -z "$new_path"   ] && { log_error "Usage : oc project move <PROJECT_ID> <path>"; exit 1; }

  project_id=$(normalize_project_id "$project_id")

  if ! project_exists "$project_id"; then
    log_error "Projet introuvable : $project_id"
    exit 1
  fi

  # Résoudre ~ et path relatifs
  new_path="${new_path/#\~/$HOME}"
  if [[ "$new_path" != /* ]]; then
    new_path="$(pwd)/$new_path"
  fi

  # Vérifier que le nouveau chemin existe
  if [ ! -d "$new_path" ]; then
    log_warn "Le dossier n'existe pas encore : $new_path"
    read -rp "  Continuer quand même ? [y/N] : " cont
    [[ "${cont:-N}" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)"; exit 0; }
  fi

  # Chemin actuel pour affichage
  local current_path; current_path=$(get_project_path "$project_id" 2>/dev/null || echo "(non défini)")

  echo ""
  log_title "Déplacer un projet"
  echo ""
  printf "  %-18s %s\n" "Projet :"         "$project_id"
  printf "  %-18s %s\n" "Chemin actuel :"  "$current_path"
  printf "  %-18s %s\n" "Nouveau chemin :" "$new_path"
  echo ""
  read -rp "  Confirmer ? [y/N] : " confirm
  [[ "${confirm:-N}" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)"; exit 0; }

  # Mettre à jour paths.local.md
  mkdir -p "$(dirname "$PATHS_FILE")"
  if [ -f "$PATHS_FILE" ] && grep -q "^${project_id}=" "$PATHS_FILE"; then
    # Remplacer la ligne existante
    perl -i -pe "s{^\Q${project_id}\E=.*}{${project_id}=${new_path}}" "$PATHS_FILE"
  else
    # Ajouter la ligne
    echo "${project_id}=${new_path}" >> "$PATHS_FILE"
  fi

  echo ""
  log_success "Chemin mis à jour pour $project_id : $new_path"
  echo ""
  log_info "Si des agents doivent être (re)déployés : ./oc.sh deploy all ${project_id}"
  echo ""
}

# ────────────────────────────────────────────────────────────────────────────────
# Main dispatcher
# ────────────────────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-}"
case "$SUBCOMMAND" in
  rename) cmd_rename "${@:2}" ;;
  move)   cmd_move   "${@:2}" ;;
  *)
    echo -e "${BOLD}oc project — Gestion des projets${RESET}"
    echo ""
    echo "  project rename <OLD_ID> <NEW_ID>   Renomme un projet dans tous les registres"
    echo "  project move <PROJECT_ID> <path>   Change le chemin local d'un projet"
    echo ""
    echo -e "${BOLD}Exemples :${RESET}"
    echo "  ./oc.sh project rename MY-APP MY-APP-V2"
    echo "  ./oc.sh project move MY-APP ~/workspace/my-app-new"
    echo ""
    [ -n "$SUBCOMMAND" ] && { log_error "Sous-commande inconnue : $SUBCOMMAND"; exit 1; }
    exit 0
    ;;
esac
