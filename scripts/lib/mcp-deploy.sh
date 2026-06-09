#!/bin/bash
# Déploiement des MCP servers
# À sourcer depuis cmd-deploy.sh
#
# Les fonctions deploy_mcp_servers et configure_mcp_in_project acceptent
# un paramètre optionnel PROJECT_ID pour filtrer les MCP selon le champ
# "- MCP :" de projects.md (get_project_mcp / should_deploy_mcp).
# Si PROJECT_ID est absent, tous les MCP disponibles sont déployés.

# Vérifie et build les MCP si nécessaire
check_and_build_mcp() {
  echo -e "${CYAN}🔧  Vérification des MCP servers...${RESET}"
  echo ""
  
  if ! bash "$HUB_DIR/scripts/check-mcp.sh" 2>&1 | grep -q "need to be built"; then
    echo "  ✓ Tous les MCP servers sont à jour"
    echo ""
    return 0
  fi
  
  echo ""
  _prompt build_mcp "Des MCP servers doivent être compilés. Les build maintenant?"
  build_mcp="${build_mcp:-Y}"
  
  if [[ "$build_mcp" =~ ^[Yy]$ ]]; then
    bash "$HUB_DIR/scripts/build-mcp.sh"
  else
    log_warn "⚠️  Déploiement des MCP ignoré (non buildés)"
    return 1
  fi
}

# Déploie les MCP servers vers un projet
# @param $1 — deploy_dir : chemin racine du projet
# @param $2 — project_id (optionnel) : filtre les MCP via le champ "- MCP :" de projects.md
deploy_mcp_servers() {
  local deploy_dir=$1
  local project_id="${2:-}"
  
  echo -e "${CYAN}📦  Déploiement des MCP servers${RESET}"
  echo ""
  
  # Créer .opencode/servers/
  mkdir -p "$deploy_dir/.opencode/servers"
  
  local deployed_count=0
  local servers_list=()
  
  # Pour chaque MCP dans servers/
  for server_dir in "$HUB_DIR/servers/"*/; do
    [ ! -d "$server_dir" ] && continue
    
    local server_name
    server_name=$(basename "$server_dir")
    
    # Filtrer selon la sélection du projet si PROJECT_ID fourni
    if [ -n "$project_id" ]; then
      if ! should_deploy_mcp "$project_id" "$server_name"; then
        echo "  ○ $server_name ignoré (non sélectionné pour $project_id)"
        continue
      fi
    fi
    
    # Vérifier que le build existe
    if [ ! -d "$server_dir/dist" ]; then
      log_warn "  ⚠️  $server_name: non buildé, ignoré"
      continue
    fi
    
    echo "  → Déploiement de $server_name"
    
    # Créer le dossier cible
    local target_dir="$deploy_dir/.opencode/servers/$server_name"
    mkdir -p "$target_dir"
    
    # Copier dist + package.json
    cp -r "$server_dir/dist" "$target_dir/" 2>/dev/null || {
      log_error "Erreur lors de la copie de dist/"
      continue
    }
    cp "$server_dir/package.json" "$target_dir/" 2>/dev/null || {
      log_error "Erreur lors de la copie de package.json"
      continue
    }
    
    # Installer dépendances prod seulement (silencieux)
    (
      cd "$target_dir"
      npm install --production --silent > /dev/null 2>&1
    ) || {
      log_warn "  ⚠️  Erreur lors de l'installation des dépendances pour $server_name"
    }
    
    deployed_count=$((deployed_count + 1))
    servers_list+=("$server_name")
  done
  
  echo ""
  
  if [ $deployed_count -eq 0 ]; then
    log_info "  Aucun MCP server déployé"
  else
    local summary_lines=()
    summary_lines+=("$deployed_count MCP server(s) déployé(s)")
    summary_lines+=("Serveurs : ${servers_list[*]}")
    _progress_summary "MCP servers déployés" "${summary_lines[@]}"
  fi
  
  echo ""
}

# Configure opencode.json avec les MCP servers
# Utilise le format validé par le schéma opencode :
#   mcp.<server-name> = { type, command (array), environment }
# Les credentials sont injectés depuis services-env.json (global),
# sauf si un override explicite existe déjà dans opencode.json du projet.
# @param $1 — deploy_dir : chemin racine du projet
# @param $2 — project_id (optionnel) : filtre les MCP via le champ "- MCP :" de projects.md
configure_mcp_in_project() {
  local deploy_dir=$1
  local project_id="${2:-}"
  local opencode_json="$deploy_dir/opencode.json"

  # Vérifier que opencode.json existe
  [ ! -f "$opencode_json" ] && return 0

  echo -e "${CYAN}⚙️  Configuration des MCP servers${RESET}"
  echo ""

  # Sauvegarder l'original
  cp "$opencode_json" "$opencode_json.bak"

  local configured_count=0

  # Pour chaque MCP déployé, ajouter la config
  for server_dir in "$deploy_dir/.opencode/servers/"*/; do
    [ ! -d "$server_dir" ] && continue

    local server_name
    server_name=$(basename "$server_dir")

    # Filtrer selon la sélection du projet si PROJECT_ID fourni
    if [ -n "$project_id" ]; then
      if ! should_deploy_mcp "$project_id" "$server_name"; then
        continue
      fi
    fi

    echo "  → Configuration de $server_name"

    # Configuration spécifique par serveur
    case "$server_name" in
      figma-mcp)
        # ── Calcul des credentials à injecter ────────────────────────────
        # 1. Lire les credentials globaux depuis services-env.json
        local global_env="{}"
        if declare -F svc_get_all_env_for_service &>/dev/null; then
          global_env=$(svc_get_all_env_for_service "figma" 2>/dev/null || printf '{}')
        fi

        # 2. Lire les overrides explicites déjà présents dans opencode.json
        local project_env="{}"
        project_env=$(jq -r '.mcp["figma-mcp"].environment // {}' "$opencode_json" 2>/dev/null || printf '{}')

        # 3. Merger : project_env écrase global_env (Option A)
        local merged_env
        merged_env=$(printf '%s\n%s' "$global_env" "$project_env" \
          | jq -s '.[0] * .[1]' 2>/dev/null || printf '{}')

        # ── Écriture dans opencode.json (format schéma opencode valide) ──
        local tmp
        tmp=$(mktemp)
        if jq --argjson env "$merged_env" \
          '.mcp["figma-mcp"] = {
            "type": "local",
            "command": ["node", ".opencode/servers/figma-mcp/dist/index.js"],
            "environment": $env
          }' "$opencode_json" > "$tmp"; then
          mv "$tmp" "$opencode_json"
          configured_count=$((configured_count + 1))
        else
          log_error "Erreur lors de la configuration de $server_name"
          rm -f "$tmp"
          mv "$opencode_json.bak" "$opencode_json"
        fi
        ;;
      gitlab-mcp)
        local global_env="{}"
        if declare -F svc_get_all_env_for_service &>/dev/null; then
          global_env=$(svc_get_all_env_for_service "gitlab" 2>/dev/null || printf '{}')
        fi

        local project_env="{}"
        project_env=$(jq -r '.mcp["gitlab-mcp"].environment // {}' "$opencode_json" 2>/dev/null || printf '{}')

        local merged_env
        merged_env=$(printf '%s\n%s' "$global_env" "$project_env" \
          | jq -s '.[0] * .[1]' 2>/dev/null || printf '{}')

        local tmp
        tmp=$(mktemp)
        if jq --argjson env "$merged_env" \
          '.mcp["gitlab-mcp"] = {
            "type": "local",
            "command": ["node", ".opencode/servers/gitlab-mcp/dist/index.js"],
            "environment": $env
          }' "$opencode_json" > "$tmp"; then
          mv "$tmp" "$opencode_json"
          configured_count=$((configured_count + 1))
        else
          log_error "Erreur lors de la configuration de $server_name"
          rm -f "$tmp"
          mv "$opencode_json.bak" "$opencode_json"
        fi
        ;;
      # Autres MCP servers ici...
    esac
  done

  # Supprimer le backup si tout s'est bien passé
  [ -f "$opencode_json.bak" ] && rm "$opencode_json.bak"

  echo ""

  if [ $configured_count -eq 0 ]; then
    log_info "  Aucun MCP configuré"
  else
    log_info "  ✓ $configured_count MCP server(s) configuré(s) dans opencode.json"
  fi

  echo ""
}

# ── Wrapper Phase 4 — réutilisable depuis cmd-deploy.sh et cmd-start.sh ────────
# Enchaîne les trois étapes de la Phase 4 : vérification/build des serveurs MCP,
# déploiement des binaires, injection du bloc mcp dans opencode.json.
# Retourne 1 (non-bloquant) si une étape échoue — ne doit jamais arrêter un start.
adapter_deploy_mcp() {
  local deploy_dir="$1"
  local project_id="${2:-}"

  if check_and_build_mcp \
      && deploy_mcp_servers "$deploy_dir" "$project_id" \
      && configure_mcp_in_project "$deploy_dir" "$project_id"; then
    return 0
  else
    log_warn "Phase 4 : déploiement MCP incomplet (vérifiez les tokens et le build)"
    return 1
  fi
}
