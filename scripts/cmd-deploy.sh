#!/bin/bash
# Déploie les agents canoniques vers les cibles configurées.
# Usage : oc deploy [target] [PROJECT_ID] [--check] [--diff]
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/spinner.sh"

# ── Mode --check ─────────────────────────────────────────────────────────────
# Vérifie si les fichiers générés sont à jour par rapport aux sources.
# Usage : oc deploy --check [target] [PROJECT_ID]
_cmd_deploy_check() {
  local target="${1:-all}" project_id="${2:-}"

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
    deploy_dir=$(resolve_project_path "$project_id")
  fi

  # Résoudre les cibles à vérifier
  local targets=()
  if [ -z "$target" ] || [ "$target" = "all" ]; then
    while IFS= read -r t; do [ -n "$t" ] && targets+=("$t"); done < <(get_active_targets)
  else
    targets=("$target")
  fi

  log_title "Vérification de fraîcheur des agents déployés"
  local stale_count=0
  local ok_count=0

  source "$LIB_DIR/prompt-builder.sh"

  if [ "${#targets[@]}" -eq 0 ]; then
    log_warn "Aucune cible configurée — vérifier active_targets dans config/hub.json"
    return 0
  fi

  for tgt in "${targets[@]}"; do
    log_info "── Cible : $tgt"

    # Répertoire des fichiers générés selon la cible
    local gen_dir=""
    case "$tgt" in
      opencode)     gen_dir="$deploy_dir/.opencode/agents" ;;
      claude-code)  gen_dir="$deploy_dir/.claude/agents" ;;
      *)            log_warn "Cible inconnue pour --check : $tgt"; continue ;;
    esac

    # Pour chaque agent source, trouver le fichier généré correspondant
    while IFS= read -r agent_file; do
      [ -f "$agent_file" ] || continue

      # Vérifier si l'agent supporte cette cible
      agent_supports_target "$agent_file" "$tgt" || continue

      local agent_id; agent_id=$(get_agent_id "$agent_file")

      # Filtrer les agents non sélectionnés pour ce projet
      should_deploy_agent "$project_id" "$agent_id" || continue

      # Nom du fichier généré selon la cible
      local gen_file=""
      case "$tgt" in
        opencode|claude-code) gen_file="$gen_dir/${agent_id}.md" ;;
      esac

      if [ ! -f "$gen_file" ]; then
        echo -e "  ${RED}✗ MANQUANT${RESET}  $agent_id → $(basename "$gen_file")"
        stale_count=$((stale_count + 1))
        continue
      fi

      # Timestamp du fichier généré
      local gen_mtime; gen_mtime=$(stat -f %m "$gen_file" 2>/dev/null || stat -c %Y "$gen_file" 2>/dev/null)

      # Timestamp le plus récent parmi : agent source + tous ses skills
      local max_src_mtime=0
      local stale_reason=""

      local agent_mtime; agent_mtime=$(stat -f %m "$agent_file" 2>/dev/null || stat -c %Y "$agent_file" 2>/dev/null)
      [ "$agent_mtime" -gt "$max_src_mtime" ] && max_src_mtime=$agent_mtime

      # Vérifier chaque skill de l'agent
      while IFS= read -r skill; do
        [ -z "$skill" ] && continue
        local skill_file="$SKILLS_DIR/${skill}.md"
        [ -f "$skill_file" ] || continue
        local skill_mtime; skill_mtime=$(stat -f %m "$skill_file" 2>/dev/null || stat -c %Y "$skill_file" 2>/dev/null)
        if [ "$skill_mtime" -gt "$max_src_mtime" ]; then
          max_src_mtime=$skill_mtime
          stale_reason="skill: $skill"
        fi
      done < <(extract_frontmatter_list "$agent_file" "skills")

      if [ "$agent_mtime" -gt "$gen_mtime" ] && [ "$max_src_mtime" -eq "$agent_mtime" ]; then
        stale_reason="agent source modifié"
      fi

      if [ "$max_src_mtime" -gt "$gen_mtime" ]; then
        echo -e "  ${YELLOW}⚠ OBSOLÈTE${RESET}  $agent_id → $(basename "$gen_file")  (${stale_reason:-source modifié})"
        stale_count=$((stale_count + 1))
      else
        echo -e "  ${GREEN}✓ À JOUR${RESET}    $agent_id → $(basename "$gen_file")"
        ok_count=$((ok_count + 1))
      fi
    done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

    echo ""
  done

  echo -e "Résultat : ${GREEN}$ok_count à jour${RESET}  |  ${stale_count:+${YELLOW}}$stale_count obsolète(s)/manquant(s)${RESET}"
  if [ "$stale_count" -gt 0 ]; then
    echo ""
    log_info "Régénérer : ./oc.sh deploy${project_id:+ all $project_id}"
    exit 1
  fi
}

# ── Mode --diff ───────────────────────────────────────────────────────────────
# Compare le contenu qui serait généré avec les fichiers déployés actuels.
# Affiche le diff complet pour les agents modifiés, "(inchangé)" pour les autres,
# "(nouveau)" si pas encore déployé.
# Propose ensuite d'appliquer le déploiement si des différences sont trouvées.
_cmd_deploy_diff() {
  local target="${1:-all}" project_id="${2:-}"

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
    deploy_dir=$(resolve_project_path "$project_id")
  fi

  # Résoudre les cibles
  local targets=()
  if [ -z "$target" ] || [ "$target" = "all" ]; then
    while IFS= read -r t; do [ -n "$t" ] && targets+=("$t"); done < <(get_active_targets)
  else
    targets=("$target")
  fi

  source "$LIB_DIR/prompt-builder.sh"

  log_title "Diff des agents (sources → déployés)"
  local changed_count=0
  local new_count=0
  local same_count=0

  if [ "${#targets[@]}" -eq 0 ]; then
    log_warn "Aucune cible configurée — vérifier active_targets dans config/hub.json"
    return 0
  fi

  for tgt in "${targets[@]}"; do
    log_info "── Cible : $tgt"
    echo ""

    local gen_dir=""
    case "$tgt" in
      opencode)    gen_dir="$deploy_dir/.opencode/agents" ;;
      claude-code) gen_dir="$deploy_dir/.claude/agents" ;;
      *)           log_warn "Cible inconnue pour --diff : $tgt"; continue ;;
    esac

    # Langue du projet si disponible
    local lang=""
    [ -n "$project_id" ] && lang=$(get_project_language "$project_id" 2>/dev/null || true)
    lang=$(resolve_agent_lang "$lang")

    while IFS= read -r agent_file; do
      [ -f "$agent_file" ] || continue
      agent_supports_target "$agent_file" "$tgt" || continue

      local agent_id; agent_id=$(get_agent_id "$agent_file")
      should_deploy_agent "$project_id" "$agent_id" || continue

      local gen_file=""
      case "$tgt" in
        opencode|claude-code) gen_file="$gen_dir/${agent_id}.md" ;;
      esac

      # Générer le contenu cible dans un fichier temporaire
      local tmpfile; tmpfile=$(mktemp /tmp/oc-diff-XXXXXX.md)
      local diff_deploy_dir="$deploy_dir"
      if ! build_agent_content "$agent_file" "$tgt" "$lang" "$diff_deploy_dir" > "$tmpfile" 2>/dev/null; then
        rm -f "$tmpfile"
        log_warn "  Génération échouée pour $agent_id — ignoré"
        continue
      fi

      if [ ! -f "$gen_file" ]; then
        echo -e "  ${GREEN}+ $agent_id${RESET}  (nouveau)"
        new_count=$((new_count + 1))
      elif diff -q "$gen_file" "$tmpfile" > /dev/null 2>&1; then
        echo -e "  ${BLUE}= $agent_id${RESET}  (inchangé)"
        same_count=$((same_count + 1))
      else
        echo -e "  ${YELLOW}~ $agent_id${RESET}  (modifié)"
        diff --unified=3 "$gen_file" "$tmpfile" 2>/dev/null | tail -n +3 \
          | sed 's/^+/    \x1b[32m+\x1b[0m/;s/^-/    \x1b[31m-\x1b[0m/;s/^ /    /' || true
        changed_count=$((changed_count + 1))
      fi
      rm -f "$tmpfile"
    done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
    echo ""
  done

  # ── Résumé ─────────────────────────────────────────────────────────────────
  echo -e "Résumé : ${GREEN}${new_count} nouveau(x)${RESET}  ${YELLOW}${changed_count} modifié(s)${RESET}  ${BLUE}${same_count} inchangé(s)${RESET}"
  echo ""

  local total_changes=$(( new_count + changed_count ))
  if [ "$total_changes" -eq 0 ]; then
    log_info "Aucun changement détecté."
    return 0
  fi

  read -rp "  Appliquer le déploiement ? [Y/n] : " apply_answer
  apply_answer="${apply_answer:-Y}"
  if [[ "$apply_answer" =~ ^[Yy]$ ]]; then
    bash "$HUB_DIR/oc.sh" deploy "${target:-all}" ${project_id:+"$project_id"}
  else
    log_info "Déploiement annulé."
  fi
}

# ── Dispatch : --check, --diff ou déploiement normal ─────────────────────────
# Analyser les arguments pour détecter --check / --diff (peut être en 1ère ou 2ème position)
CHECK_MODE=false
DIFF_MODE=false
REMAINING_ARGS=()
for arg in "$@"; do
  if [ "$arg" = "--check" ]; then
    CHECK_MODE=true
  elif [ "$arg" = "--diff" ]; then
    DIFF_MODE=true
  else
    REMAINING_ARGS+=("$arg")
  fi
done

TARGET="${REMAINING_ARGS[0]:-}"
PROJECT_ID="${REMAINING_ARGS[1]:-}"

if [ "$CHECK_MODE" = true ]; then
  _cmd_deploy_check "$TARGET" "$PROJECT_ID"
  exit 0
fi

if [ "$DIFF_MODE" = true ]; then
  _cmd_deploy_diff "$TARGET" "$PROJECT_ID"
  exit 0
fi

log_title "Déploiement des agents"

# Résoudre les cibles : CLI > projet > global (hub.json)
if [ -n "$TARGET" ] && [ "$TARGET" != "all" ]; then
  # Cible explicite passée en argument CLI → priorité maximale
  targets=("$TARGET")
elif [ -n "$PROJECT_ID" ]; then
  # Vérifier si le projet a des cibles configurées
  _proj_id_norm=$(normalize_project_id "$PROJECT_ID")
  _proj_targets=$(get_project_targets "$_proj_id_norm")
  if [ -n "$_proj_targets" ] && [ "$_proj_targets" != "all" ]; then
    # Cibles du projet → override des cibles globales
    targets=()
    while IFS=',' read -ra _t; do
      for _tgt in "${_t[@]}"; do
        _tgt=$(echo "$_tgt" | tr -d '\r' | sed 's/^ *//;s/ *$//')
        [ -n "$_tgt" ] && targets+=("$_tgt")
      done
    done <<< "$_proj_targets"
  else
    # Fallback : cibles actives globales de hub.json
    targets=()
    while IFS= read -r t; do [ -n "$t" ] && targets+=("$t"); done < <(get_active_targets)
  fi
else
  # Pas de projet spécifié → cibles actives globales
  targets=()
  while IFS= read -r t; do [ -n "$t" ] && targets+=("$t"); done < <(get_active_targets)
fi

# Résoudre le dossier de déploiement
if [ -n "$PROJECT_ID" ]; then
  PROJECT_ID=$(normalize_project_id "$PROJECT_ID")
  deploy_dir=$(resolve_project_path "$PROJECT_ID")
  log_info "Projet cible : $PROJECT_ID ($deploy_dir)"
else
  deploy_dir="$HUB_DIR"
  log_info "Déploiement au niveau du hub"
fi

echo ""

if [ "${#targets[@]}" -eq 0 ]; then
  log_warn "Aucune cible configurée — vérifier active_targets dans config/hub.json"
  exit 0
fi

for target in "${targets[@]}"; do
  log_info "── Cible : $target"
  load_adapter "$target"
  adapter_validate || { log_error "Cible $target non disponible — déploiement ignoré"; echo ""; continue; }
  _spinner_start "Déploiement vers ${target}…"
  if adapter_deploy "$deploy_dir" "$PROJECT_ID"; then
    _spinner_stop "Déployé → $target"
  else
    _spinner_stop "Échec du déploiement → $target" 1
  fi
  echo ""
done

log_success "Déploiement terminé"

