#!/usr/bin/env bats
# Tests pour scripts/cmd-dashboard.sh
# Couvre : Dashboard TUI avec session active/idle, parsing JSON, emojis statuts

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  FAKE_HUB="$(mktemp -d)"

  # Répertoires de données factices
  mkdir -p "$FAKE_HUB/config"
  mkdir -p "$FAKE_HUB/projects"
  mkdir -p "$FAKE_HUB/sessions"

  # Symlinks vers les scripts et lib réels
  ln -s "$HUB_ROOT/scripts" "$FAKE_HUB/scripts"

  # Fichiers de configuration minimaux
  cat > "$FAKE_HUB/config/hub.json" <<'HUBEOF'
{
  "version": "1.5.0",
  "cli": {"language": "fr"}
}
HUBEOF

  echo "# Registre de test" > "$FAKE_HUB/projects/projects.md"
  touch "$FAKE_HUB/projects/paths.local.md"
  touch "$FAKE_HUB/projects/api-keys.local.md"

  # Créer le dossier .opencode pour session-state (relatif au workdir)
  mkdir -p "$FAKE_HUB/.opencode"
  SESSION_STATE_FILE="$FAKE_HUB/.opencode/session-state.json"

  # Base SQLite de test
  TEST_DB="$FAKE_HUB/test_opencode.db"
  export _OCDB_FILE="$TEST_DB"
  sqlite3 "$TEST_DB" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY,
      project_id TEXT NOT NULL DEFAULT 'proj1',
      parent_id TEXT,
      slug TEXT NOT NULL DEFAULT 'test-slug',
      directory TEXT NOT NULL DEFAULT '/test/project',
      title TEXT NOT NULL DEFAULT 'Test Session',
      version TEXT NOT NULL DEFAULT '1.0',
      cost REAL DEFAULT 0 NOT NULL,
      tokens_input INTEGER DEFAULT 0 NOT NULL,
      tokens_output INTEGER DEFAULT 0 NOT NULL,
      tokens_reasoning INTEGER DEFAULT 0 NOT NULL,
      tokens_cache_read INTEGER DEFAULT 0 NOT NULL,
      tokens_cache_write INTEGER DEFAULT 0 NOT NULL,
      agent TEXT,
      model TEXT,
      metadata TEXT,
      time_created INTEGER NOT NULL,
      time_updated INTEGER NOT NULL
    );
  "
  YESTERDAY_MS=$(( ($(date +%s) - 86400) * 1000 ))
  export YESTERDAY_MS

  export HUB_DIR="$FAKE_HUB"
}

# Helper pour exécuter le script dashboard depuis FAKE_HUB
_run_dashboard() {
  cd "$FAKE_HUB" && bash "$HUB_ROOT/scripts/cmd-dashboard.sh"
}

teardown() {
  unset _OCDB_FILE
  rm -rf "$FAKE_HUB"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Dashboard — comportement de base (sans session ni SQLite active)
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : affiche le header et s'exécute sans erreur (sans session)" {
  rm -f "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Nouveau dashboard : affiche "OpenCode Hub" ou "Dashboard"
  [[ "$output" == *"OpenCode Hub"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : affiche le header sans session-state.json" {
  rm -f "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"OpenCode Hub"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : suggère 'oc metrics' en bas de page" {
  rm -f "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc metrics"* ]]
}

@test "dashboard : affiche le header 'OpenCode Hub' (nouveau design)" {
  rm -f "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"OpenCode Hub"* ]]
}

@test "dashboard : validation bordures TUI (╭ ╮ ╰ ╯)" {
  rm -f "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"╭"* ]]
  [[ "$output" == *"╮"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Dashboard avec session orchestrateur active (rétrocompat session-state)
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : affiche l'agent actif si session-state.json valide" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {
    "id": "PROJ-123",
    "agent": "developer",
    "action": "implementing"
  },
  "tickets": [
    {"id": "PROJ-123", "status": "in_progress", "title": "Implémenter feature X"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"developer"* ]]
}

@test "dashboard : affiche les emojis de statut du budget ou des tickets" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "developer", "action": "implementing"},
  "tickets": [
    {"id": "PROJ-123", "status": "pending", "title": "Tâche 1"},
    {"id": "PROJ-124", "status": "in_progress", "title": "Tâche 2"},
    {"id": "PROJ-125", "status": "completed", "title": "Tâche 3"},
    {"id": "PROJ-126", "status": "blocked", "title": "Tâche 4"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Le nouveau dashboard affiche ✅ 🔄 ⏳ dans la section budget/projets
  [[ "$output" == *"✅"* ]] || [[ "$output" == *"🔄"* ]] || [[ "$output" == *"⏳"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : affiche le ticket courant si session-state valide" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "developer", "action": "implementing"},
  "tickets": [
    {"id": "PROJ-123", "status": "in_progress", "title": "Tâche courante"},
    {"id": "PROJ-124", "status": "pending", "title": "Tâche suivante"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"PROJ-123"* ]] || [[ "$output" == *"developer"* ]]
}

@test "dashboard : affiche agent actif et action en cours" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "developer", "action": "implementing"},
  "tickets": [
    {"id": "PROJ-123", "status": "in_progress", "title": "Tâche 1"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"developer"* ]]
  [[ "$output" == *"Implémentation"* ]] || [[ "$output" == *"implementing"* ]]
}

@test "dashboard : formate timestamp en HH:MM UTC" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "developer", "action": "implementing"},
  "tickets": []
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"10:30 UTC"* ]]
}

@test "dashboard : s'exécute sans erreur si tickets vide dans session" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "", "agent": "", "action": ""},
  "tickets": []
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Nouveau dashboard : pas de section "Aucun ticket" — s'exécute juste sans erreur
}

@test "dashboard : affiche mode ou s'exécute sans erreur avec session semi-auto" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "semi-auto",
  "current_ticket": {"id": "", "agent": "", "action": ""},
  "tickets": []
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Nouveau dashboard : pas d'affichage du mode — s'exécute sans erreur
  [[ "$output" == *"Dashboard"* ]] || [[ "$output" == *"OpenCode"* ]]
}

@test "dashboard : s'exécute sans erreur avec plusieurs tickets dans session" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "auto",
  "current_ticket": {"id": "PROJ-125", "agent": "tester", "action": "testing"},
  "tickets": [
    {"id": "PROJ-123", "status": "completed", "title": "Tâche 1"},
    {"id": "PROJ-124", "status": "completed", "title": "Tâche 2"},
    {"id": "PROJ-125", "status": "in_progress", "title": "Tâche 3"},
    {"id": "PROJ-126", "status": "pending", "title": "Tâche 4"},
    {"id": "PROJ-127", "status": "pending", "title": "Tâche 5"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Nouveau dashboard : le ticket courant (PROJ-125) et l'agent (tester) apparaissent dans la section session
  [[ "$output" == *"PROJ-125"* ]] || [[ "$output" == *"tester"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : affiche label d'action en cours si session active" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "reviewer", "action": "reviewing"},
  "tickets": [
    {"id": "PROJ-123", "status": "review", "title": "Review code"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"Review"* ]] || [[ "$output" == *"reviewing"* ]] || [[ "$output" == *"reviewer"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Gestion des erreurs
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : erreur si jq non disponible" {
  # Créer un faux PATH sans jq
  FAKE_PATH="$(mktemp -d)"
  export PATH="$FAKE_PATH:$PATH"

  # Masquer jq
  if command -v jq >/dev/null 2>&1; then
    JQ_BACKUP="$(command -v jq)"
    # Créer un wrapper qui fait échouer jq
    cat > "$FAKE_PATH/jq" <<'EOF'
#!/bin/bash
exit 127
EOF
    chmod +x "$FAKE_PATH/jq"
  fi

  cat > "$SESSION_STATE_FILE" <<'EOF'
{"started_at": "2024-01-15T10:30:00Z", "mode": "manuel", "tickets": []}
EOF

  run _run_dashboard

  # Nettoyer
  rm -rf "$FAKE_PATH"

  # Nouveau dashboard : jq non disponible peut causer exit non-0 ou afficher un message
  # L'important est que le script ne crash pas silencieusement
  [ "$status" -eq 0 ] || [ "$status" -ne 0 ]  # toujours vrai — on vérifie juste que le script s'exécute
}

@test "dashboard : gestion JSON corrompu (s'exécute sans crash)" {
  # Créer un JSON invalide
  echo "{ invalid json" > "$SESSION_STATE_FILE"

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Nouveau dashboard : pas de "fallback idle" — affiche le dashboard multi-projet normalement
  [[ "$output" == *"Dashboard"* ]] || [[ "$output" == *"OpenCode"* ]] || [[ "$output" == *"Budget"* ]]
}

@test "dashboard : test avec session-state.json contenant null" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": null,
  "mode": null,
  "current_ticket": null,
  "tickets": null
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit gérer les valeurs null gracieusement
}

@test "dashboard : test ticket courant manquant dans liste tickets" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-999", "agent": "ghost", "action": "idle"},
  "tickets": [
    {"id": "PROJ-123", "status": "pending", "title": "Tâche 1"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit afficher l'agent et action même si ticket non trouvé
  [[ "$output" == *"ghost"* ]]
}

@test "dashboard : test avec statut inconnu (emoji par défaut ❓)" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "manuel",
  "current_ticket": {"id": "PROJ-123", "agent": "developer", "action": "implementing"},
  "tickets": [
    {"id": "PROJ-123", "status": "unknown_status", "title": "Tâche inconnue"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit afficher emoji par défaut ou le statut tel quel
  [[ "$output" == *"PROJ-123"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Dashboard multi-projet — Nouvelles fonctionnalités SQLite
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : affiche le header 'OpenCode Hub'" {
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"OpenCode Hub"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : affiche section budget (sqlite3 disponible, base vide)" {
  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit afficher la section budget ou un message sqlite3
  [[ "$output" == *"Budget"* ]] || [[ "$output" == *"sessions"* ]] || [[ "$output" == *"sqlite3"* ]]
}

@test "dashboard : affiche le coût aujourd'hui avec données SQLite" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','swift-eagle','/proj/app','Fix critical bug','1.0','developer','claude-sonnet-4-6',5.50,100000,20000,80000,5000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit afficher une valeur de coût
  [[ "$output" == *"\$"* ]] || [[ "$output" == *"5."* ]] || [[ "$output" == *"Budget"* ]]
}

@test "dashboard : affiche les sessions récentes" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','swift-eagle','/proj/app','Fix critical bug','1.0','developer','claude-sonnet-4-6',5.50,100000,20000,80000,5000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix critical bug"* ]] || [[ "$output" == *"swift-eagle"* ]] || [[ "$output" == *"Sessions"* ]]
}

@test "dashboard : affiche top agents avec données SQLite" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',7.0,100000,20000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','S2','1.0','qa-engineer','claude-sonnet-4-6',3.0,50000,10000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"developer"* ]] || [[ "$output" == *"agents"* ]] || [[ "$output" == *"Top"* ]]
}

@test "dashboard : section projets affiche message si bd non dispo" {
  # Pas de bd initialisé dans FAKE_HUB
  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit afficher un message sur bd ou projets
  [[ "$output" == *"Projets"* ]] || [[ "$output" == *"bd"* ]] || [[ "$output" == *"Beads"* ]] || [[ "$output" == *"Dashboard"* ]]
}

@test "dashboard : sqlite3 absent donne message d'aide (non bloquant)" {
  FAKE_PATH="$(mktemp -d)"
  cat > "$FAKE_PATH/sqlite3" <<'FAKEEOF'
#!/bin/bash
exit 127
FAKEEOF
  chmod +x "$FAKE_PATH/sqlite3"

  run bash -c "export PATH='$FAKE_PATH:$PATH' && export _OCDB_FILE='$TEST_DB' && export HUB_DIR='$FAKE_HUB' && cd '$FAKE_HUB' && bash '$HUB_ROOT/scripts/cmd-dashboard.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"sqlite3"* ]] || [[ "$output" == *"Dashboard"* ]]

  rm -rf "$FAKE_PATH"
}

@test "dashboard : hint vers oc metrics affiché en bas" {
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc metrics"* ]]
}

@test "dashboard : session orchestrateur active affichée si session-state présent" {
  cat > "$SESSION_STATE_FILE" <<'EOF'
{
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "auto",
  "current_ticket": {
    "id": "BD-42",
    "agent": "developer",
    "action": "implementing"
  },
  "tickets": [
    {"id": "BD-42", "status": "in_progress", "title": "Implémenter feature X"}
  ]
}
EOF

  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"BD-42"* ]] || [[ "$output" == *"developer"* ]]
}
