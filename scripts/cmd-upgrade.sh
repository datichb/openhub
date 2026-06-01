#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# cmd-upgrade.sh — Mettre à jour les sources du hub (git pull / checkout tag)
#
# Usage :
#   oc upgrade              → git pull --ff-only sur la branche courante (main)
#   oc upgrade v1.1.0       → git fetch --tags + git checkout v1.1.0
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

# ── Argument optionnel : version cible ────────────────────────────────────────
_raw_ref="${1:-}"
TARGET_REF=""

if [ -n "$_raw_ref" ]; then
  _raw_ver="${_raw_ref#v}"
  if ! printf '%s' "$_raw_ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    log_error "$(t upgrade.invalid_ref) : '$_raw_ref'"
    log_info  "$(t upgrade.ref_hint)"
    exit 1
  fi
  TARGET_REF="v${_raw_ver}"
fi

log_title "$(t upgrade.title)${TARGET_REF:+ → ${TARGET_REF}}"

# ── Lire la version actuelle avant la mise à jour ────────────────────────────
_version_before=""
if [ -f "$HUB_CONFIG" ] && command -v jq &>/dev/null; then
  _version_before=$(jq -r '.version // empty' "$HUB_CONFIG" 2>/dev/null || true)
elif [ -f "$HUB_CONFIG" ]; then
  _version_before=$(grep '"version"' "$HUB_CONFIG" | sed 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' || true)
fi

# ── Mise à jour ───────────────────────────────────────────────────────────────
_updated=false

if [ -n "$TARGET_REF" ]; then
  # Mode version épinglée : fetch + checkout tag
  log_info "$(t upgrade.fetching_tag) ${TARGET_REF}..."
  if git -C "$HUB_DIR" fetch --tags --quiet 2>/dev/null; then
    if git -C "$HUB_DIR" checkout --quiet "$TARGET_REF" 2>/dev/null; then
      _updated=true
    else
      log_error "$(t upgrade.tag_not_found) : ${TARGET_REF}"
      exit 1
    fi
  else
    log_warn "$(t upgrade.fetch_failed)"
    exit 1
  fi
else
  # Mode main : pull --ff-only
  log_info "$(t upgrade.pulling)..."
  _pull_output=$(git -C "$HUB_DIR" pull --ff-only 2>&1 </dev/null || true)
  if printf '%s' "$_pull_output" | grep -qi "already up.to.date\|déjà à jour"; then
    log_success "$(t upgrade.already_uptodate)"
  elif printf '%s' "$_pull_output" | grep -qiE "fatal|error"; then
    log_warn "$(t upgrade.pull_failed)"
    printf '%s\n' "$_pull_output" >&2
    exit 1
  else
    _updated=true
  fi
fi

# ── Lire la version après la mise à jour ─────────────────────────────────────
_version_after=""
if [ -f "$HUB_CONFIG" ] && command -v jq &>/dev/null; then
  _version_after=$(jq -r '.version // empty' "$HUB_CONFIG" 2>/dev/null || true)
elif [ -f "$HUB_CONFIG" ]; then
  _version_after=$(grep '"version"' "$HUB_CONFIG" | sed 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' || true)
fi

# ── Résumé de la mise à jour ─────────────────────────────────────────────────
if [ "$_updated" = true ]; then
  if [ -n "$_version_before" ] && [ -n "$_version_after" ] && [ "$_version_before" != "$_version_after" ]; then
    log_success "$(t upgrade.updated) v${_version_before} → v${_version_after}"
  else
    log_success "$(t upgrade.done)"
  fi
fi

# ── Proposer oc sync ─────────────────────────────────────────────────────────
if [ "$_updated" = true ]; then
  echo ""
  log_warn "$(t upgrade.sync_stale_warn)"
  echo ""
  printf '%s' "$(t upgrade.sync_now)" >&2
  read -r _sync_now || true
  if [[ "${_sync_now:-Y}" =~ ^[Yy]$ ]]; then
    bash "$SCRIPTS_DIR/cmd-sync.sh"
  else
    log_info "$(t upgrade.sync_later)"
  fi
fi
