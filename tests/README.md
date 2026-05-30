# Guide des Tests OpenCode Hub

Ce guide documente l'architecture, les conventions et les bonnes pratiques pour les tests du projet OpenCode Hub.

## 📊 Vue d'ensemble

**Framework** : BATS (Bash Automated Testing System)  
**Fichiers de tests** : 19+ fichiers `.bats` dans `tests/`  
**Total de tests** : 450+ tests  
**Exécution parallèle** : `bats -j 4 tests/*.bats`

## 🏗️ Architecture

### Organisation des fichiers

```
tests/
├── helpers.bash              # Bibliothèque de fonctions partagées
├── fixtures/                 # Fixtures pré-compilées réutilisables
│   ├── agents/              # Agents de test
│   ├── skills/              # Skills de test
│   ├── projects/            # Projets de test
│   └── configs/             # Configurations de test
├── test_*.bats              # Tests unitaires par module
├── test_cmd_*.bats          # Tests des commandes (cmd-*.sh)
├── test_lib_*.bats          # Tests des librairies (lib/*.sh)
├── test_integration_*.bats  # Tests d'intégration
└── test_smoke.bats          # Tests de smoke (sanity checks)
```

### Types de tests

#### Tests unitaires (90%)
- **Scope** : Fonction ou module isolé
- **Mocks** : Complets (bd, git, curl, etc.)
- **Exécution** : Rapide (<1s par test)
- **Exemples** : `test_common.bats`, `test_prompt_builder.bats`

#### Tests d'intégration (10%)
- **Scope** : Pipeline complet avec agents/skills réels
- **Isolation** : Copie des sources (ne touche jamais les fichiers réels)
- **Exécution** : Plus lent (5-10s par test)
- **Exemples** : `test_integration_deploy.bats`

#### Tests de smoke
- **Scope** : Vérification basique de l'exécutabilité
- **Objectif** : Détection précoce de régressions syntaxiques
- **Fichier** : `test_smoke.bats`

## 📝 Conventions de nommage

### Fichiers de tests
- **Pattern** : `test_<module>.bats`
- **Commandes** : `test_cmd_<commande>.bats`
- **Librairies** : `test_lib_<librairie>.bats`
- **Intégration** : `test_integration_<feature>.bats`

### Noms de tests
- **Langue** : Français (100%)
- **Format** : `fonction : contexte — comportement attendu`
- **Exemples** :
  ```bash
  @test "resolve_agent_model niveau 7 : fallback hardcodé claude-sonnet-4-5"
  @test "clamp_model retourne le plancher quand résolu est inférieur"
  @test "_get_agent_model avec clamp retourne le plancher quand global < plancher"
  ```

### Séparateurs de sections
Utilisez des séparateurs ASCII pour regrouper les tests par fonction :
```bash
# ── _get_agent_family ─────────────────────────────────────────────────────────

@test "_get_agent_family déduit la famille depuis le chemin" {
  # ...
}
```

## 🚀 Écrire un nouveau test

### Template de base

```bash
#!/usr/bin/env bats
# Tests pour <module>
# Fonctions testées : <liste des fonctions>

load helpers

setup() {
  common_setup
  
  # Configuration spécifique au test
  # ...
}

teardown() {
  common_teardown
}

# ── Nom de la fonction ────────────────────────────────────────────────────────

@test "fonction : comportement nominal" {
  # Arrange
  make_test_project "TEST-PROJ" "Mon Projet"
  
  # Act
  run ma_fonction "TEST-PROJ"
  
  # Assert
  [ "$status" -eq 0 ]
  assert_file_contains "$PROJECTS_FILE" "résultat attendu"
}

@test "fonction : cas d'erreur" {
  run ma_fonction "INEXISTANT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"erreur"* ]]
}
```

### Utilisation des helpers

#### 1. Créer des fixtures

```bash
# Projet minimal
make_test_project "PROJ-ID" "Nom" "Stack"

# Projet complet
make_full_test_project "PROJ-ID" "Nom" "Stack" "gitlab" "user/repo"

# Agent
make_test_agent "agent-id" "planning" "skill1" "skill2"

# Agent avec permissions
make_test_agent_with_permissions "agent-id" "planning" "websearch: allow"

# Hub config
make_test_hub_config "claude-opus-4"

# API keys
make_test_api_keys "anthropic" "sk-test-123"
```

#### 2. Assertions

```bash
# Vérifier qu'un fichier contient un pattern
assert_file_contains "$file" "pattern"

# Vérifier qu'un fichier NE contient PAS un pattern
assert_file_not_contains "$file" "pattern"

# Valider JSON
assert_json_valid "$file"

# Vérifier un champ JSON
assert_json_field "$file" ".key.subkey" "expected_value"

# Compter les occurrences
count=$(count_occurrences "$file" "pattern")
[ "$count" -eq 2 ]
```

#### 3. Mocks

```bash
# Mock des fonctions log_*
mock_log_functions

# Mock bd avec capture
BD_LOG="$TEST_DIR/bd.log"
mock_bd_with_log "$BD_LOG"
bd list
assert_file_contains "$BD_LOG" "bd list"

# Mock git avec capture
GIT_LOG="$TEST_DIR/git.log"
mock_git_with_log "$GIT_LOG"
git status
assert_file_contains "$GIT_LOG" "git status"
```

#### 4. Utiliser des fixtures pré-compilées

```bash
# Copier une fixture
cp "$BATS_TEST_DIRNAME/fixtures/agents/minimal_agent.md" "$TEST_DIR/agent.md"

# Ou utiliser directement
run ma_fonction "$BATS_TEST_DIRNAME/fixtures/agents/minimal_agent.md"
```

## 🎯 Bonnes pratiques

### 1. Isolation totale

✅ **À FAIRE** :
```bash
setup() {
  TEST_DIR="$(mktemp -d)"
  PROJECTS_FILE="$TEST_DIR/projects.md"
}

teardown() {
  rm -rf "$TEST_DIR"
}
```

❌ **À ÉVITER** :
```bash
# Ne JAMAIS toucher les fichiers réels du repo
PROJECTS_FILE="$HUB_ROOT/projects/projects.md"
```

### 2. Tests indépendants

✅ **À FAIRE** :
- Chaque test crée ses propres fixtures
- Aucune dépendance d'ordre d'exécution
- Tests parallélisables

❌ **À ÉVITER** :
- Partager des variables entre tests
- Dépendre de l'état laissé par un test précédent

### 3. Assertions claires

✅ **À FAIRE** :
```bash
# Assertions multiples avec messages clairs
[ "$status" -eq 0 ]
assert_file_contains "$output_file" "résultat attendu"

# Vérifier le contenu des erreurs
[ "$status" -ne 0 ]
[[ "$output" == *"projet introuvable"* ]]
```

❌ **À ÉVITER** :
```bash
# Assertion unique sans contexte
[ "$status" -eq 0 ]
```

### 4. Mocking exhaustif

✅ **À FAIRE** :
```bash
# Mock toutes les dépendances externes
bd() { echo "bd $*" >> "$BD_LOG"; return 0; }
git() { echo "git $*" >> "$GIT_LOG"; return 0; }
curl() { echo '{"mock": "response"}'; }
```

❌ **À ÉVITER** :
```bash
# Appeler de vrais outils externes (lent, fragile)
bd list  # Appel réel
```

### 5. Documenter les edge cases

✅ **À FAIRE** :
```bash
@test "fonction : retourne vide quand paramètre absent" {
  run ma_fonction ""
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "fonction : gère les caractères spéciaux" {
  run ma_fonction "test avec espaces & chars"
  [ "$status" -eq 0 ]
}
```

### 6. Tests de non-régression

✅ **À FAIRE** :
```bash
@test "fonction : fonctionne sous set -u — non-régression #issue-123" {
  bash -c "
    set -euo pipefail
    # ...
  "
}
```

Lien explicite avec l'issue tracker pour traçabilité.

## 🔧 Exécution des tests

### Locale

```bash
# Tous les tests
bats tests/*.bats

# Tests parallèles (4 jobs) - RECOMMANDÉ
bats -j 4 tests/*.bats

# Un fichier spécifique
bats tests/test_common.bats

# Tests correspondant à un pattern
bats -f "resolve_agent_model" tests/test_get_agent_model.bats

# Avec verbose
bats -t tests/test_common.bats
```

### CI/CD

Les tests s'exécutent automatiquement en parallèle dans GitHub Actions :
- `.github/workflows/ci.yml` : tests principaux (`bats -j 4`)
- `.github/workflows/test-websearch.yml` : tests WebSearch (`bats -j 2`)

### Debugging

```bash
# Afficher le contenu d'un fichier en cas d'échec
debug_file "$file" 50

# Capturer stderr
run --separate-stderr ma_fonction
echo "$stderr" | grep -q "erreur"

# Exécuter avec set -x pour trace
bash -x -c "source scripts/common.sh; ma_fonction"
```

## 📚 Helpers disponibles

### Création de fixtures
- `make_test_project` : projet minimal
- `make_full_test_project` : projet complet
- `make_test_agent` : agent minimal
- `make_test_agent_with_permissions` : agent avec permissions
- `make_test_skill` : skill minimale
- `make_test_hub_config` : hub.json minimal
- `make_test_api_keys` : api-keys.local.md
- `make_beads_test_projects` : registre complet pour tests Beads
- `make_beads_paths_file` : paths.local.md pour tests Beads
- `make_stack_skills_config` : stack-skills.json

### Assertions
- `assert_file_contains` : vérifie présence d'un pattern
- `assert_file_not_contains` : vérifie absence d'un pattern
- `assert_json_valid` : valide JSON avec jq
- `assert_json_field` : vérifie valeur d'un champ JSON
- `count_occurrences` : compte les occurrences

### Mocks
- `mock_log_functions` : mock log_info/success/warn/error
- `mock_bd_with_log` : mock bd avec capture
- `mock_git_with_log` : mock git avec capture
- `mock_get_hub_version` : mock get_hub_version

### Utilitaires
- `common_setup` : setup standard (TEST_DIR, variables, mocks)
- `common_teardown` : cleanup standard
- `beads_setup` : setup complet pour tests cmd-beads
- `prompt_builder_stack_setup` : setup pour tests prompt-builder avec stack
- `count_lines` : compte les lignes d'un fichier
- `command_exists` : vérifie existence d'une commande
- `require_command` : skip si commande absente
- `trim` : supprime espaces début/fin
- `debug_file` : affiche contenu avec numéros de ligne

## 📈 Métriques & couverture

### État actuel (post-Phase 2)
- **Tests totaux** : 460+
- **Fichiers de tests** : 20
- **Couverture commandes** : 41% (11/27)
- **Couverture librairies** : 20% (3/15)
- **Score qualité** : 9.6/10

### Objectifs Phase 5
- **Tests totaux** : 600+
- **Couverture commandes** : 70% (19/27)
- **Couverture librairies** : 40% (6/15)
- **Score qualité** : 9.8/10

## 🐛 Debugging de tests échoués

### 1. Identifier le problème

```bash
# Exécuter avec verbose
bats -t tests/test_failing.bats

# Isoler le test qui échoue
bats -f "nom du test" tests/test_failing.bats
```

### 2. Inspecter les fichiers générés

```bash
@test "debug : inspecter fichiers" {
  make_test_project "TEST"
  
  # Afficher le contenu
  debug_file "$PROJECTS_FILE"
  
  # Ou manuellement
  echo "=== PROJECTS_FILE ===" >&2
  cat "$PROJECTS_FILE" >&2
}
```

### 3. Vérifier les mocks

```bash
@test "debug : vérifier appels bd" {
  BD_LOG="$TEST_DIR/bd.log"
  mock_bd_with_log "$BD_LOG"
  
  cmd_init "PROJ"
  
  # Afficher tous les appels
  cat "$BD_LOG" >&2
}
```

### 4. Problèmes courants

#### `grep: invalid option --`
**Cause** : Pattern commence par un tiret  
**Solution** : Utiliser `grep -F -- "$pattern"` (déjà fait dans helpers)

#### `TEST_DIR not found`
**Cause** : teardown s'exécute avant la fin du test  
**Solution** : Utiliser `common_setup` / `common_teardown`

#### `jq non disponible`
**Cause** : jq absent sur la machine de test  
**Solution** : Utiliser `require_command "jq"` pour skip gracieux

## 🔗 Ressources

- [Documentation BATS](https://bats-core.readthedocs.io/)
- [helpers.bash](helpers.bash) : code source des helpers
- [test_helpers.bats](test_helpers.bats) : tests de validation des helpers
- [test_smoke.bats](test_smoke.bats) : tests de smoke globaux

## 📞 Support

Pour toute question ou problème :
1. Lire ce guide
2. Consulter les exemples dans `test_helpers.bats`
3. Examiner les tests existants similaires
4. Ouvrir une issue sur le repo

---

**Dernière mise à jour** : Phase 2.4 (Mai 2026)  
**Version du guide** : 1.0
