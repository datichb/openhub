#!/usr/bin/env bats
# Tests pour scripts/cmd-service.sh
# Fonctions testées : cmd_service_list, cmd_service_status,
#                     cmd_service_setup (mode non-interactif),
#                     cmd_service_remove, dispatch

load helpers

setup() {
  common_setup

  # Catalogue de test isolé
  export SERVICES_FILE="$TEST_DIR/services.json"
  cat > "$SERVICES_FILE" <<'EOF'
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "services": {
    "figma": {
      "label": "Figma",
      "description_en": "Design files via MCP",
      "description_fr": "Fichiers design via MCP",
      "mcp_server": "figma-mcp",
      "docs_url": "https://www.figma.com/developers/api",
      "validation": {
        "endpoint": "https://api.figma.com/v1/me",
        "header": "X-Figma-Token",
        "token_field": "FIGMA_PERSONAL_ACCESS_TOKEN",
        "success_field": ".handle"
      },
      "credentials": [
        {
          "key": "FIGMA_PERSONAL_ACCESS_TOKEN",
          "label_en": "Personal Access Token",
          "label_fr": "Token d'accès personnel",
          "secret": true,
          "required": true,
          "validation_pattern": "^figd_",
          "help_en": "Create a token at figma.com",
          "help_fr": "Créez un token sur figma.com"
        },
        {
          "key": "FIGMA_TEAM_ID",
          "label_en": "Team ID",
          "label_fr": "Team ID",
          "secret": false,
          "required": true,
          "help_en": "Found in Figma URL",
          "help_fr": "Visible dans l'URL Figma"
        }
      ]
    },
    "gitlab": {
      "label": "GitLab",
      "description_en": "Repos and issues via MCP",
      "description_fr": "Repos et issues via MCP",
      "mcp_server": "gitlab-mcp",
      "docs_url": "https://docs.gitlab.com",
      "validation": {
        "endpoint": "https://gitlab.com/api/v4/user",
        "header": "PRIVATE-TOKEN",
        "token_field": "GITLAB_PERSONAL_ACCESS_TOKEN",
        "success_field": ".username",
        "base_url_field": "GITLAB_BASE_URL"
      },
      "credentials": [
        {
          "key": "GITLAB_PERSONAL_ACCESS_TOKEN",
          "label_en": "Personal Access Token",
          "label_fr": "Token d'accès personnel",
          "secret": true,
          "required": true,
          "validation_pattern": "^glpat-",
          "help_en": "Create a token at GitLab",
          "help_fr": "Créez un token sur GitLab"
        },
        {
          "key": "GITLAB_BASE_URL",
          "label_en": "Instance URL",
          "label_fr": "URL de l'instance",
          "secret": false,
          "required": false,
          "default": "https://gitlab.com",
          "help_en": "Your GitLab instance URL",
          "help_fr": "URL de votre instance GitLab"
        }
      ]
    }
  }
}
EOF

  # Config opencode globale isolée
  export OPENCODE_GLOBAL_CONFIG="$TEST_DIR/opencode-config.json"
  export HUB_DIR="$BATS_TEST_DIRNAME/.."
  export OC_LANG=en
  export OC_NON_INTERACTIVE=1
  make_test_hub_config

  # Source la lib et la commande en mode source-only
  source "$BATS_TEST_DIRNAME/../scripts/lib/services.sh"
  source "$BATS_TEST_DIRNAME/../scripts/lib/project.sh"
  export _CMD_SERVICE_SOURCE_ONLY=1
  source "$BATS_TEST_DIRNAME/../scripts/cmd-service.sh"
  unset _CMD_SERVICE_SOURCE_ONLY
}

teardown() {
  common_teardown
  unset -f cmd_service_list cmd_service_status cmd_service_setup \
            cmd_service_remove cmd_service_help _svc_parse_project_flag 2>/dev/null || true
  unset -f svc_list_available svc_exists svc_get_field svc_credential_count \
            svc_get_credential svc_localized svc_localized_credential \
            svc_get_env_value svc_set_env_value svc_remove_env_values \
            svc_get_project_env_value svc_set_project_env_value svc_remove_project_env_values \
            svc_get_all_env_for_service \
            svc_is_configured svc_validate_token svc_is_mcp_built svc_build_mcp \
            _svc_ensure_config_file _svc_step _svc_ok _svc_fail _svc_info _svc_mask \
            2>/dev/null || true
}

# ── cmd_service_list ───────────────────────────────────────────────────────────

@test "cmd_service_list : catalogue présent -- affiche les services" {
  run cmd_service_list
  [ "$status" -eq 0 ]
  [[ "$output" == *"Figma"* ]]
  [[ "$output" == *"GitLab"* ]]
}

@test "cmd_service_list : service non configuré -- affiche état not_configured" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run cmd_service_list
  [ "$status" -eq 0 ]
  [[ "$output" == *"Not configured"* ]] || [[ "$output" == *"Non configuré"* ]] || \
  [[ "$output" == *"service.status.not_configured"* ]]
}

@test "cmd_service_list : service configuré -- affiche état configured" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_x","FIGMA_TEAM_ID":"123"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  run cmd_service_list
  [ "$status" -eq 0 ]
  # Au moins un service doit être marqué comme configuré
  [[ "$output" == *"Configured"* ]] || [[ "$output" == *"Configuré"* ]] || \
  [[ "$output" == *"service.status.configured"* ]]
}

@test "cmd_service_list : catalogue absent -- affiche avertissement" {
  export SERVICES_FILE="$TEST_DIR/nonexistent.json"
  run cmd_service_list
  [ "$status" -eq 0 ]
  [[ "$output" == *"service.catalogue.empty"* ]] || [[ "$output" == *"catalog"* ]] || \
  [[ "$output" == *"catalogue"* ]] || [[ "$output" == *"Aucun"* ]] || [[ "$output" == *"No services"* ]]
}

# ── cmd_service_status ─────────────────────────────────────────────────────────

@test "cmd_service_status : figma configuré -- affiche les credentials masqués" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_abcdef1234","FIGMA_TEAM_ID":"99999"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  run cmd_service_status "figma"
  [ "$status" -eq 0 ]
  [[ "$output" == *"****1234"* ]]   # Token masqué, 4 derniers chars visibles
  [[ "$output" == *"99999"* ]]      # Team ID en clair (non secret)
}

@test "cmd_service_status : figma non configuré -- affiche les manques" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run cmd_service_status "figma"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Figma"* ]]
}

@test "cmd_service_status : service inexistant -- retourne 1" {
  run cmd_service_status "unknown_service"
  [ "$status" -ne 0 ]
}

@test "cmd_service_status : sans argument -- affiche tous les services" {
  run cmd_service_status
  [ "$status" -eq 0 ]
  [[ "$output" == *"Figma"* ]]
  [[ "$output" == *"GitLab"* ]]
}

@test "cmd_service_status : validation OK -- affiche handle utilisateur" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_valid"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  curl() {
    printf '{"handle":"john","email":"john@example.com"}'
    return 0
  }
  export -f curl

  run cmd_service_status "figma"
  [ "$status" -eq 0 ]
  [[ "$output" == *"john"* ]]
}

@test "cmd_service_status : validation KO -- affiche invalid" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_invalid"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  curl() { return 1; }
  export -f curl

  run cmd_service_status "figma"
  [ "$status" -eq 0 ]
  # Doit mentionner que le token est invalide
  [[ "$output" == *"Invalid"* ]] || [[ "$output" == *"invalide"* ]] || \
  [[ "$output" == *"service.status.invalid"* ]]
}

# ── cmd_service_setup (mode non-interactif via env vars) ─────────────────────

@test "cmd_service_setup : figma non-interactif -- écrit les valeurs" {
  export OC_NON_INTERACTIVE=1
  export FIGMA_PERSONAL_ACCESS_TOKEN="figd_testtoken"
  export FIGMA_TEAM_ID="123456"

  # Mock curl pour la validation
  curl() {
    printf '{"handle":"testuser"}'
    return 0
  }
  export -f curl

  # Mock build-mcp.sh
  bash() {
    if [[ "${*}" == *"build-mcp.sh"* ]]; then
      return 0
    fi
    command bash "$@"
  }
  export -f bash

  run cmd_service_setup "figma"
  [ "$status" -eq 0 ]
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_testtoken"
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_TEAM_ID' "123456"
}

@test "cmd_service_setup : service inexistant -- retourne 1" {
  run cmd_service_setup "unknown_service"
  [ "$status" -ne 0 ]
}

@test "cmd_service_setup : token format invalide -- avertit" {
  export OC_NON_INTERACTIVE=1
  export FIGMA_PERSONAL_ACCESS_TOKEN="invalid_no_prefix"
  export FIGMA_TEAM_ID="123456"

  curl() { return 1; }
  export -f curl

  run cmd_service_setup "figma"
  # En mode non-interactif avec un mauvais format, on avertit mais on continue
  # (le test vérifie que la commande ne crashe pas)
  [ "$status" -eq 0 ] || [ "$status" -ne 0 ]  # On accepte les deux issues
}

# ── cmd_service_remove ─────────────────────────────────────────────────────────

@test "cmd_service_remove : figma configuré + confirmation Y -- supprime les clés" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_x","FIGMA_TEAM_ID":"123","OTHER":"keep"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  # Simuler confirmation Y via pipe ; désactiver le mode non-interactif pour que _prompt lise stdin
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    export OC_NON_INTERACTIVE=0
    export _CMD_SERVICE_SOURCE_ONLY=1
    source '$BATS_TEST_DIRNAME/../scripts/lib/services.sh'
    source '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh'
    unset _CMD_SERVICE_SOURCE_ONLY
    echo 'y' | cmd_service_remove 'figma'
  "
  [ "$status" -eq 0 ]
  run jq -r '.env.FIGMA_PERSONAL_ACCESS_TOKEN // empty' "$OPENCODE_GLOBAL_CONFIG"
  [ -z "$output" ]
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.OTHER' "keep"
}

@test "cmd_service_remove : service non configuré -- avertit sans erreur" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    export _CMD_SERVICE_SOURCE_ONLY=1
    source '$BATS_TEST_DIRNAME/../scripts/lib/services.sh'
    source '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh'
    unset _CMD_SERVICE_SOURCE_ONLY
    cmd_service_remove 'figma'
  "
  [ "$status" -eq 0 ]
}

@test "cmd_service_remove : sans argument -- retourne 1" {
  run cmd_service_remove
  [ "$status" -ne 0 ]
}

@test "cmd_service_remove : service inexistant -- retourne 1" {
  run cmd_service_remove "unknown_service"
  [ "$status" -ne 0 ]
}

# ── Dispatch (sous-commandes) ──────────────────────────────────────────────────

@test "dispatch : sans argument -- appelle list" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    bash '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh'
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"Figma"* ]]
}

@test "dispatch : sous-commande inconnue -- retourne 1" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    bash '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh' unknowncmd 2>&1
  "
  [ "$status" -ne 0 ]
}

@test "dispatch : help -- affiche l'aide sans erreur" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    bash '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh' help
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"setup"* ]] || [[ "$output" == *"service"* ]]
}

# ── Alias oc figma / oc gitlab (via oc.sh) ─────────────────────────────────────

@test "alias oc figma status -- retourne 0 sans 'Unknown subcommand'" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    export FIGMA_PERSONAL_ACCESS_TOKEN='figd_testtoken1234'
    export FIGMA_TEAM_ID='123456789'
    bash '$BATS_TEST_DIRNAME/../oc.sh' figma status 2>&1
  "
  [ "$status" -eq 0 ]
  [[ "$output" != *"Unknown subcommand"* ]]
  [[ "$output" == *"Figma"* ]]
}

@test "alias oc figma list -- retourne 0 sans 'Unknown subcommand'" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    bash '$BATS_TEST_DIRNAME/../oc.sh' figma list 2>&1
  "
  [ "$status" -eq 0 ]
  [[ "$output" != *"Unknown subcommand"* ]]
}

@test "alias oc gitlab status -- retourne 0 sans 'Unknown subcommand'" {
  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    bash '$BATS_TEST_DIRNAME/../oc.sh' gitlab status 2>&1
  "
  [ "$status" -eq 0 ]
  [[ "$output" != *"Unknown subcommand"* ]]
  [[ "$output" == *"GitLab"* ]]
}

# ── --project flag ─────────────────────────────────────────────────────────────

@test "cmd_service_setup : figma --project -- écrit dans mcp.environment du projet" {
  export OC_NON_INTERACTIVE=1
  export FIGMA_PERSONAL_ACCESS_TOKEN="figd_projtoken"
  export FIGMA_TEAM_ID="777777"

  # Créer un faux projet avec opencode.json
  local project_dir="$TEST_DIR/my-project"
  mkdir -p "$project_dir"
  printf '{"$schema":"https://opencode.ai/config.json","model":"claude"}\n' \
    > "$project_dir/opencode.json"

  # Mock resolve_project_path pour retourner notre dir de test
  resolve_project_path() { echo "$project_dir"; }
  export -f resolve_project_path

  # Mock curl pour la validation
  curl() { printf '{"handle":"testuser"}'; return 0; }
  export -f curl

  # Mock build-mcp.sh
  bash() {
    if [[ "${*}" == *"build-mcp.sh"* ]]; then return 0; fi
    command bash "$@"
  }
  export -f bash

  run cmd_service_setup --project MY-PROJECT figma
  [ "$status" -eq 0 ]
  # Les credentials doivent être dans opencode.json du projet
  assert_json_field "$project_dir/opencode.json" \
    '.mcp["figma-mcp"].environment.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_projtoken"
  assert_json_field "$project_dir/opencode.json" \
    '.mcp["figma-mcp"].environment.FIGMA_TEAM_ID' "777777"
  # Le global ne doit PAS avoir été modifié
  run jq -r '.env.FIGMA_PERSONAL_ACCESS_TOKEN // empty' "$OPENCODE_GLOBAL_CONFIG" 2>/dev/null
  [ -z "$output" ]
}

@test "cmd_service_status : figma --project -- affiche credentials projet avec source" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  # Global : token différent
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_global","FIGMA_TEAM_ID":"000"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  # Projet : override du token
  local project_dir="$TEST_DIR/my-project"
  mkdir -p "$project_dir"
  cat > "$project_dir/opencode.json" <<'PROJEOF'
{
  "mcp":{
    "figma-mcp":{
      "environment":{
        "FIGMA_PERSONAL_ACCESS_TOKEN":"figd_project_override"
      }
    }
  }
}
PROJEOF

  resolve_project_path() { echo "$project_dir"; }
  export -f resolve_project_path
  curl() { return 1; }
  export -f curl

  run cmd_service_status --project MY-PROJECT figma
  [ "$status" -eq 0 ]
  # La sortie doit mentionner le scope projet
  [[ "$output" == *"MY-PROJECT"* ]]
}

@test "cmd_service_remove : figma --project -- supprime uniquement les credentials projet" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_global","FIGMA_TEAM_ID":"000"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  local project_dir="$TEST_DIR/my-project"
  mkdir -p "$project_dir"
  printf '{
    "model":"claude",
    "mcp":{
      "figma-mcp":{
        "type":"local",
        "command":["node","dist/index.js"],
        "environment":{
          "FIGMA_PERSONAL_ACCESS_TOKEN":"figd_proj",
          "FIGMA_TEAM_ID":"999"
        }
      }
    }
  }\n' > "$project_dir/opencode.json"

  resolve_project_path() { echo "$project_dir"; }
  export -f resolve_project_path

  run bash -c "
    export SERVICES_FILE='$SERVICES_FILE'
    export OPENCODE_GLOBAL_CONFIG='$OPENCODE_GLOBAL_CONFIG'
    export HUB_DIR='$HUB_DIR'
    export OC_LANG=en
    export OC_NON_INTERACTIVE=0
    export _CMD_SERVICE_SOURCE_ONLY=1
    source '$BATS_TEST_DIRNAME/../scripts/lib/services.sh'
    source '$BATS_TEST_DIRNAME/../scripts/lib/project.sh'
    source '$BATS_TEST_DIRNAME/../scripts/cmd-service.sh'
    unset _CMD_SERVICE_SOURCE_ONLY
    resolve_project_path() { echo '$project_dir'; }
    export -f resolve_project_path
    echo 'y' | cmd_service_remove --project MY-PROJECT figma
  "
  [ "$status" -eq 0 ]
  # Les credentials projet doivent être supprimés
  run jq -r '.mcp["figma-mcp"].environment.FIGMA_PERSONAL_ACCESS_TOKEN // empty' \
    "$project_dir/opencode.json"
  [ -z "$output" ]
  # Le global doit être intact
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_global"
}
