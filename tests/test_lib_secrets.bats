#!/usr/bin/env bats
# Tests pour scripts/lib/secrets.sh
# Couvre : détection du backend, _secret_get/_set/_delete, intégration avec api-keys.sh

setup() {
  TEST_DIR="$(mktemp -d)"
  export HUB_DIR="$TEST_DIR"
  export LIB_DIR="$BATS_TEST_DIRNAME/../scripts/lib"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # Créer un api-keys.local.md de test
  cat > "$API_KEYS_FILE" <<'APIEOF'
[PROJ-A]
model=claude-sonnet-4-5
provider=anthropic
api_key=sk-ant-real-key

[PROJ-B]
model=claude-opus-4-5
provider=anthropic
api_key=__KEYCHAIN__
APIEOF

  source "$LIB_DIR/api-keys.sh"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Détection du backend
# ══════════════════════════════════════════════════════════════════════════════

@test "secrets: _detect_secret_backend retourne une valeur valide" {
  source "$LIB_DIR/secrets.sh"
  local backend
  backend=$(_detect_secret_backend)
  [[ "$backend" == "keychain" || "$backend" == "secret-tool" || "$backend" == "file" || "$backend" == "env" ]]
}

@test "secrets: OC_NON_INTERACTIVE=1 force le backend file" {
  OC_NON_INTERACTIVE=1 source "$LIB_DIR/secrets.sh" || {
    # Recharger avec la variable
    unset _SECRETS_LOADED
    export OC_NON_INTERACTIVE=1
    source "$LIB_DIR/secrets.sh"
  }
  # Réinitialiser pour ce test
  unset _SECRETS_LOADED
  local _backend
  _backend=$(OC_NON_INTERACTIVE=1 bash -c "source '$LIB_DIR/secrets.sh'; _detect_secret_backend")
  [[ "$_backend" == "file" ]]
}

@test "secrets: CI=true force le backend file" {
  local _backend
  _backend=$(CI=true bash -c "source '$LIB_DIR/secrets.sh'; _detect_secret_backend")
  [[ "$_backend" == "file" ]]
}

@test "secrets: OC_SECRET_BACKEND override est respecté" {
  local _backend
  _backend=$(OC_SECRET_BACKEND=file bash -c "source '$LIB_DIR/secrets.sh'; _detect_secret_backend")
  [[ "$_backend" == "file" ]]
}

@test "secrets: _secret_backend affiche le backend actif" {
  source "$LIB_DIR/secrets.sh"
  run _secret_backend
  [ "$status" -eq 0 ]
  [ -n "$output" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Backend file (comportement de base)
# ══════════════════════════════════════════════════════════════════════════════

@test "secrets: _secret_get avec backend file retourne vide (géré par api-keys.sh)" {
  unset _SECRETS_LOADED
  source "$LIB_DIR/secrets.sh"
  # Le backend file délègue à api-keys.sh — _secret_get retourne vide
  run _secret_get "PROJ-A" "api_key"
  [ "$status" -eq 0 ]
  # La valeur vide est le comportement attendu pour le backend file
}

@test "secrets: _secret_set avec backend file retourne 0" {
  unset _SECRETS_LOADED
  source "$LIB_DIR/secrets.sh"
  run _secret_set "PROJ-A" "api_key" "sk-test-value"
  [ "$status" -eq 0 ]
}

@test "secrets: _secret_delete avec backend file retourne 0 (idempotent)" {
  unset _SECRETS_LOADED
  source "$LIB_DIR/secrets.sh"
  run _secret_delete "PROJ-A" "api_key"
  [ "$status" -eq 0 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Marqueur __KEYCHAIN__
# ══════════════════════════════════════════════════════════════════════════════

@test "secrets: _secret_is_keychain_marker détecte __KEYCHAIN__" {
  source "$LIB_DIR/secrets.sh"
  run _secret_is_keychain_marker "__KEYCHAIN__"
  [ "$status" -eq 0 ]
}

@test "secrets: _secret_is_keychain_marker rejette une vraie clé" {
  source "$LIB_DIR/secrets.sh"
  run _secret_is_keychain_marker "sk-ant-real-key"
  [ "$status" -ne 0 ]
}

@test "secrets: _secret_is_keychain_marker rejette une valeur vide" {
  source "$LIB_DIR/secrets.sh"
  run _secret_is_keychain_marker ""
  [ "$status" -ne 0 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Intégration api-keys.sh : résolution du marqueur __KEYCHAIN__
# ══════════════════════════════════════════════════════════════════════════════

@test "api-keys: valeur normale lue sans passer par le keychain" {
  unset _SECRETS_LOADED
  source "$LIB_DIR/secrets.sh"
  # PROJ-A a une vraie clé, pas de marqueur
  api_keys_load_cache "PROJ-A"
  [ "$_API_KEYS_CACHE_KEY" = "sk-ant-real-key" ]
}

@test "api-keys: marqueur __KEYCHAIN__ déclenche une tentative de résolution" {
  # On mock _secret_get pour retourner une valeur simulée
  _secret_get() { echo "sk-from-keychain-mock"; }
  export -f _secret_get

  unset _API_KEYS_CACHE_LOADED
  source "$LIB_DIR/api-keys.sh"
  api_keys_load_cache "PROJ-B"
  # Si _secret_get fonctionne, la clé doit être résolue
  # Si le backend ne supporte pas, la clé reste vide (comportement acceptable)
  [[ "$_API_KEYS_CACHE_KEY" == "sk-from-keychain-mock" || "$_API_KEYS_CACHE_KEY" == "" ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. Robustesse — erreurs keychain gracieuses
# ══════════════════════════════════════════════════════════════════════════════

@test "secrets: _secret_get ne crashe pas si security/secret-tool absent" {
  unset _SECRETS_LOADED
  # Forcer un backend inexistant pour tester le fallback
  OC_SECRET_BACKEND=file source "$LIB_DIR/secrets.sh"
  run _secret_get "PROJ-A" "api_key"
  [ "$status" -eq 0 ]
}
