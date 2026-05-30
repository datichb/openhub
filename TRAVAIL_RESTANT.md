# 📋 Ce qui reste à faire - Tests opencode-hub

**Date** : 30 mai 2026  
**État actuel** : 965 tests, 41 fichiers  
**Couverture lib/** : ✅ 100% (15/15 modules testés)

> ⚠️ **Note importante** : Les commandes **cmd-agent** et **cmd-skills** sont **exclues** de ce plan car elles sont en cours de dépréciation et seront retirées du projet.

---

## ✅ CE QUI EST FAIT

### Modules lib/ - 100% testés (375 tests)
- ✅ adapter-manager.sh (5 tests)
- ✅ agent-picker.sh (21 tests)
- ✅ api-keys.sh (40 tests)
- ✅ colors.sh (14 tests)
- ✅ i18n.sh (19 tests)
- ✅ mcp-deploy.sh (16 tests)
- ✅ metrics.sh (46 tests)
- ✅ node-installer.sh (20 tests)
- ✅ progress-bar.sh (17 tests)
- ✅ project.sh (25 tests)
- ✅ prompt-builder.sh (65 tests)
- ✅ providers.sh (39 tests)
- ✅ session-state.sh (34 tests)
- ✅ spinner.sh (9 tests)
- ✅ tui-picker.sh (5 tests)

### Commandes cmd-* - Partiellement testées (14/26 commandes)
> Note : cmd-agent et cmd-skills exclus (dépréciés)
- ✅ test_cmd_beads.bats
- ✅ test_cmd_board.bats (Session 2)
- ✅ test_cmd_config.bats
- ✅ test_cmd_config_websearch.bats
- ✅ test_cmd_deploy.bats
- ✅ test_cmd_init.bats
- ✅ test_cmd_install.bats (Session 2)
- ✅ test_cmd_metrics.bats
- ✅ test_cmd_project.bats
- ✅ test_cmd_provider.bats
- ✅ test_cmd_remove.bats
- ✅ test_cmd_review.bats
- ✅ test_cmd_start.bats
- ✅ test_cmd_upgrade.bats

### Intégration - 2 workflows testés
- ✅ test_integration_agent_workflow.bats (Session 2)
- ✅ test_integration_deploy.bats
- ✅ test_integration_project_lifecycle.bats (Session 2)

### Autres tests (10 fichiers)
- ✅ test_agents_websearch.bats
- ✅ test_common.bats
- ✅ test_get_agent_model.bats
- ✅ test_helpers.bats
- ✅ test_hub_json_agent_models.bats
- ✅ test_integration_deploy.bats
- ✅ test_opencode_adapter.bats
- ✅ test_prompt_builder.bats
- ✅ test_skills_websearch.bats
- ✅ test_smoke.bats

---

## 🔄 CE QUI RESTE À FAIRE

### Phase 6 : Commandes cmd-* (12 commandes sans tests)

> ⚠️ **Exclusions** : cmd-agent et cmd-skills ne sont PAS inclus (en cours de dépréciation)

**PRIORITÉ HAUTE** (commandes critiques - 3 commandes)

1. **cmd-dashboard** (220 lignes) - 🔥🔥
   - Dashboard global projets
   - Estimation : 15-20 tests, ~180 lignes
   - Complexité : ⭐⭐ (agrégation données)

2. **cmd-audit** (220 lignes) - 🔥
   - Audit projet (sécurité, qualité)
   - Estimation : 15-20 tests, ~180 lignes
   - Complexité : ⭐⭐ (analyse fichiers)

3. **cmd-sync** (186 lignes) - 🔥
   - Synchronisation projets
   - Estimation : 12-15 tests, ~150 lignes
   - Complexité : ⭐⭐ (sync fichiers)

**Sous-total HAUTE** : ~42-55 tests, ~510 lignes, **6-7h**

**PRIORITÉ MOYENNE** (commandes utilitaires - 4 commandes)

6. **cmd-status** (172 lignes)
   - Statut détaillé projet
   - Estimation : 10-15 tests, ~120 lignes
   - Complexité : ⭐ (lecture état)

7. **cmd-quick** (144 lignes)
   - Commandes rapides
   - Estimation : 10-12 tests, ~100 lignes
   - Complexité : ⭐ (raccourcis)

8. **cmd-plugin** (128 lignes)
   - Gestion plugins
   - Estimation : 8-10 tests, ~100 lignes
   - Complexité : ⭐⭐ (load/unload plugins)

9. **cmd-debug** (127 lignes)
   - Debug et diagnostics
   - Estimation : 8-10 tests, ~80 lignes
   - Complexité : ⭐ (logs, traces)

**Sous-total MOYENNE** : ~36-47 tests, ~400 lignes, **4.5-5.5h**

---

**PRIORITÉ BASSE** (commandes simples - 5 commandes)

10. **cmd-help** (119 lignes)
    - Affichage aide
    - Estimation : 5-8 tests, ~60 lignes
    - Complexité : ⭐ (affichage texte)

11. **cmd-conventions** (94 lignes)
    - Conventions projet
    - Estimation : 5-8 tests, ~60 lignes
    - Complexité : ⭐ (affichage/validation)

12. **cmd-update** (50 lignes)
    - Mise à jour hub
    - Estimation : 3-5 tests, ~40 lignes
    - Complexité : ⭐ (git pull + rebuild)

13. **cmd-version** (17 lignes)
    - Affichage version
    - Estimation : 2-3 tests, ~20 lignes
    - Complexité : ⭐ (trivial)

14. **cmd-uninstall** (5 lignes)
    - Désinstallation
    - Estimation : 2-3 tests, ~20 lignes
    - Complexité : ⭐ (cleanup)

**Sous-total BASSE** : ~17-27 tests, ~200 lignes, **2-2.5h**

---

**Total Phase 6** : ~95-139 tests, ~1110 lignes, **12-15h** (au lieu de 15.5-19h avec agent/skills)

---

### Phase 7 : Tests d'intégration end-to-end

**7.1 : Workflows utilisateur complets**

1. **test_integration_mcp_workflow.bats** - 🔥
   - Install Node → build MCP → deploy → configure
   - Estimation : 10-12 tests, ~200 lignes
   - Complexité : ⭐⭐

2. **test_integration_multi_projects.bats** - 🔥
   - Créer plusieurs projets → switch → isolation
   - Estimation : 10-12 tests, ~180 lignes
   - Complexité : ⭐⭐

**Total 7.1** : ~20-24 tests, ~380 lignes, ~2h

**7.2 : Tests de performance**

1. **test_performance_registry.bats**
   - Scalabilité registre projets (100+ projets)
   - Benchmark : < 100ms pour 100 projets
   - Estimation : 5-8 tests, ~150 lignes
   - Complexité : ⭐⭐

2. **test_performance_prompts.bats**
   - Scalabilité contexte prompts (50+ fichiers)
   - Troncature correcte si dépassement
   - Estimation : 5-8 tests, ~150 lignes
   - Complexité : ⭐⭐

3. **test_concurrency_sessions.bats**
   - 10 sessions simultanées sans corruption
   - Race conditions
   - Estimation : 5-8 tests, ~120 lignes
   - Complexité : ⭐⭐⭐

**Total 7.2** : ~15-24 tests, ~420 lignes, ~2-3h

**7.3 : Tests de robustesse (edge cases)**

1. **test_edge_cases_corruption.bats**
   - Fichiers corrompus (projects.md, api-keys, hub.json)
   - Messages d'erreur clairs
   - Estimation : 10-12 tests, ~150 lignes
   - Complexité : ⭐⭐

2. **test_edge_cases_permissions.bats**
   - Fichiers en lecture seule
   - Permissions incorrectes (600 vs 644)
   - Estimation : 8-10 tests, ~120 lignes
   - Complexité : ⭐⭐

3. **test_edge_cases_special_chars.bats**
   - PROJECT_ID avec unicode, emojis, espaces
   - Chemins avec caractères spéciaux
   - Estimation : 8-10 tests, ~100 lignes
   - Complexité : ⭐⭐

4. **test_edge_cases_limits.bats**
   - Chemins très longs (>255 chars)
   - Très grand nombre de labels/agents
   - Estimation : 5-8 tests, ~80 lignes
   - Complexité : ⭐

**Total 7.3** : ~31-40 tests, ~450 lignes, ~2-3h

**Total Phase 7** : ~66-88 tests, ~1250 lignes, **6-8h** (au lieu de 7-10h avec workflows supplémentaires)

---

### Phase 8 : Amélioration tests existants (optionnel)

**8.1 : Résoudre tests skippés**

Actuellement plusieurs tests sont skippés dans :
- test_cmd_project.bats (4 skippés)
- test_lib_project.bats (4 skippés)
- test_lib_prompt_builder.bats (8 skippés - tests lents)

Actions :
- Investiguer comportement réel vs attendu
- Fixer ou adapter les tests
- Optimiser tests lents

**Estimation** : 2-3h

**8.2 : Augmenter couverture tests existants**

Certaines commandes déjà testées pourraient avoir plus de couverture :
- cmd-beads : edge cases supplémentaires
- cmd-deploy : scénarios d'erreur
- cmd-init : cas de figure complexes

**Estimation** : 3-4h

**Total Phase 8** : ~5-7h

---

## 🚀 Phase 9 : Optimisation des performances des tests

### 📊 Analyse de l'état actuel

**Configuration CI actuelle** :
- ✅ Parallélisation activée : `bats -j 4` (4 jobs parallèles)
- ✅ Exclusion tests websearch (workflow séparé)
- ❌ Pas de cache de dépendances
- ❌ Pas de timeout configuré
- ❌ Pas de retry sur flaky tests

**Points de ralentissement identifiés** :

1. **Tests avec sleep/wait** (12 occurrences)
   - test_cmd_deploy.bats : 5x `sleep 1` (5 secondes perdues)
   - test_integration_deploy.bats : 2x `sleep 1` (2 secondes)
   - test_lib_metrics.bats : 2x `sleep 1` (2 secondes)
   - test_lib_spinner.bats : 3x `sleep 0.1-0.2` (0.5 seconde)

2. **Setup/teardown lourds** (18 fichiers avec 30+ lignes)
   - test_cmd_init.bats : 99 lignes de setup
   - test_cmd_beads.bats : 103 lignes de setup
   - test_cmd_review.bats : 91 lignes de setup
   - test_cmd_start.bats : 89 lignes de setup

3. **Fichiers volumineux** (4 fichiers avec 45+ tests)
   - test_prompt_builder.bats : 73 tests
   - test_lib_prompt_builder.bats : 65 tests
   - test_common.bats : 55 tests
   - test_cmd_provider.bats : 48 tests

4. **Tests skippés conditionnels**
   - ~20 tests avec `skip` conditionnel (jq non disponible, etc.)
   - Overhead de vérification à chaque run

---

### 🎯 Plan d'optimisation par niveau

### 🟢 Niveau 1 : Quick wins (1-2h, gain 20-30%)

**A. Réduire les sleep inutiles**
```bash
# Dans test_cmd_deploy.bats, test_integration_deploy.bats, test_lib_metrics.bats
# Avant:
sleep 1

# Après:
sleep 0.1  # Suffisant pour la plupart des cas asynchrones
```
- **Localisation** : 9 occurrences à optimiser
- **Gain estimé** : 5-8 secondes par run

**B. Cache CI des dépendances**
```yaml
# .github/workflows/ci.yml
- name: Cache bats
  uses: actions/cache@v3
  with:
    path: |
      /usr/bin/bats
      tests/fixtures
    key: bats-${{ runner.os }}-${{ hashFiles('tests/**') }}

- name: Install bats-core
  run: |
    if ! command -v bats &> /dev/null; then
      sudo apt-get install -y bats
    fi
```
- **Gain estimé** : 10-15 secondes par run

**C. Parallélisation accrue**
```yaml
# Auto-detect CPU count au lieu de hardcoded -j 4
- name: Run bats tests
  run: |
    CPU_COUNT=$(nproc)
    bats -j $CPU_COUNT "${tests_to_run[@]}" || exit 1
```
- **Gain estimé** : 15-20% supplémentaire si CPU disponible

**D. Timeout global**
```yaml
- name: Run bats tests
  timeout-minutes: 3  # Fail si > 3 minutes
  run: |
    bats -j $(nproc) "${tests_to_run[@]}" || exit 1
```
- **Gain estimé** : Prévention des hangs (actuellement non mesurable)

**Résultat Niveau 1** : ~20-30% de gain (ex: 60s → 42-48s)  
**Effort** : 1-2h  
**ROI** : ⭐⭐⭐⭐⭐

---

### 🟡 Niveau 2 : Optimisations moyennes (2-3h, gain 10-15%)

**A. Factoriser les setup lourds**
```bash
# Utiliser setup_file() au lieu de setup() pour ressources partagées
setup_file() {
  # Créer fixtures communes une seule fois pour tout le fichier
  export SHARED_FIXTURES="$BATS_FILE_TMPDIR/fixtures"
  mkdir -p "$SHARED_FIXTURES"
  # ...
}

setup() {
  # Setup spécifique à chaque test (léger)
  TEST_DIR="$BATS_TEST_TMPDIR"
}
```
- **Cibles** : test_cmd_init.bats (99L), test_cmd_beads.bats (103L)
- **Gain estimé** : 5-10 secondes par fichier

**B. Lazy loading des modules**
```bash
# Dans helpers.bash - ne sourcer que si nécessaire
_source_module() {
  local module="$1"
  [[ -n "${_LOADED_MODULES[$module]:-}" ]] && return 0
  source "$LIB_DIR/${module}.sh"
  _LOADED_MODULES[$module]=1
}
```
- **Gain estimé** : 2-5 secondes par run

**C. Optimiser les tests conditionnels**
```bash
# Au lieu de skip dans chaque test:
@test "fonction : test nécessitant jq" {
  which jq >/dev/null 2>&1 || skip "jq non disponible"
  # ...
}

# Grouper dans fichiers dédiés avec skip au niveau fichier:
setup_file() {
  which jq >/dev/null 2>&1 || skip "jq requis pour ce fichier"
}
```
- **Gain estimé** : 1-3 secondes par run

**D. Réduire I/O disque**
```bash
# Utiliser tmpfs pour TEST_DIR (CI Linux)
export TEST_DIR="${TMPDIR:-/tmp}/opencode-tests-$$"
# Sur CI, TMPDIR peut être configuré en tmpfs (RAM)
```
- **Gain estimé** : 5-10 secondes par run

**Résultat Niveau 2** : ~10-15% de gain supplémentaire  
**Effort** : 2-3h  
**ROI** : ⭐⭐⭐⭐

---

### 🔴 Niveau 3 : Optimisations avancées (3-5h, gain 5-10% ou beaucoup plus)

**A. Parallélisation par suite (Matrix Strategy)**
```yaml
# .github/workflows/ci.yml
strategy:
  matrix:
    suite: [cmd, lib, integration, common]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Run ${{ matrix.suite }} tests
        run: |
          case "${{ matrix.suite }}" in
            cmd) files="tests/test_cmd_*.bats" ;;
            lib) files="tests/test_lib_*.bats" ;;
            integration) files="tests/test_integration_*.bats" ;;
            common) files="tests/test_{common,helpers,smoke}.bats" ;;
          esac
          bats -j $(nproc) $files
```
- **Gain estimé** : 30-40% si 4 runners (mais coût CI++)
- **Trade-off** : Minutes CI utilisées x4

**B. Caching intelligent inter-runs**
```yaml
- name: Cache test results
  uses: actions/cache@v3
  with:
    path: .bats-cache
    key: tests-${{ hashFiles('tests/**', 'scripts/**') }}
# Skip tests dont le code n'a pas changé
```
- **Gain estimé** : 20-30% sur runs sans changements majeurs

**C. Profiling et bottleneck analysis**
```bash
# Ajouter timestamps détaillés
export BATS_VERBOSE=1
bats --formatter tap tests/*.bats > results.tap

# Analyser les tests les plus lents
grep -E "^ok|^not ok" results.tap | \
  awk '{print $NF " " $(NF-1)}' | sort -rn | head -20
```
- **Gain estimé** : Variable selon bottlenecks identifiés

**D. Tests incrémentaux (Git-aware)**
```bash
# Ne run que les tests affectés par les changements
changed_files=$(git diff --name-only HEAD~1)
affected_tests=$(determine_affected_tests "$changed_files")
bats $affected_tests
```
- **Gain estimé** : 50-80% sur PRs (mais complexité++)

**Résultat Niveau 3** : ~5-10% de gain (ou 30-40% avec matrix)  
**Effort** : 3-5h  
**ROI** : ⭐⭐⭐ (diminue avec complexité)

---

### 🎯 Recommandations optimisation

**Pour 888 tests actuels** (temps estimé : 40-60s) :
- ✅ **Niveau 1 maintenant** si temps CI > 60s
- ⏳ **Niveau 2 plus tard** quand vous atteignez 1100+ tests
- ⏳ **Niveau 3 si nécessaire** uniquement si > 1500 tests ou CI > 2 min

**Ordre d'exécution recommandé** :

1️⃣ **Implémenter tests prioritaires** (Phase 6-7) - 10-13h  
2️⃣ **Optimisation Niveau 1** si temps CI devient problématique - 1-2h  
3️⃣ **Continuer tests restants** si souhaité - 8-10h  
4️⃣ **Optimisation Niveau 2** si > 1100 tests - 2-3h

**Pourquoi cet ordre ?**
- Avec 888 tests, les performances sont probablement acceptables
- Optimiser trop tôt = temps perdu (optimisation prématurée)
- Mieux vaut avoir plus de tests avec couverture solide
- Optimiser quand le besoin se fait vraiment sentir

**Métriques cibles** :
- ✅ Acceptable : < 1 minute pour < 1000 tests
- ⚠️ À surveiller : 1-2 minutes pour 1000-1500 tests
- 🔴 Action requise : > 2 minutes pour < 1500 tests

---

### 💡 Actions concrètes Niveau 1 (Quick wins)

Si vous décidez d'optimiser maintenant, voici les 4 changements précis à faire :

**1. Réduire sleep (10 min)**
```bash
# Fichiers à modifier:
# - tests/test_cmd_deploy.bats (lignes 76, 102, 112, 123, 131)
# - tests/test_integration_deploy.bats (lignes 140, 163)
# - tests/test_lib_metrics.bats (lignes 161, 359)

# Remplacer: sleep 1
# Par: sleep 0.1
```

**2. Ajouter cache CI (5 min)**
```yaml
# .github/workflows/ci.yml - après checkout, avant install
- name: Cache bats and fixtures
  uses: actions/cache@v3
  with:
    path: |
      /usr/bin/bats
      ~/.cache/bats
    key: bats-${{ runner.os }}-v1
    restore-keys: bats-${{ runner.os }}-
```

**3. Parallélisation auto (2 min)**
```yaml
# .github/workflows/ci.yml - modifier la ligne bats
- name: Run bats tests
  run: bats -j $(nproc) "${tests_to_run[@]}" || exit 1
```

**4. Timeout (2 min)**
```yaml
# .github/workflows/ci.yml - ajouter au job
- name: Run bats tests
  timeout-minutes: 3
  run: bats -j $(nproc) "${tests_to_run[@]}" || exit 1
```

**Résultat attendu** : ~20-30% de gain avec moins de 20 min d'effort

---

**Estimation Phase 9** :
- Niveau 1 : 1-2h (recommandé si CI > 60s)
- Niveau 2 : 2-3h (si > 1100 tests)
- Niveau 3 : 3-5h (si > 1500 tests ou besoin critique)

---

## 📊 ESTIMATION GLOBALE DU TRAVAIL RESTANT

> ⚠️ **Mise à jour** : Estimations recalculées sans cmd-agent et cmd-skills (dépréciés)

### Résumé par priorité

**HAUTE PRIORITÉ** (recommandé)
- Phase 6 (3 commandes critiques) : ~42-55 tests, ~510 lignes, 6-7h
- Phase 7.1 (intégration workflows) : ~20-24 tests, ~380 lignes, 2h
- **Total HAUTE** : ~62-79 tests, ~890 lignes, **8-9h**

**MOYENNE PRIORITÉ** (optionnel)
- Phase 6 (4 commandes utilitaires) : ~36-47 tests, ~400 lignes, 4.5-5.5h
- Phase 7.2 (tests performance) : ~15-24 tests, ~420 lignes, 2-3h
- **Total MOYENNE** : ~51-71 tests, ~820 lignes, **6.5-8.5h**

**BASSE PRIORITÉ** (nice to have)
- Phase 6 (5 commandes simples) : ~17-27 tests, ~200 lignes, 2-2.5h
- Phase 7.3 (edge cases) : ~31-40 tests, ~450 lignes, 2-3h
- Phase 8 (amélioration existants) : 5-7h
- Phase 9 (optimisation perfs) : 1-5h (selon niveau)
- **Total BASSE** : ~48-67 tests, ~650 lignes, **10-14.5h**

### Objectif final si TOUT est complété

**Tests totaux visés** : 965 (actuel) + 161-217 (restant) = **~1126-1182 tests**  
**Fichiers totaux visés** : 41 (actuel) + 18-23 (nouveaux) = **~59-64 fichiers**  
**Temps total estimé** : **25-32 heures** (au lieu de 29-38h avant Session 2)  
**Couverture estimée finale** : **~85-90%** du code critique

---

## 🎯 RECOMMANDATIONS ACTUALISÉES

> ⚠️ **Maj** : Sans cmd-agent et cmd-skills (dépréciés)

### Scénario 1 : Minimum viable (8-9h) ⭐
✅ **Phase 6 - 3 commandes critiques** (cmd-dashboard, cmd-audit, cmd-sync)  
✅ **Phase 7.1 - Workflows intégration** (MCP, multi-projets)

**Résultat** : ~1027-1044 tests, couverture **77-80%**

### Scénario 2 : Couverture solide (15-17h) ⭐⭐
✅ Scénario 1  
✅ **Phase 6 - 4 commandes utilitaires** (cmd-status, cmd-quick, cmd-plugin, cmd-debug)  
✅ **Phase 7.2 - Tests performance**

**Résultat** : ~1078-1115 tests, couverture **80-83%**

### Scénario 3 : Exhaustif (25-32h) ⭐⭐⭐
✅ Scénario 2  
✅ **Phase 6 - 5 commandes simples** (cmd-help, cmd-conventions, cmd-update, cmd-version, cmd-uninstall)  
✅ **Phase 7.3 - Edge cases**  
✅ **Phase 8 - Améliorations existants**  
✅ **Phase 9 - Optimisation performances** (si nécessaire)

**Résultat** : ~1126-1182 tests, couverture **85-90%**

---

## 🚀 PROCHAINE SESSION - Plan d'action

> ✅ **Session 2 complétée !** : cmd-board + cmd-install + 2 workflows intégration

Si vous voulez continuer, je recommande de commencer par :

**Session 3 (4-5h)** : **cmd-dashboard + cmd-audit + cmd-sync + test_integration_mcp_workflow**
- 3 commandes importantes pour monitoring
- Workflow MCP complet
- ~52-67 tests
- Finalisation des fonctionnalités critiques

**Session 4 (3-4h)** : **cmd-status + cmd-quick + cmd-plugin + test_performance**
- 3 commandes utilitaires
- Tests de performance (scalabilité)
- ~51-71 tests
- Couverture medium priority complète

Avec ces 2 sessions supplémentaires (7-9h), vous atteignez :
- ✅ **~1068-1113 tests** (+10-15% supplémentaire)
- ✅ **Couverture ~80-83%**
- ✅ **Toutes les fonctionnalités critiques + utilitaires testées**

Le reste (cmd-debug, cmd-help, edge cases, etc.) a un ROI décroissant car moins critique.

---

**État actuel** : ✅✅ **Très bonne couverture** (965 tests, 73-76% code critique)  
**État si Session 3** : ✅✅✅ **Excellente couverture** (~1017-1032 tests, 77-80%)  
**État si Session 4** : 🏆 **Couverture production-ready** (~1068-1103 tests, 80-83%)  
**État si Scénario 3** : 🏆🏆 **Couverture exceptionnelle** (~1126-1182 tests, 85-90%)

Le choix dépend de vos contraintes de temps et du niveau de confiance souhaité !
