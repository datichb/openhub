#!/bin/bash
set -euo pipefail

# ── Board kanban terminal — oc beads board ─────────────────────────────────────
# Affiche un tableau kanban dans le terminal avec les 4 colonnes actives :
# OPEN | IN PROGRESS | REVIEW | BLOCKED
#
# Usage :
#   cmd_board <PROJECT_ID> [--watch] [--interval <sec>]
#
# Options :
#   --watch              Rafraîchissement automatique (Ctrl+C pour quitter)
#   --interval <sec>     Intervalle en secondes entre rafraîchissements (défaut : 5)

# ── Helpers internes ──────────────────────────────────────────────────────────

# Longueur visible d'une chaîne (ignore les séquences ANSI/échappement)
_visible_len() {
  printf '%s' "$1" | sed 's/\x1b\[[0-9;]*[mK]//g' | awk '{print length}'
}

# Tronque une chaîne à N caractères visibles, ajoute "…" si tronqué
_trunc() {
  local str="$1" max="$2"
  if [ "${#str}" -gt "$max" ]; then
    printf '%s…' "${str:0:$((max-1))}"
  else
    printf '%s' "$str"
  fi
}

# Tronque au milieu en gardant début et fin : "abc…xyz"
# Utile pour les ids dont le suffixe numérique est la partie significative
_trunc_mid() {
  local str="$1" max="$2"
  local len=${#str}
  if [ "$len" -le "$max" ]; then
    printf '%s' "$str"
    return
  fi
  # max doit être >= 3 (au moins 1 char de chaque côté + "…")
  [ "$max" -lt 3 ] && max=3
  local tail=$(( (max - 1) / 2 ))   # partie droite (favorise la fin)
  local head=$(( max - 1 - tail ))   # partie gauche
  printf '%s…%s' "${str:0:$head}" "${str:$((len - tail))}"
}

# Répète un caractère N fois
_repeat() {
  local char="$1" n="$2"
  [ "$n" -le 0 ] && return
  printf '%*s' "$n" '' | tr ' ' "$char"
}

# Pad une chaîne à N caractères visibles (strip les codes ANSI pour compter)
_pad() {
  local str="$1" width="$2"
  local visible_len
  visible_len=$(_visible_len "$str")
  local pad=$(( width - visible_len ))
  [ $pad -lt 0 ] && pad=0
  printf '%s%*s' "$str" "$pad" ''
}

# Badge de priorité coloré (2 chars visibles)
_priority_badge() {
  local p="$1"
  case "$p" in
    0) printf '%b' "${RED}${BOLD}P0${RESET}" ;;
    1) printf '%b' "${YELLOW}P1${RESET}" ;;
    2) printf '%b' "${DIM}P2${RESET}" ;;
    3) printf '%b' "${DIM}P3${RESET}" ;;
    *) printf '%b' "${DIM}??${RESET}" ;;
  esac
}

# Type en CYAN tronqué à 7 chars visibles
_type_badge() {
  local t="$1"
  printf '%b%s%b' "${CYAN}" "$(_trunc "$t" 7)" "${RESET}"
}

# Wrap un titre sur 2 lignes maximum dans _WRAP_L1 et _WRAP_L2.
# Priorité de coupure :
#   1. Titre court (tient en 1 ligne)   → L1 = titre, L2 = ""
#   2. Espace dans les max premiers chars → couper au dernier espace
#   3. Aucun espace (mot trop long)       → couper à max-1 et ajouter "-"
# La ligne 2 est tronquée avec "…" si elle dépasse encore max chars.
_wrap_title() {
  local str="$1"
  local max="$2"
  local len=${#str}

  if [ "$len" -le "$max" ]; then
    _WRAP_L1="$str"
    _WRAP_L2=""
    return
  fi

  # Chercher le dernier espace dans la fenêtre [0..max]
  local window="${str:0:$max}"
  # ${window% *} supprime le dernier mot → donne la partie avant le dernier espace
  local before_last_space="${window% *}"

  if [ "${#before_last_space}" -gt 0 ] && [ "$before_last_space" != "$window" ]; then
    # Coupure sur espace
    _WRAP_L1="$before_last_space"
    local rest="${str:$(( ${#before_last_space} + 1 ))}"
    _WRAP_L2="$(_trunc "$rest" "$max")"
  else
    # Aucun espace — coupure forcée avec tiret
    _WRAP_L1="${str:0:$(( max - 1 ))}-"
    local rest="${str:$(( max - 1 ))}"
    _WRAP_L2="$(_trunc "$rest" "$max")"
  fi
}

# ── Rendu d'une colonne ───────────────────────────────────────────────────────
# @param $1 — label de la colonne (ex: "OPEN")
# @param $2 — couleur de la bordure (variable ANSI, ex: "$DIM")
# @param $3 — tickets JSON (tableau jq, peut être vide "[]")
# @param $4 — largeur de colonne (inner width, sans les bordures │ │)
# Retourne les lignes dans la variable globale _COL_LINES
_render_column() {
  local label="$1"
  local col_color="$2"
  local tickets_json="$3"
  local inner_w="$4"

  _COL_LINES=()

  # ── en-tête : ┌─ LABEL ───────┐ ──
  local header_text=" ${label} "
  # -2 : on a déjà "┌─" (1 dash) avant le label
  local dashes_total=$(( inner_w - ${#header_text} - 1 ))
  [ $dashes_total -lt 0 ] && dashes_total=0
  local dashes
  dashes=$(_repeat '─' "$dashes_total")
  _COL_LINES+=("${col_color}┌─${RESET}${BOLD}${header_text}${RESET}${col_color}${dashes}┐${RESET}")

  # ── lignes de tickets ──
  local count
  count=$(echo "$tickets_json" | jq 'length' 2>/dev/null || echo "0")

  # Ligne vide interne réutilisable (1 ligne de respiration)
  local blank_line
  blank_line="${col_color}│${RESET}$(printf '%*s' "$inner_w" '')${col_color}│${RESET}"

  if [ "$count" -eq 0 ]; then
    # Colonne vide — message centré
    local empty_msg
    empty_msg="$(t board.empty_column)"
    local msg_len=${#empty_msg}
    local pad_left=$(( (inner_w - msg_len) / 2 ))
    local pad_right=$(( inner_w - msg_len - pad_left ))
    [ $pad_left  -lt 0 ] && pad_left=0
    [ $pad_right -lt 0 ] && pad_right=0
    _COL_LINES+=("$blank_line")
    _COL_LINES+=("${col_color}│${RESET}$(printf '%*s' $pad_left '')${DIM}${empty_msg}${RESET}$(printf '%*s' $pad_right '')${col_color}│${RESET}")
    _COL_LINES+=("$blank_line")
  else
    local i
    for (( i=0; i<count; i++ )); do
      local ticket
      ticket=$(echo "$tickets_json" | jq -r ".[$i]" 2>/dev/null)
      [ -z "$ticket" ] && continue

      local id title priority type
      id=$(echo "$ticket"       | jq -r '.id       // "?"' 2>/dev/null)
      title=$(echo "$ticket"    | jq -r '.title    // "?"' 2>/dev/null)
      priority=$(echo "$ticket" | jq -r '.priority // "2"' 2>/dev/null)
      type=$(echo "$ticket"     | jq -r '.type     // ""'  2>/dev/null)

      # Ligne 1 : id  ·  P1  ·  feature
      # Parties fixes : " " (1) + " · P? · " (8) = 9 chars réservés pour les séparateurs/badge prio
      # Plus type tronqué à 7 max → réservé = 9 + min(len(type),7)
      local type_trunc
      type_trunc=$(_trunc "$type" 7)
      local type_len=${#type_trunc}
      # Espace disponible pour l'id : inner_w - 1(espace) - 3(" · ") - 2("P?") - 3(" · ") - type_len
      local id_max=$(( inner_w - 1 - 3 - 2 - 3 - type_len ))
      [ $id_max -lt 3 ] && id_max=3
      local id_trunc
      id_trunc=$(_trunc_mid "$id" "$id_max")

      # Longueur visible totale du contenu entre les bordures
      local meta_visible=" ${id_trunc} · P${priority} · ${type_trunc}"
      local meta_len=${#meta_visible}
      local meta_pad=$(( inner_w - meta_len ))
      [ $meta_pad -lt 0 ] && meta_pad=0

      local p_badge; p_badge=$(_priority_badge "$priority")
      local t_badge; t_badge=$(_type_badge "$type_trunc")

      _COL_LINES+=("${col_color}│${RESET} ${BOLD}${id_trunc}${RESET} ${DIM}·${RESET} ${p_badge} ${DIM}·${RESET} ${t_badge}$(printf '%*s' $meta_pad '')${col_color}│${RESET}")

      # Lignes 2 & 3 : titre wrappé sur 2 lignes fixes
      # title_max = inner_w - 1 (espace gauche) - 1 (espace droit)
      local title_max=$(( inner_w - 2 ))
      _WRAP_L1=""
      _WRAP_L2=""
      _wrap_title "$title" "$title_max"

      local tl1_pad=$(( title_max - ${#_WRAP_L1} ))
      [ $tl1_pad -lt 0 ] && tl1_pad=0
      _COL_LINES+=("${col_color}│${RESET} ${DIM}${_WRAP_L1}${RESET}$(printf '%*s' $tl1_pad '') ${col_color}│${RESET}")

      local tl2_pad=$(( title_max - ${#_WRAP_L2} ))
      [ $tl2_pad -lt 0 ] && tl2_pad=0
      _COL_LINES+=("${col_color}│${RESET} ${DIM}${_WRAP_L2}${RESET}$(printf '%*s' $tl2_pad '') ${col_color}│${RESET}")

      # Séparateur entre tickets (sauf après le dernier)
      if (( i < count - 1 )); then
        local sep_line
        sep_line=$(_repeat '╌' "$inner_w")
        _COL_LINES+=("${col_color}│${DIM}${sep_line}${RESET}${col_color}│${RESET}")
      else
        _COL_LINES+=("$blank_line")
      fi
    done
  fi

  # ── pied : └──────────────┘ ──
  local bottom_line
  bottom_line=$(_repeat '─' "$inner_w")
  _COL_LINES+=("${col_color}└${bottom_line}┘${RESET}")
}

# ── Rendu complet du board ────────────────────────────────────────────────────
_render_board() {
  local project_id="$1"
  local project_path="$2"

  # Vérifier que bd est disponible et que .beads existe
  _require_bd
  _require_beads_init "$project_path" "$project_id"

  # Récupérer les tickets par statut (1 appel au lieu de 4)
  local _all_tickets t_open t_inprog t_review t_blocked
  _all_tickets=$(bd -C "$project_path" list --status open,in_progress,review,blocked --json --no-tree 2>/dev/null || echo "[]")
  t_open=$(echo    "$_all_tickets" | jq '[.[] | select(.status == "open")]'        2>/dev/null || echo "[]")
  t_inprog=$(echo  "$_all_tickets" | jq '[.[] | select(.status == "in_progress")]' 2>/dev/null || echo "[]")
  t_review=$(echo  "$_all_tickets" | jq '[.[] | select(.status == "review")]'      2>/dev/null || echo "[]")
  t_blocked=$(echo "$_all_tickets" | jq '[.[] | select(.status == "blocked")]'     2>/dev/null || echo "[]")

  # ── Layout adaptatif ──
  local term_w
  term_w=$(tput cols 2>/dev/null || echo 100)

  # 4 colonnes + 2 chars bordure par colonne (│…│) + 2 espaces entre chaque (×3)
  # gaps = 2 × 3 = 6 ; borders = 2 × 4 = 8 → colonnes occupent toute la largeur utile
  local gaps=$(( 2 * 3 ))    # 6
  local borders=$(( 2 * 4 )) # 8
  local available=$(( term_w - gaps - borders ))
  local col_inner=$(( available / 4 ))
  [ $col_inner -lt 18 ] && col_inner=18

  # ── Compter les tickets pour le footer ──
  local cnt_open cnt_inprog cnt_review cnt_blocked
  cnt_open=$(echo "$t_open"      | jq 'length' 2>/dev/null || echo "0")
  cnt_inprog=$(echo "$t_inprog"  | jq 'length' 2>/dev/null || echo "0")
  cnt_review=$(echo "$t_review"  | jq 'length' 2>/dev/null || echo "0")
  cnt_blocked=$(echo "$t_blocked"| jq 'length' 2>/dev/null || echo "0")

  # ── Titre ──
  local now
  now=$(date '+%A %d %B %Y' 2>/dev/null || date)
  echo ""
  printf '%b\n' "${BOLD}◆  Board — ${project_id}${RESET}  ${DIM}${now}${RESET}"
  echo ""

  # ── Rendre les 4 colonnes ──
  local col_w=$(( col_inner + 2 ))  # +2 pour les bordures │ et │

  declare -a lines_open lines_inprog lines_review lines_blocked

  _COL_LINES=()
  _render_column "OPEN"        "$DIM"    "$t_open"    "$col_inner"
  lines_open=("${_COL_LINES[@]}")

  _COL_LINES=()
  _render_column "IN PROGRESS" "$BLUE"   "$t_inprog"  "$col_inner"
  lines_inprog=("${_COL_LINES[@]}")

  _COL_LINES=()
  _render_column "REVIEW"      "$YELLOW" "$t_review"  "$col_inner"
  lines_review=("${_COL_LINES[@]}")

  _COL_LINES=()
  _render_column "BLOCKED"     "$RED"    "$t_blocked" "$col_inner"
  lines_blocked=("${_COL_LINES[@]}")

  # ── Fusionner les colonnes ligne par ligne ──
  local max_lines="${#lines_open[@]}"
  [ "${#lines_inprog[@]}"  -gt "$max_lines" ] && max_lines="${#lines_inprog[@]}"
  [ "${#lines_review[@]}"  -gt "$max_lines" ] && max_lines="${#lines_review[@]}"
  [ "${#lines_blocked[@]}" -gt "$max_lines" ] && max_lines="${#lines_blocked[@]}"

  # Ligne vide de remplissage (même largeur visuelle que les vraies lignes)
  local empty_filler
  empty_filler="$(printf '%*s' "$col_w" '')"

  local i
  for (( i=0; i<max_lines; i++ )); do
    local l0="${lines_open[$i]:-$empty_filler}"
    local l1="${lines_inprog[$i]:-$empty_filler}"
    local l2="${lines_review[$i]:-$empty_filler}"
    local l3="${lines_blocked[$i]:-$empty_filler}"

    printf '%b  %b  %b  %b\n' "$l0" "$l1" "$l2" "$l3"
  done

  # ── Footer : compteurs ──
  echo ""
  printf '%b  ' "$(_pad "${DIM}${cnt_open} open${RESET}"           "$col_w")"
  printf '%b  ' "$(_pad "${BLUE}${cnt_inprog} in progress${RESET}" "$col_w")"
  printf '%b  ' "$(_pad "${YELLOW}${cnt_review} review${RESET}"    "$col_w")"
  printf '%b'   "$(_pad "${RED}${cnt_blocked} blocked${RESET}"     "$col_w")"
  echo ""
  echo ""
}

# ── Point d'entrée ────────────────────────────────────────────────────────────
cmd_board() {
  local raw_id="${1:-}"
  local watch=false
  local interval=5

  # Parser les flags supplémentaires
  shift || true
  while [ $# -gt 0 ]; do
    case "$1" in
      --watch)             watch=true ;;
      --interval)          shift; interval="${1:-5}" ;;
      --interval=*)        interval="${1#*=}" ;;
      *) ;;
    esac
    shift
  done

  # Résoudre le projet
  local id path
  if [ -n "$raw_id" ]; then
    id=$(normalize_project_id "$raw_id")
    path=$(resolve_project_path "$id")
  else
    # Auto-découverte : cherche .beads/ dans le répertoire courant et ses parents
    local current_dir="$PWD"
    path=""
    id="."
    while [ "$current_dir" != "/" ]; do
      if [ -d "${current_dir}/.beads" ]; then
        path="$current_dir"
        id=$(basename "$current_dir" | tr '[:lower:]' '[:upper:]')
        break
      fi
      current_dir=$(dirname "$current_dir")
    done
    if [ -z "$path" ]; then
      log_error "$(t board.no_project)"
      log_info  "$(t board.usage_hint)"
      exit 1
    fi
  fi

  if [ "$watch" = true ]; then
    log_info "$(t board.watch_mode) (Ctrl+C $(t board.watch_quit))"
    while true; do
      clear
      _render_board "$id" "$path"
      sleep "$interval"
    done
  else
    _render_board "$id" "$path"
  fi
}
