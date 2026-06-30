# Analyse de migration CLI — Bash → Langage typé

> Document d'historisation produit après l'analyse complète du système CLI (`oc`) réalisée en juin 2026.
> Référence les problèmes identifiés dans la base de code actuelle, leur résolution attendue par une migration, et la recommandation de langage cible.

---

## Contexte

Le CLI `oc` (openhub) est implémenté en **100% Bash** et compte :

- 1 point d'entrée principal (`oc.sh`) + 1 secondaire (`ocp.sh`)
- 30 fichiers de commandes (`scripts/cmd-*.sh`)
- 23 bibliothèques (`scripts/lib/*.sh`)
- 1 adaptateur (`scripts/adapters/opencode.adapter.sh`)
- ~3 135 lignes sourcées à **chaque invocation**
- Compatibilité bash 3.2 (macOS stock) maintenue explicitement

La suite de tests compte 80+ fichiers `.bats`. Les serveurs MCP du projet sont eux déjà en TypeScript.

---

## Problèmes résolus par la migration

### 1. Performance

#### 1.1 Double-sourcing — coût startup x2

**Problème :** `oc.sh` source `common.sh` (et ses 6 librairies) puis lance chaque commande via `bash "$SCRIPTS_DIR/cmd-*.sh"`, qui re-source `common.sh` dans un nouveau processus.

```
oc.sh         → source common.sh (3 135 lignes + 1 jq fork pour resolve_oc_lang)
  └─ bash cmd-version.sh → source common.sh à nouveau (3 135 lignes + 1 jq fork)
                            → jq pour lire la version
```

**Résultat mesuré :** Pour `oc version` (17 lignes utiles), le coût est :
- 2 processus bash
- ~6 270 lignes parsées
- 3 appels `jq`

**Fichiers :** `oc.sh:9` (source), `oc.sh:17-50` (dispatch via `bash`), `scripts/common.sh:31-55` (6 sources inconditionnels)

**Résolution par la migration :** Module system natif (Python `import` / Node `require` / Bun dynamic `import()`). Chaque module n'est chargé qu'une fois, les imports sont résolus à la compilation ou en cache mémoire.

---

#### 1.2 `i18n.sh` chargé inconditionnellement — 1 690 lignes pour toute invocation

**Problème :** `common.sh:31` source `lib/i18n.sh` systématiquement, quelle que soit la commande. Ce fichier contient une table de chaînes EN/FR sous forme d'un unique `case` statement de 1 690 lignes.

**Fichiers :** `scripts/common.sh:31`, `scripts/lib/i18n.sh` (1 690 lignes)

**Résolution par la migration :** Fichiers de traduction JSON/YAML par locale, chargés à la demande au premier appel de traduction. `i18next` (Node) ou `babel` (Python) gèrent cela nativement.

---

#### 1.3 Board watch mode — ~130 forks de processus par tick (toutes les 5s)

**Problème :** `cmd-board.sh` re-rend le kanban toutes les 5 secondes. Chaque rendu lance :
- 1 appel `bd list` (processus externe)
- 4 appels `jq` pour compter les statuts (todo/in_progress/done/blocked)
- 1 appel `jq` pour le titre du ticket en cours
- **6 appels `jq` par ticket** (id, titre, priorité, type, longueur, statut)

Avec 20 tickets : `1 + 4 + 1 + (6 × 20)` = **126 processus forkés par tick**.

```bash
# cmd-board.sh:165-172 — 5 appels jq séparés sur la même donnée
done_count=$(echo "$bd_output" | jq '[.[] | select(.status == "done")] | length')
inprogress_count=$(echo "$bd_output" | jq '[.[] | select(.status == "in_progress")] | length')
todo_count=$(echo "$bd_output" | jq '[.[] | select(.status == "todo"...)] | length')
blocked_count=$(echo "$bd_output" | jq '[.[] | select(.status == "blocked")] | length')
current_ticket_title=$(echo "$bd_output" | jq -r '[...] | first | .title // ""')
```

**Fichiers :** `scripts/cmd-board.sh:143-172`, `scripts/cmd-board.sh:366-372` (boucle watch)

**Résolution par la migration :** JSON natif dans les deux langages. Zéro fork pour parser, filtrer, compter. Un seul passage en mémoire remplace 130 processus.

---

#### 1.4 `hub.json` lu 7 fois par `jq` dans `cmd-start.sh`

**Problème :** Le résumé de configuration avant lancement d'une session lit le même fichier 7 fois via 7 appels `jq` séparés.

```bash
# scripts/cmd-start.sh:483-507
_cfg_lang=$(jq -r '.cli.language // "—"' "$HUB_CONFIG")
_cfg_cache_auto=$(jq -r '.opencode_cache.compaction.auto // false' "$HUB_CONFIG")
_cfg_cache_reserved=$(jq -r '.opencode_cache.compaction.reserved // ""' "$HUB_CONFIG")
_cfg_disabled=$(jq -r '.opencode.disabled_native_agents // [] | join(", ")' "$HUB_CONFIG")
_cfg_plugins=$(jq -r '.plugin // [] | join(", ")' "$_proj_opencode_json")
_cfg_version=$(jq -r '.version // "—"' "$HUB_CONFIG")
# + 1 appel supplémentaire provider
```

**Fichiers :** `scripts/cmd-start.sh:483-507`

**Résolution par la migration :** `JSON.parse()` une fois → accès propriétés en mémoire.

---

#### 1.5 `_get_project_field()` — un awk par champ, sur le même fichier

**Problème :** Chaque getter de propriété projet (`get_project_tracker`, `get_project_language`, `get_project_labels`, `get_project_agents`, `get_project_mcp`) appelle indépendamment `awk` sur `projects.md`. Lire 5 propriétés d'un projet = 5 scans complets du fichier.

```bash
# scripts/lib/project.sh:188 — pattern répété pour chaque getter
_get_project_field() {
  awk -v id="$1" -v field="$2" '...' "$PROJECTS_FILE"
}
```

**Fichiers :** `scripts/lib/project.sh:188-197`, `scripts/lib/project.sh:244-828` (tous les setters/getters)

**Résolution par la migration :** Parse unique du fichier de registre vers un objet/dict en mémoire. Accès à toutes les propriétés en O(1) après.

---

#### 1.6 `detect_stack()` — ~25 appels `grep` par piping

**Problème :** La détection de stack technologique dans `prompt-builder.sh` lance ~25 `echo "$deps" | grep` pour détecter chaque framework. Chaque pipe = 2 forks (echo + grep).

**Fichiers :** `scripts/lib/prompt-builder.sh:509-577`

**Résolution par la migration :** Regex en mémoire sur une string. Zéro fork.

---

#### 1.7 `cat` dans des subshells au lieu de `$(<file)`

**Problème :** Plusieurs lectures de fichiers utilisent `$(cat "$file")` (fork) au lieu du builtin bash `$(<"$file")` (sans fork). Mineur, mais symptomatique de la difficulté à écrire du bash performant.

```bash
# scripts/lib/prompt-builder.sh:552-574
py_deps=$(cat "$pyproject" 2>/dev/null)
jvm_deps=$(cat "$build_gradle" 2>/dev/null)
```

**Fichiers :** `scripts/lib/prompt-builder.sh:552-574`

**Résolution par la migration :** `fs.readFileSync()` / `open().read()` — toujours en mémoire, jamais de fork.

---

### 2. Cohérence

#### 2.1 Deux patterns d'argument parsing coexistent

**Problème :** Le parsing des arguments est réimplémenté manuellement dans chaque commande. Deux patterns incompatibles coexistent :

- **Pattern A** (`_prev` + boucle `for`) — ~20 fichiers : `cmd-start.sh`, `cmd-audit.sh`, `cmd-deploy.sh`, `cmd-worktree.sh`, `cmd-beads.sh`...
- **Pattern B** (`while/shift`) — 4 fichiers : `cmd-metrics.sh:42`, `cmd-optimize.sh:32`, `cmd-yield.sh:35`, `cmd-service.sh:60`
- **Pattern C** (boucle `for` simplifiée) — 1 fichier : `cmd-status.sh:12`

**Impact :** Maintenance difficile, bugs de parsing asymétriques entre commandes, impossibilité d'extraire un helper commun en bash.

**Résolution par la migration :** Un seul framework CLI (`commander`/`yargs` en Node, `click`/`typer` en Python) gère le parsing pour toutes les commandes avec une API déclarative.

---

#### 2.2 `--help` absent sur 23/30 commandes

**Problème :** Seules 7 commandes supportent `--help/-h` en tant que flag. Les 23 autres ignorent le flag, déclenchent l'action ou affichent une erreur.

| Commandes AVEC `--help` | Commandes SANS `--help` |
|---|---|
| `cmd-quick.sh`, `cmd-service.sh`, `cmd-worktree.sh`, `cmd-beads.sh`, `cmd-config.sh`, `cmd-skills.sh`, `cmd-agent.sh` | `cmd-start.sh`, `cmd-deploy.sh`, `cmd-audit.sh`, `cmd-review.sh`, `cmd-debug.sh`, `cmd-init.sh`, `cmd-status.sh`, `cmd-metrics.sh`, `cmd-dashboard.sh`, `cmd-conventions.sh`, `cmd-sync.sh`, `cmd-optimize.sh`, `cmd-yield.sh`, `cmd-install.sh`, `cmd-remove.sh`, `cmd-plugin.sh`, `cmd-project.sh`, `cmd-upgrade.sh`, `cmd-update.sh`, `cmd-version.sh`, `cmd-board.sh`, `cmd-worktree.sh`, `cmd-yield.sh` |

**Résolution par la migration :** Généré automatiquement par tous les frameworks CLI modernes à partir des définitions de commandes.

---

#### 2.3 Pas d'autocompletion shell

**Problème :** Aucun script de completion zsh/bash/fish n'est généré ou fourni. `grep "completion\|autocomplete\|compgen\|_oc_comp"` ne retourne aucun résultat. Sur 30+ commandes avec des dizaines de flags, l'absence de completion est un frein UX significatif.

**Résolution par la migration :** `click` (Python) génère les completions avec `oc --install-completion`. `commander`/`oclif` (Node) proposent la même fonctionnalité.

---

#### 2.4 Collision du flag `-d`

**Problème :** Le flag court `-d` a des significations différentes selon la commande :

| Commande | Fichier:Ligne | Signification de `-d` |
|---|---|---|
| `cmd-start.sh` | `cmd-start.sh:13` | `--dev` (mode développement) |
| `cmd-metrics.sh` | `cmd-metrics.sh:44` | `--period` (période temporelle) |
| `cmd-optimize.sh` | `cmd-optimize.sh:32` | `--period` (période temporelle) |
| `cmd-yield.sh` | `cmd-yield.sh:37` | `--period` (période temporelle) |

**Résolution par la migration :** Les frameworks CLI détectent et lèvent une erreur à la déclaration si un flag court est réutilisé avec une sémantique différente au sein d'un même scope.

---

#### 2.5 Style de prompts interactifs inconsistant

**Problème :** 50% des commandes utilisent le helper `_prompt()` (TUI intégré avec gouttière `│`), l'autre 50% utilisent `read -rp` brut (sans style). La dichotomie existe parfois au sein du même fichier.

**Pattern A — TUI intégré (`_prompt`) :** `cmd-start.sh`, `cmd-audit.sh`, `cmd-review.sh`, `cmd-service.sh`, `cmd-remove.sh`

**Pattern B — `read -rp` brut :** `cmd-quick.sh:71`, `cmd-install.sh:15,37,72,144,238`, `cmd-deploy.sh:524`, `cmd-skills.sh:125,137,346,495`, `cmd-agent.sh:226,289,350,477,648,732`, `cmd-config.sh:432,539,646,784`, `cmd-beads.sh:180,182,388,504`

**Résolution par la migration :** Une seule librairie de prompts (`@clack/prompts` ou `inquirer` en Node, `questionary` ou `rich` en Python) pour toutes les interactions.

---

#### 2.6 `resolve_oc_lang` absent dans 20/30 commandes

**Problème :** La fonction `resolve_oc_lang` doit être appelée en début de chaque commande pour que `t()` retourne les chaînes dans la bonne locale. Elle est absente de 20/30 commandes, ce qui fait tomber `t()` en fallback anglais alors que l'interface est en français.

**Commandes manquantes :** `cmd-metrics.sh`, `cmd-dashboard.sh`, `cmd-optimize.sh`, `cmd-yield.sh`, `cmd-deploy.sh`, `cmd-sync.sh`, `cmd-upgrade.sh`, `cmd-install.sh`, `cmd-update.sh`, `cmd-version.sh`, `cmd-audit.sh`, `cmd-review.sh`, `cmd-debug.sh`, `cmd-conventions.sh`, `cmd-board.sh`, `cmd-worktree.sh`, `cmd-status.sh`, `cmd-beads.sh`, `cmd-config.sh`, `cmd-init.sh`

**Résolution par la migration :** La locale est résolue une fois au bootstrap de l'application et propagée automatiquement à toutes les fonctions via le contexte.

---

#### 2.7 i18n incomplète — 20/30 commandes ont des chaînes françaises hardcodées

**Problème :** Malgré l'existence d'un système i18n (`t()`), la majorité des commandes contiennent des chaînes d'interface directement en français dans le code. Certains fichiers n'appellent `t()` à aucun moment.

| Commandes 100% hardcodées (aucun appel `t()`) | Commandes partiellement hardcodées |
|---|---|
| `cmd-metrics.sh` — toutes les chaînes : `"Métriques OpenCode Hub"`, `"Sessions"`, `"Aujourd'hui"` | `cmd-start.sh` — ex. `"✅ Cache valide"` (`cmd-start.sh:162`), `"Nouveau sur ce projet ?"` (`cmd-start.sh:198`) |
| `cmd-dashboard.sh` — `"Budget"`, `"Projets"`, `"Sessions récentes"` | `cmd-deploy.sh` — `"Vérification de fraîcheur"` (`cmd-deploy.sh:30`), `"Choisir un projet"` (`cmd-deploy.sh:518`), `"Phase 1..."` (`cmd-deploy.sh:494`) |
| `cmd-optimize.sh` — `"Analyse d'optimisation"`, `"Critique"` | `cmd-audit.sh` — `log_error` sans `t()` (`cmd-audit.sh:38,62`) |
| `cmd-yield.sh` — `"Yield — Sessions ↔ Commits git"` | `cmd-service.sh` — `"--project requiert un PROJECT_ID"` (`cmd-service.sh:63`) |
| `cmd-worktree.sh` — toutes les chaînes utilisateur | `cmd-status.sh` — `"API configurée"` (`cmd-status.sh:117`), `"Tracker : $tracker"` (`cmd-status.sh:127`) |
| `cmd-project.sh` — toutes les chaînes utilisateur | `cmd-quick.sh` — `"Choisir un projet :"` (`cmd-quick.sh:65`), `"Choix invalide"` (`cmd-quick.sh:73`) |
| `cmd-agent.sh` — presque toutes les chaînes | `cmd-beads.sh` — mix dans la section tracker setup |
| `cmd-board.sh` — 1 seul appel `t()` (`cmd-board.sh:152`) | `cmd-conventions.sh` — `"Aucun projet enregistré"` (`cmd-conventions.sh:31`) |

**Résolution par la migration :** La migration implique une réécriture complète — l'extraction systématique des chaînes dans les fichiers de traduction est une étape naturelle du processus.

---

### 3. Sécurité

#### 3.1 Remplacement Perl sans échappement côté substitution

**Problème :** Dans `cmd-project.sh`, les commandes `perl -i` utilisent `\Q...\E` pour la partie *match* (protège des métacaractères regex), mais le côté *remplacement* reçoit les variables utilisateur sans échappement. Une valeur de `new_path` contenant `}` ou des constructions `${...}` peut casser le remplacement ou produire un comportement inattendu.

```bash
# scripts/cmd-project.sh:65 — old_id protégé, new_id exposé côté remplacement
perl -i -0777pe "s{^## \Q${old_id}\E$}{## ${new_id}}mg" "$PROJECTS_FILE"

# scripts/cmd-project.sh:151 — new_path non échappé dans la substitution
perl -i -pe "s{^\Q${project_id}\E=.*}{${project_id}=${new_path}}" "$PATHS_FILE"
```

**Fichiers :** `scripts/cmd-project.sh:65`, `scripts/cmd-project.sh:151`

**Résolution par la migration :** Plus de `perl -i` ni de shell piping. Les modifications de fichiers se font via des API de parsing (JSON/YAML/TOML) ou des opérations string en mémoire sans interpolation shell.

---

#### 3.2 `curl | bash` dans les scripts d'installation

**Problème :** L'installation de dépendances exécute des scripts distants sans vérification d'intégrité (ni hash, ni signature).

```bash
# install.sh:187-188
curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -

# install.sh:230
curl -fsSL https://bun.sh/install | bash

# scripts/lib/node-installer.sh:52
curl -sf https://get.volta.sh | bash

# scripts/lib/node-installer.sh:75
curl -o- "https://raw.githubusercontent.com/nvm-sh/nvm/${v}/install.sh" | bash
```

**Fichiers :** `install.sh:187,230`, `scripts/lib/node-installer.sh:52,75`

**Résolution par la migration :** Un package publié sur npm/PyPI s'installe via `npm install -g openhub` ou `pip install openhub`. Les gestionnaires de paquets vérifient les checksums et signatures automatiquement.

---

#### 3.3 Token AWS visible dans `ps aux`

**Problème :** Le bearer token AWS Bedrock est transmis via une variable d'environnement préfixée à la commande `exec`. Cette variable est visible dans la liste des processus (`ps aux`, `/proc/<pid>/environ`) par tous les processus du même utilisateur.

```bash
# scripts/adapters/opencode.adapter.sh:1051
exec env AWS_BEARER_TOKEN_BEDROCK="$bearer_token" opencode ${args[@]+"${args[@]}"}
```

**Fichiers :** `scripts/adapters/opencode.adapter.sh:1051`

**Résolution par la migration :** Passage du token via stdin, fichier temporaire à permissions 600, ou intégration keychain OS (`keytar` Node, `keyring` Python).

---

### 4. Robustesse

#### 4.1 Pas de file locking — corruption possible en usage concurrent

**Problème :** Aucun mécanisme de verrouillage (`flock`, lockfile) ne protège les fichiers partagés (`projects.md`, `api-keys.local.md`, `hub.json`). Deux invocations simultanées de `oc config set` ou `oc init` peuvent lire le même état initial, modifier en parallèle, et écrire des résultats en conflit, corrompant silencieusement les fichiers.

Les tests de concurrence (`tests/test_concurrency_sessions.bats`) valident les lectures parallèles mais pas les écritures concurrentes.

**Fichiers concernés :** `scripts/cmd-config.sh:77-108`, `scripts/lib/project.sh:244-270`

**Résolution par la migration :** `fcntl.flock()` (Python), `proper-lockfile` ou `fs.open()` avec flags exclusifs (Node). Trivial à implémenter dans les deux langages.

---

#### 4.2 `perl -i` non-atomique — risque de corruption sur interruption

**Problème :** Toutes les fonctions `_set_project_*` de `project.sh` utilisent `perl -i` (édition en place). Cette opération est non-atomique : elle lit le fichier, le tronque, puis le réécrit. Un kill ou une coupure de courant entre la troncature et la réécriture produit un `projects.md` corrompu ou vide.

```bash
# scripts/lib/project.sh:252 — pattern répété pour tous les setters
perl -i -pe "s{(^${id}\s*\|[^|]*\|[^|]*\|)[^|]*(\|)}{${1}${new_value}${2}}" "$PROJECTS_FILE"
```

Ce pattern est utilisé dans **toutes les fonctions `_set_project_*`** de `scripts/lib/project.sh` (lignes 244-828).

**Résolution par la migration :** Parse en mémoire → modification → écriture atomique (`tempfile + rename()`). Aucun risque de fichier tronqué à mi-chemin.

---

#### 4.3 Chemins avec espaces silencieusement cassés

**Problème :** La fonction `get_project_path` supprime tous les espaces des chemins via `tr -d ' '`. Un projet dont le chemin contient un espace (ex: `/Users/john/My Projects/my-app`) sera silencieusement tronqué en `/Users/john/MyProjects/my-app`, produisant une erreur "dossier introuvable" sans indication de la cause réelle.

```bash
# scripts/lib/project.sh:131
grep "^${id}=" "$PATHS_FILE" | head -1 | cut -d'=' -f2- | tr -d ' ' || true
```

Ce comportement est documenté comme limitation dans `tests/test_edge_cases_special_chars.bats:123-138`.

**Fichiers :** `scripts/lib/project.sh:131`

**Résolution par la migration :** `pathlib.Path` (Python) ou `path.resolve()` (Node) traitent nativement les espaces dans les chemins.

---

#### 4.4 Deploy interrompu laisse un état inconsistant

**Problème :** `oc deploy` s'exécute en 4 phases séquentielles (agents, skills, config, MCP). Si le processus est interrompu (Ctrl+C, crash) après la Phase 1 (agents déployés) mais avant la Phase 3 (mise à jour d'`opencode.json`), le projet se retrouve avec de nouveaux agents mais une config pointant vers les anciens. Aucun mécanisme de rollback n'existe pour les phases 1 à 3.

```
Phase 1 : déploiement .opencode/agents/*.md  ← si interrompu ici
Phase 2 : déploiement skills
Phase 3 : mise à jour opencode.json          ← opencode.json = état stale
Phase 4 : injection MCP (a un backup dans mcp-deploy.sh:131)
```

**Fichiers :** `scripts/cmd-deploy.sh:551-641`, `scripts/lib/mcp-deploy.sh:131,171`

**Résolution par la migration :** Transaction : snapshot de l'état avant deployment → déploiement → commit ou rollback complet sur erreur/interruption (`SIGINT` trap).

---

## Analyse comparative — Python vs TypeScript/Bun

| Critère | Python | TypeScript + Bun | Avantage |
|---|---|---|---|
| **Startup time** | ~80ms (CPython), ~20ms (PyInstaller) | ~10ms (Bun), ~30ms (Node) | TS/Bun |
| **JSON natif** | `json` stdlib — rapide, fiable | Natif V8/JSC — très rapide | Égalité |
| **Framework CLI** | `click` (mature, excellent), `typer` (modern, type hints) | `commander` (solide), `oclif` (enterprise) | Python légèrement |
| **TUI riche** | `rich` + `textual` — reference du domaine | `ink` (React-like), `clack`, `blessed` | Python |
| **Distribution** | `pipx install`, PyInstaller binary, Nuitka | `npm install -g`, Bun binary standalone | TS/Bun (binary sans deps) |
| **Tests** | `pytest` — mature, excellent DX | `vitest`/`jest` — très bon | Égalité |
| **Type safety** | `mypy`/`pyright` (opt-in, après-coup) | TypeScript natif (build-time) | TS/Bun |
| **Keychain OS** | `keyring` — macOS/Linux/Windows natif | `keytar` — natif mais dépendances natives | Python légèrement |
| **Python/Node disponible sur macOS stock** | Python 3 retiré depuis macOS 12.3 | Node absent par défaut | Égalité (aucun pre-installé) |
| **Portabilité zero-dep** | PyInstaller/Nuitka (complexe, binaire lourd) | `bun build --compile` → single binary ~8Mo | TS/Bun |
| **Cohérence avec la stack existante** | Aucun fichier Python dans le projet | Serveurs MCP déjà en TypeScript, Bun déjà installé par `install.sh` | TS/Bun |
| **Lisibilité / maintenance** | Syntaxe claire, idiomatic pour scripts | Verbeux mais typé, familier pour les devs JS | Python légèrement |
| **Écosystème CLIs publics similaires** | `awscli`, `gh` (Python) — bonne référence | `vercel`, `netlify`, `wrangler` (Node/Bun) — bonne référence | Égalité |

### Points forts Python (`click`/`typer` + `rich`)

- `click` est le framework CLI le plus mature et le plus ergonomique disponible. L'API déclarative par décorateurs est très lisible.
- `rich` + `textual` offrent le TUI le plus avancé de l'écosystème (tables, progress bars, live panels, mouse support).
- `keyring` intègre nativement le trousseau macOS Keychain, Windows Credential Manager, Linux Secret Service.
- Lisibilité des scripts d'automatisation supérieure à TypeScript.
- `typer` génère automatiquement l'autocompletion et la doc à partir des type hints Python.

### Points forts TypeScript + Bun

- **Cohérence de stack** : les serveurs MCP (`servers/figma-mcp`, `servers/gitlab-mcp`, `servers/gslides-mcp`) sont déjà en TypeScript. Un développeur contribuant au projet ne change pas de langage.
- **Bun est déjà dans la stack** : `install.sh` installe Bun (`curl -fsSL https://bun.sh/install | bash`). Zéro dépendance supplémentaire à installer.
- **`bun build --compile`** produit un binaire standalone (~8 Mo) qui embarque le runtime. Distribution sans pré-requis.
- **Startup ~10ms** vs ~80ms Python — perceptible sur des commandes courtes (`oc version`, `oc list`, `oc status`).
- **Type safety à la compilation** : TypeScript détecte les erreurs d'interface avant l'exécution, contrairement à Bash (runtime) ou Python sans `mypy`.
- **JSON natif** : manipulation directe des objets sans sérialisation/désérialisation explicite. Élimine 100% des appels `jq`.

---

## Recommandation

**Langage recommandé : TypeScript compilé avec Bun**

### Raisonnement

La cohérence de stack est l'argument décisif. Le projet maintient déjà trois serveurs MCP en TypeScript, Bun est déjà installé sur les machines des utilisateurs, et la compilation en binaire standalone règle le problème de distribution sans ajouter de prérequis.

Les performances sont objectivement supérieures (~10ms startup vs ~80ms Python, zéro fork pour JSON), ce qui est perceptible sur un outil CLI utilisé des dizaines de fois par jour.

Le seul argument fort pour Python est la qualité du TUI (`rich`/`textual`) et l'ergonomie de `click`. Ces arguments perdent de leur poids si l'on considère que :
1. Le TUI actuel est déjà riche (spinner, progress bars, picker) et entièrement custom — il peut être réimplémenté avec `@clack/prompts` + `chalk`.
2. `commander` + `typia` en TypeScript offrent une expérience proche de `click`/`typer`.

**Stack recommandée :**
- **Runtime :** Bun
- **Langage :** TypeScript strict
- **CLI framework :** `commander` v12+ ou `oclif` (pour un CLI enterprise avec plugins)
- **Prompts :** `@clack/prompts` (cohérence visuelle avec l'UI existante du hub)
- **Couleurs/TUI :** `chalk` + `cli-table3` + `ora`
- **Keychain :** `keytar`
- **Tests :** `vitest` (unitaires) + tests `.bats` existants conservés en black-box

### Cas où Python serait préférable

- Si l'équipe ne maîtrise pas TypeScript et préfère Python
- Si le TUI doit évoluer vers des interfaces très complexes (multi-panneaux, mouse, forms) — `textual` est imbattable dans ce domaine
- Si l'intégration keychain OS est critique et doit fonctionner sur de nombreuses configurations Linux

---

## Stratégie de migration suggérée

### Principe : migration incrémentale

Le dispatcher `oc.sh` peut appeler des binaires/scripts TypeScript commande par commande. Les tests `.bats` existants servent de tests d'intégration black-box tout au long de la migration — ils testent le comportement observable (output, exit codes) indépendamment de l'implémentation.

### Phase 0 — Infrastructure (1 semaine)

- Créer `cli/` à la racine du projet
- Initialiser `bun init` + TypeScript strict + `commander`
- Mettre en place la structure de modules (config, project, i18n, logger)
- `bun build --compile` → `bin/oc-next` (binaire test)
- Configurer `vitest` + intégration CI

### Phase 1 — Commandes triviales (1 semaine)

Migrer les commandes sans interaction externe complexe :
- `oc version` — lecture hub.json
- `oc list` — lecture projects.md
- `oc status` — lecture multi-fichiers
- `oc help` — affichage statique

Objectif : valider la stack, les performances startup, le pipeline de build.

### Phase 2 — Configuration et registre (1 semaine)

- `oc config` (toutes les sous-commandes)
- `oc init` (wizard complet avec prompts)
- `oc remove`
- `oc project` (rename, move, configure)

Ces commandes couvrent l'essentiel du file locking et des écritures atomiques.

### Phase 3 — Commandes critiques (2 semaines)

- `oc deploy` (avec rollback transactionnel)
- `oc sync`
- `oc start`
- `oc agent`, `oc skills`, `oc plugin`

### Phase 4 — TUI avancé (1 semaine)

- `oc board` (watch mode, kanban)
- `oc dashboard`
- `oc metrics`, `oc optimize`, `oc yield`

### Phase 5 — Finalisation (1 semaine)

- Autocompletion zsh/bash/fish
- `oc doctor`
- Migration `ocp.sh` → sous-commande `oc start --provider`
- Suppression de `oc.sh` / Bash
- Publication du binaire

### Conservation des tests `.bats`

Les tests `.bats` existants testent le comportement observable (output stdout/stderr, codes de sortie, modifications de fichiers). Ils peuvent être conservés tels quels comme tests d'intégration black-box pendant toute la migration, puis complétés par des tests unitaires `vitest` au niveau module.

---

*Document généré le 30 juin 2026 — Analyse CLI opencode-hub v1.5.0*
