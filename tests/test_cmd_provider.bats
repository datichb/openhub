#!/usr/bin/env bats
# Tests pour la commande oc provider et les fonctions provider de common.sh
# Migration depuis tests/cmd-provider.test.sh (plain shell → BATS)

setup() {
  TEST_DIR="$(mktemp -d)"

  # Surcharger les chemins AVANT de sourcer common.sh
  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # Fournir un projects.md minimal
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test
PROJEOF

  touch "$PATHS_FILE"

  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── provider_exists() ─────────────────────────────────────────────────────────

@test "provider_exists : anthropic est un provider connu" {
  run provider_exists "anthropic"
  [ "$status" -eq 0 ]
}

@test "provider_exists : mammouth est un provider connu" {
  run provider_exists "mammouth"
  [ "$status" -eq 0 ]
}

@test "provider_exists : github-models est un provider connu" {
  run provider_exists "github-models"
  [ "$status" -eq 0 ]
}

@test "provider_exists : bedrock est un provider connu" {
  run provider_exists "bedrock"
  [ "$status" -eq 0 ]
}

@test "provider_exists : ollama est un provider connu" {
  run provider_exists "ollama"
  [ "$status" -eq 0 ]
}

@test "provider_exists : un provider inconnu retourne non-zero" {
  run provider_exists "invalid-provider"
  [ "$status" -ne 0 ]
}

# ── get_provider_info() ───────────────────────────────────────────────────────

@test "get_provider_info : anthropic a un label contenant 'Anthropic'" {
  result=$(get_provider_info "anthropic" "label")
  [ -n "$result" ]
  [[ "$result" == *"Anthropic"* ]]
}

@test "get_provider_info : mammouth a un label contenant 'MammouthAI'" {
  result=$(get_provider_info "mammouth" "label")
  [ -n "$result" ]
  [[ "$result" == *"MammouthAI"* ]]
}

# ── list_all_providers() ──────────────────────────────────────────────────────

@test "list_all_providers : contient anthropic" {
  result=$(list_all_providers)
  [[ "$result" == *"anthropic"* ]]
}

@test "list_all_providers : contient mammouth" {
  result=$(list_all_providers)
  [[ "$result" == *"mammouth"* ]]
}

@test "list_all_providers : contient github-models" {
  result=$(list_all_providers)
  [[ "$result" == *"github-models"* ]]
}

@test "list_all_providers : contient bedrock" {
  result=$(list_all_providers)
  [[ "$result" == *"bedrock"* ]]
}

@test "list_all_providers : contient ollama" {
  result=$(list_all_providers)
  [[ "$result" == *"ollama"* ]]
}

# ── get_hub_default_provider() ────────────────────────────────────────────────

@test "get_hub_default_provider : retourne un provider valide ou vide" {
  hub_provider=$(get_hub_default_provider)
  if [ -n "$hub_provider" ]; then
    run provider_exists "$hub_provider"
    [ "$status" -eq 0 ]
  else
    # Vide est acceptable — provider non configuré
    true
  fi
}

# ── get_effective_llm_model() ─────────────────────────────────────────────────

@test "get_effective_llm_model : retourne le modèle par défaut si aucune config projet" {
  result=$(get_effective_llm_model "TEST-PROVIDER-NOCONFIG")
  [ "$result" = "claude-sonnet-4-5" ]
}

# ── cmd-provider.sh : commande sans argument ──────────────────────────────────

@test "cmd-provider.sh sans argument : affiche la liste des providers" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh"
  [[ "$output" == *"Fournisseurs LLM"* ]]
}

@test "cmd-provider.sh sans argument : affiche Anthropic" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh"
  [[ "$output" == *"Anthropic"* ]]
}

# ── cmd-provider.sh list ──────────────────────────────────────────────────────

@test "cmd-provider.sh list : affiche Anthropic" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" list
  [ "$status" -eq 0 ]
  [[ "$output" == *"Anthropic"* ]]
}

@test "cmd-provider.sh list : affiche MammouthAI" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" list
  [[ "$output" == *"MammouthAI"* ]]
}

@test "cmd-provider.sh list : affiche GitHub" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" list
  [[ "$output" == *"GitHub"* ]]
}

@test "cmd-provider.sh list : affiche Bedrock" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" list
  [[ "$output" == *"Bedrock"* ]]
}

@test "cmd-provider.sh list : affiche Ollama" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" list
  [[ "$output" == *"Ollama"* ]]
}

# ── cmd-provider.sh get ───────────────────────────────────────────────────────

@test "cmd-provider.sh get : affiche l'ID projet dans la sortie" {
  run bash "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh" get "NONEXISTENT-PROJECT" 2>&1 || true
  [[ "$output" == *"NONEXISTENT-PROJECT"* ]]
}

# ── Contenu de cmd-config.sh ──────────────────────────────────────────────────

@test "cmd-config.sh : contient le provider mammouth" {
  run grep -q "mammouth" "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  [ "$status" -eq 0 ]
}

@test "cmd-config.sh : contient le provider github-models" {
  run grep -q "github-models" "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  [ "$status" -eq 0 ]
}

@test "cmd-config.sh : contient le provider bedrock" {
  run grep -q "bedrock" "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  [ "$status" -eq 0 ]
}

@test "cmd-config.sh : contient le provider ollama" {
  run grep -q "ollama" "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  [ "$status" -eq 0 ]
}

# ── Contenu de opencode.adapter.sh ───────────────────────────────────────────

@test "opencode.adapter.sh : a le support mammouth" {
  run grep -q "mammouth" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "opencode.adapter.sh : a le support github-models" {
  run grep -q "github-models" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "opencode.adapter.sh : a le support bedrock" {
  run grep -q "bedrock" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "opencode.adapter.sh : a le support ollama" {
  run grep -q "ollama" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "opencode.adapter.sh : utilise le provider natif amazon-bedrock pour bedrock" {
  run grep -q "amazon-bedrock" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "opencode.adapter.sh : injecte AWS_BEARER_TOKEN_BEDROCK au démarrage" {
  run grep -q "AWS_BEARER_TOKEN_BEDROCK" "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}



# ── Bedrock native : providers.json ──────────────────────────────────────────

@test "opencode.adapter.sh : le cas bedrock génère un bloc amazon-bedrock" {
  run grep -q '"amazon-bedrock"' "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"
  [ "$status" -eq 0 ]
}

@test "providers.json : le modèle par défaut bedrock utilise le préfixe amazon-bedrock/" {
  result=$(jq -r '.providers.bedrock.default_model' "$BATS_TEST_DIRNAME/../config/providers.json")
  [[ "$result" == *"amazon-bedrock/"* ]]
}

@test "providers.json : bedrock.litellm est false (provider natif)" {
  result=$(jq -r '.providers.bedrock.litellm' "$BATS_TEST_DIRNAME/../config/providers.json")
  [ "$result" = "false" ]
}

# ── cmd-provider.sh : set-default appelle adapter_deploy ─────────────────────

@test "cmd-provider.sh : set-default appelle adapter_deploy pour la synchro" {
  run grep -q "adapter_deploy" "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh"
  [ "$status" -eq 0 ]
}

@test "cmd-provider.sh : set-default source adapter-manager.sh" {
  run grep -q "adapter-manager.sh" "$BATS_TEST_DIRNAME/../scripts/cmd-provider.sh"
  [ "$status" -eq 0 ]
}

# ── hub.json structure ────────────────────────────────────────────────────────

@test "hub.json : contient le champ default_provider" {
  run grep -q "default_provider" "$BATS_TEST_DIRNAME/../config/hub.json"
  [ "$status" -eq 0 ]
}

@test "hub.json : default_provider a un champ name" {
  result=$(jq -r '.default_provider | has("name")' "$BATS_TEST_DIRNAME/../config/hub.json")
  [ "$result" = "true" ]
}

@test "hub.json : default_provider a un champ api_key" {
  result=$(jq -r '.default_provider | has("api_key")' "$BATS_TEST_DIRNAME/../config/hub.json")
  [ "$result" = "true" ]
}

@test "hub.json : default_provider a un champ base_url" {
  result=$(jq -r '.default_provider | has("base_url")' "$BATS_TEST_DIRNAME/../config/hub.json")
  [ "$result" = "true" ]
}

# ── providers.json catalog ────────────────────────────────────────────────────

@test "providers.json : existe" {
  [ -f "$BATS_TEST_DIRNAME/../config/providers.json" ]
}

@test "providers.json : contient anthropic" {
  run grep -q "anthropic" "$BATS_TEST_DIRNAME/../config/providers.json"
  [ "$status" -eq 0 ]
}

@test "providers.json : contient mammouth" {
  run grep -q "mammouth" "$BATS_TEST_DIRNAME/../config/providers.json"
  [ "$status" -eq 0 ]
}

@test "providers.json : contient github-models" {
  run grep -q "github-models" "$BATS_TEST_DIRNAME/../config/providers.json"
  [ "$status" -eq 0 ]
}

@test "providers.json : contient bedrock" {
  run grep -q "bedrock" "$BATS_TEST_DIRNAME/../config/providers.json"
  [ "$status" -eq 0 ]
}

@test "providers.json : contient ollama" {
  run grep -q "ollama" "$BATS_TEST_DIRNAME/../config/providers.json"
  [ "$status" -eq 0 ]
}
