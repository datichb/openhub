#!/usr/bin/env bats
# Tests pour scripts/adapters/opencode.adapter.sh
# Fonctions testées : _build_provider_block, génération opencode.json via adapter_deploy
# Stratégie : sourcer common.sh + opencode.adapter.sh après avoir mocké les dépendances
#             (HUB_DIR, prompt-builder.sh, find CANONICAL_AGENTS_DIR vide)

setup() {
  TEST_DIR="$(mktemp -d)"
  DEPLOY_DIR="$(mktemp -d)"
  AGENTS_DIR="$(mktemp -d)"  # Dossier agents vide → adapter_deploy ne boucle sur rien

  # Fixer HUB_DIR avant le source pour que prompt-builder.sh soit trouvé
  HUB_DIR="$BATS_TEST_DIRNAME/.."

  # Sourcer common.sh pour les helpers partagés
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Resurchager les chemins après le source (common.sh les recalcule depuis BASH_SOURCE)
  API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  PROJECTS_FILE="$TEST_DIR/projects.md"
  # Dossier agents vide pour éviter que adapter_deploy ne déploie de vrais agents
  CANONICAL_AGENTS_DIR="$AGENTS_DIR"

  # Sourcer prompt-builder.sh (nécessaire pour adapter_deploy)
  source "$BATS_TEST_DIRNAME/../scripts/lib/prompt-builder.sh"

  # Sourcer l'adaptateur
  source "$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh"

  # Mocks des fonctions de log
  log_info()    { true; }
  log_success() { true; }
  log_warn()    { true; }
  log_error()   { true; }

  # Mock get_project_language (pas de fichier projects.md peuplé nécessaire)
  get_project_language() { echo ""; }
}

teardown() {
  rm -rf "$TEST_DIR" "$DEPLOY_DIR" "$AGENTS_DIR"
}

# ── _build_provider_block : sans clé ─────────────────────────────────────────

@test "_build_provider_block : retourne vide si project_id vide" {
  run _build_provider_block ""
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_build_provider_block : retourne vide si provider absent" {
  # Fichier api-keys avec provider vide
  printf '[PROJ-X]\nmodel=claude-opus-4-5\nprovider=\napi_key=\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-X"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_build_provider_block : retourne vide si api_key absent" {
  printf '[PROJ-X]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-X"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ── _build_provider_block : anthropic ────────────────────────────────────────

@test "_build_provider_block : génère le bloc anthropic avec la clé" {
  printf '[PROJ-ANT]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=sk-ant-test123\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-ANT"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"anthropic"'
  echo "$output" | grep -q 'sk-ant-test123'
}

@test "_build_provider_block : bloc anthropic est du JSON valide (partiel encapsulé)" {
  printf '[PROJ-ANT]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=sk-ant-test123\n' > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  block=$(_build_provider_block "PROJ-ANT")
  # Encapsuler dans un objet JSON pour valider la syntaxe
  run bash -c "echo '{'  '$1'  '}' | jq . >/dev/null" _ "$block"
  [ "$status" -eq 0 ]
}

# ── _build_provider_block : litellm ──────────────────────────────────────────

@test "_build_provider_block : génère le bloc litellm avec apiKey et baseURL" {
  printf '[PROJ-LIT]\nmodel=claude-sonnet-4-5\nprovider=litellm\napi_key=sk-bRf-abc\nbase_url=https://api.mammouth.ai/v1\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-LIT"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"litellm"'
  echo "$output" | grep -q 'sk-bRf-abc'
  echo "$output" | grep -q 'https://api.mammouth.ai/v1'
}

@test "_build_provider_block : litellm sans base_url — pas de champ baseURL" {
  printf '[PROJ-LIT2]\nmodel=claude-sonnet-4-5\nprovider=litellm\napi_key=sk-bRf-xyz\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-LIT2"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"litellm"'
  # Vérifier que baseURL est absent de la sortie (sans écraser $output avec un second run)
  ! echo "$output" | grep -q 'baseURL'
}

# ── _build_provider_json : github-copilot (sans clé API) ─────────────────────

@test "_build_provider_json : github-copilot sans clé → génère {'provider': {'github-copilot': {}}}" {
  command -v jq &>/dev/null || skip "jq non disponible"
  result=$(_build_provider_json "github-copilot" "" "" "")
  [ -n "$result" ]
  run jq . <<< "$result"
  [ "$status" -eq 0 ]
  value=$(jq -r '.provider["github-copilot"]' <<< "$result")
  [ "$value" = "{}" ]
}

@test "_build_provider_block : github-copilot sans clé → génère un bloc valide" {
  printf '[PROJ-GHC]\nmodel=claude-sonnet-4-5\nprovider=github-copilot\napi_key=\n' > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  result=$(_build_provider_block "PROJ-GHC")
  [ -n "$result" ]
  run jq . <<< "$result"
  [ "$status" -eq 0 ]
  value=$(jq -r '.provider["github-copilot"]' <<< "$result")
  [ "$value" = "{}" ]
}

@test "_build_provider_block : anthropic sans clé → retourne vide (inchangé)" {
  printf '[PROJ-ANT-NOKEY]\nmodel=claude-sonnet-4-5\nprovider=anthropic\napi_key=\n' > "$API_KEYS_FILE"
  run _build_provider_block "PROJ-ANT-NOKEY"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_build_provider_block : ollama sans clé → génère un bloc valide (non-régression #opencode-hub-0td)" {
  # ollama a requires_api_key=false dans providers.json — doit générer un bloc même sans api_key
  printf '[PROJ-OLLAMA]\nmodel=llama3.2\nprovider=ollama\napi_key=\n' > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  result=$(_build_provider_block "PROJ-OLLAMA")
  [ -n "$result" ]
  run jq . <<< "$result"
  [ "$status" -eq 0 ]
  # Doit contenir une référence au provider ollama ou litellm (selon l'implémentation)
  echo "$result" | grep -qE '"ollama"|"litellm"'
}

# ── Génération opencode.json via adapter_deploy ───────────────────────────────

@test "adapter_deploy : génère opencode.json sans clé API (contenu minimal)" {
  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  run grep '"$schema"' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  run grep '"model"' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
}

@test "adapter_deploy : opencode.json sans clé API est du JSON valide" {
  adapter_deploy "$DEPLOY_DIR" ""

  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
}

@test "adapter_deploy : injecte le bloc anthropic dans opencode.json" {
  printf '[PROJ-ANT]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=sk-ant-inject\n' > "$API_KEYS_FILE"
  adapter_deploy "$DEPLOY_DIR" "PROJ-ANT"

  [ -f "$DEPLOY_DIR/opencode.json" ]
  run grep 'sk-ant-inject' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
}

@test "adapter_deploy : opencode.json avec clé anthropic est du JSON valide" {
  printf '[PROJ-ANT]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=sk-ant-inject\n' > "$API_KEYS_FILE"
  adapter_deploy "$DEPLOY_DIR" "PROJ-ANT"

  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
}

@test "adapter_deploy : ajoute opencode.json au .gitignore avant l'écriture" {
  printf '[PROJ-ANT]\nmodel=claude-opus-4-5\nprovider=anthropic\napi_key=sk-ant-gitignore\n' > "$API_KEYS_FILE"

  # Mock _gitignore_opencode_json pour vérifier l'appel
  gitignore_called=false
  _gitignore_opencode_json() { gitignore_called=true; }

  adapter_deploy "$DEPLOY_DIR" "PROJ-ANT"
  [ "$gitignore_called" = "true" ]
}

@test "adapter_deploy : opencode.json avec litellm + base_url est du JSON valide" {
  printf '[PROJ-LIT]\nmodel=claude-sonnet-4-5\nprovider=litellm\napi_key=sk-bRf-lit\nbase_url=https://api.mammouth.ai/v1\n' > "$API_KEYS_FILE"
  adapter_deploy "$DEPLOY_DIR" "PROJ-LIT"

  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
}

# ── Filtrage par should_deploy_agent ─────────────────────────────────────────

@test "adapter_deploy : ne déploie pas un agent filtré par should_deploy_agent" {
  # Créer un agent de test supportant opencode
  local family_dir="$AGENTS_DIR/quality"
  mkdir -p "$family_dir"
  cat > "$family_dir/reviewer.md" <<'AGENTEOF'
---
id: reviewer
label: Reviewer
description: Code reviewer
targets: [opencode]
skills: []
---

# Reviewer
Contenu de test.
AGENTEOF

  cat > "$family_dir/debugger.md" <<'AGENTEOF'
---
id: debugger
label: Debugger
description: Debugger agent
targets: [opencode]
skills: []
---

# Debugger
Contenu de test.
AGENTEOF

  # Configurer un projects.md qui n'autorise que reviewer
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-FILTER
- Nom : Filtré
- Stack : Test
- Labels : test
- Agents : reviewer
EOF

  adapter_deploy "$DEPLOY_DIR" "PROJ-FILTER"

  # reviewer doit être déployé
  [ -f "$DEPLOY_DIR/.opencode/agents/reviewer.md" ]
  # debugger ne doit PAS être déployé
  [ ! -f "$DEPLOY_DIR/.opencode/agents/debugger.md" ]
}

@test "adapter_deploy : déploie tous les agents quand agents=all" {
  # Créer deux agents de test
  local family_dir="$AGENTS_DIR/quality"
  mkdir -p "$family_dir"
  cat > "$family_dir/reviewer.md" <<'AGENTEOF'
---
id: reviewer
label: Reviewer
description: Code reviewer
targets: [opencode]
skills: []
---

# Reviewer
AGENTEOF

  cat > "$family_dir/debugger.md" <<'AGENTEOF'
---
id: debugger
label: Debugger
description: Debugger agent
targets: [opencode]
skills: []
---

# Debugger
AGENTEOF

  # Configurer un projects.md avec agents=all
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-ALL
- Nom : Tous
- Stack : Test
- Labels : test
- Agents : all
EOF

  adapter_deploy "$DEPLOY_DIR" "PROJ-ALL"

  # Les deux doivent être déployés
  [ -f "$DEPLOY_DIR/.opencode/agents/reviewer.md" ]
  [ -f "$DEPLOY_DIR/.opencode/agents/debugger.md" ]
}

# ── Cas limites : valeurs spéciales dans les clés API ─────────────────────────

@test "_build_provider_block : clé API avec espaces génère du JSON valide" {
  # Clé avec espaces (encodage base64 typique avec padding)
  printf '[PROJ-SPACE]\nmodel=claude-sonnet-4-5\nprovider=anthropic\napi_key=sk-ant-abc def ghi\n' > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  block=$(_build_provider_block "PROJ-SPACE" 2>/dev/null || true)
  [ -n "$block" ]
  run jq . <<< "$block"
  [ "$status" -eq 0 ]
  # La clé doit être présente dans le JSON
  result=$(jq -r '.provider.anthropic.apiKey' <<< "$block")
  [ "$result" = "sk-ant-abc def ghi" ]
}

@test "_build_provider_block : URL avec paramètres génère du JSON valide" {
  # URL avec caractères spéciaux (& ? = dans query string)
  printf '[PROJ-URL]\nmodel=claude-sonnet-4-5\nprovider=litellm\napi_key=sk-lit-test\nbase_url=https://api.example.com/v1?key=value&other=test\n' > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  block=$(_build_provider_block "PROJ-URL" 2>/dev/null || true)
  [ -n "$block" ]
  run jq . <<< "$block"
  [ "$status" -eq 0 ]
  # L'URL doit être correctement encodée dans le JSON
  result=$(jq -r '.provider.litellm.options.baseURL' <<< "$block")
  [[ "$result" == *"key=value"* ]]
}

@test "_build_provider_block : clé API longue (256 chars) génère du JSON valide" {
  long_key=$(python3 -c "import string, random; print('sk-' + ''.join(random.choices(string.ascii_letters + string.digits, k=253)))" 2>/dev/null || printf 'sk-%0253d' 0)
  printf '[PROJ-LONG]\nmodel=claude-sonnet-4-5\nprovider=anthropic\napi_key=%s\n' "$long_key" > "$API_KEYS_FILE"
  command -v jq &>/dev/null || skip "jq non disponible"
  block=$(_build_provider_block "PROJ-LONG" 2>/dev/null || true)
  [ -n "$block" ]
  run jq . <<< "$block"
  [ "$status" -eq 0 ]
}

# ── Injection du champ model par agent dans opencode.json ─────────────────────

@test "adapter_deploy : agent avec modèle override → champ model présent" {
  command -v jq &>/dev/null || skip "jq non disponible"

  # Créer un agent de test
  local family_dir="$AGENTS_DIR/planning"
  mkdir -p "$family_dir"
  cat > "$family_dir/orchestrator-dev.md" <<'AGENTEOF'
---
id: orchestrator-dev
label: Orchestrator Dev
description: Orchestration agent
targets: [opencode]
skills: []
---

# Orchestrator Dev
Contenu de test.
AGENTEOF

  # Configurer hub.json avec un override agent → modèle différent du global
  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "0.0.0-test",
  "agent_models": {
    "families": {},
    "agents": { "orchestrator-dev": "claude-opus-4" }
  }
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  # Le JSON doit être valide
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # L'agent doit avoir le champ model
  result=$(jq -r '.agent."orchestrator-dev".model' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "claude-opus-4" ]
}

@test "adapter_deploy : agent avec modèle résolu == global → pas de champ model" {
  command -v jq &>/dev/null || skip "jq non disponible"

  local family_dir="$AGENTS_DIR/backend"
  mkdir -p "$family_dir"
  cat > "$family_dir/developer-api.md" <<'AGENTEOF'
---
id: developer-api
label: Developer API
description: API developer
targets: [opencode]
skills: []
---

# Developer API
Contenu de test.
AGENTEOF

  # hub.json avec override explicite == global (claude-sonnet-4-5) → pas de champ model
  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "0.0.0-test",
  "agent_models": {
    "families": {},
    "agents": { "developer-api": "claude-sonnet-4-5" }
  }
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # L'agent ne doit PAS avoir de champ model (null en jq)
  result=$(jq -r '.agent."developer-api".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "ABSENT" ]
}

@test "adapter_deploy : sans agent_models dans hub.json → opencode.json sans champ model agent" {
  command -v jq &>/dev/null || skip "jq non disponible"

  local family_dir="$AGENTS_DIR/backend"
  mkdir -p "$family_dir"
  cat > "$family_dir/developer-backend.md" <<'AGENTEOF'
---
id: developer-backend
label: Developer Backend
description: Backend developer
targets: [opencode]
skills: []
---

# Developer Backend
Contenu de test.
AGENTEOF

  # hub.json sans agent_models
  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "0.0.0-test"
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # Pas de champ model sur l'agent
  result=$(jq -r '.agent."developer-backend".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "ABSENT" ]
}

@test "adapter_deploy : agent avec plancher clampé au-dessus du global → model injecté" {
  command -v jq &>/dev/null || skip "jq non disponible"

  # Agent avec min_model = opus (plancher au-dessus du global sonnet)
  local family_dir="$AGENTS_DIR/planning"
  mkdir -p "$family_dir"
  cat > "$family_dir/high-floor-agent.md" <<'AGENTEOF'
---
id: high-floor-agent
model: claude-opus-4
label: High Floor Agent
description: Agent with high minimum model
targets: [opencode]
skills: []
---

# High Floor Agent
Contenu de test.
AGENTEOF

  # Pas d'override explicite — le clamp doit remonter au-dessus du global
  HUB_CONFIG="$TEST_DIR/hub.json"
  echo '{"version":"0.0.0-test"}' > "$HUB_CONFIG"

  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # Le modèle clampé (opus) doit être injecté car > global (sonnet)
  result=$(jq -r '.agent."high-floor-agent".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "claude-opus-4" ]
}

@test "adapter_deploy : opencode.json valide avec mix d'agents avec et sans model" {
  command -v jq &>/dev/null || skip "jq non disponible"

  local family_dir="$AGENTS_DIR/planning"
  mkdir -p "$family_dir"

  # Agent 1 : override opus
  cat > "$family_dir/agent-with-model.md" <<'AGENTEOF'
---
id: agent-with-model
label: Agent With Model
description: Agent with model override
targets: [opencode]
skills: []
---

# Agent With Model
AGENTEOF

  # Agent 2 : pas d'override
  cat > "$family_dir/agent-no-model.md" <<'AGENTEOF'
---
id: agent-no-model
label: Agent No Model
description: Agent without model override
targets: [opencode]
skills: []
---

# Agent No Model
AGENTEOF

  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "0.0.0-test",
  "agent_models": {
    "families": {},
    "agents": { "agent-with-model": "claude-opus-4" }
  }
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" ""

  [ -f "$DEPLOY_DIR/opencode.json" ]
  # Le JSON doit rester valide
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # Agent avec override → model présent
  result1=$(jq -r '.agent."agent-with-model".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result1" = "claude-opus-4" ]
  # Agent sans override → model absent
  result2=$(jq -r '.agent."agent-no-model".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result2" = "ABSENT" ]
}

@test "adapter_deploy : opencode.json est du JSON valide même sans provider ni agents" {
  # Dossier agents vide → opencode.json minimal avec juste model et schema
  adapter_deploy "$DEPLOY_DIR" ""

  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  # Doit avoir $schema et model
  result=$(jq -r '.["$schema"]' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "https://opencode.ai/config.json" ]
}

# ── _apply_provider_prefix ────────────────────────────────────────────────────

@test "_apply_provider_prefix : model vide → return 0 sans sortie" {
  run _apply_provider_prefix "" "anthropic"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "_apply_provider_prefix : provider vide → echo du model brut" {
  run _apply_provider_prefix "claude-opus-4" ""
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "_apply_provider_prefix : provider inconnu (absent de providers.json) → echo du model brut" {
  # HUB_DIR pointe vers le projet réel qui contient providers.json
  # Le provider 'unknown-provider' n'existe pas dans ce fichier
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  command -v jq &>/dev/null || skip "jq non disponible"
  run _apply_provider_prefix "claude-opus-4" "unknown-provider"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-opus-4" ]
}

@test "_apply_provider_prefix : anthropic sans alias → 'anthropic/claude-opus-4'" {
  command -v jq &>/dev/null || skip "jq non disponible"
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  run _apply_provider_prefix "claude-opus-4" "anthropic"
  [ "$status" -eq 0 ]
  [ "$output" = "anthropic/claude-opus-4" ]
}

@test "_apply_provider_prefix : bedrock avec alias → 'amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0'" {
  command -v jq &>/dev/null || skip "jq non disponible"
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  run _apply_provider_prefix "claude-sonnet-4-5" "bedrock"
  [ "$status" -eq 0 ]
  [ "$output" = "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0" ]
}

@test "_apply_provider_prefix : mammouth (prefix null) → model inchangé" {
  command -v jq &>/dev/null || skip "jq non disponible"
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  run _apply_provider_prefix "claude-sonnet-4-5" "mammouth"
  [ "$status" -eq 0 ]
  [ "$output" = "claude-sonnet-4-5" ]
}

@test "_apply_provider_prefix : github-copilot avec alias → 'github-copilot/claude-sonnet-4.5'" {
  command -v jq &>/dev/null || skip "jq non disponible"
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  run _apply_provider_prefix "claude-sonnet-4-5" "github-copilot"
  [ "$status" -eq 0 ]
  [ "$output" = "github-copilot/claude-sonnet-4.5" ]
}
