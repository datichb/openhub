#!/bin/bash
set -euo pipefail

# ── Charger le module board ───────────────
# shellcheck source=scripts/cmd-board.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cmd-board.sh"

# ── Vérifier que bd est disponible ────────
_require_bd() {
  if ! command -v bd &>/dev/null; then
    log_error "$(t beads.not_installed)"
    log_info  "$(t beads.install_hint)"
    exit 1
  fi
}

# ── Aide interne ──────────────────────────
_beads_usage() {
  _h_section "oc beads <subcommand> [options]"

  _h_cmd "status"                              "$(t help.beads_status.desc)"
  _h_sub "  -p, --project <id>"               "Interactif si absent"
  _h_cmd "init"                                "$(t help.beads_init.desc)"
  _h_sub "  -p, --project <id>"               "Requis"
  _h_cmd "list"                                "$(t help.beads_list.desc)"
  _h_sub "  -p, --project <id>"               "Requis"
  _h_cmd "show"                                "$(t help.beads_show.desc)"
  _h_sub "  -p, --project <id>  --ticket/-i <id>" "Requis"
  _h_cmd "create"                              "$(t help.beads_create.desc)"
  _h_sub "  -p, --project <id>  --title/-T <title>" ""
  _h_sub "  --label/-l <l>  --type/-t <t>  --desc/-d <d>" ""
  _h_cmd "open"                                "$(t help.beads_open.desc)"
  _h_sub "  -p, --project <id>"               "Requis"

  _h_section "$(t beads.tracker.title)"
  _h_cmd "sync"                                "$(t help.beads_sync.desc)"
  _h_sub "  -p, --project <id>  [pull|push]  --dry-run/-n" ""
  _h_cmd "tracker status"                      "$(t help.beads_tracker_status.desc)"
  _h_cmd "tracker setup"                       "$(t help.beads_tracker_setup.desc)"
  _h_cmd "tracker switch"                      "$(t help.beads_tracker_switch.desc)"
  _h_cmd "tracker set-sync-mode [mode]"        "$(t help.beads_tracker_set_sync_mode.desc)"
  _h_sub "  -p, --project <id>"               "Requis"

  _h_section "$(t beads.board.title)"
  _h_cmd "board"                               "$(t help.beads_board.desc)"
  _h_sub "  -p, --project <id>  --watch/-w"   ""

  echo ""
}

# ── Vérifier que bd est disponible ────────

# ── Vérifier que .beads/ existe ───────────
_require_beads_init() {
  local path="$1" id="$2"
  if [ ! -d "$path/.beads" ]; then
    log_error "$(t beads.not_initialized) $id → ./oc.sh beads init $id"
    exit 1
  fi
}

# ── Résoudre le tracker d'un projet ───────
_resolve_tracker() {
  local id="$1"
  local tracker
  tracker=$(get_project_tracker "$id")
  # Normaliser en minuscules (protection contre casse incorrecte dans projects.md)
  tracker=$(echo "$tracker" | tr '[:upper:]' '[:lower:]')
  if [ "$tracker" = "none" ] || [ -z "$tracker" ]; then
    log_error "$(t beads.no_tracker) $id"
    log_info  "$(t beads.tracker.configure) $id"
    exit 1
  fi
  echo "$tracker"
}

# ── Mettre à jour le champ Tracker dans projects.md ──
_set_project_tracker() {
  local id="$1" new_tracker="$2"
  # Whitelist stricte — protège aussi contre l'injection Perl
  case "$new_tracker" in
    jira|gitlab|none) ;;
    *) log_error "Tracker invalide : $new_tracker (jira | gitlab | none)"; exit 1 ;;
  esac
  # Tente de remplacer une ligne "- Tracker : *" existante dans le bloc du projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Tracker : \S+)}{\${1}- Tracker : ${new_tracker}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Tracker : ${new_tracker}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Si le champ n'existe pas encore, l'ajouter après "- Labels :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Labels : [^\n]+\n)}{\${1}- Tracker : ${new_tracker}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Tracker : ${new_tracker}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : si "- Labels :" absent, ajouter après le dernier champ "- " du bloc projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- [^\n]+\n)}{\${1}- Tracker : ${new_tracker}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Tracker : ${new_tracker}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Tracker dans le bloc $id de projects.md"
  return 1
}

# ══════════════════════════════════════════
# Sous-commande : status
# ══════════════════════════════════════════
cmd_status() {
  local id="${1:-}"

  if [ -z "$id" ]; then
    log_title "$(t beads.status.all)"
    echo ""
    while IFS= read -r pid; do
      local path tracker
      path=$(get_project_path "$pid" 2>/dev/null)
      path="${path/#\~/$HOME}"
      tracker=$(get_project_tracker "$pid")
      local beads_icon tracker_str
      # Protection : si path vide, ne pas tester /.beads (racine)
      if [ -n "$path" ] && [ -d "$path/.beads" ]; then
        beads_icon="${GREEN}✔${RESET}"
      else
        beads_icon="${YELLOW}✘${RESET}"
      fi
      [ "$tracker" = "none" ] && tracker_str="" || tracker_str="  [${BLUE}${tracker}${RESET}]"
      echo -e "  ${beads_icon}  $pid${tracker_str}  ${path}"
    done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')
    echo ""
    return
  fi

  id=$(normalize_project_id "$id")
  local path
  path=$(resolve_project_path "$id")

  if [ -d "$path/.beads" ]; then
    log_success "$(t beads.initialized) $id ($path/.beads)"
  else
    log_warn "$(t beads.not_initialized) $id"
    log_info  "Lancez : ./oc.sh beads init $id"
  fi

  local tracker
  tracker=$(get_project_tracker "$id")
  if [ "$tracker" != "none" ]; then
    log_info "Tracker : $tracker"
  else
    log_info "Tracker : aucun  (configurer : ./oc.sh beads tracker setup $id)"
  fi
}

# ══════════════════════════════════════════
# Sous-commande : init
# ══════════════════════════════════════════
cmd_init() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")

  if [ -d "$path/.beads" ]; then
    log_warn "$(t beads.already_initialized) $id ($path/.beads)"
    exit 0
  fi

  log_info "Initialisation de Beads dans : $path"
  bd -C "$path" init --prefix "$id" --skip-hooks || { log_error "$(t beads.init_failed)"; exit 1; }
  log_success "$(t beads.initialized) $id ($path/.beads)"

  # Proposer de configurer l'upstream git si absent (ni upstream ni origin trouvé)
  if ! (cd "$path" && git remote get-url upstream) &>/dev/null && \
     ! (cd "$path" && git remote get-url origin) &>/dev/null; then
    echo ""
    read -rp "  Configurer l'upstream Git (git remote add upstream) ? [Y/n] : " _setup_upstream || true
    if [[ "${_setup_upstream:-Y}" =~ ^[Yy]$ ]]; then
      read -rp "  URL du remote upstream : " _upstream_url || true
      if [ -n "$_upstream_url" ]; then
        if (cd "$path" && git remote add upstream "$_upstream_url"); then
          log_success "Remote upstream configuré : $_upstream_url"
        else
          log_warn "Échec de la configuration upstream — configurer manuellement"
        fi
      else
        log_warn "URL vide — configurer plus tard : git remote add upstream <url>"
      fi
    else
      log_info "Configurer plus tard : git remote add upstream <url>"
    fi
  fi

  # Enregistrer les labels de projects.md dans la config bd
  local labels
  labels=$(get_project_labels "$id")
  if [ -n "$labels" ]; then
    log_info "Enregistrement des labels dans la config Beads…"
    local _labels_ok=1
    while IFS= read -r _lbl; do
      _lbl=$(printf '%s' "$_lbl" | sed 's/^ *//;s/ *$//')
      [ -z "$_lbl" ] && continue
      if ! bd -C "$path" label create "$_lbl"; then
        _labels_ok=0
      fi
    done < <(printf '%s\n' "$labels" | tr ',' '\n')
    if [ "$_labels_ok" = "1" ]; then
      log_success "$(t beads.labels.registered) $labels"
    else
      log_warn "$(t beads.labels.failed)"
    fi
  fi

  # Importer les labels depuis le tracker distant si configuré (GitLab ou Jira)
  _fetch_tracker_labels "$id" "$path"

  # Ajouter .beads/ et AGENTS.md au .git/info/exclude du projet (exclusion locale)
  local _exclude_dir="$path/.git/info"
  local _exclude_file="$_exclude_dir/exclude"
  mkdir -p "$_exclude_dir"
  if [ ! -f "$_exclude_file" ] || ! grep -qx ".beads/" "$_exclude_file"; then
    echo ".beads/" >> "$_exclude_file"
    log_info "$(t beads.gitignore_added)"
  else
    log_info "$(t beads.gitignore_exists)"
  fi
  if [ ! -f "$_exclude_file" ] || ! grep -qx "AGENTS.md" "$_exclude_file"; then
    echo "AGENTS.md" >> "$_exclude_file"
  fi
}

# ══════════════════════════════════════════
# Sous-commande : list
# ══════════════════════════════════════════
cmd_list() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  log_title "$(t beads.status.open_tickets) $id"
  echo ""
  bd -C "$path" list -s open || { log_error "Échec de bd list"; exit 1; }
}

# ══════════════════════════════════════════
# Sous-commande : show
# ══════════════════════════════════════════
cmd_show() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  local ticket_id="${2:-}"
  if [ -z "$ticket_id" ]; then
    log_error "Usage : oc beads show <PROJECT_ID> <TICKET_ID>"
    exit 1
  fi

  bd -C "$path" show "$ticket_id" || { log_error "Ticket introuvable : $ticket_id"; exit 1; }
}

# ══════════════════════════════════════════
# Sous-commande : open
# ══════════════════════════════════════════
cmd_open() {
  require_project_id "$1"

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")

  log_info "Répertoire du projet $id : $path"
  if ! command -v bd &>/dev/null; then
    log_warn "$(t beads.not_installed) — $(t beads.install_hint)"
  fi
  log_info "$(t beads.open_hint)"
  echo ""
  echo "  cd $path"
  echo "  bd list -s open"
}

# ══════════════════════════════════════════
# Sous-commande : sync
# ══════════════════════════════════════════
cmd_sync() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  shift
  local extra_flags=("$@")   # --pull-only, --push-only, --dry-run, etc.

  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  local tracker
  tracker=$(_resolve_tracker "$id")

  # Appliquer le sync mode du projet si aucun flag de direction n'est passé en CLI
  # Accepte l'ancienne syntaxe (--pull-only / --push-only) et la nouvelle (pull / push)
  local sync_direction="" remaining_flags=()
  for f in "${extra_flags[@]+"${extra_flags[@]}"}"; do
    case "$f" in
      --pull-only|pull) sync_direction="pull" ;;
      --push-only|push) sync_direction="push" ;;
      *) remaining_flags+=("$f") ;;
    esac
  done

  if [ -z "$sync_direction" ]; then
    local sync_mode
    sync_mode=$(get_project_sync_mode "$id")
    case "$sync_mode" in
      pull-only)  sync_direction="pull" ;;
      push-only)  sync_direction="push" ;;
      bidirectional|*) ;; # pas de direction à injecter
    esac
    [ -n "$sync_direction" ] && log_info "$(t beads.sync_mode.applied) ${sync_mode}  [$id]"
  fi

  log_info "Sync $tracker ← → Beads  [$id]"
  bd -C "$path" "$tracker" sync ${sync_direction:+"$sync_direction"} \
      ${remaining_flags[@]+"${remaining_flags[@]}"} \
      || { log_error "$(t beads.sync.failed) $tracker"; exit 1; }
  log_success "$(t beads.sync.done) $id"
}

# ══════════════════════════════════════════
# Sous-commandes : tracker *
# ══════════════════════════════════════════

# tracker status — affiche le tracker et l'état bd <tracker> status
cmd_tracker_status() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  local tracker
  tracker=$(_resolve_tracker "$id")

  log_info "Tracker : $tracker  [$id]"
  echo ""
  bd -C "$path" "$tracker" status || { log_error "Échec de bd $tracker status"; exit 1; }
}

# tracker setup — configuration interactive des credentials
cmd_tracker_setup() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  local tracker
  tracker=$(get_project_tracker "$id")

  # Si pas de tracker défini, proposer le choix
  if [ "$tracker" = "none" ] || [ -z "$tracker" ]; then
    echo ""
    echo -e "${BOLD}Choisir un tracker pour $id :${RESET}"
    echo "  1) Jira"
    echo "  2) GitLab"
    echo ""
    read -rp "  Choix : " choice
    case "$choice" in
      1) tracker="jira" ;;
      2) tracker="gitlab" ;;
      *) log_error "$(t beads.tracker.invalid)"; exit 1 ;;
    esac
    _set_project_tracker "$id" "$tracker"
    log_success "Tracker $tracker enregistré pour $id"
  fi

  echo ""
  log_info "Configuration de $tracker pour le projet $id"
  echo ""

  case "$tracker" in
    jira)
      _setup_jira "$path" "$id"
      ;;
    gitlab)
      _setup_gitlab "$path" "$id"
      ;;
  esac
}

# tracker switch — changer de tracker
cmd_tracker_switch() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  local path
  path=$(resolve_project_path "$id")

  local current
  current=$(get_project_tracker "$id")
  log_info "Tracker actuel : ${current}"
  echo ""
  echo -e "${BOLD}Nouveau tracker pour $id :${RESET}"
  echo "  1) Jira"
  echo "  2) GitLab"
  echo "  3) Aucun"
  echo ""
  read -rp "  Choix : " choice
  local new_tracker
  case "$choice" in
    1) new_tracker="jira" ;;
    2) new_tracker="gitlab" ;;
    3) new_tracker="none" ;;
    *) log_error "$(t beads.tracker.invalid)"; exit 1 ;;
  esac

  # Vérifier que Beads est initialisé avant d'appliquer le changement
  if [ "$new_tracker" != "none" ]; then
    _require_beads_init "$path" "$id"
  fi

  _set_project_tracker "$id" "$new_tracker"
  log_success "Tracker mis à jour : $current → $new_tracker"

  if [ "$new_tracker" != "none" ]; then
    echo ""
    read -rp "  Configurer $new_tracker maintenant ? [Y/n] : " setup_now
    if [[ "${setup_now:-Y}" =~ ^[Yy]$ ]]; then
      cmd_tracker_setup "$id"
    else
      log_info "Configurer plus tard : ./oc.sh beads tracker setup $id"
    fi
  fi
}

# ── Helpers de configuration ──────────────

# tracker set-sync-mode — définir le mode de synchronisation par défaut
cmd_tracker_set_sync_mode() {
  require_project_id "$1"

  local id
  id=$(normalize_project_id "$1")
  local new_mode="${2:-}"

  # Mode non-interactif si la valeur est passée en argument
  if [ -n "$new_mode" ]; then
    case "$new_mode" in
      bidirectional|pull-only|push-only) ;;
      *) log_error "$(t beads.sync_mode.invalid)"; exit 1 ;;
    esac
  else
    # Mode interactif
    local current
    current=$(get_project_sync_mode "$id")
    echo ""
    log_info "$(t beads.sync_mode.current) ${current}  [$id]"
    echo ""
    echo -e "${BOLD}$(t beads.sync_mode.choose) $id :${RESET}"
    echo "  1) bidirectional  — $(t beads.sync_mode.desc_bidi)"
    echo "  2) pull-only      — $(t beads.sync_mode.desc_pull)"
    echo "  3) push-only      — $(t beads.sync_mode.desc_push)"
    echo ""
    read -rp "  Choix : " choice
    case "$choice" in
      1) new_mode="bidirectional" ;;
      2) new_mode="pull-only" ;;
      3) new_mode="push-only" ;;
      *) log_error "$(t beads.tracker.invalid)"; exit 1 ;;
    esac
  fi

  _set_project_sync_mode "$id" "$new_mode" \
    || { log_error "$(t beads.sync_mode.save_failed) $id"; exit 1; }
  log_success "$(t beads.sync_mode.saved) $new_mode  [$id]"
}

_setup_jira() {
  local path="$1" id="$2"

  read -rp "  URL Jira (ex: https://company.atlassian.net) : " jira_url
  [ -z "$jira_url" ] && { log_error "URL Jira requise"; exit 1; }
  read -rp "  Clé de projet Jira (ex: PROJ) : " jira_project
  [ -z "$jira_project" ] && { log_error "Clé de projet Jira requise"; exit 1; }
  read -rp "  Email / username Jira : " jira_user
  [ -z "$jira_user" ] && { log_error "Email / username Jira requis"; exit 1; }
  trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
  read -rsp "  API token Jira (masqué) : " jira_token
  stty echo 2>/dev/null; trap - INT TERM
  echo ""
  [ -z "$jira_token" ] && { log_error "Token Jira requis"; exit 1; }

  bd -C "$path" config set-many \
    jira.url="$jira_url" \
    jira.project="$jira_project" \
    jira.username="$jira_user" \
    jira.api_token="$jira_token" \
    || { log_error "Échec de la configuration Jira"; exit 1; }

  log_success "Jira configuré pour $id"
  echo ""
  log_info "Tester la connexion : ./oc.sh beads tracker status $id"
  log_info "Synchroniser        : ./oc.sh beads sync $id pull --dry-run"
}

_setup_gitlab() {
  local path="$1" id="$2"

  read -rp "  URL GitLab (ex: https://gitlab.com ou instance privée) : " gl_url
  [ -z "$gl_url" ] && { log_error "URL GitLab requise"; exit 1; }
  trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
  read -rsp "  Token d'accès personnel GitLab (masqué) : " gl_token
  stty echo 2>/dev/null; trap - INT TERM
  echo ""
  [ -z "$gl_token" ] && { log_error "Token GitLab requis"; exit 1; }

  # Lister les projets accessibles pour aider à trouver l'ID
  echo ""
  log_info "Récupération des projets accessibles avec ce token..."
  (bd -C "$path" config set gitlab.url "$gl_url" \
    && bd -C "$path" config set gitlab.token "$gl_token" \
    && bd -C "$path" gitlab projects) 2>/dev/null && echo "" || log_warn "Impossible de lister les projets (vérifier l'URL et le token)"

  read -rp "  ID ou chemin du projet GitLab (ex: 12345 ou namespace/project) : " gl_project_id
  [ -z "$gl_project_id" ] && { log_error "ID de projet GitLab requis"; exit 1; }

  # Normaliser le project_id : si l'utilisateur a saisi une URL complète, extraire le chemin
  gl_project_id=$(_normalize_gitlab_project_id "$gl_url" "$gl_project_id") \
    || exit 1

  # url et token déjà configurés ci-dessus — ajouter uniquement project_id
  bd -C "$path" config set gitlab.project_id "$gl_project_id" \
    || { log_error "Échec de la configuration GitLab"; exit 1; }

  log_success "GitLab configuré pour $id"
  echo ""

  # Vérifier immédiatement la connexion pour détecter les erreurs de config tôt
  log_info "Vérification de la connexion GitLab..."
  if bd -C "$path" gitlab status 2>/dev/null; then
    log_success "Connexion GitLab OK"
  else
    log_warn "Connexion GitLab échouée — vérifier l'URL, le token et le project_id"
    log_info "  URL          : $gl_url"
    log_info "  Project ID   : $gl_project_id"
    log_info "Relancer après correction : ./oc.sh beads tracker setup $id"
  fi
  echo ""
  log_info "Synchroniser : ./oc.sh beads sync $id --pull-only --dry-run"
}

# ══════════════════════════════════════════
# Import des labels depuis le tracker distant (GitLab ou Jira)
# Usage : _fetch_tracker_labels <project_id> <project_path>
# Appelé automatiquement depuis cmd_init si un tracker est configuré.
# Skip silencieux si aucun tracker configuré ou si jq est absent.
# ══════════════════════════════════════════
_fetch_tracker_labels() {
  local id="$1" path="$2"

  # jq est requis pour parser le JSON des APIs
  if ! command -v jq &>/dev/null; then
    return 0
  fi

  # ── Tentative GitLab ──────────────────────────────────────────────────────
  local gl_url gl_token gl_project_id
  gl_url=$(bd -C "$path"          config get gitlab.url        2>/dev/null || true)
  gl_token=$(bd -C "$path"        config get gitlab.token      2>/dev/null || true)
  gl_project_id=$(bd -C "$path"   config get gitlab.project_id 2>/dev/null || true)

  if [ -n "$gl_url" ] && [ -n "$gl_token" ] && [ -n "$gl_project_id" ]; then
    log_info "$(t beads.labels.fetching_gitlab)"
    local labels_json
    # Encoder le project_id pour l'URL (namespace/project → namespace%2Fproject)
    local encoded_id
    encoded_id=$(printf '%s' "$gl_project_id" | sed 's|/|%2F|g')
    labels_json=$(curl -sf --header "PRIVATE-TOKEN: $gl_token" \
      "${gl_url%/}/api/v4/projects/${encoded_id}/labels?per_page=100" 2>/dev/null || true)
    if [ -n "$labels_json" ]; then
      local count=0
      while IFS= read -r lbl; do
        [ -z "$lbl" ] && continue
        bd -C "$path" label create "$lbl" 2>/dev/null && count=$((count + 1)) || true
      done < <(printf '%s' "$labels_json" | jq -r '.[].name' 2>/dev/null)
      if [ "$count" -gt 0 ]; then
        log_success "$(t beads.labels.imported_gitlab) $count"
      fi
    fi
    return 0
  fi

  # ── Tentative Jira ────────────────────────────────────────────────────────
  local jira_url jira_user jira_token
  jira_url=$(bd -C "$path"    config get jira.url   2>/dev/null || true)
  jira_user=$(bd -C "$path"   config get jira.user  2>/dev/null || true)
  jira_token=$(bd -C "$path"  config get jira.token 2>/dev/null || true)

  if [ -n "$jira_url" ] && [ -n "$jira_user" ] && [ -n "$jira_token" ]; then
    log_info "$(t beads.labels.fetching_jira)"
    local labels_json
    labels_json=$(curl -sf -u "$jira_user:$jira_token" \
      "${jira_url%/}/rest/api/2/label?maxResults=200" 2>/dev/null || true)
    if [ -n "$labels_json" ]; then
      local count=0
      while IFS= read -r lbl; do
        [ -z "$lbl" ] && continue
        bd -C "$path" label create "$lbl" 2>/dev/null && count=$((count + 1)) || true
      done < <(printf '%s' "$labels_json" | jq -r '.values[]' 2>/dev/null)
      if [ "$count" -gt 0 ]; then
        log_success "$(t beads.labels.imported_jira) $count"
      fi
    fi
    return 0
  fi

  # Aucun tracker configuré → skip silencieux
  return 0
}

# Normalise un gitlab.project_id saisi par l'utilisateur.
# - Si la valeur est une URL complète (commence par http:// ou https://),
#   on en extrait le chemin relatif à la base GitLab.
# - Sinon la valeur est retournée telle quelle.
# - Dans tous les cas, le résultat doit être un entier ou un chemin namespace/projet.
_normalize_gitlab_project_id() {
  local gl_url="$1" raw="$2"

  # Cas 1 : valeur numérique → OK directement
  if [[ "$raw" =~ ^[0-9]+$ ]]; then
    printf '%s' "$raw"
    return 0
  fi

  # Cas 2 : URL complète → extraire le chemin relatif à gl_url
  if [[ "$raw" == http://* || "$raw" == https://* ]]; then
    # Normaliser les deux URLs : supprimer le slash final
    local base="${gl_url%/}"
    local full="${raw%/}"

    # Vérifier que l'URL saisie commence bien par la base configurée
    if [[ "$full" == "$base"* ]]; then
      local extracted="${full#"$base"}"
      # Supprimer le slash initial s'il reste
      extracted="${extracted#/}"
      # Supprimer un éventuel suffixe .git
      extracted="${extracted%.git}"
      if [ -z "$extracted" ]; then
        log_error "Impossible d'extraire un chemin de projet depuis : $raw" >&2
        return 1
      fi
      log_warn "URL détectée — project_id extrait automatiquement : $extracted" >&2
      printf '%s' "$extracted"
      return 0
    else
      log_error "L'URL saisie ($raw) ne correspond pas à l'instance GitLab configurée ($gl_url)" >&2
      log_error "Saisir l'ID numérique ou le chemin namespace/project, pas une URL complète" >&2
      return 1
    fi
  fi

  # Cas 3 : chemin namespace/project → OK directement
  printf '%s' "$raw"
  return 0
}

# ══════════════════════════════════════════
# Sous-commande : create
# ══════════════════════════════════════════
cmd_create() {
  require_project_id "$1"
  _require_bd

  local id
  id=$(normalize_project_id "$1")
  shift  # consommer PROJECT_ID — les args restants sont titre + flags

  local path
  path=$(resolve_project_path "$id")
  _require_beads_init "$path" "$id"

  # ── Parser les flags optionnels ────────────────────────────────────────────
  local title="" label="" type="" desc=""
  local positional_done=false
  while [ $# -gt 0 ]; do
    case "$1" in
      --label) shift; label="${1:-}"; shift ;;
      --type)  shift; type="${1:-}";  shift ;;
      --desc)  shift; desc="${1:-}";  shift ;;
      --*)     log_error "Flag inconnu : $1"; exit 1 ;;
      *)
        if [ "$positional_done" = false ]; then
          title="$1"
          positional_done=true
        fi
        shift
        ;;
    esac
  done

  # ── Mode non-interactif (titre fourni) ─────────────────────────────────────
  if [ -n "$title" ]; then
    local bd_args=("create" "$title")
    [ -n "$label" ] && bd_args+=("--label" "$label")
    [ -n "$type"  ] && bd_args+=("--type"  "$type")
    [ -n "$desc"  ] && bd_args+=("--desc"  "$desc")

    log_info "$(t beads.create.creating) ${id}…"
    bd -C "$path" "${bd_args[@]}" || { log_error "$(t beads.create.failed)"; exit 1; }
    return 0
  fi

  # ── Mode interactif minimal ────────────────────────────────────────────────
  log_title "$(t beads.create.title) $id"
  echo ""

  read -rp "  Titre : " title
  [ -z "$title" ] && { log_error "$(t beads.create.title_required)"; exit 1; }

  if [ -z "$label" ]; then
    read -rp "  Label (laisser vide pour ignorer) : " label
  fi

  if [ -z "$type" ]; then
    read -rp "  Type (feature|fix|chore, laisser vide pour ignorer) : " type
  fi

  local bd_args=("create" "$title")
  [ -n "$label" ] && bd_args+=("--label" "$label")
  [ -n "$type"  ] && bd_args+=("--type"  "$type")
  [ -n "$desc"  ] && bd_args+=("--desc"  "$desc")

  echo ""
  log_info "$(t beads.create.creating) ${id}…"
  bd -C "$path" "${bd_args[@]}" || { log_error "$(t beads.create.failed)"; exit 1; }
}

# ── Dispatch (exécuté seulement si le script est lancé directement) ────────
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then  source "$(cd "$(dirname "$0")" && pwd)/common.sh"
  resolve_oc_lang

  SUBCMD="${1:-}"
  shift || true

  # ── Parser --project/-p et les flags spécifiques depuis les args restants ──
  _BEADS_PROJECT=""
  _BEADS_TITLE=""
  _BEADS_LABEL=""
  _BEADS_TYPE=""
  _BEADS_DESC=""
  _BEADS_TICKET=""
  _BEADS_SYNC_DIR=""
  _BEADS_SYNC_FLAGS=()
  _BEADS_WATCH=false
  _beads_prev=""
  for _arg in "$@"; do
    case "$_beads_prev" in
      --project|-p) _BEADS_PROJECT="$_arg";    _beads_prev=""; continue ;;
      --title|-T)   _BEADS_TITLE="$_arg";      _beads_prev=""; continue ;;
      --label|-l)   _BEADS_LABEL="$_arg";      _beads_prev=""; continue ;;
      --type|-t)    _BEADS_TYPE="$_arg";        _beads_prev=""; continue ;;
      --desc|-d)    _BEADS_DESC="$_arg";        _beads_prev=""; continue ;;
      --ticket|-i)  _BEADS_TICKET="$_arg";      _beads_prev=""; continue ;;
    esac
    case "$_arg" in
      --project|-p) _beads_prev="$_arg" ;;
      --title|-T)   _beads_prev="$_arg" ;;
      --label|-l)   _beads_prev="$_arg" ;;
      --type|-t)    _beads_prev="$_arg" ;;
      --desc|-d)    _beads_prev="$_arg" ;;
      --ticket|-i)  _beads_prev="$_arg" ;;
      --watch|-w)   _BEADS_WATCH=true ;;
      pull|push)    _BEADS_SYNC_DIR="$_arg" ;;
      --dry-run|-n) _BEADS_SYNC_FLAGS+=("$_arg") ;;
    esac
  done

  # ── Résolution interactive du projet si absent ────────────────────────────
  if [ -z "$_BEADS_PROJECT" ] && [ "$SUBCMD" != "" ] && [ "$SUBCMD" != "--help" ] && [ "$SUBCMD" != "-h" ]; then
    _ids=()
    while IFS= read -r _line; do _ids+=("$_line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')
    if [ ${#_ids[@]} -gt 0 ]; then
      echo -e "${BOLD}Choisir un projet :${RESET}"
      echo ""
      for _i in "${!_ids[@]}"; do
        printf "  ${BLUE}%d${RESET}) %s\n" "$((_i+1))" "${_ids[$_i]}"
      done
      echo ""
      read -rp "  Numéro : " _choice
      if ! [[ "$_choice" =~ ^[0-9]+$ ]] || [ "$_choice" -lt 1 ] || [ "$_choice" -gt "${#_ids[@]}" ]; then
        log_error "Choix invalide : $_choice"
        exit 1
      fi
      _BEADS_PROJECT="${_ids[$((_choice-1))]}"
    fi
  fi

  case "$SUBCMD" in
    status)  cmd_status  "$_BEADS_PROJECT" ;;
    init)    cmd_init    "$_BEADS_PROJECT" ;;
    list)    cmd_list    "$_BEADS_PROJECT" ;;
    show)    cmd_show    "$_BEADS_PROJECT" "$_BEADS_TICKET" ;;
    create)  cmd_create  "$_BEADS_PROJECT" "$_BEADS_TITLE" \
               ${_BEADS_LABEL:+--label "$_BEADS_LABEL"} \
               ${_BEADS_TYPE:+--type  "$_BEADS_TYPE"}  \
               ${_BEADS_DESC:+--desc  "$_BEADS_DESC"} ;;
    open)    cmd_open    "$_BEADS_PROJECT" ;;
    sync)    cmd_sync    "$_BEADS_PROJECT" \
               ${_BEADS_SYNC_DIR:+"$_BEADS_SYNC_DIR"} \
               ${_BEADS_SYNC_FLAGS[@]+"${_BEADS_SYNC_FLAGS[@]}"} ;;
    board)   cmd_board   "$_BEADS_PROJECT" ${_BEADS_WATCH:+--watch} ;;
    tracker)
      _TRACKER_SUBCMD="${1:-}"
      case "$_TRACKER_SUBCMD" in
        status)        cmd_tracker_status        "$_BEADS_PROJECT" ;;
        setup)         cmd_tracker_setup         "$_BEADS_PROJECT" ;;
        switch)        cmd_tracker_switch        "$_BEADS_PROJECT" ;;
        set-sync-mode) cmd_tracker_set_sync_mode "$_BEADS_PROJECT" "${2:-}" ;;
        ""|--help|-h)
          echo ""
          echo -e "${BOLD}$(t beads.tracker.usage)${RESET}"
          ;;
        *) log_error "$(t beads.tracker.unknown_subcmd) $_TRACKER_SUBCMD"; exit 1 ;;
      esac
      ;;
    ""|--help|-h) _beads_usage ;;
    *)
      log_error "Sous-commande inconnue : $SUBCMD"
      _beads_usage
      exit 1
      ;;
  esac
fi

