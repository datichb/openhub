#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/worktree.sh — Gestion des git worktrees pour le travail en parallèle
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/worktree.sh"
#   worktree_create "$PROJECT_PATH" "feat/bd-42"
#   worktree_list "$PROJECT_PATH"
#   worktree_remove "$PROJECT_PATH" "feat/bd-42"
#   worktree_cleanup_merged "$PROJECT_PATH"
#
# Les worktrees sont stockés dans .worktrees/<slug>/ à la racine du projet.
# Le répertoire .worktrees/ est automatiquement ajouté à .git/info/exclude.
# Compatible bash 3.2 (macOS).
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_WORKTREE_LOADED:-}" ] && return 0
_WORKTREE_LOADED=1

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
readonly _WORKTREE_DIR=".worktrees"

# ─────────────────────────────────────────
# HELPERS
# ─────────────────────────────────────────

# Transforme un nom de branche en slug compatible filesystem
# feat/bd-42 → feat-bd-42 | a/b/c → a-b-c | simple → simple
# Usage : slug=$(worktree_slug "feat/bd-42")
worktree_slug() {
  local branch="$1"
  # Remplacer / et espaces par -, condenser les - multiples, supprimer - en début/fin
  local slug
  slug=$(echo "$branch" | tr '/[:space:]' '-')
  # Condenser les tirets consécutifs
  while [[ "$slug" == *"--"* ]]; do slug="${slug//--/-}"; done
  # Supprimer les tirets de début et de fin
  slug="${slug#-}"
  slug="${slug%-}"
  echo "$slug"
}

# Retourne le chemin absolu du worktree pour une branche donnée
# Usage : path=$(worktree_get_path "$PROJECT_PATH" "feat/bd-42")
worktree_get_path() {
  local project_path="$1"
  local branch="$2"
  local slug
  slug=$(worktree_slug "$branch")
  echo "${project_path}/${_WORKTREE_DIR}/${slug}"
}

# Vérifie que le répertoire projet est un dépôt git valide
# Usage : _worktree_require_git "$PROJECT_PATH" || return 1
_worktree_require_git() {
  local project_path="$1"
  if [ ! -d "${project_path}/.git" ]; then
    log_error "Pas de dépôt git dans : $project_path"
    return 1
  fi
  return 0
}

# ─────────────────────────────────────────
# FONCTIONS PUBLIQUES
# ─────────────────────────────────────────

# Ajoute .worktrees/ à .git/info/exclude s'il n'y est pas déjà
# Usage : worktree_ensure_exclude "$PROJECT_PATH"
worktree_ensure_exclude() {
  local project_path="$1"
  local excl_dir="${project_path}/.git/info"
  local excl_file="${excl_dir}/exclude"

  _worktree_require_git "$project_path" || return 1

  mkdir -p "$excl_dir"
  # Créer le fichier s'il n'existe pas
  [ -f "$excl_file" ] || touch "$excl_file"

  # Ajouter seulement si absent
  if ! grep -qx "${_WORKTREE_DIR}/" "$excl_file" 2>/dev/null; then
    echo "${_WORKTREE_DIR}/" >> "$excl_file"
    log_info ".worktrees/ ajouté à .git/info/exclude"
  fi
}

# Vérifie si un worktree existe pour une branche donnée
# Usage : if worktree_exists "$PROJECT_PATH" "feat/bd-42"; then ...
# Returns : 0 si existant, 1 sinon
worktree_exists() {
  local project_path="$1"
  local branch="$2"
  local wt_path
  wt_path=$(worktree_get_path "$project_path" "$branch")
  [ -d "$wt_path" ]
}

# Crée un worktree pour une branche donnée
# Si la branche n'existe pas localement, elle est créée (-b)
# Usage : worktree_create "$PROJECT_PATH" "feat/bd-42"
# @param $1 — project_path (required)
# @param $2 — branch : nom de la branche (required)
worktree_create() {
  local project_path="$1"
  local branch="$2"

  [ -z "$project_path" ] && { log_error "worktree_create : project_path requis"; return 1; }
  [ -z "$branch" ]       && { log_error "worktree_create : branch requis"; return 1; }

  _worktree_require_git "$project_path" || return 1

  local wt_path
  wt_path=$(worktree_get_path "$project_path" "$branch")

  # Erreur si le worktree existe déjà
  if [ -d "$wt_path" ]; then
    log_error "Worktree déjà existant : $wt_path"
    return 1
  fi

  # S'assurer que .worktrees/ est exclu du suivi git
  worktree_ensure_exclude "$project_path"

  # Créer le répertoire parent si nécessaire
  mkdir -p "${project_path}/${_WORKTREE_DIR}"

  # Vérifier si la branche existe déjà localement (sans sous-shell pour compatibilité mock)
  local _prev_dir="$PWD"
  cd "$project_path" || return 1
  local branch_exists=false
  if git rev-parse --verify "$branch" >/dev/null 2>&1; then
    branch_exists=true
  fi

  local _git_status=0
  if [ "$branch_exists" = true ]; then
    # Branche existante : git worktree add <path> <branch>
    git worktree add "$wt_path" "$branch" || _git_status=$?
  else
    # Nouvelle branche : git worktree add -b <branch> <path>
    git worktree add -b "$branch" "$wt_path" || _git_status=$?
  fi
  cd "$_prev_dir" || true

  if [ "$_git_status" -ne 0 ]; then
    log_error "Échec de git worktree add pour la branche : $branch"
    return 1
  fi

  log_success "Worktree créé : $wt_path (branche : $branch)"
  echo "$wt_path"
}

# Supprime un worktree pour une branche donnée
# Usage : worktree_remove "$PROJECT_PATH" "feat/bd-42"
# @param $1 — project_path (required)
# @param $2 — branch : nom de la branche (required)
worktree_remove() {
  local project_path="$1"
  local branch="$2"

  [ -z "$project_path" ] && { log_error "worktree_remove : project_path requis"; return 1; }
  [ -z "$branch" ]       && { log_error "worktree_remove : branch requis"; return 1; }

  _worktree_require_git "$project_path" || return 1

  local wt_path
  wt_path=$(worktree_get_path "$project_path" "$branch")

  # Avertissement si le worktree n'existe pas (pas d'erreur fatale)
  if [ ! -d "$wt_path" ]; then
    log_warn "Worktree introuvable : $wt_path"
    return 0
  fi

  # Supprimer le worktree (sans sous-shell pour compatibilité mock)
  local _prev_dir="$PWD"
  cd "$project_path" || return 1
  local _git_status=0
  git worktree remove --force "$wt_path" || _git_status=$?
  # Nettoyer les métadonnées orphelines
  git worktree prune 2>/dev/null || true
  cd "$_prev_dir" || true

  if [ "$_git_status" -ne 0 ]; then
    log_error "Échec de git worktree remove : $wt_path"
    return 1
  fi

  log_success "Worktree supprimé : $wt_path"
}

# Vérifie si une branche est mergée dans une branche de base
# Usage : if worktree_is_merged "$PROJECT_PATH" "feat/bd-42" "main"; then ...
# Returns : 0 si mergée, 1 sinon
worktree_is_merged() {
  local project_path="$1"
  local branch="$2"
  local base="${3:-main}"

  [ -z "$project_path" ] && return 1
  [ -z "$branch" ]       && return 1

  _worktree_require_git "$project_path" || return 1

  local _prev_dir="$PWD"
  cd "$project_path" || return 1
  local result
  result=$(git branch --merged "$base" 2>/dev/null)
  cd "$_prev_dir" || true
  # Vérifier si la branche apparaît dans la liste
  echo "$result" | grep -qE "^[[:space:]]*${branch}$"
}

# Liste les worktrees actifs du projet (format lisible)
# Affiche : chemin | branche | merged?
# Usage : worktree_list "$PROJECT_PATH" ["base_branch"]
worktree_list() {
  local project_path="$1"
  local base_branch="${2:-main}"

  _worktree_require_git "$project_path" || return 1

  local wt_base="${project_path}/${_WORKTREE_DIR}"

  if [ ! -d "$wt_base" ] || [ -z "$(ls -A "$wt_base" 2>/dev/null)" ]; then
    echo "(aucun worktree actif)"
    return 0
  fi

  local found=0
  local _save_dir="$PWD"
  for wt_path in "$wt_base"/*/; do
    [ -d "$wt_path" ] || continue
    found=1

    # Récupérer la branche du worktree via git
    local branch="?"
    if cd "$wt_path" 2>/dev/null; then
      branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null) || branch="?"
      cd "$_save_dir" 2>/dev/null || true
    fi

    # Vérifier si mergée
    local merged_label=""
    if worktree_is_merged "$project_path" "$branch" "$base_branch" 2>/dev/null; then
      merged_label=" [merged]"
    fi

    printf "  %-40s %s%s\n" "${wt_path#"$project_path/"}" "$branch" "$merged_label"
  done

  if [ "$found" -eq 0 ]; then
    echo "(aucun worktree actif)"
  fi
  return 0
}

# Supprime tous les worktrees dont la branche est mergée dans la base
# Usage : worktree_cleanup_merged "$PROJECT_PATH" ["base_branch"]
# @param $1 — project_path (required)
# @param $2 — base_branch : branche de référence (défaut : main)
worktree_cleanup_merged() {
  local project_path="$1"
  local base_branch="${2:-main}"

  _worktree_require_git "$project_path" || return 1

  local wt_base="${project_path}/${_WORKTREE_DIR}"

  if [ ! -d "$wt_base" ] || [ -z "$(ls -A "$wt_base" 2>/dev/null)" ]; then
    log_info "Aucun worktree à nettoyer"
    return 0
  fi

  local cleaned=0
  local _prev_dir="$PWD"
  for wt_path in "$wt_base"/*/; do
    [ -d "$wt_path" ] || continue

    # Récupérer la branche du worktree (sans sous-shell)
    local branch=""
    if cd "$wt_path" 2>/dev/null; then
      branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
      cd "$_prev_dir" || true
    fi
    [ -z "$branch" ] || [ "$branch" = "HEAD" ] && continue

    if worktree_is_merged "$project_path" "$branch" "$base_branch" 2>/dev/null; then
      log_info "Suppression worktree mergé : $branch"
      cd "$project_path" || continue
      if git worktree remove --force "$wt_path" 2>/dev/null; then
        cleaned=$((cleaned + 1))
      else
        log_warn "Impossible de supprimer : $wt_path"
      fi
      cd "$_prev_dir" || true
    fi
  done

  # Nettoyer les métadonnées orphelines
  cd "$project_path" && git worktree prune 2>/dev/null || true
  cd "$_prev_dir" || true

  if [ "$cleaned" -gt 0 ]; then
    log_success "$cleaned worktree(s) mergé(s) supprimé(s)"
  else
    log_info "Aucun worktree mergé à nettoyer"
  fi
}

# Retourne un résumé des worktrees actifs (pour oc worktree status)
# Usage : worktree_status "$PROJECT_PATH" ["base_branch"]
worktree_status() {
  local project_path="$1"
  local base_branch="${2:-main}"

  _worktree_require_git "$project_path" || return 1

  local wt_base="${project_path}/${_WORKTREE_DIR}"
  local total=0
  local merged=0
  local _prev_dir="$PWD"

  if [ -d "$wt_base" ]; then
    for wt_path in "$wt_base"/*/; do
      [ -d "$wt_path" ] || continue
      total=$((total + 1))
      local branch=""
      if cd "$wt_path" 2>/dev/null; then
        branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
        cd "$_prev_dir" || true
      fi
      if [ -n "$branch" ] && worktree_is_merged "$project_path" "$branch" "$base_branch" 2>/dev/null; then
        merged=$((merged + 1))
      fi
    done
  fi

  echo "Worktrees actifs : $total"
  echo "Worktrees mergés : $merged"
  if [ "$merged" -gt 0 ]; then
    echo "→ Lancer 'oc worktree cleanup' pour supprimer les worktrees mergés"
  fi
}
