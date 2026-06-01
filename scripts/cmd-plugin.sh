#!/bin/bash
# Install RTK plugin for OpenCode
# Usage: oc plugin install rtk

set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

PLUGIN_NAME="${1:-rtk}"
PLUGIN_SOURCE="$HUB_DIR/plugins/$PLUGIN_NAME/${PLUGIN_NAME}.ts"
PLUGIN_TARGET="$HOME/.config/opencode/plugins/${PLUGIN_NAME}.ts"

resolve_oc_lang

log_title "$(t plugin.install_title): $PLUGIN_NAME"

# ── Vérifications préalables ──────────────────────────────────────────────────

# 1. Vérifier que le plugin existe dans le hub
if [ ! -f "$PLUGIN_SOURCE" ]; then
  log_error "$(t plugin.not_found): $PLUGIN_NAME"
  log_info "$(t plugin.available):"
  ls -1 "$HUB_DIR/plugins" 2>/dev/null || echo "  $(t plugin.none)"
  exit 1
fi

# 2. Vérifier qu'OpenCode est installé
if ! command -v opencode &> /dev/null; then
  log_error "$(t plugin.opencode_not_installed)"
  log_info "$(t plugin.install_opencode):"
  echo "  npm install -g opencode-ai"
  echo "  # $(t plugin.or)"
  echo "  brew install opencode"
  exit 1
fi

# 3. Vérifications spécifiques au plugin RTK
if [ "$PLUGIN_NAME" = "rtk" ]; then
  # Vérifier que RTK est installé
  if ! command -v rtk &> /dev/null; then
    log_error "$(t plugin.rtk_not_installed)"
    log_info "$(t plugin.install_rtk):"
    echo "  brew install rtk"
    exit 1
  fi

  # Vérifier la version de RTK
  RTK_VERSION=$(rtk --version 2>/dev/null | awk '{print $2}' || echo "0.0.0")
  REQUIRED_VERSION="0.42.0"
  
  if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$RTK_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    log_warning "$(t plugin.rtk_outdated): $RTK_VERSION < $REQUIRED_VERSION"
    log_info "$(t plugin.upgrade_rtk):"
    echo "  brew upgrade rtk"
    echo ""
    read -p "$(t plugin.continue_anyway) [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
    fi
  else
    log_success "$(t plugin.rtk_version_ok): $RTK_VERSION"
  fi
fi

# ── Installation ──────────────────────────────────────────────────────────────

# Créer le dossier de destination si nécessaire
mkdir -p "$HOME/.config/opencode/plugins"

# Sauvegarder l'ancien plugin si existant
if [ -f "$PLUGIN_TARGET" ]; then
  BACKUP="$PLUGIN_TARGET.backup.$(date +%Y%m%d-%H%M%S)"
  log_info "$(t plugin.backing_up): $BACKUP"
  cp "$PLUGIN_TARGET" "$BACKUP"
fi

# Copier le plugin
log_info "$(t plugin.copying)..."
cp "$PLUGIN_SOURCE" "$PLUGIN_TARGET"

log_success "$(t plugin.installed): $PLUGIN_TARGET"

# ── Vérification post-installation ────────────────────────────────────────────

echo ""
log_title "$(t plugin.verification)"

# Vérifier que le fichier est bien présent
if [ -f "$PLUGIN_TARGET" ]; then
  log_success "$(t plugin.file_present)"
  
  # Afficher les permissions
  PERMS=$(ls -lh "$PLUGIN_TARGET" | awk '{print $1, $3, $9}')
  log_info "$(t plugin.permissions): $PERMS"
else
  log_error "$(t plugin.file_missing)"
  exit 1
fi

# ── Instructions suivantes ────────────────────────────────────────────────────

echo ""
log_title "$(t plugin.next_steps)"

echo "$(t plugin.step1):"
echo "  $(t plugin.restart_opencode)"
echo ""
echo "$(t plugin.step2):"
echo "  tail -f ~/.cache/opencode/logs/opencode.log | grep ${PLUGIN_NAME}-plugin"
echo ""
echo "$(t plugin.step3):"
if [ "$PLUGIN_NAME" = "rtk" ]; then
  echo "  $(t plugin.rtk_test_command)"
fi

# Proposer d'afficher la documentation
echo ""
read -p "$(t plugin.show_docs) [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  if [ -f "$HUB_DIR/plugins/$PLUGIN_NAME/README.md" ]; then
    cat "$HUB_DIR/plugins/$PLUGIN_NAME/README.md"
  else
    log_warning "$(t plugin.no_docs)"
  fi
fi

log_success "$(t plugin.done)"
