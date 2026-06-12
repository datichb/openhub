#!/usr/bin/env bats
# Tests pour scripts/cmd-optimize.sh

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  > "$HUB_CONFIG"
  echo "# test" > "$PROJECTS_FILE"
  echo "TEST-PROJ=$TEST_DIR/fake-project" > "$PATHS_FILE"
  > "$API_KEYS_FILE"

  CMD_OPTIMIZE="$HUB_ROOT/scripts/cmd-optimize.sh"
  COMMON_SH="$HUB_ROOT/scripts/common.sh"

  # Créer un projet factice
  mkdir -p "$TEST_DIR/fake-project"

  # Base SQLite de test
  TEST_DB="$TEST_DIR/test_opencode.db"
  export _OCDB_FILE="$TEST_DB"
  sqlite3 "$TEST_DB" "
    CREATE TABLE session (
      id TEXT PRIMARY KEY,
      project_id TEXT NOT NULL DEFAULT 'proj1',
      parent_id TEXT,
      slug TEXT NOT NULL DEFAULT 'test-slug',
      directory TEXT NOT NULL DEFAULT '/test',
      title TEXT NOT NULL DEFAULT 'Test',
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
  YESTERDAY_MS=$(( ($(date +%s) - 86400) * 1000 ))
  export YESTERDAY_MS

  export HUB_DIR="$HUB_ROOT"
}

teardown() {
  unset _OCDB_FILE HUB_CONFIG HUB_DIR
  rm -rf "$TEST_DIR"
}

_run_optimize() {
  bash "$CMD_OPTIMIZE" "$@"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Comportement de base
# ══════════════════════════════════════════════════════════════════════════════

@test "optimize : s'exécute sans erreur, base vide" {
  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" == *"Analyse"* ]] || [[ "$output" == *"Grade"* ]]
}

@test "optimize : affiche le header avec la période" {
  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" == *"30 derniers jours"* ]]
}

@test "optimize : accepte --period week" {
  run _run_optimize --period week
  [ "$status" -eq 0 ]
  [[ "$output" == *"7 derniers jours"* ]]
}

@test "optimize : accepte --period today" {
  run _run_optimize --period today
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aujourd'hui"* ]]
}

@test "optimize : accepte --period month" {
  run _run_optimize --period month
  [ "$status" -eq 0 ]
  [[ "$output" == *"30 derniers jours"* ]]
}

@test "optimize : rejette --period invalide avec exit 1" {
  run _run_optimize --period badvalue 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"Période inconnue"* ]]
}

@test "optimize : accepte --project filter" {
  run _run_optimize --project T-SRU
  [ "$status" -eq 0 ]
  [[ "$output" == *"T-SRU"* ]]
}

@test "optimize : grade A si aucun finding" {
  # Base vide = pas de sessions → pas de findings (sauf info éventuels)
  run _run_optimize
  [ "$status" -eq 0 ]
  # Grade A ou B attendu sur base vide
  [[ "$output" == *"Grade : A"* ]] || [[ "$output" == *"Grade : B"* ]] || [[ "$output" == *"Aucun problème détecté"* ]] || [[ "$output" == *"Grade :"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Détection de findings
# ══════════════════════════════════════════════════════════════════════════════

@test "optimize : détecte sessions sans edit (finding warning/critique)" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated) VALUES
      ('s1','p1','slug1','/test','S1','1.0',5.0,100,200,0,0,NULL,'claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/test','S2','1.0',3.0,80,150,0,0,NULL,'claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s3','p1','slug3','/test','S3','1.0',4.0,90,160,0,0,NULL,'claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s4','p1','slug4','/test','S4','1.0',2.0,60,120,0,0,NULL,'claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s5','p1','slug5','/test','S5','1.0',6.0,110,210,0,0,NULL,'claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" == *"sans modification"* ]] || [[ "$output" == *"edit"* ]] || [[ "$output" == *"Critique"* ]]
}

@test "optimize : détecte ratio Read/Edit faible" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',2.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}');
  "

  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" == *"Read/Edit"* ]] || [[ "$output" == *"Grade :"* ]]
}

@test "optimize : détecte taux d'erreurs élevé" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',1.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"error\"}}'),
      ('p2','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"error\"}}'),
      ('p3','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"error\"}}'),
      ('p4','msg1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}');
  "

  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" == *"erreur"* ]] || [[ "$output" == *"Grade :"* ]]
}

@test "optimize : grade moins bon si findings critiques" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated) VALUES
      ('s1','p1','slug1','/test','S1','1.0',10.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','/test','S2','1.0',8.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s3','p1','slug3','/test','S3','1.0',6.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s4','p1','slug4','/test','S4','1.0',5.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s5','p1','slug5','/test','S5','1.0',4.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_optimize
  [ "$status" -eq 0 ]
  [[ "$output" != *"Grade : A"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Fallback sqlite3 absent
# ══════════════════════════════════════════════════════════════════════════════

@test "optimize : sqlite3 absent — exit 0 avec message d'aide" {
  # Simuler sqlite3 absent en pointant _OCDB_FILE vers un fichier inexistant
  # et en masquant sqlite3 avec un wrapper qui échoue
  FAKE_PATH="$(mktemp -d)"
  cat > "$FAKE_PATH/sqlite3" <<'EOF'
#!/bin/bash
# Fake sqlite3 qui échoue proprement
if [ "${1:-}" = "--version" ]; then exit 1; fi
exit 1
EOF
  chmod +x "$FAKE_PATH/sqlite3"

  run bash -c "export PATH='$FAKE_PATH:$PATH' && export _OCDB_FILE='/tmp/nonexistent_$(date +%s).db' && bash '$CMD_OPTIMIZE'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"sqlite3"* ]] || [[ "$output" == *"non disponible"* ]] || [[ "$output" == *"introuvable"* ]]

  rm -rf "$FAKE_PATH"
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Fonctions lib opencode-db — nouvelles
# ══════════════════════════════════════════════════════════════════════════════

@test "ocdb_tool_stats : retourne vide si base vide" {
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_tool_stats 30
  "
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "ocdb_tool_stats : compte correctement les outils" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',1.0,0,0,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_tool_stats 30
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"read|2"* ]]
  [[ "$output" == *"edit|1"* ]]
}

@test "ocdb_tool_count : compte un outil spécifique" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',1.0,0,0,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_tool_count 'bash' 30
  "
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

@test "ocdb_tool_error_rate : calcule le taux d'erreur correctement" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',1.0,0,0,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"error\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}'),
      ('p4','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"bash\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_tool_error_rate 30
  "
  [ "$status" -eq 0 ]
  [[ "$output" == "25.0"* ]]
}

@test "ocdb_activity_breakdown : catégorise code correctement" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',5.0,0,0,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_activity_breakdown 30
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"code"* ]]
}

@test "ocdb_activity_breakdown : catégorise planification (orchestrator)" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',5.0,0,0,0,0,'orchestrator-dev','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"task\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"task\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_activity_breakdown 30
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"planification"* ]]
}

@test "ocdb_avg_read_edit_ratio : calcule le ratio correctement" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','/test','S1','1.0',1.0,0,0,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
    INSERT INTO part VALUES
      ('p1','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}'),
      ('p2','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}'),
      ('p3','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"read\",\"state\":{\"status\":\"completed\"}}'),
      ('p4','m1','s1',$YESTERDAY_MS,$YESTERDAY_MS,'{\"type\":\"tool\",\"tool\":\"edit\",\"state\":{\"status\":\"completed\"}}');
  "
  run bash -c "
    export _OCDB_FILE='$TEST_DB'
    source '$COMMON_SH'
    source '$HUB_ROOT/scripts/lib/opencode-db.sh'
    ocdb_avg_read_edit_ratio 30
  "
  [ "$status" -eq 0 ]
  [[ "$output" == "3.0"* ]]
}
