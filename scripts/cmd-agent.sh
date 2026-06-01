#!/bin/bash
# Créateur et modificateur d'agents canoniques.
# Usage : ./oc.sh agent <commande> [args]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
resolve_oc_lang
source "$LIB_DIR/tui-picker.sh"
source "$LIB_DIR/agent-picker.sh"

# EXTERNAL_SKILLS_DIR est défini dans common.sh

# ── HELPERS ──────────────────────────────────────────────────────────────────

##
# Résout un agent-id vers son chemin de fichier dans la structure en sous-dossiers.
# Cherche récursivement dans agents/ le fichier dont le frontmatter id: correspond.
# Imprime le chemin absolu sur stdout, ou rien si non trouvé.
# @param {string} $1 — identifiant de l'agent (ex: documentarian)
##
_find_agent_file() {
  local agent_id="$1" result=""
  while IFS= read -r f; do
    local fid; fid=$(grep '^id:' "$f" 2>/dev/null | head -1 | sed 's/^id:[[:space:]]*//')
    if [ "$fid" = "$agent_id" ]; then
      result="$f"
      break
    fi
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
  echo "$result"
}

##
# Construit la liste de tous les skills disponibles (locaux + externes).
# Imprime un skill par ligne sous la forme "chemin/relatif" (sans .md).
##
_list_all_skills() {
  # Skills locaux
  find "$HUB_DIR/skills" -name "*.md" \
      -not -path "*/external/*" \
      -type f 2>/dev/null | sort \
    | while IFS= read -r f; do
        echo "${f#"$HUB_DIR"/skills/}" | sed 's/\.md$//'
      done

  # Skills externes
  if [ -d "$EXTERNAL_SKILLS_DIR" ]; then
    find "$EXTERNAL_SKILLS_DIR" -name "*.md" \
        -not -name ".sources.json" \
        -type f 2>/dev/null | sort \
      | while IFS= read -r f; do
          name=$(basename "$f" .md)
          echo "external/$name"
        done
  fi
}

##
# Rendu du sélecteur de skills (compatible bash 3.2).
# Utilise les variables partagées de _pick_from_list :
#   _pick_items, _pick_checked, _pick_cursor, _pick_total
##
_render_skills_page() {
  # Variables partagées par dynamic scoping depuis _pick_from_list (tui-picker.sh) :
  # _pick_cursor, _pick_total, _pick_items, _pick_checked sont assignées par le caller.
  # shellcheck disable=SC2154
  local page_size=10
  # shellcheck disable=SC2154
  local win_start=$(( _pick_cursor - page_size / 2 ))
  [ "$win_start" -lt 0 ] && win_start=0
  local win_end=$(( win_start + page_size ))
  [ "$win_end" -gt "$_pick_total" ] && win_end=$_pick_total
  # Réajuster win_start si on est proche de la fin
  win_start=$(( win_end - page_size ))
  [ "$win_start" -lt 0 ] && win_start=0

  printf "\033[2J\033[H"  # clear screen

  # ── En-tête ──────────────────────────────────────────────────────────────
  echo -e "${BOLD}Sélection des skills${RESET}  ($((_pick_cursor+1))/$_pick_total)"
  echo -e "  \033[0;34m↑↓\033[0m naviguer   \033[0;34mespace\033[0m cocher/décocher   \033[0;34mentrée\033[0m valider   \033[0;34mESC\033[0m annuler   \033[0;34m0\033[0m tout vider"
  echo ""

  # ── Liste (fenêtre glissante) ─────────────────────────────────────────────
  local i=$win_start
  while [ "$i" -lt "$win_end" ]; do
    local skill="${_pick_items[$i]}"
    local num=$((i + 1))

    local check_icon="   "
    local check_color=""
    local check_reset=""
    if [ "${_pick_checked[$i]}" = "1" ]; then
      check_icon="[x]"
      check_color="$GREEN"
      check_reset="$RESET"
    fi

    if [ "$i" -eq "$_pick_cursor" ]; then
      printf "  \033[1m> ${check_color}%-3s${check_reset}\033[1m %3d. %-50s\033[0m\n" \
        "$check_icon" "$num" "$skill"
    else
      printf "    ${check_color}%-3s${check_reset} %3d. %-50s\n" \
        "$check_icon" "$num" "$skill"
    fi
    i=$((i + 1))
  done

  # ── Séparateur ────────────────────────────────────────────────────────────
  echo ""
  printf "  \033[0;34m%s\033[0m\n" "────────────────────────────────────────────────────────────"

  # ── Panneau description du skill sous le curseur ──────────────────────────
  local cur_skill="${_pick_items[$_pick_cursor]}"
  local cur_desc=""
  local skill_file="$HUB_DIR/skills/${cur_skill}.md"
  [ -f "$skill_file" ] && cur_desc=$(grep '^description:' "$skill_file" | head -1 | sed 's/^description:[[:space:]]*//')

  echo ""
  printf "  \033[1m%s\033[0m\n" "$cur_skill"
  if [ -n "$cur_desc" ]; then
    printf "  %s\n" "$cur_desc"
  else
    printf "  \033[2m(pas de description)\033[0m\n"
  fi

  # ── Pied ──────────────────────────────────────────────────────────────────
  echo ""
  local count=0
  local v
  for v in "${_pick_checked[@]}"; do [ "$v" = "1" ] && count=$((count+1)); done
  echo -e "  ${BOLD}$count skill(s) sélectionné(s)${RESET}"
  echo ""
}

##
# Menu interactif de sélection de skills avec navigation flèches + espace.
# Compatible bash 3.2 (macOS). Résultat dans $PICKED_SKILLS (CSV).
# Wrapper autour de _pick_from_list.
# @param {string} $1 — sélection courante (CSV de noms de skills)
##
_pick_skills() {
  local current_csv="${1:-}"

  # Charger tous les skills dans _pick_items
  _pick_items=()
  while IFS= read -r s; do
    _pick_items+=("$s")
  done < <(_list_all_skills)

  if [ ${#_pick_items[@]} -eq 0 ]; then
    log_warn "Aucun skill disponible."
    PICKED_SKILLS="$current_csv"
    return
  fi

  _pick_total=${#_pick_items[@]}

  # Nettoyer current_csv : retirer guillemets et espaces superflus
  local clean_csv
  clean_csv=$(echo "$current_csv" | tr -d '"' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | grep -v '^$' | tr '\n' ',' | sed 's/,$//')

  # Initialiser le tableau de sélection
  _pick_checked=()
  local i=0
  while [ "$i" -lt "$_pick_total" ]; do
    if echo ",$clean_csv," | grep -qF ",${_pick_items[$i]},"; then
      _pick_checked+=("1")
    else
      _pick_checked+=("0")
    fi
    i=$((i + 1))
  done

  _pick_render_fn="_render_skills_page"
  _pick_allow_zero=1
  _PICK_RESULT=""

  _pick_from_list "$current_csv" "$current_csv"

  if [ "$_PICK_RESULT" = "$current_csv" ] && [ -n "$current_csv" ]; then
    log_warn "Sélection annulée (ESC) — skills inchangés."
  fi
  PICKED_SKILLS="$_PICK_RESULT"
}

##
# Sélection interactive de cibles (targets) avec navigation flèches + espace.
# Compatible bash 3.2 (macOS). Résultat dans $PICKED_TARGETS (CSV).
# Wrapper autour de _pick_from_list.
# @param {string} $1 — sélection courante (CSV de cibles)
##
_pick_targets() {
  local current_csv="${1:-opencode}"

  # Initialiser _pick_items avec les cibles disponibles
  _pick_items=("opencode")
  _pick_total=${#_pick_items[@]}

  # Nettoyer le CSV courant
  local clean_csv
  clean_csv=$(printf '%s' "$current_csv" | tr -d '"' | tr ',' '\n' \
    | sed 's/^ *//;s/ *$//' | grep -v '^$' | tr '\n' ',' | sed 's/,$//')

  # Initialiser le tableau de sélection
  _pick_checked=()
  local i=0
  while [ "$i" -lt "$_pick_total" ]; do
    if echo ",$clean_csv," | grep -qF ",${_pick_items[$i]},"; then
      _pick_checked+=("1")
    else
      _pick_checked+=("0")
    fi
    i=$((i + 1))
  done

  _pick_render_fn="_render_targets_page"
  _pick_allow_zero=0
  _PICK_RESULT=""

  _pick_from_list "$current_csv" "$current_csv"

  # Aucune cible cochée → garder la sélection courante
  if [ -z "$_PICK_RESULT" ]; then
    PICKED_TARGETS="$current_csv"
  else
    PICKED_TARGETS="$_PICK_RESULT"
  fi
}

##
# Convertit un CSV de skills/targets en format YAML inline ["a","b","c"].
# @param {string} $1 — CSV à convertir
# Retourne la valeur sur stdout (usage interne uniquement — pas de TUI).
##
_csv_to_yaml_list() {
  printf '%s\n' "$1" \
    | tr ',' '\n' \
    | sed 's/^ *//;s/ *$//' \
    | grep -v '^$' \
    | sed 's/.*/"&"/' \
    | tr '\n' ',' \
    | sed 's/,$//' \
    | sed 's/^/[/;s/$/]/'
}

##
# Génère le corps Markdown d'un agent via opencode run.
# Si opencode est absent ou si l'utilisateur refuse, corps TODO par défaut.
# Résultat dans $GENERATED_BODY.
# @param {string} $1 — agent_id
# @param {string} $2 — label
# @param {string} $3 — description
# @param {string} $4 — skills_csv
##
_generate_body() {
  local agent_id="$1" label="$2" description="$3" skills_csv="$4"

  local default_body
  default_body=$(printf '# %s\n\n%s\n\n## Ce que tu fais\n- TODO : décrire les responsabilités de cet agent\n\n## Workflow\n1. TODO : décrire le workflow\n\n## Ce que tu ne fais PAS\n- TODO : définir les limites\n' \
    "$label" "$description")

  # opencode non disponible → corps par défaut sans question
  if ! command -v opencode &>/dev/null; then
    GENERATED_BODY="$default_body"
    return
  fi

  echo ""
  read -rp "Générer le corps de l'agent avec opencode ? (Y/n) : " gen_choice </dev/tty
  gen_choice="${gen_choice:-Y}"
  if ! [[ "$gen_choice" =~ ^[Yy]$ ]]; then
    GENERATED_BODY="$default_body"
    return
  fi

  local skills_hint=""
  [ -n "$skills_csv" ] && skills_hint="Skills injectés automatiquement : ${skills_csv}."

  local prompt
  prompt="Tu crées le corps Markdown d'un agent IA nommé \"${label}\" (id: ${agent_id}).
Description : ${description}.
${skills_hint}
Génère uniquement le corps (sans frontmatter YAML). Structure attendue :

# ${label}

${description}

## Ce que tu fais
<liste des responsabilités précises, cohérentes avec la description et les skills>

## Workflow
<étapes numérotées, opérationnelles>

## Ce que tu ne fais PAS
<limites claires, 3-5 points>

Sois concis et directement opérationnel. Pas d'introduction, pas de conclusion, pas de balises markdown superflues."

  log_info "Génération en cours via opencode..."
  local generated
  # Sur macOS, 'timeout' n'est pas natif — utiliser 'gtimeout' si disponible, sinon sans timeout
  if command -v timeout &>/dev/null; then
    generated=$(timeout 60 opencode run "$prompt" 2>/dev/null) || true
  elif command -v gtimeout &>/dev/null; then
    generated=$(gtimeout 60 opencode run "$prompt" 2>/dev/null) || true
  else
    generated=$(opencode run "$prompt" 2>/dev/null) || true
  fi

  if [ -z "$generated" ]; then
    log_warn "opencode n'a pas retourné de contenu — corps par défaut utilisé."
    GENERATED_BODY="$default_body"
  else
    GENERATED_BODY="$generated"
    log_success "Corps généré."
  fi
}

# ── CREATE ───────────────────────────────────────────────────────────────────

##
# Crée un nouvel agent canonique de façon interactive.
# Workflow : id → label → description → cibles → skills → corps (IA optionnel)
#            → prévisualisation → confirmation → écriture
##
cmd_create() {
  log_title "Créer un nouvel agent"
  echo ""

  # ── 1. Identifiant ────────────────────────────────────────────────────────
  read -rp "Identifiant (ex: reviewer) : " agent_id
  agent_id=$(printf '%s' "$agent_id" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9_-]/-/g')
  if [ -z "$agent_id" ]; then
    log_error "$(t agent.id_required)"
    exit 1
  fi

  local file="$CANONICAL_AGENTS_DIR/${agent_id}.md"
  if [ -f "$file" ] || [ -n "$(_find_agent_file "$agent_id")" ]; then
    log_error "'${agent_id}' $(t agent.already_exists)."
    exit 1
  fi

  # ── 2. Label ──────────────────────────────────────────────────────────────
  read -rp "Label (nom court, ex: CodeReviewer) [${agent_id}] : " label
  label="${label:-$agent_id}"

  # ── 3. Description ────────────────────────────────────────────────────────
  read -rp "Description courte : " description
  description="${description:-Assistant $label}"

  # ── 4. Cibles ─────────────────────────────────────────────────────────────
  PICKED_TARGETS=""
  _pick_targets "opencode"
  local targets_csv="$PICKED_TARGETS"
  [ "$targets_csv" = "all" ] && targets_csv="opencode"

  # ── 5. Skills ─────────────────────────────────────────────────────────────
  PICKED_SKILLS=""
  _pick_skills ""
  local skills_csv="$PICKED_SKILLS"

  # ── 6. Corps (génération IA optionnelle) ──────────────────────────────────
  GENERATED_BODY=""
  _generate_body "$agent_id" "$label" "$description" "$skills_csv"
  local body="$GENERATED_BODY"

  # ── 7. Construire le contenu final ────────────────────────────────────────
  local skills_yaml targets_yaml
  skills_yaml=$(_csv_to_yaml_list "$skills_csv")
  targets_yaml=$(_csv_to_yaml_list "$targets_csv")

  local file_content
  file_content=$(
    printf '%s\n' "---"
    printf 'id: %s\n'          "$agent_id"
    printf 'label: %s\n'       "$label"
    printf 'description: %s\n' "$description"
    printf 'targets: %s\n'     "$targets_yaml"
    printf 'skills: %s\n'      "$skills_yaml"
    printf '%s\n' "---"
    printf '\n'
    printf '%s\n' "$body"
  )

  # ── 8. Prévisualisation + confirmation ────────────────────────────────────
  echo ""
  local sep="────────────────────────────────────────────────────────────"
  echo -e "  \033[0;34m${sep}\033[0m"
  printf "  \033[1m  Aperçu — agents/<famille>/%s.md\033[0m\n" "$agent_id"
  echo -e "  \033[0;34m${sep}\033[0m"
  echo ""
  # Indenter chaque ligne de l'aperçu pour lisibilité
  printf '%s\n' "$file_content" | while IFS= read -r line; do
    printf '  %s\n' "$line"
  done
  echo ""
  echo -e "  \033[0;34m${sep}\033[0m"
  echo ""

  read -rp "Créer l'agent '${agent_id}' ? (Y/n) : " confirm
  confirm="${confirm:-Y}"
  if ! [[ "$confirm" =~ ^[Yy]$ ]]; then
    log_info "Annulé."
    exit 0
  fi

  # ── 9. Écriture ───────────────────────────────────────────────────────────
  # Demander la famille pour placer le fichier dans le bon sous-dossier
  echo ""
  echo "Famille de l'agent (sous-dossier dans agents/) :"
  echo "  auditor / design / developer / quality / planning / documentation"
  read -rp "Famille [documentation] : " agent_family
  agent_family="${agent_family:-documentation}"
  # Valider et normaliser
  case "$agent_family" in
    auditor|design|developer|quality|planning|documentation) ;;
    *) log_warn "Famille inconnue '$agent_family' — fichier créé dans agents/$agent_family/ (créez le dossier si nécessaire)" ;;
  esac
  mkdir -p "$CANONICAL_AGENTS_DIR/$agent_family"
  file="$CANONICAL_AGENTS_DIR/$agent_family/${agent_id}.md"
  printf '%s\n' "$file_content" > "$file"

  echo ""
  log_success "Agent '${agent_id}' créé → agents/${agent_family}/${agent_id}.md"
  log_info    "Personnalisez le corps dans agents/${agent_family}/${agent_id}.md si nécessaire."
  log_info    "Puis déployez : ./oc.sh deploy all"
}

# ── LIST ─────────────────────────────────────────────────────────────────────

##
# Liste les agents canoniques disponibles dans agents/.
##
cmd_list() {
  log_title "Agents canoniques"
  echo ""

  local found=0
  while IFS= read -r f; do
    [ -f "$f" ] || continue
    local id label description targets skills_count
    id=$(grep '^id:' "$f" | head -1 | sed 's/^id:[[:space:]]*//')
    label=$(grep '^label:' "$f" | head -1 | sed 's/^label:[[:space:]]*//')
    description=$(grep '^description:' "$f" | head -1 | sed 's/^description:[[:space:]]*//')
    skills_count=$(grep '^skills:' "$f" | head -1 \
      | sed 's/^skills:[[:space:]]*//' | tr -d '[]"' \
      | tr ',' '\n' | sed 's/^ *//;s/ *$//' | grep -v '^$' | wc -l | tr -d ' ')
    targets=$(grep '^targets:' "$f" | head -1 | sed 's/^targets:[[:space:]]*//')
    local family; family=$(basename "$(dirname "$f")")
    echo -e "  ${BOLD}${id}${RESET}  (${label})  [${family}]"
    echo "    → $description"
    echo "    cibles : $targets"
    echo "    skills : $skills_count skill(s) configuré(s)"
    echo ""
    found=1
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  [ "$found" -eq 0 ] && echo "  $(t agent.no_agents)"
}

# ── INFO ─────────────────────────────────────────────────────────────────────

##
# Affiche le détail d'un agent (frontmatter + liste complète des skills).
# @param {string} $1 — Identifiant ou nom de fichier de l'agent
##
cmd_info() {
  local name="${1:-}"
  if [ -z "$name" ]; then
    log_error "$(t agent.usage.info)"
    exit 1
  fi

  local file; file=$(_find_agent_file "$name")
  [ -n "$file" ] || { log_error "$(t agent.not_found) : '$name'"; exit 1; }

  log_title "Agent : $name"
  echo ""
  sed -n '/^---$/,/^---$/p' "$file" | grep -v '^---$'
  echo ""
  echo -e "${BOLD}$(t agent.skills_assigned)${RESET}"
  grep '^skills:' "$file" | head -1 \
    | sed 's/^skills:[[:space:]]*//' \
    | tr -d '[]' | tr ',' '\n' \
    | sed 's/^ *//' | sed 's/ *$//' | grep -v '^$' \
    | while IFS= read -r skill; do
        cleaned=$(echo "$skill" | tr -d '"')
        local skill_file="$HUB_DIR/skills/${cleaned}.md"
        if [ -f "$skill_file" ]; then
          echo -e "  ${GREEN}✔${RESET}  $cleaned"
        else
          echo -e "  ${YELLOW}⚠${RESET}  $cleaned  (fichier absent)"
        fi
      done
}

# ── EDIT ─────────────────────────────────────────────────────────────────────

##
# Modifie les métadonnées et les skills d'un agent existant de façon interactive.
# @param {string} $1 — Identifiant de l'agent à modifier
##
cmd_edit() {
  local name="${1:-}"
  if [ -z "$name" ]; then
    log_error "$(t agent.usage.edit)"
    log_info  "$(t agent.usage.select | sed 's/select/list/')"
    exit 1
  fi

  local file; file=$(_find_agent_file "$name")
  [ -n "$file" ] || { log_error "$(t agent.not_found) : '$name'"; exit 1; }

  log_title "Modifier l'agent : $name"

  # Lire les valeurs courantes
  local cur_label cur_desc cur_targets cur_skills
  cur_label=$(grep '^label:' "$file" | head -1 | sed 's/^label:[[:space:]]*//')
  cur_desc=$(grep '^description:' "$file" | head -1 | sed 's/^description:[[:space:]]*//')
  cur_targets=$(grep '^targets:' "$file" | head -1 | sed 's/^targets:[[:space:]]*//' | tr -d '[]"')
  cur_skills=$(grep '^skills:' "$file" | head -1 | sed 's/^skills:[[:space:]]*//' | tr -d '[]"')

  echo ""
  echo -e "  label       : ${BOLD}${cur_label}${RESET}"
  echo   "  description : $cur_desc"
  echo   "  targets     : $cur_targets"
  echo   "  skills      : $cur_skills"
  echo ""

  # Label
  read -rp "Nouveau label [${cur_label}] : " new_label
  new_label="${new_label:-$cur_label}"

  # Description
  read -rp "Nouvelle description [${cur_desc}] : " new_desc
  new_desc="${new_desc:-$cur_desc}"

  # Targets
  echo ""
  read -rp "Modifier les cibles ? (y/N) : " edit_targets </dev/tty
  local new_targets="$cur_targets"
  if [[ "$edit_targets" =~ ^[Yy]$ ]]; then
    PICKED_TARGETS=""
    _pick_targets "$cur_targets"
    new_targets="$PICKED_TARGETS"
    [ "$new_targets" = "all" ] && new_targets="opencode"
  fi

  # Skills (toujours proposé)
  PICKED_SKILLS=""
  _pick_skills "$cur_skills"
  local new_skills="$PICKED_SKILLS"

  # Valider avant écriture
  echo ""
  echo -e "${BOLD}Récapitulatif des modifications :${RESET}"
  echo "  label       : $new_label"
  echo "  description : $new_desc"
  echo "  targets     : $new_targets"
  echo "  skills      : ${new_skills:-aucun}"
  echo ""
  read -rp "Appliquer ? (Y/n) : " confirm </dev/tty
  confirm="${confirm:-Y}"
  [[ "$confirm" =~ ^[Yy]$ ]] || { log_info "Annulé."; exit 0; }

  # Réécriture du frontmatter
  local skills_yaml targets_yaml
  skills_yaml=$(_csv_to_yaml_list "$new_skills")
  targets_yaml=$(_csv_to_yaml_list "$new_targets")

  local body
  body=$(awk 'BEGIN{f=0;d=0} /^---$/{if(!f){f=1;next}else if(!d){d=1;next}} d{print}' "$file")

  {
    printf '%s\n' "---"
    printf 'id: %s\n'          "$name"
    printf 'label: %s\n'       "$new_label"
    printf 'description: %s\n' "$new_desc"
    printf 'targets: %s\n'     "$targets_yaml"
    printf 'skills: %s\n'      "$skills_yaml"
    printf '%s\n' "---"
    printf '\n'
    printf '%s\n' "$body"
  } > "$file"

  local family; family=$(basename "$(dirname "$file")")
  echo ""
  log_success "Agent '$name' mis à jour → agents/${family}/${name}.md"
  log_info    "Relancez le déploiement pour appliquer : ./oc.sh deploy all"
}

# ── KEYTEST ──────────────────────────────────────────────────────────────────

##
# Diagnostic TTY : affiche les octets reçus pour chaque touche pressée.
# Permet de vérifier ce que le terminal envoie réellement pour les flèches,
# espace, entrée, ESC, etc.
# Quitter avec q ou Ctrl-C.
##
cmd_keytest() {
  echo -e "${BOLD}Diagnostic clavier (oc agent keytest)${RESET}"
  echo "Appuyez sur des touches pour voir les octets reçus."
  echo "Quittez avec  q  ou  Ctrl-C."
  echo ""

  local old_stty
  old_stty=$(stty -g </dev/tty 2>/dev/null)
  # Restauration garantie même en cas d'interruption
  trap 'stty "$old_stty" </dev/tty 2>/dev/null; echo ""; exit 0' INT TERM EXIT

  stty -echo -icanon min 1 time 0 </dev/tty 2>/dev/null

  while true; do
    local byte1 byte2 byte3
    IFS= read -rsn1 byte1 </dev/tty

    # Lire bytes supplémentaires si séquence ESC
    local extra=""
    if [ "$byte1" = $'\x1b' ]; then
      IFS= read -rsn1 -t 1 byte2 </dev/tty || byte2=""
      extra="$byte2"
      if [ "$byte2" = "[" ] || [ "$byte2" = "O" ]; then
        IFS= read -rsn1 -t 1 byte3 </dev/tty || byte3=""
        extra="${byte2}${byte3}"
      fi
    fi

    local full_seq="${byte1}${extra}"

    # Affichage hex + description lisible
    local hex
    hex=$(printf '%s' "$full_seq" | xxd -p 2>/dev/null || printf '%s' "$full_seq" | od -An -tx1 | tr -d ' \n')

    # Description humaine
    local desc=""
    case "$full_seq" in
      $'\x1b[A') desc="flèche HAUT" ;;
      $'\x1b[B') desc="flèche BAS" ;;
      $'\x1b[C') desc="flèche DROITE" ;;
      $'\x1b[D') desc="flèche GAUCHE" ;;
      $'\x1b')   desc="ESC seul" ;;
      " ")        desc="ESPACE" ;;
      $'\n')      desc="ENTRÉE (LF)" ;;
      $'\r')      desc="ENTRÉE (CR)" ;;
      "q"|"Q")
        desc="q — sortie"
        printf "  hex=%-20s  %s\n" "$hex" "$desc"
        break
        ;;
    esac

    printf "  hex=%-20s  repr=%-10s  %s\n" \
      "$hex" \
      "$(printf '%s' "$full_seq" | cat -v 2>/dev/null || echo '?')" \
      "$desc"
  done

  # Le trap gère la restauration
  trap - INT TERM EXIT
  stty "$old_stty" </dev/tty 2>/dev/null
  echo ""
  echo -e "${GREEN}Terminal restauré.${RESET}"
}

# ── SELECT ───────────────────────────────────────────────────────────────────

##
# Sélectionne les agents à déployer pour un projet donné.
# Lance le picker interactif et écrit le résultat dans projects.md.
# @param {string} $1 — PROJECT_ID
##
cmd_select() {
  local raw_id="${1:-}"
  if [ -z "$raw_id" ]; then
    log_error "$(t agent.usage.select)"
    exit 1
  fi

  local id
  id=$(normalize_project_id "$raw_id")
  if ! project_exists "$id"; then
    log_error "Projet $id introuvable → ./oc.sh list"
    exit 1
  fi

  local current
  current=$(get_project_agents "$id")
  log_title "Sélection des agents — $id"
  log_info "Sélection actuelle : ${current}"
  echo ""

  PICKED_AGENTS=""
  _pick_agents "$current"

  if [ "$PICKED_AGENTS" = "$current" ]; then
    log_info "Aucune modification."
    return
  fi

  _set_project_agents "$id" "$PICKED_AGENTS"

  echo ""
  if [ "$PICKED_AGENTS" = "all" ]; then
    log_success "Tous les agents seront déployés pour $id"
  elif [ -z "$PICKED_AGENTS" ]; then
    log_warn "Aucun agent sélectionné pour $id — le déploiement ne générera rien"
  else
    # Compter les agents sélectionnés
    local count
    count=$(echo "$PICKED_AGENTS" | tr ',' '\n' | wc -l | tr -d ' ')
    log_success "$count agent(s) sélectionné(s) pour $id"
  fi

  # Proposer un redéploiement immédiat
  echo ""
  read -rp "Redéployer maintenant ? [Y/n] : " redeploy </dev/tty
  redeploy="${redeploy:-Y}"
  if [[ "$redeploy" =~ ^[Yy]$ ]]; then
    exec "$HUB_DIR/oc.sh" deploy "$id"
  else
    log_info "Déployer plus tard : ./oc.sh deploy $id"
  fi
}

##
# Gère les overrides de mode (primary/subagent) pour un projet donné.
# Affiche le mode effectif de chaque agent et permet de les basculer.
# @param {string} $1 — PROJECT_ID
##
cmd_mode() {
  local raw_id="${1:-}"
  if [ -z "$raw_id" ]; then
    log_error "$(t agent.usage.mode)"
    exit 1
  fi

  local id
  id=$(normalize_project_id "$raw_id")
  if ! project_exists "$id"; then
    log_error "Projet $id introuvable → ./oc.sh list"
    exit 1
  fi

  log_title "Modes des agents — $id"
  echo -e "  Les modes en vert \033[0;32m(primary)\033[0m sont visibles via Tab dans OpenCode."
  echo -e "  Les modes en jaune \033[0;33m(subagent)\033[0m sont invocables uniquement par d'autres agents."
  echo ""

  # Charger la liste des agents et leurs modes effectifs
  _list_all_agents_grouped
  local total=${#_pick_items[@]}

  # Récupérer les overrides du projet
  local current_modes
  current_modes=$(get_project_modes "$id")

  # Construire les modes effectifs pour l'affichage
  # (override projet > frontmatter)
  local _effective_modes=()
  local _m=0
  while [ "$_m" -lt "$total" ]; do
    local _aid="${_pick_items[$_m]}"
    local _fmode="${_pick_modes[$_m]:-primary}"
    # Chercher override projet
    local _override=""
    if [ -n "$current_modes" ]; then
      _override=$(printf '%s\n' "$current_modes" | tr ',' '\n' | grep "^${_aid}:" | head -1 | cut -d: -f2)
    fi
    _effective_modes+=("${_override:-$_fmode}")
    _m=$((_m + 1))
  done

  # Afficher la liste actuelle
  local prev_family=""
  local _i=0
  # _pick_families/_pick_items/_pick_modes sont peuplés par _list_all_agents_grouped (agent-picker.sh)
  # shellcheck disable=SC2154
  while [ "$_i" -lt "$total" ]; do
    local cur_family="${_pick_families[$_i]}"
    if [ "$cur_family" != "$prev_family" ]; then
      printf "\n  \033[0;34m── %s ──\033[0m\n" "$cur_family"
      prev_family="$cur_family"
    fi
    local _aid="${_pick_items[$_i]}"
    local _emode="${_effective_modes[$_i]}"
    local _fmode="${_pick_modes[$_i]:-primary}"
    local _mcolor
    [ "$_emode" = "subagent" ] && _mcolor="\033[0;33m" || _mcolor="\033[0;32m"
    # Indiquer si l'override diffère du frontmatter
    local _override_marker=""
    if [ -n "$(printf '%s\n' "${current_modes:-}" | tr ',' '\n' | grep "^${_aid}:")" ]; then
      _override_marker=" \033[2m(override, défaut: ${_fmode})\033[0m"
    fi
    printf "    ${_mcolor}%-10s\033[0m  %s%s\n" "$_emode" "$_aid" "$_override_marker"
    _i=$((_i + 1))
  done
  echo ""

  # Proposer la modification
  read -rp "  Modifier les modes pour ce projet ? [y/N] : " do_edit </dev/tty
  [[ ! "$do_edit" =~ ^[Yy]$ ]] && { log_info "Aucune modification."; return; }

  echo ""
  echo -e "  Format : \033[1magent-id:mode\033[0m séparés par des virgules"
  echo -e "  Modes disponibles : \033[0;32mprimary\033[0m  \033[0;33msubagent\033[0m  \033[0;36mall\033[0m"
  echo -e "  Laisser vide pour supprimer tous les overrides et revenir aux valeurs par défaut."
  echo ""
  if [ -n "$current_modes" ]; then
    echo -e "  Overrides actuels : \033[1m$current_modes\033[0m"
  fi
  echo ""
  read -rp "  Nouveaux overrides : " new_modes </dev/tty

  if [ -z "$new_modes" ] && [ -z "$current_modes" ]; then
    log_info "Aucune modification."
    return
  fi

  if [ "$new_modes" = "$current_modes" ]; then
    log_info "Aucune modification."
    return
  fi

  _set_project_modes "$id" "$new_modes"
  echo ""
  if [ -z "$new_modes" ]; then
    log_success "Overrides supprimés pour $id — modes frontmatter utilisés par défaut"
  else
    log_success "Overrides mis à jour pour $id : $new_modes"
  fi

  # Proposer un redéploiement immédiat
  echo ""
  read -rp "Redéployer maintenant ? [Y/n] : " redeploy </dev/tty
  redeploy="${redeploy:-Y}"
  if [[ "$redeploy" =~ ^[Yy]$ ]]; then
    exec "$HUB_DIR/oc.sh" deploy "$id"
  else
    log_info "Déployer plus tard : ./oc.sh deploy $id"
  fi
}

# ── VALIDATE ─────────────────────────────────────────────────────────────────

##
# Valide la cohérence de tous les agents canoniques (ou d'un seul si agent-id fourni).
# Vérifie : champs requis, skills existants, targets valides, mode valide, unicité des id.
# @param {string} [$1] — agent-id optionnel (valide uniquement cet agent si fourni)
##
cmd_validate() {
  local filter_id="${1:-}"
  local count_ok=0 count_err=0 count_warn=0

  # Collecter tous les fichiers agents
  local agent_files=()
  while IFS= read -r f; do
    agent_files+=("$f")
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)

  # Pour détecter les doublons d'id
  local seen_ids=""

  local f
  for f in "${agent_files[@]}"; do
    [ -f "$f" ] || continue

    local agent_id label description targets_raw skills_raw mode_raw
    agent_id=$(grep    '^id:'          "$f" 2>/dev/null | head -1 | sed 's/^id:[[:space:]]*//')
    label=$(grep       '^label:'       "$f" 2>/dev/null | head -1 | sed 's/^label:[[:space:]]*//')
    description=$(grep '^description:' "$f" 2>/dev/null | head -1 | sed 's/^description:[[:space:]]*//')
    targets_raw=$(grep '^targets:'     "$f" 2>/dev/null | head -1 | sed 's/^targets:[[:space:]]*//')
    skills_raw=$(grep  '^skills:'      "$f" 2>/dev/null | head -1 | sed 's/^skills:[[:space:]]*//')
    mode_raw=$(grep    '^mode:'        "$f" 2>/dev/null | head -1 | sed 's/^mode:[[:space:]]*//')

    # Filtrer par agent-id si demandé
    if [ -n "$filter_id" ] && [ "$agent_id" != "$filter_id" ]; then
      continue
    fi

    local display_id="${agent_id:-$(basename "$f" .md)}"
    local issues=""   # accumule les messages d'erreur/avertissement pour cet agent
    local has_err=0 has_warn=0

    # ── Champs requis ──────────────────────────────────────────────────────
    for field_name in id label description targets skills; do
      local val
      case "$field_name" in
        id)          val="$agent_id" ;;
        label)       val="$label" ;;
        description) val="$description" ;;
        targets)     val="$targets_raw" ;;
        skills)      val="$skills_raw" ;;
      esac
      if [ -z "$val" ]; then
        issues="${issues}    champ manquant : ${field_name}\n"
        has_warn=1
      fi
    done

    # ── Unicité de l'id ────────────────────────────────────────────────────
    if [ -n "$agent_id" ]; then
      if printf '%s\n' "$seen_ids" | grep -qx "$agent_id" 2>/dev/null; then
        issues="${issues}    id dupliqué : ${agent_id}\n"
        has_err=1
      else
        seen_ids="$seen_ids $agent_id"
      fi
    fi

    # ── Mode valide ────────────────────────────────────────────────────────
    if [ -n "$mode_raw" ]; then
      case "$mode_raw" in
        primary|subagent|all) ;;
        *) issues="${issues}    mode invalide : ${mode_raw} (attendu : primary|subagent|all)\n"
           has_err=1 ;;
      esac
    fi

    # ── Targets valides ────────────────────────────────────────────────────
    if [ -n "$targets_raw" ]; then
      # Normaliser : retirer [ ] et virgules → liste mots
      local targets_clean
      targets_clean=$(printf '%s' "$targets_raw" | tr -d '[]' | tr ',' ' ')
      local t
      for t in $targets_clean; do
        case "$t" in
          opencode) ;;
          *) issues="${issues}    target invalide : ${t} (attendu : opencode)\n"
             has_err=1 ;;
        esac
      done
    fi

    # ── Skills existants ───────────────────────────────────────────────────
    if [ -n "$skills_raw" ]; then
      local skills_clean
      skills_clean=$(printf '%s' "$skills_raw" | tr -d '[]' | tr ',' ' ')
      local sk
      for sk in $skills_clean; do
        local found=0
        # Skill local
        if [ -f "$HUB_DIR/skills/${sk}.md" ]; then
          found=1
        fi
        # Skill externe (external/<name>)
        if [ $found -eq 0 ] && [ -d "$EXTERNAL_SKILLS_DIR" ]; then
          local ext_name="${sk#external/}"
          if [ -f "$EXTERNAL_SKILLS_DIR/${ext_name}.md" ]; then
            found=1
          fi
        fi
        if [ $found -eq 0 ]; then
          issues="${issues}    skill introuvable : ${sk}\n"
          has_err=1
        fi
      done
    fi

    # ── Affichage résultat pour cet agent ──────────────────────────────────
    if [ $has_err -eq 1 ]; then
      echo -e "  ${RED}✘${RESET}  ${BOLD}${display_id}${RESET}"
      printf '%b' "$issues"
      count_err=$(( count_err + 1 ))
    elif [ $has_warn -eq 1 ]; then
      echo -e "  ${YELLOW}⚠${RESET}  ${BOLD}${display_id}${RESET}"
      printf '%b' "$issues"
      count_warn=$(( count_warn + 1 ))
    else
      echo -e "  ${GREEN}✔${RESET}  ${display_id}"
      count_ok=$(( count_ok + 1 ))
    fi
  done

  # ── Résumé ─────────────────────────────────────────────────────────────────
  echo ""
  local summary
  summary="${BOLD}$(t agent.validate.summary)${RESET}  ${GREEN}${count_ok} $(t agent.validate.ok)${RESET}"
  [ $count_err  -gt 0 ] && summary="${summary}  ${RED}${count_err} $(t agent.validate.errors)${RESET}"
  [ $count_warn -gt 0 ] && summary="${summary}  ${YELLOW}${count_warn} $(t agent.validate.warnings)${RESET}"
  echo -e "$summary"
  echo ""

  [ $count_err -gt 0 ] && exit 1
  return 0
}

# ── DEPLOY (agent unique) ─────────────────────────────────────────────────────

##
# Déploie un seul agent canonique vers toutes les cibles actives (ou celles du projet).
# Utile pour redéployer un agent après modification sans tout redéployer.
# @param {string} $1 — agent-id
# @param {string} [$2] — PROJECT_ID (optionnel)
##
cmd_deploy() {
  local agent_id="${1:-}" project_id="${2:-}"
  [ -z "$agent_id" ] && { log_error "Usage : oc agent deploy <agent-id> [PROJECT_ID]"; exit 1; }

  source "$LIB_DIR/adapter-manager.sh"
  source "$LIB_DIR/prompt-builder.sh"

  # Retrouver le fichier source de l'agent
  local agent_file
  agent_file=$(_find_agent_file "$agent_id")
  if [ -z "$agent_file" ] || [ ! -f "$agent_file" ]; then
    log_error "Agent introuvable : $agent_id"
    exit 1
  fi

  # Résoudre le dossier de déploiement
  local deploy_dir="$HUB_DIR"
  if [ -n "$project_id" ]; then
    project_id=$(normalize_project_id "$project_id")
    deploy_dir=$(resolve_project_path "$project_id")
    log_info "Projet cible : $project_id ($deploy_dir)"
  fi

  # Résoudre les cibles
  local targets=("opencode")

  # Langue du projet
  local lang=""
  [ -n "$project_id" ] && lang=$(get_project_language "$project_id" 2>/dev/null || true)
  lang=$(resolve_agent_lang "$lang")

  log_title "Déploiement de l'agent : $agent_id"
  echo ""

  local deployed=0
  for tgt in "${targets[@]}"; do
    # Répertoire de sortie selon la cible
    local out_dir="" out_file=""
    case "$tgt" in
      opencode)
        out_dir="$deploy_dir/.opencode/agents"
        out_file="$out_dir/${agent_id}.md"
        ;;

      *)
        log_warn "Cible non reconnue : $tgt — ignorée"
        continue
        ;;
    esac

    mkdir -p "$out_dir"
    if build_agent_content "$agent_file" "$tgt" "$lang" "$deploy_dir" > "$out_file"; then
      log_success "$tgt : $agent_id → $out_file"
      deployed=$((deployed + 1))
    else
      log_error "$tgt : échec du déploiement de $agent_id"
    fi
  done

  echo ""
  if [ "$deployed" -gt 0 ]; then
    log_success "$agent_id déployé sur $deployed cible(s)"
  else
    log_warn "Aucune cible n'a accepté $agent_id"
  fi
  echo ""
}

# ── DISPATCH ─────────────────────────────────────────────────────────────────

SUBCOMMAND="${1:-}"
shift 2>/dev/null || true

case "$SUBCOMMAND" in
  list)     cmd_list ;;
  info)     cmd_info "$@" ;;
  create)   cmd_create ;;
  edit)     cmd_edit "$@" ;;
  select)   cmd_select "$@" ;;
  mode)     cmd_mode "$@" ;;
  validate) cmd_validate "$@" ;;
  deploy)   cmd_deploy "$@" ;;
  keytest)  cmd_keytest ;;
  *)
    echo -e "${BOLD}$(t agent.title)${RESET}"
    echo ""
    echo "  $(t agent.list)"
    echo "  $(t agent.info_cmd)"
    echo "  $(t agent.create_cmd)"
    echo "  $(t agent.edit_cmd)"
    echo "  $(t agent.select_cmd)"
    echo "  $(t agent.mode_cmd)"
    echo "  $(t agent.validate_cmd)"
    echo "  $(t agent.deploy_cmd)"
    echo ""
    echo -e "${BOLD}$(t agent.examples)${RESET}"
    echo "  ./oc.sh agent list"
    echo "  ./oc.sh agent create"
    echo "  ./oc.sh agent edit developer"
    echo "  ./oc.sh agent select MY-PROJECT"
    echo "  ./oc.sh agent mode MY-PROJECT"
    echo "  ./oc.sh agent info planner"
    echo "  ./oc.sh agent validate"
    echo "  ./oc.sh agent validate planner"
    echo "  ./oc.sh agent deploy planner"
    echo "  ./oc.sh agent deploy planner MY-PROJECT"
    echo "  ./oc.sh agent keytest"
    echo ""
    ;;
esac
