#!/bin/bash
# filelock.sh — Verrouillage de fichiers portable (macOS + Linux)
# Sourced by common.sh. Do not execute directly.
# Compatible bash 3.2+
#
# Stratégie multi-OS (par ordre de préférence) :
#   1. Linux  : flock (util-linux, toujours disponible)
#   2. macOS  : /usr/bin/lockf (BSD, toujours disponible)
#   3. Fallback : mkdir + fichier PID + détection de staleness
#
# API publique :
#   _acquire_lock <lockfile> [timeout_seconds]  → 0=OK, 1=timeout/erreur
#   _release_lock <lockfile>                    → libère le verrou
#
# FD utilisé : 9 (fixe pour compatibilité bash 3.2 — pas de {fd}>file)
# Les lockfiles sont créés dans $HUB_DIR/.locks/

# Guard contre le double-sourcing
[ -n "${_FILELOCK_LOADED:-}" ] && return 0
_FILELOCK_LOADED=1

# ── Répertoire des lockfiles ──────────────────────────────────────────────────
_LOCK_DIR="${HUB_DIR:-$HOME/.openhub}/.locks"

# ── Détection de la méthode de locking disponible ────────────────────────────
_detect_lock_method() {
  if command -v flock &>/dev/null; then
    echo "flock"
  elif [ -x /usr/bin/lockf ]; then
    echo "lockf"
  else
    echo "mkdir"
  fi
}
_LOCK_METHOD=$(_detect_lock_method)

# ── _acquire_lock ─────────────────────────────────────────────────────────────
# Acquiert un verrou exclusif sur le fichier donné.
# @param $1  Nom logique du verrou (ex: "projects", "api-keys", "hub")
# @param $2  Timeout en secondes (défaut: 10)
# @return 0  Verrou acquis
# @return 1  Timeout ou erreur
_acquire_lock() {
  local lock_name="${1:?lock name required}"
  local timeout="${2:-10}"

  mkdir -p "$_LOCK_DIR"
  local lockfile="$_LOCK_DIR/${lock_name}.lock"

  case "$_LOCK_METHOD" in
    flock)
      # Linux : flock(1) via fd redirection (bash 3.2 : FD numérique fixe)
      exec 9>"$lockfile"
      flock -w "$timeout" 9 || { exec 9>&-; return 1; }
      ;;
    lockf)
      # macOS/BSD : lockf(1) via fd redirection
      exec 9>"$lockfile"
      /usr/bin/lockf -s -t "$timeout" 9 || { exec 9>&-; return 1; }
      ;;
    mkdir)
      # Fallback universel : mkdir atomique + PID file + détection de staleness
      local lockdir="${lockfile}.d"
      local pidfile="${lockdir}/pid"
      local deadline=$(( $(date +%s) + timeout ))

      while ! mkdir "$lockdir" 2>/dev/null; do
        # Vérifier si le verrou est périmé (processus mort)
        if [ -f "$pidfile" ]; then
          local old_pid
          old_pid=$(cat "$pidfile" 2>/dev/null || echo "")
          if [ -n "$old_pid" ] && ! kill -0 "$old_pid" 2>/dev/null; then
            rm -rf "$lockdir"   # Verrou périmé — on le brise
            continue
          fi
        fi
        # Timeout atteint ?
        if [ "$(date +%s)" -ge "$deadline" ]; then
          return 1
        fi
        sleep 0.1 2>/dev/null || sleep 1
      done
      echo $$ > "$pidfile"
      # Nettoyage garanti à la sortie du processus
      trap "rm -rf '${lockdir}' 2>/dev/null; exit" INT TERM HUP
      ;;
  esac
  return 0
}

# ── _release_lock ─────────────────────────────────────────────────────────────
# Libère le verrou acquis par _acquire_lock.
# @param $1  Nom logique du verrou (doit correspondre à l'appel _acquire_lock)
_release_lock() {
  local lock_name="${1:?lock name required}"
  local lockfile="$_LOCK_DIR/${lock_name}.lock"

  case "$_LOCK_METHOD" in
    flock|lockf)
      # Fermer le fd libère automatiquement le verrou kernel
      exec 9>&- 2>/dev/null || true
      ;;
    mkdir)
      rm -rf "${lockfile}.d" 2>/dev/null || true
      ;;
  esac
}

# ── _with_lock ────────────────────────────────────────────────────────────────
# Exécute une fonction shell sous verrou.
# Usage : _with_lock <lock_name> [timeout] -- <function_name> [args...]
# Exemple : _with_lock "projects" 10 -- _set_project_stack "PROJ" "Node.js"
_with_lock() {
  local lock_name="${1:?lock name required}"
  shift  # consommer lock_name

  local timeout=10
  # Si le prochain argument est un nombre (timeout optionnel), le consommer
  if [ -n "${1:-}" ] && [ "${1:-}" != "--" ]; then
    case "${1:-}" in
      ''|*[!0-9]*) : ;;  # pas un entier, garder la valeur par défaut
      *)            timeout="$1"; shift ;;
    esac
  fi

  # Consommer le séparateur '--'
  [ "${1:-}" = "--" ] && shift

  _acquire_lock "$lock_name" "$timeout" || {
    printf 'filelock: timeout acquiring lock "%s" after %ds\n' "$lock_name" "$timeout" >&2
    return 1
  }
  local _ret=0
  "$@" || _ret=$?
  _release_lock "$lock_name"
  return $_ret
}
