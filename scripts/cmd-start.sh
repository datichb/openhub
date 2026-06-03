#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
resolve_oc_lang

ensure_projects_file

# ── Parsing des arguments (--dev, --onboard, --label, --assignee sont des flags libres) ───
DEV_MODE=false
ONBOARD_MODE=false
REFRESH_MODE=false
DEV_LABEL=""
DEV_ASSIGNEE=""
AGENT_NAME=""
PROVIDER_OVERRIDE=""
ARGS=()
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --label)    DEV_LABEL="$arg";       _prev=""; continue ;;
    --assignee) DEV_ASSIGNEE="$arg";    _prev=""; continue ;;
    --agent)    AGENT_NAME="$arg";      _prev=""; continue ;;
    --provider) PROVIDER_OVERRIDE="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --dev)      DEV_MODE=true ;;
    --onboard)  ONBOARD_MODE=true ;;
    --refresh)  REFRESH_MODE=true ;;
    --label|--assignee|--agent|--provider) _prev="$arg" ;;
    *)          ARGS+=("$arg") ;;
  esac
done
PROJECT_ID="${ARGS[0]:-}"
PROMPT="${ARGS[1]:-}"

# --dev et --onboard sont mutuellement exclusifs
if [ "$DEV_MODE" = true ] && [ "$ONBOARD_MODE" = true ]; then
  log_error "$(t start.dev_onboard_exclusive)"
  exit 1
fi

# --refresh nécessite --onboard
if [ "$REFRESH_MODE" = true ] && [ "$ONBOARD_MODE" = false ]; then
  log_error "--refresh nécessite --onboard (usage : oc start --onboard --refresh $PROJECT_ID)"
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

load_adapter
adapter_validate || { log_error "$(t start.target_unavailable) (puis sélectionner opencode)"; exit 1; }

# ── Vérifier que les agents sont déployés ──────────────
agents_dir="$PROJECT_PATH/.opencode/agents"

# ── Bloc contextuel ───────────────────────────────────────────────────────────
_intro "${PROJECT_ID}"
printf "${DIM}│${RESET}  %-10s %s\n" "Chemin"  "$PROJECT_PATH"

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
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID" ${PROVIDER_OVERRIDE:+--provider "$PROVIDER_OVERRIDE"}
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
      if (cd "$PROJECT_PATH" && bd init --prefix "$PROJECT_ID" --skip-hooks); then
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
            if ! (cd "$PROJECT_PATH" && bd label create "$_lbl"); then
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
    log_info "Sync ${_tracker} --pull-only avant démarrage…"
    if (cd "$PROJECT_PATH" && bd "$_tracker" sync --pull-only) 2>/dev/null; then
      log_success "Sync $_tracker terminé"
    else
      log_warn "Sync $_tracker échoué — les tickets locaux seront utilisés"
    fi
  fi

  source "$LIB_DIR/prompt-builder.sh"
  PROMPT=$(build_dev_bootstrap_prompt "$PROJECT_PATH" "$DEV_LABEL" "$DEV_ASSIGNEE")
  AGENT_NAME="${AGENT_NAME:-orchestrator-dev}"
  echo ""
  if [ -n "$DEV_ASSIGNEE" ]; then
    log_info "Mode --dev  tickets assignés à '${DEV_ASSIGNEE}'  agent: ${AGENT_NAME}"
  elif [ -n "$DEV_LABEL" ]; then
    log_info "Mode --dev  tickets label '${DEV_LABEL}'  agent: ${AGENT_NAME}"
  else
    log_info "Mode --dev  tickets ai-delegated  agent: ${AGENT_NAME}"
  fi
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

# ── Confirmation avant lancement ──────────────────────────────────────────────
_outro "$(t start.press_enter) opencode…"
_prompt _ ""

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "${AGENT_NAME:-}" "$PROVIDER_OVERRIDE"
