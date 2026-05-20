#!/bin/bash
# Lance une code review sur une branche via l'agent reviewer.
# Usage : oc review [PROJECT_ID] [--branch <branch>]
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/prompt-builder.sh"

ensure_projects_file

# ── Parsing des arguments ─────────────────────────────────────────────────────
BRANCH=""
ARGS=()
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --branch) BRANCH="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --branch) _prev="$arg" ;;
    *)        ARGS+=("$arg") ;;
  esac
done
PROJECT_ID="${ARGS[0]:-}"

# ── Sélection interactive si pas d'ID ────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_error "$(t review.no_projects)"
    exit 1
  fi

  echo -e "${BOLD}$(t review.choose_project)${RESET}"
  echo ""
  for i in "${!ids[@]}"; do
    printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
  done
  echo ""
  read -rp "$(t review.choose_number)" choice
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
    log_error "$(t review.invalid_choice)$choice (attendu 1-${#ids[@]})"
    exit 1
  fi
  PROJECT_ID="${ids[$((choice-1))]}"
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Validation + résolution du chemin ────────────────────────────────────────
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Résolution de la cible ────────────────────────────────────────────────────
default_target=$(get_default_target)
load_adapter "$default_target"
adapter_validate || { log_error "$(t review.target_unavailable)'${default_target}' $(t review.target_unavailable_suffix)"; exit 1; }

# ── Résolution de la branche ──────────────────────────────────────────────────
if [ -z "$BRANCH" ]; then
  BRANCH=$(git -C "$PROJECT_PATH" branch --show-current 2>/dev/null || true)
  if [ -z "$BRANCH" ]; then
    log_error "$(t review.no_branch)"
    exit 1
  fi
fi

# ── Agent requis ─────────────────────────────────────────────────────────────
REQUIRED_AGENT="reviewer"

# ── Dossier d'agents déployés selon la cible ─────────────────────────────────
case "$default_target" in
  opencode)    agents_dir="$PROJECT_PATH/.opencode/agents" ;;
  *)           agents_dir="" ;;
esac

# ── Bloc d'intro TUI ─────────────────────────────────────────────────────────
_intro "oc review  ${PROJECT_ID}"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t review.label_path)"   "$PROJECT_PATH"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t review.label_target)"  "$default_target"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t review.label_branch)"  "$BRANCH"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t review.label_agent)"   "$REQUIRED_AGENT"

# ── Vérifier l'agent dans projects.md ────────────────────────────────────────
agents_csv=$(get_project_agents "$PROJECT_ID")

if [ "$agents_csv" != "all" ]; then
  if ! echo ",$agents_csv," | grep -qF ",$REQUIRED_AGENT,"; then
    echo -e "${DIM}│${RESET}"
    log_warn "$(t review.agent_missing_config)${REQUIRED_AGENT}"
    _prompt _add_agent "$(t review.add_agent_prompt)"
    if [[ "${_add_agent:-Y}" =~ ^[Yy]$ ]]; then
      source "$LIB_DIR/agent-picker.sh"
      new_csv="${agents_csv},${REQUIRED_AGENT}"
      new_csv=$(echo "$new_csv" | sed 's/^,//;s/,$//')
      _set_project_agents "$PROJECT_ID" "$new_csv"
      log_success "$(t review.agent_updated)$new_csv"

      echo -e "${DIM}│${RESET}"
      _prompt _redeploy "$(t review.redeploy_prompt)"
      if [[ "${_redeploy:-Y}" =~ ^[Yy]$ ]]; then
        echo ""
        bash "$SCRIPTS_DIR/cmd-deploy.sh" "$default_target" "$PROJECT_ID"
        echo ""
      else
        log_info "$(t review.redeploy_later)$default_target $PROJECT_ID"
      fi
    fi
  fi
fi

# ── Vérifier le déploiement physique de l'agent ───────────────────────────────
echo -e "${DIM}│${RESET}"

if [ -n "$agents_dir" ] && [ ! -d "$agents_dir" ]; then
  log_warn "$(t review.agents_not_deployed)${default_target}"
  _prompt _deploy_now "$(t review.deploy_now_prompt)"
  if [[ "${_deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$default_target" "$PROJECT_ID"
    echo ""
  else
    log_warn "$(t review.deploy_skipped)"
    log_info  "$(t review.deploy_later)$default_target $PROJECT_ID"
  fi
elif [ -n "$agents_dir" ] && [ -d "$agents_dir" ] && [ ! -f "$agents_dir/${REQUIRED_AGENT}.md" ]; then
  log_warn "$(t review.agent_not_deployed)${REQUIRED_AGENT}"
  _prompt _deploy_missing "$(t review.redeploy_prompt)"
  if [[ "${_deploy_missing:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$default_target" "$PROJECT_ID"
    echo ""
  else
    log_warn "$(t review.deploy_skipped)"
  fi
fi

# ── Construire le prompt ──────────────────────────────────────────────────────
PROMPT=$(build_review_bootstrap_prompt "$PROJECT_PATH" "$PROJECT_ID" "$BRANCH")

echo -e "${DIM}│${RESET}"
log_info "$(t review.main_agent)${REQUIRED_AGENT}"

# ── Confirmation avant lancement ─────────────────────────────────────────────
_outro "$(t review.launching)${default_target}…"
IFS= read -rp "" _

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "$REQUIRED_AGENT"
