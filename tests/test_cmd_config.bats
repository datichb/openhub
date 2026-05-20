#!/usr/bin/env bats
# Tests pour les helpers de cmd-config.sh et remove_api_keys_section (common.sh)
# Stratégie : sourcer uniquement common.sh, puis redéfinir les helpers privés
#             (_ensure_api_keys_file, _remove_section, _write_section) inline
#             pour éviter les dépendances de chemin de cmd-config.sh.

setup() {
  TEST_DIR="$(mktemp -d)"

  # Sourcer common.sh pour les fonctions partagées
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Surcharger les chemins vers des fichiers temporaires de test
  API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  PROJECTS_FILE="$TEST_DIR/projects.md"
  PATHS_FILE="$TEST_DIR/paths.local.md"

  # Mocks des fonctions de log
  log_info()    { true; }
  log_success() { true; }
  log_warn()    { true; }
  log_error()   { true; }

  # Redéfinition des helpers privés de cmd-config.sh (même logique, sans dépendance de chemin)
  _ensure_api_keys_file() {
    if [ ! -f "$API_KEYS_FILE" ]; then
      mkdir -p "$(dirname "$API_KEYS_FILE")"
      printf '# Clés API — test\n' > "$API_KEYS_FILE"
    fi
  }

  _remove_section() {
    remove_api_keys_section "$1"
  }

  _write_section() {
    local id="$1" model="$2" provider="$3" api_key="$4" base_url="$5"
    _ensure_api_keys_file
    if api_keys_entry_exists "$id"; then
      _remove_section "$id"
    fi
    local tmp; tmp=$(mktemp)
    {
      echo ""
      echo "[${id}]"
      echo "model=${model}"
      echo "provider=${provider}"
      echo "api_key=${api_key}"
      [ -n "$base_url" ] && echo "base_url=${base_url}"
    } > "$tmp"
    cat "$tmp" >> "$API_KEYS_FILE"
    rm -f "$tmp"
  }
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── _write_section : écriture initiale ───────────────────────────────────────

@test "_write_section : crée une nouvelle entrée avec les 4 champs obligatoires" {
  _write_section "PROJ-A" "claude-opus-4-5" "anthropic" "sk-ant-abc123" ""
  run grep -F "[PROJ-A]" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "model=claude-opus-4-5" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "provider=anthropic" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "api_key=sk-ant-abc123" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

@test "_write_section : ajoute base_url uniquement si non vide" {
  _write_section "PROJ-B" "claude-sonnet-4-5" "litellm" "sk-bRf-xyz" "https://api.mammouth.ai/v1"
  run grep "base_url=https://api.mammouth.ai/v1" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

@test "_write_section : n'ajoute pas base_url si vide" {
  _write_section "PROJ-C" "claude-sonnet-4-5" "anthropic" "sk-ant-xyz" ""
  run grep "base_url" "$API_KEYS_FILE"
  [ "$status" -ne 0 ]
}

# ── _write_section : idempotence (mise à jour) ────────────────────────────────

@test "_write_section : remplace une entrée existante sans doublon" {
  _write_section "PROJ-A" "claude-opus-4-5" "anthropic" "sk-ant-old" ""
  _write_section "PROJ-A" "claude-sonnet-4-5" "anthropic" "sk-ant-new" ""

  run grep "sk-ant-old" "$API_KEYS_FILE"
  [ "$status" -ne 0 ]

  run grep "sk-ant-new" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]

  count=$(grep -cF "[PROJ-A]" "$API_KEYS_FILE")
  [ "$count" -eq 1 ]
}

@test "_write_section : préserve les autres sections lors d'une mise à jour" {
  _write_section "PROJ-X" "claude-opus-4-5" "anthropic" "sk-ant-x" ""
  _write_section "PROJ-Y" "claude-sonnet-4-5" "litellm" "sk-bRf-y" "https://api.y.ai/v1"
  _write_section "PROJ-X" "claude-haiku-4-5" "anthropic" "sk-ant-x2" ""

  run grep -F "[PROJ-Y]" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "sk-bRf-y" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

# ── remove_api_keys_section ───────────────────────────────────────────────────

@test "remove_api_keys_section : supprime la section demandée" {
  _write_section "PROJ-DEL" "claude-opus-4-5" "anthropic" "sk-ant-del" ""
  remove_api_keys_section "PROJ-DEL"

  run grep -F "[PROJ-DEL]" "$API_KEYS_FILE"
  [ "$status" -ne 0 ]
}

@test "remove_api_keys_section : ne touche pas aux autres sections" {
  _write_section "PROJ-KEEP" "claude-opus-4-5" "anthropic" "sk-ant-keep" ""
  _write_section "PROJ-DEL" "claude-sonnet-4-5" "anthropic" "sk-ant-del" ""
  remove_api_keys_section "PROJ-DEL"

  run grep -F "[PROJ-KEEP]" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "sk-ant-keep" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

@test "remove_api_keys_section : ne plante pas si la section est absente" {
  printf '# Fichier vide\n' > "$API_KEYS_FILE"
  run remove_api_keys_section "INEXISTANT"
  [ "$status" -eq 0 ]
}

@test "remove_api_keys_section : ne plante pas si le fichier est absent" {
  run remove_api_keys_section "PROJ-X"
  [ "$status" -eq 0 ]
}

# ── api_keys_entry_exists ─────────────────────────────────────────────────────

@test "api_keys_entry_exists : retourne 0 si la section existe" {
  _write_section "PROJ-EXISTS" "claude-opus-4-5" "anthropic" "sk-ant-e" ""
  run api_keys_entry_exists "PROJ-EXISTS"
  [ "$status" -eq 0 ]
}

@test "api_keys_entry_exists : retourne non-zero si la section est absente" {
  printf '[AUTRE]\nmodel=claude-haiku-4-5\nprovider=anthropic\napi_key=sk-ant-autre\n' > "$API_KEYS_FILE"
  run api_keys_entry_exists "PROJ-ABSENT"
  [ "$status" -ne 0 ]
}

@test "api_keys_entry_exists : retourne non-zero si le fichier est absent" {
  run api_keys_entry_exists "PROJ-X"
  [ "$status" -ne 0 ]
}

# ── Isolation de sections ────────────────────────────────────────────────────

@test "get_project_api_key : les valeurs de deux sections sont indépendantes" {
  _write_section "PROJ-1" "claude-opus-4-5" "anthropic" "sk-ant-111" ""
  _write_section "PROJ-2" "claude-sonnet-4-5" "litellm" "sk-bRf-222" "https://api.two.ai"

  run get_project_api_key "PROJ-1"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-ant-111" ]

  run get_project_api_key "PROJ-2"
  [ "$status" -eq 0 ]
  [ "$output" = "sk-bRf-222" ]
}

@test "get_project_api_base_url : retourne vide pour une section sans base_url" {
  _write_section "PROJ-1" "claude-opus-4-5" "anthropic" "sk-ant-111" ""
  _write_section "PROJ-2" "claude-sonnet-4-5" "litellm" "sk-bRf-222" "https://api.two.ai"

  run get_project_api_base_url "PROJ-1"
  [ "$output" = "" ]
}

# ── Fonctions sourcées depuis cmd-config.sh ──────────────────────────────────

# ── _validate_family_name ────────────────────────────────────────────────────

@test "_validate_family_name : accepte une famille existante (planning)" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  run _validate_family_name "planning"
  [ "$status" -eq 0 ]
}

@test "_validate_family_name : rejette une famille inexistante" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  run _validate_family_name "nonexistent-family"
  [ "$status" -ne 0 ]
}

# ── _validate_agent_name ─────────────────────────────────────────────────────

@test "_validate_agent_name : accepte un agent existant (reviewer)" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  run _validate_agent_name "reviewer"
  [ "$status" -eq 0 ]
}

@test "_validate_agent_name : rejette un agent inexistant" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  run _validate_agent_name "nonexistent-agent-xyz"
  [ "$status" -ne 0 ]
}

# ── _warn_unknown_model ──────────────────────────────────────────────────────

@test "_warn_unknown_model : pas de warning pour claude-opus-4" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  local warnings=()
  log_warn() { warnings+=("$*"); }
  _warn_unknown_model "claude-opus-4"
  [ ${#warnings[@]} -eq 0 ]
}

@test "_warn_unknown_model : pas de warning pour claude-sonnet-4-5" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  local warnings=()
  log_warn() { warnings+=("$*"); }
  _warn_unknown_model "claude-sonnet-4-5"
  [ ${#warnings[@]} -eq 0 ]
}

@test "_warn_unknown_model : warning pour modèle inconnu" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  local warnings=()
  log_warn() { warnings+=("$*"); }
  _warn_unknown_model "gpt-4o"
  [ ${#warnings[@]} -eq 1 ]
}

@test "_warn_unknown_model : warning pour modèle trop permissif (claude-opus-400-turbo)" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  local warnings=()
  log_warn() { warnings+=("$*"); }
  _warn_unknown_model "claude-opus-400-turbo"
  [ ${#warnings[@]} -eq 1 ]
}

# ── _upsert_api_keys_field : idempotence ─────────────────────────────────────

@test "_upsert_api_keys_field : double exécution ne duplique pas l'entrée" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  _ensure_api_keys_file
  printf '\n[TEST-IDEM]\nmodel=claude-sonnet-4-5\n' >> "$API_KEYS_FILE"

  _upsert_api_keys_field "TEST-IDEM" "agent_models.families.planning" "claude-opus-4"
  _upsert_api_keys_field "TEST-IDEM" "agent_models.families.planning" "claude-opus-4"

  count=$(grep -c "agent_models.families.planning" "$API_KEYS_FILE")
  [ "$count" -eq 1 ]
}

# ── Hub-level config set (integration via _cmd_set_hub) ──────────────────────

@test "_cmd_set_hub : family-model met à jour hub.json" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  HUB_CONFIG="$TEST_DIR/hub.json"
  printf '{"agent_models":{"families":{},"agents":{}}}\n' > "$HUB_CONFIG"

  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"

  _cmd_set_hub --family-model "planning=claude-opus-4"

  run jq -r '.agent_models.families.planning' "$HUB_CONFIG"
  [ "$output" = "claude-opus-4" ]
}

@test "_cmd_set_hub : agent-model met à jour hub.json" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  HUB_CONFIG="$TEST_DIR/hub.json"
  printf '{"agent_models":{"families":{},"agents":{}}}\n' > "$HUB_CONFIG"

  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/quality"
  touch "$CANONICAL_AGENTS_DIR/quality/debugger.md"

  _cmd_set_hub --agent-model "debugger=claude-sonnet-4-5"

  run jq -r '.agent_models.agents.debugger' "$HUB_CONFIG"
  [ "$output" = "claude-sonnet-4-5" ]
}

@test "_cmd_set_hub : famille inexistante → erreur" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"
  HUB_CONFIG="$TEST_DIR/hub.json"
  printf '{"agent_models":{"families":{},"agents":{}}}\n' > "$HUB_CONFIG"

  run _cmd_set_hub --family-model "nonexistent=claude-opus-4"
  [ "$status" -ne 0 ]
}

@test "_cmd_set_hub : agent inexistant → erreur" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"
  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/quality"
  HUB_CONFIG="$TEST_DIR/hub.json"
  printf '{"agent_models":{"families":{},"agents":{}}}\n' > "$HUB_CONFIG"

  run _cmd_set_hub --agent-model "fake-agent=claude-opus-4"
  [ "$status" -ne 0 ]
}

# ── Project-level config set ─────────────────────────────────────────────────

@test "cmd_set projet : --family-model met à jour api-keys.local.md" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"

  # Préparer le projet dans projects.md
  printf '[MY-APP]\nagents=reviewer\n' > "$PROJECTS_FILE"
  _ensure_api_keys_file
  printf '\n[MY-APP]\nmodel=claude-sonnet-4-5\n' >> "$API_KEYS_FILE"

  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"

  # Mocker normalize_project_id et require_project_id
  normalize_project_id() { echo "$1"; }
  require_project_id() { true; }

  cmd_set "MY-APP" --family-model "planning=claude-opus-4"

  run grep "agent_models.families.planning=claude-opus-4" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

# ── requires_api_key=false (providers sans clé OAuth) ────────────────────────

@test "cmd_set : provider requires_api_key=false — pas d'exit 1 si --api-key absent" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"

  # Préparer providers.json avec un provider sans clé
  PROVIDERS_FILE="$TEST_DIR/providers.json"
  printf '{"providers":{"no-key-provider":{"label":"No Key","requires_api_key":false,"requires_base_url":false}}}\n' > "$PROVIDERS_FILE"

  normalize_project_id() { echo "$1"; }
  require_project_id() { true; }

  # cmd_set doit se terminer sans exit 1 malgré l'absence de --api-key
  run bash -c "
    source \"$BATS_TEST_DIRNAME/../scripts/common.sh\"
    _CMD_CONFIG_SOURCE_ONLY=1 source \"$BATS_TEST_DIRNAME/../scripts/cmd-config.sh\"
    API_KEYS_FILE=\"$TEST_DIR/api-keys.local.md\"
    PROVIDERS_FILE=\"$TEST_DIR/providers.json\"
    normalize_project_id() { echo \"\$1\"; }
    require_project_id() { true; }
    path_exists() { return 1; }
    t() { echo \"\$1\"; }
    log_info() { true; }
    log_success() { true; }
    log_warn() { true; }
    log_error() { echo \"ERROR: \$*\" >&2; }
    cmd_set \"MY-PROJ\" --provider no-key-provider --model claude-sonnet-4-5
  "
  [ "$status" -eq 0 ]
}

@test "cmd_set : provider requires_api_key=false — api_key= vide écrit dans api-keys.local.md" {
  # Vérifier que _write_section accepte une clé vide sans casser le fichier
  _write_section "NO-KEY-PROJ" "claude-sonnet-4-5" "github-copilot" "" ""
  run grep "api_key=" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
  run grep "\[NO-KEY-PROJ\]" "$API_KEYS_FILE"
  [ "$status" -eq 0 ]
}

@test "cmd_set : provider requires_api_key=true sans --api-key → exit 1" {
  run bash -c "
    source \"$BATS_TEST_DIRNAME/../scripts/common.sh\"
    _CMD_CONFIG_SOURCE_ONLY=1 source \"$BATS_TEST_DIRNAME/../scripts/cmd-config.sh\"
    API_KEYS_FILE=\"$TEST_DIR/api-keys.local.md\"
    PROVIDERS_FILE=\"$TEST_DIR/providers.json\"
    printf '{\"providers\":{\"anthropic\":{\"label\":\"Anthropic\",\"requires_api_key\":true}}}\n' > \"$TEST_DIR/providers.json\"
    normalize_project_id() { echo \"\$1\"; }
    require_project_id() { true; }
    path_exists() { return 1; }
    t() { echo \"\$1\"; }
    log_info() { true; }
    log_success() { true; }
    log_warn() { true; }
    log_error() { echo \"ERROR: \$*\" >&2; }
    cmd_set \"MY-PROJ\" --provider anthropic --model claude-sonnet-4-5
  "
  [ "$status" -ne 0 ]
}

@test "cmd_set projet : --family-model idempotent — pas de doublon" {
  _CMD_CONFIG_SOURCE_ONLY=1 source "$BATS_TEST_DIRNAME/../scripts/cmd-config.sh"

  printf '[MY-APP]\nagents=reviewer\n' > "$PROJECTS_FILE"
  _ensure_api_keys_file
  printf '\n[MY-APP]\nmodel=claude-sonnet-4-5\n' >> "$API_KEYS_FILE"

  CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  mkdir -p "$CANONICAL_AGENTS_DIR/planning"

  normalize_project_id() { echo "$1"; }
  require_project_id() { true; }

  cmd_set "MY-APP" --family-model "planning=claude-opus-4"
  cmd_set "MY-APP" --family-model "planning=claude-opus-4"

  count=$(grep -c "agent_models.families.planning" "$API_KEYS_FILE")
  [ "$count" -eq 1 ]
}
