#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/worktree.sh
# Fonctions testées : worktree_slug, worktree_get_path, worktree_exists,
#                     worktree_create, worktree_remove, worktree_is_merged,
#                     worktree_cleanup_merged, worktree_list, worktree_ensure_exclude

load helpers

setup() {
  common_setup

  # Répertoire simulant un projet git
  FAKE_PROJECT="$TEST_DIR/fake-project"
  mkdir -p "$FAKE_PROJECT/.git/info"
  touch "$FAKE_PROJECT/.git/info/exclude"
  mkdir -p "$FAKE_PROJECT/.git/refs/heads"

  GIT_CALLS_LOG="$TEST_DIR/git_calls.log"
  touch "$GIT_CALLS_LOG"

  # Mock des fonctions log pour ne pas polluer la sortie
  mock_log_functions

  # Sourcer worktree.sh
  source "$BATS_TEST_DIRNAME/../scripts/lib/worktree.sh"
}

teardown() {
  common_teardown
  unset -f git 2>/dev/null || true
}

# ── worktree_slug ──────────────────────────────────────────────────────────────

@test "worktree_slug : feat/bd-42 → feat-bd-42" {
  run worktree_slug "feat/bd-42"
  [ "$status" -eq 0 ]
  [ "$output" = "feat-bd-42" ]
}

@test "worktree_slug : simple-branch → simple-branch" {
  run worktree_slug "simple-branch"
  [ "$status" -eq 0 ]
  [ "$output" = "simple-branch" ]
}

@test "worktree_slug : a/b/c → a-b-c" {
  run worktree_slug "a/b/c"
  [ "$status" -eq 0 ]
  [ "$output" = "a-b-c" ]
}

@test "worktree_slug : espaces remplacés par tirets" {
  run worktree_slug "feat with spaces"
  [ "$status" -eq 0 ]
  [ "$output" = "feat-with-spaces" ]
}

# ── worktree_get_path ──────────────────────────────────────────────────────────

@test "worktree_get_path : retourne .worktrees/<slug>" {
  run worktree_get_path "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -eq 0 ]
  [ "$output" = "$FAKE_PROJECT/.worktrees/feat-bd-42" ]
}

@test "worktree_get_path : branche simple sans slash" {
  run worktree_get_path "$FAKE_PROJECT" "my-branch"
  [ "$status" -eq 0 ]
  [ "$output" = "$FAKE_PROJECT/.worktrees/my-branch" ]
}

# ── worktree_ensure_exclude ────────────────────────────────────────────────────

@test "worktree_ensure_exclude : ajoute .worktrees/ si absent" {
  # Le fichier exclude est vide
  run worktree_ensure_exclude "$FAKE_PROJECT"
  [ "$status" -eq 0 ]
  assert_file_contains "$FAKE_PROJECT/.git/info/exclude" ".worktrees/"
}

@test "worktree_ensure_exclude : ne duplique pas si déjà présent" {
  echo ".worktrees/" >> "$FAKE_PROJECT/.git/info/exclude"
  run worktree_ensure_exclude "$FAKE_PROJECT"
  [ "$status" -eq 0 ]
  # Compter les occurrences — ne doit y en avoir qu'une
  count=$(grep -c "^\.worktrees/$" "$FAKE_PROJECT/.git/info/exclude" || echo 0)
  [ "$count" -eq 1 ]
}

@test "worktree_ensure_exclude : crée .git/info/exclude si fichier absent" {
  rm -f "$FAKE_PROJECT/.git/info/exclude"
  run worktree_ensure_exclude "$FAKE_PROJECT"
  [ "$status" -eq 0 ]
  [ -f "$FAKE_PROJECT/.git/info/exclude" ]
  assert_file_contains "$FAKE_PROJECT/.git/info/exclude" ".worktrees/"
}

@test "worktree_ensure_exclude : échoue si pas de dépôt git" {
  local no_git="$TEST_DIR/no-git-project"
  mkdir -p "$no_git"
  run worktree_ensure_exclude "$no_git"
  [ "$status" -ne 0 ]
}

# ── worktree_exists ────────────────────────────────────────────────────────────

@test "worktree_exists : retourne 1 si inexistant" {
  run worktree_exists "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -ne 0 ]
}

@test "worktree_exists : retourne 0 si répertoire présent" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42"
  run worktree_exists "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -eq 0 ]
}

# ── worktree_create ────────────────────────────────────────────────────────────

@test "worktree_create : appelle git worktree add pour branche existante" {
  # Mock git : branche existante (rev-parse réussit), worktree add réussit
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "rev-parse")
        # --verify pour vérifier l'existence de la branche
        [ "${2:-}" = "--verify" ] && return 0
        return 0 ;;
      "worktree")
        [ "${2:-}" = "add" ] && mkdir -p "${4:-${3:-}}" && return 0
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_create "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -eq 0 ]
  assert_file_contains "$GIT_CALLS_LOG" "git worktree add"
}

@test "worktree_create : appelle git worktree add -b pour nouvelle branche" {
  # Mock git : branche inexistante (rev-parse échoue), worktree add -b réussit
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "rev-parse")
        [ "${2:-}" = "--verify" ] && return 1
        return 0 ;;
      "worktree")
        [ "${2:-}" = "add" ] && mkdir -p "${5:-${4:-}}" && return 0
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_create "$FAKE_PROJECT" "new-branch"
  [ "$status" -eq 0 ]
  assert_file_contains "$GIT_CALLS_LOG" "git worktree add -b"
}

@test "worktree_create : échoue si worktree déjà existant" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42"
  run worktree_create "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -ne 0 ]
}

@test "worktree_create : échoue si git worktree add échoue" {
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "rev-parse") [ "${2:-}" = "--verify" ] && return 1; return 0 ;;
      "worktree")  [ "${2:-}" = "add" ] && return 1; return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_create "$FAKE_PROJECT" "fail-branch"
  [ "$status" -ne 0 ]
}

@test "worktree_create : échoue si project_path vide" {
  run worktree_create "" "feat/bd-42"
  [ "$status" -ne 0 ]
}

@test "worktree_create : échoue si branch vide" {
  run worktree_create "$FAKE_PROJECT" ""
  [ "$status" -ne 0 ]
}

# ── worktree_remove ────────────────────────────────────────────────────────────

@test "worktree_remove : appelle git worktree remove puis prune" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42"
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "worktree")
        [ "${2:-}" = "remove" ] && rm -rf "${4:-${3:-}}" && return 0
        [ "${2:-}" = "prune" ] && return 0
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_remove "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -eq 0 ]
  assert_file_contains "$GIT_CALLS_LOG" "git worktree remove"
  assert_file_contains "$GIT_CALLS_LOG" "git worktree prune"
}

@test "worktree_remove : avertit mais ne fail pas si worktree inexistant" {
  run worktree_remove "$FAKE_PROJECT" "inexistant-branch"
  [ "$status" -eq 0 ]
}

@test "worktree_remove : échoue si git worktree remove échoue" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42"
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "worktree") [ "${2:-}" = "remove" ] && return 1; return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_remove "$FAKE_PROJECT" "feat/bd-42"
  [ "$status" -ne 0 ]
}

# ── worktree_is_merged ─────────────────────────────────────────────────────────

@test "worktree_is_merged : retourne 0 si branche dans git branch --merged" {
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-} ${2:-} ${3:-}" in
      "branch --merged main")
        echo "  feat/bd-42"
        echo "  main"
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_is_merged "$FAKE_PROJECT" "feat/bd-42" "main"
  [ "$status" -eq 0 ]
}

@test "worktree_is_merged : retourne 1 si branche non mergée" {
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-} ${2:-} ${3:-}" in
      "branch --merged main")
        echo "  main"
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  run worktree_is_merged "$FAKE_PROJECT" "feat/not-merged" "main"
  [ "$status" -ne 0 ]
}

# ── worktree_cleanup_merged ────────────────────────────────────────────────────

@test "worktree_cleanup_merged : supprime uniquement les worktrees mergés" {
  # Créer deux worktrees simulés
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-merged"
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-unmerged"

  # Créer des sous-répertoires .git dans les worktrees (requis par git rev-parse --abbrev-ref)
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-merged/.git"
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-unmerged/.git"

  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    # rev-parse --abbrev-ref HEAD dans chaque worktree
    if [[ "$*" == *"--abbrev-ref HEAD"* ]]; then
      # Déterminer le worktree courant via PWD
      case "$PWD" in
        *feat-merged*)   echo "feat/merged"; return 0 ;;
        *feat-unmerged*) echo "feat/unmerged"; return 0 ;;
        *) echo "main"; return 0 ;;
      esac
    fi
    # branch --merged pour vérifier si mergée
    if [[ "$*" == *"--merged main"* ]]; then
      echo "  feat/merged"
      echo "  main"
      return 0
    fi
    # worktree remove
    if [[ "${2:-}" == "remove" ]]; then
      rm -rf "${4:-${3:-}}" 2>/dev/null || true
      return 0
    fi
    # worktree prune
    [ "${2:-}" = "prune" ] && return 0
    return 0
  }
  export -f git

  run worktree_cleanup_merged "$FAKE_PROJECT" "main"
  [ "$status" -eq 0 ]
  # Le worktree mergé est supprimé, l'autre non
  [ ! -d "$FAKE_PROJECT/.worktrees/feat-merged" ]
  [ -d "$FAKE_PROJECT/.worktrees/feat-unmerged" ]
}

@test "worktree_cleanup_merged : ne fait rien si aucun worktree actif" {
  run worktree_cleanup_merged "$FAKE_PROJECT" "main"
  [ "$status" -eq 0 ]
}

# ── worktree_list ──────────────────────────────────────────────────────────────

@test "worktree_list : retourne message si pas de worktrees" {
  run worktree_list "$FAKE_PROJECT" "main"
  [ "$status" -eq 0 ]
  [[ "$output" == *"aucun worktree"* ]]
}

@test "worktree_list : liste les worktrees actifs" {
  mkdir -p "$FAKE_PROJECT/.worktrees/feat-bd-42/.git"

  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    [[ "$*" == *"--abbrev-ref HEAD"* ]] && echo "feat/bd-42" && return 0
    [[ "$*" == *"--merged"* ]] && return 0
    return 0
  }
  export -f git

  run worktree_list "$FAKE_PROJECT" "main"
  [ "$status" -eq 0 ]
  [[ "$output" == *"feat-bd-42"* ]]
}

# ── Intégration : cycle create → list → remove ────────────────────────────────

@test "Intégration : worktree_create puis worktree_exists puis worktree_remove" {
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    case "${1:-}" in
      "rev-parse")
        [ "${2:-}" = "--verify" ] && return 1
        return 0 ;;
      "worktree")
        if [ "${2:-}" = "add" ]; then
          mkdir -p "${5:-${4:-}}"
          return 0
        fi
        [ "${2:-}" = "remove" ] && rm -rf "${4:-${3:-}}" && return 0
        [ "${2:-}" = "prune" ] && return 0
        return 0 ;;
      *) return 0 ;;
    esac
  }
  export -f git

  # Créer
  worktree_create "$FAKE_PROJECT" "feat/integration-test"

  # Vérifier existence
  run worktree_exists "$FAKE_PROJECT" "feat/integration-test"
  [ "$status" -eq 0 ]

  # Supprimer
  worktree_remove "$FAKE_PROJECT" "feat/integration-test"

  # Vérifier absence
  run worktree_exists "$FAKE_PROJECT" "feat/integration-test"
  [ "$status" -ne 0 ]
}
