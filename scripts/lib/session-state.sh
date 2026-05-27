#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/session-state.sh — Gestion de l'état de session pour le dashboard TUI
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/session-state.sh"
#   session_state_init "ses_abc123" "semi-auto"
#   session_state_add_ticket "bd-42" "Fix null guard"
#   session_state_update_ticket "bd-42" "in_progress"
#   session_state_set_current "bd-42" "developer-backend" "implementing"
#   session_state_clear_current
#   session_state_end
#
# L'état est stocké dans .opencode/session-state.json
# Format détaillé dans skills/orchestrator/session-state-protocol.md
# Compatible bash 3.2 (macOS).
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_SESSION_STATE_LOADED:-}" ] && return 0
_SESSION_STATE_LOADED=1

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
readonly _SESSION_STATE_DIR=".opencode"
readonly _SESSION_STATE_FILE="${_SESSION_STATE_DIR}/session-state.json"

# ─────────────────────────────────────────
# HELPERS
# ─────────────────────────────────────────

# Génère un timestamp ISO8601 UTC
# Usage : ts=$(_session_timestamp)
_session_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Échappe une chaîne pour JSON (backslashes, guillemets, newlines, tabs)
# Usage : escaped=$(_session_escape "valeur")
_session_escape() {
  local s
  s=$(printf '%s' "$1")
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  # shellcheck disable=SC2016
  s=${s//$'\n'/'\n'}
  # shellcheck disable=SC2016
  s=${s//$'\t'/'\t'}
  printf '%s' "$s"
}

# S'assure que le dossier .opencode existe
# Usage : _session_ensure_dir
_session_ensure_dir() {
  if [ ! -d "$_SESSION_STATE_DIR" ]; then
    mkdir -p "$_SESSION_STATE_DIR"
  fi
}

# Écrit l'état de manière atomique (tmp + rename)
# Usage : _session_write_state "$json"
_session_write_state() {
  local json="$1"
  _session_ensure_dir
  local tmp_file="${_SESSION_STATE_FILE}.tmp.$$"
  printf '%s\n' "$json" > "$tmp_file"
  mv "$tmp_file" "$_SESSION_STATE_FILE"
}

# ─────────────────────────────────────────
# CORE FUNCTIONS
# ─────────────────────────────────────────

# Initialise l'état de session
# Usage : session_state_init "ses_abc123" "semi-auto"
# @param $1 — session_id (required)
# @param $2 — mode : "manuel", "semi-auto", "auto" (required)
session_state_init() {
  local session_id="$1"
  local mode="$2"

  [ -z "$session_id" ] && return 1
  [ -z "$mode" ] && return 1

  local ts
  ts=$(_session_timestamp)

  local json
  json=$(cat <<EOF
{
  "session_id": "$(_session_escape "$session_id")",
  "started_at": "${ts}",
  "mode": "$(_session_escape "$mode")",
  "current_ticket": null,
  "tickets": [],
  "last_update": "${ts}"
}
EOF
)

  _session_write_state "$json"
}

# Ajoute un ticket à la session
# Usage : session_state_add_ticket "bd-42" "Fix null guard"
# @param $1 — ticket_id (required)
# @param $2 — title (required)
session_state_add_ticket() {
  local ticket_id="$1"
  local title="$2"

  [ -z "$ticket_id" ] && return 1
  [ -z "$title" ] && return 1

  # Vérifier que le fichier existe
  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  # Vérifier que jq est disponible
  if ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  local ts
  ts=$(_session_timestamp)

  local new_ticket
  new_ticket=$(jq -n \
    --arg id "$ticket_id" \
    --arg title "$title" \
    '{id: $id, status: "pending", title: $title}')

  local updated
  updated=$(jq \
    --argjson ticket "$new_ticket" \
    --arg ts "$ts" \
    '.tickets += [$ticket] | .last_update = $ts' \
    "$_SESSION_STATE_FILE")

  _session_write_state "$updated"
}

# Met à jour le statut d'un ticket
# Usage : session_state_update_ticket "bd-42" "in_progress"
# @param $1 — ticket_id (required)
# @param $2 — status : "pending", "in_progress", "review", "completed", "blocked" (required)
session_state_update_ticket() {
  local ticket_id="$1"
  local new_status="$2"

  [ -z "$ticket_id" ] && return 1
  [ -z "$new_status" ] && return 1

  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  if ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  local ts
  ts=$(_session_timestamp)

  local updated
  updated=$(jq \
    --arg id "$ticket_id" \
    --arg new_status "$new_status" \
    --arg ts "$ts" \
    '(.tickets[] | select(.id == $id)).status = $new_status | .last_update = $ts' \
    "$_SESSION_STATE_FILE")

  _session_write_state "$updated"
}

# Définit le ticket en cours avec agent et action
# Usage : session_state_set_current "bd-42" "developer-backend" "implementing"
# @param $1 — ticket_id (required)
# @param $2 — agent (required)
# @param $3 — action : "implementing", "testing", "reviewing", "waiting_cp2", "idle" (required)
session_state_set_current() {
  local ticket_id="$1"
  local agent="$2"
  local action="$3"

  [ -z "$ticket_id" ] && return 1
  [ -z "$agent" ] && return 1
  [ -z "$action" ] && return 1

  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  if ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  local ts
  ts=$(_session_timestamp)

  # Récupérer le titre et le statut du ticket en une seule requête jq
  local ticket_info
  ticket_info=$(jq -r --arg id "$ticket_id" \
    '.tickets[] | select(.id == $id) | "\(.title // "")\t\(.status // "in_progress")"' \
    "$_SESSION_STATE_FILE")
  local ticket_title="${ticket_info%%	*}"
  local ticket_status="${ticket_info##*	}"

  local current_ticket
  current_ticket=$(jq -n \
    --arg id "$ticket_id" \
    --arg title "$ticket_title" \
    --arg ticket_status "$ticket_status" \
    --arg agent "$agent" \
    --arg action "$action" \
    '{id: $id, title: $title, status: $ticket_status, agent: $agent, action: $action}')

  local updated
  updated=$(jq \
    --argjson current "$current_ticket" \
    --arg ts "$ts" \
    '.current_ticket = $current | .last_update = $ts' \
    "$_SESSION_STATE_FILE")

  _session_write_state "$updated"
}

# Efface le ticket en cours (entre deux tickets)
# Usage : session_state_clear_current
session_state_clear_current() {
  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  if ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  local ts
  ts=$(_session_timestamp)

  local updated
  updated=$(jq \
    --arg ts "$ts" \
    '.current_ticket = null | .last_update = $ts' \
    "$_SESSION_STATE_FILE")

  _session_write_state "$updated"
}

# Termine la session (supprime le fichier d'état)
# Usage : session_state_end
session_state_end() {
  if [ -f "$_SESSION_STATE_FILE" ]; then
    rm -f "$_SESSION_STATE_FILE"
  fi
}

# Lit l'état de session (retourne JSON ou chaîne vide si pas de session)
# Usage : state=$(session_state_read)
session_state_read() {
  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    echo ""
    return 0
  fi

  if [ ! -s "$_SESSION_STATE_FILE" ]; then
    echo ""
    return 0
  fi

  cat "$_SESSION_STATE_FILE"
}

# Vérifie si une session est active
# Usage : if session_state_is_active; then ... fi
# Returns : 0 si session active, 1 sinon
session_state_is_active() {
  if [ ! -f "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  if [ ! -s "$_SESSION_STATE_FILE" ]; then
    return 1
  fi

  # Vérifier que le JSON contient un session_id valide
  if command -v jq >/dev/null 2>&1; then
    local session_id
    session_id=$(jq -r '.session_id // ""' "$_SESSION_STATE_FILE" 2>/dev/null)
    if [ -n "$session_id" ] && [ "$session_id" != "null" ]; then
      return 0
    fi
    return 1
  fi

  # Sans jq, on se base juste sur l'existence du fichier
  return 0
}
