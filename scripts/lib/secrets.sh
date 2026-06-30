#!/bin/bash
# secrets.sh — Stockage sécurisé des clés API (multi-OS)
# Sourced par common.sh. Ne pas exécuter directement.
# Compatible bash 3.2+
#
# Backends supportés (détection automatique, ordre de priorité) :
#   1. "env"        — CI/non-interactif : les clés sont lues depuis les variables d'env
#   2. "keychain"   — macOS : Keychain via CLI `security`
#   3. "secret-tool"— Linux avec GNOME Keyring ou KDE Wallet via D-Bus
#   4. "file"       — Fallback universel : api-keys.local.md chmod 600 (comportement actuel)
#
# Override explicite : OC_SECRET_BACKEND=keychain|secret-tool|file|env
#
# Marqueur dans api-keys.local.md quand la clé est dans le keychain :
#   api_key=__KEYCHAIN__
#
# API publique :
#   _secret_get    <project_id> <key>          → stdout (vide si absent)
#   _secret_set    <project_id> <key> <value>  → 0=OK, 1=erreur
#   _secret_delete <project_id> <key>          → 0=OK (idempotent)
#   _secret_backend                            → affiche le backend actif

# Guard double-sourcing
[ -n "${_SECRETS_LOADED:-}" ] && return 0
_SECRETS_LOADED=1

# Timeout pour les opérations keychain (en secondes)
_SECRETS_TIMEOUT=3

# Service name pour le keychain
_SECRETS_SERVICE="opencode-hub"

# ── Détection du backend ──────────────────────────────────────────────────────
_detect_secret_backend() {
  # Override explicite
  if [ -n "${OC_SECRET_BACKEND:-}" ]; then
    echo "$OC_SECRET_BACKEND"
    return 0
  fi

  # CI / non-interactif → backend file (keychain peut nécessiter interaction)
  if [ "${OC_NON_INTERACTIVE:-0}" = "1" ] || [ "${CI:-}" = "true" ]; then
    echo "file"
    return 0
  fi

  # macOS → tenter le keychain
  if [ "$(uname -s 2>/dev/null)" = "Darwin" ] && command -v security &>/dev/null; then
    echo "keychain"
    return 0
  fi

  # Linux → tenter secret-tool si D-Bus disponible
  if command -v secret-tool &>/dev/null && [ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
    echo "secret-tool"
    return 0
  fi

  # Fallback universel
  echo "file"
}

_SECRETS_BACKEND=$(_detect_secret_backend)

# ── _secret_backend ───────────────────────────────────────────────────────────
_secret_backend() {
  echo "$_SECRETS_BACKEND"
}

# ── _secret_get ───────────────────────────────────────────────────────────────
# Lit une valeur secrète pour un projet donné.
# @param $1  project_id
# @param $2  clé (ex: "api_key")
# @return    valeur sur stdout (vide si absent ou erreur)
_secret_get() {
  local project_id="$1" key="$2"
  local account="${project_id}/${key}"

  case "$_SECRETS_BACKEND" in
    keychain)
      # Timeout via subshell + kill pour éviter les blocages GUI
      local _val
      _val=$(
        (
          security find-generic-password \
            -s "$_SECRETS_SERVICE" \
            -a "$account" \
            -w 2>/dev/null
        ) &
        local _pid=$!
        sleep "$_SECRETS_TIMEOUT" && kill "$_pid" 2>/dev/null &
        wait "$_pid" 2>/dev/null
      ) || _val=""
      echo "${_val:-}"
      ;;

    secret-tool)
      secret-tool lookup \
        application "$_SECRETS_SERVICE" \
        project "$project_id" \
        key "$key" 2>/dev/null || echo ""
      ;;

    file|*)
      # Le backend file est géré directement par api-keys.sh
      # Cette fonction ne devrait pas être appelée pour le backend file
      echo ""
      ;;
  esac
}

# ── _secret_set ───────────────────────────────────────────────────────────────
# Stocke une valeur secrète pour un projet donné.
# @param $1  project_id
# @param $2  clé (ex: "api_key")
# @param $3  valeur
# @return    0=OK, 1=erreur
_secret_set() {
  local project_id="$1" key="$2" value="$3"
  local account="${project_id}/${key}"

  case "$_SECRETS_BACKEND" in
    keychain)
      security add-generic-password \
        -U \
        -s "$_SECRETS_SERVICE" \
        -a "$account" \
        -w "$value" \
        2>/dev/null
      return $?
      ;;

    secret-tool)
      echo -n "$value" | secret-tool store \
        --label="${_SECRETS_SERVICE}: ${project_id} ${key}" \
        application "$_SECRETS_SERVICE" \
        project "$project_id" \
        key "$key" \
        2>/dev/null
      return $?
      ;;

    file|*)
      # Backend file : la valeur est gérée par _write_section dans cmd-config.sh
      return 0
      ;;
  esac
}

# ── _secret_delete ────────────────────────────────────────────────────────────
# Supprime une valeur secrète (idempotent).
# @param $1  project_id
# @param $2  clé (ex: "api_key")
_secret_delete() {
  local project_id="$1" key="$2"
  local account="${project_id}/${key}"

  case "$_SECRETS_BACKEND" in
    keychain)
      security delete-generic-password \
        -s "$_SECRETS_SERVICE" \
        -a "$account" \
        2>/dev/null || true  # idempotent
      ;;

    secret-tool)
      secret-tool clear \
        application "$_SECRETS_SERVICE" \
        project "$project_id" \
        key "$key" \
        2>/dev/null || true
      ;;

    file|*)
      : # géré dans api-keys.sh
      ;;
  esac
}

# ── _secret_is_keychain_marker ────────────────────────────────────────────────
# Vérifie si une valeur lue dans api-keys.local.md est un marqueur keychain.
# @param $1  valeur lue
# @return    0 si marqueur, 1 sinon
_secret_is_keychain_marker() {
  [ "${1:-}" = "__KEYCHAIN__" ]
}
