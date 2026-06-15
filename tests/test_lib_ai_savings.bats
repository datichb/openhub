#!/usr/bin/env bats
# Tests pour scripts/lib/ai-savings.sh
# Couvre : lecture stats context-mode, parsing RTK JSON, formatage tokens,
#          filtrage par période, comportement sans données

setup() {
  HUB_ROOT="$BATS_TEST_DIRNAME/.."
  TEST_DIR="$(mktemp -d)"

  COMMON_SH="$HUB_ROOT/scripts/common.sh"
  LIB_AI_SAVINGS="$HUB_ROOT/scripts/lib/ai-savings.sh"

  # Répertoire de fixtures stats context-mode isolé
  CTX_STATS_TEST_DIR="$TEST_DIR/ctx-stats"
  mkdir -p "$CTX_STATS_TEST_DIR"
  export CTX_STATS_DIR="$CTX_STATS_TEST_DIR"

  # Timestamps utiles (en millisecondes)
  NOW_MS=$(python3 -c "import time; print(int(time.time() * 1000))")
  TODAY_START_MS=$(python3 -c "
import time
from datetime import datetime, timezone
t = datetime.now(timezone.utc).replace(hour=0, minute=0, second=0, microsecond=0)
print(int(t.timestamp() * 1000))
")
  YESTERDAY_MS=$(python3 -c "import time; print(int((time.time() - 86400) * 1000))")
  WEEK_AGO_MS=$(python3 -c "import time; print(int((time.time() - 7 * 86400) * 1000))")
  OLD_MS=$(python3 -c "import time; print(int((time.time() - 60 * 86400) * 1000))")

  export NOW_MS TODAY_START_MS YESTERDAY_MS WEEK_AGO_MS OLD_MS

  # Fichiers de config minimaux requis par common.sh
  export PROJECTS_FILE="$TEST_DIR/projects.md"
  export PATHS_FILE="$TEST_DIR/paths.local.md"
  export API_KEYS_FILE="$TEST_DIR/api-keys.local.md"
  export HUB_CONFIG="$TEST_DIR/hub.json"
  echo "# test" > "$PROJECTS_FILE"
  > "$PATHS_FILE"
  > "$API_KEYS_FILE"
  > "$HUB_CONFIG"
}

teardown() {
  unset CTX_STATS_DIR
  rm -rf "$TEST_DIR"
}

# ─────────────────────────────────────────────────────────────────────────────
# Helper : crée un fichier stats-pid-*.json de fixture
# _make_ctx_stats <pid> <session_start_ms> <tokens_saved> <dollars_saved> <reduction_pct> [total_calls]
# ─────────────────────────────────────────────────────────────────────────────
_make_ctx_stats() {
  local pid="$1"
  local session_start_ms="$2"
  local tokens_saved="$3"
  local dollars_saved="$4"
  local reduction_pct="$5"
  local total_calls="${6:-2}"

  python3 -c "
import json
data = {
    'schemaVersion': 2,
    'version': '1.0.162',
    'updated_at': ${session_start_ms} + 3600000,
    'session_start': ${session_start_ms},
    'uptime_ms': 3600000,
    'total_calls': ${total_calls},
    'bytes_returned': 22000,
    'bytes_indexed': 31000,
    'bytes_sandboxed': 0,
    'cache_hits': 0,
    'cache_bytes_saved': 0,
    'kept_out': 31000,
    'total_processed': 53000,
    'reduction_pct': ${reduction_pct},
    'tokens_saved': ${tokens_saved},
    'dollars_saved_session': ${dollars_saved},
    'tokens_saved_lifetime': 0,
    'dollars_saved_lifetime': 0,
    'by_tool': {}
}
print(json.dumps(data))
" > "$CTX_STATS_TEST_DIR/stats-pid-${pid}.json"
}

# Helper : source la lib dans un sous-shell pour isolation
_source_ai_savings() {
  bash -c "
    export CTX_STATS_DIR='$CTX_STATS_TEST_DIR'
    export PROJECTS_FILE='$TEST_DIR/projects.md'
    export PATHS_FILE='$TEST_DIR/paths.local.md'
    export API_KEYS_FILE='$TEST_DIR/api-keys.local.md'
    export HUB_CONFIG='$TEST_DIR/hub.json'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    $1
  "
}

# ══════════════════════════════════════════════════════════════════════════════
# A. aisavings_format_tokens
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_format_tokens : formate millions" {
  run _source_ai_savings "aisavings_format_tokens 1630415"
  [ "$status" -eq 0 ]
  [ "$output" = "1.6M" ]
}

@test "aisavings_format_tokens : formate milliers" {
  run _source_ai_savings "aisavings_format_tokens 7931"
  [ "$status" -eq 0 ]
  [ "$output" = "7.9K" ]
}

@test "aisavings_format_tokens : laisse intact si < 1000" {
  run _source_ai_savings "aisavings_format_tokens 450"
  [ "$status" -eq 0 ]
  [ "$output" = "450" ]
}

@test "aisavings_format_tokens : gère 0" {
  run _source_ai_savings "aisavings_format_tokens 0"
  [ "$status" -eq 0 ]
  [ "$output" = "0" ]
}

@test "aisavings_format_tokens : gère exactement 1000" {
  run _source_ai_savings "aisavings_format_tokens 1000"
  [ "$status" -eq 0 ]
  [ "$output" = "1.0K" ]
}

@test "aisavings_format_tokens : gère exactement 1000000" {
  run _source_ai_savings "aisavings_format_tokens 1000000"
  [ "$status" -eq 0 ]
  [ "$output" = "1.0M" ]
}

# ══════════════════════════════════════════════════════════════════════════════
# B. aisavings_load_ctx_stats — répertoire absent ou vide
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_load_ctx_stats : retourne 1 si répertoire absent" {
  run bash -c "
    export CTX_STATS_DIR='/tmp/nonexistent_ctx_stats_dir_$(date +%s)'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_ctx_stats 0
  "
  [ "$status" -eq 1 ]
}

@test "aisavings_load_ctx_stats : retourne 1 si répertoire vide" {
  run _source_ai_savings "aisavings_load_ctx_stats 0"
  [ "$status" -eq 1 ]
}

@test "aisavings_load_ctx_stats : retourne 1 si tous les fichiers ont total_calls=0" {
  _make_ctx_stats 1001 "$NOW_MS" 5000 0.02 50 0
  _make_ctx_stats 1002 "$YESTERDAY_MS" 3000 0.01 40 0
  run _source_ai_savings "aisavings_load_ctx_stats 0"
  [ "$status" -eq 1 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# C. aisavings_load_ctx_stats — lifetime (since_days=0)
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_load_ctx_stats 0 : agrège toutes les sessions" {
  _make_ctx_stats 2001 "$NOW_MS"       7931  0.04 59 4
  _make_ctx_stats 2002 "$WEEK_AGO_MS"  3000  0.01 40 2
  _make_ctx_stats 2003 "$OLD_MS"       1500  0.01 30 1
  run _source_ai_savings "
    aisavings_load_ctx_stats 0
    echo \"tokens:\$CTX_TOKENS_SAVED sessions:\$CTX_SESSIONS_COUNT available:\$CTX_AVAILABLE\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"tokens:12431"* ]]
  [[ "$output" == *"sessions:3"* ]]
  [[ "$output" == *"available:1"* ]]
}

@test "aisavings_load_ctx_stats 0 : exporte CTX_PERIOD_LABEL=(lifetime)" {
  _make_ctx_stats 2010 "$NOW_MS" 7931 0.04 59 4
  run _source_ai_savings "
    aisavings_load_ctx_stats 0
    echo \"\$CTX_PERIOD_LABEL\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"(lifetime)"* ]]
}

@test "aisavings_load_ctx_stats 0 : exporte CTX_DOLLARS_SAVED correctement" {
  _make_ctx_stats 2020 "$NOW_MS" 7931 0.04 59 4
  run _source_ai_savings "
    aisavings_load_ctx_stats 0
    echo \"\$CTX_DOLLARS_SAVED\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"0.04"* ]]
}

@test "aisavings_load_ctx_stats 0 : calcule CTX_REDUCTION_PCT en moyenne" {
  _make_ctx_stats 2030 "$NOW_MS"      60 0.01 60 2
  _make_ctx_stats 2031 "$YESTERDAY_MS" 40 0.01 40 2
  run _source_ai_savings "
    aisavings_load_ctx_stats 0
    echo \"\$CTX_REDUCTION_PCT\"
  "
  [ "$status" -eq 0 ]
  # Moyenne de 60 et 40 = 50
  [[ "$output" == *"50"* ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. aisavings_load_ctx_stats — filtrage par période
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_load_ctx_stats 1 : ne prend que les sessions du jour" {
  _make_ctx_stats 3001 "$TODAY_START_MS" 5000 0.03 55 3   # aujourd'hui
  _make_ctx_stats 3002 "$YESTERDAY_MS"   9000 0.05 70 5   # hier — doit être exclu
  run _source_ai_savings "
    aisavings_load_ctx_stats 1
    echo \"tokens:\$CTX_TOKENS_SAVED sessions:\$CTX_SESSIONS_COUNT\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"tokens:5000"* ]]
  [[ "$output" == *"sessions:1"* ]]
}

@test "aisavings_load_ctx_stats 1 : retourne 1 si aucune session du jour" {
  _make_ctx_stats 3010 "$YESTERDAY_MS" 5000 0.03 55 3   # hier uniquement
  run _source_ai_savings "aisavings_load_ctx_stats 1"
  [ "$status" -eq 1 ]
}

@test "aisavings_load_ctx_stats 1 : exporte CTX_PERIOD_LABEL=(aujourd'hui)" {
  _make_ctx_stats 3020 "$TODAY_START_MS" 5000 0.03 55 3
  run _source_ai_savings "
    aisavings_load_ctx_stats 1
    echo \"\$CTX_PERIOD_LABEL\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"(aujourd'hui)"* ]]
}

@test "aisavings_load_ctx_stats 7 : inclut sessions de la semaine" {
  _make_ctx_stats 4001 "$NOW_MS"       4000 0.02 50 2   # aujourd'hui — inclus
  _make_ctx_stats 4002 "$YESTERDAY_MS" 3000 0.01 40 1   # hier — inclus
  _make_ctx_stats 4003 "$OLD_MS"       9000 0.05 70 5   # il y a 60j — exclu
  run _source_ai_savings "
    aisavings_load_ctx_stats 7
    echo \"tokens:\$CTX_TOKENS_SAVED sessions:\$CTX_SESSIONS_COUNT\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"tokens:7000"* ]]
  [[ "$output" == *"sessions:2"* ]]
}

@test "aisavings_load_ctx_stats 7 : exporte CTX_PERIOD_LABEL=(7 derniers jours)" {
  _make_ctx_stats 4010 "$NOW_MS" 4000 0.02 50 2
  run _source_ai_savings "
    aisavings_load_ctx_stats 7
    echo \"\$CTX_PERIOD_LABEL\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"(7 derniers jours)"* ]]
}

@test "aisavings_load_ctx_stats 30 : exporte CTX_PERIOD_LABEL=(30 derniers jours)" {
  _make_ctx_stats 4020 "$YESTERDAY_MS" 4000 0.02 50 2
  run _source_ai_savings "
    aisavings_load_ctx_stats 30
    echo \"\$CTX_PERIOD_LABEL\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"(30 derniers jours)"* ]]
}

@test "aisavings_load_ctx_stats 7 : retourne 1 si toutes les sessions hors période" {
  _make_ctx_stats 4030 "$OLD_MS" 5000 0.03 55 3   # il y a 60j — exclu
  run _source_ai_savings "aisavings_load_ctx_stats 7"
  [ "$status" -eq 1 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# E. aisavings_load_rtk_stats
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_load_rtk_stats : retourne 1 si rtk absent du PATH" {
  run bash -c "
    export PATH='/usr/bin:/bin'   # PATH minimal sans rtk
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_rtk_stats
  "
  [ "$status" -eq 1 ]
}

@test "aisavings_load_rtk_stats : parse correctement le JSON RTK" {
  # Créer un mock rtk qui retourne un JSON valide
  local mock_dir="$TEST_DIR/mock-bin"
  mkdir -p "$mock_dir"
  cat > "$mock_dir/rtk" <<'MOCK'
#!/bin/bash
echo '{"summary":{"total_commands":7221,"total_input":7411379,"total_output":5810198,"total_saved":1630415,"avg_savings_pct":21.998807509371737,"total_time_ms":3221938,"avg_time_ms":446}}'
MOCK
  chmod +x "$mock_dir/rtk"

  run bash -c "
    export PATH='$mock_dir:$PATH'
    export CTX_STATS_DIR='$CTX_STATS_TEST_DIR'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_rtk_stats
    echo \"saved:\$RTK_TOTAL_SAVED pct:\$RTK_AVG_SAVINGS_PCT cmds:\$RTK_TOTAL_COMMANDS available:\$RTK_AVAILABLE\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"saved:1630415"* ]]
  [[ "$output" == *"cmds:7221"* ]]
  [[ "$output" == *"available:1"* ]]
}

@test "aisavings_load_rtk_stats : RTK_AVG_SAVINGS_PCT formaté avec 1 décimale" {
  local mock_dir="$TEST_DIR/mock-bin-pct"
  mkdir -p "$mock_dir"
  cat > "$mock_dir/rtk" <<'MOCK'
#!/bin/bash
echo '{"summary":{"total_commands":100,"total_saved":50000,"avg_savings_pct":21.998807509371737}}'
MOCK
  chmod +x "$mock_dir/rtk"

  run bash -c "
    export PATH='$mock_dir:$PATH'
    export CTX_STATS_DIR='$CTX_STATS_TEST_DIR'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_rtk_stats
    echo \"\$RTK_AVG_SAVINGS_PCT\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"22.0"* ]]
}

@test "aisavings_load_rtk_stats : retourne 1 si JSON invalide" {
  local mock_dir="$TEST_DIR/mock-bin-bad"
  mkdir -p "$mock_dir"
  cat > "$mock_dir/rtk" <<'MOCK'
#!/bin/bash
echo 'not json'
MOCK
  chmod +x "$mock_dir/rtk"

  run bash -c "
    export PATH='$mock_dir:$PATH'
    export CTX_STATS_DIR='$CTX_STATS_TEST_DIR'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_rtk_stats
  "
  [ "$status" -eq 1 ]
}

@test "aisavings_load_rtk_stats : retourne 1 si total_saved=0 et total_commands=0" {
  local mock_dir="$TEST_DIR/mock-bin-zero"
  mkdir -p "$mock_dir"
  cat > "$mock_dir/rtk" <<'MOCK'
#!/bin/bash
echo '{"summary":{"total_commands":0,"total_saved":0,"avg_savings_pct":0}}'
MOCK
  chmod +x "$mock_dir/rtk"

  run bash -c "
    export PATH='$mock_dir:$PATH'
    export CTX_STATS_DIR='$CTX_STATS_TEST_DIR'
    source '$COMMON_SH'
    source '$LIB_AI_SAVINGS'
    aisavings_load_rtk_stats
  "
  [ "$status" -eq 1 ]
}

# ══════════════════════════════════════════════════════════════════════════════
# F. Tests d'intégration croisés
# ══════════════════════════════════════════════════════════════════════════════

@test "aisavings_format_tokens : cohérence avec les valeurs RTK réelles" {
  # 1630415 tokens → 1.6M
  run _source_ai_savings "aisavings_format_tokens 1630415"
  [ "$status" -eq 0 ]
  [ "$output" = "1.6M" ]
}

@test "aisavings_load_ctx_stats : CTX_AVAILABLE vide si aucune donnée" {
  # Répertoire vide → CTX_AVAILABLE non défini/vide
  run _source_ai_savings "
    aisavings_load_ctx_stats 0 || true
    echo \"available:'\${CTX_AVAILABLE}'\"
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"available:''"* ]]
}
