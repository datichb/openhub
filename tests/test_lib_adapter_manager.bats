#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/adapter-manager.sh
# Fonctions testées : load_adapter

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  export ADAPTERS_DIR="$TEST_DIR/adapters"
  source "$SCRIPT_DIR/common.sh"
  
  # Sourcer le module
  source "$BATS_TEST_DIRNAME/../scripts/lib/adapter-manager.sh"
  
  # Créer dossier adapters
  mkdir -p "$ADAPTERS_DIR"
}

teardown() {
  common_teardown
}

# ── load_adapter ────────────────────────────────────────────────────────────

@test "load_adapter : charge adaptateur valide" {
  # Créer un adaptateur de test
  cat > "$ADAPTERS_DIR/test.adapter.sh" <<'EOF'
adapter_validate() { return 0; }
adapter_needs_node() { return 0; }
adapter_deploy_files() { return 0; }
adapter_deploy_config() { return 0; }
adapter_deploy() { return 0; }
adapter_install() { return 0; }
adapter_update() { return 0; }
adapter_start() { return 0; }
EOF
  
  # Appeler sans run pour que les fonctions restent visibles dans le shell courant
  load_adapter "test"
  
  # Vérifier que les fonctions sont définies
  run declare -F adapter_validate
  [ "$status" -eq 0 ]
}

@test "load_adapter : échoue si adaptateur absent" {
  run load_adapter "nonexistent"
  [ "$status" -ne 0 ]
}

@test "load_adapter : échoue si fonction manquante" {
  # Adaptateur incomplet
  cat > "$ADAPTERS_DIR/incomplete.adapter.sh" <<'EOF'
adapter_validate() { return 0; }
adapter_needs_node() { return 0; }
# Fonctions manquantes...
EOF
  
  run load_adapter "incomplete"
  [ "$status" -ne 0 ]
}

@test "load_adapter : exporte toutes les fonctions requises" {
  cat > "$ADAPTERS_DIR/complete.adapter.sh" <<'EOF'
adapter_validate() { echo "validate"; }
adapter_needs_node() { echo "needs_node"; }
adapter_deploy_files() { echo "deploy_files"; }
adapter_deploy_config() { echo "deploy_config"; }
adapter_deploy() { echo "deploy"; }
adapter_install() { echo "install"; }
adapter_update() { echo "update"; }
adapter_start() { echo "start"; }
EOF
  
  load_adapter "complete"
  
  run adapter_validate
  [ "$output" = "validate" ]
  
  run adapter_deploy
  [ "$output" = "deploy" ]
}

# ── Intégration ─────────────────────────────────────────────────────────────

@test "Intégration : load multiple adapters" {
  # Créer 2 adaptateurs
  for name in adapter1 adapter2; do
    cat > "$ADAPTERS_DIR/$name.adapter.sh" <<EOF
adapter_validate() { echo "$name"; }
adapter_needs_node() { return 0; }
adapter_deploy_files() { return 0; }
adapter_deploy_config() { return 0; }
adapter_deploy() { return 0; }
adapter_install() { return 0; }
adapter_update() { return 0; }
adapter_start() { return 0; }
EOF
  done
  
  load_adapter "adapter1"
  run adapter_validate
  [ "$output" = "adapter1" ]
  
  load_adapter "adapter2"
  run adapter_validate
  [ "$output" = "adapter2" ]
}
