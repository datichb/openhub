#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
resolve_oc_lang
source "$LIB_DIR/worktree.sh"

ensure_projects_file

# ── Parsing des arguments ─────────────────────────────────────────────────────
DEV_MODE=false
ONBOARD_MODE=false
REFRESH_MODE=false
PARALLEL_MODE=false
WORKTREE_MODE=false
RESUME_MODE=false
WORKTREE_BRANCH=""
DEV_LABEL=""
DEV_ASSIGNEE=""
AGENT_NAME=""
PROVIDER_OVERRIDE=""
PROJECT_ID=""
PROMPT=""
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --project|-p)  PROJECT_ID="$arg";       _prev=""; continue ;;
    --label|-l)    DEV_LABEL="$arg";         _prev=""; continue ;;
    --assignee|-a) DEV_ASSIGNEE="$arg";      _prev=""; continue ;;
    --agent|-A)    AGENT_NAME="$arg";        _prev=""; continue ;;
    --provider|-P) PROVIDER_OVERRIDE="$arg"; _prev=""; continue ;;
    --worktree|-w) WORKTREE_BRANCH="$arg";   _prev=""; continue ;;
  esac
  case "$arg" in
    --dev|-d)               DEV_MODE=true ;;
    --onboard|-o)           ONBOARD_MODE=true ;;
    --refresh|-r)           REFRESH_MODE=true ;;
    --parallel|-x)          PARALLEL_MODE=true ;;
    --resume|-R)            RESUME_MODE=true ;;
    --worktree|-w)          WORKTREE_MODE=true; _prev="$arg" ;;
    --project|-p)           _prev="$arg" ;;
    --label|-l)             _prev="$arg" ;;
    --assignee|-a)          _prev="$arg" ;;
    --agent|-A)             _prev="$arg" ;;
    --provider|-P)          _prev="$arg" ;;
    *)                      if [ -z "$PROMPT" ]; then PROMPT="$arg"; fi ;;
  esac
done

# --dev et --onboard sont mutuellement exclusifs
if [ "$DEV_MODE" = true ] && [ "$ONBOARD_MODE" = true ]; then
  log_error "$(t start.dev_onboard_exclusive)"
  exit 1
fi

# --parallel et --onboard sont mutuellement exclusifs
if [ "$PARALLEL_MODE" = true ] && [ "$ONBOARD_MODE" = true ]; then
  log_error "$(t start.parallel_onboard_exclusive)"
  exit 1
fi

# --parallel et --worktree sont mutuellement exclusifs
if [ "$PARALLEL_MODE" = true ] && [ "$WORKTREE_MODE" = true ]; then
  log_error "$(t start.parallel_worktree_exclusive)"
  exit 1
fi

# --dev et --parallel sont mutuellement exclusifs
if [ "$DEV_MODE" = true ] && [ "$PARALLEL_MODE" = true ]; then
  log_error "$(t start.dev_parallel_exclusive)"
  exit 1
fi

# --resume est incompatible avec --dev, --onboard et --parallel
if [ "$RESUME_MODE" = true ] && { [ "$DEV_MODE" = true ] || [ "$ONBOARD_MODE" = true ] || [ "$PARALLEL_MODE" = true ]; }; then
  log_error "$(t start.resume_exclusive)"
  exit 1
fi

# --refresh nécessite --onboard
if [ "$REFRESH_MODE" = true ] && [ "$ONBOARD_MODE" = false ]; then
  log_error "$(t start.refresh_needs_onboard)"
  exit 1
fi

# --label et --assignee nécessitent --dev
if { [ -n "$DEV_LABEL" ] || [ -n "$DEV_ASSIGNEE" ]; } && [ "$DEV_MODE" = false ]; then
  log_error "$(t start.dev_needs_dev_flag)"
  exit 1
fi

# --label et --assignee sont mutuellement exclusifs
if [ -n "$DEV_LABEL" ] && [ -n "$DEV_ASSIGNEE" ]; then
  log_error "$(t start.dev_label_exclusive)"
  exit 1
fi

# ── Sélection interactive si pas d'ID ─────
# --resume exige un PROJECT_ID explicite (pas de sélection interactive)
if [ "$RESUME_MODE" = true ] && [ -z "$PROJECT_ID" ]; then
  log_error "$(t start.resume_flag)"
  exit 1
fi

if [ -z "$PROJECT_ID" ]; then
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_error "$(t start.no_projects)"
    exit 1
  fi

  echo -e "${BOLD}$(t start.choose_project)${RESET}"
  echo ""
  for i in "${!ids[@]}"; do
    printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
  done
  echo ""
  read -rp "  Numéro : " choice || true
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
    log_error "Choix invalide : $choice (attendu 1-${#ids[@]})"
    exit 1
  fi
  PROJECT_ID="${ids[$((choice-1))]}"
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Validation + résolution du chemin ─────
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Validation du provider override si fourni ──────────────────────────────
if [ -n "$PROVIDER_OVERRIDE" ]; then
  provider_exists "$PROVIDER_OVERRIDE" || {
    log_error "Provider inconnu : '$PROVIDER_OVERRIDE'"
    log_info "Providers disponibles : $(jq -r '.providers | keys | join(", ")' "$PROVIDERS_FILE" 2>/dev/null)"
    exit 1
  }
fi

# ── Validation opencode ──────────────────
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/session-title.sh"

load_adapter
adapter_validate || { log_error "$(t start.target_unavailable) (puis sélectionner opencode)"; exit 1; }

SESSION_TITLE=""

# ── Vérifier que les agents sont déployés ──────────────
agents_dir="$PROJECT_PATH/.opencode/agents"

# ── Bloc contextuel ───────────────────────────────────────────────────────────
_intro "${PROJECT_ID}"
printf "${DIM}│${RESET}  %-10s %s\n" "Chemin"  "$PROJECT_PATH"

# ── Statut du provider (validation transparente) ─────────────────────────────
source "$LIB_DIR/provider-warnings.sh"
_display_provider_status "$PROJECT_ID" "$PROVIDER_OVERRIDE" "$PROJECT_PATH/opencode.json"
_PROVIDER_STATUS_DISPLAYED=1

# ── Validation du cache de contexte ──────────────────────────────────────────
source "$LIB_DIR/context-cache.sh"
if cache_exists "$PROJECT_PATH"; then
  if validate_context_cache "$PROJECT_PATH"; then
    _cache_date=$(cache_get_generated_at "$PROJECT_PATH")
    printf "${DIM}│${RESET}  %-10s %s\n" "Contexte" "✅ Cache valide (${_cache_date})"
    _inject_context_instructions "$PROJECT_PATH"
  else
    printf "${DIM}│${RESET}  %-10s %s\n" "Contexte" "⚠️  Cache invalide — oc start --onboard --refresh recommandé"
    _inject_context_instructions "$PROJECT_PATH"
  fi
else
  _inject_context_instructions "$PROJECT_PATH"
fi

# Agents non déployés : proposer le déploiement (uniquement en mode interactif)
if [ -n "$agents_dir" ] && [ ! -d "$agents_dir" ] && [ -t 0 ]; then
  echo -e "${DIM}│${RESET}"
  log_warn "$(t start.agents_not_deployed) opencode"
  _prompt _deploy_now "$(t start.deploy_now)"
  if [[ "${_deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" -p "$PROJECT_ID" ${PROVIDER_OVERRIDE:+--provider "$PROVIDER_OVERRIDE"}
    echo ""
  else
    log_info "$(t deploy_later) opencode ${PROJECT_ID}"
  fi
fi

# Si --provider est fourni et agents déjà déployés : régénérer opencode.json avec le bon provider
# Seule la Phase 3 (configuration) est nécessaire — les fichiers agents et skills sont déjà en place
if [ -n "$PROVIDER_OVERRIDE" ] && [ -n "$agents_dir" ] && [ -d "$agents_dir" ]; then
  if ! adapter_deploy_config "$PROJECT_PATH" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
    log_error "Échec de la régénération d'opencode.json — lancement annulé"
    exit 1
  fi
fi

# Suggestion onboarder si les agents sont déployés et que l'onboarding n'a pas encore été fait
if [ -n "$agents_dir" ] && [ -d "$agents_dir" ] && [ "$ONBOARD_MODE" = false ] && [ ! -f "$PROJECT_PATH/ONBOARDING.md" ]; then
  echo -e "${DIM}│${RESET}"
  echo -e "${DIM}│${RESET}  ${CYAN}→${RESET} Nouveau sur ce projet ? Invoke l'agent ${BOLD}onboarder${RESET}"
  echo -e "${DIM}│${RESET}    \"Onboarde-toi sur ce projet\""
  echo -e "${DIM}│${RESET}  ${CYAN}→${RESET} Ou lance directement : ${BOLD}./oc.sh start --onboard $PROJECT_ID${RESET}"
fi

echo -e "${DIM}│${RESET}"

# ── Vérifier que Beads est initialisé dans le projet ───
if [ ! -d "$PROJECT_PATH/.beads" ]; then
  if [ "$DEV_MODE" = true ]; then
    log_error "$(t start.dev_requires_beads)"
    log_error "$(t start.dev_beads_hint) $PROJECT_ID"
    exit 1
  elif command -v bd &>/dev/null; then
    echo ""
    log_warn "$(t start.beads_not_init)"
    _prompt _init_beads "$(t start.init_beads_now)"
    if [[ "${_init_beads:-Y}" =~ ^[Yy]$ ]]; then
      if bd -C "$PROJECT_PATH" init --prefix "$PROJECT_ID" --skip-hooks; then
        log_success "$(t beads.initialized) $PROJECT_PATH"

        # Exclure .beads/ et AGENTS.md du suivi git (exclusion locale)
        _excl_dir="$PROJECT_PATH/.git/info"
        _excl_file="$_excl_dir/exclude"
        mkdir -p "$_excl_dir"
        grep -qx ".beads/" "$_excl_file" 2>/dev/null || echo ".beads/" >> "$_excl_file"
        grep -qx "AGENTS.md" "$_excl_file" 2>/dev/null || echo "AGENTS.md" >> "$_excl_file"
        # Proposer de configurer l'upstream git si absent (ni upstream ni origin trouvé)
        if ! (cd "$PROJECT_PATH" && git remote get-url upstream) &>/dev/null && \
           ! (cd "$PROJECT_PATH" && git remote get-url origin) &>/dev/null; then
          echo ""
          _prompt _setup_upstream "$(t start.setup_upstream)"
          if [[ "${_setup_upstream:-Y}" =~ ^[Yy]$ ]]; then
            _prompt _upstream_url "$(t start.upstream_url)"
            if [ -n "$_upstream_url" ]; then
              if (cd "$PROJECT_PATH" && git remote add upstream "$_upstream_url"); then
                log_success "$(t start.upstream_ok) $_upstream_url"
              else
                log_warn "$(t start.upstream_failed)"
              fi
            else
              log_warn "$(t start.upstream_empty)"
            fi
          else
            log_info "Configurer plus tard : git remote add upstream <url>"
          fi
        fi
        # Enregistrer les labels depuis projects.md dans la config Beads
        _start_labels=$(get_project_labels "$PROJECT_ID")
        if [ -n "$_start_labels" ]; then
          log_info "Enregistrement des labels dans la config Beads…"
          _labels_ok=1
          while IFS= read -r _lbl; do
            _lbl=$(printf '%s' "$_lbl" | sed 's/^ *//;s/ *$//')
            [ -z "$_lbl" ] && continue
            if ! bd -C "$PROJECT_PATH" label create "$_lbl"; then
              _labels_ok=0
            fi
          done < <(printf '%s\n' "$_start_labels" | tr ',' '\n')
          if [ "$_labels_ok" = "1" ]; then
            log_success "$(t start.labels_registered) $_start_labels"
          else
            log_warn "$(t start.labels_failed)"
          fi
        fi
      else
        log_warn "$(t start.beads_init_failed) $PROJECT_ID"
      fi
    else
      log_info "$(t start.beads_later) $PROJECT_ID"
    fi
  else
    echo ""
    log_warn "$(t start.beads_not_init)"
    log_warn "$(t start.beads_later) $PROJECT_ID"
  fi
fi

# ── Mode --dev : sync auto + bootstrap prompt ai-delegated ──
if [ "$DEV_MODE" = true ]; then
  if ! command -v bd &>/dev/null; then
    log_error "$(t start.dev_requires_bd)"
    exit 1
  fi

  # Sync non-bloquant : pull les derniers tickets avant injection
  _tracker=$(get_project_tracker "$PROJECT_ID")
  if [ "$_tracker" != "none" ]; then
    echo ""
    log_info "Sync ${_tracker} pull avant démarrage…"
    if bd -C "$PROJECT_PATH" "$_tracker" sync pull 2>/dev/null; then
      log_success "Sync $_tracker terminé"
    else
      log_warn "Sync $_tracker échoué — les tickets locaux seront utilisés"
    fi
  fi

  source "$LIB_DIR/prompt-builder.sh"
  PROMPT=$(build_dev_bootstrap_prompt "$PROJECT_PATH" "$DEV_LABEL" "$DEV_ASSIGNEE")
  AGENT_NAME="${AGENT_NAME:-orchestrator-dev}"
  # Stocker le contexte dev pour le titre de session
  _DEV_TICKET_ID="${DEV_LABEL:-${DEV_ASSIGNEE:-}}"
  _DEV_TICKET_TITLE=""
  echo ""
  if [ -n "$DEV_ASSIGNEE" ]; then
    log_info "Mode --dev  tickets assignés à '${DEV_ASSIGNEE}'  agent: ${AGENT_NAME}"
  elif [ -n "$DEV_LABEL" ]; then
    log_info "Mode --dev  tickets label '${DEV_LABEL}'  agent: ${AGENT_NAME}"
  else
    log_info "Mode --dev  tickets ai-delegated  agent: ${AGENT_NAME}"
  fi
fi

# ── Mode --parallel : orchestrator-dev dans des worktrees isolés ─────────────
if [ "$PARALLEL_MODE" = true ]; then
  if ! command -v bd &>/dev/null; then
    log_error "bd (Beads) est requis pour le mode --parallel"
    exit 1
  fi

  _wt_enabled=$(get_project_worktree_enabled "$PROJECT_ID")
  if [ "$_wt_enabled" != "enabled" ]; then
    log_warn "Worktrees non activés pour ce projet (Worktree: enabled requis dans projects.md)"
    log_info "Activation possible via : oc config set WORKTREE_ENABLED true $PROJECT_ID"
  fi

  # Auto-cleanup si activé
  _wt_auto_cleanup=$(get_project_worktree_auto_cleanup "$PROJECT_ID")
  if [ "$_wt_auto_cleanup" = "true" ]; then
    _wt_base=$(get_project_worktree_base_branch "$PROJECT_ID")
    log_info "Auto-cleanup des worktrees mergés…"
    worktree_cleanup_merged "$PROJECT_PATH" "$_wt_base" 2>/dev/null || true
  fi

  # Sync non-bloquant
  _tracker=$(get_project_tracker "$PROJECT_ID")
  if [ "$_tracker" != "none" ]; then
    echo ""
    log_info "Sync ${_tracker} pull avant démarrage…"
    if bd -C "$PROJECT_PATH" "$_tracker" sync pull 2>/dev/null; then
      log_success "Sync $_tracker terminé"
    else
      log_warn "Sync $_tracker échoué — les tickets locaux seront utilisés"
    fi
  fi

  source "$LIB_DIR/prompt-builder.sh"
  AGENT_NAME="${AGENT_NAME:-orchestrator-dev}"

  # Récupérer les tickets disponibles
  _tickets=$(bd -C "$PROJECT_PATH" ready --label ai-delegated --json 2>/dev/null) || _tickets="[]"
  if [ -z "$_tickets" ] || [[ "$_tickets" != \[* ]]; then _tickets="[]"; fi

  if [ "$_tickets" = "[]" ] || [ "$_tickets" = "null" ]; then
    log_warn "Aucun ticket ai-delegated prêt — utilisez oc start --dev pour le mode séquentiel"
    exit 0
  fi

  # Construire le prompt de bootstrap avec contexte parallel
  PROMPT=$(build_dev_bootstrap_prompt "$PROJECT_PATH" "ai-delegated" "")

  # Créer le worktree pour cette session parallel
  _parallel_branch="parallel/$(date +%Y%m%d-%H%M%S)"
  _wt_path=$(worktree_get_path "$PROJECT_PATH" "$_parallel_branch")
  if ! worktree_create "$PROJECT_PATH" "$_parallel_branch" >/dev/null 2>&1; then
    log_warn "Impossible de créer le worktree — lancement dans le répertoire principal"
    _wt_path="$PROJECT_PATH"
  fi

  echo ""
  log_info "Mode --parallel  worktree: ${_wt_path}  agent: ${AGENT_NAME}"
  echo -e "${DIM}│${RESET}  ${DIM}Le worktree sera supprimé via : oc worktree cleanup ${PROJECT_ID}${RESET}"

  # Lancer dans le worktree si créé correctement
  if [ "$_wt_path" != "$PROJECT_PATH" ] && [ -d "$_wt_path" ]; then
    # Déployer la configuration hub (agents, skills, opencode.json) dans le worktree
    log_info "Déploiement de la configuration hub dans le worktree…"
    if ! adapter_deploy "$_wt_path" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
      log_error "Échec du déploiement de la configuration dans le worktree — lancement annulé"
      exit 1
    fi
    # Phase 4 — MCP (non-bloquant : tokens ou build potentiellement absents)
    source "$LIB_DIR/mcp-deploy.sh"
    adapter_deploy_mcp "$_wt_path" "$PROJECT_ID" || true
    _outro "$(t start.press_enter) opencode…"
    _prompt _ ""
    adapter_start "$_wt_path" "$PROMPT" "$PROJECT_ID" "${AGENT_NAME:-}" "$PROVIDER_OVERRIDE"
    exit 0
  fi
fi

# ── Mode --worktree : session libre dans un worktree isolé ───────────────────
if [ "$WORKTREE_MODE" = true ]; then
  # Demander le nom de branche si non fourni
  if [ -z "$WORKTREE_BRANCH" ]; then
    _current_branch=$(git -C "$PROJECT_PATH" branch --show-current 2>/dev/null || echo "main")
    echo -e "${DIM}│${RESET}"
    _prompt WORKTREE_BRANCH "Nom de la nouvelle branche (ex: feat/ma-feature) : "
    if [ -z "$WORKTREE_BRANCH" ]; then
      log_error "Nom de branche requis pour --worktree"
      exit 1
    fi
  fi

  # Auto-cleanup si activé
  _wt_auto_cleanup=$(get_project_worktree_auto_cleanup "$PROJECT_ID")
  if [ "$_wt_auto_cleanup" = "true" ]; then
    _wt_base=$(get_project_worktree_base_branch "$PROJECT_ID")
    log_info "Auto-cleanup des worktrees mergés…"
    worktree_cleanup_merged "$PROJECT_PATH" "$_wt_base" 2>/dev/null || true
  fi

  # Créer ou réutiliser le worktree
  if worktree_exists "$PROJECT_PATH" "$WORKTREE_BRANCH"; then
    _wt_path=$(worktree_get_path "$PROJECT_PATH" "$WORKTREE_BRANCH")
    log_info "Réutilisation du worktree existant : $_wt_path"
  else
    _wt_path=$(worktree_get_path "$PROJECT_PATH" "$WORKTREE_BRANCH")
    if ! worktree_create "$PROJECT_PATH" "$WORKTREE_BRANCH" >/dev/null; then
      log_error "Impossible de créer le worktree pour : $WORKTREE_BRANCH"
      exit 1
    fi
  fi

  echo ""
  log_info "Mode --worktree  branche: ${WORKTREE_BRANCH}  worktree: ${_wt_path}"
  echo -e "${DIM}│${RESET}  ${DIM}Session libre — pas de lien Beads obligatoire${RESET}"
  echo -e "${DIM}│${RESET}  ${DIM}Supprimer plus tard : oc worktree remove ${WORKTREE_BRANCH} ${PROJECT_ID}${RESET}"

  # Déployer la configuration hub (agents, skills, opencode.json) dans le worktree
  log_info "Déploiement de la configuration hub dans le worktree…"
  if ! adapter_deploy "$_wt_path" "$PROJECT_ID" "$PROVIDER_OVERRIDE"; then
    log_error "Échec du déploiement de la configuration dans le worktree — lancement annulé"
    exit 1
  fi
  # Phase 4 — MCP (non-bloquant : tokens ou build potentiellement absents)
  source "$LIB_DIR/mcp-deploy.sh"
  adapter_deploy_mcp "$_wt_path" "$PROJECT_ID" || true

  _outro "$(t start.press_enter) opencode…"
  _prompt _ ""
  adapter_start "$_wt_path" "$PROMPT" "$PROJECT_ID" "${AGENT_NAME:-}" "$PROVIDER_OVERRIDE"
  exit 0
fi

# ── Mode --onboard : prompt de découverte projet ────────────────────────────
if [ "$ONBOARD_MODE" = true ]; then
  source "$LIB_DIR/prompt-builder.sh"
  # --refresh : invalider le cache existant avant le re-onboarding
  if [ "$REFRESH_MODE" = true ]; then
    source "$LIB_DIR/context-cache.sh"
    if cache_invalidate "$PROJECT_PATH"; then
      log_info "Cache de contexte supprimé — re-onboarding complet"
      # Mettre à jour les instructions (fallback sur fichiers contexte ou rien)
      _inject_context_instructions "$PROJECT_PATH"
    fi
  fi
  PROMPT=$(build_onboard_bootstrap_prompt "$PROJECT_PATH" "$PROJECT_ID" "$HUB_DIR")
  AGENT_NAME="${AGENT_NAME:-onboarder}"
  echo ""
  if [ "$REFRESH_MODE" = true ]; then
    log_info "Mode --onboard --refresh  re-découverte projet + régénération cache  agent: ${AGENT_NAME}"
  else
    log_info "Mode --onboard  découverte projet activée  agent: ${AGENT_NAME}"
  fi
fi

# ── Récapitulatif de la configuration ─────────────────────────────────────────
if [ -f "$HUB_CONFIG" ]; then
  # Provider & modèle — cascade projet → hub
  _cfg_provider=$(get_effective_provider "$PROJECT_ID")
  _cfg_model=$(get_effective_llm_model "$PROJECT_ID")

  # Région bedrock si applicable
  _cfg_region=$(get_project_api_region "$PROJECT_ID" 2>/dev/null || true)
  if [ -z "$_cfg_region" ] && [ "$_cfg_provider" = "amazon-bedrock" ]; then
    _cfg_region=$(jq -r '.opencode_cache.provider["amazon-bedrock"].options.region // empty' "$HUB_CONFIG" 2>/dev/null || true)
  fi
  if [ -n "$_cfg_region" ]; then
    _cfg_provider_label="${_cfg_provider}  (${_cfg_region})"
  else
    _cfg_provider_label="${_cfg_provider}"
  fi

  # Langue, compaction, agents
  _cfg_lang=$(jq -r '.cli.language // "—"' "$HUB_CONFIG" 2>/dev/null)
  _cfg_cache_auto=$(jq -r '.opencode_cache.compaction.auto // false' "$HUB_CONFIG" 2>/dev/null)
  _cfg_cache_reserved=$(jq -r '.opencode_cache.compaction.reserved // ""' "$HUB_CONFIG" 2>/dev/null)
  if [ "$_cfg_cache_auto" = "true" ]; then
    _cfg_cache_label="$(t start.config_cache_on)"
    [ -n "$_cfg_cache_reserved" ] && _cfg_cache_label="${_cfg_cache_label}  (${_cfg_cache_reserved} $(t start.config_cache_tokens))"
  else
    _cfg_cache_label="$(t start.config_cache_off)"
  fi
  _cfg_disabled=$(jq -r '.opencode.disabled_native_agents // [] | join(", ")' "$HUB_CONFIG" 2>/dev/null)

  # MCP — lu depuis projects.md
  _cfg_mcp=$(get_project_mcp "$PROJECT_ID" 2>/dev/null || true)
  [ -z "$_cfg_mcp" ] && _cfg_mcp="none"

  # Plugins — lu depuis opencode.json du projet
  _proj_opencode_json="$PROJECT_PATH/opencode.json"
  if [ -f "$_proj_opencode_json" ]; then
    _cfg_plugins=$(jq -r '.plugin // [] | join(", ")' "$_proj_opencode_json" 2>/dev/null)
    [ -z "$_cfg_plugins" ] && _cfg_plugins="—"
  else
    _cfg_plugins="$(t start.config_not_deployed)"
  fi

  _cfg_version=$(jq -r '.version // "—"' "$HUB_CONFIG" 2>/dev/null)

  echo ""
  _intro "$(t start.config_title)"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_provider)" "$_cfg_provider_label"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_model)" "$_cfg_model"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_language)" "$_cfg_lang"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_cache)" "$_cfg_cache_label"
  [ -n "$_cfg_disabled" ] && printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_agents_off)" "$_cfg_disabled"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_mcp)" "$_cfg_mcp"
  printf "${DIM}│${RESET}  %-14s %s\n" "$(t start.config_plugins)" "$_cfg_plugins"
  _outro "hub.json v${_cfg_version}"
  echo ""
fi

# ── Confirmation avant lancement ──────────────────────────────────────────────

# Mode --resume : sélection et reprise d'une session existante
if [ "$RESUME_MODE" = true ]; then
  source "$LIB_DIR/opencode-db.sh"
  _resume_sessions=()
  _resume_ids=()
  while IFS= read -r _line; do
    [ -n "$_line" ] && _resume_sessions+=("$_line")
  done < <(ocdb_project_sessions "$PROJECT_PATH" 10 30)

  if [ ${#_resume_sessions[@]} -eq 0 ]; then
    log_warn "$(t start.resume_no_sessions)"
    log_info "$(t start.resume_hint) $PROJECT_ID"
    exit 0
  fi

  echo ""
  echo -e "${BOLD}$(t start.resume_choose) [${PROJECT_ID}] :${RESET}"
  echo ""
  for _i in "${!_resume_sessions[@]}"; do
    _entry="${_resume_sessions[$_i]}"
    _sid=$(echo "$_entry"   | cut -d'|' -f1)
    _slug=$(echo "$_entry"  | cut -d'|' -f2)
    _rtitle=$(echo "$_entry" | cut -d'|' -f3)
    _ragent=$(echo "$_entry" | cut -d'|' -f4)
    _rcost=$(echo "$_entry"  | cut -d'|' -f5)
    _rts=$(echo "$_entry"    | cut -d'|' -f6)
    _rdate=$(ocdb_format_date "$_rts" 2>/dev/null || echo "—")
    [ ${#_rtitle} -gt 36 ] && _rtitle="${_rtitle:0:34}…"
    printf "  ${BLUE}%2d${RESET})  %-38s  ${DIM}%-18s${RESET}  ${GREEN}\$%s${RESET}  ${DIM}%s${RESET}\n" \
      "$((_i+1))" "$_rtitle" "${_ragent:-—}" "$_rcost" "$_rdate"
    _resume_ids+=("$_sid")
  done
  echo ""
  _prompt _resume_choice "Numéro"
  if ! [[ "${_resume_choice:-}" =~ ^[0-9]+$ ]] || \
     [ "${_resume_choice:-0}" -lt 1 ] || \
     [ "${_resume_choice:-0}" -gt "${#_resume_ids[@]}" ]; then
    log_error "$(t start.resume_invalid) (attendu 1-${#_resume_ids[@]})"
    exit 1
  fi
  _chosen_id="${_resume_ids[$((_resume_choice-1))]}"
  _outro "$(t start.press_enter) opencode…"
  _prompt _ ""
  exec opencode -s "$_chosen_id"
fi

# ── Génération du titre de session ────────────────────────────────────────────
SESSION_TITLE=$(_build_session_title \
  "$DEV_MODE" "$ONBOARD_MODE" "$PARALLEL_MODE" \
  "$PROMPT" "$PROJECT_ID" "${WORKTREE_BRANCH:-}" \
  "${_DEV_TICKET_ID:-}" "${_DEV_TICKET_TITLE:-}")

_outro "$(t start.press_enter) opencode…"
_prompt _ ""

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "${AGENT_NAME:-}" "$PROVIDER_OVERRIDE" "$SESSION_TITLE"
