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
  model="${model:-$DEFAULT_MODEL}"
  # Strip le préfixe provider (ex: "anthropic/claude-opus-4" → "claude-opus-4")
  echo "${model##*/}"
}

# Résout le modèle pour un agent et retourne vide si identique au modèle global du projet.
# $1 = agent_file (chemin .md), $2 = project_id (optionnel)
# Retourne le modèle résolu sur stdout, ou rien si == modèle global projet.
_get_agent_model() {
  local agent_file="$1"
  local project_id="${2:-}"

  [ -z "$agent_file" ] && return 0

  local resolved
  resolved=$(resolve_agent_model "$agent_file" "$project_id")
  # Strip le préfixe provider pour opencode.json
  resolved="${resolved##*/}"

  local global_model
  global_model=$(_get_opencode_model "$project_id")

  if [ "$resolved" = "$global_model" ]; then
    return 0
  fi

  echo "$resolved"
}

# Génère un objet JSON complet {"provider": {...}} selon le provider et ses paramètres
# Utilise jq end-to-end pour garantir un JSON valide même avec des valeurs spéciales
# Retourne le JSON complet ou rien si les paramètres sont insuffisants
_build_provider_json() {
  local provider="${1:-}" api_key="${2:-}" base_url="${3:-}" aws_region="${4:-}"

  [ -z "$provider" ] || [ -z "$api_key" ] && return 0

  case "$provider" in
    anthropic)
      jq -n --arg key "$api_key" \
        '{"provider": {"anthropic": {"apiKey": $key}}}'
      ;;
    bedrock)
      # Provider natif amazon-bedrock d'OpenCode.
      # La région est obligatoire ; la clé API stockée est le bearer token
      # (injecté aussi via AWS_BEARER_TOKEN_BEDROCK au lancement par adapter_start).
      local region="${aws_region:-eu-west-3}"
      jq -n --arg region "$region" \
        '{"provider": {"amazon-bedrock": {"options": {"region": $region}}}}'
      ;;
    mammouth|github-models|ollama|litellm)
      # Providers OpenAI-compatible via litellm
      if [ -n "$base_url" ]; then
        jq -n --arg key "$api_key" --arg url "$base_url" \
          '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key, "baseURL": $url}}}}'
      else
        jq -n --arg key "$api_key" \
          '{"provider": {"litellm": {"npm": "@ai-sdk/openai-compatible", "options": {"apiKey": $key}}}}'
      fi
      ;;
  esac
}

# Génère le bloc JSON "provider" selon le provider configuré pour un projet
# Retourne un objet JSON complet {"provider": {...}} ou rien
_build_provider_block() {
  local project_id="${1:-}"
  [ -z "$project_id" ] && return 0

  local provider api_key base_url aws_region
  provider=$(get_project_api_provider "$project_id")
  api_key=$(get_project_api_key "$project_id")
  { [ -z "$provider" ] || [ -z "$api_key" ]; } && return 0

  base_url=$(get_project_api_base_url "$project_id")
  aws_region=$(get_project_api_region "$project_id" 2>/dev/null || true)

  _build_provider_json "$provider" "$api_key" "$base_url" "$aws_region"
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
  # Tableaux parallèles : agent_id → mode effectif + fichier source
  # (bash 3.2 compatible — pas de declare -A)
  local _agent_modes_keys=()
  local _agent_modes_vals=()
  # Tableau des fichiers source (même index) pour éviter le scan O(n²)
  local _agent_modes_files=()

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
    _agent_modes_files+=("$agent_file")
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  # Générer opencode.json à la racine du projet
  local config_file="$deploy_dir/opencode.json"
  local model; model=$(_get_opencode_model "$project_id")
  local provider_json=""
  local has_api_key=false

  # Construire le bloc provider si une clé est configurée pour ce projet
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id"; then
    provider_json=$(_build_provider_block "$project_id")
    [ -n "$provider_json" ] && has_api_key=true
  else
    # Fallback : vérifier le hub default_provider
    local hub_api_key
    hub_api_key=$(get_hub_default_api_key)
    if [ -n "$hub_api_key" ]; then
      local hub_provider hub_base_url hub_aws_region
      hub_provider=$(get_hub_default_provider)
      hub_base_url=$(get_hub_default_base_url)
      hub_aws_region=$(jq -r '.default_provider.region // empty' "$HUB_CONFIG" 2>/dev/null || true)

      provider_json=$(_build_provider_json "$hub_provider" "$hub_api_key" "$hub_base_url" "$hub_aws_region")
      [ -n "$provider_json" ] && has_api_key=true
    fi
  fi

  # Construire l'objet "agent" via jq pour garantir un JSON valide
  # Chaque entrée est construite comme un objet jq puis fusionnée
  local agent_obj_json="{}"
  local _ai=0
  while [ "$_ai" -lt "${#_agent_modes_keys[@]}" ]; do
    local _aid="${_agent_modes_keys[$_ai]}"
    local _amode="${_agent_modes_vals[$_ai]}"
    local _asource="${_agent_modes_files[$_ai]}"

    # Extraire le bloc permission depuis le fichier source (déjà connu — pas de scan O(n²))
    local _perm_json=""
    [ -n "$_asource" ] && _perm_json=$(extract_permission_json "$_asource")

    # Construire l'entrée JSON pour cet agent via jq
    local _entry_json=""
    if [ "$_amode" != "primary" ] && [ -n "$_perm_json" ]; then
      _entry_json=$(jq -n --arg mode "$_amode" --argjson perm "{${_perm_json}}" \
        '{mode: $mode} + $perm')
    elif [ "$_amode" != "primary" ]; then
      _entry_json=$(jq -n --arg mode "$_amode" '{mode: $mode}')
    elif [ -n "$_perm_json" ]; then
      _entry_json=$(jq -n --argjson perm "{${_perm_json}}" '$perm')
    fi

    # Résoudre le modèle pour cet agent (vide si == modèle global)
    local _agent_model=""
    if [ -n "$_asource" ]; then
      _agent_model=$(_get_agent_model "$_asource" "$project_id")
    fi

    local _model_json="$_agent_model"

    # Fusionner le champ model si nécessaire
    if [ -n "$_model_json" ]; then
      if [ -n "$_entry_json" ]; then
        _entry_json=$(jq -n --argjson base "$_entry_json" --arg m "$_model_json" '$base + {model: $m}')
      else
        _entry_json=$(jq -n --arg m "$_model_json" '{model: $m}')
      fi
    fi

    if [ -n "$_entry_json" ]; then
      agent_obj_json=$(jq -n \
        --argjson base "$agent_obj_json" \
        --arg id "$_aid" \
        --argjson entry "$_entry_json" \
        '$base + {($id): $entry}')
    fi
    _ai=$((_ai + 1))
  done

  # Injecter les agents natifs désactivés (projet > hub)
  local disabled_csv=""
  if [ -n "$project_id" ]; then
    disabled_csv=$(get_project_disabled_native_agents "$project_id")
  fi
  if [ -z "$disabled_csv" ]; then
    disabled_csv=$(get_hub_disabled_native_agents)
  fi
  for agent_name in $(echo "$disabled_csv" | tr ',' ' '); do
    [ -z "$agent_name" ] && continue
    agent_obj_json=$(jq -n \
      --argjson base "$agent_obj_json" \
      --arg id "$agent_name" \
      '$base + {($id): {"disable": true}}')
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

    # Assembler opencode.json en une seule invocation jq end-to-end
    local base_obj
    base_obj=$(jq -n \
      --arg schema "https://opencode.ai/config.json" \
      --arg model "$model" \
      '{"$schema": $schema, "model": $model}')

    # Fusionner le bloc provider si présent
    if [ "$has_api_key" = true ] && [ -n "$provider_json" ]; then
      base_obj=$(jq -n \
        --argjson base "$base_obj" \
        --argjson provider "$provider_json" \
        '$base * $provider')
    fi

    # Fusionner le bloc agent si non vide ({} = pas d'entrées)
    local has_agents
    has_agents=$(jq -r 'if . == {} then "false" else "true" end' <<< "$agent_obj_json")
    if [ "$has_agents" = "true" ]; then
      base_obj=$(jq -n \
        --argjson base "$base_obj" \
        --argjson agents "$agent_obj_json" \
        '$base + {"agent": $agents}')
    fi

    # Écrire le fichier final
    printf '%s\n' "$base_obj" > "$config_file"

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
