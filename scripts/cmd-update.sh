#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"

log_title "$(t update.title)"

target="opencode"
load_adapter "$target"
adapter_update

log_info "$(t update.beads_updating)"
if command -v bd &>/dev/null; then
  if command -v brew &>/dev/null && brew list bd &>/dev/null 2>&1; then
    brew upgrade bd && log_success "$(t update.beads_done)" \
      || log_warn "$(t update.beads_failed)"
  else
    log_warn "$(t update.beads_not_brew)"
    log_info "$(t update.beads_manual_hint)"
  fi
else
  log_warn "$(t update.bd_missing)"
fi

# ── Skills externes ───────────────────────────────────────────────────────────
EXTERNAL_SOURCES="$HUB_DIR/skills/external/.sources.json"
SKILLS_UPDATED=false
if [ -f "$EXTERNAL_SOURCES" ] && [ -s "$EXTERNAL_SOURCES" ] && [ "$(cat "$EXTERNAL_SOURCES")" != '{}' ]; then
  echo ""
  log_info "$(t update.skills_updating)"
  bash "$SCRIPTS_DIR/cmd-skills.sh" update && SKILLS_UPDATED=true
else
  log_info "$(t update.skills_none)"
fi

echo ""
log_success "$(t update.done)"

# ── Proposer un sync si des skills ont été mis à jour ─────────────────────────
if [ "$SKILLS_UPDATED" = true ]; then
  echo ""
  log_warn "$(t update.skills_stale_warn)"
  echo ""
  read -rp "$(t update.sync_now)" sync_now
  if [[ "${sync_now:-Y}" =~ ^[Yy]$ ]]; then
    bash "$SCRIPTS_DIR/cmd-sync.sh"
  else
    log_info "$(t update.sync_later)"
  fi
fi
