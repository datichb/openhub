#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Fonction principale ───────────────────────────────────────────────────────
cmd_remove() {
# ── Parsing des arguments ─────────────────
local CLEAN_MODE=false
local DRY_RUN=false
local PROJECT_ID=""
local _prev=""
for arg in "$@"; do
  case "$_prev" in
    --project|-p) PROJECT_ID="$arg"; _prev=""; continue ;;
  esac
  case "$arg" in
    --clean|-c)    CLEAN_MODE=true ;;
    --dry-run|-n)  DRY_RUN=true ;;
    --project|-p)  _prev="$arg" ;;
    -*)            : ;;  # ignorer les flags inconnus
    *)             [ -z "$PROJECT_ID" ] && PROJECT_ID="$arg" ;;
  esac
done
require_project_id "$PROJECT_ID"
PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Confirmation ──────────────────────────
if ! project_exists "$PROJECT_ID"; then
  log_error "$(t remove.not_found) : $PROJECT_ID"
  exit 1
fi

# Résoudre le chemin AVANT suppression du registre (nécessaire pour --clean)
local PROJECT_PATH=""
if [ "$CLEAN_MODE" = true ]; then
  PROJECT_PATH=$(get_project_path "$PROJECT_ID" 2>/dev/null || true)
  PROJECT_PATH="${PROJECT_PATH/#\~/$HOME}"
  if [ -z "$PROJECT_PATH" ] || [ ! -d "$PROJECT_PATH" ]; then
    log_warn "$(t remove.no_path) $PROJECT_ID $(t remove.clean_ignored)"
    CLEAN_MODE=false
  fi
fi

local confirm
if [ "$DRY_RUN" = true ]; then
  log_title "$(t remove.dryrun_title)"
  echo ""
  if [ "$CLEAN_MODE" = true ]; then
    if [ -d "$PROJECT_PATH/.opencode/agents" ]; then
      log_warn "$(t remove.dryrun_would_delete_agents) ($PROJECT_PATH/.opencode/agents/)"
    fi
    if [ -f "$PROJECT_PATH/opencode.json" ]; then
      log_warn "$(t remove.dryrun_would_delete_config) ($PROJECT_PATH/opencode.json)"
    fi
  fi
  log_info "$(t remove.dryrun_would_unregister) $PROJECT_ID (projects.md)"
  path_exists "$PROJECT_ID" && log_info "$(t remove.dryrun_would_remove_path) $PROJECT_ID"
  api_keys_entry_exists "$PROJECT_ID" && log_info "$(t remove.dryrun_would_remove_key) $PROJECT_ID"
  echo ""
  log_info "$(t remove.dryrun_no_changes)"
  exit 0
fi

if [ "$CLEAN_MODE" = true ]; then
  local _confirm_msg
  _confirm_msg=$(t remove.confirm_clean | sed "s/PROJECT/$PROJECT_ID/;s|PATH|$PROJECT_PATH|")
  _prompt confirm "$(echo -e "  ${YELLOW}⚠${RESET}  ${_confirm_msg}")"
else
  local _confirm_msg
  _confirm_msg=$(t remove.confirm | sed "s/PROJECT/$PROJECT_ID/")
  _prompt confirm "$(echo -e "  ${YELLOW}⚠${RESET}  ${_confirm_msg}")"
fi
[[ "$confirm" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)"; exit 0; }

# ── Nettoyage des fichiers déployés (--clean) ─────────────────────────────
if [ "$CLEAN_MODE" = true ]; then
  source "$LIB_DIR/adapter-manager.sh"

  log_info "Nettoyage des fichiers déployés dans ${PROJECT_PATH}…"

  # .opencode/agents/ et opencode.json
  if [ -d "$PROJECT_PATH/.opencode/agents" ]; then
    rm -rf "$PROJECT_PATH/.opencode/agents"
    log_success "Supprimé : .opencode/agents/"
  fi
  if [ -f "$PROJECT_PATH/opencode.json" ]; then
    rm -f "$PROJECT_PATH/opencode.json"
    log_success "Supprimé : opencode.json"
  fi
fi

# ── Supprimer du projects.md ──────────────
# Supprime le bloc ## PROJECT_ID jusqu'au prochain ## ou fin de fichier
command -v perl &>/dev/null || { log_error "perl requis pour cette opération"; exit 1; }
_acquire_lock "${OC_LOCK_PROJECTS:-projects}" 10 || { log_error "filelock: timeout"; exit 1; }
perl -i.bak -0pe 's/\n?## \Q'"${PROJECT_ID}"'\E\n.*?(?=\n## |\z)\n?//s' "$PROJECTS_FILE" \
  && rm -f "${PROJECTS_FILE}.bak"
_release_lock "${OC_LOCK_PROJECTS:-projects}"
log_success "$(t remove.projects_removed) : $PROJECT_ID"

# ── Supprimer du paths.local.md ───────────
if path_exists "$PROJECT_ID"; then
  sed -i.bak "/^${PROJECT_ID}=/d" "$PATHS_FILE" && rm -f "${PATHS_FILE}.bak"
  log_success "$(t remove.path_removed)"
fi

# ── Supprimer de api-keys.local.md ────────
if api_keys_entry_exists "$PROJECT_ID"; then
  remove_api_keys_section "$PROJECT_ID"
  log_success "$(t remove.api_key_removed)"
fi

echo ""
log_success "$PROJECT_ID $(t remove.done)"
}

# ── Exécution directe uniquement (pas quand sourcé) ──────────────────────────
[[ "${BASH_SOURCE[0]}" != "$0" ]] && return 0
cmd_remove "$@"
