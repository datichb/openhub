#!/usr/bin/env bats
# Tests pour scripts/lib/filelock.sh
# Couvre : _acquire_lock, _release_lock, _with_lock, détection multi-OS, stale PID

setup() {
  TEST_DIR="$(mktemp -d)"
  export HUB_DIR="$TEST_DIR"
  export LIB_DIR="$BATS_TEST_DIRNAME/../scripts/lib"

  # Sourcer la lib sous test
  source "$LIB_DIR/filelock.sh"
}

teardown() {
  rm -rf "$TEST_DIR"
}

# ══════════════════════════════════════════════════════════════════════════════
# A. Acquisition et relâchement du verrou
# ══════════════════════════════════════════════════════════════════════════════

@test "filelock: _acquire_lock réussit et crée le répertoire .locks" {
  run _acquire_lock "test-lock" 5
  [ "$status" -eq 0 ]
  [ -d "$HUB_DIR/.locks" ]
  _release_lock "test-lock"
}

@test "filelock: _release_lock libère le verrou sans erreur" {
  _acquire_lock "test-lock" 5
  run _release_lock "test-lock"
  [ "$status" -eq 0 ]
}

@test "filelock: acquisition successive après relâchement réussit" {
  _acquire_lock "test-lock" 5
  _release_lock "test-lock"
  run _acquire_lock "test-lock" 5
  [ "$status" -eq 0 ]
  _release_lock "test-lock"
}

@test "filelock: deux verrous différents peuvent être acquis simultanément" {
  _acquire_lock "lock-a" 5
  run _acquire_lock "lock-b" 5
  [ "$status" -eq 0 ]
  _release_lock "lock-a"
  _release_lock "lock-b"
}

# ══════════════════════════════════════════════════════════════════════════════
# B. _with_lock — wrapper fonctionnel
# ══════════════════════════════════════════════════════════════════════════════

@test "filelock: _with_lock exécute la fonction donnée" {
  _test_fn() { echo "executed"; }
  export -f _test_fn
  run bash -c "
    source '$LIB_DIR/filelock.sh'
    export HUB_DIR='$HUB_DIR'
    _test_fn() { echo 'executed'; }
    _with_lock 'test-lock' -- _test_fn
  "
  [ "$status" -eq 0 ]
  [[ "$output" == *"executed"* ]]
}

@test "filelock: _with_lock propage le code de retour de la fonction" {
  run bash -c "
    source '$LIB_DIR/filelock.sh'
    export HUB_DIR='$HUB_DIR'
    _failing_fn() { return 42; }
    _with_lock 'test-lock' -- _failing_fn
  "
  [ "$status" -eq 42 ]
}

@test "filelock: _with_lock libère le verrou même si la fonction échoue" {
  _failing_fn() { return 1; }
  _with_lock "test-lock" -- _failing_fn || true
  # Le verrou doit être libéré — on peut en acquérir un nouveau
  run _acquire_lock "test-lock" 2
  [ "$status" -eq 0 ]
  _release_lock "test-lock"
}

# ══════════════════════════════════════════════════════════════════════════════
# C. Détection de la méthode disponible
# ══════════════════════════════════════════════════════════════════════════════

@test "filelock: _detect_lock_method retourne une valeur valide" {
  local method
  method=$(_detect_lock_method)
  [[ "$method" == "flock" || "$method" == "lockf" || "$method" == "mkdir" ]]
}

@test "filelock: _LOCK_METHOD est défini au chargement" {
  [ -n "$_LOCK_METHOD" ]
  [[ "$_LOCK_METHOD" == "flock" || "$_LOCK_METHOD" == "lockf" || "$_LOCK_METHOD" == "mkdir" ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# D. Fallback mkdir — détection de PID périmé (stale lock)
# ══════════════════════════════════════════════════════════════════════════════

@test "filelock: détecte et brise un verrou périmé (mkdir + PID mort)" {
  # Forcer le fallback mkdir même sur Linux/macOS
  _LOCK_METHOD="mkdir"

  local lockdir="$HUB_DIR/.locks/stale-lock.lock.d"
  mkdir -p "$lockdir"
  # Écrire un PID qui n'existe pas
  echo "999999" > "$lockdir/pid"

  run _acquire_lock "stale-lock" 3
  [ "$status" -eq 0 ]
  _release_lock "stale-lock"
}

@test "filelock: timeout si le verrou est tenu (mkdir, PID vivant)" {
  # Forcer le fallback mkdir
  _LOCK_METHOD="mkdir"

  local lockdir="$HUB_DIR/.locks/held-lock.lock.d"
  mkdir -p "$lockdir"
  echo $$ > "$lockdir/pid"  # notre propre PID → PID vivant

  run _acquire_lock "held-lock" 1  # timeout court
  # Doit échouer (timeout)
  [ "$status" -ne 0 ]

  # Nettoyer manuellement
  rm -rf "$lockdir"
}

# ══════════════════════════════════════════════════════════════════════════════
# E. Protection contre les écritures concurrentes (intégration)
# ══════════════════════════════════════════════════════════════════════════════

@test "filelock: protège un fichier partagé contre les écritures concurrentes" {
  local shared_file="$TEST_DIR/shared.txt"
  echo "" > "$shared_file"

  # Lancer 5 workers en parallèle qui écrivent chacun 10 lignes sous verrou
  _worker() {
    local id="$1"
    for i in $(seq 1 10); do
      _acquire_lock "shared-file" 10
      echo "worker-$id-line-$i" >> "$shared_file"
      _release_lock "shared-file"
    done
  }

  for w in 1 2 3 4 5; do
    _worker "$w" &
  done
  wait

  # Vérifier qu'on a exactement 50 lignes (pas de corruption)
  local line_count
  line_count=$(grep -c "worker-" "$shared_file")
  [ "$line_count" -eq 50 ]
}
