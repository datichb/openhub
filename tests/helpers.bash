#!/usr/bin/env bash
# Helpers partagés pour les tests BATS
# Usage : load helpers (au début du fichier .bats)

# ── Création de fixtures ──────────────────────────────────────────────────────

# Crée un projet minimal dans projects.md
# Usage : make_test_project "PROJ-ID" "Nom du Projet" "Stack"
make_test_project() {
  local id="${1:?ID requis}"
  local name="${2:-Test Project}"
  local stack="${3:-Test}"
  
  cat >> "$PROJECTS_FILE" <<EOF

## $id
- Nom : $name
- Stack : $stack
EOF
}

# Crée un projet complet avec tous les champs
# Usage : make_full_test_project "PROJ-ID" "Nom" "Stack" "gitlab" "user/repo"
make_full_test_project() {
  local id="${1:?ID requis}"
  local name="${2:-Test Project}"
  local stack="${3:-Test}"
  local tracker="${4:-}"
  local repo="${5:-}"
  
  cat >> "$PROJECTS_FILE" <<EOF

## $id
- Nom : $name
- Stack : $stack
EOF
  
  [ -n "$tracker" ] && echo "- Tracker : $tracker" >> "$PROJECTS_FILE"
  [ -n "$repo" ] && echo "- Repo : $repo" >> "$PROJECTS_FILE"
}

# Crée un agent minimal dans agents/
# Usage : make_test_agent "agent-id" "family" ["skill1" "skill2"]
make_test_agent() {
  local id="${1:?ID requis}"
  local family="${2:-planning}"
  shift 2
  local skills=("$@")
  
  # Construire la liste des skills
  local skills_array="[]"
  if [ ${#skills[@]} -gt 0 ]; then
    skills_array="["
    for i in "${!skills[@]}"; do
      skills_array+="\"${skills[$i]}\""
      [ $i -lt $((${#skills[@]} - 1)) ] && skills_array+=", "
    done
    skills_array+="]"
  fi
  
  mkdir -p "$TEST_DIR/agents/$family"
  cat > "$TEST_DIR/agents/$family/${id}.md" <<EOF
---
id: $id
label: $(echo "$id" | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($i,1,1)),$i)}1')
description: Agent de test $id
targets: [opencode]
skills: $skills_array
---

# Agent $id

Corps de l'agent de test.
EOF
}

# Crée un agent avec permissions personnalisées
# Usage : make_test_agent_with_permissions "agent-id" "family" "websearch: allow" "webfetch: allow"
make_test_agent_with_permissions() {
  local id="${1:?ID requis}"
  local family="${2:-planning}"
  shift 2
  local permissions=("$@")
  
  mkdir -p "$TEST_DIR/agents/$family"
  cat > "$TEST_DIR/agents/$family/${id}.md" <<EOF
---
id: $id
label: $(echo "$id" | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($i,1,1)),$i)}1')
description: Agent de test $id
targets: [opencode]
skills: []
permission:
EOF
  
  for perm in "${permissions[@]}"; do
    echo "  $perm" >> "$TEST_DIR/agents/$family/${id}.md"
  done
  
  cat >> "$TEST_DIR/agents/$family/${id}.md" <<'EOF'
---

# Agent avec permissions

Corps de l'agent.
EOF
}

# Crée une skill minimale
# Usage : make_test_skill "skill-id" "family"
make_test_skill() {
  local id="${1:?ID requis}"
  local family="${2:-shared}"
  
  mkdir -p "$TEST_DIR/skills/$family"
  cat > "$TEST_DIR/skills/$family/${id}.md" <<EOF
---
id: $id
description: Skill de test $id
---

# Skill $id

Contenu de la skill de test.
EOF
}

# Crée un hub.json minimal
# Usage : make_test_hub_config ["model"]
make_test_hub_config() {
  local model="${1:-claude-sonnet-4-5}"
  
  mkdir -p "$(dirname "$HUB_CONFIG")"
  cat > "$HUB_CONFIG" <<EOF
{
  "version": "test",
  "default_target": "opencode",
  "active_targets": ["opencode"],
  "opencode": {
    "model": "$model",
    "disabled_native_agents": []
  },
  "cli": {
    "language": "fr"
  }
}
EOF
}

# Crée un hub.json avec agent_models
# Usage : make_test_hub_config_with_agent_models
make_test_hub_config_with_agent_models() {
  mkdir -p "$(dirname "$HUB_CONFIG")"
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "default_target": "opencode",
  "active_targets": ["opencode"],
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": []
  },
  "agent_models": {
    "families": {},
    "agents": {}
  },
  "cli": {
    "language": "fr"
  }
}
EOF
}

# Crée un fichier api-keys.local.md minimal
# Usage : make_test_api_keys ["provider" "api_key"]
make_test_api_keys() {
  local provider="${1:-anthropic}"
  local api_key="${2:-test-key-123}"
  
  mkdir -p "$(dirname "$API_KEYS_FILE")"
  cat > "$API_KEYS_FILE" <<EOF
# API Keys de test

provider: $provider
api_key: $api_key
EOF
}

# ── Assertions personnalisées ─────────────────────────────────────────────────

# Vérifie qu'un fichier contient un pattern
# Usage : assert_file_contains "/path/to/file" "pattern"
assert_file_contains() {
  local file="${1:?fichier requis}"
  local pattern="${2:?pattern requis}"
  
  if ! grep -qF -- "$pattern" "$file" 2>/dev/null; then
    echo "❌ Pattern '$pattern' non trouvé dans $file" >&2
    echo "Contenu du fichier :" >&2
    cat "$file" >&2
    return 1
  fi
}

# Vérifie qu'un fichier NE contient PAS un pattern
# Usage : assert_file_not_contains "/path/to/file" "pattern"
assert_file_not_contains() {
  local file="${1:?fichier requis}"
  local pattern="${2:?pattern requis}"
  
  if grep -qF -- "$pattern" "$file" 2>/dev/null; then
    echo "❌ Pattern '$pattern' trouvé dans $file (ne devrait pas)" >&2
    echo "Lignes correspondantes :" >&2
    grep -F -- "$pattern" "$file" >&2
    return 1
  fi
}

# Vérifie qu'un fichier JSON est valide
# Usage : assert_json_valid "/path/to/file.json"
assert_json_valid() {
  local file="${1:?fichier requis}"
  
  if ! command -v jq &>/dev/null; then
    echo "⚠️  jq non disponible, skip validation JSON" >&2
    return 0
  fi
  
  if ! jq . "$file" > /dev/null 2>&1; then
    echo "❌ JSON invalide dans $file" >&2
    echo "Contenu :" >&2
    cat "$file" >&2
    return 1
  fi
}

# Vérifie qu'un champ JSON a une valeur spécifique
# Usage : assert_json_field "/path/to/file.json" ".key.subkey" "expected_value"
assert_json_field() {
  local file="${1:?fichier requis}"
  local jq_path="${2:?chemin jq requis}"
  local expected="${3:?valeur attendue requise}"
  
  if ! command -v jq &>/dev/null; then
    echo "⚠️  jq non disponible, skip assertion JSON" >&2
    return 0
  fi
  
  local actual
  actual=$(jq -r "$jq_path" "$file" 2>/dev/null)
  
  if [ "$actual" != "$expected" ]; then
    echo "❌ Champ $jq_path : attendu '$expected', obtenu '$actual'" >&2
    echo "Fichier : $file" >&2
    return 1
  fi
}

# Compte les occurrences d'un pattern dans un fichier
# Usage : count_occurrences "/path/to/file" "pattern"
count_occurrences() {
  local file="${1:?fichier requis}"
  local pattern="${2:?pattern requis}"
  
  grep -c "$pattern" "$file" 2>/dev/null || echo 0
}

# ── Mocks communs ─────────────────────────────────────────────────────────────

# Mock silencieux des fonctions log_*
# Usage : mock_log_functions
mock_log_functions() {
  log_info() { true; }
  log_success() { true; }
  log_warn() { true; }
  log_error() { true; }
  export -f log_info log_success log_warn log_error
}

# Mock de bd (Beads) avec capture des appels
# Usage : mock_bd_with_log "/path/to/logfile"
mock_bd_with_log() {
  export BD_MOCK_LOG="${1:?log file requis}"
  touch "$BD_MOCK_LOG"
  
  bd() {
    echo "bd $*" >> "$BD_MOCK_LOG"
    return 0
  }
  export -f bd
}

# Mock de git avec capture des appels
# Usage : mock_git_with_log "/path/to/logfile"
mock_git_with_log() {
  export GIT_MOCK_LOG="${1:?log file requis}"
  touch "$GIT_MOCK_LOG"
  
  git() {
    echo "git $*" >> "$GIT_MOCK_LOG"
    
    # Comportements par défaut utiles
    case "${1:-}" in
      "rev-parse")
        [ "${2:-}" = "--show-toplevel" ] && echo "$TEST_DIR/fake-project"
        return 0
        ;;
      "remote")
        return 1  # Pas de remote configuré par défaut
        ;;
      *)
        return 0
        ;;
    esac
  }
  export -f git
}

# Mock de get_hub_version (utilisé dans prompt-builder)
# Usage : mock_get_hub_version ["version"]
mock_get_hub_version() {
  local version="${1:-test}"
  
  get_hub_version() {
    echo "$version"
  }
  export -f get_hub_version
}

# ── Utilitaires ───────────────────────────────────────────────────────────────

# Compte les lignes dans un fichier (version portable)
# Usage : count_lines "/path/to/file"
count_lines() {
  local file="${1:?fichier requis}"
  wc -l < "$file" | tr -d ' '
}

# Vérifie si une commande existe
# Usage : command_exists "jq"
command_exists() {
  command -v "$1" &>/dev/null
}

# Skip un test si une commande n'existe pas
# Usage : require_command "jq" ou require_command "jq" "Message personnalisé"
require_command() {
  local cmd="${1:?commande requise}"
  local msg="${2:-$cmd non disponible}"
  
  if ! command_exists "$cmd"; then
    skip "$msg"
  fi
}

# Nettoie les espaces en début/fin de chaîne
# Usage : trim "  texte  "
trim() {
  local text="$1"
  text="${text#"${text%%[![:space:]]*}"}"  # trim left
  text="${text%"${text##*[![:space:]]}"}"  # trim right
  echo "$text"
}

# Affiche le contenu d'un fichier avec numéros de ligne (debug)
# Usage : debug_file "/path/to/file" [max_lines]
debug_file() {
  local file="${1:?fichier requis}"
  local max_lines="${2:-50}"
  
  echo "=== Contenu de $file (max $max_lines lignes) ===" >&2
  head -n "$max_lines" "$file" | nl >&2
  
  local total_lines
  total_lines=$(count_lines "$file")
  if [ "$total_lines" -gt "$max_lines" ]; then
    echo "... ($((total_lines - max_lines)) lignes supplémentaires)" >&2
  fi
  echo "===" >&2
}

# ── Setup helpers communs ─────────────────────────────────────────────────────

# Setup standard pour la plupart des tests
# Usage : common_setup (à appeler dans setup())
common_setup() {
  TEST_DIR="${TEST_DIR:-$(mktemp -d)}"
  
  # Variables d'environnement standard
  export HUB_ROOT="${HUB_ROOT:-$BATS_TEST_DIRNAME/..}"
  export HUB_CONFIG="${HUB_CONFIG:-$TEST_DIR/hub.json}"
  export PROJECTS_FILE="${PROJECTS_FILE:-$TEST_DIR/projects.md}"
  export API_KEYS_FILE="${API_KEYS_FILE:-$TEST_DIR/api-keys.local.md}"
  
  # Créer les fichiers vides par défaut
  mkdir -p "$(dirname "$HUB_CONFIG")"
  mkdir -p "$(dirname "$PROJECTS_FILE")"
  mkdir -p "$(dirname "$API_KEYS_FILE")"
  
  touch "$PROJECTS_FILE"
  touch "$API_KEYS_FILE"
  
  # Mock get_hub_version par défaut
  get_hub_version() {
    echo "test"
  }
  export -f get_hub_version
}

# Teardown standard
# Usage : common_teardown (à appeler dans teardown())
common_teardown() {
  [ -n "$TEST_DIR" ] && rm -rf "$TEST_DIR"
}
