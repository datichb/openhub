#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# release.sh — Créer une release versionnée de opencode-hub
#
# Usage : bash scripts/release.sh <version> [--dry-run]
#   ex  : bash scripts/release.sh 1.1.0
#   ex  : bash scripts/release.sh 1.1.0 --dry-run
#
# Ce script :
#   1. Valide le format X.Y.Z
#   2. Vérifie que le working tree est propre et qu'on est sur main
#   3. Met à jour config/hub.json (.version) — local, non tracké
#      et config/hub.json.example (.version) — tracké par git
#   3.5. Met à jour CHANGELOG.md : insère ## [X.Y.Z] — date sous [Unreleased]
#   4. Crée le commit "chore(release): vX.Y.Z" et le tag annoté vX.Y.Z
#   5. Propose de pusher (ou affiche la commande à lancer manuellement)
#
# Flags :
#   --dry-run   Affiche ce qui serait fait sans rien modifier ni committer
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HUB_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
HUB_CONFIG="$HUB_DIR/config/hub.json"
HUB_CONFIG_EXAMPLE="$HUB_DIR/config/hub.json.example"

# ── Couleurs ──────────────────────────────────────────────────────────────────
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
log_title()   { echo -e "\n${BOLD}$*${RESET}"; }

# ── Usage ─────────────────────────────────────────────────────────────────────
if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  echo -e "${BOLD}Usage :${RESET} bash scripts/release.sh <version> [--dry-run]"
  echo ""
  echo "  <version>   Numéro de version au format X.Y.Z (ex : 1.1.0)"
  echo "  --dry-run   Affiche ce qui serait fait sans modifier ni committer"
  echo ""
  echo "Ce script :"
  echo "  1. Valide le format X.Y.Z"
  echo "  2. Vérifie que le working tree est propre et qu'on est sur main"
  echo "  3. Met à jour config/hub.json (.version) et config/hub.json.example (.version)"
  echo "  3.5. Met à jour CHANGELOG.md : insère [X.Y.Z] — date sous [Unreleased]"
  echo "  4. Crée le commit chore(release): vX.Y.Z + le tag annoté vX.Y.Z"
  echo "  5. Propose de pusher ou affiche la commande"
  exit 0
fi

# ── Argument + flags ──────────────────────────────────────────────────────────
VERSION="${1:-}"
DRY_RUN=false

# Parcourir tous les arguments pour détecter --dry-run
for _arg in "$@"; do
  if [ "$_arg" = "--dry-run" ]; then
    DRY_RUN=true
  fi
done

# Retirer --dry-run de VERSION si passé en premier
VERSION="${VERSION#v}"
VERSION="${VERSION/--dry-run/}"
VERSION="${VERSION// /}"

if [ -z "$VERSION" ]; then
  log_error "Version manquante."
  echo -e "  Usage : bash scripts/release.sh <version>  (ex : 1.1.0)"
  exit 1
fi

if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  log_error "Format invalide : '$VERSION' — attendu X.Y.Z (ex : 1.1.0)"
  exit 1
fi

TAG="v${VERSION}"

if [ "$DRY_RUN" = true ]; then
  log_title "◆  [DRY-RUN] Release opencode-hub ${TAG}"
else
  log_title "◆  Release opencode-hub ${TAG}"
fi
echo ""

# ── Vérifications préalables ──────────────────────────────────────────────────

# jq requis
if ! command -v jq &>/dev/null; then
  log_error "jq est requis pour mettre à jour config/hub.json"
  exit 1
fi

# Repo git
if ! git -C "$HUB_DIR" rev-parse --git-dir &>/dev/null; then
  log_error "Le dossier $HUB_DIR n'est pas un repo git"
  exit 1
fi

# Branche main (ignorée en dry-run)
current_branch=$(git -C "$HUB_DIR" rev-parse --abbrev-ref HEAD)
if [ "$current_branch" != "main" ] && [ "$DRY_RUN" = false ]; then
  log_error "Vous n'êtes pas sur main (branche actuelle : $current_branch)"
  log_info  "Basculer sur main : git checkout main"
  exit 1
fi

# Working tree propre (ignoré en dry-run)
if [ -n "$(git -C "$HUB_DIR" status --porcelain)" ] && [ "$DRY_RUN" = false ]; then
  log_error "Working tree non propre — committer ou stasher les modifications avant de releaser"
  git -C "$HUB_DIR" status --short
  exit 1
fi

# Tag non existant (ignoré en dry-run)
if git -C "$HUB_DIR" tag --list | grep -qx "$TAG" && [ "$DRY_RUN" = false ]; then
  log_error "Le tag $TAG existe déjà"
  exit 1
fi

# hub.json présent
if [ ! -f "$HUB_CONFIG" ]; then
  log_error "config/hub.json introuvable"
  exit 1
fi

# Version actuelle
current_version=$(jq -r '.version // "inconnue"' "$HUB_CONFIG")
log_info "Version actuelle dans hub.json : ${current_version}"
log_info "Nouvelle version                : ${VERSION}"
echo ""

# ── Dry-run : afficher ce qui serait fait ─────────────────────────────────────
if [ "$DRY_RUN" = true ]; then
  CHANGELOG="$HUB_DIR/CHANGELOG.md"
  TODAY=$(date +%Y-%m-%d)

  log_title "Changements qui seraient appliqués :"
  echo ""
  echo -e "  ${BOLD}config/hub.json${RESET}         : version ${current_version} → ${VERSION}"
  echo -e "  ${BOLD}config/hub.json.example${RESET} : version ${current_version} → ${VERSION}"
  if [ -f "$CHANGELOG" ]; then
    echo -e "  ${BOLD}CHANGELOG.md${RESET}            : insertion de ## [${VERSION}] — ${TODAY} sous [Unreleased]"
  else
    echo -e "  ${BOLD}CHANGELOG.md${RESET}            : introuvable — étape ignorée"
  fi
  echo ""
  echo -e "  ${BOLD}git commit${RESET}  : chore(release): ${TAG}"
  echo -e "  ${BOLD}git tag${RESET}     : ${TAG} (annoté)"
  echo ""
  log_warn "Mode dry-run — aucune modification effectuée."
  exit 0
fi

# ── Confirmation ──────────────────────────────────────────────────────────────
read -rp "  Créer la release ${TAG} ? [Y/n] : " _confirm </dev/tty
if ! [[ "${_confirm:-Y}" =~ ^[Yy]$ ]]; then
  echo ""
  log_warn "Release annulée."
  exit 0
fi

echo ""

# ── Mise à jour de hub.json + hub.json.example ───────────────────────────────
tmp=$(mktemp)
jq --arg v "$VERSION" '.version = $v' "$HUB_CONFIG" > "$tmp"
mv "$tmp" "$HUB_CONFIG"
log_success "config/hub.json         mis à jour → version = ${VERSION}"

tmp=$(mktemp)
jq --arg v "$VERSION" '.version = $v' "$HUB_CONFIG_EXAMPLE" > "$tmp"
mv "$tmp" "$HUB_CONFIG_EXAMPLE"
log_success "config/hub.json.example mis à jour → version = ${VERSION}"

# ── Étape 3.5 — Mise à jour de CHANGELOG.md ──────────────────────────────────
CHANGELOG="$HUB_DIR/CHANGELOG.md"

if [ ! -f "$CHANGELOG" ]; then
  log_warn "CHANGELOG.md introuvable — étape ignorée"
else
  TODAY=$(date +%Y-%m-%d)
  # Insérer ## [X.Y.Z] — date juste après la ligne ## [Unreleased]
  # perl est disponible sur macOS et Linux — gère les sauts de ligne multi-plateforme
  perl -i -0pe "s|^(## \[Unreleased\])|\\1\n\n---\n\n## [${VERSION}] — ${TODAY}|m" "$CHANGELOG"
  log_success "CHANGELOG.md               mis à jour → ## [${VERSION}] — ${TODAY}"
fi

# ── Commit + tag ──────────────────────────────────────────────────────────────
git -C "$HUB_DIR" add config/hub.json.example
[ -f "$HUB_DIR/CHANGELOG.md" ] && git -C "$HUB_DIR" add CHANGELOG.md
git -C "$HUB_DIR" commit -m "chore(release): ${TAG}"
log_success "Commit créé : chore(release): ${TAG}"

git -C "$HUB_DIR" tag -a "$TAG" -m "Release ${TAG}"
log_success "Tag annoté créé : ${TAG}"

echo ""

# ── Push ──────────────────────────────────────────────────────────────────────
echo -e "${DIM}────────────────────────────────────────────────${RESET}"
read -rp "  Pusher maintenant ? (git push && git push --tags) [Y/n] : " _push </dev/tty
echo ""

if [[ "${_push:-Y}" =~ ^[Yy]$ ]]; then
  git -C "$HUB_DIR" push
  git -C "$HUB_DIR" push --tags
  echo ""
  log_success "Release ${TAG} publiée."
  echo ""
  log_info "One-liner pour cette version :"
  echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=${TAG} bash${RESET}"
else
  log_warn "Push non effectué — lancer manuellement :"
  echo ""
  echo -e "  ${BOLD}git push && git push --tags${RESET}"
  echo ""
  log_info "One-liner pour cette version (disponible après le push) :"
  echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=${TAG} bash${RESET}"
fi
