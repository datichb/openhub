#!/usr/bin/env bats

# Tests d'intégration — Mécanisme d'interruption de session inter-agents
# Vérifie que tous les agents implémentant le mécanisme ont bien les blocs
# et marqueurs requis dans leurs fichiers skill/agent.

load helpers

SKILLS_DIR="$BATS_TEST_DIRNAME/../skills"
AGENTS_DIR="$BATS_TEST_DIRNAME/../agents"

# ── Détection du contexte d'invocation ───────────────────────────────────────

@test "planner-workflow contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/planning/planner-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "pathfinder-protocol contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/planning/pathfinder-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "onboarder-workflow contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/planning/onboarder-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "auditor-workflow contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/auditor/auditor-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "debugger-workflow contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/quality/debugger-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-dev-protocol contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/orchestrator/orchestrator-dev-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "design-handoff-format contient la détection CONTEXTE = orchestrateur_feature" {
  run grep -c "CONTEXTE = orchestrator_feature" "$SKILLS_DIR/design/design-handoff-format.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Présence du bloc ## Retour intermédiaire vers orchestrator ───────────────

@test "planner-workflow contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/planning/planner-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "onboarder-workflow contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/planning/onboarder-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "auditor-workflow contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/auditor/auditor-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "debugger-workflow contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/quality/debugger-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "pathfinder-protocol contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/planning/pathfinder-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "pathfinder-handoff-format contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/planning/pathfinder-handoff-format.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "design-handoff-format contient le bloc Retour intermédiaire vers orchestrateur" {
  run grep -c "## Retour intermédiaire vers orchestrator" "$SKILLS_DIR/design/design-handoff-format.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Présence du bloc ## Question pour l'orchestrateur (ou orchestrator) ───────

@test "planner-workflow contient le bloc Question pour l'orchestrateur" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/planning/planner-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-dev-protocol contient le bloc Question pour l'orchestrator" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/orchestrator/orchestrator-dev-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "pathfinder-handoff-format contient le bloc Question pour l'orchestrateur" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/planning/pathfinder-handoff-format.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "debugger-workflow contient le bloc Question pour l'orchestrateur" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/quality/debugger-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "onboarder-workflow contient le bloc Question pour l'orchestrateur" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/planning/onboarder-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "auditor-workflow contient le bloc Question pour l'orchestrateur" {
  run grep -c "## Question pour l'orchestrator" "$SKILLS_DIR/auditor/auditor-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Présence du champ task_id dans les blocs de question montante ─────────────

@test "planner-workflow blocs Question pour l'orchestrateur contiennent task_id" {
  run grep -c "task_id" "$SKILLS_DIR/planning/planner-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-dev-protocol blocs Question pour l'orchestrator contiennent task_id" {
  run grep -c "task_id" "$SKILLS_DIR/orchestrator/orchestrator-dev-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "debugger-workflow contient task_id dans les blocs de question" {
  run grep -c "task_id" "$SKILLS_DIR/quality/debugger-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Interdiction de l'outil question en mode orchestrateur_feature ────────────

@test "pathfinder-protocol documente l'interdiction de l'outil question en mode orchestrateur" {
  run grep -c "JAMAIS.*question\|question.*JAMAIS\|Ne jamais.*question\|jamais.*outil.*question" "$SKILLS_DIR/planning/pathfinder-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "ux-designer documente l'interdiction de l'outil question en mode orchestrateur" {
  run grep -c "Ne jamais utiliser l'outil .question\|jamais.*question\|JAMAIS.*question" "$AGENTS_DIR/design/ux-designer.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "ui-designer documente l'interdiction de l'outil question en mode orchestrateur" {
  run grep -c "Ne jamais utiliser l'outil .question\|jamais.*question\|JAMAIS.*question" "$AGENTS_DIR/design/ui-designer.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Marqueur [CONTEXTE] injecté par orchestrator-protocol ────────────────────

@test "orchestrator-protocol injecte [CONTEXTE] dans l'invocation du planner" {
  run grep -A5 "Invoquer.*planner\|planner.*Invoquer" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "CONTEXTE"
}

@test "orchestrator-protocol injecte [CONTEXTE] dans l'invocation de orchestrator-dev" {
  run grep -c "\[CONTEXTE\].*orchestrateur feature" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 3 ]
}

@test "orchestrator-protocol injecte [CONTEXTE] dans l'invocation du debugger" {
  run grep -B2 -A5 "CONTEXTE.*debugger\|debugger.*CONTEXTE\|Invoquer.*debugger" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "CONTEXTE"
}

# ── Présence des sections de réception dans orchestrator-protocol ─────────────

@test "orchestrator-protocol a une section question montante depuis le planner" {
  run grep -c "question montante depuis le planner\|question montante.*planner" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-protocol a une section question montante depuis orchestrator-dev" {
  run grep -c "question montante depuis orchestrator-dev\|question montante.*orchestrator-dev" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-protocol a une section réception depuis le debugger" {
  run grep -c "question montante depuis le debugger\|réception.*debugger" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "orchestrator-protocol a une section réception depuis l'onboarder" {
  run grep -c "question montante depuis l'onboarder\|réception.*onboarder" "$SKILLS_DIR/orchestrator/orchestrator-protocol.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

# ── Cohérence : tous les agents avec CONTEXTE ont aussi ## Retour vers orchestrator

@test "onboarder-workflow contient le bloc Retour vers orchestrator final" {
  run grep -c "## Retour vers orchestrator" "$SKILLS_DIR/planning/onboarder-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "auditor-workflow contient le bloc Retour vers orchestrator final" {
  run grep -c "## Retour vers orchestrator" "$SKILLS_DIR/auditor/auditor-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "debugger-workflow contient le bloc Retour vers orchestrator final" {
  run grep -c "## Retour vers orchestrator" "$SKILLS_DIR/quality/debugger-workflow.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}

@test "pathfinder-handoff-format contient le bloc Retour vers orchestrator" {
  run grep -c "## Retour vers orchestrator" "$SKILLS_DIR/planning/pathfinder-handoff-format.md"
  [ "$status" -eq 0 ]
  [ "$output" -ge 1 ]
}
