#!/bin/bash
# Déploie les agents canoniques vers la cible opencode.
# Usage : oc deploy [PROJECT_ID] [--check] [--diff]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/spinner.sh"
source "$LIB_DIR/services.sh"
source "$LIB_DIR/mcp-deploy.sh"
source "$LIB_DIR/agent-discovery.sh"

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

  local gen_dir="$deploy_dir/.opencode/agents"

  log_title "Vérification de fraîcheur des agents déployés"
  local stale_count=0
  local ok_count=0

  source "$LIB_DIR/prompt-builder.sh"

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
  echo -e "Résultat agents : ${GREEN}$ok_count à jour${RESET}  |  ${stale_count:+${YELLOW}}$stale_count obsolète(s)/manquant(s)${RESET}"

  # ── Vérification des skills natives déployées ─────────────────────────────
  echo ""
  log_title "Vérification de fraîcheur des skills déployées"
  local skill_stale_count=0
  local skill_ok_count=0

  local skills_out_dir="$deploy_dir/.opencode/skills"

  # Collecter toutes les skills natives attendues (union native_skills + stack skills, tous agents retenus)
  local _expected_skill_names=()
  local _expected_skill_sources=()
  local _seen_skill_names=()

  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue
    read_agent_frontmatter "$agent_file"
    local agent_id="$_fm_id"
    [ -z "$agent_id" ] && agent_id=$(basename "$agent_file" .md)
    should_deploy_agent "$project_id" "$agent_id" || continue

    # native_skills explicites du frontmatter
    # Réutilise $_fm_native_skills déjà lu par read_agent_frontmatter — zéro subprocess
    local _ns
    local _raw_ns="${_fm_native_skills:-}"
    _raw_ns="${_raw_ns#[}"; _raw_ns="${_raw_ns%]}"
    IFS=',' read -ra _ns_items <<< "$_raw_ns"
    for _ns_item in "${_ns_items[@]:-}"; do
      # Trim whitespace
      _ns="${_ns_item#"${_ns_item%%[![:space:]]*}"}"; _ns="${_ns%"${_ns##*[![:space:]]}"}"
      [ -z "$_ns" ] && continue
      local _ns_name; _ns_name=$(basename "$_ns" .md)
      local _already=0
      for _s in "${_seen_skill_names[@]:-}"; do
        [ "$_s" = "$_ns_name" ] && _already=1 && break
      done
      if [ "$_already" = "0" ]; then
        _seen_skill_names+=("$_ns_name")
        _expected_skill_names+=("$_ns_name")
        _expected_skill_sources+=("$SKILLS_DIR/${_ns}.md")
      fi
    done

    # Stack skills dynamiques
    if [ -n "$_check_precomputed_stack_skills" ]; then
      local _ss
      while IFS= read -r _ss; do
        [ -z "$_ss" ] && continue
        local _ss_name; _ss_name=$(basename "$_ss" .md)
        local _already=0
        for _s in "${_seen_skill_names[@]:-}"; do
          [ "$_s" = "$_ss_name" ] && _already=1 && break
        done
        if [ "$_already" = "0" ]; then
          _seen_skill_names+=("$_ss_name")
          _expected_skill_names+=("$_ss_name")
          _expected_skill_sources+=("$SKILLS_DIR/${_ss}.md")
        fi
      done < <(_get_precomputed_stack_skills "$agent_id" "$_check_precomputed_stack_skills")
    fi
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  if [ "${#_expected_skill_names[@]}" -eq 0 ]; then
    echo -e "  ${BLUE}(aucune skill native attendue pour cette cible)${RESET}"
  else
    local _si=0
    while [ "$_si" -lt "${#_expected_skill_names[@]}" ]; do
      local _sname="${_expected_skill_names[$_si]}"
      local _ssrc="${_expected_skill_sources[$_si]}"

      # Résoudre le nom final depuis le frontmatter de la skill source
      local _final_name="$_sname"
      if [ -f "$_ssrc" ]; then
        local _fm_name_val
        _fm_name_val=$(extract_frontmatter_value "$_ssrc" "name" || true)
        [ -n "$_fm_name_val" ] && _final_name="$_fm_name_val"
      fi

      local _deployed_file="$skills_out_dir/${_final_name}/SKILL.md"

      if [ ! -f "$_deployed_file" ]; then
        echo -e "  ${RED}✗ MANQUANT${RESET}  $_sname → ${_final_name}/SKILL.md"
        skill_stale_count=$((skill_stale_count + 1))
      elif [ ! -f "$_ssrc" ]; then
        echo -e "  ${YELLOW}⚠ OBSOLÈTE${RESET}  $_sname → ${_final_name}/SKILL.md  (source introuvable)"
        skill_stale_count=$((skill_stale_count + 1))
      elif [ "$_ssrc" -nt "$_deployed_file" ]; then
        echo -e "  ${YELLOW}⚠ OBSOLÈTE${RESET}  $_sname → ${_final_name}/SKILL.md  (source modifiée)"
        skill_stale_count=$((skill_stale_count + 1))
      else
        echo -e "  ${GREEN}✓ À JOUR${RESET}    $_sname → ${_final_name}/SKILL.md"
        skill_ok_count=$((skill_ok_count + 1))
      fi

      _si=$((_si + 1))
    done
  fi

  echo ""
  echo -e "Résultat skills : ${GREEN}$skill_ok_count à jour${RESET}  |  ${skill_stale_count:+${YELLOW}}$skill_stale_count obsolète(s)/manquant(s)${RESET}"

  # ── Bilan global ──────────────────────────────────────────────────────────
  local total_stale=$((stale_count + skill_stale_count))
  if [ "$total_stale" -gt 0 ]; then
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

  local gen_dir="$deploy_dir/.opencode/agents"

  source "$LIB_DIR/prompt-builder.sh"

  log_title "Diff des agents (sources → déployés)"
  local changed_count=0
  local new_count=0
  local same_count=0

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
    if ! build_agent_content "$agent_file" "$lang" "$diff_deploy_dir" > "$tmpfile" 2>/dev/null; then
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

# ── Phase 0 : découverte des agents projet ────────────────────────────────────
# Détecte les agents existants dans le projet qui ne sont pas générés par le hub,
# résout leur similarité avec les agents hub, et propose à l'utilisateur de les
# intégrer (substitution ou complément). Les choix sont persistés dans projects.md.
#
# En mode non-interactif (OC_NON_INTERACTIVE=1 ou pas de TTY) → skip silencieux.
# Appelée aussi depuis cmd-agent.sh (oc agents discover PROJECT_ID).
#
# @param $1 — PROJECT_ID
# @param $2 — deploy_dir (chemin résolu du projet)
_cmd_deploy_discover() {
  local project_id="$1"
  local deploy_dir="$2"

  # Skip si non-interactif (CI, pipes, etc.)
  if [ "${OC_NON_INTERACTIVE:-0}" = "1" ] || [ ! -t 0 ]; then
    return 0
  fi

  # Découvrir les agents du projet non générés par le hub
  local discovered_agents=()
  while IFS= read -r agent_file; do
    discovered_agents+=("$agent_file")
  done < <(discover_project_agents "$deploy_dir")

  [ ${#discovered_agents[@]} -eq 0 ] && return 0

  # Lire la config déjà persistée pour ne pas re-proposer ce qui est déjà configuré
  local already_configured
  already_configured=$(get_project_external_agents "$project_id")

  # Filtrer les agents pas encore configurés
  local new_agents=()
  for agent_file in "${discovered_agents[@]}"; do
    local rel_path="${agent_file#"$deploy_dir/"}"
    if ! echo "$already_configured" | grep -qF "$rel_path"; then
      new_agents+=("$agent_file")
    fi
  done

  [ ${#new_agents[@]} -eq 0 ] && return 0

  echo ""
  echo -e "${BOLD}── Agents existants détectés dans le projet${RESET}"
  echo ""
  log_info "${#new_agents[@]} agent(s) non générés par le hub trouvé(s) dans $deploy_dir/.opencode/agents/"
  echo ""

  local new_entries=""

  for agent_file in "${new_agents[@]}"; do
    local agent_id
    agent_id=$(get_discovery_agent_id "$agent_file")
    local rel_path="${agent_file#"$deploy_dir/"}"

    # Résoudre la similarité avec les agents hub
    local similar_hub_ids
    similar_hub_ids=$(resolve_agent_similarity "$agent_id")

    echo -e "  ${CYAN}●${RESET} Agent trouvé : ${BOLD}${agent_id}${RESET}  (${rel_path})"

    local choice=""
    if [ -n "$similar_hub_ids" ]; then
      # Match trouvé : proposer substitution / complément / ignorer
      echo -e "    ${YELLOW}→ Similaire à l'agent hub :${RESET} ${similar_hub_ids}"
      echo ""
      echo "    Que voulez-vous faire ?"
      echo "      [s] Substituer notre agent '${similar_hub_ids}' par cet agent"
      echo "      [c] Ajouter en complément (les deux coexistent)"
      echo "      [i] Ignorer (ne pas intégrer)"
      echo ""
      while true; do
        read -rp "    Votre choix [s/c/i] : " choice </dev/tty
        choice=$(echo "$choice" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
        case "$choice" in
          s|substitute) choice="s"; break ;;
          c|complement) choice="c"; break ;;
          i|ignore)     choice="i"; break ;;
          *) echo "    Choix invalide. Entrez s, c ou i." ;;
        esac
      done

      case "$choice" in
        s)
          # Prendre le premier hub-id en cas de liste (ex: "ux-designer,ui-designer")
          local primary_hub_id="${similar_hub_ids%%,*}"
          local entry="${rel_path}:substitute:${primary_hub_id}"
          new_entries="${new_entries:+${new_entries}|}${entry}"
          log_success "  → Substitution : '${agent_id}' remplacera '${primary_hub_id}' pour ce projet"
          ;;
        c)
          local entry="${rel_path}:complement"
          new_entries="${new_entries:+${new_entries}|}${entry}"
          log_success "  → Complément : '${agent_id}' sera ajouté aux agents hub"
          ;;
        i)
          log_info "  → Ignoré"
          ;;
      esac
    else
      # Pas de match : proposer complément / ignorer seulement
      echo -e "    ${BLUE}→ Aucun agent hub équivalent trouvé${RESET}"
      echo ""
      echo "    Que voulez-vous faire ?"
      echo "      [c] Ajouter en complément des agents hub"
      echo "      [i] Ignorer (ne pas intégrer)"
      echo ""
      while true; do
        read -rp "    Votre choix [c/i] : " choice </dev/tty
        choice=$(echo "$choice" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
        case "$choice" in
          c|complement) choice="c"; break ;;
          i|ignore)     choice="i"; break ;;
          *) echo "    Choix invalide. Entrez c ou i." ;;
        esac
      done

      case "$choice" in
        c)
          local entry="${rel_path}:complement"
          new_entries="${new_entries:+${new_entries}|}${entry}"
          log_success "  → Complément : '${agent_id}' sera ajouté aux agents hub"
          ;;
        i)
          log_info "  → Ignoré"
          ;;
      esac
    fi
    echo ""
  done

  # Persister les nouveaux choix dans projects.md
  if [ -n "$new_entries" ]; then
    local merged="${already_configured:+${already_configured}|}${new_entries}"
    # Nettoyer les pipes de début/fin
    merged="${merged#|}"
    merged="${merged%|}"
    if _set_project_external_agents "$project_id" "$merged"; then
      echo ""
      log_success "Choix persistés dans projects.md (champ 'External agents')"
    else
      log_warn "Impossible de persister les choix — continuez avec : oc agents discover $project_id"
    fi
  fi
}

# ── Dispatch : --check, --diff ou déploiement normal ─────────────────────────
# Only runs when executed directly (not sourced)
[[ "${BASH_SOURCE[0]}" != "$0" ]] && return 0

# Analyser les arguments pour détecter --check / --diff / --project / --provider
CHECK_MODE=false
DIFF_MODE=false
PROVIDER_OVERRIDE=""
PROJECT_ID=""
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --project|-p)  PROJECT_ID="$arg";      _prev=""; continue ;;
    --provider|-P) PROVIDER_OVERRIDE="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --check|-c)    CHECK_MODE=true ;;
    --diff|-D)     DIFF_MODE=true ;;
    --no-progress) _progress_disable ;;
    --project|-p)  _prev="$arg" ;;
    --provider|-P) _prev="$arg" ;;
  esac
done

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
  log_info "Projet : $PROJECT_ID ($deploy_dir)"
else
  # Sélection interactive
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')
  if [ ${#ids[@]} -gt 0 ]; then
    echo -e "${BOLD}Choisir un projet :${RESET}"
    echo ""
    for i in "${!ids[@]}"; do
      printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
    done
    echo ""
    read -rp "  Numéro : " choice
    if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
      log_error "Choix invalide : $choice"
      exit 1
    fi
    PROJECT_ID="${ids[$((choice-1))]}"
    PROJECT_ID=$(normalize_project_id "$PROJECT_ID")
    deploy_dir=$(resolve_project_path "$PROJECT_ID")
    log_info "Projet : $PROJECT_ID ($deploy_dir)"
  else
    deploy_dir="$HUB_DIR"
    log_info "Déploiement au niveau du hub"
  fi
fi

echo ""

echo -e "${BOLD}── Déploiement opencode${RESET}"
echo ""
load_adapter
adapter_validate || { log_error "opencode non disponible — déploiement ignoré"; exit 1; }

# ── Phase 0 : découverte des agents existants du projet ────────────────────
if [ -n "$PROJECT_ID" ]; then
  _cmd_deploy_discover "$PROJECT_ID" "$deploy_dir"
fi

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

# ── Phase 2 : déploiement des skills natives ───────────────────────────────
echo -e "${CYAN}🧩  Phase 2 — Déploiement des skills${RESET}"

if adapter_deploy_skills "$deploy_dir" "$PROJECT_ID"; then
  summary_lines=()
  summary_lines+=("$_DEPLOY_NATIVE_SKILLS_COUNT skills déployées")
  [ "${_DEPLOY_NATIVE_SKILLS_SKIPPED:-0}" -gt 0 ] && \
    summary_lines+=("$_DEPLOY_NATIVE_SKILLS_SKIPPED skills ignorées (source introuvable)")
  
  _progress_summary "Phase 2 terminée" "${summary_lines[@]}"
else
  echo ""
  log_error "Échec de la Phase 2"
  exit 1
fi
echo ""

# ── Phase 3 : configuration provider / model ───────────────────────────────
echo -e "${CYAN}⚙️  Phase 3 — Configuration${RESET}"

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

    # Planchers modèle appliqués
    if [ "${_DEPLOY_CONFIG_CLAMPS:-0}" -gt 0 ]; then
      summary_lines+=("Planchers modèle : $_DEPLOY_CONFIG_CLAMPS agent(s) clampés au plancher défini")
    fi
    
    _progress_summary "Phase 3 terminée" "${summary_lines[@]}"
  fi
else
  echo ""
  log_error "Échec de la Phase 3"
  exit 1
fi
echo ""

# ── Phase 4 : déploiement des serveurs MCP ─────────────────────────────────
echo -e "${CYAN}🔌  Phase 4 — Serveurs MCP${RESET}"

adapter_deploy_mcp "$deploy_dir" "$PROJECT_ID" || true
echo ""

log_success "Déploiement terminé en ${SECONDS}s"

# ── Graphe de dépendances (optionnel, fire-and-forget) ───────────────────────
# Généré uniquement si le projet contient des fichiers TS/JS.
# Lancé en background sans bloquer la fin du deploy.
if [ -n "$PROJECT_ID" ] && [ -n "$deploy_dir" ]; then
  source "$LIB_DIR/dependency-graph.sh"
  log_info "Graphe de dépendances : génération en arrière-plan..."
  (generate_dependency_graph "$deploy_dir" 2>/dev/null) &
  disown 2>/dev/null || true
fi
