#!/bin/bash
# Gestion des skills externes (context7, etc.)
# Usage : ./oc.sh skills <commande> [args]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
resolve_oc_lang

# SOURCES_FILE : provenance des skills externes
SOURCES_FILE="$EXTERNAL_SKILLS_DIR/.sources.json"
# Dossier d'installation universelle ctx7 — utilisé comme relais avant copie locale
UNIVERSAL_SKILLS_DIR="$HOME/.agents/skills"

# ── HELPERS ──────────────────────────────────────────────────────────────────

# Vérifie que npx est disponible (requis pour ctx7)
_require_npx() {
  if ! command -v npx &>/dev/null; then
    log_error "npx n'est pas disponible. Installez Node.js → https://nodejs.org"
    exit 1
  fi
}

# Initialise le dossier external et le fichier de sources s'ils n'existent pas
_init_external() {
  mkdir -p "$EXTERNAL_SKILLS_DIR"
  if [ ! -f "$SOURCES_FILE" ]; then
    echo '{}' > "$SOURCES_FILE"
  fi
}

# Enregistre la provenance d'un skill dans .sources.json
_record_source() {
  local skill_name="$1" repo="$2" downloaded_at="$3"
  if ! command -v jq &>/dev/null; then
    log_warn "jq non disponible — provenance non enregistrée."
    return
  fi
  local tmp
  tmp=$(mktemp)
  jq --arg name "$skill_name" \
     --arg repo "$repo" \
     --arg date "$downloaded_at" \
     '.[$name] = {repo: $repo, downloaded_at: $date}' \
     "$SOURCES_FILE" > "$tmp" && mv "$tmp" "$SOURCES_FILE"
}

# Supprime la provenance d'un skill de .sources.json
_remove_source() {
  local skill_name="$1"
  if ! command -v jq &>/dev/null; then
    return
  fi
  local tmp
  tmp=$(mktemp)
  jq --arg name "$skill_name" 'del(.[$name])' "$SOURCES_FILE" > "$tmp" && mv "$tmp" "$SOURCES_FILE"
}

# ── SEARCH ───────────────────────────────────────────────────────────────────

##
# Recherche des skills sur context7 par mot-clé.
# @param {string} $1 — Terme de recherche
##
cmd_search() {
  local query="${1:-}"
  if [ -z "$query" ]; then
    log_error "$(t skills.search.usage)"
    log_info  "Exemple : oc skills search pdf"
    exit 1
  fi
  _require_npx
  log_title "$(t skills.search.title) \"$query\""
  echo ""
  # On autorise un code de retour non-nul (ex: aucun résultat) sans interrompre le script
  npx ctx7 skills search "$query" || true
}

# ── INFO ─────────────────────────────────────────────────────────────────────

##
# Prévisualise les skills disponibles dans un dépôt context7 sans les installer.
# @param {string} $1 — Identifiant du dépôt au format /owner/repo
##
cmd_info() {
  local repo="${1:-}"
  if [ -z "$repo" ]; then
    log_error "$(t skills.info.usage)"
    log_info  "Exemple : oc skills info /anthropics/skills"
    exit 1
  fi
  _require_npx
  log_title "$(t skills.available) dans $repo"
  echo ""
  npx ctx7 skills info "$repo" || true
}

# ── ADD ──────────────────────────────────────────────────────────────────────

##
# Télécharge un skill depuis context7 et l'ajoute dans skills/external/.
# Utilise ctx7 CLI comme relais : install dans ~/.agents/skills/ puis copie locale.
# @param {string} $1 — Identifiant du dépôt au format /owner/repo
# @param {string} $2 — (optionnel) Nom du skill à installer
# @param {string} $3 — (optionnel) --force : ne pas demander confirmation si le fichier existe
##
cmd_add() {
  local repo="${1:-}" skill_name="${2:-}" force="${3:-}"
  if [ -z "$repo" ]; then
    log_error "$(t skills.add.usage)"
    log_info  "Exemple : oc skills add /anthropics/skills pdf"
    log_info  "Pour voir les skills disponibles : oc skills info /owner/repo"
    exit 1
  fi
  _require_npx
  _init_external

  log_title "$(t skills.add.title)"

  # Si aucun nom fourni, afficher la liste et demander
  if [ -z "$skill_name" ]; then
    log_info "$(t skills.available) dans $repo :"
    echo ""
    npx ctx7 skills info "$repo" 2>/dev/null || true
    echo ""
    read -rp "Nom du skill à installer : " skill_name
    if [ -z "$skill_name" ]; then
      log_error "Nom de skill requis. $(t cancelled)."
      exit 1
    fi
  fi

  local dest="$EXTERNAL_SKILLS_DIR/${skill_name}.md"

  # Confirmation si le skill existe déjà localement (sauf en mode --force)
  if [ -f "$dest" ] && [ "$force" != "--force" ]; then
    log_warn "$(t skills.add.already_exists) 'external/${skill_name}'"
    read -rp "$(t skills.add.overwrite)" confirm
    [[ "$confirm" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)."; exit 0; }
  fi

  log_info "Téléchargement de '$skill_name' depuis $repo via ctx7..."
  mkdir -p "$UNIVERSAL_SKILLS_DIR"

  # Le chemin attendu après installation ctx7 --universal
  local installed_file="$UNIVERSAL_SKILLS_DIR/${skill_name}.md"

  # Si déjà présent dans le cache universel, on réutilise (ctx7 peut échouer sinon)
  if [ ! -f "$installed_file" ]; then
    npx ctx7 skills install "$repo" "$skill_name" --universal

    # Fallback : recherche par nom partiel si le fichier n'a pas l'extension .md attendue
    if [ ! -f "$installed_file" ]; then
      installed_file=$(find "$UNIVERSAL_SKILLS_DIR" -iname "${skill_name}.*" -type f 2>/dev/null | head -1)
    fi
  fi

  if [ -z "$installed_file" ] || [ ! -f "$installed_file" ]; then
    log_error "Skill installé mais introuvable dans $UNIVERSAL_SKILLS_DIR"
    log_warn  "Vérifiez manuellement le contenu de ce dossier."
    exit 1
  fi

  # Copie vers skills/external/ — seule cette copie est utilisée par le hub
  cp "$installed_file" "$dest"

  # Enregistrement de la provenance
  local now
  now=$(date +%Y-%m-%d)
  _record_source "$skill_name" "$repo" "$now"

  log_success "$(t skills.add.added) skills/external/${skill_name}.md"
  echo ""
  log_info "$(t skills.add.use_hint)"
}

# ── LIST ─────────────────────────────────────────────────────────────────────

##
# Liste tous les skills disponibles : locaux (skills/) et externes (skills/external/).
# Les skills génériques et les skills spécifiques aux stacks (developer/stacks/)
# sont affichés dans des groupes distincts pour une meilleure lisibilité.
##
cmd_list() {
  log_title "$(t skills.available)"
  echo ""

  echo -e "${BOLD}$(t skills.local)${RESET}"
  local found_local=0

  # Collecter tous les skills locaux (hors external/)
  local all_local_skills=()
  while IFS= read -r f; do
    all_local_skills+=("${f#$HUB_DIR/skills/}")
    found_local=1
  done < <(find "$HUB_DIR/skills" -name "*.md" -not -path "*/external/*" -type f 2>/dev/null | sort)

  if [ "$found_local" -eq 0 ]; then
    echo "  $(t skills.none_local)"
  else
    # Séparer les skills stacks des skills génériques
    local generic_skills=()
    local stack_skills=()
    for skill in "${all_local_skills[@]}"; do
      skill="${skill%.md}"
      if [[ "$skill" == *"/stacks/"* ]]; then
        stack_skills+=("$skill")
      else
        generic_skills+=("$skill")
      fi
    done

    # Afficher les skills génériques groupés par domaine de premier niveau
    local current_domain=""
    for skill in "${generic_skills[@]}"; do
      local domain
      domain=$(echo "$skill" | cut -d'/' -f1)
      if [ "$domain" != "$current_domain" ]; then
        [ -n "$current_domain" ] && echo ""
        echo -e "  ${BOLD}${domain}/${RESET}"
        current_domain="$domain"
      fi
      echo "    ${skill}"
    done

    # Afficher les skills spécifiques aux stacks dans une section dédiée
    if [ "${#stack_skills[@]}" -gt 0 ]; then
      echo ""
      echo -e "  ${BOLD}developer/stacks/${RESET}  ${YELLOW}(injection dynamique à oc deploy)${RESET}"

      local current_category=""
      for skill in "${stack_skills[@]}"; do
        # Extraire la catégorie depuis le nom du fichier
        local filename
        filename=$(basename "$skill")
        local category=""
        case "$filename" in
          dev-standards-typescript*|dev-standards-python*)  category="langages" ;;
          dev-standards-react-native*|dev-standards-flutter*|dev-standards-swift*|dev-standards-kotlin*) category="mobile" ;;
          dev-standards-vue*|dev-standards-react*|dev-standards-next*|dev-standards-nuxt*|dev-standards-angular*) category="frontend" ;;
          dev-standards-nestjs*|dev-standards-express*|dev-standards-django*|dev-standards-fastapi*|dev-standards-laravel*|dev-standards-rails*|dev-standards-springboot*) category="backend" ;;
          dev-standards-prisma*|dev-standards-typeorm*|dev-standards-sqlalchemy*|dev-standards-mongodb*) category="orm / bdd" ;;
          dev-standards-openapi*) category="api-spec" ;;
          dev-standards-vitest*|dev-standards-jest*|dev-standards-playwright*|dev-standards-cypress*) category="test" ;;
          dev-standards-pandas*|dev-standards-dbt*|dev-standards-airflow*|dev-standards-pyspark*) category="data / ml" ;;
          dev-standards-docker*|dev-standards-github-actions*|dev-standards-gitlab-ci*) category="devops / ci-cd" ;;
          dev-standards-terraform*|dev-standards-kubernetes*|dev-standards-helm*|dev-standards-argocd*) category="platform / infra" ;;
          *) category="autres" ;;
        esac

        if [ "$category" != "$current_category" ]; then
          echo -e "    ${DIM}── ${category}${RESET}"
          current_category="$category"
        fi
        echo "      ${skill}"
      done
    fi
  fi

  echo ""
  echo -e "${BOLD}$(t skills.external)${RESET}"
  local found_ext=0
  for f in "$EXTERNAL_SKILLS_DIR"/*.md; do
    [ -f "$f" ] || continue
    local name
    name=$(basename "$f" .md)
    local repo=""
    if [ -f "$SOURCES_FILE" ] && command -v jq &>/dev/null; then
      repo=$(jq -r --arg n "$name" '.[$n].repo // ""' "$SOURCES_FILE")
    fi
    if [ -n "$repo" ]; then
      echo "  external/$name  ← $repo"
    else
      echo "  external/$name"
    fi
    found_ext=1
  done
  [ "$found_ext" -eq 0 ] && echo "  $(t skills.none_external)"

  echo ""
}

# ── SYNC ─────────────────────────────────────────────────────────────────────

##
# Re-télécharge tous les skills externes enregistrés dans .sources.json.
# Utile après un git clone (skills/external/ est gitignorés).
##
cmd_sync() {
  _require_npx
  _init_external

  if ! command -v jq &>/dev/null; then
    log_error "jq est requis pour lire les sources enregistrées."
    exit 1
  fi

  if [ ! -f "$SOURCES_FILE" ] || [ "$(cat "$SOURCES_FILE")" = '{}' ]; then
    log_info "$(t skills.sync.none)"
    return
  fi

  log_title "$(t skills.sync.title)"
  echo ""

  local count=0
  while IFS= read -r skill_name; do
    [ -z "$skill_name" ] && continue
    local repo
    repo=$(jq -r --arg n "$skill_name" '.[$n].repo' "$SOURCES_FILE")
    log_info "Re-téléchargement : $skill_name depuis $repo..."
    # Forcer le re-téléchargement en supprimant le cache universel
    rm -f "$UNIVERSAL_SKILLS_DIR/${skill_name}.md"
    cmd_add "$repo" "$skill_name" "--force"
    count=$((count + 1))
  done < <(jq -r 'keys[]' "$SOURCES_FILE")

  echo ""
  log_success "$count $(t skills.sync.done)"
}

# ── REMOVE ───────────────────────────────────────────────────────────────────

##
# Supprime un skill externe du hub (ne supprime pas le cache ~/.agents/skills/).
# @param {string} $1 — Nom du skill à supprimer (sans préfixe external/)
##
cmd_remove() {
  local skill_name="${1:-}"
  if [ -z "$skill_name" ]; then
    log_error "$(t skills.remove.usage)"
    log_info  "Exemple : oc skills remove pdf"
    log_info  "Utilisez 'oc skills list' pour voir les skills disponibles."
    exit 1
  fi

  # Accepter "external/pdf" ou "pdf"
  skill_name="${skill_name#external/}"

  local dest="$EXTERNAL_SKILLS_DIR/${skill_name}.md"
  if [ ! -f "$dest" ]; then
    log_error "$(t skills.remove.not_found)"
    log_info  "Utilisez 'oc skills list' pour voir les skills disponibles."
    exit 1
  fi

  read -rp "$(t skills.remove.confirm) '${skill_name}' ? (y/N) : " confirm
  [[ "$confirm" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)."; exit 0; }

  rm "$dest"
  [ -f "$SOURCES_FILE" ] && _remove_source "$skill_name"

  log_success "$(t skills.remove.done)"
  log_info    "$(t skills.remove.agent_hint)"
}

# ── USED-BY ──────────────────────────────────────────────────────────────────

##
# Liste les agents canoniques qui référencent un skill donné.
# Fonctionne pour les skills locaux et externes (avec ou sans préfixe external/).
# @param {string} $1 — Nom du skill (ex: developer/dev-standards-frontend ou external/pdf)
##
cmd_used_by() {
  local skill="${1:-}"
  if [ -z "$skill" ]; then
    log_error "$(t skills.used_by.usage)"
    log_info  "Exemples :"
    log_info  "  oc skills used-by developer/dev-standards-frontend"
    log_info  "  oc skills used-by external/pdf"
    exit 1
  fi

  log_title "$(t skills.used_by.title) $skill"
  echo ""

  local found=0
  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue
    local skills_line
    skills_line=$(grep '^skills:' "$agent_file" | head -1 | tr -d '[]"')
    # Normaliser les espaces autour des virgules pour une recherche fiable
    if echo "$skills_line" | tr ',' '\n' | sed 's/^ *//' | sed 's/ *$//' | grep -qxF "$skill"; then
      local agent_id family
      agent_id=$(grep '^id:' "$agent_file" | head -1 | sed 's/^id:[[:space:]]*//')
      family=$(basename "$(dirname "$agent_file")")
      echo -e "  ${GREEN}✔${RESET}  $agent_id  (agents/${family}/${agent_id}.md)"
      found=1
    fi
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  if [ "$found" -eq 0 ]; then
    echo "  $(t skills.used_by.none)"
  fi
  echo ""
}

# ── UPDATE ────────────────────────────────────────────────────────────────────

##
# Télécharge la version à jour d'un skill externe depuis sa source ctx7,
# affiche un diff et demande confirmation avant d'écraser.
# Sans argument → met à jour tous les skills externes enregistrés.
# @param {string} $1 — (optionnel) Nom du skill à mettre à jour
##
cmd_update() {
  _require_npx
  _init_external

  if ! command -v jq &>/dev/null; then
    log_error "jq est requis pour lire les sources enregistrées."
    exit 1
  fi

  if [ ! -f "$SOURCES_FILE" ] || [ "$(cat "$SOURCES_FILE")" = '{}' ]; then
    log_info "$(t skills.update.no_skills)"
    return
  fi

  local target_skill="${1:-}"
  local skills_to_update=()

  if [ -n "$target_skill" ]; then
    # Accepter "external/pdf" ou "pdf"
    target_skill="${target_skill#external/}"
    if ! jq -e --arg n "$target_skill" 'has($n)' "$SOURCES_FILE" &>/dev/null; then
      log_error "Skill externe '$target_skill' non trouvé dans .sources.json"
      log_info  "Utilisez 'oc skills list' pour voir les skills enregistrés."
      exit 1
    fi
    skills_to_update=("$target_skill")
  else
    while IFS= read -r s; do
      [ -n "$s" ] && skills_to_update+=("$s")
    done < <(jq -r 'keys[]' "$SOURCES_FILE")
  fi

  log_title "$(t skills.update.title)"
  echo ""

  local updated=0 skipped=0
  for skill_name in "${skills_to_update[@]}"; do
    local repo dest tmp_file
    repo=$(jq -r --arg n "$skill_name" '.[$n].repo' "$SOURCES_FILE")
    dest="$EXTERNAL_SKILLS_DIR/${skill_name}.md"

    log_info "Vérification de '$skill_name' depuis $repo..."

    # Télécharger dans un fichier temporaire pour comparer
    tmp_file=$(mktemp /tmp/oc-skill-update-XXXXXX.md)
    # Supprimer le cache universel pour forcer le re-téléchargement
    rm -f "$UNIVERSAL_SKILLS_DIR/${skill_name}.md"

    # Installer silencieusement dans ~/.agents/skills/
    if ! npx ctx7 skills install "$repo" "$skill_name" --universal &>/dev/null; then
      log_warn "Échec du téléchargement de '$skill_name'. Ignoré."
      rm -f "$tmp_file"
      skipped=$((skipped + 1))
      continue
    fi

    local cached="$UNIVERSAL_SKILLS_DIR/${skill_name}.md"
    if [ ! -f "$cached" ]; then
      cached=$(find "$UNIVERSAL_SKILLS_DIR" -iname "${skill_name}.*" -type f 2>/dev/null | head -1)
    fi

    if [ -z "$cached" ] || [ ! -f "$cached" ]; then
      log_warn "Fichier téléchargé introuvable pour '$skill_name'. Ignoré."
      rm -f "$tmp_file"
      skipped=$((skipped + 1))
      continue
    fi

    cp "$cached" "$tmp_file"

    # Comparer avec la version locale
    if [ -f "$dest" ] && diff -q "$dest" "$tmp_file" &>/dev/null; then
      log_success "'$skill_name' $(t skills.update.already_up_to_date)"
      rm -f "$tmp_file"
      skipped=$((skipped + 1))
      continue
    fi

    # Afficher le diff
    echo ""
    echo -e "${BOLD}Diff pour '$skill_name' :${RESET}"
    if [ -f "$dest" ]; then
      echo "  (- ancienne version  /  + nouvelle version)"
      echo ""
      diff "$dest" "$tmp_file" | head -60 || true
    else
      echo "  (nouveau skill — fichier local absent)"
    fi
    echo ""

    read -rp "$(t skills.update.apply) '$skill_name' ? (Y/n) : " confirm
    confirm="${confirm:-Y}"
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
      cp "$tmp_file" "$dest"
      local now
      now=$(date +%Y-%m-%d)
      _record_source "$skill_name" "$repo" "$now"
      log_success "'$skill_name' mis à jour."
      updated=$((updated + 1))

      # Lister les agents impactés
      local agents_impacted=()
      while IFS= read -r agent_file; do
        [ -f "$agent_file" ] || continue
        local skills_line
        skills_line=$(grep '^skills:' "$agent_file" | head -1 | tr -d '[]"')
        if echo "$skills_line" | tr ',' '\n' | sed 's/^ *//' | sed 's/ *$//' | grep -qxF "external/$skill_name"; then
          local aid; aid=$(grep '^id:' "$agent_file" | head -1 | sed 's/^id:[[:space:]]*//')
          agents_impacted+=("$aid")
        fi
      done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

      if [ ${#agents_impacted[@]} -gt 0 ]; then
        echo -e "  ${YELLOW}⚠${RESET}  Agents impactés : ${agents_impacted[*]}"
        echo -e "  ${YELLOW}⚠${RESET}  Relancez : ./oc.sh deploy all"
      fi
    else
      log_info "$(t skills.update.skipped) '$skill_name'."
      skipped=$((skipped + 1))
    fi

    rm -f "$tmp_file"
    echo ""
  done

  echo ""
  [ "$updated" -gt 0 ] && log_success "$updated $(t skills.update.updated)"
  [ "$skipped" -gt 0 ] && log_info    "$skipped $(t skills.update.unchanged)"
}

# ── VALIDATE ─────────────────────────────────────────────────────────────────

##
# Valide la cohérence de tous les skills (locaux + externes).
# Vérifie pour chaque fichier .md :
#   - présence des champs frontmatter requis (name, description)
#   - cohérence du nom dans le frontmatter vs le nom de fichier
#   - pour les externes : présence dans .sources.json
# Affiche un résumé et exit 1 si des erreurs sont trouvées.
# @param {string} [$1] — nom de skill optionnel (valide uniquement ce skill)
##
cmd_validate() {
  local filter_name="${1:-}"
  local count_ok=0 count_err=0 count_warn=0

  log_title "Validation des skills"
  echo ""

  # ── Construire la liste des skills locaux ─────────────────────────────────
  local skill_files=()
  while IFS= read -r f; do
    skill_files+=("$f")
  done < <(find "$HUB_DIR/skills" -name "*.md" \
              -not -path "*/external/*" \
              -type f 2>/dev/null | sort)

  # ── Ajouter les skills externes ───────────────────────────────────────────
  if [ -d "$EXTERNAL_SKILLS_DIR" ]; then
    while IFS= read -r f; do
      skill_files+=("$f")
    done < <(find "$EXTERNAL_SKILLS_DIR" -name "*.md" \
                -not -name ".sources.json" \
                -type f 2>/dev/null | sort)
  fi

  if [ "${#skill_files[@]}" -eq 0 ]; then
    log_warn "Aucun skill trouvé."
    exit 0
  fi

  # ── Charger les sources externes pour validation croisée ─────────────────
  local known_externals=""
  if [ -f "$SOURCES_FILE" ] && command -v jq &>/dev/null; then
    known_externals=$(jq -r 'keys[]' "$SOURCES_FILE" 2>/dev/null || true)
  fi

  for f in "${skill_files[@]}"; do
    [ -f "$f" ] || continue

    local skill_name_front skill_desc_front
    skill_name_front=$(grep '^name:'        "$f" 2>/dev/null | head -1 | sed 's/^name:[[:space:]]*//')
    skill_desc_front=$(grep '^description:' "$f" 2>/dev/null | head -1 | sed 's/^description:[[:space:]]*//')

    # Nom du skill depuis le chemin (sans extension, relatif à skills/)
    local rel_path file_basename
    rel_path="${f#"$HUB_DIR"/skills/}"
    file_basename=$(basename "$f" .md)

    # Filtrer si un nom spécifique est demandé
    if [ -n "$filter_name" ]; then
      local filter_base; filter_base=$(basename "$filter_name" .md)
      [ "$file_basename" = "$filter_base" ] || continue
    fi

    local issues=""
    local has_err=0 has_warn=0

    # ── Champs requis ────────────────────────────────────────────────────────
    if [ -z "$skill_name_front" ]; then
      issues="${issues}    champ manquant : name\n"
      has_warn=1
    fi
    if [ -z "$skill_desc_front" ]; then
      issues="${issues}    champ manquant : description\n"
      has_warn=1
    fi

    # ── Cohérence name frontmatter vs nom de fichier ──────────────────────
    if [ -n "$skill_name_front" ] && [ "$skill_name_front" != "$file_basename" ]; then
      issues="${issues}    nom incohérent : frontmatter='${skill_name_front}' vs fichier='${file_basename}'\n"
      has_warn=1
    fi

    # ── Skills externes : vérifier présence dans .sources.json ───────────
    if [[ "$rel_path" == external/* ]]; then
      local ext_base; ext_base="${rel_path#external/}"; ext_base="${ext_base%.md}"
      if [ -n "$known_externals" ] && ! printf '%s\n' "$known_externals" | grep -qx "$ext_base"; then
        issues="${issues}    skill externe sans source enregistrée dans .sources.json\n"
        has_warn=1
      fi
    fi

    # ── Affichage résultat ───────────────────────────────────────────────
    if [ $has_err -eq 1 ]; then
      echo -e "  ${RED}✘${RESET}  ${BOLD}${rel_path}${RESET}"
      printf '%b' "$issues"
      count_err=$((count_err + 1))
    elif [ $has_warn -eq 1 ]; then
      echo -e "  ${YELLOW}⚠${RESET}  ${BOLD}${rel_path}${RESET}"
      printf '%b' "$issues"
      count_warn=$((count_warn + 1))
    else
      echo -e "  ${GREEN}✔${RESET}  ${rel_path}"
      count_ok=$((count_ok + 1))
    fi
  done

  # ── Résumé ─────────────────────────────────────────────────────────────────
  echo ""
  local summary
  summary="${BOLD}Résumé :${RESET}  ${GREEN}${count_ok} OK${RESET}"
  [ $count_err  -gt 0 ] && summary="${summary}  ${RED}${count_err} erreur(s)${RESET}"
  [ $count_warn -gt 0 ] && summary="${summary}  ${YELLOW}${count_warn} avertissement(s)${RESET}"
  echo -e "$summary"
  echo ""

  [ $count_err -gt 0 ] && exit 1
  return 0
}

# ── DISPATCH ─────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-}"
shift 2>/dev/null || true

case "$SUBCOMMAND" in
  search)  cmd_search "$@" ;;
  info)    cmd_info "$@" ;;
  add)     cmd_add "$@" ;;
  list)    cmd_list ;;
  sync)    cmd_sync ;;
  update)  cmd_update "$@" ;;
  used-by) cmd_used_by "$@" ;;
  remove)   cmd_remove "$@" ;;
  validate) cmd_validate "$@" ;;
  *)
    echo -e "${BOLD}$(t skills.title)${RESET}"
    echo ""
    echo "  $(t help.skills_search)"
    echo "  $(t help.skills_info)"
    echo "  $(t help.skills_add)"
    echo "  $(t help.skills_list)"
    echo "  $(t help.skills_update)"
    echo "  $(t help.skills_used_by)"
    echo "  $(t help.skills_sync)"
    echo "  $(t help.skills_remove)"
    echo "  $(t help.skills_validate)"
    echo ""
    echo -e "${BOLD}$(t skills.examples)${RESET}"
    echo "  ./oc.sh skills update pdf"
    echo "  ./oc.sh skills update"
    echo "  ./oc.sh skills used-by external/pdf"
    echo "  ./oc.sh skills used-by developer/dev-standards-frontend"
    echo ""
    ;;
esac
