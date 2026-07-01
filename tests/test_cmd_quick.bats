#!/usr/bin/env bats
# Tests pour scripts/cmd-quick.sh
# Vérifie : validation args, détection agent, lancement adapter_start

setup() {
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"

    > "$HUB_CONFIG"

  CMD_QUICK="$BATS_TEST_DIRNAME/../scripts/cmd-quick.sh"

  mkdir -p "$TEST_DIR/proj-app"
  mkdir -p "$TEST_DIR/proj-app/.opencode/agents"
  touch "$TEST_DIR/proj-app/.opencode/agents/developer-frontend.md"
  touch "$TEST_DIR/proj-app/.opencode/agents/developer-backend.md"
  touch "$TEST_DIR/proj-app/.opencode/agents/developer-fullstack.md"
  touch "$TEST_DIR/proj-app/.opencode/agents/developer-devops.md"

  cat > "$PROJECTS_FILE" <<'EOF'
# Registre de test

## MY-APP
- Nom : My App
- Stack : Node.js
- Board Beads : MY-APP
- Tracker : none
- Labels : feature
- Agents : all
EOF

  cat > "$PATHS_FILE" <<EOF
MY-APP=$TEST_DIR/proj-app
EOF

  : > "$API_KEYS_FILE"

  # Mock opencode
  OPENCODE_LOG="$TEST_DIR/opencode_calls.log"
  export OPENCODE_LOG
  : > "$OPENCODE_LOG"
  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode $*" >> "$OPENCODE_LOG"
exit 0
OCEOF
  chmod +x "$TEST_DIR/bin/opencode"
  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ── Validation des arguments ──────────────────────────────────────────────────

@test "quick : sans argument → erreur et code non-zéro" {
  run bash "$CMD_QUICK"
  [ "$status" -ne 0 ]
}

@test "quick : sans prompt → erreur et code non-zéro" {
  run bash "$CMD_QUICK" -p "MY-APP"
  [ "$status" -ne 0 ]
}

@test "quick : --help → affiche l'aide et code 0" {
  run bash "$CMD_QUICK" --help
  [ "$status" -eq 0 ]
  [[ "$output" =~ "quick" ]] || [[ "$output" =~ "Quick" ]]
}

@test "quick : -h → alias --help fonctionne" {
  run bash "$CMD_QUICK" -h
  [ "$status" -eq 0 ]
}

@test "quick : projet inexistant → erreur" {
  run bash "$CMD_QUICK" -p "INEXISTANT" "Ajoute un bouton"
  [ "$status" -ne 0 ]
}

# ── Détection d'agent ─────────────────────────────────────────────────────────

@test "quick : mot-clé frontend → détecte developer-frontend" {
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  eval "$(sed -n '/^detect_agent_from_prompt()/,/^}/p' "$CMD_QUICK")"
  run detect_agent_from_prompt "Ajoute un composant React"
  [ "$output" = "developer-frontend" ]
}

@test "quick : mot-clé api → détecte developer-backend" {
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  eval "$(sed -n '/^detect_agent_from_prompt()/,/^}/p' "$CMD_QUICK")"
  run detect_agent_from_prompt "Crée un endpoint /api/users"
  [ "$output" = "developer-backend" ]
}

@test "quick : mot-clé docker → détecte developer-devops" {
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  eval "$(sed -n '/^detect_agent_from_prompt()/,/^}/p' "$CMD_QUICK")"
  run detect_agent_from_prompt "Configure un pipeline CI avec docker"
  [ "$output" = "developer-devops" ]
}

@test "quick : mot-clé neutre → détecte developer-fullstack par défaut" {
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  eval "$(sed -n '/^detect_agent_from_prompt()/,/^}/p' "$CMD_QUICK")"
  run detect_agent_from_prompt "Refactorise le code du projet"
  [ "$output" = "developer-fullstack" ]
}

@test "quick : mot-clé database → détecte developer-backend" {
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  eval "$(sed -n '/^detect_agent_from_prompt()/,/^}/p' "$CMD_QUICK")"
  run detect_agent_from_prompt "Crée une migration de base de données"
  [ "$output" = "developer-backend" ]
}

# ── Lancement ─────────────────────────────────────────────────────────────────

@test "quick : projet valide + prompt → lance opencode" {
  run bash "$CMD_QUICK" -p "MY-APP" "Ajoute un bouton de connexion"
  [ "$status" -eq 0 ]
  [ -f "$OPENCODE_LOG" ]
  grep -q "opencode" "$OPENCODE_LOG"
}
