#!/usr/bin/env bats
# Tests unitaires et d'intégration pour scripts/lib/provider-warnings.sh
#
# Couvre :
#   - _validate_provider_config (Approche C — cohérence model ↔ provider bloc)
#   - _validate_provider_connectivity (Approche A — pre-flight curl)
#   - _display_provider_status (affichage bloc contextuel)
#   - _warn_provider_if_needed (warning minimal pour les autres cmds)

load helpers

# ─────────────────────────────────────────────────────────────────────────────
# Setup / teardown
# ─────────────────────────────────────────────────────────────────────────────

setup() {
  common_setup

  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export OC_LANG=fr

  # Sourcer common.sh pour charger i18n, providers, etc.
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Surcharger les chemins de config pour pointer vers TEST_DIR
  export PROVIDERS_FILE="$TEST_DIR/config/providers.json"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  mkdir -p "$TEST_DIR/config"

  # providers.json minimal de test
  cat > "$PROVIDERS_FILE" <<'PEOF'
{
  "providers": {
    "anthropic": {
      "label": "Anthropic",
      "requires_api_key": true,
      "requires_base_url": false,
      "litellm": false,
      "default_model": "claude-sonnet-4-6",
      "opencode_prefix": "anthropic"
    },
    "mammouth": {
      "label": "MammouthAI",
      "requires_api_key": true,
      "requires_base_url": true,
      "litellm": true,
      "default_base_url": "https://api.mammouth.ai/v1",
      "default_model": "claude-sonnet-4-6",
      "opencode_prefix": null
    },
    "bedrock": {
      "label": "AWS Bedrock",
      "requires_api_key": false,
      "auth_method": "aws",
      "requires_base_url": false,
      "requires_region": true,
      "litellm": false,
      "default_region": "eu-west-3",
      "default_model": "claude-sonnet-4-6",
      "opencode_prefix": "amazon-bedrock"
    },
    "github-copilot": {
      "label": "GitHub Copilot",
      "requires_api_key": false,
      "auth_method": "oauth",
      "requires_base_url": false,
      "litellm": false,
      "default_model": "claude-sonnet-4-6",
      "opencode_prefix": "github-copilot"
    },
    "openrouter": {
      "label": "OpenRouter",
      "requires_api_key": true,
      "requires_base_url": false,
      "litellm": false,
      "default_model": "anthropic/claude-sonnet-4-6",
      "opencode_prefix": "openrouter"
    },
    "ollama": {
      "label": "Ollama (local)",
      "requires_api_key": false,
      "requires_base_url": true,
      "litellm": true,
      "default_base_url": "http://localhost:11434/v1",
      "default_model": "llama3.2",
      "opencode_prefix": null
    }
  }
}
PEOF

  # hub.json minimal sans provider configuré
  cat > "$HUB_CONFIG" <<'HEOF'
{
  "opencode": {
    "model": "claude-sonnet-4-6"
  }
}
HEOF

  # Mocks des fonctions de log
  mock_log_functions

  # Sourcer provider-warnings.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh"

  # Forcer le mode TTY pour les tests de connectivité
  # (BATS redirige stdout, ce qui fait échouer le check [ -t 1 ])
  export _PW_FORCE_TTY=1

  # Variables d'env AWS vides pour éviter interférences
  unset AWS_BEARER_TOKEN_BEDROCK AWS_ACCESS_KEY_ID AWS_PROFILE AWS_SECRET_ACCESS_KEY 2>/dev/null || true
}

teardown() {
  common_teardown
  unset AWS_BEARER_TOKEN_BEDROCK AWS_ACCESS_KEY_ID AWS_PROFILE 2>/dev/null || true
}

# ─────────────────────────────────────────────────────────────────────────────
# _validate_provider_config — Approche C
# ─────────────────────────────────────────────────────────────────────────────

@test "validate_provider_config : OK si fichier absent (skip silencieux)" {
  run _validate_provider_config "/tmp/inexistant.json" "amazon-bedrock/model" "bedrock"
  [ "$status" -eq 0 ]
  [ -z "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : OK si model sans préfixe (pas d'orphelin possible)" {
  echo '{"model": "claude-sonnet-4-6"}' > "$TEST_DIR/opencode.json"
  run _validate_provider_config "$TEST_DIR/opencode.json" "claude-sonnet-4-6" "mammouth"
  [ "$status" -eq 0 ]
  [ -z "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : OK si model prefix correspond au bloc provider" {
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-6",
  "provider": {
    "amazon-bedrock": {"options": {"region": "eu-west-3"}}
  }
}
EOF
  _validate_provider_config "$TEST_DIR/opencode.json" "amazon-bedrock/anthropic.claude-sonnet-4-6" "bedrock"
  [ $? -eq 0 ]
  [ -z "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : FAIL si modèle orphelin (prefix sans bloc provider)" {
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-6"
}
EOF
  # Appel direct (pas via run) pour pouvoir lire _PW_CONFIG_WARNING et l'exit code
  _validate_provider_config "$TEST_DIR/opencode.json" "amazon-bedrock/anthropic.claude-sonnet-4-6" "bedrock" || true
  [ -n "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : FAIL si github-copilot orphelin" {
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "github-copilot/claude-sonnet-4.6"
}
EOF
  _validate_provider_config "$TEST_DIR/opencode.json" "github-copilot/claude-sonnet-4.6" "github-copilot" || true
  [ -n "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : FAIL si openrouter orphelin" {
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "openrouter/anthropic/claude-sonnet-4-6"
}
EOF
  _validate_provider_config "$TEST_DIR/opencode.json" "openrouter/anthropic/claude-sonnet-4-6" "openrouter" || true
  [ -n "${_PW_CONFIG_WARNING:-}" ]
}

@test "validate_provider_config : OK si anthropic avec bloc provider" {
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "anthropic/claude-sonnet-4-6",
  "provider": {
    "anthropic": {"apiKey": "sk-ant-test"}
  }
}
EOF
  _validate_provider_config "$TEST_DIR/opencode.json" "anthropic/claude-sonnet-4-6" "anthropic"
  [ $? -eq 0 ]
  [ -z "${_PW_CONFIG_WARNING:-}" ]
}

# ─────────────────────────────────────────────────────────────────────────────
# _validate_provider_connectivity — Approche A
# Utilise des mocks curl pour simuler les réponses réseau.
# ─────────────────────────────────────────────────────────────────────────────

@test "connectivity : skip si curl absent" {
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "curl" ]; then return 1; fi
    builtin command "$@"
  }
  export -f command
  run _validate_provider_connectivity "mammouth" "sk-test" "https://api.mammouth.ai/v1"
  [ "$status" -eq 0 ]
  unset -f command
}

@test "connectivity : skip si pas de TTY et _PW_FORCE_TTY absent" {
  # Désactiver le forçage TTY pour ce test — simule un vrai environnement non-TTY
  export _PW_FORCE_TTY=0
  # run crée un sous-shell sans TTY — doit retourner PW_OK (skip gracieux)
  run _validate_provider_connectivity "mammouth" "sk-test" "https://api.mammouth.ai/v1"
  [ "$status" -eq 0 ]
  export _PW_FORCE_TTY=1  # restaurer pour les tests suivants
}

@test "connectivity : mammouth — retourne 0 si curl réussit" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "mammouth" "sk-test" "https://api.mammouth.ai/v1"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_OK" ]
}

@test "connectivity : mammouth — retourne PW_UNREACHABLE si curl échoue (timeout)" {
  curl() { return 28; }
  export -f curl
  run _validate_provider_connectivity "mammouth" "sk-test" "https://api.mammouth.ai/v1"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_UNREACHABLE" ]
}

@test "connectivity : mammouth — retourne PW_NO_KEY si api_key vide" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "mammouth" "" "https://api.mammouth.ai/v1"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_NO_KEY" ]
}

@test "connectivity : mammouth — retourne PW_BAD_URL si baseURL contient /chat/completions" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "mammouth" "sk-test" "https://api.mammouth.ai/v1/chat/completions"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_BAD_URL" ]
}

@test "connectivity : mammouth — retourne PW_NO_CREDS si base_url vide et pas de default" {
  echo '{"providers": {"mammouth": {"requires_api_key": true, "litellm": true}}}' > "$PROVIDERS_FILE"
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "mammouth" "sk-test" ""
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_NO_CREDS" ]
}

@test "connectivity : anthropic — retourne PW_OK si HTTP 401 (joignable, clé invalide)" {
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "401"; fi
    return 0
  }
  export -f curl
  run _validate_provider_connectivity "anthropic" "sk-invalid"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_OK" ]
}

@test "connectivity : anthropic — retourne PW_OK si HTTP 400" {
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "400"; fi
    return 0
  }
  export -f curl
  run _validate_provider_connectivity "anthropic" "sk-test"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_OK" ]
}

@test "connectivity : anthropic — retourne PW_UNREACHABLE si HTTP 000" {
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "000"; fi
    return 0
  }
  export -f curl
  run _validate_provider_connectivity "anthropic" "sk-test"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_UNREACHABLE" ]
}

@test "connectivity : anthropic — retourne PW_NO_KEY si api_key vide" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "anthropic" ""
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_NO_KEY" ]
}

@test "connectivity : bedrock — retourne PW_OK si AWS_BEARER_TOKEN_BEDROCK défini" {
  export AWS_BEARER_TOKEN_BEDROCK="test-bearer-token"
  run _validate_provider_connectivity "bedrock" "" ""
  [ "$status" -eq "$_PW_OK" ]
}

@test "connectivity : bedrock — retourne PW_OK si AWS_ACCESS_KEY_ID défini" {
  unset AWS_BEARER_TOKEN_BEDROCK 2>/dev/null || true
  export AWS_ACCESS_KEY_ID="AKIATEST"
  run _validate_provider_connectivity "bedrock" "" ""
  [ "$status" -eq "$_PW_OK" ]
}

@test "connectivity : bedrock — retourne PW_NO_CREDS si aucune credential AWS" {
  unset AWS_BEARER_TOKEN_BEDROCK AWS_ACCESS_KEY_ID AWS_PROFILE 2>/dev/null || true
  local _real_home="$HOME"
  export HOME="$TEST_DIR"
  aws() { return 1; }
  export -f aws
  run _validate_provider_connectivity "bedrock" "" ""
  local rc=$status
  export HOME="$_real_home"
  unset -f aws
  [ "$rc" -eq "$_PW_NO_CREDS" ]
}

@test "connectivity : github-copilot — retourne PW_OK si token dans auth.json" {
  mkdir -p "$TEST_DIR/.local/share/opencode"
  echo '{"github-copilot": {"token": "ghu_test"}}' > "$TEST_DIR/.local/share/opencode/auth.json"
  export _PW_AUTH_JSON="$TEST_DIR/.local/share/opencode/auth.json"
  run _validate_provider_connectivity "github-copilot" ""
  [ "$status" -eq "$_PW_OK" ]
}

@test "connectivity : github-copilot — retourne PW_NO_CREDS si auth.json absent" {
  export _PW_AUTH_JSON="$TEST_DIR/inexistant/auth.json"
  run _validate_provider_connectivity "github-copilot" ""
  [ "$status" -eq "$_PW_NO_CREDS" ]
}

@test "connectivity : openrouter — retourne PW_OK si curl réussit" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "openrouter" "sk-or-test"
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_OK" ]
}

@test "connectivity : openrouter — retourne PW_NO_KEY si api_key vide" {
  curl() { return 0; }
  export -f curl
  run _validate_provider_connectivity "openrouter" ""
  local rc=$status
  unset -f curl
  [ "$rc" -eq "$_PW_NO_KEY" ]
}

@test "connectivity : provider vide — retourne PW_NO_PROVIDER" {
  run _validate_provider_connectivity "" "" ""
  [ "$status" -eq "$_PW_NO_PROVIDER" ]
}

@test "connectivity : provider inconnu — retourne PW_OK (skip gracieux)" {
  run _validate_provider_connectivity "unknown-provider" "key" "url"
  [ "$status" -eq "$_PW_OK" ]
}

# ─────────────────────────────────────────────────────────────────────────────
# _display_provider_status — Affichage bloc contextuel
# ─────────────────────────────────────────────────────────────────────────────

@test "display_provider_status : affiche le label Provider" {
  # Provider sans credentials configurées → warning no_key ou no_creds
  curl() { return 28; }
  export -f curl

  # Simuler un hub.json sans clé
  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "mammouth", "api_key": "", "base_url": "https://api.mammouth.ai/v1"},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    mock_log_functions 2>/dev/null || true
    _display_provider_status '' '' ''
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "Provider"
  unset -f curl
}

@test "display_provider_status : affiche ✅ si provider joignable" {
  curl() { return 0; }
  export -f curl

  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "anthropic", "api_key": "sk-ant-test", "base_url": ""},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF

  # Mock curl pour simuler 401 Anthropic (joignable)
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "401"; fi
    return 0
  }
  export -f curl

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' '' ''
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "✅"
  unset -f curl
}

@test "display_provider_status : affiche ⚠️ et hint /connect si injoignable" {
  curl() { return 28; }
  export -f curl

  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "anthropic", "api_key": "sk-ant-test"},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF
  # Mock curl HTTP 000
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "000"; fi
    return 0
  }
  export -f curl

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' '' ''
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "⚠️"
  echo "$output" | grep -qi "connect"
  unset -f curl
}

@test "display_provider_status : affiche ⚠️ et hint si modèle orphelin (Approche C)" {
  # opencode.json avec modèle prefixé mais sans bloc provider
  cat > "$TEST_DIR/opencode.json" <<'EOF'
{
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-6"
}
EOF

  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "bedrock", "api_key": "", "region": "eu-west-3"},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF

  unset AWS_BEARER_TOKEN_BEDROCK AWS_ACCESS_KEY_ID AWS_PROFILE 2>/dev/null || true
  aws() { return 1; }
  export -f aws

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' '' '$TEST_DIR/opencode.json'
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "⚠️"
  unset -f aws
}

@test "display_provider_status : affiche ⚠️ si baseURL contient /chat/completions" {
  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {
    "name": "mammouth",
    "api_key": "sk-test",
    "base_url": "https://api.mammouth.ai/v1/chat/completions"
  },
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF

  curl() { return 0; }
  export -f curl

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' '' ''
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "⚠️"
  echo "$output" | grep -qi "chat/completions\|baseURL\|URL"
  unset -f curl
}

@test "display_provider_status : fonctionne sans project_id (hub self-deploy)" {
  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' '' ''
  "
  [ "$status" -eq 0 ]
}

@test "display_provider_status : utilise le provider_override si fourni" {
  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "mammouth", "api_key": "sk-default"},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF

  # Avec override bedrock et credential AWS
  export AWS_BEARER_TOKEN_BEDROCK="test-token"

  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    export AWS_BEARER_TOKEN_BEDROCK='test-token'
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _display_provider_status '' 'bedrock' ''
  "
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "bedrock"
}

# ─────────────────────────────────────────────────────────────────────────────
# _warn_provider_if_needed — Warning minimal pour cmds non-start
# ─────────────────────────────────────────────────────────────────────────────

@test "warn_provider_if_needed : ne bloque pas même si provider KO" {
  curl() { return 28; }
  export -f curl
  cat > "$HUB_CONFIG" <<'EOF'
{
  "default_provider": {"name": "anthropic", "api_key": "sk-test"},
  "opencode": {"model": "claude-sonnet-4-6"}
}
EOF
  # Mock curl HTTP 000
  curl() {
    if [[ "$*" == *"-w"* ]]; then echo "000"; fi
    return 0
  }
  export -f curl

  # Doit retourner 0 (non bloquant)
  run bash -c "
    export HUB_DIR='$HUB_DIR'
    export PROVIDERS_FILE='$PROVIDERS_FILE'
    export HUB_CONFIG='$HUB_CONFIG'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export OC_LANG=fr
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/provider-warnings.sh'
    _warn_provider_if_needed '' ''
  "
  [ "$status" -eq 0 ]
  unset -f curl
}

@test "warn_provider_if_needed : ne produit aucune sortie si pas de TTY" {
  # Exécuté depuis run (stdout redirigé = pas de TTY)
  run _warn_provider_if_needed "" ""
  [ "$status" -eq 0 ]
  # Pas de sortie (skip gracieux)
  [ -z "$output" ]
}
