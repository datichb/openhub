#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# ai-savings.sh — Économies IA : context-mode + RTK
# ─────────────────────────────────────────────────────────────────────────────
#
# Fournit deux fonctions pour lire les statistiques d'économies des plugins
# context-mode et RTK, utilisées par oc dashboard et oc metrics.
#
# Usage :
#   source "$LIB_DIR/ai-savings.sh"
#   aisavings_load_ctx_stats [since_days]   # 0 = lifetime
#   aisavings_load_rtk_stats
#
# Variables exportées par aisavings_load_ctx_stats :
#   CTX_TOKENS_SAVED      — tokens économisés (entier)
#   CTX_DOLLARS_SAVED     — dollars économisés (ex: "0.04")
#   CTX_REDUCTION_PCT     — réduction de contexte en % (entier)
#   CTX_SESSIONS_COUNT    — nombre de sessions agrégées
#   CTX_PERIOD_LABEL      — libellé humain de la période
#   CTX_AVAILABLE         — "1" si des données existent, "" sinon
#
# Variables exportées par aisavings_load_rtk_stats :
#   RTK_TOTAL_SAVED       — tokens économisés (entier)
#   RTK_AVG_SAVINGS_PCT   — pourcentage moyen d'économies (ex: "22.0")
#   RTK_TOTAL_COMMANDS    — nombre de commandes réécrites
#   RTK_AVAILABLE         — "1" si RTK est disponible et a des données, "" sinon
#
# ─────────────────────────────────────────────────────────────────────────────

# Répertoire des stats context-mode (surcharger en test via CTX_STATS_DIR)
_CTX_STATS_DIR="${CTX_STATS_DIR:-${HOME}/.claude/context-mode/sessions}"

# ─────────────────────────────────────────────────────────────────────────────
# aisavings_format_tokens <n>
# Formate un nombre de tokens en format lisible : 1630415 → "1.6M", 7931 → "7.9K"
# ─────────────────────────────────────────────────────────────────────────────
aisavings_format_tokens() {
  local n="${1:-0}"
  # Supprimer les éventuels espaces
  n="${n// /}"
  # Valeur non numérique → retourner tel quel
  if ! echo "$n" | grep -qE '^[0-9]+$'; then
    echo "$n"
    return
  fi
  if [ "$n" -ge 1000000 ]; then
    LC_ALL=C awk "BEGIN { printf \"%.1fM\", $n / 1000000 }"
  elif [ "$n" -ge 1000 ]; then
    LC_ALL=C awk "BEGIN { printf \"%.1fK\", $n / 1000 }"
  else
    echo "$n"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# aisavings_load_ctx_stats [since_days]
#
# Lit et agrège les fichiers stats-pid-*.json de context-mode.
#   since_days=0  → lifetime (tous les fichiers)
#   since_days=1  → sessions démarrées aujourd'hui
#   since_days=7  → sessions des 7 derniers jours
#   since_days=30 → sessions des 30 derniers jours
#
# Retourne 1 si aucune donnée disponible.
# ─────────────────────────────────────────────────────────────────────────────
aisavings_load_ctx_stats() {
  local since_days="${1:-0}"

  # Réinitialiser les variables exportées
  CTX_TOKENS_SAVED=0
  CTX_DOLLARS_SAVED="0.00"
  CTX_REDUCTION_PCT=0
  CTX_SESSIONS_COUNT=0
  CTX_PERIOD_LABEL="(lifetime)"
  CTX_AVAILABLE=""

  # Calculer le libellé de période
  case "$since_days" in
    0)  CTX_PERIOD_LABEL="(lifetime)" ;;
    1)  CTX_PERIOD_LABEL="(aujourd'hui)" ;;
    7)  CTX_PERIOD_LABEL="(7 derniers jours)" ;;
    30) CTX_PERIOD_LABEL="(30 derniers jours)" ;;
    *)  CTX_PERIOD_LABEL="(${since_days} derniers jours)" ;;
  esac

  # Vérifier la disponibilité de python3 et du répertoire
  if ! command -v python3 &>/dev/null; then
    return 1
  fi
  if [ ! -d "$_CTX_STATS_DIR" ]; then
    return 1
  fi

  # Calculer le seuil en millisecondes (epoch ms)
  # Pour since_days=1 : minuit du jour courant (date calendaire, pas 24h glissantes)
  # Pour since_days>1 : fenêtre glissante (now - N*86400s)
  local threshold_ms=0
  if [ "$since_days" -gt 0 ]; then
    if [ "$since_days" -eq 1 ]; then
      # BSD date (macOS) d'abord, python3 en fallback
      local midnight_s
      midnight_s=$(date -v0H -v0M -v0S +%s 2>/dev/null) || \
      midnight_s=$(python3 -c "
from datetime import datetime, date, time as dtime
print(int(datetime.combine(date.today(), dtime.min).timestamp()))
" 2>/dev/null) || midnight_s=$(( $(date +%s) - 86400 ))
      threshold_ms=$(( midnight_s * 1000 ))
    else
      threshold_ms=$(python3 -c "
import time
print(int((time.time() - ${since_days} * 86400) * 1000))
" 2>/dev/null || echo "0")
    fi
  fi

  # Agréger via python3 (évite les problèmes de parsing JSON en bash pur)
  local result
  result=$(python3 - "$_CTX_STATS_DIR" "$threshold_ms" <<'PYEOF' 2>/dev/null
import json, sys, pathlib, glob

stats_dir = sys.argv[1]
threshold_ms = int(sys.argv[2])

total_tokens = 0
total_dollars = 0.0
total_reduction_sum = 0
sessions_with_reduction = 0
session_count = 0

files = sorted(pathlib.Path(stats_dir).glob("stats-pid-*.json"))

for f in files:
    try:
        data = json.loads(f.read_text())
    except Exception:
        continue

    session_start = data.get("session_start", 0)
    if threshold_ms > 0 and session_start < threshold_ms:
        continue

    tokens = data.get("tokens_saved", 0)
    dollars = data.get("dollars_saved_session", 0)
    reduction = data.get("reduction_pct", 0)

    # Ne compter que les sessions ayant une activité réelle
    if data.get("total_calls", 0) == 0:
        continue

    total_tokens += tokens
    total_dollars += dollars
    if reduction > 0:
        total_reduction_sum += reduction
        sessions_with_reduction += 1
    session_count += 1

avg_reduction = int(total_reduction_sum / sessions_with_reduction) if sessions_with_reduction > 0 else 0

print(f"{total_tokens}|{total_dollars:.2f}|{avg_reduction}|{session_count}")
PYEOF
)

  if [ -z "$result" ]; then
    return 1
  fi

  local tokens dollars reduction sessions
  IFS='|' read -r tokens dollars reduction sessions <<< "$result"

  if [ "${tokens:-0}" -eq 0 ] && [ "${sessions:-0}" -eq 0 ]; then
    return 1
  fi

  CTX_TOKENS_SAVED="${tokens:-0}"
  CTX_DOLLARS_SAVED="${dollars:-0.00}"
  CTX_REDUCTION_PCT="${reduction:-0}"
  CTX_SESSIONS_COUNT="${sessions:-0}"
  CTX_AVAILABLE="1"

  export CTX_TOKENS_SAVED CTX_DOLLARS_SAVED CTX_REDUCTION_PCT CTX_SESSIONS_COUNT CTX_PERIOD_LABEL CTX_AVAILABLE
  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# aisavings_load_rtk_stats
#
# Lit les statistiques RTK via `rtk gain`.
# Essaie d'abord --project (scope projet, RTK 0.42+), puis fallback global.
# Toujours libellé "(global)" — RTK ne filtre pas par période.
#
# Retourne 1 si RTK n'est pas disponible ou si le JSON est invalide.
# ─────────────────────────────────────────────────────────────────────────────
aisavings_load_rtk_stats() {
  RTK_TOTAL_SAVED=0
  RTK_AVG_SAVINGS_PCT="0.0"
  RTK_TOTAL_COMMANDS=0
  RTK_AVAILABLE=""

  export RTK_TOTAL_SAVED RTK_AVG_SAVINGS_PCT RTK_TOTAL_COMMANDS RTK_AVAILABLE

  # Vérifier que rtk est disponible
  if ! command -v rtk &>/dev/null; then
    return 1
  fi

  # Essayer --project d'abord (RTK 0.42+), puis fallback global
  local raw=""
  raw=$(rtk gain --project --format json 2>/dev/null) || \
  raw=$(rtk gain --format json 2>/dev/null) || true

  if [ -z "$raw" ]; then
    return 1
  fi

  # Parser le JSON via python3
  local result
  result=$(python3 - "$raw" <<'PYEOF' 2>/dev/null
import json, sys
try:
    data = json.loads(sys.argv[1])
    s = data.get("summary", {})
    saved = s.get("total_saved", 0)
    pct = s.get("avg_savings_pct", 0.0)
    cmds = s.get("total_commands", 0)
    print(f"{saved}|{pct:.1f}|{cmds}")
except Exception:
    pass
PYEOF
)

  if [ -z "$result" ]; then
    return 1
  fi

  local saved pct cmds
  IFS='|' read -r saved pct cmds <<< "$result"

  if [ "${saved:-0}" -eq 0 ] && [ "${cmds:-0}" -eq 0 ]; then
    return 1
  fi

  RTK_TOTAL_SAVED="${saved:-0}"
  RTK_AVG_SAVINGS_PCT="${pct:-0.0}"
  RTK_TOTAL_COMMANDS="${cmds:-0}"
  RTK_AVAILABLE="1"

  export RTK_TOTAL_SAVED RTK_AVG_SAVINGS_PCT RTK_TOTAL_COMMANDS RTK_AVAILABLE
  return 0
}
