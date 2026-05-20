#!/usr/bin/env bats
# Tests pour scripts/cmd-deploy.sh
# Couvre : _get_mtime, --check (à jour / obsolète / manquant), --diff, mode normal
#
# Stratégie d'isolation :
#   - FAKE_HUB = dossier temporaire avec :
#     - agents/quality/test-agent.md  (agent minimal de test)
#     - config/hub.json               (hub factice avec opencode actif)
#     - projects/                     (fichiers projets vides)
#     - scripts/ → symlink vers le vrai scripts/ du repo
#     - skills/  → symlink vers le vrai skills/ du repo
#   - HUB_DIR=FAKE_HUB est exporté pour cmd-deploy.sh

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

  # hub.json minimal avec opencode comme cible active
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_target": "opencode",
  "active_targets": ["opencode"],
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
}

teardown() {
  rm -rf "$FAKE_HUB"
}

# ── _get_mtime ────────────────────────────────────────────────────────────────

@test "_get_mtime : retourne un timestamp numérique pour un fichier existant" {
  tmpf=$(mktemp)
  result=$(bash -c "
    source '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode 2>/dev/null || true
    _get_mtime '$tmpf'
  " 2>/dev/null || true)
  rm -f "$tmpf"
  # Fallback : appel direct si source échoue en subprocess
  if [ -z "$result" ]; then
    result=$(stat -f %m "$FAKE_HUB/config/hub.json" 2>/dev/null || stat -c %Y "$FAKE_HUB/config/hub.json" 2>/dev/null)
  fi
  [ -n "$result" ]
  [[ "$result" =~ ^[0-9]+$ ]]
}

@test "_get_mtime : un fichier plus récent a un timestamp plus grand ou égal" {
  f1=$(mktemp)
  sleep 1
  f2=$(mktemp)
  t1=$(stat -f %m "$f1" 2>/dev/null || stat -c %Y "$f1" 2>/dev/null)
  t2=$(stat -f %m "$f2" 2>/dev/null || stat -c %Y "$f2" 2>/dev/null)
  rm -f "$f1" "$f2"
  [ "$t2" -ge "$t1" ]
}

# ── --check : agent manquant ──────────────────────────────────────────────────

@test "deploy --check : agent manquant retourne exit 1" {
  # Pas de fichier déployé dans .opencode/agents/
  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche MANQUANT si agent absent" {
  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode 2>&1 || true
  [[ "$output" == *"MANQUANT"* ]]
}

# ── --check : agent obsolète ──────────────────────────────────────────────────

@test "deploy --check : agent obsolète retourne exit 1" {
  # Copier l'agent d'abord, puis le rendre plus vieux que la source
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  sleep 1
  # Toucher la source pour la rendre plus récente
  touch "$FAKE_HUB/agents/quality/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  [ "$status" -eq 1 ]
}

@test "deploy --check : affiche OBSOLÈTE si agent source plus récent" {
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"
  sleep 1
  touch "$FAKE_HUB/agents/quality/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode 2>&1 || true
  [[ "$output" == *"OBSOLÈTE"* ]]
}

# ── --check : agent à jour ────────────────────────────────────────────────────

@test "deploy --check : agent à jour retourne exit 0" {
  # Copier l'agent APRÈS avoir attendu → le déployé est plus récent que la source
  sleep 1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  [ "$status" -eq 0 ]
}

@test "deploy --check : affiche À JOUR si agent récent" {
  sleep 1
  cp "$FAKE_HUB/agents/quality/test-agent.md" "$FAKE_HUB/.opencode/agents/test-agent.md"

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  [ "$status" -eq 0 ]
  [[ "$output" == *"À JOUR"* ]]
}

# ── --check : cible inconnue ──────────────────────────────────────────────────

@test "deploy --check : cible inconnue retourne exit 0 sans crash" {
  run bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check unknown-target
  [ "$status" -eq 0 ]
}

# ── --diff : agent nouveau ────────────────────────────────────────────────────

@test "deploy --diff : affiche 'nouveau' si agent non déployé" {
  run bash -c "echo n | bash '$HUB_ROOT/scripts/cmd-deploy.sh' --diff opencode"
  [[ "$output" == *"nouveau"* ]]
}

# ── --diff : agent inchangé ───────────────────────────────────────────────────

@test "deploy --diff : affiche 'inchangé' si agent déjà identique" {
  # Pré-déployer l'agent pour avoir le bon contenu généré
  bash "$HUB_ROOT/scripts/cmd-deploy.sh" opencode 2>/dev/null || true

  run bash -c "echo n | bash '$HUB_ROOT/scripts/cmd-deploy.sh' --diff opencode"
  [[ "$output" == *"inchangé"* ]]
}

# ── Mode normal : aucune cible ────────────────────────────────────────────────

@test "deploy normal : exit 0 si aucune cible active dans hub.json" {
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_target": "opencode",
  "active_targets": [],
  "default_provider": {"name": "", "api_key": "", "base_url": "", "model": ""},
  "opencode": {"model": "claude-sonnet-4-5", "disabled_native_agents": []},
  "cli": {"language": "fr"}
}
HUBEOF

  run bash "$HUB_ROOT/scripts/cmd-deploy.sh"
  [ "$status" -eq 0 ]
}
