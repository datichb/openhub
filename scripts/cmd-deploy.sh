#!/bin/bash
# Déploie les agents canoniques vers les cibles configurées.
# Usage : oc deploy [target] [PROJECT_ID] [--check] [--diff]
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/spinner.sh"

# ── Helper : timestamp portable (macOS + Linux) ───────────────────────────────
# Usage : _get_mtime <fichier>
# Retourne le timestamp mtime en secondes depuis l'epoch
_get_mtime() {
  stat -f %m "$1" 2>/dev/null || stat -c %Y "$1" 2>/dev/null
}

# ── Helper partagé : résoudre deploy_dir + targets ───────────────────────────
# Usage : _deploy_resolve TARGET PROJECT_ID
# Remplit les variables deploy_dir et _resolved_targets (tableau via stdout ligne à ligne)
# Retourne deploy_dir sur stdout ligne 1, puis les cibles une par ligne
_deploy_resolve_context() {
  local target="${1:-}" project_id="${2:-}"

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    deploy_dir=$(resolve_project_path "$project_id")
  fi

  # Résoudre les cibles
  local targets=()
  if [ -z "$target" ] || [ "$target" = "all" ]; then
    while IFS= read -r t; do [ -n "$t" ] && targets+=("$t"); done < <(get_active_targets)
  else
    targets=("$target")
  fi

  # Sortie : deploy_dir sur la première ligne, puis les cibles
  printf '%s\n' "$deploy_dir" "${targets[@]}"
}

# ── Mode --check ─────────────────────────────────────────────────────────────
# Vérifie si les fichiers générés sont à jour par rapport aux sources.
# Usage : oc deploy --check [target] [PROJECT_ID]
_cmd_deploy_check() {
  local target="${1:-all}" project_id="${2:-}"

  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
  fi

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
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

  # Détecter les stacks et précalculer les stack skills — une seule passe jq pour tout le projet
  local _check_detected_stacks="" _check_precomputed_stack_skills=""
  if [ -n "$project_id" ] && [ -f "$HUB_DIR/config/stack-skills.json" ]; then
    _check_detected_stacks=$(detect_stack "$deploy_dir" 2>/dev/null | sort -u || true)
    if [ -n "$_check_detected_stacks" ]; then
      _check_precomputed_stack_skills=$(precompute_stack_skills \
        "$_check_detected_stacks" "$HUB_DIR/config/stack-skills.json")
    fi
  fi

  for tgt in "${targets[@]}"; do
    log_info "── Cible : $tgt"

    # Répertoire des fichiers générés selon la cible
    local gen_dir=""
    case "$tgt" in
      opencode)     gen_dir="$deploy_dir/.opencode/agents" ;;
      *)            log_warn "Cible inconnue pour --check : $tgt"; continue ;;
    esac

    # Pour chaque agent source, trouver le fichier généré correspondant
    while IFS= read -r agent_file; do
      [ -f "$agent_file" ] || continue

      # Lire le frontmatter en une seule passe (builtins bash uniquement — pas de subprocess)
      read_agent_frontmatter "$agent_file"

      # Vérifier si l'agent supporte cette cible via _fm_targets (pas de subprocess)
      case "$_fm_targets" in *"$tgt"*) ;; *) continue ;; esac

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

  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
  fi

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
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
        opencode) gen_file="$gen_dir/${agent_id}.md" ;;
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

  _prompt apply_answer "$(t deploy_apply_prompt)"
  apply_answer="${apply_answer:-Y}"
  if [[ "$apply_answer" =~ ^[Yy]$ ]]; then
    bash "$HUB_DIR/oc.sh" deploy "${target:-all}" ${project_id:+"$project_id"} ${PROVIDER_OVERRIDE:+--provider "$PROVIDER_OVERRIDE"}
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
  elif [ "$arg" = "--provider" ]; then
    _prev="$arg"
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

# Valider le provider override si fourni
if [ -n "$PROVIDER_OVERRIDE" ]; then
  provider_exists "$PROVIDER_OVERRIDE" || {
    log_error "Provider inconnu : '$PROVIDER_OVERRIDE'"
    log_info "Providers disponibles : $(jq -r '.providers | keys | join(", ")' "$PROVIDERS_FILE" 2>/dev/null)"
    exit 1
  }
fi

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
  echo -e "${BOLD}── Cible : $target${RESET}"
  echo ""
  load_adapter "$target"
  adapter_validate || { log_error "Cible $target non disponible — déploiement ignoré"; echo ""; continue; }

  # ── Phase 1 : copie des fichiers agents ────────────────────────────────────
  echo -e "${CYAN}▶  Phase 1 — Copie des agents${RESET}"
  _spinner_start "Copie des agents vers ${target}…"
  if adapter_deploy_files "$deploy_dir" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
    _spinner_stop "$_DEPLOY_FILES_COUNT agent(s) déployés"
  else
    _spinner_stop "Échec de la copie des agents" 1
    echo ""
    continue
  fi
  echo ""

  # ── Phase 2 : configuration provider / model ───────────────────────────────
  echo -e "${CYAN}▶  Phase 2 — Configuration provider / model${RESET}"
  _spinner_start "Application de la configuration provider/model…"
  if adapter_deploy_config "$deploy_dir" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
    _spinner_stop "Configuration appliquée"
  else
    _spinner_stop "Échec de la configuration" 1
  fi
  echo ""
done

log_success "Déploiement terminé"
