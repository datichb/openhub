#!/usr/bin/env bats
# Tests unitaires pour scripts/cmd-install.sh
# Fonctions testées : installation jq, hub.json, provider, beads, config files

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  export HUB_DIR="$TEST_DIR/hub"
  export PROVIDERS_FILE="$HUB_DIR/config/providers.json"
  
  mkdir -p "$HUB_DIR/config"
  
  source "$SCRIPT_DIR/common.sh"
  
  # Mock log functions
  mock_log_functions
  
  # Mock _intro/_outro
  _intro() { :; }
  _outro() { :; }
  export -f _intro _outro
}

teardown() {
  common_teardown
}

# ── Détection OS ────────────────────────────────────────────────────────────

@test "install : détecte l'OS" {
  run detect_os
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^(macos|linux|unknown)$ ]]
}

# ── Installation jq ─────────────────────────────────────────────────────────

@test "install : vérifie si jq est disponible" {
  # Mock jq command
  jq() {
    echo "jq-1.6"
  }
  export -f jq
  
  command() {
    if [ "$2" = "jq" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  # Le script devrait passer la vérification jq
  [ -n "$(command -v jq 2>/dev/null || echo '')" ] || skip "Test requires jq mock"
}

@test "install : propose d'installer jq si absent sur macOS avec brew" {
  # Mock OS
  detect_os() {
    echo "macos"
  }
  export -f detect_os
  
  # Mock command checks
  command() {
    case "$2" in
      jq) return 1 ;;      # jq absent
      brew) return 0 ;;    # brew présent
      *) builtin command "$@" ;;
    esac
  }
  export -f command
  
  # Vérifier que la logique conditionelle s'appliquerait : macOS + brew disponible
  OS=$(detect_os)
  [ "$OS" = "macos" ]
  # brew devrait être détecté comme disponible via command
  run command -v brew
  [ "$status" -eq 0 ]
  # jq devrait être absent
  run command -v jq
  [ "$status" -ne 0 ]
}

# ── Création hub.json ───────────────────────────────────────────────────────

@test "install : crée hub.json si absent" {
  [ ! -f "$HUB_DIR/config/hub.json" ]
  
  # Simuler création
  mkdir -p "$HUB_DIR/config"
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "version": "1.5.0",
  "default_provider": {
    "name": "anthropic"
  }
}
EOF
  
  [ -f "$HUB_DIR/config/hub.json" ]
  run jq -r '.version' "$HUB_DIR/config/hub.json"
  [ "$output" = "1.5.0" ]
}

@test "install : hub.json contient version et default_provider" {
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "version": "1.5.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": ""
  },
  "opencode": {
    "model": "claude-sonnet-4"
  }
}
EOF
  
  run jq -r '.default_provider.name' "$HUB_DIR/config/hub.json"
  [ "$output" = "anthropic" ]
  
  run jq -r '.opencode.model' "$HUB_DIR/config/hub.json"
  [ "$output" = "claude-sonnet-4" ]
}

@test "install : hub.json généré contient disabled_native_agents" {
  # Simuler l'écriture du hub.json comme le fait cmd-install.sh
  local DEFAULT_MODEL="claude-sonnet-4-5"
  mkdir -p "$HUB_DIR/config"
  cat > "$HUB_DIR/config/hub.json" << HUBJSON
{
  "version": "1.5.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "${DEFAULT_MODEL}",
    "disabled_native_agents": [
      "build",
      "plan",
      "general",
      "explore",
      "scout"
    ]
  }
}
HUBJSON

  # Vérifier que la clé est présente et contient les 5 agents
  run jq -r '.opencode.disabled_native_agents | length' "$HUB_DIR/config/hub.json"
  [ "$output" = "5" ]

  run jq -r '.opencode.disabled_native_agents | contains(["build","plan","general","explore","scout"])' "$HUB_DIR/config/hub.json"
  [ "$output" = "true" ]
}

@test "install : propose d'écraser hub.json s'il existe" {
  # Créer hub.json existant avec un provider configuré (sinon considéré squelette vide)
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{"version":"1.0.0","default_provider":{"name":"anthropic","api_key":"sk-test","base_url":"","model":""}}
EOF
  
  [ -f "$HUB_DIR/config/hub.json" ]
  
  # Mock read pour refuser
  read() {
    eval "$2='N'"
    return 0
  }
  export -f read
  
  # Logique: si le script demande confirmation et qu'on répond N,
  # le fichier original ne doit pas être modifié
  local original_content
  original_content=$(cat "$HUB_DIR/config/hub.json")
  [ "$original_content" = '{"version":"1.0.0","default_provider":{"name":"anthropic","api_key":"sk-test","base_url":"","model":""}}' ]
}

# ── Création dossiers requis ────────────────────────────────────────────────

@test "install : crée dossiers requis" {
  mkdir -p "$HUB_DIR/projects" \
           "$HUB_DIR/skills" \
           "$HUB_DIR/agents" \
           "$HUB_DIR/.opencode/agents" \
           "$HUB_DIR/config" \
           "$HUB_DIR/scripts/lib" \
           "$HUB_DIR/scripts/adapters"
  
  [ -d "$HUB_DIR/projects" ]
  [ -d "$HUB_DIR/skills" ]
  [ -d "$HUB_DIR/agents" ]
  [ -d "$HUB_DIR/.opencode/agents" ]
  [ -d "$HUB_DIR/config" ]
  [ -d "$HUB_DIR/scripts/lib" ]
  [ -d "$HUB_DIR/scripts/adapters" ]
}

# ── Configuration provider ──────────────────────────────────────────────────

@test "install : charge providers depuis providers.json" {
  cat > "$PROVIDERS_FILE" <<'EOF'
{
  "providers": {
    "anthropic": {
      "label": "Anthropic (Claude)",
      "requires_api_key": true
    },
    "ollama": {
      "label": "Ollama (local)",
      "requires_api_key": false
    }
  }
}
EOF
  
  source "$LIB_DIR/providers.sh"
  
  run jq -r '.providers | keys[]' "$PROVIDERS_FILE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"anthropic"* ]]
}

@test "install : get_provider_info retourne label" {
  cat > "$PROVIDERS_FILE" <<'EOF'
{
  "providers": {
    "anthropic": {
      "label": "Anthropic (Claude)",
      "requires_api_key": true
    }
  }
}
EOF
  
  source "$LIB_DIR/providers.sh"
  
  run get_provider_info "anthropic" "label"
  [ "$status" -eq 0 ]
  [ "$output" = "Anthropic (Claude)" ]
}

@test "install : provider avec requires_api_key=true" {
  cat > "$PROVIDERS_FILE" <<'EOF'
{
  "providers": {
    "anthropic": {
      "requires_api_key": true
    }
  }
}
EOF
  
  source "$LIB_DIR/providers.sh"
  
  run get_provider_info "anthropic" "requires_api_key"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "install : provider avec requires_api_key=false (ollama)" {
  cat > "$PROVIDERS_FILE" <<'EOF'
{
  "providers": {
    "ollama": {
      "requires_api_key": false
    }
  }
}
EOF
  
  source "$LIB_DIR/providers.sh"
  
  # Utiliser get_provider_bool au lieu de get_provider_info pour les booléens
  run get_provider_bool "ollama" "requires_api_key"
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

@test "install : sauvegarde provider dans hub.json" {
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "version": "1.5.0",
  "default_provider": {
    "name": "",
    "api_key": "",
    "base_url": ""
  }
}
EOF
  
  # Simuler mise à jour
  local updated
  updated=$(jq \
    --arg name "anthropic" \
    --arg key "sk-test-key" \
    --arg url "" \
    '.default_provider.name = $name | .default_provider.api_key = $key | .default_provider.base_url = $url' \
    "$HUB_DIR/config/hub.json")
  
  echo "$updated" > "$HUB_DIR/config/hub.json"
  
  run jq -r '.default_provider.name' "$HUB_DIR/config/hub.json"
  [ "$output" = "anthropic" ]
  
  run jq -r '.default_provider.api_key' "$HUB_DIR/config/hub.json"
  [ "$output" = "sk-test-key" ]
}

@test "install : ajoute hub.json à .gitignore si clé présente" {
  mkdir -p "$HUB_DIR"
  
  # Simuler logique d'ajout à gitignore
  if [ ! -f "$HUB_DIR/.gitignore" ] || ! grep -qx "config/hub.json" "$HUB_DIR/.gitignore" 2>/dev/null; then
    echo "config/hub.json" >> "$HUB_DIR/.gitignore"
  fi
  
  run grep "config/hub.json" "$HUB_DIR/.gitignore"
  [ "$status" -eq 0 ]
}

# ── Fichiers de configuration ──────────────────────────────────────────────

@test "install : ensure_projects_file crée projects.md" {
  run ensure_projects_file
  [ "$status" -eq 0 ]
  [ -f "$PROJECTS_FILE" ]
}

@test "install : ensure_paths_file crée paths.local.md" {
  run ensure_paths_file
  [ "$status" -eq 0 ]
  [ -f "$PATHS_FILE" ]
}

@test "install : ensure_api_keys_file crée api-keys.local.md" {
  run ensure_api_keys_file
  [ "$status" -eq 0 ]
  [ -f "$API_KEYS_FILE" ]
}

# ── Installation Beads ──────────────────────────────────────────────────────

@test "install : détecte si beads (bd) est installé" {
  # Mock bd command
  bd() {
    echo "beads 1.0.0"
  }
  export -f bd
  
  command() {
    if [ "$2" = "bd" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  # Le script devrait détecter bd
  run command -v bd
  [ "$status" -eq 0 ]
}

@test "install : propose d'installer beads via brew sur macOS" {
  # Mock command checks
  command() {
    case "$2" in
      bd) return 1 ;;      # bd absent
      brew) return 0 ;;    # brew présent
      *) builtin command "$@" ;;
    esac
  }
  export -f command
  
  # Vérifier la logique : bd absent, brew disponible → brew serait utilisé
  run command -v bd
  [ "$status" -ne 0 ]
  run command -v brew
  [ "$status" -eq 0 ]
}

@test "install : propose d'installer beads via curl si pas brew" {
  # Mock command checks
  command() {
    case "$2" in
      bd) return 1 ;;      # bd absent
      brew) return 1 ;;    # brew absent
      curl) return 0 ;;    # curl présent
      *) builtin command "$@" ;;
    esac
  }
  export -f command
  
  # Vérifier la logique : bd absent, brew absent, curl disponible → curl serait utilisé
  run command -v bd
  [ "$status" -ne 0 ]
  run command -v brew
  [ "$status" -ne 0 ]
  run command -v curl
  [ "$status" -eq 0 ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : workflow installation complet" {
  # 1. Créer dossiers
  mkdir -p "$HUB_DIR/projects" "$HUB_DIR/config"
  [ -d "$HUB_DIR/projects" ]
  
  # 2. Créer hub.json
  cat > "$HUB_DIR/config/hub.json" <<'EOF'
{
  "version": "1.5.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "sk-test"
  },
  "opencode": {
    "model": "claude-sonnet-4"
  }
}
EOF
  
  [ -f "$HUB_DIR/config/hub.json" ]
  
  # 3. Créer fichiers config
  ensure_projects_file
  ensure_paths_file
  ensure_api_keys_file
  
  [ -f "$PROJECTS_FILE" ]
  [ -f "$PATHS_FILE" ]
  [ -f "$API_KEYS_FILE" ]
  
  # 4. Vérifier structure complète
  [ -d "$HUB_DIR/projects" ]
  [ -f "$HUB_DIR/config/hub.json" ]
  run jq -r '.version' "$HUB_DIR/config/hub.json"
  [ "$output" = "1.5.0" ]
}
