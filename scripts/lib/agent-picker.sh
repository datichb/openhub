#!/bin/bash
# Picker interactif d'agents groupés par famille.
# Dépendances : common.sh (déjà sourcé), lib/tui-picker.sh.
#
# Fonctions publiques :
#   _pick_agents <current_csv>   — sélecteur interactif, résultat dans $PICKED_AGENTS
#   _set_project_agents <id> <csv> — écrit le champ "- Agents :" dans projects.md

[ "${_AGENT_PICKER_LOADED:-}" = "1" ] && return 0
_AGENT_PICKER_LOADED=1

source "$LIB_DIR/tui-picker.sh"

# ── Données internes ─────────────────────────────────────────────────────────
# Tableaux parallèles à _pick_items[] :
#   _pick_families[]     — famille/catégorie de chaque agent (pour séparateurs visuels)
#   _pick_descriptions[] — description de chaque agent (précalculée, évite I/O au rendu)
#   _pick_modes[]        — mode de déploiement de chaque agent (primary|subagent|all)
_pick_families=()
_pick_descriptions=()
_pick_modes=()

##
# Liste tous les agents groupés par famille.
# Remplit les tableaux globaux _pick_items[], _pick_families[], _pick_descriptions[] et _pick_modes[].
##
_list_all_agents_grouped() {
  _pick_items=()
  _pick_families=()
  _pick_descriptions=()
  _pick_modes=()
  while IFS= read -r agent_file; do
    [ -f "$agent_file" ] || continue
    local agent_id family agent_desc agent_mode
    agent_id=$(grep '^id:' "$agent_file" 2>/dev/null | head -1 | sed 's/^id:[[:space:]]*//')
    family=$(basename "$(dirname "$agent_file")")
    agent_desc=$(grep '^description:' "$agent_file" 2>/dev/null | head -1 | sed 's/^description:[[:space:]]*//')
    agent_mode=$(grep '^mode:' "$agent_file" 2>/dev/null | head -1 | sed 's/^mode:[[:space:]]*//')
    agent_mode="${agent_mode:-primary}"
    [ -z "$agent_id" ] && continue
    _pick_items+=("$agent_id")
    _pick_families+=("$family")
    _pick_descriptions+=("$agent_desc")
    _pick_modes+=("$agent_mode")
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
}

##
# Rendu du sélecteur d'agents (compatible bash 3.2).
# Affiche une liste plate avec séparateurs de famille.
# Utilise les variables partagées de _pick_from_list :
#   _pick_items, _pick_checked, _pick_cursor, _pick_total
# Plus _pick_families[] (tableau parallèle).
##
_render_agents_page() {
  # Variables partagées par dynamic scoping depuis _pick_from_list :
  # _pick_cursor, _pick_total, _pick_items, _pick_checked, _pick_families
  # shellcheck disable=SC2154
  local page_size=15
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
  echo -e "${BOLD}Sélection des agents${RESET}  ($((_pick_cursor+1))/$_pick_total)"
  echo -e "  \033[0;34m↑↓\033[0m naviguer   \033[0;34mespace\033[0m cocher/décocher   \033[0;34mc\033[0m catégorie   \033[0;34m*\033[0m tout cocher   \033[0;34m0\033[0m tout vider   \033[0;34mentrée\033[0m valider   \033[0;34mESC\033[0m annuler"
  echo ""

  # ── Liste (fenêtre glissante) avec séparateurs de famille ─────────────────
  local prev_family=""
  local i=$win_start
  while [ "$i" -lt "$win_end" ]; do
    local cur_family="${_pick_families[$i]}"
    # Séparateur de famille si on change de famille
    if [ "$cur_family" != "$prev_family" ]; then
      printf "  \033[0;34m── %s ──\033[0m\n" "$cur_family"
      prev_family="$cur_family"
    fi

    local agent="${_pick_items[$i]}"

    local check_icon="   "
    local check_color=""
    local check_reset=""
    if [ "${_pick_checked[$i]}" = "1" ]; then
      check_icon="[x]"
      check_color="$GREEN"
      check_reset="$RESET"
    fi

    if [ "$i" -eq "$_pick_cursor" ]; then
      printf "  \033[1m> ${check_color}%-3s${check_reset}\033[1m  %-40s\033[0m\n" \
        "$check_icon" "$agent"
    else
      printf "    ${check_color}%-3s${check_reset}  %-40s\n" \
        "$check_icon" "$agent"
    fi
    i=$((i + 1))
  done

  # ── Séparateur ────────────────────────────────────────────────────────────
  echo ""
  printf "  \033[0;34m%s\033[0m\n" "────────────────────────────────────────────────────────────"

  # ── Panneau description de l'agent sous le curseur ────────────────────────
  local cur_agent="${_pick_items[$_pick_cursor]}"
  local cur_desc="${_pick_descriptions[$_pick_cursor]}"
  local cur_mode="${_pick_modes[$_pick_cursor]:-primary}"

  # Couleur et label selon le mode
  local mode_label mode_color
  if [ "$cur_mode" = "subagent" ]; then
    mode_label="subagent"
    mode_color="\033[0;33m"   # jaune
  elif [ "$cur_mode" = "all" ]; then
    mode_label="primary+subagent"
    mode_color="\033[0;36m"   # cyan
  else
    mode_label="primary"
    mode_color="\033[0;32m"   # vert
  fi

  echo ""
  printf "  \033[1m%s\033[0m  [%s]  ${mode_color}(%s)\033[0m\n" \
    "$cur_agent" "${_pick_families[$_pick_cursor]}" "$mode_label"
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
  echo -e "  ${BOLD}$count agent(s) sélectionné(s)${RESET}  (${_pick_total} disponibles)"
  echo ""
}

##
# Sélection interactive d'agents avec navigation flèches + espace.
# Compatible bash 3.2 (macOS). Résultat dans $PICKED_AGENTS ("all" ou CSV).
# @param {string} $1 — sélection courante ("all" ou CSV d'agent IDs)
##
_pick_agents() {
  local current_csv="${1:-all}"

  # Charger tous les agents groupés par famille
  _list_all_agents_grouped

  if [ ${#_pick_items[@]} -eq 0 ]; then
    log_warn "Aucun agent disponible dans agents/."
    PICKED_AGENTS="$current_csv"
    return
  fi

  _pick_total=${#_pick_items[@]}

  # Initialiser le tableau de sélection
  _pick_checked=()
  local i=0
  if [ "$current_csv" = "all" ]; then
    # Tout coché
    while [ "$i" -lt "$_pick_total" ]; do
      _pick_checked+=("1")
      i=$((i + 1))
    done
  else
    # Cocher uniquement les agents dans le CSV
    local clean_csv
    clean_csv=$(printf '%s' "$current_csv" | tr -d '"' | tr ',' '\n' \
      | sed 's/^ *//;s/ *$//' | grep -v '^$' | tr '\n' ',' | sed 's/,$//')
    while [ "$i" -lt "$_pick_total" ]; do
      if echo ",$clean_csv," | grep -qF ",${_pick_items[$i]},"; then
        _pick_checked+=("1")
      else
        _pick_checked+=("0")
      fi
      i=$((i + 1))
    done
  fi

  _pick_render_fn="_render_agents_page"
  _pick_allow_zero=1
  _pick_allow_star=1
  _pick_allow_family_toggle=1
  _PICK_RESULT=""

  _pick_from_list "$current_csv" "$current_csv"

  # Si annulé (ESC), garder la sélection courante
  if [ "$_PICK_RESULT" = "$current_csv" ] && [ -n "$current_csv" ]; then
    log_warn "Sélection annulée (ESC) — agents inchangés."
    PICKED_AGENTS="$current_csv"
    return
  fi

  # Si tous cochés → "all"
  local all_checked=1
  local j=0
  while [ "$j" -lt "$_pick_total" ]; do
    if [ "${_pick_checked[$j]}" != "1" ]; then
      all_checked=0
      break
    fi
    j=$((j + 1))
  done

  # PICKED_AGENTS est une variable de résultat exportée vers le caller
  # shellcheck disable=SC2034
  if [ "$all_checked" = "1" ]; then
    PICKED_AGENTS="all"
  elif [ -z "$_PICK_RESULT" ]; then
    PICKED_AGENTS=""
  else
    PICKED_AGENTS="$_PICK_RESULT"
  fi
}

# ── Agents natifs OpenCode ────────────────────────────────────────────────────

##
# Rendu du sélecteur d'agents natifs OpenCode (build, plan, general, explore).
# Affiche la liste plate sans séparateurs de famille.
# Utilise les variables partagées de _pick_from_list :
#   _pick_items, _pick_checked, _pick_cursor, _pick_total
# Plus _pick_descriptions[] (tableau parallèle).
##
_render_native_agents_page() {
  # Variables partagées par dynamic scoping depuis _pick_from_list :
  # _pick_cursor, _pick_total, _pick_items, _pick_checked, _pick_descriptions
  # shellcheck disable=SC2154
  printf "\033[2J\033[H"  # clear screen

  # ── En-tête ────────────────────────────────────────────────────────────────
  echo -e "${BOLD}Agents natifs OpenCode à désactiver${RESET}  ($((_pick_cursor+1))/$_pick_total)"
  echo -e "  \033[0;34m↑↓\033[0m naviguer   \033[0;34mespace\033[0m cocher/décocher   \033[0;34m*\033[0m tout cocher   \033[0;34m0\033[0m tout vider   \033[0;34mentrée\033[0m valider   \033[0;34mESC\033[0m annuler"
  echo ""

  # ── Liste ──────────────────────────────────────────────────────────────────
  local i=0
  while [ "$i" -lt "$_pick_total" ]; do
    local agent="${_pick_items[$i]}"

    local check_icon="   "
    local check_color=""
    local check_reset=""
    if [ "${_pick_checked[$i]}" = "1" ]; then
      check_icon="[x]"
      check_color="$GREEN"
      check_reset="$RESET"
    fi

    if [ "$i" -eq "$_pick_cursor" ]; then
      printf "  \033[1m> ${check_color}%-3s${check_reset}\033[1m  %-20s\033[0m  \033[2m%s\033[0m\n" \
        "$check_icon" "$agent" "${_pick_descriptions[$i]}"
    else
      printf "    ${check_color}%-3s${check_reset}  %-20s  \033[2m%s\033[0m\n" \
        "$check_icon" "$agent" "${_pick_descriptions[$i]}"
    fi
    i=$((i + 1))
  done

  # ── Pied ───────────────────────────────────────────────────────────────────
  echo ""
  local count=0
  local v
  for v in "${_pick_checked[@]}"; do [ "$v" = "1" ] && count=$((count+1)); done
  echo -e "  ${BOLD}$count agent(s) à désactiver${RESET}"
  echo ""
}

##
# Sélection interactive des agents natifs OpenCode à désactiver.
# Compatible bash 3.2 (macOS). Résultat dans $PICKED_DISABLED_AGENTS (CSV ou "").
# @param {string} $1 — sélection courante (CSV d'agents à désactiver, ou "")
##
_pick_native_agents() {
  local current_csv="${1:-}"

  _pick_items=("build" "plan" "general" "explore")
  _pick_descriptions=(
    "Lance des builds/compilations automatiquement"
    "Génère un plan avant d'exécuter les tâches"
    "Agent généraliste polyvalent"
    "Exploration et analyse du code"
  )
  _pick_total=4

  # Initialiser le tableau de sélection depuis le CSV courant
  _pick_checked=()
  local i=0
  while [ "$i" -lt "$_pick_total" ]; do
    if [ -n "$current_csv" ] && echo ",${current_csv}," | grep -qF ",${_pick_items[$i]},"; then
      _pick_checked+=("1")
    else
      _pick_checked+=("0")
    fi
    i=$((i + 1))
  done

  _pick_render_fn="_render_native_agents_page"
  _pick_allow_zero=1
  _pick_allow_star=1
  _pick_allow_family_toggle=0
  _PICK_RESULT=""

  _pick_from_list "$current_csv" "$current_csv"

  # ESC → garder la sélection courante sans message (champ optionnel)
  if [ "$_PICK_RESULT" = "$current_csv" ]; then
    PICKED_DISABLED_AGENTS="$current_csv"
    return
  fi

  # PICKED_DISABLED_AGENTS est une variable de résultat exportée vers le caller
  # shellcheck disable=SC2034
  PICKED_DISABLED_AGENTS="$_PICK_RESULT"
}

##
# Met à jour le champ "- Agents :" dans le bloc d'un projet dans projects.md.
# Pattern identique à _set_project_tracker dans cmd-beads.sh.
# @param $1 — PROJECT_ID
# @param $2 — valeur (CSV ou "all")
##
_set_project_agents() {
  local id="$1" new_agents="$2"
  # Contraindre le match au bloc du projet uniquement (stopper avant le prochain ## header)
  # Le pattern (?:(?!^##)[^\n]*\n)* matche des lignes qui ne commencent pas par ##
  # Helper de vérification : s'assure que le champ Agents est bien dans le bloc ${id}
  _verify_agents_in_block() {
    # Extraire uniquement le bloc du projet (jusqu'au prochain ## header ou fin de fichier)
    awk "/^## ${id}/{found=1; next} found && /^## /{exit} found{print}" "$PROJECTS_FILE" \
      | grep -qF -- "- Agents : ${new_agents}"
  }
  # Tenter de remplacer une ligne "- Agents : *" existante dans le bloc du projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?)(- Agents : [^\n]+)}{\${1}- Agents : ${new_agents}}ms
  " "$PROJECTS_FILE" 2>/dev/null && _verify_agents_in_block; then
    return 0
  fi
  # Si le champ n'existe pas encore, l'ajouter après "- Labels :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- Labels : [^\n]+\n)}{\${1}- Agents : ${new_agents}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && _verify_agents_in_block; then
    return 0
  fi
  # Fallback : ajouter après le dernier champ "- " du bloc projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- [^\n]+\n)}{\${1}- Agents : ${new_agents}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && _verify_agents_in_block; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Agents dans le bloc $id de projects.md"
  return 1
}
