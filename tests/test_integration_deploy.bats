#!/usr/bin/env bats
# Tests d'intégration pour oc deploy — vérifie avec les vrais agents canoniques
#
# Contrairement à test_cmd_deploy.bats (tests unitaires avec FAKE_HUB + 1 agent minimal),
# ces tests exercent le pipeline complet :
#   1. deploy opencode → génère .opencode/agents/ depuis agents/ et skills/ réels
#   2. --check opencode → vérifie que tous les agents générés sont à jour
#   3. Scénarios de régression : modification source → --check détecte OBSOLÈTE
#
# Ce fichier couvre la lacune qui a permis au --check de rater en production :
# les tests unitaires n'exercent jamais les 27 vrais agents avec leurs vrais skills.

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."

  # DEPLOY_DIR = répertoire temporaire où les agents seront déployés
  # HUB_DIR reste le vrai repo — les sources (agents/, skills/, scripts/) sont réelles
  # On surcharge uniquement HUB_CONFIG pour pointer vers un hub.json de test
  DEPLOY_DIR="$(mktemp -d)"
  mkdir -p "$DEPLOY_DIR/.opencode/agents"

  # hub.json de test — opencode actif, pas de clé API
  HUB_CONFIG_TEST="$(mktemp)"
  cat > "$HUB_CONFIG_TEST" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_target": "opencode",
  "active_targets": ["opencode"],
  "default_provider": {"name": "", "api_key": "", "base_url": "", "model": ""},
  "opencode": {"model": "claude-sonnet-4-5", "disabled_native_agents": []},
  "cli": {"language": "fr"}
}
HUBEOF

  # Exporter HUB_CONFIG pour que common.sh l'utilise plutôt que le vrai hub.json
  export HUB_CONFIG="$HUB_CONFIG_TEST"
  # HUB_DIR reste le vrai repo (sources réelles)
  export HUB_DIR="$HUB_ROOT"

  # Helper : deploy direct via adapter_deploy (bypass adapter_validate → pas besoin d'opencode installé)
  _integration_deploy() {
    bash -c "
      export HUB_DIR='$HUB_ROOT'
      export HUB_CONFIG='$HUB_CONFIG_TEST'
      source '$HUB_ROOT/scripts/common.sh'
      source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
      source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
      log_info()    { true; }
      log_success() { true; }
      log_warn()    { true; }
      adapter_deploy '$DEPLOY_DIR' ''
    " 2>/dev/null
  }

  # Helper : --check en isolant DEPLOY_DIR comme répertoire cible
  # HUB_DIR=DEPLOY_DIR → gen_dir=$DEPLOY_DIR/.opencode/agents (où sont les agents déployés)
  # CANONICAL_AGENTS_DIR=$HUB_ROOT/agents → sources réelles (grâce au fallback ${:-} dans common.sh)
  _integration_check() {
    # Préparer DEPLOY_DIR comme un hub valide pour --check
    mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
    echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
    echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
    touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

    # Exporter les overrides avant que common.sh soit sourcé par cmd-deploy.sh
    # Grâce aux fallbacks ${VAR:-...} dans common.sh, ces valeurs seront respectées
    HUB_DIR="$DEPLOY_DIR" \
    CANONICAL_AGENTS_DIR="$HUB_ROOT/agents" \
    SKILLS_DIR="$HUB_ROOT/skills" \
    LIB_DIR="$HUB_ROOT/scripts/lib" \
    ADAPTERS_DIR="$HUB_ROOT/scripts/adapters" \
    SCRIPTS_DIR="$HUB_ROOT/scripts" \
      bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  }
}

teardown() {
  rm -rf "$DEPLOY_DIR" "$HUB_CONFIG_TEST"
}

@test "intégration : deploy opencode génère tous les agents canoniques" {
  _integration_deploy

  deployed_count=$(find "$DEPLOY_DIR/.opencode/agents" -name "*.md" | wc -l | tr -d ' ')
  source_count=$(find "$HUB_ROOT/agents" -name "*.md" | wc -l | tr -d ' ')

  [ "$deployed_count" -gt 0 ]
  [ "$deployed_count" -le "$source_count" ]
}

@test "intégration : deploy puis --check retourne exit 0 (tous à jour)" {
  _integration_deploy

  mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
  echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
  echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
  touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

  run bash -c "HUB_DIR='$DEPLOY_DIR' CANONICAL_AGENTS_DIR='$HUB_ROOT/agents' SKILLS_DIR='$HUB_ROOT/skills' LIB_DIR='$HUB_ROOT/scripts/lib' ADAPTERS_DIR='$HUB_ROOT/scripts/adapters' SCRIPTS_DIR='$HUB_ROOT/scripts' bash '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode"
  [ "$status" -eq 0 ]
  [[ "$output" == *"à jour"* ]]
  [[ "$output" != *"MANQUANT"* ]]
  [[ "$output" != *"OBSOLÈTE"* ]]
}

@test "intégration : --check sans deploy préalable retourne exit 1 (agents manquants)" {
  mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
  echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
  echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
  touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

  run bash -c "HUB_DIR='$DEPLOY_DIR' CANONICAL_AGENTS_DIR='$HUB_ROOT/agents' SKILLS_DIR='$HUB_ROOT/skills' LIB_DIR='$HUB_ROOT/scripts/lib' ADAPTERS_DIR='$HUB_ROOT/scripts/adapters' SCRIPTS_DIR='$HUB_ROOT/scripts' bash '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode"
  [ "$status" -eq 1 ]
  [[ "$output" == *"MANQUANT"* ]]
}

@test "intégration : --check détecte OBSOLÈTE après modification d'un agent source" {
  _integration_deploy

  mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
  echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
  echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
  touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

  # Trouver le premier agent source
  first_agent=$(find "$HUB_ROOT/agents" -name "*.md" | sort | head -1)
  agent_id=$(grep '^id:' "$first_agent" | head -1 | sed 's/^id:[[:space:]]*//')

  # Rendre la source plus récente que le déployé
  sleep 1
  touch "$first_agent"

  run bash -c "HUB_DIR='$DEPLOY_DIR' CANONICAL_AGENTS_DIR='$HUB_ROOT/agents' SKILLS_DIR='$HUB_ROOT/skills' LIB_DIR='$HUB_ROOT/scripts/lib' ADAPTERS_DIR='$HUB_ROOT/scripts/adapters' SCRIPTS_DIR='$HUB_ROOT/scripts' bash '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]] || [[ "$output" == *"$agent_id"* ]]
}

@test "intégration : --check détecte OBSOLÈTE après modification d'un skill" {
  # Créer un hub factice avec un seul agent qui référence un vrai skill
  TEST_AGENT_DIR="$DEPLOY_DIR/tmp_agents/quality"
  mkdir -p "$TEST_AGENT_DIR"

  # Trouver un skill utilisé par un vrai agent (reviewer → dev-standards-universal)
  skill_rel="developer/dev-standards-universal"
  skill_path="$HUB_ROOT/skills/${skill_rel}.md"
  [ -f "$skill_path" ] || skip "Skill de test introuvable"

  cat > "$TEST_AGENT_DIR/skill-test.md" <<AGENTEOF
---
id: skill-test
label: SkillTest
description: Agent de test avec skill
mode: primary
targets: [opencode]
skills: [$skill_rel]
---
# SkillTest
Contenu.
AGENTEOF

  mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects" "$DEPLOY_DIR/.opencode/agents"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
  echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
  echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
  touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

  # Déployer avec cet agent seul
  bash -c "
    export HUB_DIR='$HUB_ROOT'
    export HUB_CONFIG='$HUB_CONFIG_TEST'
    export CANONICAL_AGENTS_DIR='$DEPLOY_DIR/tmp_agents'
    export SKILLS_DIR='$HUB_ROOT/skills'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
    log_info() { true; }; log_success() { true; }; log_warn() { true; }
    adapter_deploy '$DEPLOY_DIR' ''
  " 2>/dev/null

  # Toucher le skill pour le rendre plus récent que l'agent déployé
  sleep 1
  touch "$skill_path"

  run bash -c "HUB_DIR='$DEPLOY_DIR' CANONICAL_AGENTS_DIR='$DEPLOY_DIR/tmp_agents' SKILLS_DIR='$HUB_ROOT/skills' LIB_DIR='$HUB_ROOT/scripts/lib' ADAPTERS_DIR='$HUB_ROOT/scripts/adapters' SCRIPTS_DIR='$HUB_ROOT/scripts' bash '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]]
}

@test "intégration : opencode.json est généré et valide après deploy" {
  _integration_deploy

  [ -f "$DEPLOY_DIR/opencode.json" ]
  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  result=$(jq -r '.["$schema"]' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "https://opencode.ai/config.json" ]
}

@test "intégration : deploy est idempotent (second deploy ne change pas --check)" {
  mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
  cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
  echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
  echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
  touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"

  _integration_deploy
  _integration_deploy

  run bash -c "HUB_DIR='$DEPLOY_DIR' CANONICAL_AGENTS_DIR='$HUB_ROOT/agents' SKILLS_DIR='$HUB_ROOT/skills' LIB_DIR='$HUB_ROOT/scripts/lib' ADAPTERS_DIR='$HUB_ROOT/scripts/adapters' SCRIPTS_DIR='$HUB_ROOT/scripts' bash '$HUB_ROOT/scripts/cmd-deploy.sh' --check opencode"
  [ "$status" -eq 0 ]
}
