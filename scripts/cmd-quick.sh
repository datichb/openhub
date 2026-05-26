#!/bin/bash
# cmd-quick.sh — Quick task execution with auto-detected agent
#
# Usage: oc quick <PROJECT_ID> "<prompt>"
#
# Launches a quick task with:
#   - Auto-detected agent based on prompt keywords
#   - Semi-auto mode (default)
#   - Skipped CP-0 (no planning phase)
#   - CP-2 remains a pause (commit or correct)

set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
resolve_oc_lang

# ── Help ──────────────────────────────────────────────────────────────────────
show_help() {
  echo ""
  echo "  $(t quick.title)"
  echo ""
  echo "  $(t quick.usage)"
  echo ""
  echo "  $(t quick.help.options)"
  echo "    --help, -h    $(t quick.help.desc)"
  echo ""
  echo "  $(t quick.help.examples)"
  echo "    oc quick MY-APP \"Ajoute un bouton de connexion\""
  echo "    oc quick MY-APP \"Crée un endpoint /api/users\""
  echo "    oc quick MY-APP \"Configure le pipeline CI\""
  echo ""
}

# ── Argument parsing ──────────────────────────────────────────────────────────
PROJECT_ID=""
PROMPT=""

for arg in "$@"; do
  case "$arg" in
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      if [ -z "$PROJECT_ID" ]; then
        PROJECT_ID="$arg"
      elif [ -z "$PROMPT" ]; then
        PROMPT="$arg"
      fi
      ;;
  esac
done

# ── Validation ────────────────────────────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  log_error "$(t quick.project_required)"
  show_help
  exit 1
fi

if [ -z "$PROMPT" ]; then
  log_error "$(t quick.prompt_required)"
  show_help
  exit 1
fi

ensure_projects_file
PROJECT_ID=$(normalize_project_id "$PROJECT_ID")
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Agent detection from prompt keywords ──────────────────────────────────────
detect_agent_from_prompt() {
  local prompt="$1"
  local prompt_lower
  prompt_lower=$(echo "$prompt" | tr '[:upper:]' '[:lower:]')

  # Frontend signals
  if echo "$prompt_lower" | grep -qE "(composant|bouton|ui|css|frontend|vue|react|component|button|style|form|modal|dialog|page|layout)"; then
    echo "developer-frontend"
    return 0
  fi

  # Backend signals
  if echo "$prompt_lower" | grep -qE "(endpoint|api|route|controller|service|handler|middleware|auth|token|session)"; then
    echo "developer-backend"
    return 0
  fi

  # Database signals (also backend)
  if echo "$prompt_lower" | grep -qE "(migration|schema|base|database|db|model|entity|table|query|sql)"; then
    echo "developer-backend"
    return 0
  fi

  # DevOps signals
  if echo "$prompt_lower" | grep -qE "(docker|ci|deploy|pipeline|dockerfile|compose|github[.]action|gitlab|helm|k8s|kubernetes|infra)"; then
    echo "developer-devops"
    return 0
  fi

  # Default: fullstack
  echo "developer-fullstack"
}

DETECTED_AGENT=$(detect_agent_from_prompt "$PROMPT")

# ── Display context ───────────────────────────────────────────────────────────
_intro "${PROJECT_ID}"
printf "${DIM}│${RESET}  %-10s %s\n" "Agent" "$DETECTED_AGENT"
printf "${DIM}│${RESET}  %-10s %s\n" "Mode"  "semi-auto"
echo -e "${DIM}│${RESET}"

# ── Load adapter ──────────────────────────────────────────────────────────────
source "$LIB_DIR/adapter-manager.sh"
load_adapter "opencode"
adapter_validate || { log_error "$(t start.target_unavailable)"; exit 1; }

# ── Check agents deployment ───────────────────────────────────────────────────
agents_dir="$PROJECT_PATH/.opencode/agents"
if [ ! -d "$agents_dir" ]; then
  log_warn "$(t start.agents_not_deployed) opencode"
  _prompt _deploy_now "$(t start.deploy_now)"
  if [[ "${_deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
    echo ""
  else
    log_info "$(t deploy_later)"
  fi
fi

# ── Build the quick prompt with mode and skip-CP0 instructions ────────────────
# The prompt includes instructions to skip CP-0 and use semi-auto mode
QUICK_PROMPT="$(t quick.prompt.mode)
$(t quick.prompt.skip_cp0)
$(t quick.prompt.cp2_pause)

$(t quick.prompt.task)
$PROMPT"

# ── Launch ────────────────────────────────────────────────────────────────────
log_info "$(t quick.launching)"
echo ""

adapter_start "$PROJECT_PATH" "$QUICK_PROMPT" "$PROJECT_ID" "$DETECTED_AGENT" ""
