#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Parsing des arguments ─────────────────
CLEAN_MODE=false
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --clean) CLEAN_MODE=true ;;
    *)       ARGS+=("$arg") ;;
  esac
done

PROJECT_ID="${ARGS[0]:-}"
require_project_id "$PROJECT_ID"
PROJECT_ID=$(normalize_project_id "$PROJECT_ID")

# ── Confirmation ──────────────────────────
if ! project_exists "$PROJECT_ID"; then
  log_error "$(t remove.not_found) : $PROJECT_ID"
  exit 1
fi

# Résoudre le chemin AVANT suppression du registre (nécessaire pour --clean)
PROJECT_PATH=""
if [ "$CLEAN_MODE" = true ]; then
  PROJECT_PATH=$(get_project_path "$PROJECT_ID" 2>/dev/null || true)
  PROJECT_PATH="${PROJECT_PATH/#\~/$HOME}"
  if [ -z "$PROJECT_PATH" ] || [ ! -d "$PROJECT_PATH" ]; then
    log_warn "$(t remove.no_path) $PROJECT_ID $(t remove.clean_ignored)"
    CLEAN_MODE=false
  fi
fi

if [ "$CLEAN_MODE" = true ]; then
  _confirm_msg=$(t remove.confirm_clean | sed "s/PROJECT/$PROJECT_ID/;s|PATH|$PROJECT_PATH|")
  read -rp "$(echo -e "  ${YELLOW}⚠${RESET}  ${_confirm_msg}")" confirm
else
  _confirm_msg=$(t remove.confirm | sed "s/PROJECT/$PROJECT_ID/")
  read -rp "$(echo -e "  ${YELLOW}⚠${RESET}  ${_confirm_msg}")" confirm
fi
[[ "$confirm" =~ ^[Yy]$ ]] || { log_info "$(t cancelled)"; exit 0; }

# ── Nettoyage des fichiers déployés (--clean) ─────────────────────────────
if [ "$CLEAN_MODE" = true ]; then
  source "$LIB_DIR/adapter-manager.sh"

  # Déterminer les cibles actives
  local_targets="opencode"

  log_info "Nettoyage des fichiers déployés dans ${PROJECT_PATH}…"

  while IFS= read -r tgt; do
    case "$tgt" in
      opencode)
        # .opencode/agents/ et opencode.json
        if [ -d "$PROJECT_PATH/.opencode/agents" ]; then
          rm -rf "$PROJECT_PATH/.opencode/agents"
          log_success "Supprimé : .opencode/agents/"
        fi
        if [ -f "$PROJECT_PATH/opencode.json" ]; then
          rm -f "$PROJECT_PATH/opencode.json"
          log_success "Supprimé : opencode.json"
        fi
        ;;

    esac
  done <<< "$local_targets"
fi

# ── Supprimer du projects.md ──────────────
# Supprime le bloc ## PROJECT_ID jusqu'au prochain ## ou fin de fichier
command -v perl &>/dev/null || { log_error "perl requis pour cette opération"; exit 1; }
perl -i.bak -0pe 's/\n## \Q'"${PROJECT_ID}"'\E\n.*?(?=\n## |\z)//s' "$PROJECTS_FILE" \
  && rm -f "${PROJECTS_FILE}.bak"
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
