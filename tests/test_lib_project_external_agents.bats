#!/usr/bin/env bats
# Tests unitaires pour les fonctions External agents de scripts/lib/project.sh
# Fonctions testées : get_project_external_agents, _set_project_external_agents,
#                     get_project_substitute_agents, get_project_complement_agents

load helpers

setup() {
  common_setup

  source "$BATS_TEST_DIRNAME/../scripts/common.sh"
  PROJECTS_FILE="$TEST_DIR/projects.md"
  API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  PATHS_FILE="$TEST_DIR/paths.local.md"
}

teardown() {
  common_teardown
}

# ── get_project_external_agents ───────────────────────────────────────────────

@test "get_project_external_agents : retourne vide si champ absent" {
  make_test_project "PROJ-A" "Projet A" "TypeScript"
  run get_project_external_agents "PROJ-A"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_external_agents : retourne la valeur du champ" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-B
- Nom : Projet B
- Stack : Vue
- External agents : .opencode/agents/planner.md:substitute:planner
EOF
  run get_project_external_agents "PROJ-B"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/planner.md:substitute:planner" ]
}

@test "get_project_external_agents : retourne la valeur multi-entrées avec pipe" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-C
- Nom : Projet C
- Stack : React
- External agents : .opencode/agents/planner.md:substitute:planner|.opencode/agents/my-qa.md:complement
EOF
  run get_project_external_agents "PROJ-C"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/planner.md:substitute:planner|.opencode/agents/my-qa.md:complement" ]
}

@test "get_project_external_agents : retourne vide pour projet inexistant" {
  run get_project_external_agents "PROJ-INEXISTANT"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── _set_project_external_agents ─────────────────────────────────────────────

@test "_set_project_external_agents : crée le champ si absent (après Agents)" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SET
- Nom : Projet Set
- Stack : TypeScript
- Agents : all
EOF
  _set_project_external_agents "PROJ-SET" ".opencode/agents/planner.md:substitute:planner"
  assert_file_contains "$PROJECTS_FILE" "- External agents : .opencode/agents/planner.md:substitute:planner"
}

@test "_set_project_external_agents : met à jour le champ existant" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-UPD
- Nom : Projet Update
- Stack : TypeScript
- External agents : ancienne-valeur:complement
EOF
  _set_project_external_agents "PROJ-UPD" ".opencode/agents/new.md:complement"
  assert_file_contains "$PROJECTS_FILE" "- External agents : .opencode/agents/new.md:complement"
  assert_file_not_contains "$PROJECTS_FILE" "ancienne-valeur:complement"
}

@test "_set_project_external_agents : supprime le champ si valeur vide" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-DEL
- Nom : Projet Delete
- Stack : TypeScript
- External agents : .opencode/agents/planner.md:substitute:planner
EOF
  _set_project_external_agents "PROJ-DEL" ""
  assert_file_not_contains "$PROJECTS_FILE" "External agents"
}

@test "_set_project_external_agents : ne modifie pas les autres projets" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-X
- Nom : Projet X
- Stack : A
- External agents : valeur-x:complement

## PROJ-Y
- Nom : Projet Y
- Stack : B
- Agents : all
EOF
  _set_project_external_agents "PROJ-Y" ".opencode/agents/y.md:complement"
  # PROJ-X non modifié
  assert_file_contains "$PROJECTS_FILE" "valeur-x:complement"
  # PROJ-Y mis à jour
  assert_file_contains "$PROJECTS_FILE" ".opencode/agents/y.md:complement"
}

# ── get_project_substitute_agents ────────────────────────────────────────────

@test "get_project_substitute_agents : retourne vide si champ absent" {
  make_test_project "PROJ-SUB-ABSENT" "Projet" "Stack"
  run get_project_substitute_agents "PROJ-SUB-ABSENT"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_substitute_agents : retourne vide si aucune substitution" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-NO-SUB
- Nom : Projet
- Stack : Stack
- External agents : .opencode/agents/qa.md:complement
EOF
  run get_project_substitute_agents "PROJ-NO-SUB"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_substitute_agents : retourne les substitutions au format path:hub-id" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SUB
- Nom : Projet Sub
- Stack : Stack
- External agents : .opencode/agents/planner.md:substitute:planner
EOF
  run get_project_substitute_agents "PROJ-SUB"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/planner.md:planner" ]
}

@test "get_project_substitute_agents : filtre uniquement les substitutions parmi entrées mixtes" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MIX
- Nom : Projet Mix
- Stack : Stack
- External agents : .opencode/agents/planner.md:substitute:planner|.opencode/agents/qa.md:complement
EOF
  run get_project_substitute_agents "PROJ-MIX"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/planner.md:planner" ]
  [[ "$output" != *"qa.md"* ]]
}

@test "get_project_substitute_agents : retourne plusieurs substitutions" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MULTI-SUB
- Nom : Projet Multi
- Stack : Stack
- External agents : .opencode/agents/a.md:substitute:planner|.opencode/agents/b.md:substitute:reviewer
EOF
  run get_project_substitute_agents "PROJ-MULTI-SUB"
  [ "$status" -eq 0 ]
  [ "$(echo "$output" | wc -l | tr -d ' ')" -eq 2 ]
  [[ "$output" == *".opencode/agents/a.md:planner"* ]]
  [[ "$output" == *".opencode/agents/b.md:reviewer"* ]]
}

# ── get_project_complement_agents ────────────────────────────────────────────

@test "get_project_complement_agents : retourne vide si champ absent" {
  make_test_project "PROJ-COMP-ABSENT" "Projet" "Stack"
  run get_project_complement_agents "PROJ-COMP-ABSENT"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_complement_agents : retourne vide si aucun complément" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-NO-COMP
- Nom : Projet
- Stack : Stack
- External agents : .opencode/agents/planner.md:substitute:planner
EOF
  run get_project_complement_agents "PROJ-NO-COMP"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_complement_agents : retourne les chemins des compléments" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-COMP
- Nom : Projet Comp
- Stack : Stack
- External agents : .opencode/agents/my-qa.md:complement
EOF
  run get_project_complement_agents "PROJ-COMP"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/my-qa.md" ]
}

@test "get_project_complement_agents : filtre uniquement les compléments parmi entrées mixtes" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MIX-COMP
- Nom : Projet Mix Comp
- Stack : Stack
- External agents : .opencode/agents/planner.md:substitute:planner|.opencode/agents/my-qa.md:complement
EOF
  run get_project_complement_agents "PROJ-MIX-COMP"
  [ "$status" -eq 0 ]
  [ "$output" = ".opencode/agents/my-qa.md" ]
  [[ "$output" != *"planner.md"* ]]
}

@test "get_project_complement_agents : retourne plusieurs compléments" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MULTI-COMP
- Nom : Projet Multi Comp
- Stack : Stack
- External agents : .opencode/agents/qa.md:complement|.opencode/agents/custom.md:complement
EOF
  run get_project_complement_agents "PROJ-MULTI-COMP"
  [ "$status" -eq 0 ]
  [ "$(echo "$output" | wc -l | tr -d ' ')" -eq 2 ]
  [[ "$output" == *".opencode/agents/qa.md"* ]]
  [[ "$output" == *".opencode/agents/custom.md"* ]]
}
