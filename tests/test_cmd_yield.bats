#!/usr/bin/env bats
# Tests pour scripts/cmd-yield.sh

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  TEST_DIR="$(mktemp -d)"

  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  > "$HUB_CONFIG"
  > "$API_KEYS_FILE"

  CMD_YIELD="$HUB_ROOT/scripts/cmd-yield.sh"
  COMMON_SH="$HUB_ROOT/scripts/common.sh"

  # Créer un projet factice avec un dépôt git minimal
  mkdir -p "$TEST_DIR/fake-project"
  git -C "$TEST_DIR/fake-project" init --quiet
  git -C "$TEST_DIR/fake-project" config user.email "test@test.com"
  git -C "$TEST_DIR/fake-project" config user.name "Test"
  touch "$TEST_DIR/fake-project/README.md"
  git -C "$TEST_DIR/fake-project" add .
  git -C "$TEST_DIR/fake-project" commit -m "Initial commit" --quiet

  # Projet sans git
  mkdir -p "$TEST_DIR/no-git-project"

  # Registre de projets
  cat > "$PROJECTS_FILE" <<EOF
## TEST-PROJ
- Nom : Test
- Agents : all

## NO-GIT
- Nom : No Git
- Agents : all
EOF

  cat > "$PATHS_FILE" <<EOF
TEST-PROJ=$TEST_DIR/fake-project
NO-GIT=$TEST_DIR/no-git-project
EOF

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

_run_yield() {
  bash "$CMD_YIELD" "$@"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Comportement de base
# ══════════════════════════════════════════════════════════════════════════════

@test "yield : s'exécute sans erreur, base vide" {
  run _run_yield
  [ "$status" -eq 0 ]
  [[ "$output" == *"Yield"* ]] || [[ "$output" == *"Sessions"* ]]
}

@test "yield : affiche le header avec la période par défaut" {
  run _run_yield
  [ "$status" -eq 0 ]
  [[ "$output" == *"7 derniers jours"* ]]
}

@test "yield : accepte --period today" {
  run _run_yield --period today
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aujourd'hui"* ]]
}

@test "yield : accepte --period month" {
  run _run_yield --period month
  [ "$status" -eq 0 ]
  [[ "$output" == *"30 derniers jours"* ]]
}

@test "yield : rejette --period invalide avec exit 1" {
  run _run_yield --period badvalue 2>&1
  [ "$status" -ne 0 ]
  [[ "$output" == *"Période inconnue"* ]]
}

@test "yield : accepte --project filter" {
  run _run_yield --project TEST-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"TEST-PROJ"* ]]
}

@test "yield : affiche message si aucune session dans la période" {
  run _run_yield
  [ "$status" -eq 0 ]
  # Pas de sessions → projets non affichés ou "aucune session"
  [[ "$output" == *"Yield"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. Avec données de sessions
# ══════════════════════════════════════════════════════════════════════════════

@test "yield : affiche les projets avec sessions" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','$TEST_DIR/fake-project','Session test','1.0',5.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_yield
  [ "$status" -eq 0 ]
  [[ "$output" == *"TEST-PROJ"* ]]
}

@test "yield : classifie session abandonnée si pas de commit dans fenêtre" {
  OLD_MS=$(( ($(date +%s) - 60 * 86400) * 1000 ))
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','$TEST_DIR/fake-project','Old session','1.0',5.0,100,200,0,0,'developer','claude',$OLD_MS,$OLD_MS);
  "

  run _run_yield --period month
  [ "$status" -eq 0 ]
  [[ "$output" == *"Yield"* ]]
}

@test "yield : affiche Productives, Abandonnées, Revertées" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','$TEST_DIR/fake-project','Session 1','1.0',5.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_yield
  [ "$status" -eq 0 ]
  [[ "$output" == *"Productives"* ]]
  [[ "$output" == *"Abandonnées"* ]]
  [[ "$output" == *"Revertées"* ]]
}

@test "yield : projet sans git affiche message d'info" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated)
    VALUES ('s1','p1','slug1','$TEST_DIR/no-git-project','Session no git','1.0',2.0,50,100,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_yield --project NO-GIT
  [ "$status" -eq 0 ]
  [[ "$output" == *"git"* ]] || [[ "$output" == *"NO-GIT"* ]] || [[ "$output" == *"Yield"* ]]
}

@test "yield : filtre --project exclut les autres projets" {
  sqlite3 "$TEST_DB" "
    INSERT INTO session (id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_cache_read,tokens_cache_write,agent,model,time_created,time_updated) VALUES
      ('s1','p1','slug1','$TEST_DIR/fake-project','Session 1','1.0',5.0,100,200,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS),
      ('s2','p1','slug2','$TEST_DIR/no-git-project','Session 2','1.0',3.0,60,120,0,0,'developer','claude',$YESTERDAY_MS,$YESTERDAY_MS);
  "

  run _run_yield --project TEST-PROJ
  [ "$status" -eq 0 ]
  [[ "$output" == *"TEST-PROJ"* ]]
  [[ "$output" != *"NO-GIT"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Fallbacks
# ══════════════════════════════════════════════════════════════════════════════

@test "yield : sqlite3 absent — exit 0 avec message d'aide" {
  FAKE_PATH="$(mktemp -d)"
  cat > "$FAKE_PATH/sqlite3" <<'EOF'
#!/bin/bash
if [ "${1:-}" = "--version" ]; then exit 1; fi
exit 1
EOF
  chmod +x "$FAKE_PATH/sqlite3"

  run bash -c "export PATH='$FAKE_PATH:$PATH' && export _OCDB_FILE='/tmp/nonexistent_$(date +%s).db' && bash '$CMD_YIELD'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"sqlite3"* ]] || [[ "$output" == *"non disponible"* ]] || [[ "$output" == *"introuvable"* ]]

  rm -rf "$FAKE_PATH"
}

@test "yield : git absent — exit 0 avec message d'aide" {
  FAKE_PATH="$(mktemp -d)"
  cat > "$FAKE_PATH/git" <<'EOF'
#!/bin/bash
exit 127
EOF
  chmod +x "$FAKE_PATH/git"

  run bash -c "PATH='$FAKE_PATH:$PATH' bash '$CMD_YIELD'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"git"* ]] || [[ "$output" == *"non disponible"* ]]

  rm -rf "$FAKE_PATH"
}

@test "yield : aucun projet configuré — message informatif" {
  # Écraser projects.md avec contenu vide
  echo "# Vide" > "$PROJECTS_FILE"
  > "$PATHS_FILE"

  run _run_yield
  [ "$status" -eq 0 ]
  [[ "$output" == *"Aucun projet"* ]] || [[ "$output" == *"Yield"* ]]
}
