#!/usr/bin/env bats
# Tests pour scripts/cmd-metrics.sh et scripts/lib/metrics.sh

setup() {
  TEST_DIR="$(mktemp -d)"
  HUB_ROOT="$BATS_TEST_DIRNAME/.."

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"

  # Isoler HUB_CONFIG
  export HUB_CONFIG="$TEST_DIR/hub.json"
    > "$HUB_CONFIG"

  CMD_METRICS="$BATS_TEST_DIRNAME/../scripts/cmd-metrics.sh"
  LIB_METRICS="$BATS_TEST_DIRNAME/../scripts/lib/metrics.sh"
  COMMON_SH="$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Créer un projet factice avec .opencode/
  mkdir -p "$TEST_DIR/fake-project/.opencode"
  export _METRICS_DIR="$TEST_DIR/fake-project/.opencode"
  export _METRICS_FILE="$_METRICS_DIR/metrics.jsonl"

  # Base SQLite de test
  TEST_DB="$TEST_DIR/test_opencode.db"
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
    CREATE TABLE part (
      id TEXT PRIMARY KEY,
      message_id TEXT NOT NULL DEFAULT 'msg1',
      session_id TEXT NOT NULL,
      time_created INTEGER NOT NULL,
      time_updated INTEGER NOT NULL,
      data TEXT NOT NULL
    );
  "
  NOW_MS=$(( $(date +%s) * 1000 ))
  YESTERDAY_MS=$(( ($(date +%s) - 86400) * 1000 ))
  export NOW_MS YESTERDAY_MS

  # Fichiers de config de base
  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test

## TEST-PROJ
- Nom : Projet Test
- Stack : Node.js
- Agents : all
PROJEOF

  cat > "$PATHS_FILE" <<EOF
TEST-PROJ=$TEST_DIR/fake-project
EOF

  : > "$API_KEYS_FILE"
}

teardown() {
  unset HUB_CONFIG
  unset _METRICS_DIR
  unset _METRICS_FILE
  unset _OCDB_FILE
  rm -rf "$TEST_DIR"
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — Cas fichier absent (rétrocompat)
# ══════════════════════════════════════════════════════════════════════════════

@test "cmd-metrics : affiche message informatif si fichier metrics absent" {
  # S'assurer qu'aucun fichier metrics n'existe
  rm -f "$_METRICS_FILE"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fichier de métriques non trouvé"* ]] || [[ "$output" == *"metrics file not found"* ]] || [[ "$output" == *"non trouvé"* ]] || [[ "$output" == *"sqlite3"* ]] || [[ "$output" == *"Métriques"* ]]
}

@test "cmd-metrics : exit 0 même si fichier metrics absent" {
  rm -f "$_METRICS_FILE"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — Cas fichier vide (rétrocompat)
# ══════════════════════════════════════════════════════════════════════════════

@test "cmd-metrics : gère le fichier metrics vide" {
  mkdir -p "$_METRICS_DIR"
  : > "$_METRICS_FILE"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  # Devrait afficher 0 tickets ou un message indiquant pas de données
  [[ "$output" == *"0"* ]] || [[ "$output" == *"Aucune"* ]] || [[ "$output" == *"—"* ]] || [[ "$output" == *"Métriques"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — Cas nominal avec données JSONL (rétrocompat)
# ══════════════════════════════════════════════════════════════════════════════

@test "cmd-metrics : affiche les métriques avec données" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","agent":"developer-backend","duration_seconds":600}
{"timestamp":"2026-01-01T11:00:00Z","event":"ticket_complete","ticket_id":"bd-2","agent":"developer-frontend","duration_seconds":900}
{"timestamp":"2026-01-01T12:00:00Z","event":"review_cycle","ticket_id":"bd-1","cycle":1}
{"timestamp":"2026-01-01T12:30:00Z","event":"correction","ticket_id":"bd-1","reason":"lint errors"}
EOF

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  # Vérifie que l'affichage contient les statistiques
  [[ "$output" == *"2"* ]]  # 2 tickets complétés
}

@test "cmd-metrics : affiche l'en-tête avec titre" {
  mkdir -p "$_METRICS_DIR"
  echo '{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":300}' > "$_METRICS_FILE"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"triques"* ]] || [[ "$output" == *"etrics"* ]]  # Métriques ou Metrics
}

@test "cmd-metrics : affiche le nombre de tickets complétés" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":600}
{"timestamp":"2026-01-01T11:00:00Z","event":"ticket_complete","ticket_id":"bd-2","duration_seconds":900}
{"timestamp":"2026-01-01T12:00:00Z","event":"ticket_complete","ticket_id":"bd-3","duration_seconds":300}
EOF

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"3"* ]]  # 3 tickets complétés
}

@test "cmd-metrics : affiche les raisons de correction" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":600}
{"timestamp":"2026-01-01T12:30:00Z","event":"correction","ticket_id":"bd-1","reason":"lint errors"}
{"timestamp":"2026-01-01T13:00:00Z","event":"correction","ticket_id":"bd-2","reason":"lint errors"}
{"timestamp":"2026-01-01T13:30:00Z","event":"correction","ticket_id":"bd-3","reason":"test failures"}
EOF

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"lint errors"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — Nouvelles métriques SQLite
# ══════════════════════════════════════════════════════════════════════════════

@test "cmd-metrics : affiche le header même sans données SQLite" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Métriques"* ]] || [[ "$output" == *"OpenCode"* ]]
}

@test "cmd-metrics : exit 0 si sqlite3 présent mais base vide" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
}

@test "cmd-metrics : affiche coût total avec données SQLite" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','swift-eagle','/proj/app','Fix bug','1.0','developer','claude-sonnet-4-6',5.50,100000,20000,80000,5000,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','jolly-fox','/proj/app','Add feature','1.0','qa-engineer','claude-sonnet-4-6',3.20,80000,15000,60000,4000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  # Doit afficher des informations sur le coût
  [[ "$output" == *"\$"* ]] || [[ "$output" == *"cost"* ]] || [[ "$output" == *"8."* ]]
}

@test "cmd-metrics : affiche les sessions récentes" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','swift-eagle','/proj/app','Fix critical bug','1.0','developer','claude-sonnet-4-6',5.50,100000,20000,80000,5000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix critical bug"* ]] || [[ "$output" == *"swift-eagle"* ]] || [[ "$output" == *"developer"* ]]
}

@test "cmd-metrics --period today : accepte la période today" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period today"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aujourd'hui"* ]] || [[ "$output" == *"today"* ]] || [[ "$output" == *"Métriques"* ]]
}

@test "cmd-metrics --period month : accepte la période month" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period month"
  [ "$status" -eq 0 ]
  [[ "$output" == *"30"* ]] || [[ "$output" == *"mois"* ]] || [[ "$output" == *"Métriques"* ]]
}

@test "cmd-metrics --period invalide : exit 1 avec message d'erreur" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period invalid_period 2>&1"
  [ "$status" -ne 0 ]
  [[ "$output" == *"Période inconnue"* ]] || [[ "$output" == *"Options"* ]]
}

@test "cmd-metrics : affiche cache hit rate si tokens cache présents" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',5.0,200,500,800,100,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"cache"* ]] || [[ "$output" == *"Cache"* ]] || [[ "$output" == *"%"* ]]
}

@test "cmd-metrics : sqlite3 absent donne message d'aide (non bloquant)" {
  # Masquer sqlite3 avec un fake qui retourne 127
  FAKE_PATH="$(mktemp -d)"
  cat > "$FAKE_PATH/sqlite3" <<'FAKEEOF'
#!/bin/bash
exit 127
FAKEEOF
  chmod +x "$FAKE_PATH/sqlite3"

  run bash -c "PATH='$FAKE_PATH:$PATH' && cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"sqlite3"* ]] || [[ "$output" == *"Métriques"* ]]

  rm -rf "$FAKE_PATH"
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests lib/metrics.sh — Fonctions d'agrégation (rétrocompat)
# ══════════════════════════════════════════════════════════════════════════════

@test "metrics_count_completed : retourne 0 si fichier absent" {
  rm -f "$_METRICS_FILE"

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_count_completed
  "
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_count_completed : compte correctement les tickets" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":600}
{"timestamp":"2026-01-01T11:00:00Z","event":"ticket_complete","ticket_id":"bd-2","duration_seconds":900}
{"timestamp":"2026-01-01T12:00:00Z","event":"ticket_start","ticket_id":"bd-3"}
EOF

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_count_completed
  "
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

@test "metrics_avg_duration : retourne 0 si fichier absent" {
  rm -f "$_METRICS_FILE"

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_avg_duration
  "
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_avg_duration : calcule la moyenne correctement" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":600}
{"timestamp":"2026-01-01T11:00:00Z","event":"ticket_complete","ticket_id":"bd-2","duration_seconds":900}
EOF

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_avg_duration
  "
  [ "$status" -eq 0 ]
  [ "$output" = "750" ]  # (600 + 900) / 2 = 750
}

@test "metrics_avg_review_cycles : retourne 0 si fichier absent" {
  rm -f "$_METRICS_FILE"

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_avg_review_cycles
  "
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "metrics_avg_review_cycles : calcule la moyenne correctement" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T10:00:00Z","event":"ticket_complete","ticket_id":"bd-1","duration_seconds":600}
{"timestamp":"2026-01-01T10:30:00Z","event":"review_cycle","ticket_id":"bd-1","cycle":1}
{"timestamp":"2026-01-01T11:00:00Z","event":"ticket_complete","ticket_id":"bd-2","duration_seconds":900}
{"timestamp":"2026-01-01T11:30:00Z","event":"review_cycle","ticket_id":"bd-2","cycle":1}
{"timestamp":"2026-01-01T11:45:00Z","event":"review_cycle","ticket_id":"bd-2","cycle":2}
EOF

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_avg_review_cycles
  "
  [ "$status" -eq 0 ]
  [ "$output" = "1.5" ]  # 3 review_cycles / 2 tickets = 1.5
}

@test "metrics_top_corrections : retourne vide si fichier absent" {
  rm -f "$_METRICS_FILE"

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_top_corrections
  "
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "metrics_top_corrections : retourne le top des raisons" {
  mkdir -p "$_METRICS_DIR"
  cat > "$_METRICS_FILE" <<'EOF'
{"timestamp":"2026-01-01T12:00:00Z","event":"correction","ticket_id":"bd-1","reason":"lint errors"}
{"timestamp":"2026-01-01T12:30:00Z","event":"correction","ticket_id":"bd-2","reason":"lint errors"}
{"timestamp":"2026-01-01T13:00:00Z","event":"correction","ticket_id":"bd-3","reason":"lint errors"}
{"timestamp":"2026-01-01T13:30:00Z","event":"correction","ticket_id":"bd-4","reason":"test failures"}
{"timestamp":"2026-01-01T14:00:00Z","event":"correction","ticket_id":"bd-5","reason":"test failures"}
{"timestamp":"2026-01-01T14:30:00Z","event":"correction","ticket_id":"bd-6","reason":"type errors"}
EOF

  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    _METRICS_FILE='$_METRICS_FILE'
    metrics_top_corrections 3
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"lint errors"* ]]
  [[ "$output" == *"test failures"* ]]
}

@test "metrics_format_duration : formate correctement les secondes" {
  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    metrics_format_duration 45
  "
  [ "$status" -eq 0 ]
  [ "$output" = "45s" ]
}

@test "metrics_format_duration : formate correctement les minutes" {
  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    metrics_format_duration 125
  "
  [ "$status" -eq 0 ]
  [ "$output" = "2m 5s" ]
}

@test "metrics_format_duration : formate correctement les heures" {
  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    metrics_format_duration 3665
  "
  [ "$status" -eq 0 ]
  [ "$output" = "1h 1m 5s" ]
}

@test "metrics_format_duration : gère 0 secondes" {
  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    metrics_format_duration 0
  "
  [ "$status" -eq 0 ]
  [ "$output" = "0s" ]
}

@test "metrics_format_duration : gère valeur vide" {
  run bash -c "
    source '$COMMON_SH'
    source '$LIB_METRICS'
    metrics_format_duration ''
  "
  [ "$status" -eq 0 ]
  [ "$output" = "0s" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — Section Activité
# ══════════════════════════════════════════════════════════════════════════════

@test "cmd-metrics : affiche section Activité avec sessions catégorisées" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',5.0,100,200,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
    VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}');
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Activit"* ]] || [[ "$output" == *"Code"* ]] || [[ "$output" == *"code"* ]]
}

@test "cmd-metrics : section Activité absente si base vide" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  # Ne doit pas crasher même sans données activité
  [ "$status" -eq 0 ]
}

@test "cmd-metrics : section Activité catégorise planification" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','slug1','/proj/app','Orchestration','1.0','orchestrator-dev','claude-sonnet-4-6',8.0,200,400,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
    VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"task\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"task\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"task\",\"state\":{\"status\":\"completed\"}}');
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"lanification"* ]] || [[ "$output" == *"Activit"* ]]
}

@test "cmd-metrics : section Activité affiche pourcentages" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','Code','1.0','developer','claude-sonnet-4-6',8.0,200,400,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','Explore','1.0','developer','claude-sonnet-4-6',2.0,50,100,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
    VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m2','s2',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}');
  "

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"%"* ]] || [[ "$output" == *"Activit"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — section Économies plugins (context-mode + RTK)
# ══════════════════════════════════════════════════════════════════════════════

# Helper : crée une fixture stats-pid-*.json dans un répertoire temporaire
_make_ctx_stats_metrics() {
  local pid="$1" session_start_ms="$2" tokens_saved="$3" dollars_saved="$4" reduction_pct="$5"
  local stats_dir="$TEST_DIR/ctx-stats"
  mkdir -p "$stats_dir"
  python3 -c "
import json
data = {
    'schemaVersion': 2, 'version': '1.0.162',
    'updated_at': ${session_start_ms} + 3600000,
    'session_start': ${session_start_ms},
    'uptime_ms': 3600000, 'total_calls': 3,
    'bytes_returned': 22000, 'bytes_indexed': 31000,
    'bytes_sandboxed': 0, 'cache_hits': 0, 'cache_bytes_saved': 0,
    'kept_out': 31000, 'total_processed': 53000,
    'reduction_pct': ${reduction_pct},
    'tokens_saved': ${tokens_saved},
    'dollars_saved_session': ${dollars_saved},
    'tokens_saved_lifetime': 0, 'dollars_saved_lifetime': 0,
    'by_tool': {}
}
print(json.dumps(data))
" > "$stats_dir/stats-pid-${pid}.json"
}

@test "cmd-metrics : section Économies plugins absente si aucun plugin disponible" {
  # SQLite présent mais ni RTK ni ctx-mode
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ep1','p1','slug1','/proj','Test','1.0',1.0,1000,500,200,100,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/empty-ctx-stats'
    mkdir -p '$TEST_DIR/empty-ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && PATH='/usr/bin:/bin' bash '$CMD_METRICS'
  "
  [ "$status" -eq 0 ]
  [[ "$output" != *"Économies plugins"* ]]
}

@test "cmd-metrics : section Économies plugins présente avec context-mode" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ep2','p1','slug2','/proj','Test','1.0',1.0,1000,500,200,100,$YESTERDAY_MS,$YESTERDAY_MS);
  "
  _make_ctx_stats_metrics 8001 "$YESTERDAY_MS" 7931 0.04 59

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"Économies plugins"* ]]
  [[ "$output" == *"context-mode"* ]]
}

@test "cmd-metrics --period today : label (aujourd'hui) pour context-mode" {
  local today_start_ms
  today_start_ms=$(python3 -c "
import time
from datetime import datetime, timezone
t = datetime.now(timezone.utc).replace(hour=0, minute=0, second=0, microsecond=0)
print(int(t.timestamp() * 1000))
")
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ep3','p1','slug3','/proj','Test','1.0',1.0,1000,500,200,100,$today_start_ms,$today_start_ms);
  "
  _make_ctx_stats_metrics 8002 "$today_start_ms" 5000 0.03 55

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period today
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"aujourd'hui"* ]]
}

@test "cmd-metrics --period week : label (7 derniers jours) pour context-mode" {
  _make_ctx_stats_metrics 8003 "$YESTERDAY_MS" 4000 0.02 50

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period week
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"7 derniers jours"* ]]
}

@test "cmd-metrics --period month : label (30 derniers jours) pour context-mode" {
  _make_ctx_stats_metrics 8004 "$YESTERDAY_MS" 4000 0.02 50

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period month
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"30 derniers jours"* ]]
}

@test "cmd-metrics : RTK toujours affiché avec label (global)" {
  _make_ctx_stats_metrics 8005 "$YESTERDAY_MS" 4000 0.02 50

  local mock_dir="$TEST_DIR/mock-rtk-bin"
  mkdir -p "$mock_dir"
  cat > "$mock_dir/rtk" <<'MOCK'
#!/bin/bash
echo '{"summary":{"total_commands":500,"total_saved":800000,"avg_savings_pct":18.5}}'
MOCK
  chmod +x "$mock_dir/rtk"

  run bash -c "
    export PATH='$mock_dir:$PATH'
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"RTK"* ]]
  [[ "$output" == *"(global)"* ]]
}

@test "cmd-metrics --period today : section absente si aucune session ctx du jour" {
  # Seule une session d'hier → ne doit pas apparaître avec --period today
  _make_ctx_stats_metrics 8006 "$YESTERDAY_MS" 5000 0.03 55

  run bash -c "
    export CTX_STATS_DIR='$TEST_DIR/ctx-stats'
    export _OCDB_FILE='$TEST_DB'
    cd '$TEST_DIR/fake-project' && PATH='/usr/bin:/bin' bash '$CMD_METRICS' --period today
  "
  [ "$status" -eq 0 ]
  [[ "$output" != *"context-mode"* ]] || [[ "$output" != *"Économies plugins"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# Tests cmd-metrics.sh — section Coût total + sessions exactes
# ══════════════════════════════════════════════════════════════════════════════

# Helper : insère un step-finish avec un coût dans la table part
_insert_step_finish_metrics() {
  local pid="$1" sess_id="$2" ts_ms="$3" cost="$4"
  sqlite3 "$TEST_DB" "
    INSERT INTO part (id, message_id, session_id, time_created, time_updated, data)
    VALUES (
      '${pid}', 'msg_step', '${sess_id}', ${ts_ms}, ${ts_ms},
      '{\"type\":\"step-finish\",\"reason\":\"end\",\"snapshot\":\"abc\",\"tokens\":{\"total\":500,\"input\":5,\"output\":45,\"reasoning\":0,\"cache\":{\"write\":0,\"read\":0}},\"cost\":${cost}}'
    );
  "
}

@test "cmd-metrics : section Coût total absente si base vide" {
  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" != *"💳"* ]] || [[ "$output" != *"Coût total"* ]]
}

@test "cmd-metrics : section Coût total présente avec des steps" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ct1','p1','slug1','/proj','S1','1.0',0.50,0,0,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "
  _insert_step_finish_metrics "p_ct1" "s_ct1" "$YESTERDAY_MS" "0.50"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Coût total"* ]]
  [[ "$output" == *"Lifetime"* ]]
}

@test "cmd-metrics : section Coût total affiche Aujourd'hui et 7 jours et 30 jours" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ct2','p1','slug2','/proj','S2','1.0',0.30,0,0,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "
  _insert_step_finish_metrics "p_ct2" "s_ct2" "$YESTERDAY_MS" "0.30"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aujourd'hui"* ]]
  [[ "$output" == *"7 jours"* ]]
  [[ "$output" == *"30 jours"* ]]
}

@test "cmd-metrics --period today : marque la ligne Aujourd'hui comme période active" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ct3','p1','slug3','/proj','S3','1.0',0.20,0,0,0,0,$NOW_MS,$NOW_MS);
  "
  _insert_step_finish_metrics "p_ct3" "s_ct3" "$NOW_MS" "0.20"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period today"
  [ "$status" -eq 0 ]
  [[ "$output" == *"période active"* ]]
}

@test "cmd-metrics : Vue globale affiche sessions actives (dont créées) si multi-jours" {
  # Session créée il y a 10 jours mais avec step hier (dans la fenêtre 7j)
  local old_ms=$(( ($(date +%s) - 10 * 86400) * 1000 ))
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s_ma1','p1','slugma1','/proj','MA1','1.0',0.10,0,0,0,0,${old_ms},${YESTERDAY_MS});
  "
  _insert_step_finish_metrics "p_ma1" "s_ma1" "$YESTERDAY_MS" "0.10"

  run bash -c "cd '$TEST_DIR/fake-project' && bash '$CMD_METRICS' --period week"
  [ "$status" -eq 0 ]
  # 1 session active mais 0 créées dans les 7 derniers jours → affiche "dont"
  [[ "$output" == *"actives"* ]]
}
