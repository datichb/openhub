#!/usr/bin/env bats
# Tests pour scripts/adapters/opencode.adapter.sh
# Fonctions testées : _build_provider_block, génération opencode.json via adapter_deploy
# Stratégie : sourcer common.sh + opencode.adapter.sh après avoir mocké les dépendances
#             (HUB_DIR, prompt-builder.sh, find CANONICAL_AGENTS_DIR vide)

setup() {
  TEST_DIR="$(mktemp -d)"
  DEPLOY_DIR="$TEST_DIR/deploy"
  AGENTS_DIR="$TEST_DIR/agents"  # Dossier agents vide → adapter_deploy ne boucle sur rien
  mkdir -p "$DEPLOY_DIR" "$AGENTS_DIR"

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

@test "_build_provider_block : ollama sans clé → génère un bloc valide (non-régression #openhub-0td)" {
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

# ── Injection du champ instructions selon le contexte projet ─────────────────

@test "adapter_deploy : pas de champ instructions si aucun fichier contexte ni cache" {
  command -v jq &>/dev/null || skip "jq non disponible"
  # Aucun ONBOARDING.md, CONVENTIONS.md ni context.json dans DEPLOY_DIR
  adapter_deploy "$DEPLOY_DIR" ""

  run jq 'has("instructions")' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "false" ]
}

@test "adapter_deploy : injecte ONBOARDING.md dans instructions si présent" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$DEPLOY_DIR/ONBOARDING.md"

  adapter_deploy "$DEPLOY_DIR" ""

  run jq -r '.instructions[]' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ONBOARDING.md"* ]]
}

@test "adapter_deploy : injecte CONVENTIONS.md dans instructions si présent" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$DEPLOY_DIR/CONVENTIONS.md"

  adapter_deploy "$DEPLOY_DIR" ""

  run jq -r '.instructions[]' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONVENTIONS.md"* ]]
}

@test "adapter_deploy : injecte les deux fichiers si ONBOARDING.md et CONVENTIONS.md présents" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$DEPLOY_DIR/ONBOARDING.md"
  touch "$DEPLOY_DIR/CONVENTIONS.md"

  adapter_deploy "$DEPLOY_DIR" ""

  run jq -r '.instructions | length' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "2" ]
}

@test "adapter_deploy : préfère context.json valide à ONBOARDING.md" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$DEPLOY_DIR/ONBOARDING.md"

  # Créer un context.json minimal valide
  mkdir -p "$DEPLOY_DIR/.opencode"
  # Mock validate_context_cache pour retourner succès
  validate_context_cache() { return 0; }

  cat > "$DEPLOY_DIR/.opencode/context.json" <<'EOF'
{"version":"1.0","generated_at":"2026-01-01T00:00:00Z","stack":{"languages":["typescript"]},"conventions":{"source":"CONVENTIONS.md","hash":"sha256:abc"},"key_files":{}}
EOF

  adapter_deploy "$DEPLOY_DIR" ""

  run jq -r '.instructions[]' "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *".opencode/context.json"* ]]
  [[ "$output" != *"ONBOARDING.md"* ]]
}

@test "adapter_deploy : opencode.json avec instructions est du JSON valide" {
  command -v jq &>/dev/null || skip "jq non disponible"
  touch "$DEPLOY_DIR/ONBOARDING.md"
  touch "$DEPLOY_DIR/CONVENTIONS.md"

  adapter_deploy "$DEPLOY_DIR" ""

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
  [ "$result" = "anthropic/claude-opus-4" ]
}

@test "adapter_deploy : agent avec modèle résolu == global → champ model présent" {
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

  # hub.json avec override explicite == global (claude-sonnet-4-5) → champ model présent maintenant
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
  # L'agent DOIT avoir le champ model (nouveau comportement)
  result=$(jq -r '.agent."developer-api".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result" != "ABSENT" ]
  [[ "$result" == *"claude-sonnet-4"* ]]
}

@test "adapter_deploy : sans agent_models dans hub.json → opencode.json avec champ model global" {
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
  # Le champ model DOIT être présent maintenant (utilise le modèle global)
  result=$(jq -r '.agent."developer-backend".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result" != "ABSENT" ]
  [[ "$result" == *"claude"* ]]
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
  [ "$result" = "anthropic/claude-opus-4" ]
}

@test "adapter_deploy : opencode.json valide avec mix d'agents avec et sans override" {
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
  [ "$result1" = "anthropic/claude-opus-4" ]
  # Agent sans override → model présent aussi maintenant (utilise global)
  result2=$(jq -r '.agent."agent-no-model".model // "ABSENT"' "$DEPLOY_DIR/opencode.json")
  [ "$result2" != "ABSENT" ]
  [[ "$result2" == *"claude"* ]]
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

@test "deploy_native_skills : génère SKILL.md pour native_skills d'un agent" {
  local family_dir="$AGENTS_DIR/developer"
  mkdir -p "$family_dir"

  # Créer un agent avec native_skills
  cat > "$family_dir/developer-test.md" <<'AGENTEOF'
---
id: developer-test
label: Developer Test
description: Test agent
targets: [opencode]
skills: [developer/dev-standards-universal]
native_skills: [developer/dev-standards-testing]
permission:
  skill: allow
---

# Developer Test
AGENTEOF

  CANONICAL_AGENTS_DIR="$AGENTS_DIR"
  adapter_deploy "$DEPLOY_DIR" ""

  # La skill native doit être déployée dans .opencode/skills/
  [ -f "$DEPLOY_DIR/.opencode/skills/dev-standards-testing/SKILL.md" ]
}

@test "deploy_native_skills : le dossier .opencode/skills/ est vidé puis recréé à chaque déploiement" {
  local family_dir="$AGENTS_DIR/developer"
  mkdir -p "$family_dir"

  # Pré-peupler avec une skill obsolète
  mkdir -p "$DEPLOY_DIR/.opencode/skills/obsolete-skill"
  echo "old content" > "$DEPLOY_DIR/.opencode/skills/obsolete-skill/SKILL.md"

  # Aucun agent avec native_skills → le dossier doit être nettoyé
  cat > "$family_dir/developer-noskills.md" <<'AGENTEOF'
---
id: developer-noskills
label: Developer No Skills
description: Test agent
targets: [opencode]
skills: []
---

# Developer No Skills
AGENTEOF

  CANONICAL_AGENTS_DIR="$AGENTS_DIR"
  adapter_deploy "$DEPLOY_DIR" ""

  # La skill obsolète ne doit plus exister
  [ ! -f "$DEPLOY_DIR/.opencode/skills/obsolete-skill/SKILL.md" ]
}

@test "_apply_provider_prefix : github-copilot avec alias → 'github-copilot/claude-sonnet-4.5'" {
  command -v jq &>/dev/null || skip "jq non disponible"
  HUB_DIR="$BATS_TEST_DIRNAME/.."
  run _apply_provider_prefix "claude-sonnet-4-5" "github-copilot"
  [ "$status" -eq 0 ]
  [ "$output" = "github-copilot/claude-sonnet-4.5" ]
}

# ── disabled_native_agents dans opencode.json ────────────────────────────────

@test "adapter_deploy : injecte {disable:true} pour chaque agent natif désactivé depuis hub.json" {
  command -v jq &>/dev/null || skip "jq non disponible"

  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan", "general"]
  }
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" "PROJ-DIS"

  # Chaque agent listé doit avoir {"disable": true} dans opencode.json
  run jq -r '.agent.build.disable' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "true" ]

  run jq -r '.agent.plan.disable' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "true" ]

  run jq -r '.agent.general.disable' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "true" ]
}

@test "adapter_deploy : pas d'entrée disable si disabled_native_agents est vide" {
  command -v jq &>/dev/null || skip "jq non disponible"

  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": []
  }
}
HUBEOF

  adapter_deploy "$DEPLOY_DIR" "PROJ-NODIS"

  # Aucun agent natif ne doit avoir de bloc disable
  run jq 'has("agent") and (.agent | has("build"))' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "false" ]
}

@test "adapter_deploy : priorité projet > hub pour disabled_native_agents" {
  command -v jq &>/dev/null || skip "jq non disponible"

  HUB_CONFIG="$TEST_DIR/hub.json"
  cat > "$HUB_CONFIG" <<'HUBEOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan"]
  }
}
HUBEOF

  # Configurer un override projet qui ne désactive que "build"
  cat > "$PROJECTS_FILE" <<'EOF'
## PROJ-OVERRIDE
- Nom : Projet Override
- Stack : TypeScript
- Disable agents : build
EOF

  PATHS_FILE="$TEST_DIR/paths.local.md"
  echo "PROJ-OVERRIDE=$DEPLOY_DIR" > "$PATHS_FILE"

  adapter_deploy "$DEPLOY_DIR" "PROJ-OVERRIDE"

  # build doit être désactivé (vient du projet)
  run jq -r '.agent.build.disable' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "true" ]

  # plan NE doit PAS être désactivé (le projet override prend précédence sur le hub)
  run jq 'has("agent") and (.agent | has("plan"))' "$DEPLOY_DIR/opencode.json"
  [ "$output" = "false" ]
}

# ── adapter_start : passage du titre de session ───────────────────────────────

@test "adapter_start : passe --title si session_title fourni" {
  # Créer un mock opencode dans le PATH du sous-shell
  local mock_bin="$TEST_DIR/mock-bin"
  local oc_log="$TEST_DIR/oc_title.log"
  mkdir -p "$mock_bin"
  cat > "$mock_bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*" >> "$OPENCODE_LOG"
exit 0
OCEOF
  chmod +x "$mock_bin/opencode"

  run bash -c "
    export OPENCODE_LOG='$oc_log'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export PATH='$mock_bin:\$PATH'
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh'
    log_info()  { true; }
    log_error() { true; }
    adapter_start '$DEPLOY_DIR' '' '' '' '' 'mon titre de session'
  "
  [ "$status" -eq 0 ]
  grep -q -- '--title' "$oc_log"
  grep -q -- 'mon titre de session' "$oc_log"
}

@test "adapter_start : omet --title si session_title vide" {
  local mock_bin="$TEST_DIR/mock-bin2"
  local oc_log="$TEST_DIR/oc_notitle.log"
  mkdir -p "$mock_bin"
  cat > "$mock_bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*" >> "$OPENCODE_LOG"
exit 0
OCEOF
  chmod +x "$mock_bin/opencode"

  run bash -c "
    export OPENCODE_LOG='$oc_log'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    export API_KEYS_FILE='$API_KEYS_FILE'
    export PATH='$mock_bin:\$PATH'
    source '$BATS_TEST_DIRNAME/../scripts/common.sh'
    source '$BATS_TEST_DIRNAME/../scripts/adapters/opencode.adapter.sh'
    log_info()  { true; }
    log_error() { true; }
    adapter_start '$DEPLOY_DIR' '' '' '' '' ''
  "
  [ "$status" -eq 0 ]
  ! grep -q -- '--title' "$oc_log"
}

# ─────────────────────────────────────────────────────────────────────────────
# _build_provider_json — Approche B : bloc models{} pour litellm + fix openrouter
# ─────────────────────────────────────────────────────────────────────────────

@test "_build_provider_json : mammouth inclut bloc models avec default_model" {
  command -v jq &>/dev/null || skip "jq non disponible"
  printf '[PROJ-MAM]\nprovider=mammouth\napi_key=sk-test\nbase_url=https://api.mammouth.ai/v1\n' > "$API_KEYS_FILE"
  run _build_provider_json "mammouth" "sk-test" "https://api.mammouth.ai/v1" ""
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  echo "$output" | jq -e '.provider.litellm.models' >/dev/null 2>&1
}

@test "_build_provider_json : mammouth inclut name (label du provider)" {
  command -v jq &>/dev/null || skip "jq non disponible"
  run _build_provider_json "mammouth" "sk-test" "https://api.mammouth.ai/v1" ""
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  echo "$output" | jq -e '.provider.litellm.name' >/dev/null 2>&1
}

@test "_build_provider_json : ollama inclut bloc models" {
  command -v jq &>/dev/null || skip "jq non disponible"
  run _build_provider_json "ollama" "nokey" "http://localhost:11434/v1" ""
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  echo "$output" | jq -e '.provider.litellm.models' >/dev/null 2>&1
}

@test "_build_provider_json : openrouter génère un bloc provider valide" {
  command -v jq &>/dev/null || skip "jq non disponible"
  run _build_provider_json "openrouter" "sk-or-test" "" ""
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  echo "$output" | jq -e '.provider.openrouter.apiKey' >/dev/null 2>&1
}

@test "_build_provider_json : anthropic ne contient pas de bloc models (pas litellm)" {
  command -v jq &>/dev/null || skip "jq non disponible"
  run _build_provider_json "anthropic" "sk-ant-test" "" ""
  [ "$status" -eq 0 ]
  [ -n "$output" ]
  # Le bloc anthropic ne doit pas avoir de models
  local has_models
  has_models=$(echo "$output" | jq -r '.provider.anthropic.models // "null"')
  [ "$has_models" = "null" ]
}

@test "_build_provider_json : merge opencode_cache.provider ne supprime pas apiKey anthropic" {
  command -v jq &>/dev/null || skip "jq non disponible"
  # Simuler le merge effectué dans adapter_deploy_config
  local provider_json
  provider_json=$(_build_provider_json "anthropic" "sk-ant-secret" "" "")
  local cache_json='{"anthropic": {"options": {"setCacheKey": true}}}'

  local merged
  merged=$(jq -n \
    --argjson base "$provider_json" \
    --argjson cache "$cache_json" \
    '$base * {"provider": $cache}')

  # La clé API doit être préservée après le merge
  local api_key
  api_key=$(echo "$merged" | jq -r '.provider.anthropic.apiKey // empty')
  [ "$api_key" = "sk-ant-secret" ]
}
