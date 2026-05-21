#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/agent-picker.sh"
resolve_oc_lang

# S'assurer que projects.md et hub.json existent avant toute opération
ensure_projects_file
ensure_hub_config

PROJECT_ID="${1:-}"
PROJECT_PATH="${2:-}"

# ── Helper d'affichage wizard ──────────────────────────────────────────────────
_step() {
  local num="$1" total="$2" label="$3"
  echo ""
  echo -e "${DIM}│${RESET}"
  echo -e "${CYAN}◇${RESET}  ${BOLD}Étape ${num}/${total} — ${label}${RESET}"
  echo -e "${DIM}│${RESET}"
}

# ── Récapitulatif final ────────────────────────────────────────────────────────
_summary() {
  local id="$1" path="$2" name="$3" stack="$4" tracker="$5" beads_ok="$6"
  local git_remote="$7" agents="$8" provider="$9" deployed="${10}"
  local width=54
  local bar=""
  local i=0
  while [ "$i" -lt "$width" ]; do bar="${bar}─"; i=$(( i + 1 )); done
  local top_pad=$(( width - ${#id} - 14 ))
  [ "$top_pad" -lt 0 ] && top_pad=0

  echo ""
  echo -e "${GREEN}┌─ ${BOLD}${id} initialisé${RESET}${GREEN} ${bar:0:${top_pad}}┐${RESET}"
  printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Chemin"    "${path:0:36}"
  [ -n "$name"  ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Nom"       "${name:0:36}"
  [ -n "$stack" ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Stack"     "${stack:0:36}"
  printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Tracker"   "${tracker}"
  if [ "$beads_ok" = "1" ]; then
    printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Beads"   "◆ initialisé"
  else
    printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Beads"   "non initialisé  (oc beads init ${id})"
  fi
  [ -n "$git_remote" ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Git remote"  "${git_remote:0:36}"
  [ -n "$agents"     ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Agents"      "${agents:0:36}"
  [ -n "$provider"   ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Provider"    "${provider:0:36}"
  [ -n "$deployed"   ] && printf "${GREEN}│${RESET}  %-14s %-36s ${GREEN}│${RESET}\n" "Déployé"     "${deployed}"
  echo -e "${GREEN}│${RESET}"
  printf "${GREEN}│${RESET}  %-52s ${GREEN}│${RESET}\n" "Prochain → ./oc.sh start ${id}"
  [ "$deployed" != "oui" ] && \
  printf "${GREEN}│${RESET}  %-52s ${GREEN}│${RESET}\n" "Déployer  → ./oc.sh deploy all ${id}"
  echo -e "${GREEN}└─${bar}┘${RESET}"
}

# ── Titre de la commande ───────────────────────────────────────────────────────
_intro "Initialisation d'un projet"

# ─────────────────────────────────────────────────────────────────────────────
# ÉTAPE 1 — Informations projet
# ─────────────────────────────────────────────────────────────────────────────
_step 1 5 "Informations projet"

if [ -z "$PROJECT_ID" ]; then
  _prompt PROJECT_ID "PROJECT_ID (ex: MON-APP) : "
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# Validation du format PROJECT_ID : lettres, chiffres, tirets et underscores uniquement
if ! echo "$PROJECT_ID" | grep -qE '^[A-Z0-9_-]+$'; then
  log_error "PROJECT_ID invalide : '$PROJECT_ID'"
  log_info  "Caractères autorisés : lettres, chiffres, tirets (-) et underscores (_). Pas d'espaces ni de slashes."
  exit 1
fi
log_info "ID projet : ${BOLD}${PROJECT_ID}${RESET}"

if [ -z "$PROJECT_PATH" ]; then
  _prompt PROJECT_PATH "Chemin local (ex: ~/workspace/mon-app) : "
fi

# Expand ~ manuellement
PROJECT_PATH="${PROJECT_PATH/#\~/$HOME}"
log_info "Chemin : ${BOLD}${PROJECT_PATH}${RESET}"

if project_exists "$PROJECT_ID"; then
  log_warn "Le projet $PROJECT_ID existe déjà dans le registre"
  PROJECT_NAME=""
  PROJECT_STACK=""
  PROJECT_LABELS=""
  PROJECT_TRACKER="none"
else
  _prompt PROJECT_NAME   "Nom complet : "
  _prompt PROJECT_STACK  "Stack (ex: Vue 3 + Laravel) : "
  _prompt PROJECT_LABELS "Labels Beads (ex: feature,fix,front,back) : "
  log_info "Nom : ${BOLD}${PROJECT_NAME:-$PROJECT_ID}${RESET}"
  [ -n "$PROJECT_STACK" ]  && log_info "Stack : ${BOLD}${PROJECT_STACK}${RESET}"
  [ -n "$PROJECT_LABELS" ] && log_info "Labels : ${BOLD}${PROJECT_LABELS}${RESET}"

  echo -e "${DIM}│${RESET}"
  echo -e "  ${BOLD}Tracker externe (optionnel) :${RESET}"
  echo   "    1) Aucun"
  echo   "    2) Jira"
  echo   "    3) GitLab"
  echo ""
  _prompt tracker_choice "Choix [1] : "
  case "${tracker_choice:-1}" in
    2) PROJECT_TRACKER="jira" ;;
    3) PROJECT_TRACKER="gitlab" ;;
    *)  PROJECT_TRACKER="none" ;;
  esac
  log_info "Tracker : ${BOLD}${PROJECT_TRACKER}${RESET}"

  # Ajouter dans projects.md
  cat >> "$PROJECTS_FILE" <<EOF

## $PROJECT_ID
- Nom : ${PROJECT_NAME:-$PROJECT_ID}
- Stack : ${PROJECT_STACK:-N/A}
- Tracker : ${PROJECT_TRACKER}
- Labels : ${PROJECT_LABELS:-feature,fix}
- Agents : all
EOF

  log_success "Projet $PROJECT_ID ajouté dans projects.md"
fi

# ── Chemin local ───────────────────────────────────────────────────────────────
if path_exists "$PROJECT_ID"; then
  log_warn "Chemin déjà enregistré pour $PROJECT_ID"
else
  if [ ! -d "$PROJECT_PATH" ]; then
    _prompt create_dir "Le dossier $PROJECT_PATH n'existe pas. Le créer ? [Y/n] : "
    if [[ "${create_dir:-Y}" =~ ^[Yy]$ ]]; then
      mkdir -p "$PROJECT_PATH"
      log_success "Dossier créé : $PROJECT_PATH"
    else
      log_warn "Le dossier $PROJECT_PATH n'existe pas encore — Beads et le déploiement seront ignorés"
    fi
  fi
  echo "${PROJECT_ID}=${PROJECT_PATH}" >> "$PATHS_FILE"
  log_success "Chemin enregistré dans paths.local.md"
fi

# ─────────────────────────────────────────────────────────────────────────────
# ÉTAPE 2 — Beads
# ─────────────────────────────────────────────────────────────────────────────
_step 2 5 "Beads & tracker"

BEADS_OK=0
GIT_REMOTE_STATUS=""

# Vérifier que bd est disponible
if ! command -v bd &>/dev/null; then
  log_warn "Beads (bd) n'est pas installé — nécessaire pour la gestion des tickets"
  _prompt install_bd "Installer Beads maintenant ? [Y/n] : "
  if [[ "${install_bd:-Y}" =~ ^[Yy]$ ]]; then
    if command -v brew &>/dev/null; then
      brew install bd && log_success "Beads installé" \
        || log_warn "Échec de l'installation — installer manuellement : brew install bd"
    else
      log_warn "Homebrew non disponible — installer manuellement"
      log_info "  macOS  : brew install bd"
      log_info "  Linux  : voir https://beads.sh/install"
    fi
  else
    log_info "Installer plus tard : ./oc.sh install"
  fi
fi

# Proposer bd init dans le projet
if command -v bd &>/dev/null && [ -d "$PROJECT_PATH" ] && [ ! -d "$PROJECT_PATH/.beads" ]; then
  echo ""
  _prompt init_beads "Initialiser Beads dans le projet ? [Y/n] : "
  if [[ "${init_beads:-Y}" =~ ^[Yy]$ ]]; then
    if (cd "$PROJECT_PATH" && bd init --prefix "$PROJECT_ID" --skip-hooks); then
      log_success "Beads initialisé dans $PROJECT_PATH"
      BEADS_OK=1

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
        _prompt _setup_upstream "Configurer l'upstream Git (git remote add upstream) ? [Y/n] : "
        if [[ "${_setup_upstream:-Y}" =~ ^[Yy]$ ]]; then
          _prompt _upstream_url "URL du remote upstream : "
          if [ -n "$_upstream_url" ]; then
            if (cd "$PROJECT_PATH" && git remote add upstream "$_upstream_url"); then
              log_success "Remote upstream configuré : $_upstream_url"
              GIT_REMOTE_STATUS="upstream ajouté ($_upstream_url)"
            else
              log_warn "Échec de la configuration upstream — configurer manuellement"
              GIT_REMOTE_STATUS="échec de configuration"
            fi
          else
            log_warn "URL vide — configurer plus tard : git remote add upstream <url>"
            GIT_REMOTE_STATUS="non configuré"
          fi
        else
          log_info "Configurer plus tard : git remote add upstream <url>"
          GIT_REMOTE_STATUS="ignoré"
        fi
      else
        if (cd "$PROJECT_PATH" && git remote get-url upstream) &>/dev/null; then
          GIT_REMOTE_STATUS="upstream (existant)"
        else
          GIT_REMOTE_STATUS="origin (existant)"
        fi
      fi

      # Enregistrer les labels dans la config Beads
      _init_labels="${PROJECT_LABELS:-feature,fix}"
      if [ -n "$_init_labels" ]; then
        log_info "Enregistrement des labels dans la config Beads…"
        _labels_ok=1
        while IFS= read -r _lbl; do
          _lbl=$(printf '%s' "$_lbl" | sed 's/^ *//;s/ *$//')
          [ -z "$_lbl" ] && continue
          if ! (cd "$PROJECT_PATH" && bd label create "$_lbl"); then
            _labels_ok=0
          fi
        done < <(printf '%s\n' "$_init_labels" | tr ',' '\n')
        if [ "$_labels_ok" = "1" ]; then
          log_success "Labels enregistrés : $_init_labels"
        else
          log_warn "Échec enregistrement labels dans Beads"
        fi
      fi
    else
      log_warn "Échec de bd init — initialiser plus tard : ./oc.sh beads init $PROJECT_ID"
    fi
  else
    log_info "Initialiser plus tard : ./oc.sh beads init $PROJECT_ID"
  fi
elif [ -d "$PROJECT_PATH/.beads" ]; then
  BEADS_OK=1
  log_info "Beads déjà initialisé dans ce projet"
else
  log_info "Beads non configuré — dossier absent ou bd indisponible. Initialiser plus tard : ./oc.sh beads init $PROJECT_ID"
fi

# Proposer la configuration du tracker si non-none
if [ "${PROJECT_TRACKER:-none}" != "none" ]; then
  if command -v bd &>/dev/null; then
    echo ""
    _prompt setup_now "Configurer $PROJECT_TRACKER maintenant ? [Y/n] : "
    if [[ "${setup_now:-Y}" =~ ^[Yy]$ ]]; then
      bash "$SCRIPTS_DIR/cmd-beads.sh" tracker setup "$PROJECT_ID"
    else
      log_info "Configurer plus tard : ./oc.sh beads tracker setup $PROJECT_ID"
    fi
  else
    log_info "Configurer le tracker plus tard (bd requis) : ./oc.sh beads tracker setup $PROJECT_ID"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# ÉTAPE 3 — Agents & cibles
# ─────────────────────────────────────────────────────────────────────────────
_step 3 5 "Agents & cibles"

_prompt select_agents "Sélectionner les agents à déployer ? [y/N] : "
AGENTS_SUMMARY=""
# shellcheck disable=SC2154
if [[ "$select_agents" =~ ^[Yy]$ ]]; then
  PICKED_AGENTS=""
  _pick_agents "all"
  if [ -n "$PICKED_AGENTS" ] && [ "$PICKED_AGENTS" != "all" ]; then
    _set_project_agents "$PROJECT_ID" "$PICKED_AGENTS"
    _agent_count=$(echo "$PICKED_AGENTS" | tr ',' '\n' | wc -l | tr -d ' ')
    log_success "$_agent_count agent(s) sélectionné(s) pour $PROJECT_ID"
    AGENTS_SUMMARY="${_agent_count} sélectionné(s)"
  else
    log_info "Tous les agents seront déployés (par défaut)"
    AGENTS_SUMMARY="tous (par défaut)"
  fi
else
  log_info "Tous les agents seront déployés (par défaut)"
  AGENTS_SUMMARY="tous (par défaut)"
fi

# ── Agents natifs OpenCode (désactivation) ────────────────────────────────────
# opencode est toujours la cible active
_is_opencode_target=true

if [ "$_is_opencode_target" = true ] && [ -t 0 ]; then
  echo ""
  _hub_disabled=$(get_hub_disabled_native_agents)
  if [ -n "$_hub_disabled" ]; then
    echo -e "  Agents natifs désactivés par défaut (hub) : ${BOLD}${_hub_disabled}${RESET}"
  else
    echo -e "  ${DIM}Aucun agent natif désactivé au niveau hub${RESET}"
  fi
  _prompt disable_native "Surcharger les agents désactivés pour ce projet ? [y/N] : "
  # shellcheck disable=SC2154
  if [[ "$disable_native" =~ ^[Yy]$ ]]; then
    PICKED_DISABLED_AGENTS=""
    _pick_native_agents "${_hub_disabled}"
    _set_project_disabled_native_agents "$PROJECT_ID" "$PICKED_DISABLED_AGENTS"
    if [ -n "$PICKED_DISABLED_AGENTS" ]; then
      log_success "Agents désactivés pour $PROJECT_ID : $PICKED_DISABLED_AGENTS"
    else
      log_success "Aucun agent désactivé pour $PROJECT_ID (tous actifs)"
    fi
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# ÉTAPE 4 — Fournisseur LLM (optionnel, surcharge le hub)
# ─────────────────────────────────────────────────────────────────────────────
_step 4 5 "Fournisseur LLM"

# Afficher le fournisseur actuel du hub comme contexte
_hub_provider_name=$(get_hub_default_provider 2>/dev/null || echo "")
_hub_provider_label=""
if [ -n "$_hub_provider_name" ]; then
  _hub_provider_label=$(get_provider_info "$_hub_provider_name" "label" 2>/dev/null || echo "$_hub_provider_name")
fi

if [ -n "$_hub_provider_label" ]; then
  echo -e "  Fournisseur par défaut du hub : ${BOLD}${_hub_provider_label}${RESET}"
else
  echo -e "  ${DIM}Aucun fournisseur configuré au niveau hub${RESET}"
fi
echo ""

_prompt setup_provider "Utiliser un fournisseur différent pour ce projet ? [y/N] : "
PROVIDER_SUMMARY=""
# shellcheck disable=SC2154
if [[ "$setup_provider" =~ ^[Yy]$ ]]; then
  echo ""

  # Menu dynamique depuis providers.json
  _init_provider_names=()
  if [ -f "$PROVIDERS_FILE" ] && command -v jq &>/dev/null; then
    while IFS= read -r pname; do
      _init_provider_names+=("$pname")
    done < <(jq -r '.providers | keys[]' "$PROVIDERS_FILE")
  else
    _init_provider_names=("anthropic" "mammouth" "github-models" "bedrock" "ollama")
  fi

  _pi=1
  for pname in "${_init_provider_names[@]}"; do
    _plabel=$(get_provider_info "$pname" "label" 2>/dev/null || echo "$pname")
    printf "  %d. %s\n" "$_pi" "$_plabel"
    _pi=$((_pi + 1))
  done
  echo ""

  _prompt _proj_choice "Choisir (1-${#_init_provider_names[@]}) : "

  _proj_provider=""
  # shellcheck disable=SC2154
  if [[ "$_proj_choice" =~ ^[0-9]+$ ]] && [ "$_proj_choice" -ge 1 ] && [ "$_proj_choice" -le "${#_init_provider_names[@]}" ]; then
    _proj_provider="${_init_provider_names[$((_proj_choice - 1))]}"
  fi

  if [ -n "$_proj_provider" ]; then
    _proj_label=$(get_provider_info "$_proj_provider" "label" 2>/dev/null || echo "$_proj_provider")
    _proj_requires_key=$(get_provider_info "$_proj_provider" "requires_api_key" 2>/dev/null || echo "true")
    _proj_default_url=$(get_provider_info "$_proj_provider" "default_base_url" 2>/dev/null || echo "")
    _proj_requires_url=$(get_provider_info "$_proj_provider" "requires_base_url" 2>/dev/null || echo "false")

    _proj_api_key=""
    _proj_base_url="$_proj_default_url"

    if [ "$_proj_requires_key" = "true" ]; then
      echo ""
      trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
      read -rsp "  Clé API ${_proj_label} (laisser vide pour ignorer) : " _proj_api_key
      stty echo 2>/dev/null
      trap - INT TERM
      echo ""
    fi

    if [ "$_proj_requires_url" = "true" ] && [ -n "$_proj_default_url" ]; then
      echo ""
      _prompt _proj_url_input "  URL de base [${_proj_default_url}] : "
      _proj_base_url="${_proj_url_input:-$_proj_default_url}"
    fi

    _proj_should_save=false
    [ -n "$_proj_api_key" ] && _proj_should_save=true
    [ "$_proj_requires_key" = "false" ] && _proj_should_save=true

    if [ "$_proj_should_save" = "true" ]; then
      bash "$SCRIPTS_DIR/cmd-config.sh" set "$PROJECT_ID" \
        --provider "$_proj_provider" --api-key "${_proj_api_key}" ${_proj_base_url:+--base-url "${_proj_base_url}"} 2>/dev/null \
        && log_success "Fournisseur configuré pour ${PROJECT_ID} : ${_proj_label}" \
        && PROVIDER_SUMMARY="${_proj_label} (projet)" \
        || { log_warn "Impossible de configurer le fournisseur — réessayer : ./oc.sh config set ${PROJECT_ID}"; PROVIDER_SUMMARY="erreur de configuration"; }
    else
      log_info "Fournisseur non configuré — le hub sera utilisé par défaut"
      PROVIDER_SUMMARY="${_hub_provider_label:-hub par défaut}"
    fi
  else
    log_info "Fournisseur non configuré — le hub sera utilisé par défaut"
    PROVIDER_SUMMARY="${_hub_provider_label:-hub par défaut}"
  fi
else
  if [ -n "$_hub_provider_label" ]; then
    log_info "Fournisseur du hub utilisé : ${_hub_provider_label}"
    PROVIDER_SUMMARY="${_hub_provider_label} (depuis le hub)"
  else
    log_info "Fournisseur non configuré — utiliser : ./oc.sh provider set-default"
    PROVIDER_SUMMARY="non configuré"
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# ÉTAPE 5 — Déploiement
# ─────────────────────────────────────────────────────────────────────────────
_step 5 5 "Déploiement"

if [ -d "$PROJECT_PATH" ]; then
  DEPLOYED="non"
  if [ -t 0 ]; then
    _prompt deploy_now "Déployer les agents maintenant ? [Y/n] : "
  fi
  if [ -t 0 ] && [[ "${deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    bash "$SCRIPTS_DIR/cmd-deploy.sh" all "$PROJECT_ID"
    DEPLOYED="oui"
  else
    log_info "Déployer plus tard : ./oc.sh deploy all $PROJECT_ID"
  fi

  # Proposition d'ajout de opencode.json et .opencode/ au .git/info/exclude du projet
  # (utilise exclude plutôt que .gitignore pour ne pas polluer le dépôt partagé)
  if [ -t 0 ]; then
    _prompt add_gitignore "$(t init.gitignore_opencode_prompt)"
    if [[ "${add_gitignore:-N}" =~ ^[Yy]$ ]]; then
      _exclude_dir="$PROJECT_PATH/.git/info"
      _exclude_file="$_exclude_dir/exclude"
      # S'assurer que .git/info/ existe (cas git init récent)
      mkdir -p "$_exclude_dir"
      _already=true
      if [ ! -f "$_exclude_file" ] || ! grep -qx "opencode.json" "$_exclude_file"; then
        echo "opencode.json" >> "$_exclude_file"
        _already=false
      fi
      if [ ! -f "$_exclude_file" ] || ! grep -qx ".opencode/" "$_exclude_file"; then
        echo ".opencode/" >> "$_exclude_file"
        _already=false
      fi
      if [ "$_already" = false ]; then
        log_info "$(t init.gitignore_opencode_added)"
      else
        log_info "$(t init.gitignore_opencode_exists)"
      fi
    fi
  fi
else
  log_warn "Déploiement impossible — dossier $PROJECT_PATH introuvable"
  DEPLOYED="impossible (dossier absent)"
fi

# ─────────────────────────────────────────────────────────────────────────────
# RÉCAPITULATIF
# ─────────────────────────────────────────────────────────────────────────────
_summary \
  "$PROJECT_ID" \
  "$PROJECT_PATH" \
  "${PROJECT_NAME:-}" \
  "${PROJECT_STACK:-}" \
  "${PROJECT_TRACKER:-none}" \
  "$BEADS_OK" \
  "${GIT_REMOTE_STATUS:-}" \
  "${AGENTS_SUMMARY:-}" \
  "${PROVIDER_SUMMARY:-}" \
  "${DEPLOYED:-}"
_outro "Projet ${PROJECT_ID} prêt — ./oc.sh start ${PROJECT_ID}"
