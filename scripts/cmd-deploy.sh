#!/bin/bash
# Déploie les agents canoniques vers la cible opencode.
# Usage : oc deploy [PROJECT_ID] [--check] [--diff]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/spinner.sh"

# ── Mode --check ─────────────────────────────────────────────────────────────
# Vérifie si les fichiers générés sont à jour par rapport aux sources.
# Usage : oc deploy --check [PROJECT_ID]
_cmd_deploy_check() {
  local project_id="${1:-}"

  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
  fi

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    deploy_dir=$(resolve_project_path "$project_id")
  fi

  local tgt="opencode"
  local gen_dir="$deploy_dir/.opencode/agents"

  log_title "Vérification de fraîcheur des agents déployés"
  local stale_count=0
  local ok_count=0

  source "$LIB_DIR/prompt-builder.sh"

  log_info "── Cible : $tgt"

  # Détecter les stacks et précalculer les stack skills — une seule passe jq pour tout le projet
  local _check_detected_stacks="" _check_precomputed_stack_skills=""
  if [ -n "$project_id" ] && [ -f "$HUB_DIR/config/stack-skills.json" ]; then
    _check_detected_stacks=$(detect_stack "$deploy_dir" 2>/dev/null | sort -u || true)
    if [ -n "$_check_detected_stacks" ]; then
      _check_precomputed_stack_skills=$(precompute_stack_skills \
        "$_check_detected_stacks" "$HUB_DIR/config/stack-skills.json")
    fi
  fi

  # Pour chaque agent source, trouver le fichier généré correspondant
  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue

    # Lire le frontmatter en une seule passe (builtins bash uniquement — pas de subprocess)
    read_agent_frontmatter "$agent_file"

    # _fm_id is set by read_agent_frontmatter() called above
    # shellcheck disable=SC2154
    local agent_id="$_fm_id"
    [ -z "$agent_id" ] && agent_id=$(basename "$agent_file" .md)

    # Filtrer les agents non sélectionnés pour ce projet
    should_deploy_agent "$project_id" "$agent_id" || continue

    # Nom du fichier généré selon la cible
    local gen_file="$gen_dir/${agent_id}.md"

    if [ ! -f "$gen_file" ]; then
      echo -e "  ${RED}✗ MANQUANT${RESET}  $agent_id → ${agent_id}.md"
      stale_count=$((stale_count + 1))
      continue
    fi

    # Vérifier si une source quelconque est plus récente que le fichier généré
    # Utilise l'opérateur bash -nt (newer than) : builtin, pas de subprocess
    local stale_reason=""

    # Agent source plus récent que le déployé ?
    if [ "$agent_file" -nt "$gen_file" ]; then
      stale_reason="agent source modifié"
    fi

    # Un skill déclaré dans le frontmatter est-il plus récent ?
    # _fm_skills est déjà en mémoire — _fm_list_items utilise tr (pas de sed)
    if [ -z "$stale_reason" ] && [ -n "$_fm_skills" ]; then
      while IFS= read -r skill; do
        [ -z "$skill" ] && continue
        local skill_file="$SKILLS_DIR/${skill}.md"
        [ -f "$skill_file" ] || continue
        if [ "$skill_file" -nt "$gen_file" ]; then
          stale_reason="skill: $skill"
          break
        fi
      done < <(_fm_list_items "$_fm_skills")
    fi

    # Un stack skill dynamique est-il plus récent ?
    if [ -z "$stale_reason" ] && [ -n "$_check_precomputed_stack_skills" ]; then
      while IFS= read -r stack_skill; do
        [ -z "$stack_skill" ] && continue
        local sf="$SKILLS_DIR/${stack_skill}.md"
        [ -f "$sf" ] || continue
        if [ "$sf" -nt "$gen_file" ]; then
          stale_reason="stack-skill: $stack_skill"
          break
        fi
      done < <(_get_precomputed_stack_skills "$agent_id" "$_check_precomputed_stack_skills")
    fi

    if [ -n "$stale_reason" ]; then
      echo -e "  ${YELLOW}⚠ OBSOLÈTE${RESET}  $agent_id → $(basename "$gen_file")  ($stale_reason)"
      stale_count=$((stale_count + 1))
    else
      echo -e "  ${GREEN}✓ À JOUR${RESET}    $agent_id → $(basename "$gen_file")"
      ok_count=$((ok_count + 1))
    fi
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  echo ""

  echo -e "Résultat : ${GREEN}$ok_count à jour${RESET}  |  ${stale_count:+${YELLOW}}$stale_count obsolète(s)/manquant(s)${RESET}"
  if [ "$stale_count" -gt 0 ]; then
    echo ""
    log_info "Régénérer : ./oc.sh deploy${project_id:+ $project_id}"
    exit 1
  fi
}

# ── Mode --diff ───────────────────────────────────────────────────────────────
# Compare le contenu qui serait généré avec les fichiers déployés actuels.
# Affiche le diff complet pour les agents modifiés, "(inchangé)" pour les autres,
# "(nouveau)" si pas encore déployé.
# Propose ensuite d'appliquer le déploiement si des différences sont trouvées.
_cmd_deploy_diff() {
  local project_id="${1:-}"

  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
  fi

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    deploy_dir=$(resolve_project_path "$project_id")
  fi

  local tgt="opencode"
  local gen_dir="$deploy_dir/.opencode/agents"

  source "$LIB_DIR/prompt-builder.sh"

  log_title "Diff des agents (sources → déployés)"
  local changed_count=0
  local new_count=0
  local same_count=0

  log_info "── Cible : $tgt"
  echo ""

  # Langue du projet si disponible
  local lang=""
  [ -n "$project_id" ] && lang=$(get_project_language "$project_id" 2>/dev/null || true)
  lang=$(resolve_agent_lang "$lang")

  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue

    local agent_id; agent_id=$(get_agent_id "$agent_file")
    should_deploy_agent "$project_id" "$agent_id" || continue

    local gen_file="$gen_dir/${agent_id}.md"

    # Générer le contenu cible dans un fichier temporaire
    local tmpfile; tmpfile=$(mktemp /tmp/oc-diff-XXXXXX.md)
    local diff_deploy_dir="$deploy_dir"
    # TODO perf: transmettre precomputed_stacks ici comme pour adapter_deploy_files et --check
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

  # ── Résumé ─────────────────────────────────────────────────────────────────
  echo -e "Résumé : ${GREEN}${new_count} nouveau(x)${RESET}  ${YELLOW}${changed_count} modifié(s)${RESET}  ${BLUE}${same_count} inchangé(s)${RESET}"
  echo ""

  local total_changes=$(( new_count + changed_count ))
  if [ "$total_changes" -eq 0 ]; then
    log_info "Aucun changement détecté."
    return 0
  fi

  _prompt apply_answer "$(t deploy_apply_prompt)"
  apply_answer="${apply_answer:-Y}"
  if [[ "$apply_answer" =~ ^[Yy]$ ]]; then
    bash "$HUB_DIR/oc.sh" deploy ${project_id:+"$project_id"} ${PROVIDER_OVERRIDE:+--provider "$PROVIDER_OVERRIDE"}
  else
    log_info "$(t deploy_cancelled)"
  fi
}

# ── Dispatch : --check, --diff ou déploiement normal ─────────────────────────
# Analyser les arguments pour détecter --check / --diff (peut être en 1ère ou 2ème position)
CHECK_MODE=false
DIFF_MODE=false
PROVIDER_OVERRIDE=""
REMAINING_ARGS=()
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --provider) PROVIDER_OVERRIDE="$arg"; _prev=""; continue ;;
  esac
  if [ "$arg" = "--check" ]; then
    CHECK_MODE=true
  elif [ "$arg" = "--diff" ]; then
    DIFF_MODE=true
  elif [ "$arg" = "--no-progress" ]; then
    _progress_disable
  elif [ "$arg" = "--provider" ]; then
    _prev="$arg"
  else
    REMAINING_ARGS+=("$arg")
  fi
done

PROJECT_ID="${REMAINING_ARGS[0]:-}"

if [ "$CHECK_MODE" = true ]; then
  _cmd_deploy_check "$PROJECT_ID"
  exit 0
fi

if [ "$DIFF_MODE" = true ]; then
  _cmd_deploy_diff "$PROJECT_ID"
  exit 0
fi

log_title "Déploiement des agents"

# Timer de déploiement (utilise SECONDS, bash builtin)
SECONDS=0

# Valider le provider override si fourni
if [ -n "$PROVIDER_OVERRIDE" ]; then
  provider_exists "$PROVIDER_OVERRIDE" || {
    log_error "Provider inconnu : '$PROVIDER_OVERRIDE'"
    log_info "Providers disponibles : $(jq -r '.providers | keys | join(", ")' "$PROVIDERS_FILE" 2>/dev/null)"
    exit 1
  }
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

echo -e "${BOLD}── Déploiement opencode${RESET}"
echo ""
load_adapter "opencode"
adapter_validate || { log_error "opencode non disponible — déploiement ignoré"; exit 1; }

# ── Phase 1 : copie des fichiers agents ────────────────────────────────────
echo -e "${CYAN}📦  Phase 1 — Copie des agents${RESET}"

if adapter_deploy_files "$deploy_dir" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
  # Construire le résumé
  summary_lines=()
  summary_lines+=("$_DEPLOY_FILES_COUNT agents déployés")
  
  # Familles d'agents
  if [ -n "$_DEPLOY_FILES_FAMILIES" ]; then
    summary_lines+=("Familles : $_DEPLOY_FILES_FAMILIES")
  fi
  
  # Stacks détectés (toujours afficher)
  if [ -n "$_DEPLOY_FILES_STACKS" ]; then
    stacks_formatted=$(echo "$_DEPLOY_FILES_STACKS" | tr '\n' ', ' | sed 's/, $//')
    summary_lines+=("Stack skills : $stacks_formatted détectés")
  else
    summary_lines+=("Stack skills : Aucun stack détecté")
  fi
  
  _progress_summary "Phase 1 terminée" "${summary_lines[@]}"
else
  echo ""
  log_error "Échec de la Phase 1"
  exit 1
fi
echo ""

# ── Phase 2 : configuration provider / model ───────────────────────────────
echo -e "${CYAN}⚙️  Phase 2 — Configuration${RESET}"

if adapter_deploy_config "$deploy_dir" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
  # Cas particulier : fichier conservé (aucun changement)
  if [ "$_DEPLOY_CONFIG_SKIP" = true ]; then
    echo ""
    log_info "   opencode.json conservé (aucun changement nécessaire)"
  else
    # Construire le récapitulatif détaillé
    summary_lines=()
    summary_lines+=("opencode.json généré ($_DEPLOY_CONFIG_SIZE)")
    summary_lines+=("Modèle : $_DEPLOY_CONFIG_MODEL")
    summary_lines+=("Provider : $_DEPLOY_CONFIG_PROVIDER")
    summary_lines+=("Agents configurés : $_DEPLOY_CONFIG_TOTAL")
    
    # Sous-items (indentés avec espace + tiret)
    if [ "$_DEPLOY_CONFIG_SUBAGENTS" -gt 0 ] || [ "$_DEPLOY_CONFIG_DISABLED" -gt 0 ]; then
      [ "$_DEPLOY_CONFIG_SUBAGENTS" -gt 0 ] && summary_lines+=("  - $_DEPLOY_CONFIG_SUBAGENTS en mode subagent")
      [ "$_DEPLOY_CONFIG_DISABLED" -gt 0 ] && summary_lines+=("  - $_DEPLOY_CONFIG_DISABLED désactivés")
    fi
    
    # Permissions
    if [ "$_DEPLOY_CONFIG_PERMS" -gt 0 ]; then
      summary_lines+=("Permissions : $_DEPLOY_CONFIG_PERMS agents avec restrictions")
    fi
    
    _progress_summary "Phase 2 terminée" "${summary_lines[@]}"
  fi
else
  echo ""
  log_error "Échec de la Phase 2"
  exit 1
fi
echo ""

log_success "Déploiement terminé en ${SECONDS}s"
