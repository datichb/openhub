#!/bin/bash
# cmd-doctor.sh — Diagnostic de santé du hub
# Usage : oc doctor
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Compteurs globaux ─────────────────────────────────────────────────────────
_DOC_FAIL=0
_DOC_WARN=0

# ── Helpers d'affichage ───────────────────────────────────────────────────────
_doc_ok()    { printf "  ${GREEN}[%-4s]${RESET}  %s\n" "$(t doctor.ok)"   "$1"; }
_doc_warn_p(){ printf "  ${YELLOW}[%-4s]${RESET}  %s\n" "$(t doctor.warn)" "$1"; _DOC_WARN=$((_DOC_WARN + 1)); }
_doc_fail_p(){ printf "  ${RED}[%-4s]${RESET}  %s\n"   "$(t doctor.fail)" "$1"; _DOC_FAIL=$((_DOC_FAIL + 1)); }

# ── Section 1 : Outils externes ───────────────────────────────────────────────
_check_tools() {
  _h_section "$(t doctor.section.tools)"

  # Outils critiques (FAIL si absent)
  local _tool
  for _tool in jq git opencode node perl; do
    if command -v "$_tool" &>/dev/null; then
      _doc_ok "$_tool — $(t doctor.tool_ok)"
    else
      _doc_fail_p "$_tool — $(t doctor.tool_missing_critical)"
    fi
  done

  # Outils importants (WARN si absent)
  for _tool in sqlite3 bd; do
    if command -v "$_tool" &>/dev/null; then
      _doc_ok "$_tool — $(t doctor.tool_ok)"
    else
      _doc_warn_p "$_tool — $(t doctor.tool_missing_warn)"
    fi
  done

  # Outils optionnels (INFO si absent — pas de compteur)
  for _tool in fzf bun npx; do
    if command -v "$_tool" &>/dev/null; then
      _doc_ok "$_tool — $(t doctor.tool_ok)"
    else
      printf "  ${DIM}[INFO]${RESET}  %s — %s\n" "$_tool" "$(t doctor.tool_missing_info)"
    fi
  done
}

# ── Section 2 : Fichiers de configuration ────────────────────────────────────
_check_config() {
  _h_section "$(t doctor.section.config)"

  # hub.json
  if [ ! -f "$HUB_CONFIG" ]; then
    _doc_fail_p "hub.json — $(t doctor.config_missing)"
  elif ! jq empty "$HUB_CONFIG" 2>/dev/null; then
    _doc_fail_p "hub.json — $(t doctor.config_invalid)"
  else
    _doc_ok "hub.json — $(t doctor.config_ok)"
    # Vérifier la dérive de version
    if [ -f "$HUB_CONFIG_EXAMPLE" ] && command -v jq &>/dev/null; then
      local _v_current _v_example
      _v_current=$(jq -r '.version // ""' "$HUB_CONFIG" 2>/dev/null || true)
      _v_example=$(jq -r '.version // ""' "$HUB_CONFIG_EXAMPLE" 2>/dev/null || true)
      if [ -n "$_v_current" ] && [ -n "$_v_example" ] && [ "$_v_current" != "$_v_example" ]; then
        _doc_warn_p "hub.json version $_v_current ≠ hub.json.example $_v_example — $(t doctor.config_version_drift)"
      fi
    fi
  fi

  # providers.json
  if [ -f "${PROVIDERS_FILE:-}" ]; then
    _doc_ok "providers.json — $(t doctor.config_ok)"
  else
    _doc_warn_p "providers.json — $(t doctor.config_missing)"
  fi

  # projects.md
  if [ -f "$PROJECTS_FILE" ]; then
    _doc_ok "projects.md — $(t doctor.config_ok)"
  else
    _doc_ok "projects.md — absent (sera créé automatiquement)"
  fi

  # api-keys.local.md — vérifier les permissions
  if [ -f "$API_KEYS_FILE" ]; then
    local _perms
    _perms=$(stat -c "%a" "$API_KEYS_FILE" 2>/dev/null \
          || stat -f "%A" "$API_KEYS_FILE" 2>/dev/null \
          || echo "unknown")
    if [ "$_perms" = "600" ]; then
      _doc_ok "api-keys.local.md — $(t doctor.perms_ok)"
    elif [ "$_perms" = "unknown" ]; then
      printf "  ${DIM}[INFO]${RESET}  api-keys.local.md — permissions non vérifiables sur ce système\n"
    else
      _doc_warn_p "api-keys.local.md — $(t doctor.perms_warn) (actuellement: $_perms)"
      printf "    ${DIM}Corriger : chmod 600 %s${RESET}\n" "$API_KEYS_FILE"
    fi
  fi
}

# ── Section 3 : Projets ───────────────────────────────────────────────────────
_check_one_project() {
  local _pid="$1"
  local _path
  _path=$(get_project_path "$_pid" 2>/dev/null || true)
  _path="${_path/#\~/$HOME}"

  if [ -z "$_path" ]; then
    _doc_warn_p "$_pid — $(t doctor.project_path_missing) (path non défini dans paths.local.md)"
    return
  fi

  if [ ! -d "$_path" ]; then
    _doc_warn_p "$_pid — $(t doctor.project_path_missing) ($_path)"
    return
  fi

  local _agents_ok=true _config_ok=true _key_ok=true

  [ ! -d "$_path/.opencode/agents" ] && _agents_ok=false
  [ ! -f "$_path/opencode.json" ]     && _config_ok=false
  api_keys_entry_exists "$_pid" || _key_ok=false

  if [ "$_agents_ok" = true ] && [ "$_config_ok" = true ]; then
    local _detail
    _detail="$(t doctor.project_ok)"
    [ "$_key_ok" = false ] && _detail="$_detail, $(t doctor.project_no_api_key)"
    _doc_ok "$_pid — $_detail"
  else
    [ "$_agents_ok" = false ] && _doc_warn_p "$_pid — $(t doctor.project_agents_missing)"
    [ "$_config_ok" = false ] && _doc_warn_p "$_pid — $(t doctor.project_config_missing)"
    [ "$_key_ok"    = false ] && printf "  ${DIM}[INFO]${RESET}  %s — %s\n" "$_pid" "$(t doctor.project_no_api_key)"
  fi
}

_check_projects() {
  _h_section "$(t doctor.section.projects)"

  ensure_projects_file
  local _project_ids=()
  while IFS= read -r _line; do
    _project_ids+=("$_line")
  done < <(grep "^## " "$PROJECTS_FILE" 2>/dev/null | sed 's/^## //' || true)

  if [ ${#_project_ids[@]} -eq 0 ]; then
    printf "  ${DIM}Aucun projet enregistré — lancez : oc init${RESET}\n"
  else
    local _p
    for _p in "${_project_ids[@]}"; do
      _check_one_project "$_p"
    done
  fi
}

# ── Programme principal ───────────────────────────────────────────────────────
log_title "$(t doctor.title)"
echo ""

_check_tools
echo ""

_check_config
echo ""

_check_projects
echo ""

# Répertoire de locks (auto-créé si absent)
mkdir -p "${HUB_DIR}/.locks" 2>/dev/null || true

# ── Résumé ────────────────────────────────────────────────────────────────────
if [ "$_DOC_FAIL" -gt 0 ]; then
  log_error "$(t doctor.summary_fail) ($_DOC_FAIL FAIL, $_DOC_WARN WARN)"
  exit 1
elif [ "$_DOC_WARN" -gt 0 ]; then
  log_warn "$(t doctor.summary_warn) ($_DOC_WARN WARN)"
  exit 2
else
  log_success "$(t doctor.summary_ok)"
  exit 0
fi
