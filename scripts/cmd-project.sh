#!/bin/bash
# Gestion des projets enregistrés : rename, move, configure
# Usage :
#   oc project rename <OLD_ID> <NEW_ID>     — renomme un projet dans tous les fichiers registre
#   oc project move <PROJECT_ID> <path>     — change le chemin local d'un projet
#   oc project configure [PROJECT_ID]       — reconfigure les champs d'un projet existant
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
resolve_oc_lang
source "$LIB_DIR/project.sh"

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
  _prompt confirm "  Confirmer le renommage ? [y/N] : "
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
    _prompt cont "  Continuer quand même ? [y/N] : "
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
  _prompt confirm "  Confirmer ? [y/N] : "
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
# Subcommande : oc project configure [PROJECT_ID]
# Reconfigure les champs d'un projet existant dans projects.md
# ────────────────────────────────────────────────────────────────────────────────
cmd_configure() {
  local project_id="${1:-}"

  # ── Résolution du projet ────────────────────────────────────────────────────
  if [ -z "$project_id" ]; then
    # Lister les projets disponibles pour sélection interactive
    local project_ids
    project_ids=$(grep '^## ' "$PROJECTS_FILE" 2>/dev/null | sed 's/^## //' | grep -v '^$' || true)
    if [ -z "$project_ids" ]; then
      log_error "Aucun projet trouvé dans projects.md"
      exit 1
    fi
    echo ""
    log_title "Configurer un projet"
    echo ""
    echo "  Projets disponibles :"
    local i=1
    local ids_array=()
    while IFS= read -r pid; do
      printf "    %d) %s\n" "$i" "$pid"
      ids_array+=("$pid")
      i=$((i + 1))
    done <<< "$project_ids"
    echo ""
    _prompt choice "  Numéro du projet : "
    local idx="${choice:-0}"
    if ! [[ "$idx" =~ ^[0-9]+$ ]] || [ "$idx" -lt 1 ] || [ "$idx" -gt "${#ids_array[@]}" ]; then
      log_error "Choix invalide : $idx"
      exit 1
    fi
    project_id="${ids_array[$((idx - 1))]}"
  fi

  project_id=$(normalize_project_id "$project_id")

  if ! project_exists "$project_id"; then
    log_error "Projet introuvable : $project_id"
    exit 1
  fi

  # ── Lecture des valeurs courantes ────────────────────────────────────────────
  local cur_stack cur_tracker cur_labels cur_language
  local cur_agents cur_disable cur_mcp
  local cur_wt_enabled cur_wt_auto_cleanup cur_wt_base_branch

  cur_stack=$(_get_project_field "$project_id" "Stack")
  cur_tracker=$(get_project_tracker "$project_id")
  cur_labels=$(get_project_labels "$project_id")
  cur_language=$(get_project_language "$project_id")
  cur_agents=$(get_project_agents "$project_id")
  cur_disable=$(get_project_disabled_native_agents "$project_id")
  cur_mcp=$(get_project_mcp "$project_id")
  cur_wt_enabled=$(get_project_worktree_enabled "$project_id")
  cur_wt_auto_cleanup=$(get_project_worktree_auto_cleanup "$project_id")
  cur_wt_base_branch=$(get_project_worktree_base_branch "$project_id")

  echo ""
  log_title "Configurer le projet : $project_id"
  echo ""
  echo -e "  ${DIM}Appuyer sur Entrée pour conserver la valeur actuelle.${RESET}"
  echo ""

  # ── Stack ────────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Stack${RESET}"
  printf "  Valeur actuelle : %s\n" "${cur_stack:-N/A}"
  _prompt new_stack "  Nouvelle valeur (Entrée = conserver) : "
  if [ -n "${new_stack:-}" ]; then
    _set_project_stack "$project_id" "$new_stack" \
      && log_success "Stack mis à jour : $new_stack"
  fi
  echo ""

  # ── Tracker ──────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Tracker externe${RESET}"
  printf "  Valeur actuelle : %s\n" "$cur_tracker"
  echo "    1) none"
  echo "    2) jira"
  echo "    3) gitlab"
  _prompt tracker_choice "  Choix (Entrée = conserver) : "
  if [ -n "${tracker_choice:-}" ]; then
    local new_tracker
    case "$tracker_choice" in
      1) new_tracker="none" ;;
      2) new_tracker="jira" ;;
      3) new_tracker="gitlab" ;;
      none|jira|gitlab) new_tracker="$tracker_choice" ;;
      *) log_warn "Choix invalide ignoré : $tracker_choice" ; new_tracker="" ;;
    esac
    if [ -n "$new_tracker" ] && [ "$new_tracker" != "$cur_tracker" ]; then
      _set_project_tracker "$project_id" "$new_tracker" \
        && log_success "Tracker mis à jour : $new_tracker"
    fi
  fi
  echo ""

  # ── Labels ───────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Labels Beads${RESET} (CSV, ex: feature,fix,front,back)"
  printf "  Valeur actuelle : %s\n" "${cur_labels:-}"
  _prompt new_labels "  Nouvelle valeur (Entrée = conserver) : "
  if [ -n "${new_labels:-}" ]; then
    _set_project_labels "$project_id" "$new_labels" \
      && log_success "Labels mis à jour : $new_labels"
  fi
  echo ""

  # ── Langue ───────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Langue de travail des agents${RESET}"
  printf "  Valeur actuelle : %s\n" "${cur_language:-français (défaut)}"
  echo -e "  ${DIM}Exemples : english, spanish — laisser vide pour revenir au français${RESET}"
  _prompt new_language "  Nouvelle valeur (Entrée = conserver) : "
  if [ -n "${new_language:-}" ]; then
    local norm_lang
    norm_lang=$(echo "$new_language" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
    _set_project_language "$project_id" "$norm_lang" \
      && log_success "Langue mise à jour : $norm_lang"
  fi
  echo ""

  # ── Disable agents ───────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Agents natifs OpenCode à désactiver${RESET}"
  echo -e "  ${DIM}Valeurs possibles : build, plan, general, explore, scout${RESET}"
  printf "  Valeur actuelle : %s\n" "${cur_disable:-aucun (défaut hub)}"
  _prompt new_disable "  Nouvelle valeur CSV (Entrée = conserver, 'none' pour vider) : "
  if [ -n "${new_disable:-}" ]; then
    local val_disable="$new_disable"
    [ "$val_disable" = "none" ] && val_disable=""
    _set_project_disabled_native_agents "$project_id" "$val_disable" \
      && log_success "Disable agents mis à jour : ${val_disable:-<vide>}"
  fi
  echo ""

  # ── MCP ──────────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Serveurs MCP activés${RESET}"
  echo -e "  ${DIM}Valeurs : all | none | CSV (ex: figma-mcp,gitlab-mcp)${RESET}"
  printf "  Valeur actuelle : %s\n" "$cur_mcp"
  _prompt new_mcp "  Nouvelle valeur (Entrée = conserver) : "
  if [ -n "${new_mcp:-}" ]; then
    _set_project_mcp "$project_id" "$new_mcp" \
      && log_success "MCP mis à jour : $new_mcp"
  fi
  echo ""

  # ── Worktrees ────────────────────────────────────────────────────────────────
  echo -e "  ${BOLD}Git Worktrees${RESET}"
  printf "  Statut actuel : %s\n" "$cur_wt_enabled"
  _prompt wt_choice "  Activer les worktrees ? [enabled/disabled] (Entrée = conserver) : "
  if [ -n "${wt_choice:-}" ]; then
    local norm_wt
    norm_wt=$(echo "$wt_choice" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
    case "$norm_wt" in
      enabled|disabled)
        if [ "$norm_wt" != "$cur_wt_enabled" ]; then
          _set_project_worktree_enabled "$project_id" "$norm_wt" \
            && log_success "Worktrees : $norm_wt"
          cur_wt_enabled="$norm_wt"
        fi
        ;;
      y|yes|oui) norm_wt="enabled"
        _set_project_worktree_enabled "$project_id" "enabled" \
          && log_success "Worktrees : enabled"
        cur_wt_enabled="enabled"
        ;;
      n|no|non) norm_wt="disabled"
        _set_project_worktree_enabled "$project_id" "disabled" \
          && log_success "Worktrees : disabled"
        cur_wt_enabled="disabled"
        ;;
      *) log_warn "Valeur ignorée : $wt_choice (attendu : enabled | disabled)" ;;
    esac
  fi

  if [ "$cur_wt_enabled" = "enabled" ]; then
    # Auto-cleanup
    printf "  Auto-cleanup actuel : %s\n" "$cur_wt_auto_cleanup"
    _prompt wt_cleanup "  Activer le nettoyage automatique des worktrees mergés ? [true/false] (Entrée = conserver) : "
    if [ -n "${wt_cleanup:-}" ]; then
      local norm_cleanup
      norm_cleanup=$(echo "$wt_cleanup" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
      case "$norm_cleanup" in
        true|false)
          _set_project_worktree_auto_cleanup "$project_id" "$norm_cleanup" \
            && log_success "Worktree auto cleanup : $norm_cleanup"
          ;;
        y|yes|oui) _set_project_worktree_auto_cleanup "$project_id" "true" \
            && log_success "Worktree auto cleanup : true" ;;
        n|no|non)  _set_project_worktree_auto_cleanup "$project_id" "false" \
            && log_success "Worktree auto cleanup : false" ;;
        *) log_warn "Valeur ignorée : $wt_cleanup" ;;
      esac
    fi

    # Base branch
    printf "  Branche de base actuelle : %s\n" "$cur_wt_base_branch"
    _prompt wt_branch "  Branche de base (Entrée = conserver) : "
    if [ -n "${wt_branch:-}" ]; then
      _set_project_worktree_base_branch "$project_id" "$wt_branch" \
        && log_success "Worktree base branch : $wt_branch"
    fi

    # S'assurer que .worktrees/ est dans .git/info/exclude
    local project_path
    project_path=$(get_project_path "$project_id" 2>/dev/null || true)
    if [ -n "$project_path" ] && [ -d "${project_path}/.git" ]; then
      source "$LIB_DIR/worktree.sh" 2>/dev/null || true
      worktree_ensure_exclude "$project_path" 2>/dev/null || true
    fi
  fi
  echo ""

  log_success "Configuration de $project_id mise à jour"
  echo ""
  log_info "Pour redéployer les agents : ./oc.sh deploy all $project_id"
  echo ""
}

# ────────────────────────────────────────────────────────────────────────────────
# Main dispatcher — only runs when executed directly (not sourced)
# ────────────────────────────────────────────────────────────────────────────────
[[ "${BASH_SOURCE[0]}" != "$0" ]] && return 0

SUBCOMMAND="${1:-}"
case "$SUBCOMMAND" in
  rename)    cmd_rename    "${@:2}" ;;
  move)      cmd_move      "${@:2}" ;;
  configure) cmd_configure "${@:2}" ;;
  *)
    echo -e "${BOLD}oc project — Gestion des projets${RESET}"
    echo ""
    echo "  project rename <OLD_ID> <NEW_ID>      Renomme un projet dans tous les registres"
    echo "  project move <PROJECT_ID> <path>      Change le chemin local d'un projet"
    echo "  project configure [PROJECT_ID]        Reconfigure les champs d'un projet existant"
    echo ""
    echo -e "${BOLD}Exemples :${RESET}"
    echo "  ./oc.sh project rename MY-APP MY-APP-V2"
    echo "  ./oc.sh project move MY-APP ~/workspace/my-app-new"
    echo "  ./oc.sh project configure MY-APP"
    echo "  ./oc.sh project configure"
    echo ""
    [ -n "$SUBCOMMAND" ] && { log_error "Sous-commande inconnue : $SUBCOMMAND"; exit 1; }
    exit 0
    ;;
esac
