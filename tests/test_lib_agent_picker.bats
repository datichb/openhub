#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/agent-picker.sh
# Fonctions testées : _list_all_agents_grouped, _set_project_agents
# Note : _pick_agents et _render_agents_page nécessitent TUI interactif (non testés)

load helpers

setup() {
  common_setup
  
  # Sourcer common.sh pour avoir les variables
  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  source "$SCRIPT_DIR/common.sh"
  
  # Sourcer agent-picker.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/agent-picker.sh"
  
  # Utiliser les fixtures agents
  export CANONICAL_AGENTS_DIR="$BATS_TEST_DIRNAME/fixtures/agents"
  
  # Créer un projects.md de test
  cat > "$PROJECTS_FILE" <<'EOF'
# Projets

## TEST-PROJECT
- Nom : Test Project
- Stack : TypeScript
- Labels : backend, api

## ANOTHER-PROJECT
- Nom : Another Project  
- Stack : Python
- Labels : data, ml
- Agents : orchestrator,developer-backend
EOF
}

teardown() {
  common_teardown
}

# ── _list_all_agents_grouped ───────────────────────────────────────────────────

@test "_list_all_agents_grouped : liste tous les agents" {
  _list_all_agents_grouped
  
  # Vérifier qu'on a trouvé des agents
  [ "${#_pick_items[@]}" -gt 0 ]
}

@test "_list_all_agents_grouped : remplit _pick_items" {
  _list_all_agents_grouped
  
  # Vérifier que _pick_items contient les IDs des agents
  local found_orchestrator=0
  local found_developer_backend=0
  
  for item in "${_pick_items[@]}"; do
    [ "$item" = "orchestrator" ] && found_orchestrator=1
    [ "$item" = "developer-backend" ] && found_developer_backend=1
  done
  
  [ "$found_orchestrator" = "1" ]
  [ "$found_developer_backend" = "1" ]
}

@test "_list_all_agents_grouped : remplit _pick_families en parallèle" {
  _list_all_agents_grouped
  
  # Les tableaux doivent avoir la même taille
  [ "${#_pick_items[@]}" -eq "${#_pick_families[@]}" ]
}

@test "_list_all_agents_grouped : extrait les familles correctement" {
  _list_all_agents_grouped
  
  # Trouver l'index de orchestrator
  local idx=0
  for i in "${!_pick_items[@]}"; do
    if [ "${_pick_items[$i]}" = "orchestrator" ]; then
      idx=$i
      break
    fi
  done
  
  # Vérifier que la famille est planning
  [ "${_pick_families[$idx]}" = "planning" ]
}

@test "_list_all_agents_grouped : extrait les descriptions" {
  _list_all_agents_grouped
  
  # Les descriptions ne doivent pas être vides
  [ "${#_pick_descriptions[@]}" -eq "${#_pick_items[@]}" ]
  
  # Vérifier qu'au moins une description n'est pas vide
  local has_desc=0
  for desc in "${_pick_descriptions[@]}"; do
    [ -n "$desc" ] && has_desc=1 && break
  done
  [ "$has_desc" = "1" ]
}

@test "_list_all_agents_grouped : extrait les modes" {
  _list_all_agents_grouped
  
  # Trouver qa-engineer qui est en mode subagent
  local found_subagent=0
  for i in "${!_pick_items[@]}"; do
    if [ "${_pick_items[$i]}" = "qa-engineer" ]; then
      [ "${_pick_modes[$i]}" = "subagent" ] && found_subagent=1
      break
    fi
  done
  
  [ "$found_subagent" = "1" ]
}

@test "_list_all_agents_grouped : mode par défaut est primary" {
  _list_all_agents_grouped
  
  # orchestrator n'a pas de mode explicite, devrait être primary
  local found=0
  for i in "${!_pick_items[@]}"; do
    if [ "${_pick_items[$i]}" = "orchestrator" ]; then
      [ "${_pick_modes[$i]}" = "primary" ] && found=1
      break
    fi
  done
  
  [ "$found" = "1" ]
}

@test "_list_all_agents_grouped : trie par ordre alphabétique au sein de chaque famille" {
  _list_all_agents_grouped
  
  # Vérifier qu'il n'y a pas de doublons (propriété minimale attendue d'un tri)
  local count total
  total="${#_pick_items[@]}"
  count=$(printf '%s\n' "${_pick_items[@]}" | sort -u | wc -l | tr -d ' ')
  [ "$count" -eq "$total" ]
}

@test "_list_all_agents_grouped : groupe par famille" {
  _list_all_agents_grouped
  
  # Vérifier qu'on a plusieurs familles
  local unique_families=$(printf '%s\n' "${_pick_families[@]}" | sort -u | wc -l)
  [ "$unique_families" -ge 3 ]  # Au moins planning, dev, ops
}

@test "_list_all_agents_grouped : ignore agents sans id" {
  # Créer un agent sans id
  mkdir -p "$TEST_DIR/agents/test"
  cat > "$TEST_DIR/agents/test/no-id.md" <<'EOF'
---
label: No ID Agent
---
# Agent sans ID
EOF
  
  export CANONICAL_AGENTS_DIR="$TEST_DIR/agents"
  _list_all_agents_grouped
  
  # Ne devrait pas contenir d'entrée vide
  for item in "${_pick_items[@]}"; do
    [ -n "$item" ]
  done
}

# ── _set_project_agents ────────────────────────────────────────────────────────

@test "_set_project_agents : ajoute champ Agents si absent" {
  _set_project_agents "TEST-PROJECT" "orchestrator,developer-backend"
  
  # Vérifier que le champ a été ajouté
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : orchestrator,developer-backend"* ]]
}

@test "_set_project_agents : met à jour champ Agents existant" {
  _set_project_agents "ANOTHER-PROJECT" "devops,qa-engineer"
  
  # Vérifier que le champ a été mis à jour
  run grep "ANOTHER-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : devops,qa-engineer"* ]]
  [[ "$output" != *"orchestrator"* ]]
}

@test "_set_project_agents : préserve autres champs" {
  _set_project_agents "TEST-PROJECT" "orchestrator"
  
  # Vérifier que les autres champs sont toujours là
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Nom : Test Project"* ]]
  [[ "$output" == *"- Stack : TypeScript"* ]]
  [[ "$output" == *"- Labels : backend, api"* ]]
}

@test "_set_project_agents : gère liste vide" {
  _set_project_agents "TEST-PROJECT" ""
  
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : "* ]]
}

@test "_set_project_agents : gère agent unique" {
  _set_project_agents "TEST-PROJECT" "orchestrator"
  
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : orchestrator"* ]]
}

@test "_set_project_agents : gère multiples agents CSV" {
  _set_project_agents "TEST-PROJECT" "orchestrator,developer-backend,devops"
  
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : orchestrator,developer-backend,devops"* ]]
}

@test "_set_project_agents : gère valeur 'all'" {
  _set_project_agents "TEST-PROJECT" "all"
  
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents : all"* ]]
}

@test "_set_project_agents : échoue si projet inexistant" {
  run _set_project_agents "NONEXISTENT-PROJECT" "orchestrator"
  [ "$status" -ne 0 ]
}

@test "_set_project_agents : insère après Labels si présent" {
  _set_project_agents "TEST-PROJECT" "orchestrator"
  
  # Vérifier l'ordre : Labels puis Agents
  awk '/^## TEST-PROJECT/{found=1; next} found && /^## /{exit} found{print NR": "$0}' "$PROJECTS_FILE" > "$TEST_DIR/order.txt"
  
  local labels_line=$(grep -n "Labels" "$TEST_DIR/order.txt" | cut -d: -f1)
  local agents_line=$(grep -n "Agents" "$TEST_DIR/order.txt" | cut -d: -f1)
  
  [ -n "$labels_line" ]
  [ -n "$agents_line" ]
  [ "$agents_line" -gt "$labels_line" ]
}

# ── Intégration ────────────────────────────────────────────────────────────────

@test "Intégration : liste agents puis définit projet" {
  # Lister les agents
  _list_all_agents_grouped
  [ "${#_pick_items[@]}" -gt 0 ]
  
  # Prendre les 3 premiers agents
  local agents_csv="${_pick_items[0]},${_pick_items[1]},${_pick_items[2]}"
  
  # Les définir pour un projet
  _set_project_agents "TEST-PROJECT" "$agents_csv"
  
  # Vérifier
  run grep "TEST-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"- Agents :"* ]]
}

@test "Intégration : workflow complet modification agents" {
  # État initial
  run grep "ANOTHER-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"orchestrator,developer-backend"* ]]
  
  # Lister les agents disponibles
  _list_all_agents_grouped
  
  # Changer les agents
  _set_project_agents "ANOTHER-PROJECT" "devops,qa-engineer"
  
  # Vérifier le changement
  run grep "ANOTHER-PROJECT" -A 5 "$PROJECTS_FILE"
  [[ "$output" == *"devops,qa-engineer"* ]]
  [[ "$output" != *"orchestrator"* ]]
}
