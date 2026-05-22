#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"

_intro "$(t install.title)"

OS=$(detect_os)
log_info "$(t install.os_detected) $OS"

# ── Vérifier jq ─────────────────────────
if ! command -v jq &>/dev/null; then
  log_warn "$(t install.jq_missing)"
  if [ "$OS" = "macos" ] && command -v brew &>/dev/null; then
    read -rp "  $(t install.jq_install_brew)" jq_choice
    if [[ "${jq_choice:-Y}" =~ ^[Yy]$ ]]; then
      brew install jq && log_success "$(t install.jq_installed)" || log_error "$(t install.jq_failed)"
    else
      log_warn "$(t install.jq_degraded)"
    fi
  else
    log_warn "$(t install.jq_manual)"
    log_info "  macOS  : brew install jq"
    log_info "  Ubuntu : sudo apt-get install jq"
    log_info "  Autre  : https://jqlang.github.io/jq/download/"
  fi
else
  log_success "jq $(jq --version)"
fi

_outro "$(t install.os_done) $OS"

active_targets=("opencode")

# ── Installation opencode ────────────────
_intro "$(t install.opencode_title)"

# ── Dossiers requis ──────────────────────
mkdir -p "$HUB_DIR/projects" "$HUB_DIR/skills" "$HUB_DIR/agents" \
         "$HUB_DIR/.opencode/agents" "$HUB_DIR/config" \
         "$HUB_DIR/scripts/lib" "$HUB_DIR/scripts/adapters"

# ── Écrire config/hub.json (seulement si absent ou si l'utilisateur confirme) ──
if [ -f "$HUB_DIR/config/hub.json" ]; then
  log_warn "$(t install.hub_json_exists)"
  read -rp "  $(t install.hub_json_overwrite)" overwrite_choice
  if [[ "${overwrite_choice:-N}" =~ ^[Yy]$ ]]; then
    _write_hub_json=true
  else
    log_info "$(t install.hub_json_kept)"
    _write_hub_json=false
  fi
else
  _write_hub_json=true
fi

if [ "$_write_hub_json" = true ]; then
  cat > "$HUB_DIR/config/hub.json" << HUBJSON
{
  "version": "2.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "${DEFAULT_MODEL}"
  }
}
HUBJSON
  log_success "$(t install.hub_json_created) ${active_targets[*]})"
fi

# ── Installer chaque cible sélectionnée ──
for target in "${active_targets[@]}"; do
  load_adapter "$target"
  adapter_install
done

_outro "$(t install.opencode_done)"

# ── Fournisseur LLM par défaut ────────────────────────────────────────────────
# Cette section s'exécute APRÈS adapter_install pour éviter de persister la clé
# dans hub.json si l'installation des outils échoue.
_intro "$(t install.provider_title)"
log_info "$(t install.provider_choose)"
echo -e "${DIM}│${RESET}"

# Construire le menu dynamiquement depuis providers.json
_provider_names=()
if [ -f "$PROVIDERS_FILE" ] && command -v jq &>/dev/null; then
  while IFS= read -r pname; do
    _provider_names+=("$pname")
  done < <(jq -r '.providers | keys[]' "$PROVIDERS_FILE")
fi

_provider_count="${#_provider_names[@]}"
if [ "$_provider_count" -gt 0 ]; then
  _i=1
  for pname in "${_provider_names[@]}"; do
    _label=$(get_provider_info "$pname" "label")
    if [ "$_i" -eq 1 ]; then
      printf "  %d. %s %s\n" "$_i" "$_label" "$(t install.provider_recommended)"
    else
      printf "  %d. %s\n" "$_i" "$_label"
    fi
    _i=$((_i + 1))
  done
  printf "  %d. %s\n" "$((_provider_count + 1))" "$(t install.provider_skip)"
  echo ""
  read -rp "$(t install.choose_prompt)" _provider_choice
  _provider_choice="${_provider_choice:-1}"
else
  # Fallback sans providers.json : menu statique
  echo "  1. Anthropic $(t install.provider_recommended)"
  echo "  2. MammouthAI"
  echo "  3. GitHub Models"
  echo "  4. AWS Bedrock"
  echo "  5. Ollama (local)"
  echo "  6. $(t install.provider_skip)"
  echo ""
  read -rp "$(t install.choose_prompt)" _provider_choice
  _provider_choice="${_provider_choice:-1}"
  _provider_names=("anthropic" "mammouth" "github-models" "bedrock" "ollama")
  _provider_count=5
fi

# Résoudre le fournisseur sélectionné
_selected_provider=""
if [[ "$_provider_choice" =~ ^[0-9]+$ ]] && [ "$_provider_choice" -ge 1 ] && [ "$_provider_choice" -le "$_provider_count" ]; then
  _selected_provider="${_provider_names[$((_provider_choice - 1))]}"
fi
# Si choix hors plage ou "Ignorer", _selected_provider reste vide

if [ -n "$_selected_provider" ]; then
  _selected_label=$(get_provider_info "$_selected_provider" "label" 2>/dev/null || echo "$_selected_provider")
  _requires_api_key=$(get_provider_info "$_selected_provider" "requires_api_key" 2>/dev/null || echo "true")
  _default_base_url=$(get_provider_info "$_selected_provider" "default_base_url" 2>/dev/null || echo "")
  _requires_base_url=$(get_provider_info "$_selected_provider" "requires_base_url" 2>/dev/null || echo "false")

  echo ""
  _provider_api_key=""
  _provider_base_url="$_default_base_url"

  if [ "$_requires_api_key" = "true" ]; then
    trap 'stty echo 2>/dev/null; echo ""; exit 130' INT TERM
    read -rsp "  Clé API ${_selected_label} $(t install.provider_api_key) " _provider_api_key
    stty echo 2>/dev/null
    trap - INT TERM
    echo ""
  fi

  if [ "$_requires_base_url" = "true" ] && [ -n "$_default_base_url" ]; then
    read -rp "  $(t install.provider_base_url) [${_default_base_url}] : " _input_base_url
    _provider_base_url="${_input_base_url:-$_default_base_url}"
  fi

  # Écrire dans hub.json seulement si une clé est fournie (ou si ollama, pas besoin de clé)
  _should_save=false
  [ -n "$_provider_api_key" ] && _should_save=true
  [ "$_requires_api_key" = "false" ] && _should_save=true

  if [ "$_should_save" = "true" ] && [ -f "$HUB_DIR/config/hub.json" ]; then
    _hub_json=$(jq \
      --arg name "$_selected_provider" \
      --arg key  "$_provider_api_key" \
      --arg url  "$_provider_base_url" \
      '.default_provider.name = $name | .default_provider.api_key = $key | .default_provider.base_url = $url' \
      "$HUB_DIR/config/hub.json")
    echo "$_hub_json" > "$HUB_DIR/config/hub.json"

    # Protéger hub.json si clé présente
    if [ -n "$_provider_api_key" ]; then
      if [ ! -f "$HUB_DIR/.gitignore" ] || ! grep -qx "config/hub.json" "$HUB_DIR/.gitignore"; then
        echo "config/hub.json" >> "$HUB_DIR/.gitignore"
      fi
    fi

    _outro "$(t install.provider_configured) ${_selected_label}"
  else
    _outro "$(t install.provider_skipped)"
  fi
else
  _outro "$(t install.provider_skipped)"
fi

# ── Fichiers initiaux ────────────────────
_intro "$(t install.config_title)"
ensure_projects_file
ensure_paths_file
ensure_api_keys_file

log_info "$(t install.skills_tip)"
log_info "  ./oc.sh skills search <query>        # Rechercher"
log_info "  ./oc.sh skills add /owner/repo name  # Ajouter"
_outro "$(t install.config_done)"

# ── Installer Beads (bd) ─────────────────
_intro "$(t install.beads_title)"
if command -v bd &>/dev/null; then
  bd_version=$(bd --version 2>/dev/null || bd version 2>/dev/null || echo '?')
  log_success "$(t install.beads_already) ($bd_version)"
else
  log_warn "$(t install.beads_missing)"
  read -rp "  $(t install.beads_install_prompt)" _beads_choice </dev/tty
  if [[ "${_beads_choice:-Y}" =~ ^[Yy]$ ]]; then
    if command -v brew &>/dev/null; then
      log_info "$(t install.beads_via_brew)"
      if brew install beads; then
        log_success "$(t install.beads_installed)"
      else
        log_warn "$(t install.beads_brew_failed)"
        if curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash; then
          log_success "$(t install.beads_curl_installed)"
        else
          log_warn "$(t install.beads_failed)"
        fi
      fi
    elif command -v curl &>/dev/null; then
      log_info "$(t install.beads_via_curl)"
      if curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash; then
        log_success "$(t install.beads_installed)"
      else
        log_warn "$(t install.beads_failed)"
        log_info "  macOS  : brew install beads"
        log_info "  Linux  : curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash"
      fi
    else
      log_warn "$(t install.beads_no_tools)"
      log_info "  macOS  : brew install beads"
      log_info "  Linux  : curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash"
    fi
  else
    log_info "$(t install.beads_later)"
  fi
fi
_outro "$(t install.beads_done)"

log_success "$(t install.ready)"
