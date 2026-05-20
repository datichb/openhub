#!/bin/bash
# Picker interactif de cibles de déploiement (opencode).
# Dépendances : common.sh (déjà sourcé), lib/tui-picker.sh.
#
# Fonctions publiques :
#   _pick_project_targets <current_csv>   — sélecteur interactif, résultat dans $PICKED_TARGETS
#   _set_project_targets <id> <csv>       — écrit le champ "- Targets :" dans projects.md

[ "${_TARGET_PICKER_LOADED:-}" = "1" ] && return 0
_TARGET_PICKER_LOADED=1

source "$LIB_DIR/tui-picker.sh"

# Cibles disponibles (ordre d'affichage)
_AVAILABLE_TARGETS=("opencode")

##
# Rendu du sélecteur de cibles (compatible bash 3.2).
# Utilise les variables partagées de _pick_from_list :
#   _pick_items, _pick_checked, _pick_cursor, _pick_total
##
_render_targets_page() {
  # Variables partagées par dynamic scoping depuis _pick_from_list :
  # _pick_cursor, _pick_total, _pick_items, _pick_checked — assignées par le caller
  # shellcheck disable=SC2154
  printf "\033[2J\033[H"

  # ── En-tête ────────────────────────────────────────────────────────────────
  echo -e "${BOLD}Sélection des cibles de déploiement${RESET}"
  echo -e "  \033[0;34m↑↓\033[0m naviguer   \033[0;34mespace\033[0m cocher/décocher   \033[0;34m*\033[0m tout cocher   \033[0;34mentrée\033[0m valider   \033[0;34mESC\033[0m annuler"
  echo ""

  # ── Liste ──────────────────────────────────────────────────────────────────
  local j=0
  while [ "$j" -lt "$_pick_total" ]; do
    local check_icon="   "
    local check_color="" check_reset=""
    if [ "${_pick_checked[$j]}" = "1" ]; then
      check_icon="[x]"
      check_color="$GREEN"
      check_reset="$RESET"
    fi
    # shellcheck disable=SC2154
    if [ "$j" -eq "$_pick_cursor" ]; then
      printf "  \033[1m> ${check_color}%-3s${check_reset}\033[1m  %s\033[0m\n" \
        "$check_icon" "${_pick_items[$j]}"
    else
      printf "    ${check_color}%-3s${check_reset}  %s\n" \
        "$check_icon" "${_pick_items[$j]}"
    fi
    j=$((j + 1))
  done

  # ── Pied ───────────────────────────────────────────────────────────────────
  echo ""
  local count=0
  local v
  for v in "${_pick_checked[@]}"; do [ "$v" = "1" ] && count=$((count+1)); done
  echo -e "  ${BOLD}$count cible(s) sélectionnée(s)${RESET}  (au moins 1 requise)"
  echo ""
}

##
# Sélection interactive des cibles d'un projet.
# Compatible bash 3.2 (macOS). Résultat dans $PICKED_TARGETS (CSV ou "all").
# @param {string} $1 — sélection courante ("all" ou CSV de cibles)
##
_pick_project_targets() {
  local current_csv="${1:-all}"

  # Normaliser "all" → sélectionner toutes les cibles
  local init_csv="$current_csv"
  if [ "$current_csv" = "all" ]; then
    init_csv=$(printf '%s\n' "${_AVAILABLE_TARGETS[@]}" | tr '\n' ',' | sed 's/,$//')
  fi

  _pick_items=("${_AVAILABLE_TARGETS[@]}")
  _pick_total=${#_pick_items[@]}

  # Nettoyer le CSV courant
  local clean_csv
  clean_csv=$(printf '%s' "$init_csv" | tr -d '"' | tr ',' '\n' \
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
  _pick_allow_zero=0   # Au moins une cible doit rester sélectionnée
  _pick_allow_star=1
  _PICK_RESULT=""

  _pick_from_list "$init_csv" "$init_csv"

  # ESC ou résultat vide → garder la sélection courante
  if [ -z "$_PICK_RESULT" ] || [ "$_PICK_RESULT" = "$init_csv" ]; then
    log_warn "Sélection annulée (ESC) — cibles inchangées."
    PICKED_TARGETS="$current_csv"
    return
  fi

  # Si toutes les cibles cochées → "all"
  local all_count=${#_AVAILABLE_TARGETS[@]}
  local sel_count
  sel_count=$(printf '%s\n' "$_PICK_RESULT" | tr ',' '\n' | grep -v '^$' | wc -l | tr -d ' ')
  # PICKED_TARGETS est une variable de résultat exportée vers le caller
  # shellcheck disable=SC2034
  if [ "$sel_count" -eq "$all_count" ]; then
    PICKED_TARGETS="all"
  else
    PICKED_TARGETS="$_PICK_RESULT"
  fi
}

##
# Met à jour le champ "- Targets :" dans le bloc d'un projet dans projects.md.
# Pattern identique à _set_project_agents dans agent-picker.sh.
# @param $1 — PROJECT_ID
# @param $2 — valeur (CSV ou "all")
##
_set_project_targets() {
  local id="$1" new_targets="$2"
  # Tenter de remplacer une ligne "- Targets : *" existante dans le bloc du projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Targets : [^\n]+)}{\${1}- Targets : ${new_targets}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Targets : ${new_targets}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Si le champ n'existe pas encore, l'ajouter après "- Agents :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Agents : [^\n]+\n)}{\${1}- Targets : ${new_targets}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Targets : ${new_targets}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : ajouter après "- Labels :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Labels : [^\n]+\n)}{\${1}- Targets : ${new_targets}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Targets : ${new_targets}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Targets dans le bloc $id de projects.md"
  return 1
}
