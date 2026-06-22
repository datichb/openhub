#!/usr/bin/env bats
# Tests unitaires pour scripts/cmd-beads.sh
# Fonctions testées : _set_project_tracker, _resolve_tracker, cmd_init,
#                     _normalize_gitlab_project_id

bats_require_minimum_version 1.5.0
#
# Grâce au guard BASH_SOURCE, cmd-beads.sh peut être sourcé directement :
# il n'exécute le dispatch que lorsqu'il est lancé en tant que script principal.
# On source common.sh puis cmd-beads.sh pour obtenir les vraies fonctions.

setup() {
  TEST_DIR="$(mktemp -d)"

  # Sourcer common.sh pour les fonctions partagées
  source "$BATS_TEST_DIRNAME/../scripts/common.sh"

  # Forcer la langue FR pour que les messages i18n soient stables
  # indépendamment de la présence ou non de config/hub.json
  export OC_LANG=fr

  # Surcharger les fichiers de données
  PROJECTS_FILE="$TEST_DIR/projects.md"
  PATHS_FILE="$TEST_DIR/paths.local.md"

  # Sourcer cmd-beads.sh (le dispatch ne s'exécute pas grâce au guard BASH_SOURCE)
  source "$BATS_TEST_DIRNAME/../scripts/cmd-beads.sh"

  # Mock de bd — enregistre les appels dans BD_CALLS_LOG
  BD_CALLS_LOG="$TEST_DIR/bd_calls.log"
  export BD_CALLS_LOG
  : > "$BD_CALLS_LOG"
  bd() {
    local _oifs="${IFS-}" ; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    # Gérer le flag -C <path> (bd -C <path> <cmd> ...)
    local _bd_dir="."
    local _args=("$@")
    if [ "${1:-}" = "-C" ]; then
      _bd_dir="$2"
      shift 2
    fi
    # bd init → créer .beads/ dans le répertoire cible
    if [ "${1:-}" = "init" ]; then
      mkdir -p "$_bd_dir/.beads"
    fi
    return 0
  }
  export -f bd

  # Mock git — intercepte remote, enregistre les appels, délègue le reste
  GIT_CALLS_LOG="$TEST_DIR/git_calls.log"
  export GIT_CALLS_LOG
  : > "$GIT_CALLS_LOG"
  REAL_GIT="$(command -v git)"
  git() {
    echo "git $*" >> "$GIT_CALLS_LOG"
    if [ "${1:-}" = "remote" ]; then
      if [ "${2:-}" = "get-url" ]; then
        return 1  # Simuler aucun remote configuré
      elif [ "${2:-}" = "add" ]; then
        return 0
      fi
      return 0
    fi
    "$REAL_GIT" "$@"
  }
  export -f git

  # Fonctions guard simplifiées
  _require_bd() { return 0; }
  require_project_id() { [ -z "${1:-}" ] && { log_error "PROJECT_ID requis"; exit 1; }; return 0; }

  # ── Données de test ───────────────────────────────────────────────────────

  cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre de test

## PROJ-FULL
- Nom : Projet complet
- Stack : Test
- Board Beads : PROJ-FULL
- Tracker : jira
- Labels : feature,fix

## PROJ-NO-TRACKER
- Nom : Sans Tracker
- Stack : Test
- Board Beads : PROJ-NO-TRACKER
- Labels : test

## PROJ-NONE
- Nom : Tracker none
- Stack : Test
- Board Beads : PROJ-NONE
- Tracker : none
- Labels : test

## PROJ-NO-LABELS
- Nom : Sans Labels
- Stack : Test
- Board Beads : PROJ-NO-LABELS

## PROJ-CASE
- Nom : Casse incorrecte
- Stack : Test
- Board Beads : PROJ-CASE
- Tracker : Jira
- Labels : test
PROJEOF

  mkdir -p "$TEST_DIR/fake-project"
  cat > "$PATHS_FILE" <<EOF
PROJ-FULL=$TEST_DIR/fake-project
PROJ-NO-TRACKER=$TEST_DIR/fake-project
PROJ-NONE=$TEST_DIR/fake-project
PROJ-NO-LABELS=$TEST_DIR/fake-project
PROJ-CASE=$TEST_DIR/fake-project
EOF
}

teardown() {
  rm -rf "$TEST_DIR"
  unset -f bd git jq command curl 2>/dev/null || true
  unset -f _require_bd _require_beads_init resolve_project_path 2>/dev/null || true
}

# ── _resolve_tracker ──────────────────────────────────────────────────────────

@test "_resolve_tracker : retourne le tracker quand il est configuré" {
  run _resolve_tracker "PROJ-FULL"
  [ "$status" -eq 0 ]
  [ "$output" = "jira" ]
}

@test "_resolve_tracker : normalise en minuscules (Jira → jira)" {
  run _resolve_tracker "PROJ-CASE"
  [ "$status" -eq 0 ]
  [ "$output" = "jira" ]
}

@test "_resolve_tracker : exit si tracker est none" {
  run _resolve_tracker "PROJ-NONE"
  [ "$status" -ne 0 ]
}

@test "_resolve_tracker : exit si aucun tracker configuré" {
  run _resolve_tracker "PROJ-NO-TRACKER"
  [ "$status" -ne 0 ]
}

# ── _set_project_tracker ─────────────────────────────────────────────────────

@test "_set_project_tracker : remplace un tracker existant" {
  _set_project_tracker "PROJ-FULL" "gitlab"
  run grep -F -- "- Tracker : gitlab" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
  # L'ancien tracker ne doit plus être présent pour ce projet
  local block
  block=$(sed -n '/^## PROJ-FULL$/,/^## /{/^## PROJ-FULL$/d;/^## /d;p;}' "$PROJECTS_FILE")
  ! echo "$block" | grep -q -- "- Tracker : jira"
}

@test "_set_project_tracker : ajoute après Labels si Tracker absent" {
  _set_project_tracker "PROJ-NO-TRACKER" "gitlab"
  run grep -F -- "- Tracker : gitlab" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "_set_project_tracker : fallback — ajoute après dernier champ si Labels absent" {
  _set_project_tracker "PROJ-NO-LABELS" "jira"
  run grep -F -- "- Tracker : jira" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "_set_project_tracker : whitelist — rejette une valeur invalide" {
  run _set_project_tracker "PROJ-FULL" "bitbucket"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qi "invalide"
}

@test "_set_project_tracker : accepte none comme valeur valide" {
  _set_project_tracker "PROJ-FULL" "none"
  run grep -F -- "- Tracker : none" "$PROJECTS_FILE"
  [ "$status" -eq 0 ]
}

@test "_set_project_tracker : ne modifie pas les autres projets" {
  _set_project_tracker "PROJ-FULL" "gitlab"
  # PROJ-NONE doit toujours avoir son tracker original
  local block
  block=$(sed -n '/^## PROJ-NONE$/,/^## /{/^## PROJ-NONE$/d;/^## /d;p;}' "$PROJECTS_FILE")
  echo "$block" | grep -q -- "- Tracker : none"
}

# ── resolve_project_path ──────────────────────────────────────────────────────

@test "resolve_project_path : retourne le chemin d'un projet valide" {
  run resolve_project_path "PROJ-FULL"
  [ "$status" -eq 0 ]
  [ "$output" = "$TEST_DIR/fake-project" ]
}

@test "resolve_project_path : exit si le projet n'existe pas" {
  run resolve_project_path "INEXISTANT"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qi "introuvable"
}

@test "resolve_project_path : exit si le chemin est vide" {
  # Écraser paths avec un projet sans chemin
  cat > "$PATHS_FILE" <<EOF
PROJ-NO-TRACKER=
EOF
  run resolve_project_path "PROJ-NO-TRACKER"
  [ "$status" -ne 0 ]
}

@test "resolve_project_path : exit si le dossier n'existe pas sur le disque" {
  cat > "$PATHS_FILE" <<EOF
PROJ-FULL=$TEST_DIR/dossier-inexistant
EOF
  run resolve_project_path "PROJ-FULL"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qi "introuvable"
}

# ── cmd_init — propagation des labels ─────────────────────────────────────────

@test "cmd_init : appelle bd init et propage les labels du projet" {
  # Nettoyer le log bd et le .beads potentiel
  : > "$BD_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  run cmd_init "PROJ-FULL" < /dev/null
  [ "$status" -eq 0 ]
  # bd init a été appelé (avec ou sans flag -C)
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  # Les labels feature,fix ont été propagés via bd label create
  grep -q "label create feature" "$BD_CALLS_LOG"
  grep -q "label create fix" "$BD_CALLS_LOG"
}

@test "cmd_init : ne propage rien si aucun label configuré" {
  : > "$BD_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  run cmd_init "PROJ-NO-LABELS" < /dev/null
  [ "$status" -eq 0 ]
  # bd init appelé, mais pas bd label create
  grep -qE "bd( -C [^ ]+)? init" "$BD_CALLS_LOG"
  ! grep -q "label create" "$BD_CALLS_LOG"
}

@test "cmd_init : exit si .beads existe déjà" {
  # .beads existe déjà (créé par un test précédent ou manuellement)
  mkdir -p "$TEST_DIR/fake-project/.beads"
  run cmd_init "PROJ-FULL"
  # exit 0 avec un warning
  [ "$status" -eq 0 ]
  echo "$output" | grep -qi "déjà initialisé"
}

# ── cmd_init — proposition upstream git ──────────────────────────────────────

@test "cmd_init : propose upstream et avertit URL vide si stdin vide" {
  # Fournir explicitement /dev/null comme stdin — read retourne immédiatement avec valeur vide
  : > "$BD_CALLS_LOG"
  : > "$GIT_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  run cmd_init "PROJ-FULL" < /dev/null
  [ "$status" -eq 0 ]

  # git remote get-url upstream a été testé
  grep -q "git remote get-url upstream" "$GIT_CALLS_LOG"
  # URL vide → avertissement
  [[ "$output" == *"URL vide"* ]]
}

@test "cmd_init : configure upstream si URL fournie via stdin" {
  : > "$BD_CALLS_LOG"
  : > "$GIT_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  # Fournir Y + URL via fichier stdin
  printf "Y\nhttps://github.com/test/repo.git\n" > "$TEST_DIR/stdin_upstream.txt"
  run cmd_init "PROJ-FULL" < "$TEST_DIR/stdin_upstream.txt"
  [ "$status" -eq 0 ]

  # git remote add upstream a été appelé avec l'URL
  grep -q "git remote add upstream https://github.com/test/repo.git" "$GIT_CALLS_LOG"
  [[ "$output" == *"Remote upstream configuré"* ]]
}

@test "cmd_init : respecte le refus de configurer upstream" {
  : > "$BD_CALLS_LOG"
  : > "$GIT_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  # Fournir n via fichier stdin → pas de question URL
  printf "n\n" > "$TEST_DIR/stdin_refuse.txt"
  run cmd_init "PROJ-FULL" < "$TEST_DIR/stdin_refuse.txt"
  [ "$status" -eq 0 ]

  # git remote add ne doit PAS avoir été appelé
  ! grep -q "git remote add upstream" "$GIT_CALLS_LOG"
  [[ "$output" == *"Configurer plus tard"* ]]
}

# ── cmd_init — exclusion .beads/ dans .git/info/exclude ──────────────────────

@test "cmd_init : ajoute .beads/ au .git/info/exclude du projet" {
  : > "$BD_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  mkdir -p "$TEST_DIR/fake-project/.git/info"
  run cmd_init "PROJ-FULL" < /dev/null
  [ "$status" -eq 0 ]

  # .beads/ doit être présent dans le fichier exclude
  grep -qx ".beads/" "$TEST_DIR/fake-project/.git/info/exclude"
}

@test "cmd_init : n'ajoute pas .beads/ en double si déjà présent dans exclude" {
  : > "$BD_CALLS_LOG"
  rm -rf "$TEST_DIR/fake-project/.beads"
  mkdir -p "$TEST_DIR/fake-project/.git/info"
  echo ".beads/" > "$TEST_DIR/fake-project/.git/info/exclude"

  run cmd_init "PROJ-FULL" < /dev/null
  [ "$status" -eq 0 ]

  # Compter les occurrences — doit être exactement 1
  local count
  count=$(grep -cx ".beads/" "$TEST_DIR/fake-project/.git/info/exclude")
  [ "$count" -eq 1 ]
  # Message "déjà présent" attendu
  [[ "$output" == *"déjà présent"* ]]
}

# ── _normalize_gitlab_project_id ─────────────────────────────────────────────

@test "_normalize_gitlab_project_id : retourne un ID numérique tel quel" {
  run _normalize_gitlab_project_id "https://gitlab.com" "12345"
  [ "$status" -eq 0 ]
  [ "$output" = "12345" ]
}

@test "_normalize_gitlab_project_id : retourne un chemin namespace/projet tel quel" {
  run _normalize_gitlab_project_id "https://gitlab.com" "my-group/my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "my-group/my-project" ]
}

@test "_normalize_gitlab_project_id : extrait le chemin d'une URL gitlab.com" {
  run --separate-stderr _normalize_gitlab_project_id "https://gitlab.com" "https://gitlab.com/my-group/my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "my-group/my-project" ]
  echo "$stderr" | grep -qi "extrait automatiquement"
}

@test "_normalize_gitlab_project_id : extrait le chemin d'une URL d'instance privée" {
  run --separate-stderr _normalize_gitlab_project_id "https://git.example.com" "https://git.example.com/team/backend"
  [ "$status" -eq 0 ]
  [ "$output" = "team/backend" ]
}

@test "_normalize_gitlab_project_id : supprime le suffixe .git de l'URL" {
  run --separate-stderr _normalize_gitlab_project_id "https://gitlab.com" "https://gitlab.com/my-group/my-project.git"
  [ "$status" -eq 0 ]
  [ "$output" = "my-group/my-project" ]
}

@test "_normalize_gitlab_project_id : tolère un slash final dans la base URL" {
  run --separate-stderr _normalize_gitlab_project_id "https://gitlab.com/" "https://gitlab.com/my-group/my-project"
  [ "$status" -eq 0 ]
  [ "$output" = "my-group/my-project" ]
}

@test "_normalize_gitlab_project_id : échoue si l'URL ne correspond pas à l'instance" {
  run --separate-stderr _normalize_gitlab_project_id "https://git.example.com" "https://gitlab.com/my-group/my-project"
  [ "$status" -ne 0 ]
  echo "$stderr" | grep -qi "ne correspond pas"
}

@test "_normalize_gitlab_project_id : échoue si l'URL ne contient que la base (pas de chemin)" {
  run --separate-stderr _normalize_gitlab_project_id "https://gitlab.com" "https://gitlab.com/"
  [ "$status" -ne 0 ]
}

# ── _fetch_tracker_labels ─────────────────────────────────────────────────────

@test "_fetch_tracker_labels : skip silencieux si jq absent" {
  # Masquer jq pour simuler son absence
  jq() { return 127; }
  export -f jq
  command() {
    if [ "${1:-}" = "-v" ] && [ "${2:-}" = "jq" ]; then return 1; fi
    builtin command "$@"
  }
  export -f command

  : > "$BD_CALLS_LOG"
  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]
  ! grep -q "bd label create" "$BD_CALLS_LOG"
}

@test "_fetch_tracker_labels : skip silencieux si aucun tracker configuré" {
  # bd config get ne retourne rien (déjà le comportement du mock bd)
  # Le mock bd enregistre les appels mais retourne 0 sans stdout
  : > "$BD_CALLS_LOG"
  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]
  # Aucun label create déclenché par le fetch (pas de curl, pas de données)
  # Le test valide uniquement le code de sortie (pas d'erreur)
}

@test "_fetch_tracker_labels : importe les labels GitLab et appelle bd label create" {
  # Mock jq disponible
  # Mock curl — retourne un JSON GitLab simulé
  CURL_CALLS_LOG="$TEST_DIR/curl_calls.log"
  : > "$CURL_CALLS_LOG"
  curl() {
    echo "curl $*" >> "$CURL_CALLS_LOG"
    # Simuler la réponse de l'API GitLab /labels
    printf '[{"name":"frontend"},{"name":"backend"},{"name":"hotfix"}]'
    return 0
  }
  export -f curl
  export CURL_CALLS_LOG

  # Mock bd config get — retourne des valeurs pour gitlab.*
  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    # Ignorer le flag -C <path> si présent
    if [ "${1:-}" = "-C" ]; then shift 2; fi
    case "${1:-} ${2:-} ${3:-}" in
      "config get gitlab.url")        printf 'https://gitlab.example.com' ;;
      "config get gitlab.token")      printf 'glpat-test-token' ;;
      "config get gitlab.project_id") printf 'my-group/my-project' ;;
      "init")                          mkdir -p .beads ;;
    esac
    return 0
  }
  export -f bd

  : > "$BD_CALLS_LOG"
  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # curl a été appelé avec l'URL GitLab
  grep -q "gitlab.example.com" "$CURL_CALLS_LOG"
  # Les 3 labels ont été créés (format : bd -C <path> label create <name>)
  grep -q "label create frontend" "$BD_CALLS_LOG"
  grep -q "label create backend"  "$BD_CALLS_LOG"
  grep -q "label create hotfix"   "$BD_CALLS_LOG"
  # Message de succès présent
  [[ "$output" == *"GitLab"* ]] || [[ "$output" == *"3"* ]]
}

@test "_fetch_tracker_labels : encode le slash dans le project_id pour l'URL GitLab" {
  CURL_CALLS_LOG="$TEST_DIR/curl_calls.log"
  : > "$CURL_CALLS_LOG"
  curl() {
    echo "curl $*" >> "$CURL_CALLS_LOG"
    printf '[]'
    return 0
  }
  export -f curl
  export CURL_CALLS_LOG

  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    if [ "${1:-}" = "-C" ]; then shift 2; fi
    case "${1:-} ${2:-} ${3:-}" in
      "config get gitlab.url")        printf 'https://gitlab.example.com' ;;
      "config get gitlab.token")      printf 'glpat-test' ;;
      "config get gitlab.project_id") printf 'my-group/my-project' ;;
    esac
    return 0
  }
  export -f bd

  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]
  # L'URL doit contenir le slash encodé %2F
  grep -q "%2F" "$CURL_CALLS_LOG"
}

@test "_fetch_tracker_labels : importe les labels Jira et appelle bd label create" {
  CURL_CALLS_LOG="$TEST_DIR/curl_calls.log"
  : > "$CURL_CALLS_LOG"
  curl() {
    echo "curl $*" >> "$CURL_CALLS_LOG"
    # Simuler la réponse de l'API Jira /label
    printf '{"values":["bug","improvement","task"],"total":3}'
    return 0
  }
  export -f curl
  export CURL_CALLS_LOG

  # Mock bd config get — gitlab.* vide, jira.* renseigné
  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    if [ "${1:-}" = "-C" ]; then shift 2; fi
    case "${1:-} ${2:-} ${3:-}" in
      "config get gitlab.url")   printf '' ;;
      "config get gitlab.token") printf '' ;;
      "config get jira.url")     printf 'https://jira.example.com' ;;
      "config get jira.user")    printf 'user@example.com' ;;
      "config get jira.token")   printf 'jira-api-token' ;;
    esac
    return 0
  }
  export -f bd

  : > "$BD_CALLS_LOG"
  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # curl a été appelé avec l'URL Jira
  grep -q "jira.example.com" "$CURL_CALLS_LOG"
  # Les 3 labels ont été créés (format : bd -C <path> label create <name>)
  grep -q "label create bug"         "$BD_CALLS_LOG"
  grep -q "label create improvement" "$BD_CALLS_LOG"
  grep -q "label create task"        "$BD_CALLS_LOG"
}

@test "_fetch_tracker_labels : GitLab prioritaire sur Jira si les deux sont configurés" {
  CURL_CALLS_LOG="$TEST_DIR/curl_calls.log"
  : > "$CURL_CALLS_LOG"
  curl() {
    echo "curl $*" >> "$CURL_CALLS_LOG"
    printf '[]'
    return 0
  }
  export -f curl
  export CURL_CALLS_LOG

  # Les deux trackers configurés
  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    if [ "${1:-}" = "-C" ]; then shift 2; fi
    case "${1:-} ${2:-} ${3:-}" in
      "config get gitlab.url")        printf 'https://gitlab.example.com' ;;
      "config get gitlab.token")      printf 'glpat-test' ;;
      "config get gitlab.project_id") printf '42' ;;
      "config get jira.url")          printf 'https://jira.example.com' ;;
      "config get jira.user")         printf 'user@example.com' ;;
      "config get jira.token")        printf 'jira-token' ;;
    esac
    return 0
  }
  export -f bd

  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]

  # Seul GitLab a été appelé (return 0 après le bloc gitlab)
  grep -q "gitlab.example.com" "$CURL_CALLS_LOG"
  ! grep -q "jira.example.com" "$CURL_CALLS_LOG"
}

@test "_fetch_tracker_labels : skip silencieux si curl échoue (réseau indisponible)" {
  curl() { return 1; }
  export -f curl

  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    if [ "${1:-}" = "-C" ]; then shift 2; fi
    case "${1:-} ${2:-} ${3:-}" in
      "config get gitlab.url")        printf 'https://gitlab.example.com' ;;
      "config get gitlab.token")      printf 'glpat-test' ;;
      "config get gitlab.project_id") printf '42' ;;
    esac
    return 0
  }
  export -f bd

  : > "$BD_CALLS_LOG"
  run _fetch_tracker_labels "PROJ-FULL" "$TEST_DIR/fake-project"
  [ "$status" -eq 0 ]
  # Aucun label créé
  ! grep -q "label create" "$BD_CALLS_LOG"
}

# ── cmd_board — vérifications de base ────────────────────────────────────────

@test "cmd_board : exit si bd n'est pas disponible" {
  _require_bd() { log_error "bd non installé"; exit 1; }
  export -f _require_bd

  run cmd_board "TEST-PROJECT"
  [ "$status" -ne 0 ]
}

@test "cmd_board : exit si .beads/ n'existe pas dans le projet" {
  _require_bd() { return 0; }
  export -f _require_bd
  _require_beads_init() { log_error "Beads non initialisé"; exit 1; }
  export -f _require_beads_init

  run cmd_board "TEST-PROJECT"
  [ "$status" -ne 0 ]
}

@test "cmd_board : appelle bd list pour les 4 statuts" {
  BD_CALLS_LOG="$TEST_DIR/bd_board_calls.log"
  : > "$BD_CALLS_LOG"

  _require_bd() { return 0; }
  export -f _require_bd
  _require_beads_init() { return 0; }
  export -f _require_beads_init
  resolve_project_path() { echo "$TEST_DIR/fake-project"; }
  export -f resolve_project_path

  mkdir -p "$TEST_DIR/fake-project/.beads"

  bd() {
    local _oifs="${IFS-}"; IFS=' '
    echo "bd $*" >> "$BD_CALLS_LOG"
    IFS="$_oifs"
    echo "[]"
  }
  export -f bd
  export BD_CALLS_LOG

  run cmd_board "TEST-PROJECT"
  # Vérifier que bd list a été appelé avec tous les statuts actifs (1 appel groupé)
  grep -q "list --status open,in_progress,review,blocked" "$BD_CALLS_LOG"
}
