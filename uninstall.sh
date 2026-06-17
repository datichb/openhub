#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# openhub — Script de désinstallation
#
# Usage :
#   bash uninstall.sh
#   ou : oc uninstall
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
INSTALL_DIR="${OPENCODE_HUB_DIR:-$HOME/.openhub}"

# ─────────────────────────────────────────
# COLORS & LOGGERS
# ─────────────────────────────────────────
RED='\033[91m'
GREEN='\033[92m'
YELLOW='\033[93m'
BLUE='\033[94m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

log_info()    { echo -e "${BLUE}◆${RESET}  $*"; }
log_success() { echo -e "${GREEN}◆${RESET}  $*"; }
log_warn()    { echo -e "${YELLOW}◆${RESET}  $*" >&2; }
log_error()   { echo -e "${RED}◆${RESET}  $*" >&2; }

_intro() { echo ""; echo -e "${BOLD}◆  $*${RESET}"; echo -e "${DIM}│${RESET}"; }
_outro() { echo -e "${DIM}└${RESET}  $*"; echo ""; }

# ─────────────────────────────────────────
# OS DETECTION
# ─────────────────────────────────────────
detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "macos" ;;
    *)       echo "unknown" ;;
  esac
}

OS=$(detect_os)

# ─────────────────────────────────────────
# INTRO
# ─────────────────────────────────────────
echo ""
echo -e "${BOLD}◆  Désinstallation de openhub${RESET}"
echo -e "${DIM}│${RESET}"
echo -e "${DIM}│${RESET}  Hub détecté : ${INSTALL_DIR}"
echo -e "${DIM}│${RESET}"
echo -e "${DIM}│${RESET}  Ce script va vous guider pour supprimer :"
echo -e "${DIM}│${RESET}    • Les agents déployés dans vos projets"
echo -e "${DIM}│${RESET}    • Le hub lui-même (~/.openhub)"
echo -e "${DIM}│${RESET}    • L'alias shell et les exports bun"
echo -e "${DIM}│${RESET}    • Les outils installés (opencode, beads, bun)"
echo -e "${DIM}│${RESET}"
echo -e "${DIM}│${RESET}  Chaque étape est optionnelle et demande confirmation."
echo -e "${DIM}└${RESET}"
echo ""

# ─────────────────────────────────────────
# ÉTAPE 1 — NETTOYAGE DES PROJETS DÉPLOYÉS
# ─────────────────────────────────────────
_intro "Nettoyage des agents déployés dans les projets"

PATHS_FILE="$INSTALL_DIR/projects/paths.local.md"
_projects_cleaned=0
_projects_skipped=0

if [ ! -f "$PATHS_FILE" ]; then
  log_info "Aucun fichier paths.local.md trouvé — étape ignorée"
  _outro "Aucun projet à nettoyer"
else
  # Extraire les chemins : lignes de la forme "PROJECT_ID=/chemin/absolu"
  _project_paths=()
  while IFS= read -r line; do
    # Ignorer commentaires et lignes vides
    [[ "$line" =~ ^#.*$ || -z "$line" ]] && continue
    _path="${line#*=}"
    _path="${_path%%[[:space:]]*}"  # trim trailing spaces
    [ -n "$_path" ] && _project_paths+=("$_path")
  done < "$PATHS_FILE"

  if [ "${#_project_paths[@]}" -eq 0 ]; then
    log_info "Aucun projet enregistré dans paths.local.md"
    _outro "Aucun projet à nettoyer"
  else
    echo -e "${DIM}│${RESET}  Projets enregistrés :"
    echo -e "${DIM}│${RESET}"
    _deployments_found=()
    for _proj_path in "${_project_paths[@]}"; do
      _has_deployment=false
      _items=""
      [ -d "$_proj_path/.opencode/agents" ]  && { _has_deployment=true; _items="$_items .opencode/agents/"; }
      [ -f "$_proj_path/opencode.json" ]      && { _has_deployment=true; _items="$_items opencode.json"; }

      if [ "$_has_deployment" = "true" ]; then
        echo -e "${DIM}│${RESET}    ${_proj_path}"
        echo -e "${DIM}│${RESET}      →${_items}"
        _deployments_found+=("$_proj_path")
      else
        echo -e "${DIM}│${RESET}    ${_proj_path}  ${DIM}(aucun artefact détecté)${RESET}"
      fi
    done
    echo -e "${DIM}│${RESET}"

    if [ "${#_deployments_found[@]}" -eq 0 ]; then
      log_info "Aucun artefact de déploiement détecté dans les projets"
      _outro "Projets déjà propres"
    else
      read -rp "  Supprimer les agents déployés dans les projets listés ? [y/N] : " _clean_projects </dev/tty
      if [[ "${_clean_projects:-N}" =~ ^[Yy]$ ]]; then
        for _proj_path in "${_deployments_found[@]}"; do
          [ -d "$_proj_path/.opencode/agents" ] && rm -rf "$_proj_path/.opencode/agents" \
            && log_success "Supprimé : $_proj_path/.opencode/agents/"
          [ -f "$_proj_path/opencode.json" ] && rm -f "$_proj_path/opencode.json" \
            && log_success "Supprimé : $_proj_path/opencode.json"

          _projects_cleaned=$((_projects_cleaned + 1))
        done
        _outro "$_projects_cleaned projet(s) nettoyé(s)"
      else
        _projects_skipped="${#_deployments_found[@]}"
        log_info "Nettoyage des projets ignoré"
        _outro "Projets conservés tels quels"
      fi
    fi
  fi
fi

# ─────────────────────────────────────────
# ÉTAPE 2 — SUPPRESSION DU HUB
# ─────────────────────────────────────────
_intro "Suppression du hub"

_hub_removed=false

if [ ! -d "$INSTALL_DIR" ]; then
  log_info "Hub introuvable dans $INSTALL_DIR — déjà supprimé ?"
  _outro "Rien à supprimer"
else
  # Avertir si des clés API sont présentes
  if [ -f "$INSTALL_DIR/projects/api-keys.local.md" ]; then
    log_warn "api-keys.local.md contient potentiellement des clés API — il sera supprimé définitivement."
    echo -e "${DIM}│${RESET}"
  fi

  echo -e "${DIM}│${RESET}  Ceci supprimera définitivement : ${INSTALL_DIR}"
  echo -e "${DIM}│${RESET}"
  read -rp "  Supprimer $INSTALL_DIR ? [y/N] : " _remove_hub </dev/tty
  if [[ "${_remove_hub:-N}" =~ ^[Yy]$ ]]; then
    rm -rf "$INSTALL_DIR"
    _hub_removed=true
    log_success "Hub supprimé : $INSTALL_DIR"
    _outro "Hub supprimé"
  else
    log_info "Hub conservé"
    _outro "Hub conservé dans $INSTALL_DIR"
  fi
fi

# ─────────────────────────────────────────
# ÉTAPE 3 — RETRAIT ALIAS + EXPORTS DU RC
# ─────────────────────────────────────────
_intro "Retrait de l'alias shell et des exports bun"

_rc_file=""
if [ -n "${ZSH_VERSION:-}" ] || [ "${SHELL:-}" = "/bin/zsh" ] || [ "${SHELL:-}" = "/usr/bin/zsh" ]; then
  _rc_file="$HOME/.zshrc"
elif [ -n "${BASH_VERSION:-}" ] || [ "${SHELL:-}" = "/bin/bash" ] || [ "${SHELL:-}" = "/usr/bin/bash" ]; then
  _rc_file="$HOME/.bashrc"
  [ "$OS" = "macos" ] && _rc_file="$HOME/.bash_profile"
fi

_rc_cleaned=false

if [ -z "$_rc_file" ] || [ ! -f "$_rc_file" ]; then
  log_info "Fichier rc shell introuvable ou shell non reconnu"
  _outro "Retrait alias ignoré"
else
  _has_alias=false
  _has_bun=false
  grep -qF "# openhub" "$_rc_file" 2>/dev/null && _has_alias=true
  # Couvrir aussi le cas où l'alias existe sans le commentaire
  grep -qF "openhub/oc.sh" "$_rc_file" 2>/dev/null && _has_alias=true
  grep -qF "BUN_INSTALL" "$_rc_file" 2>/dev/null && _has_bun=true

  if [ "$_has_alias" = "false" ] && [ "$_has_bun" = "false" ]; then
    log_info "Aucun alias openhub ni export bun trouvé dans $_rc_file"
    _outro "Fichier rc déjà propre"
  else
    echo -e "${DIM}│${RESET}  Fichier rc : $_rc_file"
    echo -e "${DIM}│${RESET}"
    [ "$_has_alias" = "true" ] && echo -e "${DIM}│${RESET}    • Bloc '# openhub' + alias 'oc' (ou équivalent)"
    [ "$_has_bun" = "true" ]   && echo -e "${DIM}│${RESET}    • Exports BUN_INSTALL / PATH bun"
    echo -e "${DIM}│${RESET}"

    read -rp "  Retirer ces entrées de $_rc_file ? [Y/n] : " _clean_rc </dev/tty
    if [[ "${_clean_rc:-Y}" =~ ^[Yy]$ ]]; then
      cp "$_rc_file" "$_rc_file.bak"
      if [ "$_has_alias" = "true" ]; then
        # Retirer la ligne "# openhub" et la ligne alias qui suit
        sed -i.tmp '/^# openhub$/d' "$_rc_file"
        sed -i.tmp '/openhub\/oc\.sh/d' "$_rc_file"
        rm -f "$_rc_file.tmp"
        log_success "Alias openhub retiré de $_rc_file"
      fi
      if [ "$_has_bun" = "true" ]; then
        sed -i.tmp '/BUN_INSTALL/d' "$_rc_file"
        sed -i.tmp '/\$BUN_INSTALL\/bin/d' "$_rc_file"
        rm -f "$_rc_file.tmp"
        log_success "Exports bun retirés de $_rc_file"
      fi
      # Supprimer les lignes vides consécutives en fin de fichier (nettoyage cosmétique)
      rm -f "$_rc_file.bak"
      _rc_cleaned=true
      _outro "Fichier rc nettoyé"
    else
      log_info "Fichier rc conservé tel quel"
      _outro "Fichier rc non modifié"
    fi
  fi
fi

# ─────────────────────────────────────────
# ÉTAPE 4 — OUTILS SYSTÈME (optionnel)
# ─────────────────────────────────────────
_intro "Désinstallation des outils système"

_tools_removed=()
_tools_kept=()

# ── opencode ──────────────────────────────
if command -v opencode &>/dev/null; then
  echo -e "${DIM}│${RESET}  opencode détecté : $(opencode --version 2>/dev/null || echo '?')"
  read -rp "  Désinstaller opencode (npm uninstall -g opencode-ai) ? [y/N] : " _rm_opencode </dev/tty
  if [[ "${_rm_opencode:-N}" =~ ^[Yy]$ ]]; then
    if npm uninstall -g opencode-ai 2>/dev/null; then
      log_success "opencode désinstallé"
      _tools_removed+=("opencode")
    else
      log_warn "Échec désinstallation opencode — faire manuellement : npm uninstall -g opencode-ai"
    fi
  else
    _tools_kept+=("opencode")
  fi
  echo -e "${DIM}│${RESET}"
fi

# ── beads ─────────────────────────────────
if command -v bd &>/dev/null && command -v brew &>/dev/null; then
  _bd_version=$(bd --version 2>/dev/null || bd version 2>/dev/null || echo '?')
  echo -e "${DIM}│${RESET}  Beads (bd) détecté : $_bd_version"
  read -rp "  Désinstaller Beads (brew uninstall beads) ? [y/N] : " _rm_beads </dev/tty
  if [[ "${_rm_beads:-N}" =~ ^[Yy]$ ]]; then
    if brew uninstall beads 2>/dev/null; then
      log_success "Beads désinstallé"
      _tools_removed+=("beads")
    else
      log_warn "Échec désinstallation Beads — faire manuellement : brew uninstall beads"
    fi
  else
    _tools_kept+=("beads")
  fi
  echo -e "${DIM}│${RESET}"
fi

# ── bun ───────────────────────────────────
if [ -d "$HOME/.bun" ]; then
  echo -e "${DIM}│${RESET}  bun détecté : $HOME/.bun"
  log_warn "bun peut être utilisé par d'autres outils — supprimer uniquement si installé exclusivement pour openhub."
  echo -e "${DIM}│${RESET}"
  read -rp "  Supprimer bun (~/.bun) ? [y/N] : " _rm_bun </dev/tty
  if [[ "${_rm_bun:-N}" =~ ^[Yy]$ ]]; then
    rm -rf "$HOME/.bun"
    log_success "bun supprimé (~/.bun)"
    _tools_removed+=("bun")
  else
    _tools_kept+=("bun")
  fi
  echo -e "${DIM}│${RESET}"
fi

if [ "${#_tools_removed[@]}" -eq 0 ] && [ "${#_tools_kept[@]}" -eq 0 ]; then
  log_info "Aucun outil système à désinstaller détecté"
fi

_outro "Outils système traités"

# ─────────────────────────────────────────
# RÉSUMÉ FINAL
# ─────────────────────────────────────────
echo ""
echo -e "${BOLD}◆  Désinstallation terminée${RESET}"
echo -e "${DIM}│${RESET}"

# Ce qui a été fait
if [ "$_hub_removed" = "true" ]; then
  echo -e "${DIM}│${RESET}  ${GREEN}✓${RESET}  Hub supprimé : $INSTALL_DIR"
fi
if [ "$_rc_cleaned" = "true" ]; then
  echo -e "${DIM}│${RESET}  ${GREEN}✓${RESET}  Alias et exports retirés de $_rc_file"
fi
if [ "$_projects_cleaned" -gt 0 ]; then
  echo -e "${DIM}│${RESET}  ${GREEN}✓${RESET}  $_projects_cleaned projet(s) nettoyé(s)"
fi
for _t in "${_tools_removed[@]}"; do
  echo -e "${DIM}│${RESET}  ${GREEN}✓${RESET}  $_t désinstallé"
done

echo -e "${DIM}│${RESET}"

# Ce qui reste
_has_remainder=false
if [ "$_hub_removed" = "false" ] && [ -d "$INSTALL_DIR" ]; then
  echo -e "${DIM}│${RESET}  ${YELLOW}○${RESET}  Hub conservé : $INSTALL_DIR"
  _has_remainder=true
fi
if [ "$_rc_cleaned" = "false" ] && [ -n "$_rc_file" ]; then
  echo -e "${DIM}│${RESET}  ${YELLOW}○${RESET}  Alias conservé dans $_rc_file — retirer manuellement si besoin"
  _has_remainder=true
fi
if [ "$_projects_skipped" -gt 0 ]; then
  echo -e "${DIM}│${RESET}  ${YELLOW}○${RESET}  $_projects_skipped projet(s) avec artefacts conservés"
  _has_remainder=true
fi
for _t in "${_tools_kept[@]}"; do
  echo -e "${DIM}│${RESET}  ${YELLOW}○${RESET}  $_t conservé"
  _has_remainder=true
done

if [ "$_has_remainder" = "false" ] && [ "$_hub_removed" = "true" ]; then
  echo -e "${DIM}│${RESET}  Désinstallation complète."
fi

# Recharger le shell si rc modifié
if [ "$_rc_cleaned" = "true" ] && [ -n "$_rc_file" ]; then
  echo -e "${DIM}│${RESET}"
  echo -e "${DIM}│${RESET}  Recharger votre shell pour appliquer les changements :"
  echo -e "${DIM}│${RESET}"
  echo -e "${DIM}│${RESET}    source $_rc_file"
fi

echo -e "${DIM}└${RESET}"
echo ""
