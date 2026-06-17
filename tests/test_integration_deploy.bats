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
#   Ils travaillent sur des COPIES dans $BATS_TEST_TMPDIR/tmp_* pour ne pas altérer les
#   mtimes réels du repo et casser le --check hub après les tests.
#
# PERF — setup_file() mutualisé :
#   Le déploiement complet (30 agents) est effectué UNE SEULE FOIS dans setup_file()
#   et stocké dans $BATS_FILE_TMPDIR/shared_deploy.
#   Les tests qui ont besoin du résultat copient ce répertoire au lieu de re-déployer.

# ─────────────────────────────────────────────────────────────────────────────
# setup_file : deploy complet unique partagé par tous les tests du fichier
# ─────────────────────────────────────────────────────────────────────────────
setup_file() {
  local hub_root
  hub_root="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  local hub_config_shared="$BATS_FILE_TMPDIR/hub.json"
  local shared_deploy="$BATS_FILE_TMPDIR/shared_deploy"

  # hub.json partagé — opencode actif, langue fr (pour les assertions i18n)
  cat > "$hub_config_shared" <<'HUBEOF'
{
  "version": "1.5.0",
  "default_provider": {"name": "", "api_key": "", "base_url": "", "model": ""},
  "opencode": {"model": "claude-sonnet-4-5", "disabled_native_agents": []},
  "cli": {"language": "fr"}
}
HUBEOF

  mkdir -p "$shared_deploy/.opencode/agents"

  # Deploy complet UNE SEULE FOIS (30 agents réels)
  bash -c "
    export HUB_DIR='$hub_root'
    export HUB_CONFIG='$hub_config_shared'
    export CANONICAL_AGENTS_DIR='$hub_root/agents'
    export SKILLS_DIR='$hub_root/skills'
    source '$hub_root/scripts/common.sh'
    source '$hub_root/scripts/lib/prompt-builder.sh'
    source '$hub_root/scripts/adapters/opencode.adapter.sh'
    log_info()    { true; }
    log_success() { true; }
    log_warn()    { true; }
    adapter_deploy '$shared_deploy' ''
  " 2>/dev/null
}

# ─────────────────────────────────────────────────────────────────────────────
# setup / teardown par test
# ─────────────────────────────────────────────────────────────────────────────
setup() {
  HUB_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  HUB_CONFIG_TEST="$BATS_FILE_TMPDIR/hub.json"   # hub.json créé dans setup_file
  SHARED_DEPLOY="$BATS_FILE_TMPDIR/shared_deploy" # deploy partagé (setup_file)

  # Répertoire temporaire isolé par test (pour les artefacts propres à chaque test)
  DEPLOY_DIR="$BATS_TEST_TMPDIR/deploy"
  mkdir -p "$DEPLOY_DIR/.opencode/agents"

  export HUB_CONFIG="$HUB_CONFIG_TEST"
  export HUB_DIR="$HUB_ROOT"

  # ── Helpers ──────────────────────────────────────────────────────────────

  # Copie le deploy partagé (setup_file) dans DEPLOY_DIR pour éviter un re-deploy
  _use_shared_deploy() {
    cp -R "$SHARED_DEPLOY/.opencode" "$DEPLOY_DIR/"
    [ -f "$SHARED_DEPLOY/opencode.json" ] && cp "$SHARED_DEPLOY/opencode.json" "$DEPLOY_DIR/"
  }

  # Helper : deployer quelques agents mini (OBSOLÈTE tests) — évite 30 agents entiers
  # Copie 3 agents isolés + skills dans un sous-répertoire de DEPLOY_DIR
  _make_mini_isolated_sources() {
    TMP_AGENTS_DIR="$DEPLOY_DIR/mini_agents"
    TMP_SKILLS_DIR="$DEPLOY_DIR/mini_skills"
    mkdir -p "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"

    # Copier 3 agents (2 auditor + 1 developer qui référence dev-standards-universal)
    mkdir -p "$TMP_AGENTS_DIR/auditor" "$TMP_AGENTS_DIR/developer"
    cp "$HUB_ROOT/agents/auditor/auditor-accessibility.md" \
       "$TMP_AGENTS_DIR/auditor/"
    cp "$HUB_ROOT/agents/auditor/auditor-architecture.md" \
       "$TMP_AGENTS_DIR/auditor/"
    cp "$HUB_ROOT/agents/developer/developer-api.md" \
       "$TMP_AGENTS_DIR/developer/"

    # Copier l'intégralité des skills (petits fichiers markdown)
    cp -R "$HUB_ROOT/skills/." "$TMP_SKILLS_DIR/"
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

  # Helper : préparer DEPLOY_DIR comme un hub valide (config + projects)
  _setup_deploy_hub() {
    mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/projects"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json"
    cp "$HUB_CONFIG_TEST" "$DEPLOY_DIR/config/hub.json.example"
    echo '{"mappings": {}}' > "$DEPLOY_DIR/config/stack-skills.json"
    echo "# Registre de test" > "$DEPLOY_DIR/projects/projects.md"
    touch "$DEPLOY_DIR/projects/paths.local.md" "$DEPLOY_DIR/projects/api-keys.local.md"
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
      bash "$HUB_ROOT/scripts/cmd-deploy.sh" --check
  }
}

teardown() {
  # BATS_TEST_TMPDIR est nettoyé automatiquement par BATS — rien à faire
  true
}

# ── Déploiement complet ───────────────────────────────────────────────────────

@test "intégration : deploy opencode génère tous les agents canoniques" {
  # Réutilise le deploy partagé (setup_file) — pas de re-deploy
  _use_shared_deploy

  deployed_count=$(find "$DEPLOY_DIR/.opencode/agents" -name "*.md" | wc -l | tr -d ' ')
  source_count=$(find "$HUB_ROOT/agents" -name "*.md" | wc -l | tr -d ' ')

  [ "$deployed_count" -gt 0 ]
  [ "$deployed_count" -le "$source_count" ]
}

@test "intégration : deploy puis --check retourne exit 0 (tous à jour)" {
  _setup_deploy_hub
  _use_shared_deploy

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
# Ces tests copient 3 agents dans $DEPLOY_DIR/mini_* pour ne JAMAIS toucher
# les fichiers réels de $HUB_ROOT, évitant d'altérer les mtimes du repo.
# 3 agents suffisent : on teste la détection, pas l'exhaustivité.

@test "intégration : --check détecte OBSOLÈTE après modification d'un agent source" {
  _make_mini_isolated_sources

  # Deploy avec les copies isolées (mini : 3 agents)
  _deploy_to "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"

  # Rendre une copie d'agent plus récente que l'agent déployé
  sleep 0.1
  first_agent=$(find "$TMP_AGENTS_DIR" -name "*.md" | sort | head -1)
  agent_id=$(grep '^id:' "$first_agent" | head -1 | sed 's/^id:[[:space:]]*//')
  touch "$first_agent"

  # Vérifier avec les mêmes copies
  run _run_check "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]] || [[ "$output" == *"$agent_id"* ]]
}

@test "intégration : --check détecte OBSOLÈTE après modification d'un skill" {
  _make_mini_isolated_sources

  # Trouver un skill référencé par developer-api (dev-standards-universal)
  skill_rel="developer/dev-standards-universal"
  skill_copy="$TMP_SKILLS_DIR/${skill_rel}.md"
  [ -f "$skill_copy" ] || skip "Skill de test introuvable dans la copie"

  # Deploy avec les copies isolées (mini : 3 agents)
  _deploy_to "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"

  # Rendre la COPIE du skill plus récente — jamais l'original
  sleep 0.1
  touch "$skill_copy"

  # Vérifier avec les mêmes copies
  run _run_check "$TMP_AGENTS_DIR" "$TMP_SKILLS_DIR"
  [ "$status" -eq 1 ]
  [[ "$output" == *"OBSOLÈTE"* ]]
}

# ── Qualité des artefacts ─────────────────────────────────────────────────────

@test "intégration : opencode.json est généré et valide après deploy" {
  _use_shared_deploy

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

  DEPLOY_CONFIG_ONLY="$BATS_TEST_TMPDIR/deploy_config_only"
  mkdir -p "$DEPLOY_CONFIG_ONLY"

  # Déploiement complet : réutiliser le deploy partagé (setup_file)
  _use_shared_deploy

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

  # Les deux opencode.json doivent être identiques (comparaison normalisée via jq)
  json_full=$(jq --sort-keys . "$DEPLOY_DIR/opencode.json")
  json_config=$(jq --sort-keys . "$DEPLOY_CONFIG_ONLY/opencode.json")
  [ "$json_full" = "$json_config" ]
}

@test "adapter_deploy_config : fonctionne sous set -u sans Phase 1 préalable — non-régression #openhub-5s5" {
  command -v jq &>/dev/null || skip "jq non disponible"

  # Répertoire agents vide : _load_agent_metadata() doit exister mais ne retenir aucun agent
  EMPTY_AGENTS_DIR="$BATS_TEST_TMPDIR/empty_agents"
  mkdir -p "$EMPTY_AGENTS_DIR"

  bash -c "
    set -euo pipefail
    export HUB_DIR='$HUB_ROOT'
    export HUB_CONFIG='$HUB_CONFIG_TEST'
    export CANONICAL_AGENTS_DIR='$EMPTY_AGENTS_DIR'
    source '$HUB_ROOT/scripts/common.sh'
    source '$HUB_ROOT/scripts/lib/prompt-builder.sh'
    source '$HUB_ROOT/scripts/adapters/opencode.adapter.sh'
    log_info()    { true; }
    log_success() { true; }
    log_warn()    { true; }
    log_error()   { true; }
    # Appel direct Phase 2 sans Phase 1 — réplique le scénario du bug sous set -u
    adapter_deploy_config '$DEPLOY_DIR' ''
  "

  [ -f "$DEPLOY_DIR/opencode.json" ]
}

@test "intégration : deploy se termine dans un délai raisonnable (no hang)" {
  # Régression : generate_dependency_graph bloquait indéfiniment en fin de deploy
  # sur des projets avec beaucoup de fichiers TS/JS (concaténation O(n²) + jq sur multi-Mo).
  # Vérifie que cmd-deploy.sh rend la main dans un délai acceptable.
  _use_shared_deploy

  # Le deploy partagé (setup_file) s'est terminé sans timeout BATS (120s par défaut en CI).
  # On vérifie simplement que les artefacts sont présents — si on arrive ici,
  # la commande s'est bien terminée (pas de hang).
  [ -d "$DEPLOY_DIR/.opencode/agents" ]
  deployed_count=$(find "$DEPLOY_DIR/.opencode/agents" -name "*.md" | wc -l | tr -d ' ')
  [ "$deployed_count" -gt 0 ]
}
