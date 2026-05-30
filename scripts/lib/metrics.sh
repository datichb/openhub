#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/metrics.sh — Logging des métriques de vélocité workflow
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/metrics.sh"
#   metrics_ticket_start "bd-42" "developer-backend"
#   metrics_ticket_complete "bd-42" "developer-backend" 900
#   metrics_review_cycle "bd-42" 1
#   metrics_correction "bd-42" "lint errors"
#
# Les événements sont loggés dans .opencode/metrics.jsonl (format JSONL).
# Le fichier est créé au premier événement si inexistant.
# Compatible bash 3.2 (macOS).
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_METRICS_LOADED:-}" ] && return 0
_METRICS_LOADED=1

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
_METRICS_DIR=".opencode"
_METRICS_FILE="${_METRICS_DIR}/metrics.jsonl"

# ─────────────────────────────────────────
# HELPERS
# ─────────────────────────────────────────

# Génère un timestamp ISO8601 UTC
# Usage : ts=$(_metrics_timestamp)
_metrics_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Échappe une chaîne pour JSON (backslashes, guillemets, newlines, tabs)
# Usage : escaped=$(_metrics_escape "valeur")
# Note : utilise printf %s et substitution bash — compatible bash 3.2 (macOS)
_metrics_escape() {
  local s
  # printf %s évite l'interprétation de \n, \t, etc. dans l'argument
  s=$(printf '%s' "$1")
  # Ordre important : d'abord les backslashes, puis les autres caractères
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  # shellcheck disable=SC2016
  s=${s//$'\n'/'\n'}
  # shellcheck disable=SC2016
  s=${s//$'\t'/'\t'}
  printf '%s' "$s"
}

# S'assure que le dossier et fichier metrics existent
# Usage : _metrics_ensure_file
_metrics_ensure_file() {
  if [ ! -d "$_METRICS_DIR" ]; then
    mkdir -p "$_METRICS_DIR"
  fi
  if [ ! -f "$_METRICS_FILE" ]; then
    touch "$_METRICS_FILE"
  fi
}

# ─────────────────────────────────────────
# CORE — Logging générique
# ─────────────────────────────────────────

# Log un événement générique dans metrics.jsonl
# Usage : metrics_log_event "event_type" "ticket_id" ["agent"] ["duration_seconds"] ["extra_json_fields"]
# @param $1 — event type (required)
# @param $2 — ticket_id (required)
# @param $3 — agent (optional, empty string to skip)
# @param $4 — duration_seconds (optional, empty string to skip)
# @param $5 — extra JSON fields as raw JSON object content (optional, without braces)
#             Example: '"reason":"lint errors","count":3'
metrics_log_event() {
  local event_type="$1"
  local ticket_id="$2"
  local agent="${3:-}"
  local duration="${4:-}"
  local extra="${5:-}"

  [ -z "$event_type" ] && return 1
  [ -z "$ticket_id" ] && return 1

  _metrics_ensure_file

  local ts
  ts=$(_metrics_timestamp)

  # Build JSON line
  local json
  json="{\"timestamp\":\"${ts}\",\"event\":\"$(_metrics_escape "$event_type")\",\"ticket_id\":\"$(_metrics_escape "$ticket_id")\""

  if [ -n "$agent" ]; then
    json="${json},\"agent\":\"$(_metrics_escape "$agent")\""
  fi

  if [ -n "$duration" ]; then
    if [[ "$duration" =~ ^[0-9]+$ ]]; then
      json="${json},\"duration_seconds\":${duration}"
    fi
  fi

  if [ -n "$extra" ]; then
    json="${json},${extra}"
  fi

  json="${json}}"

  # Append to file
  echo "$json" >> "$_METRICS_FILE"
}

# ─────────────────────────────────────────
# TIMER FUNCTIONS
# ─────────────────────────────────────────

# Répertoire temporaire pour stocker les timestamps de démarrage
_METRICS_TIMER_DIR="${TMPDIR:-/tmp}/opencode-metrics-timers"

# Démarre un timer pour un ticket
# Usage : metrics_start_timer "bd-42"
# @param $1 — ticket_id (required)
# Stocke l'epoch timestamp dans un fichier temporaire
metrics_start_timer() {
  local ticket_id="$1"
  [ -z "$ticket_id" ] && return 1

  # Créer le répertoire des timers s'il n'existe pas
  if [ ! -d "$_METRICS_TIMER_DIR" ]; then
    mkdir -p "$_METRICS_TIMER_DIR"
  fi

  # Stocker l'epoch timestamp
  local timer_file
  timer_file="${_METRICS_TIMER_DIR}/${ticket_id}.timer"
  date +%s > "$timer_file"
}

# Calcule la durée en secondes depuis le start d'un ticket
# Usage : duration=$(metrics_get_duration "bd-42")
# @param $1 — ticket_id (required)
# Retourne : durée en secondes, ou chaîne vide si pas de timer
metrics_get_duration() {
  local ticket_id="$1"
  [ -z "$ticket_id" ] && return 1

  local timer_file
  timer_file="${_METRICS_TIMER_DIR}/${ticket_id}.timer"
  if [ ! -f "$timer_file" ]; then
    echo ""
    return 1
  fi

  local start_time
  start_time=$(cat "$timer_file")
  local now
  now=$(date +%s)

  local duration
  duration=$((now - start_time))
  echo "$duration"
}

# Nettoie le timer d'un ticket (optionnel, appelé après ticket_complete)
# Usage : metrics_clear_timer "bd-42"
# @param $1 — ticket_id (required)
metrics_clear_timer() {
  local ticket_id="$1"
  [ -z "$ticket_id" ] && return 1

  local timer_file
  timer_file="${_METRICS_TIMER_DIR}/${ticket_id}.timer"
  if [ -f "$timer_file" ]; then
    rm -f "$timer_file"
  fi
}

# ─────────────────────────────────────────
# EVENT-SPECIFIC FUNCTIONS
# ─────────────────────────────────────────

# Log le démarrage d'un ticket
# Usage : metrics_ticket_start "bd-42" ["developer-backend"]
# @param $1 — ticket_id (required)
# @param $2 — agent (optional)
metrics_ticket_start() {
  local ticket_id="$1"
  local agent="${2:-}"
  metrics_log_event "ticket_start" "$ticket_id" "$agent"
}

# Log la complétion d'un ticket avec durée
# Usage : metrics_ticket_complete "bd-42" ["developer-backend"] [900]
# @param $1 — ticket_id (required)
# @param $2 — agent (optional)
# @param $3 — duration_seconds (optional)
metrics_ticket_complete() {
  local ticket_id="$1"
  local agent="${2:-}"
  local duration="${3:-}"
  metrics_log_event "ticket_complete" "$ticket_id" "$agent" "$duration"
}

# Log un cycle de review
# Usage : metrics_review_cycle "bd-42" [1]
# @param $1 — ticket_id (required)
# @param $2 — cycle_number (optional)
metrics_review_cycle() {
  local ticket_id="$1"
  local cycle="${2:-}"
  local extra=""
  if [ -n "$cycle" ]; then
    if [[ "$cycle" =~ ^[0-9]+$ ]]; then
      extra="\"cycle\":${cycle}"
    fi
  fi
  metrics_log_event "review_cycle" "$ticket_id" "" "" "$extra"
}

# Log une correction avec raison
# Usage : metrics_correction "bd-42" "lint errors"
# @param $1 — ticket_id (required)
# @param $2 — reason (optional)
metrics_correction() {
  local ticket_id="$1"
  local reason="${2:-}"
  local extra=""
  if [ -n "$reason" ]; then
    extra="\"reason\":\"$(_metrics_escape "$reason")\""
  fi
  metrics_log_event "correction" "$ticket_id" "" "" "$extra"
}

# Log un événement WebSearch
# Usage : metrics_websearch "bd-42" "websearch" ["CVE lookup"]
# @param $1 — ticket_id (required)
# @param $2 — tool (websearch ou webfetch)
# @param $3 — query_type (optional)
metrics_websearch() {
  local ticket_id="$1"
  local tool="${2:-websearch}"
  local query_type="${3:-}"
  local extra=""
  if [ -n "$query_type" ]; then
    extra="\"query_type\":\"$(_metrics_escape "$query_type")\",\"tool\":\"$tool\""
  else
    extra="\"tool\":\"$tool\""
  fi
  metrics_log_event "websearch" "$ticket_id" "" "" "$extra"
}

# ─────────────────────────────────────────
# AGGREGATION FUNCTIONS
# ─────────────────────────────────────────

# Vérifie si le fichier metrics existe et contient des données
# Usage : if metrics_file_exists; then ... fi
# Returns : 0 si le fichier existe et n'est pas vide, 1 sinon
metrics_file_exists() {
  [ -f "$_METRICS_FILE" ] && [ -s "$_METRICS_FILE" ]
}

# Retourne le chemin du fichier metrics
# Usage : path=$(metrics_get_file_path)
metrics_get_file_path() {
  printf '%s' "$_METRICS_FILE"
}

# Compte le nombre de tickets complétés
# Usage : count=$(metrics_count_completed)
# Returns : nombre de tickets (0 si aucun ou fichier absent)
metrics_count_completed() {
  if ! metrics_file_exists; then
    echo "0"
    return 0
  fi
  local count
  count=$(grep -c '"event":"ticket_complete"' "$_METRICS_FILE" 2>/dev/null) || count=0
  # Nettoyer les espaces éventuels
  count=$(echo "$count" | tr -d '[:space:]')
  echo "${count:-0}"
}

# Calcule la durée moyenne par ticket (en secondes)
# Usage : avg=$(metrics_avg_duration)
# Returns : durée moyenne en secondes (0 si pas de données)
metrics_avg_duration() {
  if ! metrics_file_exists; then
    echo "0"
    return 0
  fi

  local total=0
  local count=0

  # Extraire toutes les durées des événements ticket_complete
  while IFS= read -r duration; do
    if [ -n "$duration" ] && [[ "$duration" =~ ^[0-9]+$ ]]; then
      total=$((total + duration))
      count=$((count + 1))
    fi
  done < <(grep '"event":"ticket_complete"' "$_METRICS_FILE" 2>/dev/null | \
           sed -n 's/.*"duration_seconds":\([0-9]*\).*/\1/p')

  if [ "$count" -eq 0 ]; then
    echo "0"
    return 0
  fi

  echo $((total / count))
}

# Calcule le nombre moyen de cycles de review par ticket
# Usage : avg=$(metrics_avg_review_cycles)
# Returns : moyenne (format X.X) ou "0" si pas de données
metrics_avg_review_cycles() {
  if ! metrics_file_exists; then
    echo "0"
    return 0
  fi

  # Compter les événements review_cycle
  local review_count
  review_count=$(grep -c '"event":"review_cycle"' "$_METRICS_FILE" 2>/dev/null) || review_count=0
  # Nettoyer les espaces éventuels
  review_count=$(echo "$review_count" | tr -d '[:space:]')
  review_count="${review_count:-0}"

  # Compter les tickets complétés (pour la moyenne)
  local ticket_count
  ticket_count=$(metrics_count_completed)
  ticket_count="${ticket_count:-0}"

  if [ "$ticket_count" -eq 0 ]; then
    echo "0"
    return 0
  fi

  # Calcul avec une décimale (bash integer arithmetic)
  # Multiplier par 10 pour avoir une décimale
  local avg_x10
  avg_x10=$(( (review_count * 10) / ticket_count ))

  # Formater X.X
  local integer_part=$((avg_x10 / 10))
  local decimal_part=$((avg_x10 % 10))
  echo "${integer_part}.${decimal_part}"
}

# Retourne le top N des raisons de correction
# Usage : metrics_top_corrections [N]
# @param $1 — nombre de raisons à retourner (défaut: 3)
# Returns : liste "reason|count" par ligne, triée par count décroissant
metrics_top_corrections() {
  local top_n="${1:-3}"

  if ! metrics_file_exists; then
    return 0
  fi

  # Extraire les raisons, compter les occurrences, trier, prendre le top N
  grep '"event":"correction"' "$_METRICS_FILE" 2>/dev/null | \
    sed -n 's/.*"reason":"\([^"]*\)".*/\1/p' | \
    sort | uniq -c | sort -rn | head -n "$top_n" | \
    while read -r count reason; do
      printf '%s|%s\n' "$reason" "$count"
    done
}

# Formate une durée en secondes en format lisible (Xh Xm Xs)
# Usage : formatted=$(metrics_format_duration 3665)
# @param $1 — durée en secondes
# Returns : chaîne formatée (ex: "1h 1m 5s", "15m 30s", "45s")
metrics_format_duration() {
  local seconds="$1"
  [ -z "$seconds" ] && seconds=0

  local hours=$((seconds / 3600))
  local minutes=$(( (seconds % 3600) / 60 ))
  local secs=$((seconds % 60))

  local result=""
  if [ "$hours" -gt 0 ]; then
    result="${hours}h "
  fi
  if [ "$minutes" -gt 0 ] || [ "$hours" -gt 0 ]; then
    result="${result}${minutes}m "
  fi
  result="${result}${secs}s"

  printf '%s' "$result"
}

# Compte le nombre de WebSearch calls
# Usage : count=$(metrics_count_websearch)
metrics_count_websearch() {
  if ! metrics_file_exists; then
    echo "0"
    return 0
  fi
  local count
  count=$(grep -c '"event":"websearch"' "$_METRICS_FILE" 2>/dev/null) || count=0
  count=$(echo "$count" | tr -d '[:space:]')
  echo "${count:-0}"
}

# Retourne le top N des types de queries WebSearch
# Usage : metrics_top_websearch_types [N]
metrics_top_websearch_types() {
  local top_n="${1:-3}"
  
  if ! metrics_file_exists; then
    return 0
  fi
  
  grep '"event":"websearch"' "$_METRICS_FILE" 2>/dev/null | \
    sed -n 's/.*"query_type":"\([^"]*\)".*/\1/p' | \
    sort | uniq -c | sort -rn | head -n "$top_n" | \
    while read -r count type; do
      printf '%s|%s\n' "$type" "$count"
    done
}

# Agrège toutes les métriques en un seul appel
# Usage : metrics_aggregate
# Returns : données formatées pour affichage (utilise des variables globales)
# Variables set:
#   METRICS_TOTAL_TICKETS — nombre de tickets complétés
#   METRICS_AVG_DURATION — durée moyenne en secondes
#   METRICS_AVG_DURATION_FMT — durée moyenne formatée
#   METRICS_AVG_CYCLES — cycles review moyens
#   METRICS_TOP_CORRECTIONS — tableau des top corrections (reason|count)
#   METRICS_WEBSEARCH_COUNT — nombre de WebSearch calls
#   METRICS_TOP_WEBSEARCH_TYPES — tableau des top query types (type|count)
metrics_aggregate() {
  METRICS_TOTAL_TICKETS=$(metrics_count_completed)
  export METRICS_TOTAL_TICKETS
  METRICS_AVG_DURATION=$(metrics_avg_duration)
  export METRICS_AVG_DURATION
  METRICS_AVG_DURATION_FMT=$(metrics_format_duration "$METRICS_AVG_DURATION")
  export METRICS_AVG_DURATION_FMT
  METRICS_AVG_CYCLES=$(metrics_avg_review_cycles)
  export METRICS_AVG_CYCLES

  # Top corrections dans un tableau
  METRICS_TOP_CORRECTIONS=()
  while IFS= read -r line; do
    [ -n "$line" ] && METRICS_TOP_CORRECTIONS+=("$line")
  done < <(metrics_top_corrections 3)
  
  # WebSearch metrics
  METRICS_WEBSEARCH_COUNT=$(metrics_count_websearch)
  export METRICS_WEBSEARCH_COUNT
  METRICS_TOP_WEBSEARCH_TYPES=()
  while IFS= read -r line; do
    [ -n "$line" ] && METRICS_TOP_WEBSEARCH_TYPES+=("$line")
  done < <(metrics_top_websearch_types 3)
}
