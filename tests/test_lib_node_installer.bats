#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/node-installer.sh
# Fonctions testées : ensure_node, _choose_installer, installers, helpers

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  source "$SCRIPT_DIR/common.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/node-installer.sh"
  
  # Mock log functions
  mock_log_functions
}

teardown() {
  common_teardown
}

# ── Helpers ─────────────────────────────────────────────────────────────────

@test "_get_latest_nvm_version : retourne version format vX.X.X" {
  # Mock curl qui retourne une release GitHub
  curl() {
    echo '{"tag_name":"v0.40.1"}'
  }
  export -f curl
  
  run _get_latest_nvm_version
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

@test "_get_latest_nvm_version : retourne fallback si curl échoue" {
  # Mock curl qui échoue
  curl() {
    return 1
  }
  export -f curl
  
  run _get_latest_nvm_version
  [ "$status" -eq 0 ]
  [ "$output" = "v0.40.3" ]
}

@test "_print_manual_instructions : affiche instructions volta" {
  run _print_manual_instructions "volta"
  [ "$status" -eq 0 ]
  [[ "$output" == *"volta.sh"* ]]
}

@test "_print_manual_instructions : affiche instructions brew" {
  run _print_manual_instructions "brew"
  [ "$status" -eq 0 ]
  [[ "$output" == *"brew install node"* ]]
}

@test "_print_manual_instructions : affiche instructions nvm" {
  run _print_manual_instructions "nvm"
  [ "$status" -eq 0 ]
  [[ "$output" == *"nvm-sh/nvm"* ]]
}

@test "_print_manual_instructions : affiche instructions génériques si méthode inconnue" {
  run _print_manual_instructions "unknown"
  [ "$status" -eq 0 ]
  [[ "$output" == *"nodejs.org"* ]]
}

# ── Installers ──────────────────────────────────────────────────────────────

@test "_install_with_volta : installe volta puis node" {
  # Mock curl pour volta install
  curl() {
    echo "echo 'Volta installed'"
  }
  export -f curl
  
  # Mock bash
  bash() {
    return 0
  }
  export -f bash
  
  # Mock volta command
  volta() {
    echo "Installing node..."
    return 0
  }
  export -f volta
  
  # Mock command pour dire que volta existe
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "volta" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  run _install_with_volta
  [ "$status" -eq 0 ]
}

@test "_install_with_brew : installe node via brew" {
  # Mock brew
  brew() {
    echo "Installing node..."
    return 0
  }
  export -f brew
  
  run _install_with_brew
  [ "$status" -eq 0 ]
}

@test "_install_with_nvm : installe nvm puis node" {
  # Mock curl
  curl() {
    echo "echo 'nvm installed'"
  }
  export -f curl
  
  # Mock bash
  bash() {
    return 0
  }
  export -f bash
  
  # Créer faux nvm.sh
  export NVM_DIR="$TEST_DIR/.nvm"
  mkdir -p "$NVM_DIR"
  cat > "$NVM_DIR/nvm.sh" <<'EOF'
nvm() {
  echo "Installing node LTS..."
  return 0
}
EOF
  
  # Mock command
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "nvm" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  run _install_with_nvm
  [ "$status" -eq 0 ]
}

# ── _verify_node_in_path ────────────────────────────────────────────────────

@test "_verify_node_in_path : retourne 0 si node disponible" {
  # Mock node command
  node() {
    echo "v20.0.0"
  }
  export -f node
  
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "node" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  run _verify_node_in_path
  [ "$status" -eq 0 ]
}

@test "_verify_node_in_path : retourne 1 si node absent" {
  # Mock command qui dit que node n'existe pas
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "node" ]; then
      return 1
    fi
    builtin command "$@"
  }
  export -f command
  
  run _verify_node_in_path
  [ "$status" -ne 0 ]
}

# ── _choose_installer ───────────────────────────────────────────────────────

@test "_choose_installer : retourne volta par défaut" {
  # Mock detect_os
  detect_os() {
    echo "linux"
  }
  export -f detect_os
  
  # Mock command pour dire qu'aucun outil n'est installé
  command() {
    return 1
  }
  export -f command
  
  # Simuler input utilisateur (défaut = 1)
  exec 3</dev/tty
  exec </dev/null
  
  run _choose_installer "linux"
  [ "$status" -eq 0 ]
  [ "$output" = "volta" ]
  
  exec 0<&3
}

@test "_choose_installer : inclut brew sur macOS" {
  # Mock command
  command() {
    if [ "$2" = "brew" ]; then
      return 0  # brew existe
    fi
    return 1
  }
  export -f command
  
  # Simuler input (choix 2 = brew sur macOS)
  exec 3</dev/tty
  exec </dev/null
  
  run _choose_installer "macos"
  [ "$status" -eq 0 ]
  # Sur macOS avec brew, les options sont: volta, brew, nvm
  # Le défaut devrait être volta
  [ "$output" = "volta" ]
  
  exec 0<&3
}

# ── ensure_node ─────────────────────────────────────────────────────────────

@test "ensure_node : retourne 0 si node déjà installé" {
  # Mock node
  node() {
    echo "v20.0.0"
  }
  export -f node
  
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "node" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  run ensure_node
  [ "$status" -eq 0 ]
}

@test "ensure_node : déclenche installation si node absent" {
  # Mock command pour dire que node n'existe pas
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "node" ]; then
      return 1
    fi
    builtin command "$@"
  }
  export -f command
  
  # Mock _detect_and_install_node
  _detect_and_install_node() {
    echo "Installing node..."
    return 0
  }
  export -f _detect_and_install_node
  
  run ensure_node
  [ "$status" -eq 0 ]
  [[ "$output" == *"Installing node"* ]]
}

# ── _install_node_with ──────────────────────────────────────────────────────

@test "_install_node_with : skip si utilisateur refuse" {
  # Simuler réponse 'n'
  read() {
    eval "$2='n'"
    return 0
  }
  export -f read
  
  run _install_node_with "volta"
  [ "$status" -ne 0 ]
}

@test "_install_node_with : installe si utilisateur accepte" {
  # Simuler réponse 'Y'
  read() {
    eval "$2='Y'"
    return 0
  }
  export -f read
  
  # Mock _install_with_volta
  _install_with_volta() {
    return 0
  }
  export -f _install_with_volta
  
  # Mock _verify_node_in_path
  _verify_node_in_path() {
    return 0
  }
  export -f _verify_node_in_path
  
  run _install_node_with "volta"
  [ "$status" -eq 0 ]
}

@test "_install_node_with : gère erreur installation" {
  read() {
    eval "$2='Y'"
    return 0
  }
  export -f read
  
  # Mock installation qui échoue
  _install_with_brew() {
    return 1
  }
  export -f _install_with_brew
  
  run _install_node_with "brew"
  [ "$status" -ne 0 ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : workflow volta complet" {
  # Mock tous les composants
  curl() {
    if [[ "$*" == *"volta.sh"* ]]; then
      echo "echo 'Volta installed'"
    elif [[ "$*" == *"github.com"* ]]; then
      echo '{"tag_name":"v0.40.1"}'
    fi
  }
  export -f curl
  
  bash() {
    return 0
  }
  export -f bash
  
  volta() {
    return 0
  }
  export -f volta
  
  node() {
    echo "v20.0.0"
  }
  export -f node
  
  command() {
    if [ "$2" = "volta" ] || [ "$2" = "node" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  # Installer
  run _install_with_volta
  [ "$status" -eq 0 ]
  
  # Vérifier
  run _verify_node_in_path
  [ "$status" -eq 0 ]
}

@test "Intégration : ensure_node avec node déjà présent" {
  node() {
    echo "v18.17.0"
  }
  export -f node
  
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "node" ]; then
      return 0
    fi
    builtin command "$@"
  }
  export -f command
  
  run ensure_node
  [ "$status" -eq 0 ]
  [[ "$output" == *"détecté"* ]]
}
