#!/bin/bash
# Manage OpenCode plugins
# Usage: oc plugin install <name>

set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

SUBCOMMAND="${1:-}"
PLUGIN_NAME="${2:-rtk}"
PLUGIN_SOURCE="$HUB_DIR/plugins/$PLUGIN_NAME/${PLUGIN_NAME}.ts"
PLUGIN_TARGET="$HOME/.config/opencode/plugins/${PLUGIN_NAME}.ts"

resolve_oc_lang

# Seule sous-commande supportée : install
if [ "$SUBCOMMAND" != "install" ]; then
  log_error "$(t plugin.not_found): $SUBCOMMAND"
  log_info "$(t help.plugin_install.cmd)"
  exit 1
fi

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

# 4. Vérifications spécifiques au plugin context-mode
if [ "$PLUGIN_NAME" = "context-mode" ]; then
  # Vérifier Node >= 22.5 (prérequis context-mode)
  if ! command -v node &> /dev/null; then
    log_error "Node.js introuvable — requis pour context-mode"
    log_info "Installer Node.js >= 22.5 :"
    echo "  brew install node"
    exit 1
  fi

  NODE_VERSION=$(node --version 2>/dev/null | sed 's/v//' || echo "0.0.0")
  REQUIRED_NODE="22.5.0"

  if [ "$(printf '%s\n' "$REQUIRED_NODE" "$NODE_VERSION" | sort -V | head -n1)" != "$REQUIRED_NODE" ]; then
    log_error "Node.js $NODE_VERSION < $REQUIRED_NODE requis pour context-mode"
    log_info "Mettre à jour Node.js :"
    echo "  brew upgrade node"
    exit 1
  else
    log_success "Node.js OK : $NODE_VERSION"
  fi

  # Installer le package npm si absent
  if ! node -e "require('context-mode')" &>/dev/null 2>&1; then
    log_info "Installation du package npm 'context-mode'..."
    if npm install -g context-mode; then
      log_success "Package context-mode installé"
    else
      log_warning "Installation npm globale échouée — le plugin tentera via npx au runtime"
      echo ""
      read -p "Continuer l'installation du plugin quand même ? [y/N] " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
      fi
    fi
  else
    CM_VERSION=$(node -e "console.log(require('context-mode/package.json').version)" 2>/dev/null || echo "inconnu")
    log_success "Package context-mode déjà installé : $CM_VERSION"
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
elif [ "$PLUGIN_NAME" = "context-mode" ]; then
  echo "  Dans OpenCode, ouvrir un fichier volumineux : le plugin loggue 'context-mode-plugin initialized'"
  echo "  tail -f ~/.cache/opencode/logs/opencode.log | grep context-mode-plugin"
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
