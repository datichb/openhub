#!/bin/bash
# Adaptateur OpenCode — déploie vers .opencode/agents/ + opencode.json

source "$HUB_DIR/scripts/lib/prompt-builder.sh"

# Modèle résolu par priorité :
#   1. api-keys.local.md (clé project-level) si project_id défini
#   2. variable d'env OPENCODE_MODEL
#   3. config/hub.json → default_provider.model ou opencode.model
#   4. fallback : claude-sonnet-4-5
_get_opencode_model() {
  local project_id="${1:-}"
  local model=""
  # Niveau 1 : configuration projet (api-keys.local.md)
  if [ -n "$project_id" ]; then
    model=$(get_project_api_model "$project_id")
  fi
  # Niveau 2 : variable d'environnement
  [ -z "$model" ] && model="${OPENCODE_MODEL:-}"
  # Niveau 3 : hub.json (default_provider.model ou opencode.model)
  if [ -z "$model" ] && command -v jq &>/dev/null && [ -f "$HUB_DIR/config/hub.json" ]; then
    model=$(jq -r '.default_provider.model // .opencode.model // empty' "$HUB_DIR/config/hub.json" 2>/dev/null)
  fi
  echo "${model:-$DEFAULT_MODEL}"
}

# Génère le bloc JSON "provider" selon le provider configuré
# Retourne une chaîne JSON partielle (sans virgule de tête) ou vide
_build_provider_block() {
  local project_id="${1:-}"
  [ -z "$project_id" ] && return 0

  local provider api_key base_url
  provider=$(get_project_api_provider "$project_id")
  api_key=$(get_project_api_key "$project_id")
  { [ -z "$provider" ] || [ -z "$api_key" ]; } && return 0

  case "$provider" in
    anthropic)
      # Utiliser jq pour encoder proprement la clé API dans le JSON
      jq -n --arg key "$api_key" \
        '{"provider": {"anthropic": {"apiKey": $key}}}' \
        | sed 's/^{//;s/^}$//;/^$/d'
      ;;
    bedrock)
      # Provider natif amazon-bedrock d'OpenCode.
      # La région est obligatoire ; la clé API stockée est le bearer token
      # (injecté aussi via AWS_BEARER_TOKEN_BEDROCK au lancement par adapter_start).
      local aws_region
      aws_region=$(get_project_api_region "$project_id")
      aws_region="${aws_region:-eu-west-3}"
      jq -n --arg region "$aws_region" \
        '{"provider": {"amazon-bedrock": {"options": {"region": $region}}}}' \
        | sed 's/^{//;s/^}$//;/^$/d'
      ;;
    mammouth|github-models|ollama|litellm)
      # Providers OpenAI-compatible via litellm
      base_url=$(get_project_api_base_url "$project_id")
      if [ -n "$base_url" ]; then
        jq -n --arg key "$api_key" --arg url "$base_url" \
          '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key, "baseURL": $url}}}}' \
          | sed 's/^{//;s/^}$//;/^$/d'
      else
        jq -n --arg key "$api_key" \
          '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key}}}}' \
          | sed 's/^{//;s/^}$//;/^$/d'
      fi
      ;;
  esac
}

# Ajoute opencode.json et .opencode/ au .git/info/exclude du projet cible si une clé API est injectée
# Utilise .git/info/exclude plutôt que .gitignore pour ne pas polluer le dépôt partagé
_gitignore_opencode_json() {
  local deploy_dir="$1"
  local git_dir="$deploy_dir/.git"
  local exclude_file="$git_dir/info/exclude"
  local _added=false

  # S'assurer que .git/info/ existe (cas git init récent)
  if [ ! -d "$git_dir/info" ]; then
    mkdir -p "$git_dir/info"
  fi

  if [ ! -f "$exclude_file" ] || ! grep -qx "opencode.json" "$exclude_file"; then
    echo "opencode.json" >> "$exclude_file"
    _added=true
  fi
  if [ ! -f "$exclude_file" ] || ! grep -qx ".opencode/" "$exclude_file"; then
    echo ".opencode/" >> "$exclude_file"
    _added=true
  fi
  [ "$_added" = true ] && log_info "$(t init.gitignore_opencode_added)"
}

adapter_validate() {
  command -v opencode &>/dev/null || { log_error "OpenCode non installé → oc install"; return 1; }
}

adapter_needs_node() { return 0; }

adapter_deploy() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local out_dir="$deploy_dir/.opencode/agents"
  mkdir -p "$out_dir"
  [ -d "$CANONICAL_AGENTS_DIR" ] || { log_error "[opencode] Dossier agents/ introuvable"; return 1; }

  # Lire la langue du projet si project_id est défini (ADR-005)
  local lang=""
  if [ -n "$project_id" ]; then
    lang=$(get_project_language "$project_id")
  fi
  lang=$(resolve_agent_lang "$lang")

  local deployed=0
  # Tableau associatif : agent_id → mode effectif (pour générer opencode.json)
  local _agent_modes_keys=()
  local _agent_modes_vals=()

  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue
    agent_supports_target "$agent_file" "opencode" || { log_warn "[opencode] Ignoré : $(basename "$agent_file")"; continue; }

    local agent_id; agent_id=$(get_agent_id "$agent_file")
    should_deploy_agent "$project_id" "$agent_id" || { log_info "[opencode] Filtré : $agent_id"; continue; }
    log_info "[opencode] Génération : $agent_id"
    build_agent_content "$agent_file" "opencode" "$lang" "$deploy_dir" > "$out_dir/${agent_id}.md"
    log_success "[opencode] $agent_id"
    deployed=$((deployed + 1))

    # Résoudre le mode effectif (override projet > frontmatter > "primary")
    local eff_mode
    eff_mode=$(get_effective_agent_mode "$agent_file" "$project_id")
    _agent_modes_keys+=("$agent_id")
    _agent_modes_vals+=("$eff_mode")
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  # Générer opencode.json à la racine du projet
  local config_file="$deploy_dir/opencode.json"
  local model; model=$(_get_opencode_model "$project_id")
  local provider_block=""
  local has_api_key=false

  # Construire le bloc provider si une clé est configurée pour ce projet
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id"; then
    provider_block=$(_build_provider_block "$project_id")
    [ -n "$provider_block" ] && has_api_key=true
  else
    # Fallback : vérifier le hub default_provider
    local hub_api_key
    hub_api_key=$(get_hub_default_api_key)
    if [ -n "$hub_api_key" ]; then
      # Construire le provider_block basé sur le hub default
      local hub_provider hub_base_url
      hub_provider=$(get_hub_default_provider)
      hub_base_url=$(get_hub_default_base_url)
      
      case "$hub_provider" in
        anthropic)
          provider_block=$(jq -n --arg key "$hub_api_key" \
            '{"provider": {"anthropic": {"apiKey": $key}}}' \
            | sed 's/^{//;s/^}$//;/^$/d')
          has_api_key=true
          ;;
        bedrock)
          # Provider natif amazon-bedrock d'OpenCode.
          # La région est lue depuis hub.json (.default_provider.region) ou défaut eu-west-3.
          local hub_aws_region
          hub_aws_region=$(jq -r '.default_provider.region // empty' "$HUB_CONFIG" 2>/dev/null)
          hub_aws_region="${hub_aws_region:-eu-west-3}"
          provider_block=$(jq -n --arg region "$hub_aws_region" \
            '{"provider": {"amazon-bedrock": {"options": {"region": $region}}}}' \
            | sed 's/^{//;s/^}$//;/^$/d')
          has_api_key=true
          ;;
        mammouth|github-models|ollama|litellm)
          # Providers OpenAI-compatible via litellm
          if [ -n "$hub_base_url" ]; then
            provider_block=$(jq -n --arg key "$hub_api_key" --arg url "$hub_base_url" \
              '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key, "baseURL": $url}}}}' \
              | sed 's/^{//;s/^}$//;/^$/d')
          else
            provider_block=$(jq -n --arg key "$hub_api_key" \
              '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key}}}}' \
              | sed 's/^{//;s/^}$//;/^$/d')
          fi
          has_api_key=true
          ;;
      esac
    fi
  fi

  # Construire le bloc "agent": pour les agents dont le mode n'est pas "primary"
  # ou qui ont des permissions à injecter (ex: permission.question: allow)
  local agent_block=""
  local _ai=0
  while [ "$_ai" -lt "${#_agent_modes_keys[@]}" ]; do
    local _aid="${_agent_modes_keys[$_ai]}"
    local _amode="${_agent_modes_vals[$_ai]}"

    # Extraire le bloc permission depuis le fichier source de l'agent
    local _agent_source_file=""
    while IFS= read -r _f; do
      [ -f "$_f" ] || continue
      local _fid; _fid=$(get_agent_id "$_f")
      if [ "$_fid" = "$_aid" ]; then
        _agent_source_file="$_f"
        break
      fi
    done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

    local _perm_json=""
    [ -n "$_agent_source_file" ] && _perm_json=$(extract_permission_json "$_agent_source_file")

    # Construire l'entrée JSON pour cet agent
    local _entry=""
    if [ "$_amode" != "primary" ] && [ -n "$_perm_json" ]; then
      _entry="{ \"mode\": \"${_amode}\", ${_perm_json} }"
    elif [ "$_amode" != "primary" ]; then
      _entry="{ \"mode\": \"${_amode}\" }"
    elif [ -n "$_perm_json" ]; then
      _entry="{ ${_perm_json} }"
    fi

    if [ -n "$_entry" ]; then
      [ -n "$agent_block" ] && agent_block="${agent_block},"$'\n'
      agent_block="${agent_block}    \"${_aid}\": ${_entry}"
    fi
    _ai=$((_ai + 1))
  done

  # Injecter les agents natifs désactivés (projet > hub)
  # Si le projet a le champ "- Disable agents :" → utiliser la valeur projet
  # Sinon → utiliser la valeur de hub.json (.opencode.disabled_native_agents)
  local disabled_csv=""
  if [ -n "$project_id" ]; then
    disabled_csv=$(get_project_disabled_native_agents "$project_id")
  fi
  if [ -z "$disabled_csv" ]; then
    disabled_csv=$(get_hub_disabled_native_agents)
  fi
  for agent_name in $(echo "$disabled_csv" | tr ',' ' '); do
    [ -z "$agent_name" ] && continue
    [ -n "$agent_block" ] && agent_block="${agent_block},"$'\n'
    agent_block="${agent_block}    \"${agent_name}\": { \"disable\": true }"
  done

  # Régénérer si : fichier absent, clé API à injecter, ou project_id défini
  local should_write=false
  if [ ! -f "$config_file" ]; then
    should_write=true
  elif [ "$has_api_key" = true ]; then
    should_write=true
  elif [ -n "$project_id" ]; then
    should_write=true
  fi

  if [ "$should_write" = true ]; then
    if [ "$has_api_key" = true ]; then
      _gitignore_opencode_json "$deploy_dir"
    fi
    {
      echo '{'
      echo '  "$schema": "https://opencode.ai/config.json",'
      if [ "$has_api_key" = true ]; then
        echo "  \"model\": \"${model}\","
        printf '%s' "$provider_block"
        if [ -n "$agent_block" ]; then
          printf ',\n  "agent": {\n%s\n  }' "$agent_block"
        fi
      else
        if [ -n "$agent_block" ]; then
          echo "  \"model\": \"${model}\","
          printf '  "agent": {\n%s\n  }' "$agent_block"
        else
          echo "  \"model\": \"${model}\""
        fi
      fi
      echo ""
      echo '}'
    } > "$config_file"
    if [ "$has_api_key" = true ]; then
      log_success "[opencode] opencode.json créé avec clé API (modèle : $model, provider : $(get_project_api_provider "$project_id"))"
      chmod 600 "$config_file"
    else
      local subagent_count=0
      local _si=0
      while [ "$_si" -lt "${#_agent_modes_vals[@]}" ]; do
        [ "${_agent_modes_vals[$_si]}" != "primary" ] && subagent_count=$((subagent_count + 1))
        _si=$((_si + 1))
      done
      local disabled_count=0
      [ -n "$disabled_csv" ] && disabled_count=$(echo "$disabled_csv" | tr ',' '\n' | grep -v '^$' | wc -l | tr -d ' ')
      log_success "[opencode] opencode.json créé (modèle : $model, $subagent_count agent(s) en mode subagent, $disabled_count désactivé(s))"
    fi
  else
    log_info "[opencode] opencode.json existant conservé"
  fi

  log_success "[opencode] $deployed agent(s) → ${deploy_dir}/.opencode/agents/"
}

adapter_install() {
  if ! command -v opencode &>/dev/null; then
    command -v npm &>/dev/null || { log_error "[opencode] npm non disponible — relancez le terminal et réessayez"; return 1; }
    log_info "Installation de OpenCode..."
    npm install -g opencode-ai
    log_success "OpenCode installé"
  else
    log_success "OpenCode déjà installé ($(opencode --version 2>/dev/null || echo '?'))"
  fi
}

adapter_update() {
  command -v npm &>/dev/null || { log_error "[opencode] npm non disponible — relancez le terminal et réessayez"; return 1; }
  log_info "Mise à jour OpenCode..."
  npm update -g opencode-ai && log_success "OpenCode mis à jour" || log_warn "Échec mise à jour OpenCode"
}

adapter_start() {
  local project_path="$1" prompt="${2:-}" project_id="${3:-}" agent="${4:-}"
  cd "$project_path" || { log_error "[opencode] Impossible de naviguer vers $project_path"; exit 1; }
  local args=()
  [ -n "$agent"  ] && args+=(--agent "$agent")
  [ -n "$prompt" ] && args+=(--prompt "$prompt")

  # Résoudre le provider effectif (projet > hub) et injecter les credentials si besoin
  local effective_provider=""
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id" 2>/dev/null; then
    effective_provider=$(get_project_api_provider "$project_id" 2>/dev/null || echo "")
  fi
  [ -z "$effective_provider" ] && effective_provider=$(get_hub_default_provider 2>/dev/null || echo "")

  if [ "$effective_provider" = "bedrock" ]; then
    # Récupérer le bearer token depuis la config projet ou hub
    local bearer_token=""
    if [ -n "$project_id" ] && api_keys_entry_exists "$project_id" 2>/dev/null; then
      bearer_token=$(get_project_api_key "$project_id" 2>/dev/null || echo "")
    fi
    [ -z "$bearer_token" ] && bearer_token=$(get_hub_default_api_key 2>/dev/null || echo "")

    if [ -n "$bearer_token" ]; then
      log_info "[opencode] Injection AWS_BEARER_TOKEN_BEDROCK"
      exec env AWS_BEARER_TOKEN_BEDROCK="$bearer_token" opencode ${args[@]+"${args[@]}"}
    fi
  fi

  exec opencode ${args[@]+"${args[@]}"}
}
