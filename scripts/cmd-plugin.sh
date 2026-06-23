#!/bin/bash
# Manage OpenCode plugins
# Usage: oc plugin install <name>
#        oc plugin remove <name>
#        oc plugin status

set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

SUBCOMMAND="${1:-}"
PLUGIN_NAME="${2:-rtk}"

OC_LOG_DIR="$HOME/.local/share/opencode/log"
OC_LOG_FILE="$OC_LOG_DIR/opencode.log"

resolve_oc_lang

# ── Sous-commandes supportées ─────────────────────────────────────────────────

if [ "$SUBCOMMAND" != "install" ] && [ "$SUBCOMMAND" != "remove" ] && [ "$SUBCOMMAND" != "status" ]; then
  log_error "$(t plugin.not_found): $SUBCOMMAND"
  log_info "$(t help.plugin_install.cmd)"
  exit 1
fi

# ── Sous-commande : status ────────────────────────────────────────────────────

if [ "$SUBCOMMAND" = "status" ]; then
  # Désactiver set -e pour cette sous-commande : les checks non-fatals peuvent
  # retourner exit code 1 (binaire absent, grep vide, etc.) sans arrêter le script.
  set +e

  log_title "$(t plugin.status_title)"

  # ── RTK ────────────────────────────────────────────────────────────────────

  echo ""
  echo "$(t plugin.status_rtk_section)"

  RTK_FILE="$HOME/.config/opencode/plugins/rtk.ts"
  if [ -f "$RTK_FILE" ]; then
    log_success "$(t plugin.status_file): $RTK_FILE"
  else
    log_error "$(t plugin.status_file_missing): $RTK_FILE"
    log_info "  → oc plugin install rtk"
  fi

  # Vérifier le binaire rtk seulement si le plugin est installé
  # (sans plugin, l'absence du binaire n'est pas un problème)
  if [ -f "$RTK_FILE" ]; then
    if command -v rtk &>/dev/null; then
      RTK_VER=$(rtk --version 2>/dev/null | awk '{print $2}' || echo "?")
      log_success "$(t plugin.status_binary): rtk $RTK_VER"
    else
      echo -e "  \033[31m◆\033[0m  $(t plugin.status_binary_missing): rtk"
      log_info "  macOS  : brew install rtk"
      log_info "  Rust   : cargo install rtk"
      log_info "  Autre  : https://www.rtk-ai.app/"
    fi
  fi

  # OpenCode charge automatiquement les fichiers .ts dans ~/.config/opencode/plugins/
  # donc si le fichier existe, il est actif — pas besoin d'entrée dans opencode.jsonc
  if [ -f "$RTK_FILE" ]; then
    log_success "$(t plugin.status_active): global (auto-load depuis plugins/)"
  fi

  # Dernières activités RTK dans les logs
  if [ -f "$OC_LOG_FILE" ]; then
    RTK_LAST=$(grep -E '"RTK plugin initialized|rtk-plugin' "$OC_LOG_FILE" 2>/dev/null \
      | grep -v "evaluated\|permission\|touching\|formatting" \
      | tail -3)
    if [ -n "$RTK_LAST" ]; then
      echo ""
      echo "  $(t plugin.status_last_logs):"
      echo "$RTK_LAST" | while IFS= read -r line; do
        # Extraire timestamp et message du format JSON
        TS=$(echo "$line" | grep -oE 'timestamp=[^ ]+' | cut -d= -f2 | cut -dT -f2 | cut -dZ -f1 || echo "")
        MSG=$(echo "$line" | grep -oE 'message="[^"]+"' | sed 's/message="//;s/"//' || echo "$line")
        EXTRAS=$(echo "$line" | grep -oE 'rtk_version=[^ ]+' | head -1 || echo "")
        if [ -n "$TS" ]; then
          printf "  •  [%s] %s %s\n" "$TS" "$MSG" "$EXTRAS"
        fi
      done
    else
      echo "  $(t plugin.status_no_logs) ($(t plugin.status_trigger_rtk))"
    fi
  else
    echo "  $(t plugin.status_log_not_found): $OC_LOG_FILE"
  fi

  # ── context-mode ───────────────────────────────────────────────────────────

  echo ""
  echo "$(t plugin.status_cm_section)"

  LOCAL_CONFIG="$HUB_DIR/.opencode/opencode.json"
  CM_IN_CONFIG=false

  if [ -f "$LOCAL_CONFIG" ] && command -v jq &>/dev/null; then
    CM_COUNT=$(jq -r '.plugin // [] | map(select(. == "context-mode")) | length' "$LOCAL_CONFIG" 2>/dev/null || echo "0")
    if [ "$CM_COUNT" -gt 0 ]; then
      log_success "$(t plugin.status_declared): $LOCAL_CONFIG"
      CM_IN_CONFIG=true
    else
      log_error "$(t plugin.status_not_declared): $LOCAL_CONFIG"
      log_info "  → oc plugin install context-mode"
    fi
  elif [ -f "$LOCAL_CONFIG" ] && grep -q '"context-mode"' "$LOCAL_CONFIG" 2>/dev/null; then
    log_success "$(t plugin.status_declared): $LOCAL_CONFIG"
    CM_IN_CONFIG=true
  else
    log_error "$(t plugin.status_not_declared): $LOCAL_CONFIG"
    log_info "  → oc plugin install context-mode"
  fi

  # Vérifier le cache OpenCode
  CM_CACHE="$HOME/.cache/opencode/packages/context-mode@latest"
  if [ -d "$CM_CACHE" ]; then
    CM_VER=$(grep '"version"' "$CM_CACHE/node_modules/context-mode/package.json" 2>/dev/null \
      | head -1 | sed 's/.*"version": "\(.*\)".*/\1/' || echo "?")
    log_success "$(t plugin.status_cached): context-mode@$CM_VER"
    log_info "  $(t plugin.status_cache_path): $CM_CACHE"
  else
    if [ "$CM_IN_CONFIG" = "true" ]; then
      log_warn "$(t plugin.status_cache_missing)"
      log_info "  $(t plugin.status_cache_install_on_start)"
    fi
  fi

  # Dernières activités context-mode dans les logs
  if [ -f "$OC_LOG_FILE" ]; then
    CM_LAST=$(grep -iE 'context.mode|context-mode' "$OC_LOG_FILE" 2>/dev/null \
      | grep -v "evaluated\|permission\|touching\|formatting\|config\|path=\|spec=\|source=\|resolved path\|loading" \
      | tail -3)
    if [ -n "$CM_LAST" ]; then
      echo ""
      echo "  $(t plugin.status_last_logs):"
      echo "$CM_LAST" | while IFS= read -r line; do
        TS=$(echo "$line" | grep -oE 'timestamp=[^ ]+' | cut -d= -f2 | cut -dT -f2 | cut -dZ -f1 || echo "")
        MSG=$(echo "$line" | grep -oE 'message="[^"]+"' | sed 's/message="//;s/"//' || echo "$line" | cut -c1-80)
        if [ -n "$TS" ]; then
          printf "  •  [%s] %s\n" "$TS" "$MSG"
        fi
      done
    else
      echo "  $(t plugin.status_no_logs) ($(t plugin.status_trigger_cm))"
    fi
  fi

  # ── Config OpenCode résolue ─────────────────────────────────────────────────

  echo ""
  echo "$(t plugin.status_config_section)"

  if command -v opencode &>/dev/null; then
    OC_VER=$(opencode --version 2>/dev/null || echo "?")
    log_success "OpenCode $OC_VER"

    if command -v jq &>/dev/null; then
      RESOLVED=$(cd "$HUB_DIR" && opencode debug config 2>/dev/null)
      if [ -n "$RESOLVED" ]; then
        ACTIVE_PLUGINS=$(echo "$RESOLVED" | jq -r '.plugin // [] | .[]' 2>/dev/null || echo "")
        ORIGINS=$(echo "$RESOLVED" | jq -r '.plugin_origins // [] | .[] | "  \(.spec)  [\(.scope)]"' 2>/dev/null || echo "")
        if [ -n "$ACTIVE_PLUGINS" ]; then
          echo "  $(t plugin.status_active_plugins):"
          echo "$ACTIVE_PLUGINS" | while IFS= read -r p; do
            echo "  ✓ $p"
          done
          if [ -n "$ORIGINS" ]; then
            echo ""
            echo "  $(t plugin.status_origins):"
            echo "$ORIGINS"
          fi
        else
          log_warn "$(t plugin.status_no_active_plugins)"
        fi
      fi
    fi

    # Démarrage
    STARTUP_MS=$(cd "$HUB_DIR" && opencode debug startup 2>/dev/null | grep -oE '^[0-9]+' || echo "?")
    if [ "$STARTUP_MS" != "?" ]; then
      echo ""
      log_info "$(t plugin.status_startup): ${STARTUP_MS}ms"
      if [ "$STARTUP_MS" -lt 1000 ] 2>/dev/null; then
        log_success "$(t plugin.status_startup_ok)"
      elif [ "$STARTUP_MS" -lt 3000 ] 2>/dev/null; then
        log_info "$(t plugin.status_startup_normal)"
      else
        log_warn "$(t plugin.status_startup_slow)"
      fi
    fi
  else
    log_error "$(t plugin.opencode_not_installed)"
  fi

  # ── Fichier de log ──────────────────────────────────────────────────────────

  echo ""
  echo "  $(t plugin.status_log_path): $OC_LOG_FILE"
  echo "  $(t plugin.status_log_cmd): tail -f $OC_LOG_FILE | grep -E 'rtk-plugin|context-mode'"

  echo ""
  log_success "$(t plugin.status_done)"
  exit 0
fi

# ── Sous-commande : remove ────────────────────────────────────────────────────

if [ "$SUBCOMMAND" = "remove" ]; then
  log_title "$(t plugin.remove_title): $PLUGIN_NAME"

  if [ "$PLUGIN_NAME" = "context-mode" ]; then
    # Retirer du opencode.json local du hub
    LOCAL_CONFIG="$HUB_DIR/.opencode/opencode.json"
    if [ ! -f "$LOCAL_CONFIG" ]; then
      log_warn "$(t plugin.config_not_found): $LOCAL_CONFIG"
      exit 0
    fi

    if command -v jq &>/dev/null; then
      UPDATED=$(jq 'if .plugin then .plugin = (.plugin | map(select(. != "context-mode"))) | if (.plugin | length) == 0 then del(.plugin) else . end else . end' "$LOCAL_CONFIG")
      echo "$UPDATED" > "$LOCAL_CONFIG"
      log_success "$(t plugin.removed): context-mode"
    else
      log_error "jq requis pour modifier opencode.json"
      log_info "$(t plugin.remove_manual): retirer 'context-mode' du tableau 'plugin' dans $LOCAL_CONFIG"
      exit 1
    fi

  elif [ "$PLUGIN_NAME" = "rtk" ]; then
    PLUGIN_TARGET="$HOME/.config/opencode/plugins/rtk.ts"
    if [ -f "$PLUGIN_TARGET" ]; then
      rm "$PLUGIN_TARGET"
      log_success "$(t plugin.removed): rtk"
    else
      log_warn "$(t plugin.not_installed): rtk"
    fi

  else
    log_error "$(t plugin.not_found): $PLUGIN_NAME"
    exit 1
  fi

  log_success "$(t plugin.done)"
  exit 0
fi

# ── Sous-commande : install ───────────────────────────────────────────────────

log_title "$(t plugin.install_title): $PLUGIN_NAME"

# ── Vérifications préalables ──────────────────────────────────────────────────

# 1. Vérifier que le plugin est connu
KNOWN_PLUGINS=("rtk" "context-mode")
PLUGIN_KNOWN=false
for known in "${KNOWN_PLUGINS[@]}"; do
  [ "$PLUGIN_NAME" = "$known" ] && PLUGIN_KNOWN=true && break
done

if [ "$PLUGIN_KNOWN" = "false" ]; then
  log_error "$(t plugin.not_found): $PLUGIN_NAME"
  log_info "$(t plugin.available):"
  for known in "${KNOWN_PLUGINS[@]}"; do
    echo "  - $known"
  done
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

# ── Helpers locaux ───────────────────────────────────────────────────────────

# Affiche les instructions d'installation manuelle de RTK
_rtk_manual_install_instructions() {
  log_info "  macOS  : brew install rtk"
  log_info "  Rust   : cargo install rtk"
  log_info "  Autre  : https://www.rtk-ai.app/"
}

# Installe le binaire RTK via l'installeur disponible
# Retourne 0 si installé avec succès, 1 sinon
_rtk_install_binary() {
  if command -v brew &>/dev/null; then
    log_info "$(t plugin.rtk_installing_brew)..."
    if brew install rtk; then
      log_success "$(t plugin.rtk_installed_binary)"
      return 0
    else
      log_error "$(t plugin.rtk_brew_failed)"
      return 1
    fi
  elif command -v cargo &>/dev/null; then
    log_info "$(t plugin.rtk_installing_cargo)..."
    if cargo install rtk; then
      log_success "$(t plugin.rtk_installed_binary)"
      return 0
    else
      log_error "$(t plugin.rtk_cargo_failed)"
      return 1
    fi
  else
    log_error "$(t plugin.rtk_no_installer)"
    return 1
  fi
}

# Met à jour le binaire RTK via l'installeur disponible
# Retourne 0 si mis à jour avec succès, 1 sinon
_rtk_upgrade_binary() {
  if command -v brew &>/dev/null; then
    log_info "$(t plugin.rtk_installing_brew)..."
    if brew upgrade rtk; then
      log_success "$(t plugin.rtk_installed_binary)"
      return 0
    else
      log_error "$(t plugin.rtk_brew_failed)"
      return 1
    fi
  elif command -v cargo &>/dev/null; then
    log_info "$(t plugin.rtk_installing_cargo)..."
    if cargo install rtk; then
      log_success "$(t plugin.rtk_installed_binary)"
      return 0
    else
      log_error "$(t plugin.rtk_cargo_failed)"
      return 1
    fi
  else
    log_error "$(t plugin.rtk_no_installer)"
    return 1
  fi
}

# ── Installation : RTK (plugin fichier local) ─────────────────────────────────

if [ "$PLUGIN_NAME" = "rtk" ]; then
  PLUGIN_SOURCE="$HUB_DIR/plugins/rtk/rtk.ts"
  PLUGIN_TARGET="$HOME/.config/opencode/plugins/rtk.ts"

  if [ ! -f "$PLUGIN_SOURCE" ]; then
    log_error "$(t plugin.not_found): $PLUGIN_NAME (source manquante: $PLUGIN_SOURCE)"
    exit 1
  fi

  # Vérifier que RTK est installé — proposer l'installation si absent
  if ! command -v rtk &>/dev/null; then
    log_warn "$(t plugin.rtk_not_installed)"
    echo ""

    if [[ "${OC_NON_INTERACTIVE:-0}" != "1" ]]; then
      read -p "$(t plugin.rtk_install_prompt) [y/N] " -n 1 -r
      echo
    else
      REPLY="N"
    fi

    if [[ $REPLY =~ ^[Yy]$ ]]; then
      if ! _rtk_install_binary; then
        echo ""
        log_info "$(t plugin.rtk_install_later):"
        _rtk_manual_install_instructions
        exit 1
      fi
    else
      log_info "$(t plugin.rtk_install_later):"
      _rtk_manual_install_instructions
      exit 1
    fi
  fi

  # Vérifier la version de RTK
  RTK_VERSION=$(rtk --version 2>/dev/null | awk '{print $2}' || echo "0.0.0")
  REQUIRED_VERSION="0.42.0"

  if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$RTK_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    log_warn "$(t plugin.rtk_outdated): $RTK_VERSION < $REQUIRED_VERSION"
    echo ""

    if [[ "${OC_NON_INTERACTIVE:-0}" != "1" ]]; then
      read -p "$(t plugin.rtk_upgrade_prompt) [y/N] " -n 1 -r
      echo
    else
      REPLY="N"
    fi

    if [[ $REPLY =~ ^[Yy]$ ]]; then
      if ! _rtk_upgrade_binary; then
        echo ""
        log_info "$(t plugin.rtk_install_later):"
        _rtk_manual_install_instructions
        echo ""
        read -p "$(t plugin.continue_anyway) [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
          exit 1
        fi
      fi
    else
      log_info "$(t plugin.upgrade_rtk):"
      _rtk_manual_install_instructions
      echo ""
      read -p "$(t plugin.continue_anyway) [y/N] " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
      fi
    fi

    # Re-vérifier la version après mise à jour éventuelle
    RTK_VERSION=$(rtk --version 2>/dev/null | awk '{print $2}' || echo "0.0.0")
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$RTK_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
      log_success "$(t plugin.rtk_version_ok): $RTK_VERSION"
    fi
  else
    log_success "$(t plugin.rtk_version_ok): $RTK_VERSION"
  fi

  # Créer le dossier de destination si nécessaire
  mkdir -p "$HOME/.config/opencode/plugins"

  # Sauvegarder l'ancien plugin si existant (dans .backup/ pour ne pas être chargé par OpenCode)
  if [ -f "$PLUGIN_TARGET" ]; then
    BACKUP_DIR="$HOME/.config/opencode/plugins/.backup"
    mkdir -p "$BACKUP_DIR"
    BACKUP="$BACKUP_DIR/rtk.ts.$(date +%Y%m%d-%H%M%S)"
    log_info "$(t plugin.backing_up): $BACKUP"
    cp "$PLUGIN_TARGET" "$BACKUP"
  fi

  # Copier le plugin
  log_info "$(t plugin.copying)..."
  cp "$PLUGIN_SOURCE" "$PLUGIN_TARGET"

  log_success "$(t plugin.installed): $PLUGIN_TARGET"

  # ── Vérification post-installation ──────────────────────────────────────────
  echo ""
  log_title "$(t plugin.verification)"
  if [ -f "$PLUGIN_TARGET" ]; then
    log_success "$(t plugin.file_present)"
    PERMS=$(ls -lh "$PLUGIN_TARGET" | awk '{print $1, $3, $9}')
    log_info "$(t plugin.permissions): $PERMS"
  else
    log_error "$(t plugin.file_missing)"
    exit 1
  fi

  # ── Instructions suivantes ───────────────────────────────────────────────────
  echo ""
  log_title "$(t plugin.next_steps)"
  echo "$(t plugin.step1):"
  echo "  $(t plugin.restart_opencode)"
  echo ""
  echo "$(t plugin.step2):"
  echo "  tail -f $OC_LOG_FILE | grep rtk-plugin"
  echo ""
  echo "$(t plugin.step3):"
  echo "  $(t plugin.rtk_test_command)"

  # Proposer d'afficher la documentation
  echo ""
  if [[ "${OC_NON_INTERACTIVE:-0}" != "1" ]]; then
    read -p "$(t plugin.show_docs) [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      if [ -f "$HUB_DIR/plugins/rtk/README.md" ]; then
        cat "$HUB_DIR/plugins/rtk/README.md"
      else
        log_warn "$(t plugin.no_docs)"
      fi
    fi
  fi

  log_success "$(t plugin.done)"
  exit 0
fi

# ── Installation : context-mode (npm plugin natif OpenCode) ───────────────────

if [ "$PLUGIN_NAME" = "context-mode" ]; then
  # context-mode est un package npm publié. OpenCode supporte les npm plugins
  # déclarés dans opencode.json via "plugin": ["context-mode"].
  # OpenCode installe et gère le package automatiquement via Bun natif au
  # démarrage — pas de wrapper .ts ni de bun add manuel nécessaire.
  #
  # Cette commande ajoute simplement "context-mode" au tableau "plugin" dans
  # .opencode/opencode.json (config locale du hub).

  LOCAL_CONFIG="$HUB_DIR/.opencode/opencode.json"

  log_info "$(t plugin.config_updating)"

  # Créer opencode.json si absent
  if [ ! -f "$LOCAL_CONFIG" ]; then
    echo '{"$schema":"https://opencode.ai/config.json","plugin":[]}' > "$LOCAL_CONFIG"
    log_info "$(t plugin.config_created): $LOCAL_CONFIG"
  fi

  # Vérifier si context-mode est déjà présent
  if command -v jq &>/dev/null; then
    ALREADY=$(jq -r '.plugin // [] | map(select(. == "context-mode")) | length' "$LOCAL_CONFIG")
    if [ "$ALREADY" -gt 0 ]; then
      log_success "$(t plugin.already_registered): context-mode"
    else
      # Ajouter context-mode au tableau plugin
      UPDATED=$(jq '.plugin = ((.plugin // []) + ["context-mode"])' "$LOCAL_CONFIG")
      echo "$UPDATED" > "$LOCAL_CONFIG"
      log_success "$(t plugin.config_updated): context-mode"
    fi
  else
    # Fallback sans jq : vérifier manuellement
    if grep -q '"context-mode"' "$LOCAL_CONFIG" 2>/dev/null; then
      log_success "$(t plugin.already_registered): context-mode"
    else
      log_error "jq requis pour modifier opencode.json automatiquement"
      log_info "$(t plugin.config_manual_add):"
      echo "  Ajouter \"context-mode\" au tableau \"plugin\" dans $LOCAL_CONFIG"
      echo "  Exemple : { \"plugin\": [\"context-mode\"] }"
      exit 1
    fi
  fi

  # ── Instructions suivantes ───────────────────────────────────────────────────
  echo ""
  log_title "$(t plugin.next_steps)"
  echo "$(t plugin.step1):"
  echo "  $(t plugin.restart_opencode)"
  log_info "$(t plugin.config_auto_install)"
  echo ""
  echo "$(t plugin.step2):"
  echo "  tail -f $OC_LOG_FILE | grep context-mode"
  echo ""
  echo "$(t plugin.step3):"
  echo "  $(t plugin.context_mode_test)"

  # Proposer d'afficher la documentation
  echo ""
  read -p "$(t plugin.show_docs) [y/N] " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [ -f "$HUB_DIR/plugins/context-mode/README.md" ]; then
      cat "$HUB_DIR/plugins/context-mode/README.md"
    else
      log_warn "$(t plugin.no_docs)"
    fi
  fi

  log_success "$(t plugin.done)"
  exit 0
fi
