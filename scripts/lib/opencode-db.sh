#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# opencode-db.sh — Bibliothèque de requêtes SQLite read-only sur la base
#                  OpenCode (~/.local/share/opencode/opencode.db)
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/opencode-db.sh"
#   ocdb_check_available || { echo "sqlite3 requis"; exit 1; }
#   ocdb_aggregate 7          # agrège les 7 derniers jours
#   echo "$OCDB_TOTAL_COST"   # affiche le coût total
#
# Variables exportées par ocdb_aggregate() :
#   OCDB_TOTAL_COST          ex: "12.45"
#   OCDB_TOTAL_SESSIONS      ex: "47"
#   OCDB_TOKENS_INPUT        ex: "1234567"
#   OCDB_TOKENS_OUTPUT       ex: "345678"
#   OCDB_TOKENS_CACHE_READ   ex: "890123"
#   OCDB_TOKENS_CACHE_WRITE  ex: "234567"
#   OCDB_CACHE_HIT_RATE      ex: "78.3"  (pourcentage, 0–100)
#   OCDB_TOP_PROJECTS        tableau "dir|cost"
#   OCDB_TOP_AGENTS          tableau "agent|cost"
#   OCDB_TOP_MODELS          tableau "model|cost"
#   OCDB_RECENT_SESSIONS     tableau "slug|title|agent|cost|date"
#
# Notes :
#   - Toutes les requêtes sont en LECTURE SEULE (mode ro ou PRAGMA query_only)
#   - Si sqlite3 est absent ou la db inaccessible, les fonctions retournent
#     des valeurs vides / zéro sans provoquer d'erreur bloquante
#   - Supporte XDG_DATA_HOME pour la localisation de la db
# ─────────────────────────────────────────────────────────────────────────────

# ─────────────────────────────────────────
# CHEMINS
# ─────────────────────────────────────────

# Chemin vers la base SQLite OpenCode
# Priorité : _OCDB_FILE (test) > XDG_DATA_HOME > défaut ~/.local/share/opencode
_ocdb_resolve_path() {
  if [ -n "${_OCDB_FILE:-}" ]; then
    echo "$_OCDB_FILE"
    return
  fi
  local data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
  echo "${data_home}/opencode/opencode.db"
}

# ─────────────────────────────────────────
# VÉRIFICATIONS
# ─────────────────────────────────────────

# Retourne 0 si sqlite3 est disponible ET la db accessible
# Retourne 1 sinon (avec message d'erreur sur stderr)
ocdb_check_available() {
  if ! command -v sqlite3 &>/dev/null; then
    echo "sqlite3 non trouvé — requis pour oc metrics / oc dashboard" >&2
    echo "  macOS  : sqlite3 est natif, vérifier /usr/bin/sqlite3" >&2
    echo "  Linux  : sudo apt-get install sqlite3" >&2
    return 1
  fi
  local db_path
  db_path=$(_ocdb_resolve_path)
  if [ ! -f "$db_path" ]; then
    echo "Base OpenCode introuvable : $db_path" >&2
    echo "  Lancer OpenCode au moins une fois pour initialiser la base." >&2
    return 1
  fi
  return 0
}

# Retourne le chemin vers la db OpenCode
ocdb_get_db_path() {
  _ocdb_resolve_path
}

# ─────────────────────────────────────────
# REQUÊTES INTERNES
# ─────────────────────────────────────────

# Exécute une requête SQL en lecture seule sur la db OpenCode
# Usage : _ocdb_query "SELECT ..."
# Retourne les résultats sur stdout (séparateur | pour les colonnes)
_ocdb_query() {
  local sql="$1"
  local db_path
  db_path=$(_ocdb_resolve_path)
  sqlite3 -separator "|" "file:${db_path}?mode=ro" "$sql" 2>/dev/null || true
}

# Calcule le timestamp Unix pour "il y a N jours"
# Usage : _ocdb_since_ts 7   → retourne epoch en millisecondes
_ocdb_since_ts() {
  local days="${1:-7}"
  local now_s
  now_s=$(date +%s)
  # La db stocke time_created en millisecondes (epoch * 1000)
  echo $(( (now_s - days * 86400) * 1000 ))
}

# ─────────────────────────────────────────
# FONCTIONS PUBLIQUES — STATISTIQUES
# ─────────────────────────────────────────

# Coût total sur N jours (en USD, 2 décimales)
# Usage : ocdb_total_cost [days=7]
ocdb_total_cost() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local result
  result=$(_ocdb_query "
    SELECT COALESCE(ROUND(SUM(cost), 4), 0)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts};
  ")
  echo "${result:-0}"
}

# Nombre total de sessions sur N jours
# Usage : ocdb_sessions_count [days=7]
ocdb_sessions_count() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local result
  result=$(_ocdb_query "
    SELECT COUNT(*)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts};
  ")
  echo "${result:-0}"
}

# Résumé des tokens sur N jours
# Affiche : input|output|cache_read|cache_write (sur une ligne, séparés par |)
# Usage : ocdb_tokens_summary [days=7]
ocdb_tokens_summary() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local result
  result=$(_ocdb_query "
    SELECT
      COALESCE(SUM(tokens_input), 0),
      COALESCE(SUM(tokens_output), 0),
      COALESCE(SUM(tokens_cache_read), 0),
      COALESCE(SUM(tokens_cache_write), 0)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts};
  ")
  echo "${result:-0|0|0|0}"
}

# Cache hit rate en pourcentage sur N jours (0–100, 1 décimale)
# Formule : cache_read / (input + cache_read) * 100
# Usage : ocdb_cache_hit_rate [days=7]
ocdb_cache_hit_rate() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local row
  row=$(_ocdb_query "
    SELECT
      COALESCE(SUM(tokens_cache_read), 0),
      COALESCE(SUM(tokens_input), 0)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts};
  ")
  local cache_read input_tokens
  cache_read=$(echo "$row" | cut -d'|' -f1)
  input_tokens=$(echo "$row" | cut -d'|' -f2)
  cache_read="${cache_read:-0}"
  input_tokens="${input_tokens:-0}"

  if [ "$((cache_read + input_tokens))" -eq 0 ] 2>/dev/null; then
    echo "0.0"
    return
  fi
  # LC_ALL=C force le séparateur décimal "." indépendamment de la locale
  LC_ALL=C awk "BEGIN { total=$cache_read + $input_tokens; if(total>0) printf \"%.1f\", $cache_read/total*100; else print \"0.0\" }"
}

# Économies estimées du cache en USD sur N jours
# Hypothèse : prix cache_read ≈ 10% du prix input → économie ≈ 90% * cache_read * prix_input
# Usage : ocdb_cache_savings [days=7]
ocdb_cache_savings() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local row
  row=$(_ocdb_query "
    SELECT
      COALESCE(SUM(tokens_cache_read), 0),
      COALESCE(SUM(tokens_input), 0),
      COALESCE(SUM(cost), 0)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts};
  ")
  local cache_read input_tokens total_cost
  cache_read=$(echo "$row" | cut -d'|' -f1)
  input_tokens=$(echo "$row" | cut -d'|' -f2)
  total_cost=$(echo "$row" | cut -d'|' -f3)
  cache_read="${cache_read:-0}"
  input_tokens="${input_tokens:-0}"
  total_cost="${total_cost:-0}"

  # Estimation conservatrice : 
  # - On déduit un prix moyen input depuis (coût total / (input + cache_read))
  # - Économie = cache_read_tokens * prix_input * 0.9 (car cache_read coûte ~10% du prix input)
  LC_ALL=C awk "BEGIN {
    total_tokens = $input_tokens + $cache_read;
    if (total_tokens > 0 && $total_cost > 0) {
      price_per_token = $total_cost / total_tokens;
      savings = $cache_read * price_per_token * 0.9;
      printf \"%.2f\", savings;
    } else {
      print \"0.00\";
    }
  }"
}

# Coût par projet (répertoire) sur N jours, trié par coût décroissant
# Retourne un tableau de lignes "directory|cost"
# Usage : ocdb_cost_by_project [days=7] [limit=10]
ocdb_cost_by_project() {
  local days="${1:-7}"
  local limit="${2:-10}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      COALESCE(directory, 'unknown'),
      ROUND(SUM(cost), 4)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts}
    GROUP BY directory
    ORDER BY SUM(cost) DESC
    LIMIT ${limit};
  "
}

# Coût par agent sur N jours, trié par coût décroissant
# Retourne des lignes "agent|cost"
# Usage : ocdb_cost_by_agent [days=7] [limit=10]
ocdb_cost_by_agent() {
  local days="${1:-7}"
  local limit="${2:-10}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      COALESCE(agent, 'unknown'),
      ROUND(SUM(cost), 4)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts}
    GROUP BY agent
    ORDER BY SUM(cost) DESC
    LIMIT ${limit};
  "
}

# Coût par modèle sur N jours, trié par coût décroissant
# Retourne des lignes "model|cost"
# Note : le champ model peut être un JSON {"id":"...","providerID":"..."} — extrait l'id si possible
# Usage : ocdb_cost_by_model [days=7] [limit=10]
ocdb_cost_by_model() {
  local days="${1:-7}"
  local limit="${2:-10}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  # Récupérer les données brutes puis normaliser le nom du modèle côté shell
  _ocdb_query "
    SELECT
      COALESCE(model, 'unknown'),
      ROUND(SUM(cost), 4)
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts}
    GROUP BY model
    ORDER BY SUM(cost) DESC
    LIMIT ${limit};
  " | while IFS='|' read -r model cost; do
    # Extraire l'id si c'est un JSON {"id":"model-name","providerID":"..."}
    local clean_model="$model"
    if [[ "$model" == "{"* ]]; then
      clean_model=$(echo "$model" | sed 's/.*"id":"\([^"]*\)".*/\1/' 2>/dev/null || echo "$model")
    fi
    echo "${clean_model}|${cost}"
  done
}

# N dernières sessions (hors sous-sessions), triées par date décroissante
# Retourne des lignes "slug|title|agent|cost|timestamp_ms"
# Usage : ocdb_recent_sessions [limit=5] [days=7]
ocdb_recent_sessions() {
  local limit="${1:-5}"
  local days="${2:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      slug,
      REPLACE(REPLACE(title, '|', '/'), CHAR(10), ' '),
      COALESCE(agent, ''),
      ROUND(cost, 4),
      time_created
    FROM session
    WHERE parent_id IS NULL
      AND time_created >= ${since_ts}
    ORDER BY time_created DESC
    LIMIT ${limit};
  "
}

# ─────────────────────────────────────────
# FORMATAGE
# ─────────────────────────────────────────

# Formate un timestamp en millisecondes vers une date lisible
# Usage : ocdb_format_date 1749600000000
ocdb_format_date() {
  local ts_ms="${1:-0}"
  local ts_s=$(( ts_ms / 1000 ))
  if [ "$ts_s" -eq 0 ] 2>/dev/null; then
    echo "--"
    return
  fi
  # date -d fonctionne sur Linux, date -r sur macOS
  if date -r "$ts_s" "+%d/%m %H:%M" 2>/dev/null; then
    return
  fi
  date -d "@${ts_s}" "+%d/%m %H:%M" 2>/dev/null || echo "--"
}

# Formate un nombre de tokens en format lisible (K, M)
# Usage : ocdb_format_tokens 1234567
ocdb_format_tokens() {
  local n="${1:-0}"
  n="${n%%.*}"  # enlever décimales éventuelles
  if [ "${n:-0}" -eq 0 ] 2>/dev/null; then
    echo "0"
    return
  fi
  # LANG=C force le séparateur décimal en "." (évite "," selon la locale)
  LC_ALL=C awk "BEGIN {
    n = $n;
    if (n >= 1000000) printf \"%.1fM\", n/1000000;
    else if (n >= 1000) printf \"%.1fK\", n/1000;
    else printf \"%d\", n;
  }"
}

# ─────────────────────────────────────────
# AGRÉGATION PRINCIPALE
# ─────────────────────────────────────────

# Agrège toutes les métriques et exporte les variables globales
# Usage : ocdb_aggregate [days=7]
#
# Variables exportées :
#   OCDB_TOTAL_COST, OCDB_TOTAL_SESSIONS,
#   OCDB_TOKENS_INPUT, OCDB_TOKENS_OUTPUT,
#   OCDB_TOKENS_CACHE_READ, OCDB_TOKENS_CACHE_WRITE,
#   OCDB_CACHE_HIT_RATE, OCDB_CACHE_SAVINGS,
#   OCDB_TOP_PROJECTS, OCDB_TOP_AGENTS, OCDB_TOP_MODELS,
#   OCDB_RECENT_SESSIONS
ocdb_aggregate() {
  local days="${1:-7}"

  # Initialisation à zéro
  OCDB_TOTAL_COST="0"
  OCDB_TOTAL_SESSIONS="0"
  OCDB_TOKENS_INPUT="0"
  OCDB_TOKENS_OUTPUT="0"
  OCDB_TOKENS_CACHE_READ="0"
  OCDB_TOKENS_CACHE_WRITE="0"
  OCDB_CACHE_HIT_RATE="0.0"
  OCDB_CACHE_SAVINGS="0.00"
  OCDB_TOP_PROJECTS=()
  OCDB_TOP_AGENTS=()
  OCDB_TOP_MODELS=()
  OCDB_RECENT_SESSIONS=()

  # Vérifier que sqlite3 + db sont disponibles
  if ! ocdb_check_available 2>/dev/null; then
    return 1
  fi

  # Coût et sessions totaux
  OCDB_TOTAL_COST=$(ocdb_total_cost "$days")
  OCDB_TOTAL_SESSIONS=$(ocdb_sessions_count "$days")

  # Tokens
  local token_row
  token_row=$(ocdb_tokens_summary "$days")
  OCDB_TOKENS_INPUT=$(echo "$token_row" | cut -d'|' -f1)
  OCDB_TOKENS_OUTPUT=$(echo "$token_row" | cut -d'|' -f2)
  OCDB_TOKENS_CACHE_READ=$(echo "$token_row" | cut -d'|' -f3)
  OCDB_TOKENS_CACHE_WRITE=$(echo "$token_row" | cut -d'|' -f4)

  # Cache hit rate + économies
  OCDB_CACHE_HIT_RATE=$(ocdb_cache_hit_rate "$days")
  OCDB_CACHE_SAVINGS=$(ocdb_cache_savings "$days")

  # Top projets
  while IFS= read -r line; do
    [ -n "$line" ] && OCDB_TOP_PROJECTS+=("$line")
  done < <(ocdb_cost_by_project "$days" 10)

  # Top agents
  while IFS= read -r line; do
    [ -n "$line" ] && OCDB_TOP_AGENTS+=("$line")
  done < <(ocdb_cost_by_agent "$days" 5)

  # Top modèles
  while IFS= read -r line; do
    [ -n "$line" ] && OCDB_TOP_MODELS+=("$line")
  done < <(ocdb_cost_by_model "$days" 5)

  # Sessions récentes
  while IFS= read -r line; do
    [ -n "$line" ] && OCDB_RECENT_SESSIONS+=("$line")
  done < <(ocdb_recent_sessions 5 "$days")

  return 0
}

# ─────────────────────────────────────────
# FONCTIONS TOOL-USE (table part)
# ─────────────────────────────────────────

# Statistiques des tool calls par nom d'outil sur N jours
# Retourne des lignes "tool|count" triées par count décroissant
# Usage : ocdb_tool_stats [days=30] [limit=20]
ocdb_tool_stats() {
  local days="${1:-30}"
  local limit="${2:-20}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      json_extract(p.data, '$.tool')  AS tool_name,
      COUNT(*)                        AS cnt
    FROM part p
    JOIN session s ON p.session_id = s.id
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND json_extract(p.data, '$.type') = 'tool'
      AND tool_name IS NOT NULL
    GROUP BY tool_name
    ORDER BY cnt DESC
    LIMIT ${limit};
  "
}

# Décompte d'un outil spécifique sur N jours
# Usage : ocdb_tool_count "edit" [days=30]
ocdb_tool_count() {
  local tool="${1:?tool requis}"
  local days="${2:-30}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local result
  result=$(_ocdb_query "
    SELECT COUNT(*)
    FROM part p
    JOIN session s ON p.session_id = s.id
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') = '${tool}';
  ")
  echo "${result:-0}"
}

# Taux d'erreurs sur les tool calls sur N jours (en %)
# Usage : ocdb_tool_error_rate [days=30]
ocdb_tool_error_rate() {
  local days="${1:-30}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local row
  row=$(_ocdb_query "
    SELECT
      COUNT(*) AS total,
      SUM(CASE WHEN json_extract(p.data, '$.state.status') = 'error' THEN 1 ELSE 0 END) AS errors
    FROM part p
    JOIN session s ON p.session_id = s.id
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND json_extract(p.data, '$.type') = 'tool';
  ")
  local total errors
  total=$(echo "$row" | cut -d'|' -f1)
  errors=$(echo "$row" | cut -d'|' -f2)
  total="${total:-0}"
  errors="${errors:-0}"
  if [ "$total" -eq 0 ] 2>/dev/null; then
    echo "0.0"
    return
  fi
  LC_ALL=C awk "BEGIN { printf \"%.1f\", $errors/$total*100 }"
}

# Répartition des sessions par catégorie d'activité sur N jours
# Catégories déterministes basées sur les tool-use patterns :
#   code        : sessions avec edit ou write
#   exploration : sessions avec read/grep/glob mais sans edit/write
#   planification : sessions avec task > 2 ou agent contient orchestrator
#   review      : sessions avec agent contenant reviewer
#   debug       : sessions avec agent contenant debugger
#   conversation : sessions sans aucun tool call
#
# Retourne des lignes "category|session_count|total_cost"
# Usage : ocdb_activity_breakdown [days=7]
ocdb_activity_breakdown() {
  local days="${1:-7}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")

  _ocdb_query "
    WITH session_tools AS (
      SELECT
        s.id                                                  AS session_id,
        s.cost                                                AS cost,
        s.agent                                               AS agent,
        COUNT(CASE WHEN json_extract(p.data, '$.tool') IN ('edit','write') THEN 1 END)       AS edit_count,
        COUNT(CASE WHEN json_extract(p.data, '$.tool') IN ('read','grep','glob') THEN 1 END) AS read_count,
        COUNT(CASE WHEN json_extract(p.data, '$.tool') = 'task' THEN 1 END)                  AS task_count,
        COUNT(CASE WHEN json_extract(p.data, '$.type') = 'tool' THEN 1 END)                  AS tool_total
      FROM session s
      LEFT JOIN part p ON p.session_id = s.id
      WHERE s.parent_id IS NULL
        AND s.time_created >= ${since_ts}
      GROUP BY s.id
    ),
    categorized AS (
      SELECT
        CASE
          WHEN agent LIKE '%reviewer%'                              THEN 'review'
          WHEN agent LIKE '%debugger%'                              THEN 'debug'
          WHEN agent LIKE '%orchestrator%' OR task_count > 2        THEN 'planification'
          WHEN edit_count > 0                                        THEN 'code'
          WHEN read_count > 0 AND edit_count = 0                   THEN 'exploration'
          WHEN tool_total = 0                                        THEN 'conversation'
          ELSE 'autre'
        END AS category,
        cost
      FROM session_tools
    )
    SELECT
      category,
      COUNT(*)            AS session_count,
      ROUND(SUM(cost), 4) AS total_cost
    FROM categorized
    GROUP BY category
    ORDER BY total_cost DESC;
  "
}

# Sessions coûteuses sans aucun edit/write sur N jours
# Retourne des lignes "slug|title|agent|cost"
# Usage : ocdb_sessions_no_edit [days=30] [min_cost=1.0]
ocdb_sessions_no_edit() {
  local days="${1:-30}"
  local min_cost="${2:-1.0}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      s.slug,
      REPLACE(REPLACE(s.title, '|', '/'), CHAR(10), ' '),
      COALESCE(s.agent, ''),
      ROUND(s.cost, 4)
    FROM session s
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND s.cost >= ${min_cost}
      AND NOT EXISTS (
        SELECT 1 FROM part p
        WHERE p.session_id = s.id
          AND json_extract(p.data, '$.type') = 'tool'
          AND json_extract(p.data, '$.tool') IN ('edit', 'write')
      )
    ORDER BY s.cost DESC
    LIMIT 10;
  "
}

# Ratio Read/Edit moyen sur N jours (idéal >= 2.0)
# Usage : ocdb_avg_read_edit_ratio [days=30]
ocdb_avg_read_edit_ratio() {
  local days="${1:-30}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local row
  row=$(_ocdb_query "
    SELECT
      SUM(read_cnt) AS total_read,
      SUM(edit_cnt) AS total_edit
    FROM (
      SELECT
        s.id,
        COUNT(CASE WHEN json_extract(p.data, '$.tool') IN ('read','grep','glob') THEN 1 END) AS read_cnt,
        COUNT(CASE WHEN json_extract(p.data, '$.tool') IN ('edit','write') THEN 1 END)       AS edit_cnt
      FROM session s
      JOIN part p ON p.session_id = s.id
      WHERE s.parent_id IS NULL
        AND s.time_created >= ${since_ts}
      GROUP BY s.id
      HAVING edit_cnt > 0
    );
  ")
  local total_read total_edit
  total_read=$(echo "$row" | cut -d'|' -f1)
  total_edit=$(echo "$row" | cut -d'|' -f2)
  total_read="${total_read:-0}"
  total_edit="${total_edit:-0}"
  if [ "$total_edit" -eq 0 ] 2>/dev/null; then
    echo "0.0"
    return
  fi
  LC_ALL=C awk "BEGIN { printf \"%.1f\", $total_read/$total_edit }"
}

# Nombre de sessions avec délégation lourde (tool=task > 40% des tools) sur N jours
# Usage : ocdb_sessions_heavy_delegation [days=30]
ocdb_sessions_heavy_delegation() {
  local days="${1:-30}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  local result
  result=$(_ocdb_query "
    SELECT COUNT(*) FROM (
      SELECT s.id
      FROM session s
      JOIN part p ON p.session_id = s.id
      WHERE s.parent_id IS NULL
        AND s.time_created >= ${since_ts}
        AND json_extract(p.data, '$.type') = 'tool'
      GROUP BY s.id
      HAVING
        COUNT(CASE WHEN json_extract(p.data, '$.tool') = 'task' THEN 1 END) * 100.0
        / COUNT(*) > 40
    );
  ")
  echo "${result:-0}"
}

# Fichiers re-lus >= threshold fois dans une même session sur N jours
# Retourne des lignes "filename|read_count"
# Usage : ocdb_repeated_reads [days=30] [threshold=5]
ocdb_repeated_reads() {
  local days="${1:-30}"
  local threshold="${2:-5}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")
  _ocdb_query "
    SELECT
      json_extract(p.data, '$.state.input.path') AS file_path,
      COUNT(*)                                    AS read_count
    FROM part p
    JOIN session s ON p.session_id = s.id
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') = 'read'
      AND file_path IS NOT NULL
    GROUP BY p.session_id, file_path
    HAVING read_count >= ${threshold}
    ORDER BY read_count DESC
    LIMIT 10;
  " | while IFS='|' read -r fpath cnt; do
    local short
    short=$(basename "$fpath" 2>/dev/null || echo "$fpath")
    echo "${short}|${cnt}"
  done
}

# Détecte les MCP servers déployés mais inutilisés sur N jours
# Compare les tools *-mcp_* utilisés vs les servers présents dans servers/
# Retourne les noms de MCP servers inutilisés (un par ligne)
# Usage : ocdb_unused_mcp [days=30]
ocdb_unused_mcp() {
  local days="${1:-30}"
  local since_ts
  since_ts=$(_ocdb_since_ts "$days")

  local used_tools
  used_tools=$(_ocdb_query "
    SELECT DISTINCT json_extract(p.data, '$.tool') AS tool_name
    FROM part p
    JOIN session s ON p.session_id = s.id
    WHERE s.parent_id IS NULL
      AND s.time_created >= ${since_ts}
      AND json_extract(p.data, '$.type') = 'tool'
      AND tool_name LIKE '%-mcp_%';
  ")

  local hub_dir="${HUB_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
  local servers_dir="${hub_dir}/servers"

  [ -d "$servers_dir" ] || return

  for server_dir in "$servers_dir"/*/; do
    local server_name
    server_name=$(basename "$server_dir")
    local prefix="${server_name%%-mcp}-mcp"
    if ! echo "$used_tools" | grep -q "^${prefix}_" 2>/dev/null; then
      echo "$server_name"
    fi
  done
}
