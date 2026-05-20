#!/usr/bin/env bats
# Tests d'intégration pour oc deploy — vérifie avec les vrais agents canoniques
#
# Contrairement à test_cmd_deploy.bats (tests unitaires avec FAKE_HUB + 1 agent minimal),
# ces tests exercent le pipeline complet :
#   1. deploy opencode → génère .opencode/agents/ depuis agents/ et skills/ réels
#   2. --check opencode → vérifie que tous les agents générés sont à jour
#   3. Scénarios de régression : modification source → --check détecte OBSOLÈTE
#
# IMPORTANT — isolation des sources :
#   Les tests qui simulent une modification (OBSOLÈTE) NE TOUCHENT JAMAIS les fichiers
#   de $HUB_ROOT/agents/ ou $HUB_ROOT/skills/ directement.
#   Ils travaillent sur des COPIES dans $DEPLOY_DIR/tmp_* pour ne pas altérer les
#   mtimes réels du repo et casser le --check hub après les tests.

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."

  # DEPLOY_DIR = répertoire temporaire isolé pour tous les artefacts de test
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

  export HUB_CONFIG="$HUB_CONFIG_TEST"
  export HUB_DIR="$HUB_ROOT"

  # Helper : copier agents/ et skills/ dans DEPLOY_DIR pour isolation complète
  # Retourne les chemins des copies via les variables TMP_AGENTS_DIR et TMP_SKILLS_DIR
  _make_isolated_sources() {
    TMP_AGENTS_DIR="$DEPLOY_DIR/tmp_agents"
    TMP_SKILLS_DIR="$DEPLOY_DIR/tmp_skills"
    cp -R "$HUB_ROOT/agents" "$TMP_AGENTS_DIR"
    cp -R "$HUB_ROOT/skills" "$TMP_SKILLS_DIR"
  }

  # Helper : préparer DEPLOY_DIR comme un hub valide (config + projects)
  _setup_deploy_hub() {
    mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
    echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
    echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
    touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"
  }

  # Helper : deploy via adapter_deploy direct (bypass adapter_validate — pas d'opencode en CI)
  # Usage : _deploy_to AGENTS_DIR SKILLS_DIR
  _deploy_to() {
    local agents_dir="${1:-$HUB_ROOT/agents}"
    local skills_dir="${2:-$HUB_ROOT/skills}"
    bash -c "
      export HUB_DIR='$HUB_ROOT'
      export HUB_CONFIG='$HUB_CONFIG_TEST'
      export CANONICAL_AGENTS_DIR='$agents_dir'
      export SKILLS_DIR='$skills_dir'
      source '$HUB_ROOT/scripts/common.sh'
      source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
      source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
      log_info()    { true; }
      log_success() { true; }
      log_warn()    { true; }
      adapter_deploy '$DEPLOY_DIR' ''
    " 2>/dev/null
  }

  # Helper : --check avec overrides d'environnement
  # Appelle cmd-deploy.sh dans un vrai subprocess pour que run capture le bon exit code
  # Usage : run _run_check AGENTS_DIR SKILLS_DIR
  _run_check() {
    local agents_dir="${1:-$HUB_ROOT/agents}"
    local skills_dir="${2:-$HUB_ROOT/skills}"
    _setup_deploy_hub
    HUB_DIR="$DEPLOY_DIR" \
    CANONICAL_AGENTS_DIR="$agents_dir" \
    SKILLS_DIR="$skills_dir" \
    LIB_DIR="$HUB_ROOT/scripts/lib" \
    ADAPTERS_DIR="$HUB_ROOT/scripts/adapters" \
    SCRIPTS_DIR="$HUB_ROOT/scripts" \
      bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check opencode
  }
}

teardown() {
  rm -rf "$DEPLOY_DIR" "$HUB_CONFIG_TEST"
}

# ── Déploiement complet ───────────────────────────────────────────────────────

@test "intégration : deploy opencode génère tous les agents canoniques" {
  _deploy_to "$HUB_ROOT/agents" "$HUB_ROOT/skills"

  deployed_count=$(find "$DEPLOY_DIR/.opencode/agents" -name "*.md" | wc -l | tr -d ' ')
  source_count=$(find "$HUB_ROOT/agents" -name "*.md" | wc -l | tr -d ' ')

  [ "$deployed_count" -gt 0 ]
  [ "$deployed_count" -le "$source_count" ]
}

@test "intégration : deploy puis --check retourne exit 0 (tous à jour)" {
  _setup_deploy_hub
  _deploy_to "$HUB_ROOT/agents" "$HUB_ROOT/skills"

  run _run_check "$HUB_ROOT/agents" "$HUB_ROOT/skills"
  [ "$status" -eq 0 ]
  [[ "$output" == *"à jour"* ]]
  [[ "$output" != *"MANQUANT"* ]]
  [[ "$output" != *"OBSOLÈTE"* ]]
}

@test "intégration : --check sans deploy préalable retourne exit 1 (agents manquants)" {
  # .opencode/agents/ est vide — aucun deploy effectué
  run _run_check "$HUB_ROOT/agents" "$HUB_ROOT/skills"
  [ "$status" -eq 1 ]
  [[ "$output" == *"MANQUANT"* ]]
}

# ── Scénarios OBSOLÈTE — isolation totale des sources ────────────────────────
# Ces tests copient agents/ et skills/ dans $DEPLOY_DIR pour ne JAMAIS toucher
# les fichiers réels de $HUB_ROOT, évitant d'altérer les mtimes du repo.

@test "intégration : --check détecte OBSOLÈTE après modification d'un agent source" {
  _make_isolated_sources

  # Deploy avec les copies isolées
  _deploy_to "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"

  # Rendre une copie d'agent plus récente que l'agent déployé
  sleep 1
  first_agent=$(find "$TMP_AGENTS_DIR" -name "*.md" | sort | head -1)
  agent_id=$(grep '^id:' "$first_agent" | head -1 | sed 's/^id:[[:space:]]*//')
  touch "$first_agent"

  # Vérifier avec les mêmes copies
  run _run_check "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]] || [[ "$output" == *"$agent_id"* ]]
}

@test "intégration : --check détecte OBSOLÈTE après modification d'un skill" {
  _make_isolated_sources

  # Trouver un skill référencé (dev-standards-universal est utilisé par reviewer, developer-*)
  skill_rel="developer/dev-standards-universal"
  skill_copy="$TMP_SKILLS_DIR/${skill_rel}.md"
  [ -f "$skill_copy" ] || skip "Skill de test introuvable dans la copie"

  # Deploy avec les copies isolées
  _deploy_to "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"

  # Rendre la COPIE du skill plus récente — jamais l'original
  sleep 1
  touch "$skill_copy"

  # Vérifier avec les mêmes copies
  run _run_check "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]]
}

# ── Qualité des artefacts ─────────────────────────────────────────────────────

@test "intégration : opencode.json est généré et valide après deploy" {
  _deploy_to "$HUB_ROOT/agents" "$HUB_ROOT/skills"

  [ -f "$DEPLOY_DIR/opencode.json" ]
  command -v jq &>/dev/null || skip "jq non disponible"
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]
  result=$(jq -r '.["$schema"]' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "https://opencode.ai/config.json" ]
}

@test "intégration : deploy est idempotent (second deploy ne change pas --check)" {
  # Tester l'idempotence avec un seul agent pour rester rapide
  # On déploie 2 fois de suite — le --check doit passer après les deux
  mkdir -p "$DEPLOY_DIR/tmp_single/quality"
  cat > "$DEPLOY_DIR/tmp_single/quality/single.md" <<'AGENTEOF'
---
id: single
label: Single
description: Agent idempotence test
mode: primary
targets: [opencode]
skills: []
---
# Single
AGENTEOF

  _setup_deploy_hub
  _deploy_to "$DEPLOY_DIR/tmp_single" "$HUB_ROOT/skills"
  _deploy_to "$DEPLOY_DIR/tmp_single" "$HUB_ROOT/skills"

  run _run_check "$DEPLOY_DIR/tmp_single" "$HUB_ROOT/skills"
  [ "$status" -eq 0 ]
}

# ── adapter_deploy_config autonome (sans Phase 1 préalable) ──────────────────
# Vérifie que la Phase 2 est autonome et produit un opencode.json complet
# même sans avoir exécuté adapter_deploy_files avant.

@test "intégration : adapter_deploy_config seul génère opencode.json complet" {
  command -v jq &>/dev/null || skip "jq non disponible"

  bash -c "
    export HUB_DIR='$HUB_ROOT'
    export HUB_CONFIG='$HUB_CONFIG_TEST'
    export CANONICAL_AGENTS_DIR='$HUB_ROOT/agents'
    export SKILLS_DIR='$HUB_ROOT/skills'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
    log_info()    { true; }
    log_success() { true; }
    log_warn()    { true; }
    # Appel direct de la Phase 2 sans Phase 1 préalable
    adapter_deploy_config '$DEPLOY_DIR' ''
  " 2>/dev/null

  # opencode.json doit exister et être valide
  [ -f "$DEPLOY_DIR/opencode.json" ]
  run jq . "$DEPLOY_DIR/opencode.json"
  [ "$status" -eq 0 ]

  # Le schéma doit être présent
  result=$(jq -r '.["$schema"]' "$DEPLOY_DIR/opencode.json")
  [ "$result" = "https://opencode.ai/config.json" ]
}

@test "intégration : adapter_deploy_config seul produit le même opencode.json que le déploiement complet" {
  command -v jq &>/dev/null || skip "jq non disponible"

  DEPLOY_FULL="$(mktemp -d)"
  DEPLOY_CONFIG_ONLY="$(mktemp -d)"

  # Déploiement complet (Phase 1 + Phase 2)
  bash -c "
    export HUB_DIR='$HUB_ROOT'
    export HUB_CONFIG='$HUB_CONFIG_TEST'
    export CANONICAL_AGENTS_DIR='$HUB_ROOT/agents'
    export SKILLS_DIR='$HUB_ROOT/skills'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
    log_info()    { true; }
    log_success() { true; }
    log_warn()    { true; }
    adapter_deploy '$DEPLOY_FULL' ''
  " 2>/dev/null

  # Phase 2 seule
  bash -c "
    export HUB_DIR='$HUB_ROOT'
    export HUB_CONFIG='$HUB_CONFIG_TEST'
    export CANONICAL_AGENTS_DIR='$HUB_ROOT/agents'
    export SKILLS_DIR='$HUB_ROOT/skills'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
    log_info()    { true; }
    log_success() { true; }
    log_warn()    { true; }
    adapter_deploy_config '$DEPLOY_CONFIG_ONLY' ''
  " 2>/dev/null

  # Les deux opencode.json doivent être identiques
  diff "$DEPLOY_FULL/opencode.json" "$DEPLOY_CONFIG_ONLY/opencode.json"

  rm -rf "$DEPLOY_FULL" "$DEPLOY_CONFIG_ONLY"
}
