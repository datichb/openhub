#!/bin/bash

# ─────────────────────────────────────────
# PATHS
# ─────────────────────────────────────────
HUB_DIR="${HUB_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
PROJECTS_FILE="${PROJECTS_FILE:-$HUB_DIR/projects/projects.md}"
PROJECTS_EXAMPLE_FILE="$HUB_DIR/projects/projects.example.md"
export PROJECTS_EXAMPLE_FILE
PATHS_FILE="${PATHS_FILE:-$HUB_DIR/projects/paths.local.md}"
API_KEYS_FILE="${API_KEYS_FILE:-$HUB_DIR/projects/api-keys.local.md}"
SKILLS_DIR="${SKILLS_DIR:-$HUB_DIR/skills}"
export SKILLS_DIR
SCRIPTS_DIR="${SCRIPTS_DIR:-$HUB_DIR/scripts}"
export SCRIPTS_DIR

# Phase 2+ : sources canoniques (agents/ et config/)
CANONICAL_AGENTS_DIR="${CANONICAL_AGENTS_DIR:-$HUB_DIR/agents}"
export CANONICAL_AGENTS_DIR
HUB_CONFIG="${HUB_CONFIG:-$HUB_DIR/config/hub.json}"
HUB_CONFIG_EXAMPLE="${HUB_CONFIG_EXAMPLE:-$HUB_DIR/config/hub.json.example}"
LIB_DIR="${LIB_DIR:-$HUB_DIR/scripts/lib}"
export LIB_DIR
ADAPTERS_DIR="${ADAPTERS_DIR:-$HUB_DIR/scripts/adapters}"
export ADAPTERS_DIR
EXTERNAL_SKILLS_DIR="${EXTERNAL_SKILLS_DIR:-$HUB_DIR/skills/external}"
export EXTERNAL_SKILLS_DIR

# Load i18n string table (bash 3.2 compatible)
# shellcheck source=scripts/lib/i18n.sh
[ -f "$HUB_DIR/scripts/lib/i18n.sh" ] && source "$HUB_DIR/scripts/lib/i18n.sh"

# Load colors, loggers and TUI helpers
# shellcheck source=scripts/lib/colors.sh
[ -f "$LIB_DIR/colors.sh" ] && source "$LIB_DIR/colors.sh"

# Load progress bar library
# shellcheck source=scripts/lib/progress-bar.sh
[ -f "$LIB_DIR/progress-bar.sh" ] && source "$LIB_DIR/progress-bar.sh"

# Load API keys INI parser
# shellcheck source=scripts/lib/api-keys.sh
[ -f "$LIB_DIR/api-keys.sh" ] && source "$LIB_DIR/api-keys.sh"

# Load providers resolution
# shellcheck source=scripts/lib/providers.sh
[ -f "$LIB_DIR/providers.sh" ] && source "$LIB_DIR/providers.sh"

# Load project registry (projects.md, paths.local.md, native agents)
# shellcheck source=scripts/lib/project.sh
[ -f "$LIB_DIR/project.sh" ] && source "$LIB_DIR/project.sh"

# Load portable file locking library
# shellcheck source=scripts/lib/filelock.sh
[ -f "$LIB_DIR/filelock.sh" ] && source "$LIB_DIR/filelock.sh"

# Load secrets/keychain abstraction layer
# shellcheck source=scripts/lib/secrets.sh
[ -f "$LIB_DIR/secrets.sh" ] && source "$LIB_DIR/secrets.sh"

# Noms de verrous canoniques (partagés entre tous les scripts)
OC_LOCK_PROJECTS="projects"
OC_LOCK_API_KEYS="api-keys"
OC_LOCK_HUB="hub"
export OC_LOCK_PROJECTS OC_LOCK_API_KEYS OC_LOCK_HUB

# ─────────────────────────────────────────
# DEFAULTS
# ─────────────────────────────────────────
DEFAULT_MODEL="claude-sonnet-4-5"
export DEFAULT_MODEL

# ─────────────────────────────────────────
# I18N — Language resolution
# ─────────────────────────────────────────

# Reads the global CLI language from hub.json (.cli.language)
# Returns "" if not set or jq unavailable
get_hub_language() {
  [ -f "$HUB_CONFIG" ] || return 0
  command -v jq >/dev/null 2>&1 || return 0
  jq -r '.cli.language // empty' "$HUB_CONFIG" 2>/dev/null || true
}

# Resolves and exports OC_LANG for the current invocation.
# Priority: project Langue field > global hub.json .cli.language > "en"
# Normalises french/English → fr/en.
# @param $1 — PROJECT_ID (optional)
# shellcheck disable=SC2120
resolve_oc_lang() {
  local project_id="${1:-}"
  local lang=""

  # 1. Per-project Langue field in projects.md
  if [ -n "$project_id" ]; then
    lang=$(get_project_language "$project_id")
  fi

  # 2. Global CLI language from hub.json
  if [ -z "$lang" ]; then
    lang=$(get_hub_language)
  fi

  # 3. Default to English
  lang="${lang:-en}"

  # Normalise: french → fr, english/anything else → en
  case "$lang" in
    french|fr) lang="fr" ;;
    *)         lang="en" ;;
  esac

  export OC_LANG="$lang"
}

# Resolves the human-readable language name to inject in agent prompts.
# Priority: per-project Langue field → OC_LANG code → empty (no injection)
# Maps language codes to human-readable names: fr → "français", en → "english"
# @param $1 — raw lang string from get_project_language (may be empty)
# Returns the human-readable name to pass to build_agent_content, or "" for none.
resolve_agent_lang() {
  local raw="${1:-}"
  if [ -n "$raw" ]; then
    # Per-project Langue field: return as-is (already human-readable)
    printf '%s' "$raw"
    return 0
  fi
  # Fall back to OC_LANG code → human-readable name
  local code="${OC_LANG:-}"
  case "$code" in
    fr) printf '%s' "français" ;;
    en) printf '%s' "english" ;;
    *)  printf '%s' "" ;;
  esac
}

# Auto-resolve language on source so t() works without explicit call
resolve_oc_lang

# ─────────────────────────────────────────
# HELP FORMATTING — shared helpers
# Single set of functions used by cmd-help.sh and all sub-command scripts.
# ─────────────────────────────────────────
OC_HELP_W=40   # width of the command column (shared by all help renderers)

# Print a section header: bold title + separator line.
# Usage: _h_section "Title text"
_h_section() {
  echo ""
  echo -e "${BOLD}$1${RESET}"
  printf '%.0s─' $(seq 1 52); echo
}

# Print one command row: cyan command column + description.
# Usage: _h_cmd "command [args]" "Description text"
_h_cmd() {
  local cmd="$1" desc="$2"
  if [ "${#cmd}" -le "$OC_HELP_W" ]; then
    printf "  ${CYAN}%-${OC_HELP_W}s${RESET}  %s\n" "$cmd" "$desc"
  else
    printf "  ${CYAN}%s${RESET}\n" "$cmd"
    printf "  %*s  %s\n" "$OC_HELP_W" "" "$desc"
  fi
}

# Print a flag/variant row indented under the parent command.
# Usage: _h_sub "--flag [val]" "Description text"
_h_sub() {
  local flag="$1" desc="$2"
  printf "  %2s${DIM}%-$((OC_HELP_W - 2))s${RESET}  %s\n" "" "$flag" "$desc"
}

# Print a plain indented note (examples, free text).
# Usage: _h_note "oc start"
_h_note() {
  echo "  $1"
}

