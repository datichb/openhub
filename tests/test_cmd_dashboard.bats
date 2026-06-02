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

  export HUB_DIR="$FAKE_HUB"
}

# Helper pour exécuter le script dashboard depuis FAKE_HUB
# Évite les problèmes de quoting avec `run bash -c "..."` (BW01)
_run_dashboard() {
  cd "$FAKE_HUB" && bash "$HUB_ROOT/scripts/cmd-dashboard.sh"
}

teardown() {
  rm -rf "$FAKE_HUB"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Dashboard sans session active
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : affiche 'Aucune session active' si pas de fichier session" {
  rm -f "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aucune session active"* ]]
}

@test "dashboard : affiche 'Aucune session active' si fichier session vide" {
  echo "" > "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aucune session active"* ]]
}

@test "dashboard : suggère commande 'oc start' quand pas de session" {
  rm -f "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"oc start"* ]]
}

@test "dashboard : affiche le header 'OpenCode Dashboard' en mode idle" {
  rm -f "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"OpenCode Dashboard"* ]]
}

@test "dashboard : validation bordures TUI (╭ ╮ ╰ ╯) en mode idle" {
  rm -f "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  [[ "$output" == *"╭"* ]]
  [[ "$output" == *"╮"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Dashboard avec session active
# ══════════════════════════════════════════════════════════════════════════════

@test "dashboard : affiche 'Session Active' si session-state.json valide" {
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
  [[ "$output" == *"Session Active"* ]]
}

@test "dashboard : affiche les tickets avec emojis de statut" {
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
  # Vérifier que les emojis sont présents (⏳ 🔄 ✅ 🚫)
  [[ "$output" == *"⏳"* ]] || [[ "$output" == *"pending"* ]]
  [[ "$output" == *"🔄"* ]] || [[ "$output" == *"in_progress"* ]]
  [[ "$output" == *"✅"* ]] || [[ "$output" == *"completed"* ]]
  [[ "$output" == *"🚫"* ]] || [[ "$output" == *"blocked"* ]]
}

@test "dashboard : met en évidence le ticket courant avec flèche ◀" {
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
  [[ "$output" == *"PROJ-123"*"◀"* ]]
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

@test "dashboard : affiche 'Aucun ticket' si tableau tickets vide" {
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
  [[ "$output" == *"Aucun ticket"* ]]
}

@test "dashboard : affiche mode session (manuel, semi-auto, auto)" {
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
  [[ "$output" == *"Semi-auto"* ]] || [[ "$output" == *"semi-auto"* ]]
}

@test "dashboard : test avec plusieurs tickets (3+)" {
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
  [[ "$output" == *"PROJ-123"* ]]
  [[ "$output" == *"PROJ-124"* ]]
  [[ "$output" == *"PROJ-125"* ]]
  [[ "$output" == *"PROJ-126"* ]]
  [[ "$output" == *"PROJ-127"* ]]
}

@test "dashboard : test labels d'action (implementing, testing, reviewing, waiting_cp2, idle)" {
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
  [[ "$output" == *"Review"* ]] || [[ "$output" == *"reviewing"* ]]
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
  
  # Le script doit détecter l'absence de jq
  [ "$status" -ne 0 ] || [[ "$output" == *"jq"* ]]
}

@test "dashboard : gestion JSON corrompu (fallback idle dashboard)" {
  # Créer un JSON invalide
  echo "{ invalid json" > "$SESSION_STATE_FILE"
  
  run _run_dashboard
  [ "$status" -eq 0 ]
  # Doit fallback sur idle dashboard
  [[ "$output" == *"Aucune session active"* ]] || [[ "$output" == *"corrompu"* ]]
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
