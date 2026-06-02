#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/services.sh
# Fonctions testées : svc_list_available, svc_exists, svc_get_field,
#                     svc_credential_count, svc_get_credential,
#                     svc_localized, svc_localized_credential,
#                     svc_get_env_value, svc_set_env_value,
#                     svc_remove_env_values, svc_is_configured,
#                     svc_validate_token, svc_is_mcp_built

load helpers

setup() {
  common_setup

  # Créer un catalogue de test minimal (indépendant du vrai config/services.json)
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

  # Config opencode globale isolée dans TEST_DIR
  export OPENCODE_GLOBAL_CONFIG="$TEST_DIR/opencode-config.json"
  export HUB_DIR="$BATS_TEST_DIRNAME/.."

  source "$BATS_TEST_DIRNAME/../scripts/lib/services.sh"
}

teardown() {
  common_teardown
  unset -f svc_list_available svc_exists svc_get_field svc_credential_count \
            svc_get_credential svc_localized svc_localized_credential \
            svc_get_env_value svc_set_env_value svc_remove_env_values \
            svc_is_configured svc_validate_token svc_is_mcp_built svc_build_mcp \
            _svc_ensure_config_file 2>/dev/null || true
}

# ── svc_list_available ─────────────────────────────────────────────────────────

@test "svc_list_available : catalogue valide -- retourne les IDs" {
  run svc_list_available
  [ "$status" -eq 0 ]
  [[ "$output" == *"figma"* ]]
  [[ "$output" == *"gitlab"* ]]
}

@test "svc_list_available : catalogue absent -- retourne 1" {
  export SERVICES_FILE="$TEST_DIR/nonexistent.json"
  run svc_list_available
  [ "$status" -ne 0 ]
}

# ── svc_exists ─────────────────────────────────────────────────────────────────

@test "svc_exists : service existant -- retourne 0" {
  run svc_exists "figma"
  [ "$status" -eq 0 ]
}

@test "svc_exists : service inexistant -- retourne 1" {
  run svc_exists "github"
  [ "$status" -ne 0 ]
}

# ── svc_get_field ──────────────────────────────────────────────────────────────

@test "svc_get_field : label figma -- retourne Figma" {
  run svc_get_field "figma" "label"
  [ "$status" -eq 0 ]
  [ "$output" = "Figma" ]
}

@test "svc_get_field : mcp_server figma -- retourne figma-mcp" {
  run svc_get_field "figma" "mcp_server"
  [ "$status" -eq 0 ]
  [ "$output" = "figma-mcp" ]
}

@test "svc_get_field : champ inexistant -- retourne vide" {
  run svc_get_field "figma" "nonexistent_field"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── svc_credential_count ───────────────────────────────────────────────────────

@test "svc_credential_count : figma -- retourne 2" {
  run svc_credential_count "figma"
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

@test "svc_credential_count : gitlab -- retourne 2" {
  run svc_credential_count "gitlab"
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

# ── svc_get_credential ─────────────────────────────────────────────────────────

@test "svc_get_credential : figma index 0 key -- retourne FIGMA_PERSONAL_ACCESS_TOKEN" {
  run svc_get_credential "figma" 0 "key"
  [ "$status" -eq 0 ]
  [ "$output" = "FIGMA_PERSONAL_ACCESS_TOKEN" ]
}

@test "svc_get_credential : figma index 0 secret -- retourne true" {
  run svc_get_credential "figma" 0 "secret"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "svc_get_credential : figma index 1 key -- retourne FIGMA_TEAM_ID" {
  run svc_get_credential "figma" 1 "key"
  [ "$status" -eq 0 ]
  [ "$output" = "FIGMA_TEAM_ID" ]
}

@test "svc_get_credential : gitlab index 1 default -- retourne https://gitlab.com" {
  run svc_get_credential "gitlab" 1 "default"
  [ "$status" -eq 0 ]
  [ "$output" = "https://gitlab.com" ]
}

@test "svc_get_credential : champ inexistant -- retourne vide" {
  run svc_get_credential "figma" 0 "nonexistent"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── svc_localized ──────────────────────────────────────────────────────────────

@test "svc_localized : OC_LANG=fr -- retourne champ_fr" {
  export OC_LANG=fr
  run svc_localized "figma" "description"
  [ "$status" -eq 0 ]
  [ "$output" = "Fichiers design via MCP" ]
}

@test "svc_localized : OC_LANG=en -- retourne champ_en" {
  export OC_LANG=en
  run svc_localized "figma" "description"
  [ "$status" -eq 0 ]
  [ "$output" = "Design files via MCP" ]
}

# ── svc_localized_credential ───────────────────────────────────────────────────

@test "svc_localized_credential : OC_LANG=fr index 0 label -- retourne label_fr" {
  export OC_LANG=fr
  run svc_localized_credential "figma" 0 "label"
  [ "$status" -eq 0 ]
  [ "$output" = "Token d'accès personnel" ]
}

@test "svc_localized_credential : OC_LANG=en index 0 help -- retourne help_en" {
  export OC_LANG=en
  run svc_localized_credential "figma" 0 "help"
  [ "$status" -eq 0 ]
  [ "$output" = "Create a token at figma.com" ]
}

# ── svc_get_env_value / svc_set_env_value ──────────────────────────────────────

@test "svc_set_env_value : config absente -- crée le fichier" {
  [ ! -f "$OPENCODE_GLOBAL_CONFIG" ]
  svc_set_env_value "FIGMA_PERSONAL_ACCESS_TOKEN" "figd_test123"
  [ -f "$OPENCODE_GLOBAL_CONFIG" ]
  assert_json_valid "$OPENCODE_GLOBAL_CONFIG"
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_test123"
}

@test "svc_set_env_value : config existante -- préserve les autres clés" {
  # Créer une config avec une clé existante
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"EXISTING_KEY":"existing_value"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  svc_set_env_value "FIGMA_PERSONAL_ACCESS_TOKEN" "figd_new"

  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.EXISTING_KEY' "existing_value"
  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_new"
}

@test "svc_set_env_value : mise à jour d'une valeur existante" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_old"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  svc_set_env_value "FIGMA_PERSONAL_ACCESS_TOKEN" "figd_new"

  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.FIGMA_PERSONAL_ACCESS_TOKEN' "figd_new"
}

@test "svc_get_env_value : clé présente -- retourne la valeur" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_abc"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_get_env_value "FIGMA_PERSONAL_ACCESS_TOKEN"
  [ "$status" -eq 0 ]
  [ "$output" = "figd_abc" ]
}

@test "svc_get_env_value : clé absente -- retourne vide" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_get_env_value "NONEXISTENT_KEY"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "svc_get_env_value : fichier absent -- retourne 1" {
  run svc_get_env_value "FIGMA_PERSONAL_ACCESS_TOKEN"
  [ "$status" -ne 0 ]
}

# ── svc_remove_env_values ──────────────────────────────────────────────────────

@test "svc_remove_env_values : service configuré -- supprime ses clés" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_x","FIGMA_TEAM_ID":"123","OTHER":"keep"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  svc_remove_env_values "figma"

  assert_json_field "$OPENCODE_GLOBAL_CONFIG" '.env.OTHER' "keep"
  # Les clés Figma doivent avoir été supprimées (valeur null → empty via jq)
  run jq -r '.env.FIGMA_PERSONAL_ACCESS_TOKEN // empty' "$OPENCODE_GLOBAL_CONFIG"
  [ -z "$output" ]
  run jq -r '.env.FIGMA_TEAM_ID // empty' "$OPENCODE_GLOBAL_CONFIG"
  [ -z "$output" ]
}

@test "svc_remove_env_values : config absente -- ne plante pas" {
  run svc_remove_env_values "figma"
  [ "$status" -eq 0 ]
}

# ── svc_is_configured ─────────────────────────────────────────────────────────

@test "svc_is_configured : toutes clés requises présentes -- retourne 0" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_x","FIGMA_TEAM_ID":"123"}}\n' \
    > "$OPENCODE_GLOBAL_CONFIG"

  run svc_is_configured "figma"
  [ "$status" -eq 0 ]
}

@test "svc_is_configured : clé requise manquante -- retourne 1" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_x"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_is_configured "figma"
  [ "$status" -ne 0 ]
}

@test "svc_is_configured : service non configuré du tout -- retourne 1" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_is_configured "figma"
  [ "$status" -ne 0 ]
}

@test "svc_is_configured : gitlab avec clé optionnelle absente -- retourne 0" {
  # GITLAB_BASE_URL est required=false → pas obligatoire
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"GITLAB_PERSONAL_ACCESS_TOKEN":"glpat-test"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_is_configured "gitlab"
  [ "$status" -eq 0 ]
}

# ── svc_validate_token ─────────────────────────────────────────────────────────

@test "svc_validate_token : mock curl OK -- retourne 0 et le handle" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_test"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  # Mock curl qui retourne une réponse Figma valide
  curl() {
    if [[ "$*" == *"figma.com"* ]]; then
      printf '{"handle":"testuser","email":"test@example.com"}'
      return 0
    fi
    return 1
  }
  export -f curl

  run svc_validate_token "figma"
  [ "$status" -eq 0 ]
  [ "$output" = "testuser" ]
}

@test "svc_validate_token : mock curl KO -- retourne 1" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{"FIGMA_PERSONAL_ACCESS_TOKEN":"figd_invalid"}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  curl() { return 1; }
  export -f curl

  run svc_validate_token "figma"
  [ "$status" -ne 0 ]
}

@test "svc_validate_token : token absent -- retourne 1" {
  mkdir -p "$(dirname "$OPENCODE_GLOBAL_CONFIG")"
  printf '{"env":{}}\n' > "$OPENCODE_GLOBAL_CONFIG"

  run svc_validate_token "figma"
  [ "$status" -ne 0 ]
}

@test "svc_validate_token : service sans endpoint -- retourne 0 (skip)" {
  # Créer un service sans validation dans le catalogue
  export SERVICES_FILE="$TEST_DIR/services_no_validation.json"
  cat > "$SERVICES_FILE" <<'EOF'
{
  "services": {
    "novalidation": {
      "label": "NoValidation",
      "credentials": [
        {"key": "NO_VAL_KEY", "secret": false, "required": true}
      ]
    }
  }
}
EOF
  run svc_validate_token "novalidation"
  [ "$status" -eq 0 ]
}

# ── svc_is_mcp_built ───────────────────────────────────────────────────────────

@test "svc_is_mcp_built : dist/index.js présent -- retourne 0" {
  mkdir -p "$HUB_DIR/servers/figma-mcp/dist"
  touch "$HUB_DIR/servers/figma-mcp/dist/index.js"

  run svc_is_mcp_built "figma"
  [ "$status" -eq 0 ]
}

@test "svc_is_mcp_built : dist absent -- retourne 1" {
  # S'assurer que le dossier dist n'existe pas (on utilise HUB_DIR = TEST_DIR)
  export HUB_DIR="$TEST_DIR"

  run svc_is_mcp_built "figma"
  [ "$status" -ne 0 ]
}

@test "svc_is_mcp_built : service sans mcp_server -- retourne 0 (skip)" {
  export SERVICES_FILE="$TEST_DIR/services_no_mcp.json"
  cat > "$SERVICES_FILE" <<'EOF'
{
  "services": {
    "nomcp": {
      "label": "NoMCP",
      "credentials": []
    }
  }
}
EOF
  run svc_is_mcp_built "nomcp"
  [ "$status" -eq 0 ]
}
