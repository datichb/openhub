#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/project.sh
# Fonctions testées : project_exists, normalize_project_id, get_project_path, 
#                     get_project_tracker, get_project_labels, get_project_agents

load helpers

setup() {
  common_setup
  
  # Sourcer project.sh (nécessite log functions)
  source "$BATS_TEST_DIRNAME/../scripts/lib/project.sh"
  
  # Créer un registre de projets de test
  cat > "$PROJECTS_FILE" <<'EOF'
# Registre de projets

## PROJ-A
- Nom : Projet Alpha
- Stack : TypeScript
- Tracker : jira
- Labels : backend, api, production
- Agents : code-review, security

## PROJ-B
- Nom : Projet Beta
- Stack : Python
- Board Beads : PROJ-B

## proj-lower
- Nom : Projet Lowercase
- Stack : Rust
EOF

  # Créer paths.local.md
  cat > "$PATHS_FILE" <<EOF
PROJ-A=$TEST_DIR/project-a
PROJ-B=$TEST_DIR/project-b
proj-lower=$TEST_DIR/project-lower
EOF

  # Créer les répertoires de projets
  mkdir -p "$TEST_DIR/project-a"
  mkdir -p "$TEST_DIR/project-b"
  mkdir -p "$TEST_DIR/project-lower"
}

# ── project_exists ─────────────────────────────────────────────────────────────

@test "project_exists : retourne 0 si projet existe" {
  run project_exists "PROJ-A"
  [ "$status" -eq 0 ]
}

@test "project_exists : retourne 1 si projet n'existe pas" {
  run project_exists "INEXISTANT"
  [ "$status" -eq 1 ]
}

@test "project_exists : sensible à la casse (proj-lower existe)" {
  run project_exists "proj-lower"
  [ "$status" -eq 0 ]
}

@test "project_exists : gère les IDs avec tirets" {
  run project_exists "PROJ-A"
  [ "$status" -eq 0 ]
}

# ── normalize_project_id ───────────────────────────────────────────────────────

@test "normalize_project_id : convertit en majuscules" {
  run normalize_project_id "proj-lower"
  [ "$status" -eq 0 ]
  [ "$output" = "PROJ-LOWER" ]
}

@test "normalize_project_id : préserve les majuscules" {
  run normalize_project_id "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "PROJ-A" ]
}

@test "normalize_project_id : gère les tirets" {
  run normalize_project_id "my-long-project-id"
  [ "$status" -eq 0 ]
  [ "$output" = "MY-LONG-PROJECT-ID" ]
}

@test "normalize_project_id : gère les underscores" {
  run normalize_project_id "proj_test"
  [ "$status" -eq 0 ]
  [ "$output" = "PROJ_TEST" ]
}

# ── get_project_path ───────────────────────────────────────────────────────────

@test "get_project_path : retourne le chemin depuis paths.local.md" {
  run get_project_path "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "$TEST_DIR/project-a" ]
}

@test "get_project_path : sensible à la casse (doit matcher exactement)" {
  run get_project_path "proj-a"
  [ "$status" -eq 0 ]
  [ -z "$output" ]  # Pas de match car les clés sont sensibles à la casse
}

@test "get_project_path : retourne vide si projet n'a pas de chemin" {
  echo -e "\n## PROJ-NO-PATH\n- Nom : Sans Chemin" >> "$PROJECTS_FILE"
  
  run get_project_path "PROJ-NO-PATH"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── get_project_tracker ────────────────────────────────────────────────────────

@test "get_project_tracker : retourne la valeur du champ Tracker" {
  run get_project_tracker "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "jira" ]
}

@test "get_project_tracker : retourne 'none' si champ Tracker absent" {
  run get_project_tracker "PROJ-B"
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

# ── get_project_labels ─────────────────────────────────────────────────────────

@test "get_project_labels : retourne la liste de labels" {
  run get_project_labels "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "backend, api, production" ]
}

@test "get_project_labels : retourne vide si pas de labels" {
  run get_project_labels "PROJ-B"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "get_project_labels : gère les espaces dans les labels" {
  echo -e "\n## PROJ-SPACES\n- Labels : front end, back end" >> "$PROJECTS_FILE"
  
  run get_project_labels "PROJ-SPACES"
  [ "$status" -eq 0 ]
  [ "$output" = "front end, back end" ]
}

# ── get_project_agents ─────────────────────────────────────────────────────────

@test "get_project_agents : retourne la liste d'agents" {
  run get_project_agents "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "code-review, security" ]
}

@test "get_project_agents : retourne 'all' si pas d'agents" {
  run get_project_agents "PROJ-B"
  [ "$status" -eq 0 ]
  [ "$output" = "all" ]
}

# ── path_exists ────────────────────────────────────────────────────────────────

@test "path_exists : retourne 0 si chemin existe dans paths.local.md" {
  run path_exists "PROJ-A"
  [ "$status" -eq 0 ]
}

@test "path_exists : retourne 1 si chemin n'existe pas" {
  echo -e "\n## PROJ-NO-PATH\n- Nom : Sans Chemin" >> "$PROJECTS_FILE"
  
  run path_exists "PROJ-NO-PATH"
  [ "$status" -eq 1 ]
}

@test "path_exists : sensible à la casse" {
  run path_exists "proj-a"
  [ "$status" -eq 1 ]  # proj-a ne matche pas PROJ-A
}

# ── get_project_language ───────────────────────────────────────────────────────

@test "get_project_language : retourne le champ Langue du projet" {
  # get_project_language lit le champ "Langue", pas "Stack".
  # Ajouter un projet avec le champ Langue explicite.
  echo -e "\n## PROJ-LANG\n- Nom : Projet Langue\n- Stack : Go\n- Langue : english" >> "$PROJECTS_FILE"
  
  run get_project_language "PROJ-LANG"
  [ "$status" -eq 0 ]
  [ "$output" = "english" ]
}

@test "get_project_language : retourne vide si pas de Stack" {
  echo -e "\n## PROJ-NO-STACK\n- Nom : Sans Stack" >> "$PROJECTS_FILE"
  
  run get_project_language "PROJ-NO-STACK"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── Tests d'intégration ────────────────────────────────────────────────────────

@test "Intégration : lecture de plusieurs champs pour un projet" {
  # Lire plusieurs champs du même projet
  tracker=$(get_project_tracker "PROJ-A")
  labels=$(get_project_labels "PROJ-A")
  agents=$(get_project_agents "PROJ-A")
  path=$(get_project_path "PROJ-A")
  
  [ "$tracker" = "jira" ]
  [ "$labels" = "backend, api, production" ]
  [ "$agents" = "code-review, security" ]
  [ "$path" = "$TEST_DIR/project-a" ]
}

@test "Intégration : gestion projet sans chemin" {
  echo -e "\n## PROJ-VIRTUAL\n- Nom : Projet Virtuel\n- Stack : Go" >> "$PROJECTS_FILE"
  
  # Le projet existe
  run project_exists "PROJ-VIRTUAL"
  [ "$status" -eq 0 ]
  
  # Mais il n'a pas de chemin
  run path_exists "PROJ-VIRTUAL"
  [ "$status" -eq 1 ]
  
  # On peut quand même lire ses métadonnées
  # (get_project_language lit "Langue", pas "Stack" — champ Langue absent → vide)
  run get_project_language "PROJ-VIRTUAL"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ── get_project_worktree_enabled ───────────────────────────────────────────────

@test "get_project_worktree_enabled : retourne enabled si configuré" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-WT
- Nom : Projet Worktree
- Stack : TypeScript
- Worktree : enabled
EOF

  run get_project_worktree_enabled "PROJ-WT"
  [ "$status" -eq 0 ]
  [ "$output" = "enabled" ]
}

@test "get_project_worktree_enabled : retourne disabled si champ absent" {
  run get_project_worktree_enabled "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "disabled" ]
}

@test "get_project_worktree_enabled : insensible à la casse" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-WT-CASE
- Nom : Test casse
- Stack : TypeScript
- Worktree : Enabled
EOF

  run get_project_worktree_enabled "PROJ-WT-CASE"
  [ "$status" -eq 0 ]
  [ "$output" = "enabled" ]
}

# ── get_project_worktree_auto_cleanup ──────────────────────────────────────────

@test "get_project_worktree_auto_cleanup : retourne true si configuré" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-WTA
- Nom : Projet Worktree AutoCleanup
- Stack : TypeScript
- Worktree auto cleanup : true
EOF

  run get_project_worktree_auto_cleanup "PROJ-WTA"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "get_project_worktree_auto_cleanup : retourne false si champ absent" {
  run get_project_worktree_auto_cleanup "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

@test "get_project_worktree_auto_cleanup : retourne false si configuré false" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-WTF
- Nom : Projet Worktree False
- Stack : TypeScript
- Worktree auto cleanup : false
EOF

  run get_project_worktree_auto_cleanup "PROJ-WTF"
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

# ── get_project_worktree_base_branch ──────────────────────────────────────────

@test "get_project_worktree_base_branch : retourne main par défaut" {
  run get_project_worktree_base_branch "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "main" ]
}

@test "get_project_worktree_base_branch : retourne valeur configurée" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-WTBB
- Nom : Projet Base Branch
- Stack : TypeScript
- Worktree base branch : develop
EOF

  run get_project_worktree_base_branch "PROJ-WTBB"
  [ "$status" -eq 0 ]
  [ "$output" = "develop" ]
}

# ── get_hub_disabled_native_agents ────────────────────────────────────────────

@test "get_hub_disabled_native_agents : retourne CSV pour un tableau non-vide" {
  command -v jq >/dev/null 2>&1 || skip "jq requis pour ce test"

  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan", "general", "explore", "scout"]
  }
}
EOF

  run get_hub_disabled_native_agents
  [ "$status" -eq 0 ]
  [ "$output" = "build,plan,general,explore,scout" ]
}

@test "get_hub_disabled_native_agents : retourne vide pour un tableau vide []" {
  command -v jq >/dev/null 2>&1 || skip "jq requis pour ce test"

  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": []
  }
}
EOF

  run get_hub_disabled_native_agents
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "get_hub_disabled_native_agents : retourne vide si hub.json absent" {
  rm -f "$HUB_CONFIG"

  run get_hub_disabled_native_agents
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "get_hub_disabled_native_agents : fallback bash sans jq retourne CSV correct" {
  # Ce test vérifie le fallback bash en mockant jq comme absent
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan", "general"]
  }
}
EOF

  # Mocker la commande pour simuler l'absence de jq
  command() {
    if [ "$1" = "-v" ] && [ "$2" = "jq" ]; then
      return 1
    fi
    builtin command "$@"
  }
  export -f command

  run get_hub_disabled_native_agents
  [ "$status" -eq 0 ]
  [ "$output" = "build,plan,general" ]

  unset -f command
}

@test "get_hub_disabled_native_agents : fallback bash retourne vide pour tableau vide" {
  cat > "$HUB_CONFIG" <<'EOF'
{
  "version": "test",
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": []
  }
}
EOF

  command() {
    if [ "$1" = "-v" ] && [ "$2" = "jq" ]; then
      return 1
    fi
    builtin command "$@"
  }
  export -f command

  run get_hub_disabled_native_agents
  [ "$status" -eq 0 ]
  [ "$output" = "" ]

  unset -f command
}

# ── get_project_mcp / _set_project_mcp / should_deploy_mcp ────────────────────

@test "get_project_mcp : retourne CSV quand le champ existe" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MCP
- Nom : Projet MCP
- Stack : TypeScript
- Agents : all
- MCP : figma-mcp,gitlab-mcp
EOF

  run get_project_mcp "PROJ-MCP"
  [ "$status" -eq 0 ]
  [ "$output" = "figma-mcp,gitlab-mcp" ]
}

@test "get_project_mcp : retourne none quand le champ est absent" {
  run get_project_mcp "PROJ-A"
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

@test "get_project_mcp : retourne all quand MCP : all" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MCP-ALL
- Nom : Projet MCP All
- MCP : all
EOF

  run get_project_mcp "PROJ-MCP-ALL"
  [ "$status" -eq 0 ]
  [ "$output" = "all" ]
}

@test "get_project_mcp : retourne none quand MCP : none" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-MCP-NONE
- Nom : Projet MCP None
- MCP : none
EOF

  run get_project_mcp "PROJ-MCP-NONE"
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

@test "_set_project_mcp : écrit le champ dans projects.md" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SET-MCP
- Nom : Projet Set MCP
- Agents : all
EOF

  _set_project_mcp "PROJ-SET-MCP" "figma-mcp"

  run get_project_mcp "PROJ-SET-MCP"
  [ "$output" = "figma-mcp" ]
}

@test "_set_project_mcp : met à jour un champ existant" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-UPDATE-MCP
- Nom : Projet Update MCP
- Agents : all
- MCP : figma-mcp
EOF

  _set_project_mcp "PROJ-UPDATE-MCP" "figma-mcp,gitlab-mcp"

  run get_project_mcp "PROJ-UPDATE-MCP"
  [ "$output" = "figma-mcp,gitlab-mcp" ]
}

@test "should_deploy_mcp : retourne 0 (vrai) pour all" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SDMCP-ALL
- MCP : all
EOF

  run should_deploy_mcp "PROJ-SDMCP-ALL" "figma-mcp"
  [ "$status" -eq 0 ]
}

@test "should_deploy_mcp : retourne 1 (faux) pour none" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SDMCP-NONE
- MCP : none
EOF

  run should_deploy_mcp "PROJ-SDMCP-NONE" "figma-mcp"
  [ "$status" -eq 1 ]
}

@test "should_deploy_mcp : retourne 0 si serveur dans la liste CSV" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SDMCP-CSV
- MCP : figma-mcp
EOF

  run should_deploy_mcp "PROJ-SDMCP-CSV" "figma-mcp"
  [ "$status" -eq 0 ]
}

@test "should_deploy_mcp : retourne 1 si serveur absent de la liste CSV" {
  cat >> "$PROJECTS_FILE" <<'EOF'

## PROJ-SDMCP-CSV2
- MCP : figma-mcp
EOF

  run should_deploy_mcp "PROJ-SDMCP-CSV2" "gitlab-mcp"
  [ "$status" -eq 1 ]
}

@test "should_deploy_mcp : retourne 1 si champ absent (défaut = none)" {
  run should_deploy_mcp "PROJ-A" "figma-mcp"
  [ "$status" -eq 1 ]
}
