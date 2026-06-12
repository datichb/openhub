#!/usr/bin/env bats
# Tests pour scripts/lib/opencode-db.sh
# Couvre : vérifications disponibilité, requêtes SQLite, formatage, agrégation

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  TEST_DIR="$(mktemp -d)"

  COMMON_SH="$HUB_ROOT/scripts/common.sh"
  LIB_OCDB="$HUB_ROOT/scripts/lib/opencode-db.sh"

  # Créer une base SQLite de test avec le schéma minimal d'OpenCode
  TEST_DB="$TEST_DIR/test_opencode.db"
  export _OCDB_FILE="$TEST_DB"

  # Initialiser le schéma minimal
  sqlite3 "$TEST_DB" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY,
      project_id TEXT NOT NULL DEFAULT 'proj1',
      parent_id TEXT,
      slug TEXT NOT NULL DEFAULT 'test-slug',
      directory TEXT NOT NULL DEFAULT '/test/project',
      title TEXT NOT NULL DEFAULT 'Test Session',
      version TEXT NOT NULL DEFAULT '1.0',
      share_url TEXT,
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

  # Timestamp "maintenant - 1 jour" en ms
  NOW_MS=$(( $(date +%s) * 1000 ))
  YESTERDAY_MS=$(( ($(date +%s) - 86400) * 1000 ))
  WEEK_AGO_MS=$(( ($(date +%s) - 7 * 86400) * 1000 ))
  OLD_MS=$(( ($(date +%s) - 60 * 86400) * 1000 ))

  export NOW_MS YESTERDAY_MS WEEK_AGO_MS OLD_MS

  # Source commune
  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  > "$HUB_CONFIG"
  echo "# test" > "$PROJECTS_FILE"
  > "$PATHS_FILE"
  > "$API_KEYS_FILE"
}

teardown() {
  unset _OCDB_FILE
  rm -rf "$TEST_DIR"
}

# Helper : source la lib dans un sous-shell pour isolation
_source_ocdb() {
  bash -c "
    export _OCDB_FILE='$TEST_DB'
    export PROJECTS_FILE='$TEST_DIR/projects.md'
    export PATHS_FILE='$TEST_DIR/paths.local.md'
    export API_KEYS_FILE='$TEST_DIR/api-keys.local.md'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    $1
  "
}

# ══════════════════════════════════════════════════════════════════════════════
# A. ocdb_check_available
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_check_available : retourne 0 si sqlite3 présent et db existante" {
  run _source_ocdb "ocdb_check_available"
  [ "$status" -eq 0 ]
}

@test "ocdb_check_available : retourne 1 si db introuvable" {
  run bash -c "
    export _OCDB_FILE='/tmp/nonexistent_db_$(date +%s).db'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_check_available
  "
  [ "$status" -eq 1 ]
}

@test "ocdb_check_available : affiche message utile si db absente" {
  run bash -c "
    export _OCDB_FILE='/tmp/nonexistent_db_$(date +%s).db'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_check_available 2>&1
  "
  [ "$status" -eq 1 ]
  [[ "$output" == *"introuvable"* ]] || [[ "$output" == *"non trouvé"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. ocdb_get_db_path
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_get_db_path : retourne _OCDB_FILE si défini" {
  run _source_ocdb "ocdb_get_db_path"
  [ "$status" -eq 0 ]
  [ "$output" = "$TEST_DB" ]
}

@test "ocdb_get_db_path : utilise XDG_DATA_HOME si défini" {
  run bash -c "
    unset _OCDB_FILE
    export XDG_DATA_HOME='/custom/data'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_get_db_path
  "
  [ "$status" -eq 0 ]
  [ "$output" = "/custom/data/opencode/opencode.db" ]
}

@test "ocdb_get_db_path : utilise le chemin par défaut si pas de XDG_DATA_HOME" {
  run bash -c "
    unset _OCDB_FILE XDG_DATA_HOME
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_get_db_path
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *".local/share/opencode/opencode.db" ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. ocdb_total_cost — base vide
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_total_cost : retourne 0 si base vide" {
  run _source_ocdb "ocdb_total_cost 7"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "ocdb_sessions_count : retourne 0 si base vide" {
  run _source_ocdb "ocdb_sessions_count 7"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "ocdb_cache_hit_rate : retourne 0.0 si base vide" {
  run _source_ocdb "ocdb_cache_hit_rate 7"
  [ "$status" -eq 0 ]
  [ "$output" = "0.0" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. ocdb_total_cost — avec données
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_total_cost : somme correcte des coûts (7j)" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','Session 1','1.0','developer','claude-sonnet-4-6',5.50,100000,20000,80000,5000,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','Session 2','1.0','qa-engineer','claude-sonnet-4-6',3.20,80000,15000,60000,4000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_total_cost 7"
  [ "$status" -eq 0 ]
  # 5.50 + 3.20 = 8.70
  [[ "$output" == "8.7"* ]]
}

@test "ocdb_total_cost : exclut les sessions trop anciennes" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','Recent','1.0','developer','claude-sonnet-4-6',5.00,100000,20000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','Old','1.0','developer','claude-sonnet-4-6',100.00,100000,20000,0,0,$OLD_MS,$OLD_MS);
  "

  run _source_ocdb "ocdb_total_cost 7"
  [ "$status" -eq 0 ]
  # Seul s1 (hier) doit être compté
  [[ "$output" == "5"* ]]
  [[ "$output" != *"100"* ]]
}

@test "ocdb_total_cost : exclut les sous-sessions (parent_id non null)" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1',NULL,'slug1','/proj/app','Parent','1.0','developer','claude-sonnet-4-6',5.00,100000,20000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','s1','slug2','/proj/app','Child','1.0','developer','claude-sonnet-4-6',2.00,50000,10000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_total_cost 7"
  [ "$status" -eq 0 ]
  # Seul s1 (parent_id IS NULL) doit être compté
  [[ "$output" == "5"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. ocdb_sessions_count avec données
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_sessions_count : compte correctement (7j)" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/a','S1','1.0','developer','claude-sonnet-4-6',1.0,10000,2000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/b','S2','1.0','qa-engineer','claude-sonnet-4-6',2.0,20000,4000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s3','p1','slug3','/proj/c','S3','1.0','developer','claude-sonnet-4-6',3.0,30000,6000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_sessions_count 7"
  [ "$status" -eq 0 ]
  [ "$output" = "3" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# F. ocdb_cache_hit_rate
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_cache_hit_rate : calcule correctement le pourcentage" {
  # cache_read=800, input=200 → 800/(200+800)*100 = 80.0%
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',5.0,200,500,800,100,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_cache_hit_rate 7"
  [ "$status" -eq 0 ]
  [[ "$output" == "80.0"* ]]
}

@test "ocdb_cache_hit_rate : retourne 0.0 si pas de tokens" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',0.0,0,0,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_cache_hit_rate 7"
  [ "$status" -eq 0 ]
  [ "$output" = "0.0" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# G. ocdb_cost_by_project
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_cost_by_project : retourne les projets triés par coût décroissant" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/alpha','S1','1.0','developer','claude-sonnet-4-6',8.0,100000,20000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/alpha','S2','1.0','developer','claude-sonnet-4-6',2.0,50000,10000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s3','p1','slug3','/proj/beta','S3','1.0','developer','claude-sonnet-4-6',5.0,80000,15000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_cost_by_project 7"
  [ "$status" -eq 0 ]
  # alpha doit être en premier (10.0 total), beta second (5.0)
  local lines=($output)
  [[ "${lines[0]}" == *"alpha"* ]]
  [[ "${lines[1]}" == *"beta"* ]]
}

@test "ocdb_cost_by_project : retourne vide si base vide" {
  run _source_ocdb "ocdb_cost_by_project 7"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# H. ocdb_cost_by_agent
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_cost_by_agent : retourne les agents triés par coût décroissant" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',7.0,100000,20000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','S2','1.0','qa-engineer','claude-sonnet-4-6',3.0,50000,10000,0,0,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s3','p1','slug3','/proj/app','S3','1.0','developer','claude-sonnet-4-6',2.0,40000,8000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_cost_by_agent 7"
  [ "$status" -eq 0 ]
  # developer (9.0) avant qa-engineer (3.0)
  [[ "$output" == *"developer"* ]]
  [[ "$output" == *"qa-engineer"* ]]
  local first_line
  first_line=$(echo "$output" | head -1)
  [[ "$first_line" == *"developer"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# I. ocdb_recent_sessions
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_recent_sessions : retourne N sessions max, ordre décroissant" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','first-slug','/proj/app','First Session','1.0','developer','claude-sonnet-4-6',1.0,10000,2000,0,0,$(( YESTERDAY_MS - 3600000 )),$(( YESTERDAY_MS - 3600000 ))),
      ('s2','p1','second-slug','/proj/app','Second Session','1.0','qa-engineer','claude-sonnet-4-6',2.0,20000,4000,0,0,$(( YESTERDAY_MS - 1800000 )),$(( YESTERDAY_MS - 1800000 ))),
      ('s3','p1','third-slug','/proj/app','Third Session','1.0','developer','claude-sonnet-4-6',3.0,30000,6000,0,0,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_recent_sessions 2 7"
  [ "$status" -eq 0 ]
  # Max 2 sessions
  [ "$(echo "$output" | wc -l | tr -d ' ')" -eq 2 ]
  # La plus récente en premier (third-slug)
  local first_line
  first_line=$(echo "$output" | head -1)
  [[ "$first_line" == *"third-slug"* ]]
}

@test "ocdb_recent_sessions : retourne vide si base vide" {
  run _source_ocdb "ocdb_recent_sessions 5 7"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# J. ocdb_format_tokens
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_format_tokens : formate 0" {
  run _source_ocdb "ocdb_format_tokens 0"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "ocdb_format_tokens : formate en K (milliers)" {
  run _source_ocdb "ocdb_format_tokens 1500"
  [ "$status" -eq 0 ]
  [[ "$output" == *"K"* ]]
  [[ "$output" == "1.5K" ]]
}

@test "ocdb_format_tokens : formate en M (millions)" {
  run _source_ocdb "ocdb_format_tokens 2500000"
  [ "$status" -eq 0 ]
  [[ "$output" == *"M"* ]]
  [[ "$output" == "2.5M" ]]
}

@test "ocdb_format_tokens : formate des petits nombres" {
  run _source_ocdb "ocdb_format_tokens 999"
  [ "$status" -eq 0 ]
  [ "$output" = "999" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# K. ocdb_format_date
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_format_date : retourne '--' pour timestamp 0" {
  run _source_ocdb "ocdb_format_date 0"
  [ "$status" -eq 0 ]
  [ "$output" = "--" ]
}

@test "ocdb_format_date : formate un timestamp valide" {
  # 2026-01-15 12:00:00 UTC en ms
  local ts_ms=$(( 1768564800 * 1000 ))
  run _source_ocdb "ocdb_format_date $ts_ms"
  [ "$status" -eq 0 ]
  # Doit contenir une date au format DD/MM HH:MM
  [[ "$output" =~ [0-9]{2}/[0-9]{2}[[:space:]][0-9]{2}:[0-9]{2} ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# L. ocdb_aggregate — intégration
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_aggregate : exporte toutes les variables globales (base vide)" {
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    export PROJECTS_FILE='$TEST_DIR/projects.md'
    export PATHS_FILE='$TEST_DIR/paths.local.md'
    export API_KEYS_FILE='$TEST_DIR/api-keys.local.md'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_aggregate 7
    echo \"COST=\$OCDB_TOTAL_COST\"
    echo \"SESSIONS=\$OCDB_TOTAL_SESSIONS\"
    echo \"INPUT=\$OCDB_TOKENS_INPUT\"
    echo \"RATE=\$OCDB_CACHE_HIT_RATE\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"COST=0"* ]]
  [[ "$output" == *"SESSIONS=0"* ]]
  [[ "$output" == *"INPUT=0"* ]]
  [[ "$output" == *"RATE=0.0"* ]]
}

@test "ocdb_aggregate : exporte les bonnes valeurs avec données" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','Session 1','1.0','developer','claude-sonnet-4-6',10.0,500000,100000,400000,50000,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','Session 2','1.0','qa-engineer','claude-haiku-3-5',5.0,200000,40000,100000,20000,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    export PROJECTS_FILE='$TEST_DIR/projects.md'
    export PATHS_FILE='$TEST_DIR/paths.local.md'
    export API_KEYS_FILE='$TEST_DIR/api-keys.local.md'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_aggregate 7
    echo \"COST=\$OCDB_TOTAL_COST\"
    echo \"SESSIONS=\$OCDB_TOTAL_SESSIONS\"
    echo \"INPUT=\$OCDB_TOKENS_INPUT\"
    echo \"CACHE_READ=\$OCDB_TOKENS_CACHE_READ\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"COST=15"* ]]
  [[ "$output" == *"SESSIONS=2"* ]]
  [[ "$output" == *"INPUT=700000"* ]]
  [[ "$output" == *"CACHE_READ=500000"* ]]
}

@test "ocdb_aggregate : retourne 1 si db inaccessible, sans crash" {
  run bash -c "
    export _OCDB_FILE='/tmp/nonexistent_$(date +%s).db'
    source '$COMMON_SH'
    source '$LIB_OCDB'
    ocdb_aggregate 7
    echo \"exit=\$?\"
  "
  # Ne doit pas crash (set -e dans le contexte parent)
  [[ "$output" == *"exit=1"* ]] || [ "$status" -ne 0 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# M. ocdb_tokens_summary
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_tokens_summary : retourne 0|0|0|0 si base vide" {
  run _source_ocdb "ocdb_tokens_summary 7"
  [ "$status" -eq 0 ]
  [ "$output" = "0|0|0|0" ]
}

@test "ocdb_tokens_summary : agrège correctement tous les tokens" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, cost, tokens_input, tokens_output, tokens_cache_read, tokens_cache_write, time_created, time_updated)
    VALUES
      ('s1','p1','slug1','/proj/app','S1','1.0','developer','claude-sonnet-4-6',1.0,100,200,300,400,$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/proj/app','S2','1.0','developer','claude-sonnet-4-6',1.0,100,200,300,400,$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _source_ocdb "ocdb_tokens_summary 7"
  [ "$status" -eq 0 ]
  [ "$output" = "200|400|600|800" ]
}
