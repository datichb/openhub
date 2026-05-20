#!/bin/bash
# Adaptateur Claude Code — déploie vers .claude/agents/

source "$HUB_DIR/scripts/lib/prompt-builder.sh"

adapter_validate() {
  command -v claude &>/dev/null || { log_warn "[claude-code] CLI non détecté → https://code.claudeai.ai"; return 1; }
}

adapter_needs_node() { return 0; }

# ── Phase 1 : copie des fichiers agents ──────────────────────────────────────
# Copie les agents canoniques vers .claude/agents/.
# Remplit _DEPLOY_FILES_COUNT pour le reporting dans cmd-deploy.sh.
#
# $1 = deploy_dir, $2 = project_id (optionnel), $3 = provider_override (ignoré — claude-code ne gère pas les providers)
adapter_deploy_files() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local out_dir="$deploy_dir/.claude/agents"

  mkdir -p "$out_dir"
  [ -d "$CANONICAL_AGENTS_DIR" ] || { log_error "[claude-code] Dossier agents/ introuvable"; return 1; }

  # Lire la langue du projet si project_id est défini (ADR-005)
  local lang=""
  if [ -n "$project_id" ]; then
    lang=$(get_project_language "$project_id")
  fi
  lang=$(resolve_agent_lang "$lang")

  _DEPLOY_FILES_COUNT=0

  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue
    agent_supports_target "$agent_file" "claude-code" || { log_warn "[claude-code] Ignoré : $(basename "$agent_file")"; continue; }

    local agent_id; agent_id=$(get_agent_id "$agent_file")
    should_deploy_agent "$project_id" "$agent_id" || { log_info "[claude-code] Filtré : $agent_id"; continue; }
    local label; label=$(extract_frontmatter_value "$agent_file" "label"); label="${label:-$agent_id}"
    local description; description=$(extract_frontmatter_value "$agent_file" "description")

    # Résoudre le mode effectif pour orienter la description vers la délégation
    local eff_mode; eff_mode=$(get_effective_agent_mode "$agent_file" "$project_id")
    local cc_description="$description"
    if [ "$eff_mode" = "subagent" ]; then
      # Préfixer la description pour signaler à Claude Code qu'il doit déléguer via un agent primaire
      cc_description="Sous-agent interne — invoquer uniquement via un agent coordinateur, ne pas appeler directement. ${description}"
    fi

    {
      echo "---"
      echo "name: ${label}"
      if [ -n "$cc_description" ]; then
        echo "description: >-"
        echo "  ${cc_description}"
      fi
      echo "---"
      echo ""
      build_agent_content "$agent_file" "claude-code" "$lang" "$deploy_dir"
    } > "$out_dir/${agent_id}.md"
    log_success "  $agent_id${eff_mode:+ ($eff_mode)}"
    _DEPLOY_FILES_COUNT=$((_DEPLOY_FILES_COUNT + 1))
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
}

# ── Phase 2 : configuration provider/model ────────────────────────────────────
# Claude Code ne gère pas de fichier de configuration provider via le hub.
# Cette phase est un no-op — présente pour respecter le contrat adapter.
#
# $1 = deploy_dir, $2 = project_id (optionnel), $3 = provider_override (optionnel)
adapter_deploy_config() {
  log_info "  Aucune configuration provider/model à appliquer (non supporté par Claude Code)"
}

# ── Wrapper de compatibilité ─────────────────────────────────────────────────
# Enchaîne les deux phases pour les usages qui ne distinguent pas files/config
# (cmd-deploy.sh --diff, tests, etc.)
adapter_deploy() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local provider_override="${3:-}"

  adapter_deploy_files "$deploy_dir" "$project_id" "$provider_override"
  adapter_deploy_config "$deploy_dir" "$project_id" "$provider_override"
  log_success "[claude-code] $_DEPLOY_FILES_COUNT agent(s) → ${deploy_dir}/.claude/agents/"
}

adapter_install() {
  if command -v claude &>/dev/null; then
    log_success "Claude Code déjà installé"
  else
    log_warn "[claude-code] CLI non détecté"
    log_info "  → npm install -g @anthropic-ai/claude-code"
    log_info "  → ou : https://code.claudeai.ai"
  fi
}

adapter_update() {
  command -v claude &>/dev/null || { log_warn "[claude-code] Non installé"; return; }
  command -v npm &>/dev/null || { log_error "[claude-code] npm non disponible — relancez le terminal et réessayez"; return 1; }
  log_info "Mise à jour Claude Code..."
  npm update -g @anthropic-ai/claude-code 2>/dev/null \
    && log_success "Claude Code mis à jour" \
    || { log_warn "Mise à jour auto indisponible → https://code.claudeai.ai"; }
}

adapter_start() {
  local project_path="$1" prompt="${2:-}" project_id="${3:-}" agent="${4:-}"
  command -v claude &>/dev/null || { log_error "[claude-code] Non installé → oc install (puis sélectionner Claude Code)"; exit 1; }
  cd "$project_path" || { log_error "[claude-code] Impossible de naviguer vers $project_path"; exit 1; }

  # Injecter ANTHROPIC_API_KEY si une clé anthropic est configurée pour ce projet
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id"; then
    local provider; provider=$(get_project_api_provider "$project_id")
    if [ "$provider" = "anthropic" ]; then
      local api_key; api_key=$(get_project_api_key "$project_id")
      if [ -n "$api_key" ]; then
        export ANTHROPIC_API_KEY="$api_key"
        log_info "[claude-code] Clé API anthropic injectée (ANTHROPIC_API_KEY)"
      fi
    else
      log_warn "[claude-code] Provider '$provider' configuré pour ce projet — Claude Code ne supporte que anthropic (clé API non injectée)"
    fi
  else
    # Fallback : vérifier le hub default_provider
    local hub_provider; hub_provider=$(get_hub_default_provider)
    if [ -n "$hub_provider" ] && [ "$hub_provider" != "anthropic" ]; then
      log_warn "[claude-code] Provider par défaut du hub : '$hub_provider' — Claude Code ne supporte que anthropic"
    elif [ -n "$hub_provider" ] && [ "$hub_provider" = "anthropic" ]; then
      local hub_api_key; hub_api_key=$(get_hub_default_api_key)
      if [ -n "$hub_api_key" ]; then
        export ANTHROPIC_API_KEY="$hub_api_key"
        log_info "[claude-code] Clé API anthropic du hub injectée (ANTHROPIC_API_KEY)"
      fi
    fi
  fi

  local args=()
  [ -n "$agent"  ] && args+=(--agent "$agent")
  [ -n "$prompt" ] && args+=("$prompt")
  if [ ${#args[@]} -gt 0 ]; then
    exec claude "${args[@]}"
  else
    exec claude
  fi
}
