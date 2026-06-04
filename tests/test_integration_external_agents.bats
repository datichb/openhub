#!/usr/bin/env bats
# Tests d'intégration du pipeline complet agents externes → deploy
# Vérifie que les substitutions et compléments sont correctement déployés
# dans .opencode/agents/ du projet.

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  FAKE_HUB="$(mktemp -d)"
  FAKE_PROJECT="$(mktemp -d)"

  # Structure du hub factice
  mkdir -p "$FAKE_HUB/agents/planning"
  mkdir -p "$FAKE_HUB/agents/quality"
  mkdir -p "$FAKE_HUB/config"
  mkdir -p "$FAKE_HUB/projects"

  ln -s "$HUB_ROOT/scripts" "$FAKE_HUB/scripts"
  ln -s "$HUB_ROOT/skills"  "$FAKE_HUB/skills"

  # Agent hub planner
  cat > "$FAKE_HUB/agents/planning/planner.md" <<'AGENTEOF'
---
id: planner
label: ProjectPlanner
description: Planificateur hub
mode: primary
targets: [opencode]
skills: []
---

# ProjectPlanner Hub

Corps du planificateur hub.
AGENTEOF

  # Agent hub reviewer
  cat > "$FAKE_HUB/agents/quality/reviewer.md" <<'AGENTEOF'
---
id: reviewer
label: CodeReviewer
description: Revieweur hub
mode: primary
targets: [opencode]
skills: []
---

# CodeReviewer Hub

Corps du revieweur hub.
AGENTEOF

  # hub.json minimal
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.0.0",
  "default_provider": {"name": "", "api_key": "", "base_url": "", "model": ""},
  "opencode": {"model": "claude-sonnet-4-5", "disabled_native_agents": []},
  "cli": {"language": "fr"}
}
HUBEOF
  cp "$FAKE_HUB/config/hub.json" "$FAKE_HUB/config/hub.json.example"
  echo '{"mappings": {}}' > "$FAKE_HUB/config/stack-skills.json"

  touch "$FAKE_HUB/projects/paths.local.md"
  touch "$FAKE_HUB/projects/api-keys.local.md"

  # Créer l'arborescence du projet avec un agent personnalisé
  mkdir -p "$FAKE_PROJECT/.opencode/agents"

  # Agent planner personnalisé dans le projet (non généré par le hub)
  cat > "$FAKE_PROJECT/.opencode/agents/my-planner.md" <<'AGENTEOF'
---
id: my-planner
label: MyPlanner
description: Planificateur personnalisé du projet
mode: primary
targets: [opencode]
skills: []
---

# MyPlanner

Corps du planificateur personnalisé.
AGENTEOF

  # Agent custom sans équivalent hub
  cat > "$FAKE_PROJECT/.opencode/agents/custom-agent.md" <<'AGENTEOF'
---
id: custom-agent
label: CustomAgent
description: Agent spécifique au projet
mode: primary
targets: [opencode]
skills: []
---

# CustomAgent

Corps de l'agent custom.
AGENTEOF

  export HUB_DIR="$FAKE_HUB"
  export CANONICAL_AGENTS_DIR="$FAKE_HUB/agents"
  export AGENT_ALIASES_FILE="$HUB_ROOT/config/agent-aliases.json"
  export OC_NON_INTERACTIVE=1
}

teardown() {
  rm -rf "$FAKE_HUB" "$FAKE_PROJECT"
}

# ── _load_agent_metadata avec substitution ────────────────────────────────────

@test "intégration : substitution — source de l'agent hub remplacée par l'agent projet" {
  # Configurer projects.md avec une substitution
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-SUB
- Nom : Projet Substitution
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/my-planner.md:substitute:planner
EOF
  echo "INT-SUB=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-SUB" "$FAKE_PROJECT"

  # Vérifier que le tableau contient "planner" avec le fichier du projet
  local found=0
  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    if [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ]; then
      found=1
      # La source doit pointer vers le fichier du projet
      [[ "${_DEPLOY_FILES_AGENT_FILES[$i]}" == *"my-planner.md"* ]]
      break
    fi
    i=$((i + 1))
  done
  [ "$found" -eq 1 ]
}

@test "intégration : substitution — l'agent substitué n'utilise pas le fichier hub" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-SUB2
- Nom : Projet Substitution 2
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/my-planner.md:substitute:planner
EOF
  echo "INT-SUB2=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-SUB2" "$FAKE_PROJECT"

  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    if [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ]; then
      # Ne doit PAS pointer vers le fichier hub
      [[ "${_DEPLOY_FILES_AGENT_FILES[$i]}" != *"$FAKE_HUB/agents"* ]]
      break
    fi
    i=$((i + 1))
  done
}

# ── _load_agent_metadata avec complément ──────────────────────────────────────

@test "intégration : complément — agent ajouté en plus des agents hub" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-COMP
- Nom : Projet Complément
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/custom-agent.md:complement
EOF
  echo "INT-COMP=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-COMP" "$FAKE_PROJECT"

  # Doit contenir à la fois "planner" (hub) et "custom-agent" (complément)
  local has_planner=0 has_custom=0
  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ] && has_planner=1
    [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "custom-agent" ] && has_custom=1
    i=$((i + 1))
  done
  [ "$has_planner" -eq 1 ]
  [ "$has_custom" -eq 1 ]
}

@test "intégration : complément — le nombre total d'agents est hub + compléments" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-COUNT
- Nom : Projet Comptage
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/custom-agent.md:complement
EOF
  echo "INT-COUNT=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-COUNT" "$FAKE_PROJECT"

  # Hub a 2 agents (planner + reviewer), complément ajoute 1 → total 3
  [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 3 ]
}

@test "intégration : complément en doublon avec ID hub existant — ignoré avec warning" {
  # Créer un agent complément avec le même ID que "planner" hub
  cat > "$FAKE_PROJECT/.opencode/agents/dup-planner.md" <<'AGENTEOF'
---
id: planner
label: DupPlanner
---
# Planner en doublon
AGENTEOF

  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-DUP
- Nom : Projet Doublon
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/dup-planner.md:complement
EOF
  echo "INT-DUP=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  # Appel direct (pas via run) pour accéder aux tableaux globaux
  _load_agent_metadata "INT-DUP" "$FAKE_PROJECT"

  # Le doublon doit être ignoré : planner ne doit apparaître qu'une seule fois
  local planner_count=0
  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ] && planner_count=$((planner_count + 1))
    i=$((i + 1))
  done
  [ "$planner_count" -eq 1 ]
}

@test "intégration : substitution avec chemin absolu" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-ABS
- Nom : Projet Absolu
- Stack : TypeScript
- Agents : all
- External agents : ${FAKE_PROJECT}/.opencode/agents/my-planner.md:substitute:planner
EOF
  echo "INT-ABS=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-ABS" "$FAKE_PROJECT"

  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    if [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ]; then
      [[ "${_DEPLOY_FILES_AGENT_FILES[$i]}" == *"my-planner.md"* ]]
      break
    fi
    i=$((i + 1))
  done
}

@test "intégration : fichier substitut introuvable — warning non bloquant" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-MISSING
- Nom : Projet Manquant
- Stack : TypeScript
- Agents : all
- External agents : .opencode/agents/inexistant.md:substitute:planner
EOF
  echo "INT-MISSING=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  # Doit se terminer sans erreur
  run _load_agent_metadata "INT-MISSING" "$FAKE_PROJECT"
  [ "$status" -eq 0 ]

  # Le planner doit toujours pointer vers le hub (substitution ignorée)
  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    if [ "${_DEPLOY_FILES_AGENT_KEYS[$i]}" = "planner" ]; then
      [[ "${_DEPLOY_FILES_AGENT_FILES[$i]}" == *"$FAKE_HUB/agents"* ]]
      break
    fi
    i=$((i + 1))
  done
}

@test "intégration : sans External agents — comportement identique à avant" {
  cat > "$FAKE_HUB/projects/projects.md" <<EOF
## INT-CLEAN
- Nom : Projet Propre
- Stack : TypeScript
- Agents : all
EOF
  echo "INT-CLEAN=${FAKE_PROJECT}" > "$FAKE_HUB/projects/paths.local.md"

  source "$FAKE_HUB/scripts/common.sh"
  source "$FAKE_HUB/scripts/lib/agent-discovery.sh"
  source "$FAKE_HUB/scripts/adapters/opencode.adapter.sh"

  _load_agent_metadata "INT-CLEAN" "$FAKE_PROJECT"

  # Hub a 2 agents → count doit être 2
  [ "${#_DEPLOY_FILES_AGENT_KEYS[@]}" -eq 2 ]
  # Tous les fichiers doivent pointer vers le hub
  local i=0
  while [ "$i" -lt "${#_DEPLOY_FILES_AGENT_KEYS[@]}" ]; do
    [[ "${_DEPLOY_FILES_AGENT_FILES[$i]}" == *"$FAKE_HUB/agents"* ]]
    i=$((i + 1))
  done
}
