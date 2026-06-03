#!/usr/bin/env bats
# Tests pour scripts/cmd-deploy.sh
# Couvre : --check (à jour / obsolète / manquant), --diff, mode normal
#
# Stratégie d'isolation :
#   - FAKE_HUB = dossier temporaire avec :
#     - agents/quality/test-agent.md  (agent minimal de test)
#     - config/hub.json               (hub factice avec opencode actif)
#     - projects/                     (fichiers projets vides)
#     - scripts/ → symlink vers le vrai scripts/ du repo
#     - skills/  → symlink vers le vrai skills/ du repo
#   - HUB_DIR=FAKE_HUB est exporté pour cmd-deploy.sh
#   - CANONICAL_AGENTS_DIR=FAKE_HUB/agents est exporté pour isoler le scan aux agents du FAKE_HUB

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  FAKE_HUB="$(mktemp -d)"

  # Répertoires de données factices
  mkdir -p "$FAKE_HUB/agents/quality"
  mkdir -p "$FAKE_HUB/config"
  mkdir -p "$FAKE_HUB/projects"
  mkdir -p "$FAKE_HUB/.opencode/agents"

  # Symlinks vers les scripts et skills réels
  ln -s "$HUB_ROOT/scripts" "$FAKE_HUB/scripts"
  ln -s "$HUB_ROOT/skills"  "$FAKE_HUB/skills"

  # Agent minimal de test supportant opencode
  cat > "$FAKE_HUB/agents/quality/test-agent.md" <<'AGENTEOF'
---
id: test-agent
label: TestAgent
description: Agent de test minimal
mode: primary
targets: [opencode]
skills: []
---

# TestAgent

Contenu de l'agent de test.
AGENTEOF

  # hub.json minimal
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_provider": {"name": "", "api_key": "", "base_url": "", "model": ""},
  "opencode": {"model": "claude-sonnet-4-5", "disabled_native_agents": []},
  "cli": {"language": "fr"}
}
HUBEOF
  cp "$FAKE_HUB/config/hub.json" "$FAKE_HUB/config/hub.json.example"
  echo '{"mappings": {}}' > "$FAKE_HUB/config/stack-skills.json"
  echo "# Registre de test" > "$FAKE_HUB/projects/projects.md"
  touch "$FAKE_HUB/projects/paths.local.md"
  touch "$FAKE_HUB/projects/api-keys.local.md"

  export HUB_DIR="$FAKE_HUB"
  export CANONICAL_AGENTS_DIR="$FAKE_HUB/agents"
}

teardown() {
  rm -rf "$FAKE_HUB"
}

# ── _get_mtime ────────────────────────────────────────────────────────────────
# _get_mtime a été supprimée dans le commit 6c29c94. Le test d'ordre de timestamps
# reste valide en s'appuyant directement sur stat.

@test "_get_mtime : un fichier plus récent a un timestamp plus grand ou égal" {
  f1=$(mktemp)
  sleep 0.1
  f2=$(mktemp)
  t1=$(stat -c %Y "$f1" 2>/dev/null || stat -f %m "$f1" 2>/dev/null)
  t2=$(stat -c %Y "$f2" 2>/dev/null || stat -f %m "$f2" 2>/dev/null)
  rm -f "$f1" "$f2"
  [ "$t2" -ge "$t1" ]
}

# ── --check : agent manquant ──────────────────────────────────────────────────

@test "deploy --check : agent manquant retourne exit 1" {
  # Pas de fichier déployé dans .opencode/agents/
  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche MANQUANT si agent absent" {
  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check 2>&1 || true
  [[ "$output" == *"MANQUANT"* ]]
}

# ── --check : agent obsolète ──────────────────────────────────────────────────

@test "deploy --check : agent obsolète retourne exit 1" {
  # Copier l'agent d'abord, puis le rendre plus vieux que la source
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  sleep 0.1
  # Toucher la source pour la rendre plus récente
  touch "$FAKE_HUB/agents/quality/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche OBSOLÈTE si agent source plus récent" {
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  sleep 0.1
  touch "$FAKE_HUB/agents/quality/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check 2>&1 || true
  [[ "$output" == *"OBSOLÈTE"* ]]
}

# ── --check : agent à jour ────────────────────────────────────────────────────

@test "deploy --check : agent à jour retourne exit 0" {
  # Copier l'agent APRÈS avoir attendu → le déployé est plus récent que la source
  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 0 ]
}

@test "deploy --check : affiche À JOUR si agent récent" {
  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 0 ]
  [[ "$output" == *"À JOUR"* ]]
}

# ── --check : aucun agent à vérifier ─────────────────────────────────────────

@test "deploy --check : aucun agent à vérifier retourne exit 0 sans crash" {
  # Vider CANONICAL_AGENTS_DIR pour simuler un hub sans agents
  rm -rf "$FAKE_HUB/agents/quality"
  mkdir -p "$FAKE_HUB/agents"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 0 ]
}

# ── --diff : agent nouveau ────────────────────────────────────────────────────

@test "deploy --diff : affiche 'nouveau' si agent non déployé" {
  run bash -c "echo n | bash '$HUB_ROOT/scripts/cmd-deploy.sh' --diff"
  [[ "$output" == *"nouveau"* ]]
}

# ── --diff : agent inchangé ───────────────────────────────────────────────────

@test "deploy --diff : affiche 'inchangé' si agent déjà identique" {
  # Pré-déployer l'agent via build_agent_content direct (bypass adapter_validate —
  # opencode n'est pas nécessairement installé en CI)
  bash -c "
    export HUB_DIR='$FAKE_HUB'
    export CANONICAL_AGENTS_DIR='$FAKE_HUB/agents'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    mkdir -p '$FAKE_HUB/.opencode/agents'
    build_agent_content '$FAKE_HUB/agents/quality/test-agent.md' 'fr' '$FAKE_HUB' \
      > '$FAKE_HUB/.opencode/agents/test-agent.md'
  " 2>/dev/null

  run bash -c "echo n | bash '$HUB_ROOT/scripts/cmd-deploy.sh' --diff"
  [[ "$output" == *"inchangé"* ]]
}

# ── --check : skills natives ──────────────────────────────────────────────────

# Ajoute une skill native dans l'agent de test et crée le fichier source
# Note : on remplace le symlink skills/ par un vrai répertoire isolé pour éviter
# que les fichiers de test soient créés dans le vrai skills/ du repo.
_setup_native_skill() {
  cat > "$FAKE_HUB/agents/quality/test-agent.md" <<'AGENTEOF'
---
id: test-agent
label: TestAgent
description: Agent de test minimal
mode: primary
targets: [opencode]
skills: []
native_skills: [shared/test-native-skill]
---

# TestAgent

Contenu de l'agent de test.
AGENTEOF

  # Remplacer le symlink skills/ par un répertoire isolé (évite de polluer le vrai skills/)
  rm -f "$FAKE_HUB/skills"
  mkdir -p "$FAKE_HUB/skills/shared"
  cat > "$FAKE_HUB/skills/shared/test-native-skill.md" <<'SKILLEOF'
---
name: test-native-skill
description: Skill de test pour bats
---

Contenu de la skill de test.
SKILLEOF
}

@test "deploy --check : skill native manquante retourne exit 1" {
  _setup_native_skill

  # Déployer l'agent (mais pas la skill)
  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  # Pas de .opencode/skills/ → MANQUANT

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche MANQUANT pour skill native absente" {
  _setup_native_skill

  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check 2>&1 || true
  [[ "$output" == *"MANQUANT"* ]]
}

@test "deploy --check : skill native obsolète retourne exit 1" {
  _setup_native_skill

  # Déployer l'agent et la skill
  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  mkdir -p "$FAKE_HUB/.opencode/skills/test-native-skill"
  cp "$FAKE_HUB/skills/shared/test-native-skill.md" \
     "$FAKE_HUB/.opencode/skills/test-native-skill/SKILL.md"

  # Toucher la source pour la rendre plus récente que la skill déployée
  sleep 0.1
  touch "$FAKE_HUB/skills/shared/test-native-skill.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche OBSOLÈTE pour skill native plus récente que déployée" {
  _setup_native_skill

  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  mkdir -p "$FAKE_HUB/.opencode/skills/test-native-skill"
  cp "$FAKE_HUB/skills/shared/test-native-skill.md" \
     "$FAKE_HUB/.opencode/skills/test-native-skill/SKILL.md"
  sleep 0.1
  touch "$FAKE_HUB/skills/shared/test-native-skill.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check 2>&1 || true
  [[ "$output" == *"OBSOLÈTE"* ]]
}

@test "deploy --check : skill native à jour retourne exit 0" {
  _setup_native_skill

  # Déployer l'agent
  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  # Déployer la skill APRÈS la source → déployée est plus récente
  sleep 0.1
  mkdir -p "$FAKE_HUB/.opencode/skills/test-native-skill"
  cp "$FAKE_HUB/skills/shared/test-native-skill.md" \
     "$FAKE_HUB/.opencode/skills/test-native-skill/SKILL.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 0 ]
}

@test "deploy --check : affiche À JOUR pour skill native fraîche" {
  _setup_native_skill

  sleep 0.1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  sleep 0.1
  mkdir -p "$FAKE_HUB/.opencode/skills/test-native-skill"
  cp "$FAKE_HUB/skills/shared/test-native-skill.md" \
     "$FAKE_HUB/.opencode/skills/test-native-skill/SKILL.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  [ "$status" -eq 0 ]
  [[ "$output" == *"À JOUR"* ]]
}

# ── Phase 4 : déploiement MCP ─────────────────────────────────────────────────

@test "deploy Phase 4 : cmd-deploy.sh source mcp-deploy.sh (fonctions disponibles)" {
  # Vérifie que les fonctions de mcp-deploy.sh sont bien déclarées après sourcing de cmd-deploy.sh
  run bash -c "
    export HUB_DIR='$FAKE_HUB'
    export _CMD_DEPLOY_SOURCE_ONLY=1
    source '$HUB_ROOT/scripts/cmd-deploy.sh' 2>/dev/null || true
    declare -F check_and_build_mcp deploy_mcp_servers configure_mcp_in_project
  "
  [[ "$output" == *"check_and_build_mcp"* ]]
  [[ "$output" == *"deploy_mcp_servers"* ]]
  [[ "$output" == *"configure_mcp_in_project"* ]]
}

@test "deploy Phase 4 : deploy_mcp_servers copie dist/ dans .opencode/servers/" {
  # Test direct de la fonction (pas via cmd-deploy.sh entier)
  local project_dir
  project_dir=$(mktemp -d)

  # Créer un faux serveur buildé dans le FAKE_HUB
  mkdir -p "$FAKE_HUB/servers/figma-mcp/dist"
  echo '{}' > "$FAKE_HUB/servers/figma-mcp/package.json"
  echo 'console.log("ok")' > "$FAKE_HUB/servers/figma-mcp/dist/index.js"

  run bash -c "
    export HUB_DIR='$FAKE_HUB'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/mcp-deploy.sh'
    # Mocker npm pour éviter un vrai install
    npm() { return 0; }
    export -f npm
    deploy_mcp_servers '$project_dir'
  "
  [ "$status" -eq 0 ]
  [ -f "$project_dir/.opencode/servers/figma-mcp/dist/index.js" ]

  rm -rf "$project_dir"
}
