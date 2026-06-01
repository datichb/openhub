#!/bin/bash
# Lance une session de debug sur un projet via l'agent debugger.
# Usage : oc debug [PROJECT_ID]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/prompt-builder.sh"

ensure_projects_file

# ── Parsing des arguments ─────────────────────────────────────────────────────
PROJECT_ID="${1:-}"

# ── Sélection interactive si pas d'ID ────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_error "$(t debug.no_projects)"
    exit 1
  fi

  echo -e "${BOLD}$(t debug.choose_project)${RESET}"
  echo ""
  for i in "${!ids[@]}"; do
    printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
  done
  echo ""
  read -rp "$(t debug.choose_number)" choice
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
    log_error "$(t debug.invalid_choice)$choice (1-${#ids[@]})"
    exit 1
  fi
  PROJECT_ID="${ids[$((choice-1))]}"
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Validation + résolution du chemin ────────────────────────────────────────
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Résolution de la cible ────────────────────────────────────────────────────
default_target="opencode"
load_adapter "$default_target"
adapter_validate || { log_error "$(t debug.target_unavailable)'${default_target}' $(t debug.target_unavailable_suffix)"; exit 1; }

# ── Agent requis ─────────────────────────────────────────────────────────────
REQUIRED_AGENT="debugger"

# ── Dossier d'agents déployés selon la cible ─────────────────────────────────
case "$default_target" in
  opencode)    agents_dir="$PROJECT_PATH/.opencode/agents" ;;
  *)           agents_dir="" ;;
esac

# ── Bloc d'intro TUI ─────────────────────────────────────────────────────────
_intro "oc debug  ${PROJECT_ID}"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t debug.label_path)"   "$PROJECT_PATH"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t debug.label_target)"  "$default_target"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t debug.label_agent)"   "$REQUIRED_AGENT"

# ── Vérifier l'agent dans projects.md ────────────────────────────────────────
agents_csv=$(get_project_agents "$PROJECT_ID")

if [ "$agents_csv" != "all" ]; then
  if ! echo ",$agents_csv," | grep -qF ",$REQUIRED_AGENT,"; then
    echo -e "${DIM}│${RESET}"
    log_warn "$(t debug.agent_missing_config)${REQUIRED_AGENT}"
    _prompt _add_agent "$(t debug.add_agent_prompt)"
    if [[ "${_add_agent:-Y}" =~ ^[Yy]$ ]]; then
      source "$LIB_DIR/agent-picker.sh"
      new_csv="${agents_csv},${REQUIRED_AGENT}"
      new_csv=$(echo "$new_csv" | sed 's/^,//;s/,$//')
      _set_project_agents "$PROJECT_ID" "$new_csv"
      log_success "$(t debug.agent_updated)$new_csv"

      echo -e "${DIM}│${RESET}"
      _prompt _redeploy "$(t debug.redeploy_prompt)"
      if [[ "${_redeploy:-Y}" =~ ^[Yy]$ ]]; then
        echo ""
        bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
        echo ""
      else
        log_info "$(t debug.redeploy_later)$default_target $PROJECT_ID"
      fi
    fi
  fi
fi

# ── Vérifier le déploiement physique de l'agent ───────────────────────────────
echo -e "${DIM}│${RESET}"

if [ -n "$agents_dir" ] && [ ! -d "$agents_dir" ]; then
  log_warn "$(t debug.agents_not_deployed)${default_target}"
  _prompt _deploy_now "$(t debug.deploy_now_prompt)"
  if [[ "${_deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
    echo ""
  else
    log_warn "$(t debug.deploy_skipped)"
    log_info  "$(t debug.deploy_later)$default_target $PROJECT_ID"
  fi
elif [ -n "$agents_dir" ] && [ -d "$agents_dir" ] && [ ! -f "$agents_dir/${REQUIRED_AGENT}.md" ]; then
  log_warn "$(t debug.agent_not_deployed)${REQUIRED_AGENT}"
  _prompt _deploy_missing "$(t debug.redeploy_prompt)"
  if [[ "${_deploy_missing:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
    echo ""
  else
    log_warn "$(t debug.deploy_skipped)"
  fi
fi

# ── Construire le prompt ──────────────────────────────────────────────────────
PROMPT=$(build_debug_bootstrap_prompt "$PROJECT_PATH" "$PROJECT_ID")

echo -e "${DIM}│${RESET}"
log_info "$(t debug.main_agent)${REQUIRED_AGENT}"

# ── Confirmation avant lancement ─────────────────────────────────────────────
_outro "$(t debug.launching)${default_target}…"
IFS= read -rp "" _

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "$REQUIRED_AGENT"
