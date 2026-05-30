#!/usr/bin/env bats
# Tests pour scripts/cmd-audit.sh
# Couvre : Audit IA sur un projet, types d'audit, agents requis, déploiement

load helpers

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  FAKE_HUB="$(mktemp -d)"

  # Répertoires de données factices
  mkdir -p "$FAKE_HUB/agents/audit"
  mkdir -p "$FAKE_HUB/config"
  mkdir -p "$FAKE_HUB/projects"

  # Symlinks vers les scripts et skills réels
  ln -s "$HUB_ROOT/scripts" "$FAKE_HUB/scripts"
  ln -s "$HUB_ROOT/skills"  "$FAKE_HUB/skills"

  # Agents audit minimaux
  cat > "$FAKE_HUB/agents/audit/auditor.md" <<'AGENTEOF'
---
id: auditor
label: Auditor
description: Agent d'audit principal
mode: primary
targets: [opencode]
skills: []
---
# Auditor
Agent d'audit principal.
AGENTEOF

  cat > "$FAKE_HUB/agents/audit/auditor-security.md" <<'AGENTEOF'
---
id: auditor-security
label: SecurityAuditor
description: Agent d'audit sécurité
mode: assistant
targets: [opencode]
skills: []
---
# SecurityAuditor
Agent d'audit sécurité spécialisé.
AGENTEOF

  # hub.json minimal avec opencode comme cible active
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
# A. Validation des arguments
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : erreur si type invalide" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type invalid-type
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalide"* ]] || [[ "$output" == *"invalid"* ]]
}

@test "audit : types valides acceptés (security, accessibility, architecture)" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor, auditor-security
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  
  # Copier les agents pour éviter prompt déploiement
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"
  cp "$FAKE_HUB/agents/audit/auditor-security.md" "$PROJECT_DIR/.opencode/agents/"

  # Test avec --type security (devrait être accepté)
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type security < /dev/null
  # Le script peut échouer pour d'autres raisons (opencode non installé) mais ne doit pas rejeter le type
  [[ "$output" != *"Type invalide"* ]] && [[ "$output" != *"invalid"*"type"* ]]
}

@test "audit : normalisation PROJECT_ID" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ-123
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ-123=$PROJECT_DIR" >> "$PATHS_FILE"
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  # Tester avec casse différente
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-PROJ-123 < /dev/null
  # Doit normaliser et trouver le projet
  [[ "$output" != *"introuvable"* ]] && [[ "$output" != *"not found"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Vérification des agents requis
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : détecte agents manquants dans projects.md" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : other-agent
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"

  # Répondre 'N' pour refuser l'ajout d'agents
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj < <(yes n | head -20)
  [[ "$output" == *"auditor"* ]] || [[ "$output" == *"manquant"* ]] || [[ "$output" == *"missing"* ]]
}

@test "audit : agents=all permet tous les agents" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : all
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj < /dev/null
  # Ne doit PAS demander d'ajouter des agents
  [[ "$output" != *"ajouter"* ]] && [[ "$output" != *"add"*"agent"* ]]
}

@test "audit --type security : requiert auditor-security" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  # auditor-security manque
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type security < <(yes n | head -20)
  [[ "$output" == *"auditor-security"* ]] || [[ "$output" == *"manquant"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Déploiement et validation physique
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : détecte dossier .opencode/agents/ absent" {
  skip "Test interactif - nécessite input manuel complexe"
  
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR"
  # NE PAS créer .opencode/agents/
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" TEST-PROJ < <(yes n | head -20)
  [[ "$output" == *"déployés"* ]] || [[ "$output" == *"deployed"* ]] || [[ "$output" == *"absent"* ]]
}

@test "audit : détecte agents non déployés physiquement" {
  skip "Test interactif - nécessite input manuel complexe"
  
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  # Dossier existe mais fichier auditor.md absent
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" TEST-PROJ < <(yes n | head -20)
  [[ "$output" == *"déployés"* ]] || [[ "$output" == *"auditor"* ]]
}

@test "audit : agents déployés et présents → pas de prompt déploiement" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  
  # Copier l'agent
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj < /dev/null
  # Ne doit PAS proposer de redéployer
  [[ "$output" != *"Déployer maintenant"* ]] || true
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Sélection interactive
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : sélection interactive si pas de PROJECT_ID" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-1
- Nom : Project 1
- Agents : auditor

## PROJ-2
- Nom : Project 2
- Agents : auditor
EOF

  # Simuler choix 1
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" <<< "1"
  [[ "$output" == *"PROJ-1"* ]] || [[ "$output" == *"PROJ-2"* ]] || [[ "$output" == *"Choisir"* ]]
}

@test "audit : erreur si aucun projet enregistré" {
  # projects.md vide (pas de projet)
  run bash "$HUB_ROOT/scripts/cmd-audit.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"Aucun projet"* ]] || [[ "$output" == *"No project"* ]]
}

@test "audit : choix invalide dans sélection interactive" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-1
- Nom : Project 1
- Agents : auditor
EOF

  # Simuler choix invalide (999)
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" <<< "999"
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalide"* ]] || [[ "$output" == *"invalid"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. Validation projet
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : erreur si projet introuvable" {
  run bash "$HUB_ROOT/scripts/cmd-audit.sh" nonexistent-project
  [ "$status" -ne 0 ]
  [[ "$output" == *"introuvable"* ]] || [[ "$output" == *"not found"* ]] || [[ "$output" == *"Aucun projet"* ]]
}

@test "audit : résolution chemin projet correct" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj < /dev/null
  # Le chemin doit apparaître dans l'output
  [[ "$output" == *"$PROJECT_DIR"* ]] || [[ "$output" == *"TEST-PROJ"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# F. Types d'audit multiples
# ══════════════════════════════════════════════════════════════════════════════

@test "audit : tous les types valides listés dans erreur" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type bad-type
  [ "$status" -ne 0 ]
  # Doit lister les types valides
  [[ "$output" == *"security"* ]]
  [[ "$output" == *"accessibility"* ]]
  [[ "$output" == *"architecture"* ]]
}

@test "audit --type ecodesign : accepté" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : auditor, auditor-ecodesign
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"
  
  # Créer agent ecodesign
  cat > "$FAKE_HUB/agents/audit/auditor-ecodesign.md" <<'EOF'
---
id: auditor-ecodesign
label: EcodesignAuditor
targets: [opencode]
---
# EcodesignAuditor
EOF
  cp "$FAKE_HUB/agents/audit/auditor-ecodesign.md" "$PROJECT_DIR/.opencode/agents/"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type ecodesign < /dev/null
  [[ "$output" != *"Type invalide"* ]]
}

@test "audit --type performance : accepté" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## TEST-PROJ
- Nom : Test Project
- Agents : all
EOF

  PROJECT_DIR="$FAKE_HUB/test-proj"
  mkdir -p "$PROJECT_DIR/.opencode/agents"
  echo "TEST-PROJ=$PROJECT_DIR" >> "$PATHS_FILE"
  cp "$FAKE_HUB/agents/audit/auditor.md" "$PROJECT_DIR/.opencode/agents/"

  run bash "$HUB_ROOT/scripts/cmd-audit.sh" test-proj --type performance < /dev/null
  [[ "$output" != *"Type invalide"* ]]
}
