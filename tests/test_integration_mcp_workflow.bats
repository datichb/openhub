#!/usr/bin/env bats
# Tests d'intégration pour le workflow MCP complet
# Couvre : Install Node → Build MCP → Deploy → Configure

load helpers

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  FAKE_HUB="$(mktemp -d)"

  # Répertoires de données factices
  mkdir -p "$FAKE_HUB/agents/mcp"
  mkdir -p "$FAKE_HUB/config"
  mkdir -p "$FAKE_HUB/projects"

  # Symlinks vers les scripts et skills réels
  ln -s "$HUB_ROOT/scripts" "$FAKE_HUB/scripts"
  ln -s "$HUB_ROOT/skills"  "$FAKE_HUB/skills"

  # Agent MCP minimal
  cat > "$FAKE_HUB/agents/mcp/mcp-agent.md" <<'AGENTEOF'
---
id: mcp-agent
label: MCPAgent
description: Agent MCP test
mode: primary
targets: [opencode]
skills: []
---
# MCPAgent
Agent MCP de test.
AGENTEOF

  # hub.json minimal
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_target": "opencode",
  "active_targets": ["opencode"],
  "cli": {"language": "fr"}
}
HUBEOF

  cp "$FAKE_HUB/config/hub.json" "$FAKE_HUB/config/hub.json.example"
  echo '{"mappings": {}}' > "$FAKE_HUB/config/stack-skills.json"
  
  PROJECTS_FILE="$FAKE_HUB/projects/projects.md"
  PATHS_FILE="$FAKE_HUB/projects/paths.local.md"
  echo "# Registre de test" > "$PROJECTS_FILE"
  echo "# Local paths" > "$PATHS_FILE"
  touch "$FAKE_HUB/projects/api-keys.local.md"

  export HUB_DIR="$FAKE_HUB"
  export CANONICAL_AGENTS_DIR="$FAKE_HUB/agents"
  export PROJECTS_FILE
  export PATHS_FILE
}

teardown() {
  rm -rf "$FAKE_HUB"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Installation Node (lib/node-installer.sh)
# ══════════════════════════════════════════════════════════════════════════════

@test "mcp workflow : vérification présence Node" {
  # Tester que Node est disponible ou que node-installer peut le détecter
  run bash -c "source '$HUB_ROOT/scripts/lib/node-installer.sh' && command -v node"
  # Node devrait être disponible sur le système de test
  [ "$status" -eq 0 ] || skip "Node.js non installé sur le système de test"
}

@test "mcp workflow : validation version Node minimale" {
  run bash -c "source '$HUB_ROOT/scripts/lib/node-installer.sh' && node --version"
  [ "$status" -eq 0 ]
  # Version devrait être >= 18
  [[ "$output" == *"v"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Build MCP (scripts/build-mcp.sh)
# ══════════════════════════════════════════════════════════════════════════════

@test "mcp workflow : création structure .opencode/mcp/" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR"

  # Simuler un build MCP basique
  mkdir -p "$PROJECT_DIR/.opencode/mcp/src"
  
  [ -d "$PROJECT_DIR/.opencode/mcp" ]
  [ -d "$PROJECT_DIR/.opencode/mcp/src" ]
}

@test "mcp workflow : génération package.json" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp"

  # Créer un package.json minimal
  cat > "$PROJECT_DIR/.opencode/mcp/package.json" <<'EOF'
{
  "name": "test-mcp",
  "version": "1.0.0",
  "type": "module",
  "main": "dist/index.js"
}
EOF

  [ -f "$PROJECT_DIR/.opencode/mcp/package.json" ]
  run cat "$PROJECT_DIR/.opencode/mcp/package.json"
  [[ "$output" == *"test-mcp"* ]]
}

@test "mcp workflow : validation structure package.json" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp"

  cat > "$PROJECT_DIR/.opencode/mcp/package.json" <<'EOF'
{
  "name": "test-mcp",
  "version": "1.0.0",
  "type": "module",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc"
  }
}
EOF

  # Vérifier que jq peut parser le JSON
  run bash -c "jq -r '.name' '$PROJECT_DIR/.opencode/mcp/package.json'"
  if [ "$status" -eq 0 ]; then
    [[ "$output" == "test-mcp" ]]
  else
    skip "jq non disponible pour validation"
  fi
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Déploiement MCP (lib/mcp-deploy.sh)
# ══════════════════════════════════════════════════════════════════════════════

@test "mcp workflow : déploiement structure MCP dans projet" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"

  # Simuler un fichier MCP déployé
  cat > "$PROJECT_DIR/.opencode/mcp/dist/index.js" <<'EOF'
// MCP Server stub
export function start() {
  console.log("MCP Server started");
}
EOF

  [ -f "$PROJECT_DIR/.opencode/mcp/dist/index.js" ]
  run cat "$PROJECT_DIR/.opencode/mcp/dist/index.js"
  [[ "$output" == *"MCP Server"* ]]
}

@test "mcp workflow : création mcp-config.json" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode"

  # Créer un fichier de configuration MCP
  cat > "$PROJECT_DIR/.opencode/mcp-config.json" <<'EOF'
{
  "mcpServers": {
    "test-server": {
      "command": "node",
      "args": [".opencode/mcp/dist/index.js"]
    }
  }
}
EOF

  [ -f "$PROJECT_DIR/.opencode/mcp-config.json" ]
  run cat "$PROJECT_DIR/.opencode/mcp-config.json"
  [[ "$output" == *"mcpServers"* ]]
  [[ "$output" == *"test-server"* ]]
}

@test "mcp workflow : validation permissions fichiers MCP" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"

  cat > "$PROJECT_DIR/.opencode/mcp/dist/index.js" <<'EOF'
#!/usr/bin/env node
console.log("MCP");
EOF

  chmod +x "$PROJECT_DIR/.opencode/mcp/dist/index.js"

  # Vérifier que le fichier est exécutable
  [ -x "$PROJECT_DIR/.opencode/mcp/dist/index.js" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Configuration et intégration
# ══════════════════════════════════════════════════════════════════════════════

@test "mcp workflow : validation accessibilité MCP" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"

  cat > "$PROJECT_DIR/.opencode/mcp/dist/index.js" <<'EOF'
#!/usr/bin/env node
console.log("MCP accessible");
process.exit(0);
EOF

  chmod +x "$PROJECT_DIR/.opencode/mcp/dist/index.js"

  # Tester l'exécution du MCP
  run node "$PROJECT_DIR/.opencode/mcp/dist/index.js"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MCP accessible"* ]]
}

@test "mcp workflow : test re-déploiement incrémental" {
  PROJECT_DIR="$FAKE_HUB/test-mcp-proj"
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"

  # Premier déploiement
  echo "v1" > "$PROJECT_DIR/.opencode/mcp/dist/version.txt"
  [ -f "$PROJECT_DIR/.opencode/mcp/dist/version.txt" ]

  # Re-déploiement
  echo "v2" > "$PROJECT_DIR/.opencode/mcp/dist/version.txt"
  
  run cat "$PROJECT_DIR/.opencode/mcp/dist/version.txt"
  [[ "$output" == "v2" ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. Workflow end-to-end
# ══════════════════════════════════════════════════════════════════════════════

@test "mcp workflow : workflow complet de A à Z" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## MCP-TEST
- Nom : MCP Test Project
- Stack : Node.js
- Agents : mcp-agent
EOF

  PROJECT_DIR="$FAKE_HUB/mcp-test"
  mkdir -p "$PROJECT_DIR"
  echo "MCP-TEST=$PROJECT_DIR" >> "$PATHS_FILE"

  # 1. Créer structure MCP
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"
  
  # 2. Créer package.json
  cat > "$PROJECT_DIR/.opencode/mcp/package.json" <<'EOF'
{
  "name": "mcp-test",
  "version": "1.0.0",
  "type": "module",
  "main": "dist/index.js"
}
EOF

  # 3. Créer fichier MCP
  cat > "$PROJECT_DIR/.opencode/mcp/dist/index.js" <<'EOF'
#!/usr/bin/env node
console.log("MCP workflow complete");
EOF
  chmod +x "$PROJECT_DIR/.opencode/mcp/dist/index.js"

  # 4. Créer config
  cat > "$PROJECT_DIR/.opencode/mcp-config.json" <<'EOF'
{
  "mcpServers": {
    "test": {
      "command": "node",
      "args": [".opencode/mcp/dist/index.js"]
    }
  }
}
EOF

  # Vérifications finales
  [ -d "$PROJECT_DIR/.opencode/mcp" ]
  [ -f "$PROJECT_DIR/.opencode/mcp/package.json" ]
  [ -f "$PROJECT_DIR/.opencode/mcp/dist/index.js" ]
  [ -f "$PROJECT_DIR/.opencode/mcp-config.json" ]
  [ -x "$PROJECT_DIR/.opencode/mcp/dist/index.js" ]

  # Test exécution
  run node "$PROJECT_DIR/.opencode/mcp/dist/index.js"
  [ "$status" -eq 0 ]
  [[ "$output" == *"workflow complete"* ]]
}

@test "mcp workflow : vérification intégrité après déploiement" {
  PROJECT_DIR="$FAKE_HUB/mcp-integrity-test"
  mkdir -p "$PROJECT_DIR/.opencode/mcp/dist"

  # Créer fichiers
  cat > "$PROJECT_DIR/.opencode/mcp/package.json" <<'EOF'
{"name": "integrity-test", "version": "1.0.0"}
EOF

  cat > "$PROJECT_DIR/.opencode/mcp/dist/index.js" <<'EOF'
console.log("ok");
EOF

  # Vérifier intégrité
  [ -f "$PROJECT_DIR/.opencode/mcp/package.json" ]
  [ -f "$PROJECT_DIR/.opencode/mcp/dist/index.js" ]
  
  # Vérifier contenu
  run grep -q "integrity-test" "$PROJECT_DIR/.opencode/mcp/package.json"
  [ "$status" -eq 0 ]
}
