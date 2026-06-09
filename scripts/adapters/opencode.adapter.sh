#!/bin/bash
# Adaptateur OpenCode — déploie vers .opencode/agents/ + opencode.json

source "$HUB_DIR/scripts/lib/prompt-builder.sh"
source "$HUB_DIR/scripts/lib/context-cache.sh"
source "$HUB_DIR/scripts/lib/agent-discovery.sh"

# Initialisations globales des tableaux _DEPLOY_FILES_* — obligatoires sous set -u.
# Sans ces déclarations, adapter_deploy_config() appelée directement (Phase 3 sans Phase 1+2)
# échoue à la garde «if [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 0 ]»
# avec «variable sans liaison» car set -u est hérité de ocp.sh.
_DEPLOY_FILES_AGENT_KEYS=()
_DEPLOY_FILES_AGENT_VALS=()
_DEPLOY_FILES_AGENT_FILES=()
_DEPLOY_FILES_COUNT=0
_DEPLOY_NATIVE_SKILLS_COUNT=0
_DEPLOY_NATIVE_SKILLS_SKIPPED=0
_DEPLOY_PRECOMPUTED_STACKS=""
_CLAMP_APPLIED_AGENTS=""
_DEPLOY_CONFIG_CLAMPS=0

# Applique le préfixe opencode et les model_aliases du provider au modèle court.
# $1 = modèle (nom court, ex: claude-sonnet-4-5)
# $2 = provider_name (ex: anthropic, bedrock, github-copilot)
# Retourne le modèle préfixé sur stdout.
_apply_provider_prefix() {
  local model="$1"
  local provider_name="$2"

  [ -z "$model" ] && return 0
  [ -z "$provider_name" ] && { echo "$model"; return 0; }

  local providers_file="$HUB_DIR/config/providers.json"
  [ -f "$providers_file" ] || { echo "$model"; return 0; }

  # Appliquer le model_alias si défini pour ce provider
  local aliased
  aliased=$(jq -r --arg p "$provider_name" --arg m "$model" \
    '.providers[$p].model_aliases[$m] // empty' "$providers_file" 2>/dev/null)
  [ -n "$aliased" ] && model="$aliased"

  # Lire le opencode_prefix (null → pas de préfixe)
  local prefix
  prefix=$(jq -r --arg p "$provider_name" \
    '.providers[$p].opencode_prefix // empty' "$providers_file" 2>/dev/null)

  if [ -n "$prefix" ]; then
    echo "${prefix}/${model}"
  else
    echo "$model"
  fi
}

# Modèle résolu par priorité :
#   1. api-keys.local.md (clé project-level) si project_id défini
#   2. variable d'env OPENCODE_MODEL
#   3. config/hub.json → default_provider.model ou opencode.model
#   4. fallback : claude-sonnet-4-5
# $1 = project_id (optionnel)
# $2 = provider_override (optionnel) — transmet l'override --provider de oc start/deploy
_get_opencode_model() {
  local project_id="${1:-}"
  local provider_override="${2:-}"
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
  # Strip le préfixe provider pour obtenir le nom court
  model="${model##*/}"
  # Appliquer le préfixe et les aliases du provider effectif
  local provider
  provider=$(get_effective_provider "$project_id" "$provider_override")
  _apply_provider_prefix "$model" "$provider"
}

# Résout le modèle pour un agent et retourne vide si identique au modèle global du projet.
# $1 = agent_file (chemin .md), $2 = project_id (optionnel), $3 = provider_override (optionnel)
# $4 = hub_agent_models (optionnel) — précalculé par adapter_deploy_config
# $5 = hub_family_models (optionnel) — précalculé par adapter_deploy_config
# $6 = hub_global_model (optionnel) — précalculé par adapter_deploy_config
# Retourne le modèle résolu sur stdout, ou rien si == modèle global projet.
_get_agent_model() {
  local agent_file="$1"
  local project_id="${2:-}"
  local provider_override="${3:-}"
  local hub_agent_models="${4:-}"
  local hub_family_models="${5:-}"
  local hub_global_model="${6:-}"

  [ -z "$agent_file" ] && return 0

  # Résoudre le modèle avec source (format "MODEL|SOURCE")
  local resolved_with_source
  resolved_with_source=$(resolve_agent_model "$agent_file" "$project_id" "$hub_agent_models" "$hub_family_models" "$hub_global_model")
  
  # Extraire le modèle et la source
  local resolved="${resolved_with_source%%|*}"  # Avant le pipe
  local source="${resolved_with_source##*|}"    # Après le pipe
  
  # Strip le préfixe provider pour obtenir le nom court
  resolved="${resolved##*/}"
  # Appliquer le préfixe et les aliases du provider effectif
  local provider
  provider=$(get_effective_provider "$project_id" "$provider_override")
  resolved=$(_apply_provider_prefix "$resolved" "$provider")

  # CHANGEMENT: Ne plus comparer avec le global, toujours retourner le modèle
  # Retourner "MODEL|SOURCE" pour utilisation ultérieure
  echo "${resolved}|${source}"
}

# Génère un objet JSON complet {"provider": {...}} selon le provider et ses paramètres
# Utilise jq end-to-end pour garantir un JSON valide même avec des valeurs spéciales
# Retourne le JSON complet ou rien si les paramètres sont insuffisants
_build_provider_json() {
  local provider="${1:-}" api_key="${2:-}" base_url="${3:-}" aws_region="${4:-}"

  [ -z "$provider" ] && return 0

  # Vérifier si le provider requiert une clé API (défaut : true pour la rétrocompatibilité)
  local requires_api_key
  requires_api_key=$(get_provider_bool "$provider" "requires_api_key")

  # Early return uniquement si la clé est vide ET que le provider en requiert une
  [ -z "$api_key" ] && [ "$requires_api_key" != "false" ] && return 0

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
    github-copilot)
      # Provider natif github-copilot d'OpenCode — authentification OAuth, pas de clé API
      jq -n '{"provider": {"github-copilot": {}}}'
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
  [ -z "$provider" ] && return 0

  # Vérifier si le provider requiert une clé API (défaut : true pour la rétrocompatibilité)
  local requires_api_key
  requires_api_key=$(get_provider_bool "$provider" "requires_api_key")

  # Early return uniquement si la clé est vide ET que le provider en requiert une
  [ -z "$api_key" ] && [ "$requires_api_key" != "false" ] && return 0

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
  if [ ! -f "$exclude_file" ] || ! grep -qx ".worktrees/" "$exclude_file"; then
    echo ".worktrees/" >> "$exclude_file"
    _added=true
  fi
  [ "$_added" = true ] && log_info "$(t init.gitignore_opencode_added)"
}

# ── Lecture des métadonnées agents ────────────────────────────────────────────
# Scanne les agents canoniques et remplit les tableaux parallèles :
#   _DEPLOY_FILES_AGENT_KEYS   — agent_id de chaque agent retenu
#   _DEPLOY_FILES_AGENT_VALS   — mode effectif (primary/subagent/…)
#   _DEPLOY_FILES_AGENT_FILES  — chemin source du fichier canonique
#   _DEPLOY_FILES_COUNT        — nombre d'agents retenus
#
# Après le scan des agents hub, applique les entrées "External agents" du projet :
#   - :substitute:hub-id → remplace la source du hub-id par le fichier projet
#   - :complement        → ajoute un nouvel agent en plus des agents hub
#
# Aucune écriture de fichier — appelable seul pour alimenter adapter_deploy_config().
# $1 = project_id (optionnel)
# $2 = deploy_dir (optionnel — requis pour résoudre les chemins relatifs des agents externes)
_load_agent_metadata() {
  local project_id="${1:-}"
  local deploy_dir="${2:-}"

  [ -d "$CANONICAL_AGENTS_DIR" ] || { log_error "[opencode] Dossier agents/ introuvable"; return 1; }

  _DEPLOY_FILES_AGENT_KEYS=()
  _DEPLOY_FILES_AGENT_VALS=()
  _DEPLOY_FILES_AGENT_FILES=()
  _DEPLOY_FILES_COUNT=0

  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue

    local agent_id; agent_id=$(get_agent_id "$agent_file")
    should_deploy_agent "$project_id" "$agent_id" || continue

    local eff_mode
    eff_mode=$(get_effective_agent_mode "$agent_file" "$project_id")
    _DEPLOY_FILES_AGENT_KEYS+=("$agent_id")
    _DEPLOY_FILES_AGENT_VALS+=("$eff_mode")
    _DEPLOY_FILES_AGENT_FILES+=("$agent_file")
    _DEPLOY_FILES_COUNT=$((_DEPLOY_FILES_COUNT + 1))
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  # ── Appliquer les agents externes déclarés dans "External agents" ─────────
  [ -z "$project_id" ] && return 0
  [ -z "$deploy_dir" ] && return 0

  # Substitutions : remplacer la source d'un agent hub par l'agent projet
  local subst_line
  while IFS= read -r subst_line; do
    [ -z "$subst_line" ] && continue
    local ext_path="${subst_line%%:*}"
    local hub_id="${subst_line##*:}"

    # Résoudre le chemin absolu (relatif au deploy_dir si non absolu)
    local abs_path
    if [[ "$ext_path" == /* ]]; then
      abs_path="$ext_path"
    else
      abs_path="${deploy_dir}/${ext_path}"
    fi

    [ -f "$abs_path" ] || {
      log_warn "[external-agents] Fichier substitut introuvable : $abs_path — ignoré"
      continue
    }

    # Chercher l'index du hub-id dans les tableaux
    local _idx=0
    local _found=0
    while [ "$_idx" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
      if [ "${_DEPLOY_FILES_AGENT_KEYS[$_idx]}" = "$hub_id" ]; then
        _DEPLOY_FILES_AGENT_FILES[$_idx]="$abs_path"
        _found=1
        break
      fi
      _idx=$((_idx + 1))
    done

    if [ "$_found" = "0" ]; then
      log_warn "[external-agents] Agent hub '$hub_id' non trouvé pour substitution — ignoré"
    fi
  done < <(get_project_substitute_agents "$project_id")

  # Compléments : ajouter de nouveaux agents en plus des agents hub
  local comp_path
  while IFS= read -r comp_path; do
    [ -z "$comp_path" ] && continue

    # Résoudre le chemin absolu
    local abs_comp_path
    if [[ "$comp_path" == /* ]]; then
      abs_comp_path="$comp_path"
    else
      abs_comp_path="${deploy_dir}/${comp_path}"
    fi

    [ -f "$abs_comp_path" ] || {
      log_warn "[external-agents] Fichier complément introuvable : $abs_comp_path — ignoré"
      continue
    }

    local comp_id; comp_id=$(get_agent_id "$abs_comp_path" 2>/dev/null || basename "$abs_comp_path" .md)

    # Vérifier qu'un agent avec ce même ID n'existe pas déjà (éviter les doublons)
    local _dup=0
    local _di=0
    while [ "$_di" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
      if [ "${_DEPLOY_FILES_AGENT_KEYS[$_di]}" = "$comp_id" ]; then
        _dup=1
        break
      fi
      _di=$((_di + 1))
    done

    if [ "$_dup" = "1" ]; then
      log_warn "[external-agents] Agent complément '$comp_id' en doublon avec un agent existant — ignoré"
      continue
    fi

    local comp_mode
    comp_mode=$(get_effective_agent_mode "$abs_comp_path" "$project_id")
    _DEPLOY_FILES_AGENT_KEYS+=("$comp_id")
    _DEPLOY_FILES_AGENT_VALS+=("$comp_mode")
    _DEPLOY_FILES_AGENT_FILES+=("$abs_comp_path")
    _DEPLOY_FILES_COUNT=$((_DEPLOY_FILES_COUNT + 1))
  done < <(get_project_complement_agents "$project_id")
}

adapter_validate() {
  command -v opencode &>/dev/null || { log_error "OpenCode non installé → oc install"; return 1; }
  command -v jq &>/dev/null || { log_error "jq non installé — requis pour le déploiement → brew install jq / sudo apt-get install jq"; return 1; }
}

# ── Phase 1b : Déploiement des skills natives ─────────────────────────────────
# Crée .opencode/skills/<name>/SKILL.md pour chaque skill native unique :
#   - native_skills déclarées dans le frontmatter de chaque agent (Bucket B explicite)
#   - stack skills résolues via precomputed_stacks (Bucket B dynamique)
# La déduplication est faite par nom de skill (basename sans .md).
# Le répertoire .opencode/skills/ est recréé à chaque déploiement (nettoyage des anciennes skills).
# Doit être appelée APRÈS _load_agent_metadata() (tableaux _DEPLOY_FILES_* remplis).
# $1 = deploy_dir
# $2 = precomputed_stacks (optionnel — chaîne "agent_id:skill_path\n..." depuis precompute_stack_skills)
deploy_native_skills() {
  local deploy_dir="$1"
  local precomputed_stacks="${2:-}"
  local skills_out_dir="$deploy_dir/.opencode/skills"

  [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 0 ] && return 0

  # Collecter toutes les skills natives uniques (native_skills + stack skills pour tous les agents)
  local _all_native_paths=()
  local _seen_names=()

  local _ai=0
  while [ "$_ai" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    local _aid="${_DEPLOY_FILES_AGENT_KEYS[$_ai]}"
    local _asource="${_DEPLOY_FILES_AGENT_FILES[$_ai]}"

    # 1. native_skills explicites du frontmatter agent
    local _ns
    while IFS= read -r _ns; do
      [ -z "$_ns" ] && continue
      local _ns_name; _ns_name=$(basename "$_ns" .md)
      local _already_seen=0
      local _s
      for _s in "${_seen_names[@]:-}"; do
        [ "$_s" = "$_ns_name" ] && _already_seen=1 && break
      done
      if [ "$_already_seen" = "0" ]; then
        _seen_names+=("$_ns_name")
        _all_native_paths+=("$_ns")
      fi
    done < <(extract_frontmatter_list "$_asource" "native_skills")

    # 2. Stack skills de l'agent (depuis precomputed)
    if [ -n "$precomputed_stacks" ]; then
      local _ss
      while IFS= read -r _ss; do
        [ -z "$_ss" ] && continue
        local _ss_name; _ss_name=$(basename "$_ss" .md)
        local _already_seen=0
        local _s
        for _s in "${_seen_names[@]:-}"; do
          [ "$_s" = "$_ss_name" ] && _already_seen=1 && break
        done
        if [ "$_already_seen" = "0" ]; then
          _seen_names+=("$_ss_name")
          _all_native_paths+=("$_ss")
        fi
      done < <(_get_precomputed_stack_skills "$_aid" "$precomputed_stacks")
    fi

    _ai=$((_ai + 1))
  done

  # Nettoyer les anciennes skills (toujours, pour garantir cohérence)
  rm -rf "$skills_out_dir" 2>/dev/null || true

  # Pas de skills natives à déployer
  if [ ${#_all_native_paths[@]} -eq 0 ]; then
    _DEPLOY_NATIVE_SKILLS_COUNT=0
    _DEPLOY_NATIVE_SKILLS_SKIPPED=0
    return 0
  fi

  # Recréer le répertoire
  mkdir -p "$skills_out_dir"

  # Déployer chaque skill native
  local _deployed=0 _skipped=0
  local _skill_path
  for _skill_path in "${_all_native_paths[@]}"; do
    [ -z "$_skill_path" ] && continue
    local _skill_name; _skill_name=$(basename "$_skill_path" .md)
    local _skill_src="$SKILLS_DIR/${_skill_path}.md"

    if [ ! -f "$_skill_src" ]; then
      log_warn "Skill native introuvable : ${_skill_path}.md" >&2
      _skipped=$((_skipped + 1))
      continue
    fi

    # Lire le frontmatter pour name et description
    local _skill_name_fm; _skill_name_fm=$(extract_frontmatter_value "$_skill_src" "name")
    local _skill_desc; _skill_desc=$(extract_frontmatter_value "$_skill_src" "description")

    # Utiliser le nom du frontmatter si présent, sinon le basename
    local _final_name="${_skill_name_fm:-$_skill_name}"

    # Créer le dossier et générer SKILL.md
    local _skill_out_dir="${skills_out_dir}/${_final_name}"
    mkdir -p "$_skill_out_dir"
    {
      echo "---"
      echo "name: ${_final_name}"
      echo "description: ${_skill_desc:-${_final_name}}"
      echo "---"
      echo ""
      strip_frontmatter "$_skill_src"
    } > "${_skill_out_dir}/SKILL.md"

    _deployed=$((_deployed + 1))
  done

  _DEPLOY_NATIVE_SKILLS_COUNT="$_deployed"
  _DEPLOY_NATIVE_SKILLS_SKIPPED="$_skipped"
}

adapter_needs_node() { return 0; }

# ── Phase 2 : déploiement des skills natives ─────────────────────────────────
# Déploie les skills natives dans .opencode/skills/<name>/SKILL.md
# Autonome : charge les métadonnées agents et précalcule les stacks si la Phase 1
# n'a pas été exécutée au préalable.
#
# $1 = deploy_dir, $2 = project_id (optionnel)
adapter_deploy_skills() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"

  source "$LIB_DIR/prompt-builder.sh"

  # Charger les métadonnées agents si les tableaux sont vides (appel direct sans Phase 1)
  if [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 0 ]; then
    _load_agent_metadata "$project_id" "$deploy_dir"
  fi

  # Réutiliser les stacks précalculés par la Phase 1, ou recalculer si appel autonome
  local _stacks="${_DEPLOY_PRECOMPUTED_STACKS:-}"
  if [ -z "$_stacks" ] && [ -n "$deploy_dir" ] && [ -d "$deploy_dir" ]; then
    local _stack_skills_config="${HUB_DIR:-}/config/stack-skills.json"
    if [ -f "$_stack_skills_config" ]; then
      local _detected
      _detected=$(detect_stack "$deploy_dir" 2>/dev/null | sort -u || true)
      if [ -n "$_detected" ]; then
        _stacks=$(precompute_stack_skills "$_detected" "$_stack_skills_config")
      fi
    fi
  fi

  deploy_native_skills "$deploy_dir" "$_stacks"
}


# Copie les agents canoniques vers .opencode/agents/ et charge les métadonnées
# nécessaires à la phase de configuration via les variables globales :
#   _DEPLOY_FILES_AGENT_KEYS / _DEPLOY_FILES_AGENT_VALS / _DEPLOY_FILES_AGENT_FILES / _DEPLOY_FILES_COUNT
#
# $1 = deploy_dir, $2 = project_id (optionnel), $3 = provider_override (ignoré — non utilisé en Phase 1)
adapter_deploy_files() {
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

  # Charger les métadonnées (réinitialise les tableaux _DEPLOY_FILES_*)
  _load_agent_metadata "$project_id" "$deploy_dir"

  # Précalculer les stack skills une seule fois pour tous les agents (une seule invocation au lieu d'une par agent)
  local _precomputed_stacks=""
  local _detected_stacks=""
  local _stack_skills_config="${HUB_DIR:-}/config/stack-skills.json"
  if [ -n "$deploy_dir" ] && [ -d "$deploy_dir" ] && [ -f "$_stack_skills_config" ]; then
    # deploy_dir est toujours non vide ici (fallback HUB_DIR), guard [ -n ] conservé par cohérence défensive
    _detected_stacks=$(detect_stack "$deploy_dir" 2>/dev/null | sort -u || true)
    if [ -n "$_detected_stacks" ]; then
      _precomputed_stacks=$(precompute_stack_skills "$_detected_stacks" "$_stack_skills_config")
    fi
  fi

  # Copier chaque agent retenu dans le répertoire cible
  local _i=0
  local _total="${#_DEPLOY_FILES_AGENT_KEYS[@]}"
  local _families_list=""  # Liste de toutes les familles pour comptage

  while [ "$_i" -lt "$_total" ]; do
    local _aid="${_DEPLOY_FILES_AGENT_KEYS[$_i]}"
    local _asource="${_DEPLOY_FILES_AGENT_FILES[$_i]}"
    
    # Afficher la progression
    _progress_bar $((_i + 1)) "$_total" "$_aid"
    
    # Build avec gestion d'erreur
    local _build_err=""
    if ! _build_err=$(build_agent_content "$_asource" "$lang" "$deploy_dir" "$_precomputed_stacks" 2>&1 > "$out_dir/${_aid}.md"); then
      # Échec du build - afficher l'erreur
      _progress_bar $((_i + 1)) "$_total" "$_aid" "error"
      _progress_done
      echo ""
      log_error "Échec du build pour $_aid"
      
      # Afficher les 5 premières lignes de l'erreur
      echo "$_build_err" | head -5 | while IFS= read -r line; do
        log_error "   $line"
      done
      echo ""
      return 1
    fi
    
    # Collecter la famille pour le comptage (bash pur, 0 subprocess)
    local _dir="${_asource%/*}"        # Équivalent dirname
    local _family="${_dir##*/}"        # Équivalent basename
    _families_list="${_families_list}${_family} "
    
    _i=$((_i + 1))
  done

  # Finaliser la progression
  _progress_done

  # Exposer les stacks précalculés pour que adapter_deploy_skills() puisse les réutiliser
  # sans recalcul (performance : une seule invocation jq/detect_stack pour tout le déploiement)
  _DEPLOY_PRECOMPUTED_STACKS="$_precomputed_stacks"

  # Compter les familles avec sort/uniq (compatible bash 3.2)
  # Format résultat : "11 developer, 8 auditor, 3 planning, ..."
  _DEPLOY_FILES_FAMILIES=$(echo "$_families_list" | tr ' ' '\n' | grep -v '^$' | sort | uniq -c | awk '{printf "%d %s, ", $1, $2}' | sed 's/, $//')

  # Stocker les stacks détectés pour le récapitulatif
  _DEPLOY_FILES_STACKS="$_detected_stacks"
}

# ── Phase 3 : configuration provider/model (opencode.json) ───────────────────
# Génère opencode.json à la racine du projet cible.
# Autonome : charge elle-même les métadonnées agents via _load_agent_metadata()
# si les tableaux _DEPLOY_FILES_* ne sont pas déjà remplis (appel direct sans Phase 1+2).
#
# $1 = deploy_dir, $2 = project_id (optionnel), $3 = provider_override (optionnel)
adapter_deploy_config() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local provider_override="${3:-}"
  local config_file="$deploy_dir/opencode.json"

  # Charger les métadonnées si les tableaux sont vides (appel direct sans Phase 1)
  # Passer deploy_dir pour que la résolution des agents externes (substituts/compléments)
  # fonctionne correctement, notamment lors d'un appel depuis un worktree.
  if [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 0 ]; then
    _load_agent_metadata "$project_id" "$deploy_dir"
  fi

  # Définir les étapes de la Phase 3 pour la progression
  local _config_steps=4
  local _step=0

  # Étape 1/4 : Chargement métadonnées
  _step=1
  _progress_bar $_step $_config_steps "Chargement métadonnées"

  # Charger le cache api-keys une seule fois (évite 30+ lectures du fichier en boucle)
  if [ -n "$project_id" ]; then
    api_keys_load_cache "$project_id"
  fi

  local effective_provider
  effective_provider=$(get_effective_provider "$project_id" "$provider_override")

  local model; model=$(_get_opencode_model "$project_id" "$provider_override")
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

  # Construire l'objet "agent" — accumulation en bash, une seule invocation jq finale
  # Deux tableaux parallèles : identifiants et fragments JSON des agents à inclure
  local _agent_ids=()
  local _agent_jsons=()

  # Réinitialiser le collecteur de clamps pour cette phase
  _CLAMP_APPLIED_AGENTS=""

  # Précalculer les 3 niveaux hub.json une seule fois (évite N×3 lectures de hub.json en boucle)
  # Si jq absent ou HUB_CONFIG inexistant, les vars restent vides → resolve_agent_model bascule sur le chemin lent (sed) — dégradation gracieuse.
  local _hub_agent_models="" _hub_family_models="" _hub_global_model="" _hub_default_provider=""
  if command -v jq &>/dev/null && [ -f "$HUB_CONFIG" ]; then
    _hub_agent_models=$(jq -r '.agent_models.agents // {} | tojson' "$HUB_CONFIG" 2>/dev/null || true)
    _hub_family_models=$(jq -r '.agent_models.families // {} | tojson' "$HUB_CONFIG" 2>/dev/null || true)
    _hub_global_model=$(jq -r '.opencode.model // empty' "$HUB_CONFIG" 2>/dev/null || true)
    _hub_default_provider=$(jq -r '.default_provider.name // empty' "$HUB_CONFIG" 2>/dev/null || true)
  fi

  local _ai=0
  while [ "$_ai" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    local _aid="${_DEPLOY_FILES_AGENT_KEYS[$_ai]}"
    local _amode="${_DEPLOY_FILES_AGENT_VALS[$_ai]}"
    local _asource="${_DEPLOY_FILES_AGENT_FILES[$_ai]}"

    # Lire le frontmatter UNE FOIS (économie de 30 appels sed)
    # Fournit : _fm_id, _fm_model, _fm_skills, _fm_raw
    read_agent_frontmatter "$_asource"

    # Extraire le bloc permission en utilisant le frontmatter déjà lu (évite sed)
    local _perm_json=""
    [ -n "$_fm_raw" ] && _perm_json=$(extract_permission_json "$_asource" "$_fm_raw")

    # Résoudre le modèle pour cet agent (retourne "MODEL|SOURCE")
    local _agent_model_with_source=""
    local _agent_model=""
    local _agent_model_source=""
    
    if [ -n "$_asource" ]; then
      _agent_model_with_source=$(_get_agent_model "$_asource" "$project_id" "$provider_override" "$_hub_agent_models" "$_hub_family_models" "$_hub_global_model")
      
      # Extraire modèle et source
      _agent_model="${_agent_model_with_source%%|*}"
      _agent_model_source="${_agent_model_with_source##*|}"
      
      # Gestion du cas limite : si aucun modèle résolu, utiliser le default du provider
      if [ -z "$_agent_model" ] || [ "$_agent_model" = "null" ]; then
        local provider
        provider=$(get_effective_provider "$project_id" "$provider_override")
        
        local provider_default
        provider_default=$(get_provider_default_model "$provider")
        
        if [ -n "$provider_default" ]; then
          _agent_model=$(_apply_provider_prefix "$provider_default" "$provider")
          _agent_model_source="provider_default"
          log_warn "Agent ${_aid} has no model configured, using provider default: $_agent_model"
        else
          log_error "Agent ${_aid} has no model configured and provider '$provider' has no default"
          return 1
        fi
      fi
    fi

    # Construire le fragment JSON de l'agent en bash (zéro fork)
    # Ordre des champs : mode (si subagent) → permission → model → _modelSource
    local _json_parts=()
    [ "$_amode" != "primary" ] && _json_parts+=('"mode": "'"${_amode}"'"')
    [ -n "$_perm_json" ]       && _json_parts+=("${_perm_json}")
    
    # CHANGEMENT: Toujours injecter le modèle, même si == global
    if [ -n "$_agent_model" ]; then
      _json_parts+=('"model": "'"${_agent_model}"'"')
      [ -n "$_agent_model_source" ] && _json_parts+=('"_modelSource": "'"${_agent_model_source}"'"')
    fi

    if [ "${#_json_parts[@]}" -gt 0 ]; then
      local _parts_str
      _parts_str=$(IFS=','; echo "${_json_parts[*]}")  # IFS réduit à ',' (supprime l'espace ambigu — comportement identique)
      _agent_ids+=("$_aid")
      _agent_jsons+=("{${_parts_str}}")
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
  if [ -n "$disabled_csv" ]; then
    local _dname
    IFS=',' read -ra _disabled_arr <<< "$disabled_csv"
    for _dname in "${_disabled_arr[@]}"; do
      _dname="${_dname#"${_dname%%[! ]*}"}"; _dname="${_dname%"${_dname##*[! ]}"}"  # trim complet leading/trailing
      [ -z "$_dname" ] && continue
      _agent_ids+=("$_dname")
      _agent_jsons+=('{"disable": true}')
    done
  elif [ -f "$HUB_CONFIG" ]; then
    # hub.json présent mais disabled_csv vide : vérifier si c'est intentionnel (tableau [])
    # ou si la clé est absente (probablement un hub.json créé avant la version 2.0.0)
    if grep -q '"disabled_native_agents"' "$HUB_CONFIG" 2>/dev/null; then
      : # tableau explicitement vide dans hub.json — intentionnel, pas de warning
    else
      log_warn "disabled_native_agents absent de hub.json — les agents natifs OpenCode (build, plan, general, explore, scout) ne seront PAS désactivés"
      log_warn "Ajouter dans config/hub.json : \"disabled_native_agents\": [\"build\",\"plan\",\"general\",\"explore\",\"scout\"]"
      log_warn "Ou relancer : ./oc.sh install  (met à jour hub.json avec les valeurs par défaut)"
    fi
  fi

  # Étape 2/4 : Construction JSON agents terminée
  _step=2
  _progress_bar $_step $_config_steps "Construction JSON agents"

  # Assembler agent_obj_json en une seule invocation jq
  # Construire la chaîne JSON brute "{\"id1\": {...}, \"id2\": {...}}" et valider via jq '.'
  local agent_obj_json="{}"
  if [ "${#_agent_ids[@]}" -gt 0 ]; then
    local _tmp_json=""
    local _ji=0
    while [ "$_ji" -lt "${#_agent_ids[@]}" ]; do
      [ -n "$_tmp_json" ] && _tmp_json="${_tmp_json},"
      _tmp_json="${_tmp_json}\"${_agent_ids[$_ji]}\": ${_agent_jsons[$_ji]}"
      _ji=$((_ji + 1))
    done
    
    # Valider le JSON avec gestion d'erreur explicite
    local _jq_error
    if ! agent_obj_json=$(printf '{%s}' "$_tmp_json" | jq '.' 2>&1); then
      _jq_error="$agent_obj_json"
      log_error "Erreur de parsing JSON lors de la construction de opencode.json"
      log_error "Détails jq: $_jq_error"
      log_error "Taille du JSON: ${#_tmp_json} caractères"
      # Afficher les 500 premiers caractères pour debug
      log_error "Début du JSON: ${_tmp_json:0:500}..."
      return 1
    fi
  fi

  # Régénérer si : fichier absent, clé API à injecter, project_id défini, ou provider_override fourni
  local should_write=false
  if [ ! -f "$config_file" ]; then
    should_write=true
  elif [ "$has_api_key" = true ]; then
    should_write=true
  elif [ -n "$project_id" ]; then
    should_write=true
  elif [ -n "$provider_override" ]; then
    # Cas sans project_id (ex : déploiement hub direct avec --provider)
    should_write=true
  fi

  if [ "$should_write" = true ]; then
    if [ "$has_api_key" = true ]; then
      _gitignore_opencode_json "$deploy_dir"
    fi

    # Étape 3/4 : Fusion des blocs
    _step=3
    _progress_bar $_step $_config_steps "Fusion configuration"

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

    # Préserver le bloc mcp existant s'il était déjà dans opencode.json
    # (écrit lors d'un déploiement précédent par configure_mcp_in_project)
    if [ -f "$config_file" ]; then
      local _existing_mcp
      _existing_mcp=$(jq '.mcp // empty' "$config_file" 2>/dev/null || true)
      if [ -n "$_existing_mcp" ]; then
        base_obj=$(jq -n \
          --argjson base "$base_obj" \
          --argjson mcp "$_existing_mcp" \
          '$base + {"mcp": $mcp}')
      fi
    fi

    # Fusionner le bloc instructions selon l'état du cache de contexte
    # Priorité : cache valide > fichiers contexte présents > rien
    local _instructions_json=""
    local _cache_file="$deploy_dir/.opencode/context.json"
    if [ -f "$_cache_file" ]; then
      # Valider le cache via context-cache.sh
      if validate_context_cache "$deploy_dir" 2>/dev/null; then
        _instructions_json='[".opencode/context.json"]'
      elif [ -f "$deploy_dir/ONBOARDING.md" ] || [ -f "$deploy_dir/CONVENTIONS.md" ]; then
        # Cache invalide mais fichiers présents — fallback sur les fichiers
        local _instr_arr="[]"
        [ -f "$deploy_dir/ONBOARDING.md" ] && _instr_arr=$(jq -n --argjson a "$_instr_arr" '$a + ["ONBOARDING.md"]')
        [ -f "$deploy_dir/CONVENTIONS.md" ] && _instr_arr=$(jq -n --argjson a "$_instr_arr" '$a + ["CONVENTIONS.md"]')
        _instructions_json="$_instr_arr"
      fi
    elif [ -f "$deploy_dir/ONBOARDING.md" ] || [ -f "$deploy_dir/CONVENTIONS.md" ]; then
      # Pas de cache — fichiers contexte directement
      local _instr_arr="[]"
      [ -f "$deploy_dir/ONBOARDING.md" ] && _instr_arr=$(jq -n --argjson a "$_instr_arr" '$a + ["ONBOARDING.md"]')
      [ -f "$deploy_dir/CONVENTIONS.md" ] && _instr_arr=$(jq -n --argjson a "$_instr_arr" '$a + ["CONVENTIONS.md"]')
      _instructions_json="$_instr_arr"
    fi
    if [ -n "$_instructions_json" ] && [ "$_instructions_json" != "[]" ]; then
      base_obj=$(jq -n \
        --argjson base "$base_obj" \
        --argjson instructions "$_instructions_json" \
        '$base + {"instructions": $instructions}')
    fi

    # Étape 4/4 : Écriture du fichier
    _step=4
    _progress_bar $_step $_config_steps "Écriture opencode.json"

    # Écriture atomique : tmp puis mv pour éviter un état corrompu si le script est interrompu
    local _tmp_config="${config_file}.tmp"
    printf '%s\n' "$base_obj" > "$_tmp_config"

    if [ "$has_api_key" = true ]; then
      chmod 600 "$_tmp_config"
      mv "$_tmp_config" "$config_file"
    else
      mv "$_tmp_config" "$config_file"
    fi
    
    # Finaliser la progression
    _progress_done
    
    # Calculer les statistiques pour le récapitulatif
    local subagent_count=0
    local _si=0
    while [ "$_si" -lt "${#_DEPLOY_FILES_AGENT_VALS[@]}" ]; do
      [ "${_DEPLOY_FILES_AGENT_VALS[$_si]}" != "primary" ] && subagent_count=$((subagent_count + 1))
      _si=$((_si + 1))
    done
    
    # Compter les agents désactivés en bash pur (zéro fork)
    local disabled_count=0
    if [ -n "$disabled_csv" ]; then
      IFS=',' read -ra _count_arr <<< "$disabled_csv"
      for _entry in "${_count_arr[@]}"; do
        _entry="${_entry#"${_entry%%[! ]*}"}"; _entry="${_entry%"${_entry##*[! ]}"}"  # trim complet leading/trailing
        [ -n "$_entry" ] && disabled_count=$((disabled_count + 1))
      done
    fi
    
    # Calculer la taille du fichier
    local _file_size=""
    if [ -f "$config_file" ]; then
      _file_size=$(du -h "$config_file" 2>/dev/null | cut -f1)
    fi
    
    # Compter les agents avec permissions personnalisées
    local _perm_count=0
    local _ji=0
    while [ "$_ji" -lt "${#_agent_jsons[@]}" ]; do
      case "${_agent_jsons[$_ji]}" in
        *"permission"*) _perm_count=$((_perm_count + 1)) ;;
      esac
      _ji=$((_ji + 1))
    done
    
    # Extraire la région pour bedrock
    local _provider_detail="$effective_provider"
    if [ "$effective_provider" = "bedrock" ] && [ -n "$project_id" ]; then
      local _region
      _region=$(get_project_api_region "$project_id" 2>/dev/null || echo "")
      [ -n "$_region" ] && _provider_detail="${effective_provider} (${_region})"
    fi
    
    # Variables globales pour le récapitulatif (utilisées par cmd-deploy.sh)
    _DEPLOY_CONFIG_MODEL="$model"
    _DEPLOY_CONFIG_PROVIDER="$_provider_detail"
    _DEPLOY_CONFIG_SIZE="${_file_size:-inconnu}"
    _DEPLOY_CONFIG_TOTAL="${#_DEPLOY_FILES_AGENT_KEYS[@]}"
    _DEPLOY_CONFIG_SUBAGENTS="$subagent_count"
    _DEPLOY_CONFIG_DISABLED="$disabled_count"
    _DEPLOY_CONFIG_PERMS="$_perm_count"
    # Compter les agents dont le plancher modèle a été appliqué
    local _clamp_count=0
    if [ -n "${_CLAMP_APPLIED_AGENTS:-}" ]; then
      _clamp_count=$(printf '%s' "$_CLAMP_APPLIED_AGENTS" | tr -cd ';' | wc -c | tr -d ' ')
    fi
    _DEPLOY_CONFIG_CLAMPS="$_clamp_count"
    _DEPLOY_CONFIG_SKIP=false
  else
    _progress_done
    _DEPLOY_CONFIG_SKIP=true
  fi
}

# ── Wrapper de compatibilité ─────────────────────────────────────────────────
# Enchaîne les trois phases pour les usages qui ne distinguent pas files/skills/config
# (cmd-deploy.sh --diff, cmd-sync, cmd-provider, etc.)
adapter_deploy() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local provider_override="${3:-}"

  adapter_deploy_files "$deploy_dir" "$project_id" "$provider_override"
  adapter_deploy_skills "$deploy_dir" "$project_id"
  adapter_deploy_config "$deploy_dir" "$project_id" "$provider_override"
  log_success "[opencode] $_DEPLOY_FILES_COUNT agent(s) → ${deploy_dir}/.opencode/agents/"
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
  local project_path="$1" prompt="${2:-}" project_id="${3:-}" agent="${4:-}" provider_override="${5:-}"
  cd "$project_path" || { log_error "[opencode] Impossible de naviguer vers $project_path"; exit 1; }
  local args=()
  [ -n "$agent"  ] && args+=(--agent "$agent")
  [ -n "$prompt" ] && args+=(--prompt "$prompt")

  # Résoudre le provider effectif (override > projet > hub) et injecter les credentials si besoin
  local effective_provider
  effective_provider=$(get_effective_provider "$project_id" "$provider_override")

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
