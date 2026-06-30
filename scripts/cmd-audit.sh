#!/bin/bash
# Lance un audit IA sur un projet via l'agent auditor.
# Le type d'audit (--type) est transmis comme paramètre de prompt au coordinateur
# qui délègue à l'agent auditor-subagent avec le domaine et le skill appropriés.
# Usage : oc audit [PROJECT_ID] [--type <type>]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/prompt-builder.sh"

ensure_projects_file

# ── Types d'audit supportés ───────────────────────────────────────────────────
VALID_AUDIT_TYPES="security accessibility architecture ecodesign observability performance privacy"

# ── Parsing des arguments ─────────────────────────────────────────────────────
AUDIT_TYPE=""
PROJECT_ID=""
_prev=""
for arg in "$@"; do
  case "$_prev" in
    --project|-p) PROJECT_ID="$arg"; _prev=""; continue ;;
    --type|-t)    AUDIT_TYPE="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --project|-p) _prev="$arg" ;;
    --type|-t)    _prev="$arg" ;;
  esac
done

# ── Validation --type ─────────────────────────────────────────────────────────
if [ -n "$AUDIT_TYPE" ]; then
  valid=false
  for t in $VALID_AUDIT_TYPES; do
    [ "$AUDIT_TYPE" = "$t" ] && valid=true && break
  done
  if [ "$valid" = false ]; then
    log_error "$(t audit.invalid_type)$AUDIT_TYPE'"
    log_info  "$(t audit.valid_types)$VALID_AUDIT_TYPES"
    exit 1
  fi
fi

# ── Sélection interactive si pas d'ID ────────────────────────────────────────
if [ -z "$PROJECT_ID" ]; then
  ids=()
  while IFS= read -r line; do ids+=("$line"); done < <(grep "^## " "$PROJECTS_FILE" | sed 's/^## //')

  if [ ${#ids[@]} -eq 0 ]; then
    log_error "$(t audit.no_projects)"
    exit 1
  fi

  echo -e "${BOLD}$(t audit.choose_project)${RESET}"
  echo ""
  for i in "${!ids[@]}"; do
    printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${ids[$i]}"
  done
  echo ""
  read -rp "$(t audit.choose_number)" choice
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#ids[@]}" ]; then
    log_error "$(t audit.invalid_choice)$choice (attendu 1-${#ids[@]})"
    exit 1
  fi
  PROJECT_ID="${ids[$((choice-1))]}"
fi

PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Validation + résolution du chemin ────────────────────────────────────────
PROJECT_PATH=$(resolve_project_path "$PROJECT_ID")

# ── Validation opencode ───────────────────────────────────────────────────────
load_adapter
adapter_validate || { log_error "opencode non disponible → oc install"; exit 1; }

# ── Agents nécessaires ────────────────────────────────────────────────────────
# Seul l'agent coordinateur auditor est requis, quel que soit le --type.
# Le type est transmis dans le prompt — c'est le coordinateur qui invoque
# auditor-subagent avec le domaine et le native_skill appropriés.
REQUIRED_AGENTS=("auditor")

# ── Dossier d'agents déployés ────────────────────────────────────────────────
agents_dir="$PROJECT_PATH/.opencode/agents"

# ── Bloc d'intro TUI ─────────────────────────────────────────────────────────
_intro "oc audit  ${PROJECT_ID}"
printf "${DIM}│${RESET}  %-12s %s\n" "$(t audit.label_path)"  "$PROJECT_PATH"
if [ -n "$AUDIT_TYPE" ]; then
  printf "${DIM}│${RESET}  %-12s %s\n" "$(t audit.label_type)"  "$AUDIT_TYPE"
fi
printf "${DIM}│${RESET}  %-12s %s\n" "$(t audit.label_agents)"  "${REQUIRED_AGENTS[*]}"

# ── Vérifier les agents dans projects.md ─────────────────────────────────────
agents_csv=$(get_project_agents "$PROJECT_ID")

if [ "$agents_csv" != "all" ]; then
  missing_in_config=()
  for agent_id in "${REQUIRED_AGENTS[@]}"; do
    if ! echo ",$agents_csv," | grep -qF ",$agent_id,"; then
      missing_in_config+=("$agent_id")
    fi
  done

  if [ ${#missing_in_config[@]} -gt 0 ]; then
    echo -e "${DIM}│${RESET}"
    log_warn "$(t audit.agents_missing_config)${missing_in_config[*]}"
    _prompt _add_agents "$(t audit.add_agents_prompt)"
    if [[ "${_add_agents:-Y}" =~ ^[Yy]$ ]]; then
      source "$LIB_DIR/agent-picker.sh"
      # Merger les agents manquants dans le CSV existant
      new_csv="$agents_csv"
      for agent_id in "${missing_in_config[@]}"; do
        new_csv="${new_csv},${agent_id}"
      done
      # Nettoyer les virgules en début / fin
      new_csv=$(echo "$new_csv" | sed 's/^,//;s/,$//')
      _set_project_agents "$PROJECT_ID" "$new_csv"
      log_success "$(t audit.agents_updated)$new_csv"
      agents_csv="$new_csv"

      # Proposer le redéploiement
      echo -e "${DIM}│${RESET}"
      _prompt _redeploy "$(t audit.redeploy_prompt)"
      if [[ "${_redeploy:-Y}" =~ ^[Yy]$ ]]; then
        echo ""
        bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
        echo ""
      else
        log_info "$(t audit.redeploy_later)opencode $PROJECT_ID"
      fi
    else
      # Refus → lister les agents audit physiquement déployés
      echo -e "${DIM}│${RESET}"
      log_info "$(t audit.searching_agents)$agents_dir…"

      available_audit_agents=()
      if [ -d "$agents_dir" ]; then
          while IFS= read -r f; do
            agent_name=$(basename "$f" .md)
            case "$agent_name" in
              auditor|auditor-subagent) available_audit_agents+=("$agent_name") ;;
            esac
          done < <(find "$agents_dir" -name "*.md" | sort)
      fi

      if [ ${#available_audit_agents[@]} -eq 0 ]; then
        log_error "$(t audit.no_agents_deployed)$agents_dir"
        log_info  "$(t audit.deploy_hint)opencode $PROJECT_ID"
        log_info  "$(t audit.add_agents_hint)$PROJECT_ID"
        exit 1
      fi

      echo -e "${DIM}│${RESET}"
      log_info "$(t audit.available_agents)"
      for i in "${!available_audit_agents[@]}"; do
        printf "  ${BLUE}%d${RESET}) %s\n" "$((i+1))" "${available_audit_agents[$i]}"
      done
      echo ""
      read -rp "$(t audit.choose_agent)" _choice
      if ! [[ "$_choice" =~ ^[0-9]+$ ]] || [ "$_choice" -lt 1 ] || [ "$_choice" -gt "${#available_audit_agents[@]}" ]; then
        log_error "$(t invalid_choice)"
        exit 1
      fi
      AUDIT_AGENT="${available_audit_agents[$((_choice-1))]}"
      REQUIRED_AGENTS=("$AUDIT_AGENT")
      log_info "$(t audit.agent_selected)$AUDIT_AGENT"
    fi
  fi
fi

# ── Vérifier le déploiement physique des agents ───────────────────────────────
echo -e "${DIM}│${RESET}"

if [ -n "$agents_dir" ] && [ ! -d "$agents_dir" ]; then
  log_warn "$(t audit.agents_not_deployed)opencode (dossier absent : $agents_dir)"
  _prompt _deploy_now "$(t audit.deploy_now_prompt)"
  if [[ "${_deploy_now:-Y}" =~ ^[Yy]$ ]]; then
    echo ""
    bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
    echo ""
  else
    log_warn "$(t audit.deploy_skipped)"
    log_info  "$(t audit.deploy_later)opencode $PROJECT_ID"
  fi
else
  # Vérifier chaque agent requis individuellement
  missing_deployed=()
  if [ -n "$agents_dir" ] && [ -d "$agents_dir" ]; then
    for agent_id in "${REQUIRED_AGENTS[@]}"; do
      [ ! -f "$agents_dir/${agent_id}.md" ] && missing_deployed+=("$agent_id")
    done
  fi

  if [ ${#missing_deployed[@]} -gt 0 ]; then
    log_warn "$(t audit.agents_not_deployed_list)${missing_deployed[*]}"
    _prompt _deploy_missing "$(t audit.redeploy_prompt)"
    if [[ "${_deploy_missing:-Y}" =~ ^[Yy]$ ]]; then
      echo ""
      bash "$SCRIPTS_DIR/cmd-deploy.sh" "$PROJECT_ID"
      echo ""
    else
      log_warn "$(t audit.deploy_skipped)"
    fi
  fi
fi

# ── Construire le prompt ──────────────────────────────────────────────────────
PROMPT=$(build_audit_bootstrap_prompt "$PROJECT_PATH" "$PROJECT_ID" "$AUDIT_TYPE")
AGENT_NAME="${REQUIRED_AGENTS[0]}"

echo -e "${DIM}│${RESET}"
log_info "$(t audit.main_agent)${AGENT_NAME}"

# ── Confirmation avant lancement ─────────────────────────────────────────────
_outro "$(t audit.launching)opencode…"
_prompt _ ""

adapter_start "$PROJECT_PATH" "$PROMPT" "$PROJECT_ID" "$AGENT_NAME"
