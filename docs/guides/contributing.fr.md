> [Read in English](contributing.en.md)

# Guide de contribution

Ce guide explique comment contribuer au hub : développement du CLI Go,
ajout d'agents et de skills, et processus de PR.

---

## Développement du CLI (contributions code)

### Prérequis

| Outil | Version | Installation |
|-------|---------|-------------|
| Go | 1.26.4+ | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | 2.1+ | `brew install golangci-lint` ou [docs](https://golangci-lint.run/welcome/install/) |
| git | 2.5+ | Fourni par le système |

### Installation locale

```bash
git clone https://github.com/datichb/openhub.git
cd openhub/cli

# Telecharger les dependances
make deps

# Synchroniser le contenu du hub dans le répertoire embarqué et compiler le faux bd des conteneurs
# (étape OBLIGATOIRE avant le premier build — GoReleaser le fait automatiquement en release)
make embed-sync

# Compiler le binaire
make build          # -> bin/oh
```

> **Attention :** le binaire embarque une copie de `agents/`, `skills/`, `permissions/` et
> `workflows/` (dans `internal/hubcontent/hub/`). Après une modification de ce contenu, lancez
> `make sync-content` : il refait la copie sans recompiler le faux bd (plus rapide que
> `make embed-sync`). Tant que la copie diffère des sources, `TestEmbeddedContentInSync`
> échoue et liste les fichiers en cause (seulement dans le dépôt de développement : les builds
> de release copient les sources d'abord).

### Lancer les tests

```bash
make test           # Tous les tests avec -race detector (délai : TEST_TIMEOUT=40m)
make test-unit      # Tests unitaires uniquement (-short, plus rapide)
make test-tui       # Tests TUI uniquement (./internal/tui/...)
make test-cover     # Genere coverage.out + coverage.html
```

Mesurer les durées de la suite : `make test-durations` lance `go test ./... -short -count=1 -json`
(`TEST_DURATIONS_PKGS`, `TEST_DURATIONS_ARGS` et `TEST_DURATIONS_OUT` changent les paquets, les
arguments — par exemple `-race` — et le fichier JSON, `bin/test-durations.json` par défaut), puis
affiche la durée réelle, le nombre de tests réussis, en échec et ignorés, la durée de chaque paquet et
les tests de premier niveau les plus lents (`TEST_DURATIONS_TOP`). Le même résumé se lit sur n'importe
quelle sortie `go test -json`, et deux mesures se comparent paquet par paquet :

```bash
go test ./internal/semver/... -json | go run scripts/testdurations/main.go
go run scripts/testdurations/main.go -compare avant.json apres.json   # depuis cli/
```

Le CI exige un seuil de couverture minimum de **60%** avec le race detector actif.

La CI GitHub (`.github/workflows/ci.yml`) lance, sous Linux, macOS et Windows, `go vet`, le lint,
`govulncheck` et une seule exécution `go test -race` de la suite (délai de 40 minutes) qui écrit aussi
le profil de couverture, dont le total est comparé au seuil sous Linux et macOS ; un job à
part lance les tests de contrat (tag `integration`) contre un vrai `opencode serve` de version
épinglée, sans appeler de modèle. Les tests e2e (tag `e2e`, vrai modèle Bedrock) et les évaluations
(`make test-eval`) ne tournent jamais en CI : ils sont locaux et manuels, et aucune clé de modèle
n'est confiée aux secrets GitHub.

Sous Windows, la CI ne teste que les paquets listés dans `.github/windows-test-packages.txt` : les
tests des 24 autres, écrits pour Unix, y échouent. Ils sont listés avec leur cause dans
`.github/windows-test-packages-excluded.txt` et suivis par le lot `windows-tests`, qui les rendra
verts. Un nouveau paquet absent des deux fichiers n'est pas testé sous Windows : la CI le signale par
un avertissement (`.github/scripts/windows-packages.sh check`) ; ajoutez-le à l'un des deux fichiers.
Le seuil de couverture n'est pas calculé sous Windows, pour rester celui du module entier.

La suite complète dépasse le délai par défaut de `go test` (10 minutes), surtout quand d'autres
suites tournent en parallèle : en dehors de `make test`, passez le délai vous-même
(`go test -timeout 40m ./...`).

Les diagrammes de `docs/` sont des blocs ` ```mermaid ` (sources dans `docs/diagrams/*.mermaid`,
plus aucun SVG). `internal/docstest` vérifie qu'aucun SVG de diagramme n'est référencé, qu'un bloc
suivi de « Source du diagramme » est identique à sa source et, si `mmdc` est installé
(`npm i -g @mermaid-js/mermaid-cli`), que chaque diagramme se rend sans erreur.

#### Isolation automatique des tests

Les tests ne touchent jamais à votre vrai hub ni à votre vrai `HOME`, même si votre shell exporte
`OH_HOME` ou des `OH_TEST_*` (par exemple après `source openhub-test/env.sh`). Le `TestMain` des
paquets concernés appelle `internal/testenv`, qui, avant `m.Run()` :

- crée un dossier temporaire, y place `HOME` et `XDG_CONFIG_HOME`, `XDG_DATA_HOME`,
  `XDG_CACHE_HOME`, `XDG_STATE_HOME` ;
- retire les variables `OH_*` (`OH_HOME`, `OH_TEST_*`, `OH_PASSPHRASE`, session et passerelle…),
  sauf `OH_E2E_*` et `OH_EVAL_*` (tests qui appellent un vrai modèle). Le hub est alors `~/.oh` du
  `HOME` temporaire ; un test qui veut un autre hub fixe `OH_HOME` avec `t.Setenv` ;
- garde le `PATH` (`opencode`, `bd`) et les caches Go ;
- remplace votre configuration git globale par celle des tests (`gittest.Defaults` : une identité
  factice, `main` comme branche initiale, pas de signature, pas de maintenance automatique), comme
  sur un runner de CI qui n'a ni l'un ni l'autre : un test ne doit dépendre ni de votre identité ni de
  vos réglages git.

Un garde-fou compare, avant et après les tests, la taille et la date de modification de `hub.toml` et
de `oh.db` dans l'ancien `OH_HOME` et dans `~/.oh` : si l'un a changé, le paquet échoue (message
« changed the real hub »). Lecture seule : il ne crée rien. Si vous utilisiez oh sur ce hub pendant
les tests, relancez.

Il n'y a donc plus besoin de `env -u OH_HOME go test`. Un nouveau paquet dont les tests lisent
`OH_HOME`, `HOME` ou un chemin dérivé (`~/.oh`, `~/.config`, trousseau…), directement ou par le code
testé, ajoute un `main_test.go` :

```go
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}
```

(avec `gittest.Main(m)` à la place de `m.Run()` si les tests lancent git). Un test qui relance le
binaire de test comme processus auxiliaire hérite de l'environnement isolé.

### Linting et formatage

```bash
make lint           # golangci-lint run ./...
make vet            # go vet ./...
make fmt            # gofmt -s -w .
```

**Cache de golangci-lint par worktree.** golangci-lint garde un cache commun à tous les
dossiers (`~/Library/Caches/golangci-lint` sur macOS). Avec plusieurs worktrees du dépôt lancés
en parallèle, ce cache partagé peut renvoyer des résultats d'un autre arbre ou se bloquer
sur son verrou. Donnez à chaque worktree son propre cache :

```bash
# Un cache jetable, pour un passage
GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run ./...

# Ou un cache propre au worktree, gardé d'un passage à l'autre
export GOLANGCI_LINT_CACHE="${TMPDIR:-/tmp}/golangci-lint-$(basename "$(git rev-parse --show-toplevel)")"   # hors de l'arbre, un par worktree
```

Les linters actifs incluent : `bodyclose`, `gocritic` (diagnostic + style), `misspell`, `nilerr`.

### Vue d'ensemble du code

Le CLI est un binaire Go unique (`cli/main.go` -> `cli/cmd/root.go`). L'architecture interne :

| Package | Responsabilite |
|---------|---------------|
| `cmd/` | Commandes Cobra (point d'entrée CLI) |
| `internal/config/` | Configuration hub (`hub.toml`, TOML + Viper) |
| `internal/mcp/` | Serveurs MCP built-in (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team, Workflow) |
| `internal/teamstate/` | Gestion de l'état équipe (claims, wiki, policies, patterns, workflows d'équipe) |
| `internal/workflow/` | Workflows declaratifs `oh/v1` (schema, couches, validation) |
| `internal/bundle/` | Paquet de session (agents, skills, permissions, compile hors du projet sous `~/.oh/bundles/`) |
| `internal/sessionspec/` | Modèle de session independant de l'outil (paquet, emplacement, runtime, fournisseur) |
| `internal/adapters/` | Contrat oh ↔ outil agentique ; adaptateur opencode V2 (`opencodev2/`) |
| `internal/runsvc/` | Lancement des sessions v5 (groupes de serveurs, proxy, monde ferme, checkpoints) |
| `internal/daemon/` | Démon `ohd` (supervision des sessions, décisions, notifications) |
| `internal/credproxy/` | Proxy d'identifiants LLM (jetons par groupe) |
| `internal/runtime/`, `internal/remote/` | Environnements d'exécution (conteneur, distant GitLab CI) |
| `internal/limits/` | Restrictions des sessions (sessions actives, budgets, mémoire, modèles) |
| `internal/deploycleanup/` | Nettoyage des anciens déploiements (`oh migrate deploy-cleanup`) |
| `internal/tui/` | Interface TUI (tview ; huh pour quelques invites en ligne hors TUI ; shell, vues, widgets) |
| `internal/storage/` | SQLite (projets, sessions), keychain, chiffrement fichier |
| `internal/tracker/` | Sync trackers externes (GitLab, Jira) |
| `internal/i18n/` | Internationalisation (FR + EN, JSON) |

Le binaire embarque les agents, skills et permissions via `go:embed` (`internal/hubcontent/`).
`CGO_ENABLED=0` — pas de dépendance C, binaire statique.

### Conventions de code

- **Commandes minces** : une commande Cobra (`cmd/<domaine>.go`, `cmd/<domaine>_<action>.go`) lit ses flags, appelle un service ou un package de `internal/` et met en forme la sortie (`cmd.OutOrStdout()`) ; elle retourne ses erreurs, jamais `os.Exit`. Le niveau de démarrage se déclare à l'enregistrement (`withInitDB`, `withInitLevel(cmd, levelNone)`) ; les stores s'obtiennent par `MustApp()` (ou `TryApp()`).
- **Logique partagée CLI / TUI** dans `internal/services/<domaine>/` : la TUI n'appelle pas les commandes.
- **Contrats dans `internal/domain/`** (types, interfaces `XxxStore`, erreurs sentinelles `ErrXxx`), implémentations dans `internal/storage/sqlite/` (migrations dans `migrate.go`), vérifiées à la compilation (`var _ domain.XxxStore = (*XxxStore)(nil)`).
- **Indépendance de l'outil agentique** : seul `internal/adapters/<outil>/` et la racine de composition (`cmd/v5_adapters.go`) nomment l'outil ; `internal/archtest/` le vérifie.
- **Textes affichés** par `i18n.T` / `i18n.Tf`, clés dans `internal/i18n/locales/fr.json` et `en.json` (un test vérifie la parité). Aucun message n'est écrit en dur dans `cmd/` ni dans les vues de la TUI (`TestNoLiteralUserMessages` ; les rares exceptions sont listées avec leur raison dans `cmd/messages_literal_test.go`), et une clé de `en.json` que le code n'utilise nulle part est refusée (`TestNoOrphanKeys` ; une clé construite à l'exécution, `i18n.T("prefixe." + x)`, est reconnue par son préfixe). Une confirmation tapée au clavier passe par `confirmed()` (`cmd/confirm.go` : `o`, `oui`, `y`, `yes`).
- **Erreurs** enveloppées avec leur contexte (`fmt.Errorf("creating project %s: %w", id, err)`), testées par `errors.Is` / `errors.As`.
- **Tests** dans le même package, avec `testify` (`require` pour les préconditions), des cas en table, `t.TempDir()` et des helpers `t.Helper()`. Un test qui lance des processus (git, bd, serveurs) attend leur fin avant le nettoyage de son dossier ; les tests qui poussent vers des dépôts nus coupent la maintenance automatique de git (`internal/gitutil/gittest`).

---

## Ajouter un agent

### 1. Créer le fichier agent

```bash
touch agents/<famille>/<id>.md
```

Respecter la convention de nommage :
- `<domaine>-<spécialité>.md` pour les sous-agents (ex: `auditor-subagent.md`)
- `<rôle>.md` pour les agents principaux (ex: `orchestrator.md`)

### 2. Structure minimale du frontmatter

```markdown
---
id: <identifiant-unique>
label: <NomAffiché>
description: <Description courte en une phrase — visible dans les listes d'agents>
permission:
  skill: allow        # allow pour les agents utilisant des skills natives ; deny pour les coordinateurs
  ctx_search: allow   # allow pour les agents qui analysent du code ou des données
skills: [chemin/vers/skill, ...]          # Bucket A — skills inline toujours actives
native_skills: [chemin/vers/skill, ...]   # Bucket B — skills natives à la demande (optionnel)
---
```

**Règles :**
- `id` : slug unique, minuscules, tirets autorisés, pas d'espaces
- `label` : PascalCase, affiché dans l'outil IA
- `description` : une phrase, commence par un verbe ou un nom de rôle
- `skills` : chemins Bucket A relatifs à `skills/` — protocoles de workflow, formats de handoff, principes universels (toujours actifs)
- `native_skills` : chemins Bucket B relatifs à `skills/` — standards de domaine, checklists, skills contextuelles (chargées à la demande via l'outil `skill`)
- `permission.skill` : `allow` si l'agent utilise des skills natives ; `deny` pour les coordinateurs/orchestrateurs
- `permission.ctx_*` : les outils ctx doivent être explicitement autorisés par agent — ils ne sont **pas** hérités. Voir le tableau ci-dessous pour le jeu recommandé par type d'agent.

**Permissions ctx par type d'agent :**

| Type d'agent | Permissions ctx recommandées |
|---|---|
| Orchestrateurs (planning) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Développeur (developer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute`, `ctx_fetch_and_index`, `ctx_index` |
| Qualité (debugger, reviewer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` |
| Planning (planner, pathfinder, onboarder) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Design (designer) | `ctx_search`, `ctx_batch_execute` |
| Documentation (documentarian) | `ctx_search`, `ctx_batch_execute`, `ctx_index` |
| Audit (auditor, auditor-subagent) | `ctx_search`, `ctx_batch_execute` |

Voir [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md) pour le raisonnement Bucket A / B.

### 3. Corps de l'agent

Structure recommandée (voir `agents/auditor/auditor.md` comme référence pour les coordinateurs,
`agents/developer/developer.md` pour les agents implémenteurs) :

```markdown
# <NomAffiché>

<Phrase d'identité : qui tu es et ce que tu fais en 2-3 lignes>

## Ce que tu fais

- <Action 1>
- <Action 2>

## Ce que tu NE fais PAS

- <Contrainte 1>
- <Contrainte 2>

## Workflow

<Workflow condensé en 4-6 étapes>

## Exemples d'invocation (optionnel)

| Demande | Action |
|---------|--------|
| "..." | ... |
```

### 4. Créer ou référencer les skills

Si l'agent nécessite un protocole dédié, créer le skill correspondant
(voir section "Ajouter un skill" ci-dessous) avant de le référencer dans le frontmatter.

### 5. Construire et tester

```bash
# Construire le paquet de session d'un workflow qui utilise l'agent
oh bundle build <workflow>
# Vérifier que l'agent apparaît dans le paquet
oh bundle show <workflow>
```

---

## Ajouter un skill

### 1. Choisir le bon dossier

Les skills sont organisés par domaine dans `skills/` :

| Dossier | Usage |
|---------|-------|
| `skills/developer/` | Standards de développement (partagés entre developers et reviewer) |
| `skills/auditor/` | Protocoles d'audit |
| `skills/orchestrator/` | Protocoles de coordination |
| `skills/planning/` | Protocoles de planification |
| `skills/qa/` | Protocoles qualité |
| `skills/debugger/` | Protocoles de diagnostic |
| `skills/reviewer/` | Protocoles de review |
| `skills/documentarian/` | Protocoles de documentation |
| `skills/designer/` | Protocoles de design (designer) |
| `skills/design/` | Contrats de handoff design |
| `skills/quality/` | Contrats de handoff qualité (debugger et agents hors `qa/` et `reviewer/`) |
| `skills/posture/` | Posture et comportement transversal (`expert-posture`, `core-posture`, `user-facing-posture`, `coordinator-posture`) |

Pour un nouveau domaine, créer un nouveau sous-dossier.

### 2. Structure minimale du frontmatter

```markdown
---
name: <nom-du-skill>
description: <Description courte — visible dans la liste de skills de l'agent>
---
```

> La clé `name` est documentaire. La construction du paquet lit uniquement `description`.
> Le chemin du fichier est la référence utilisée dans le frontmatter des agents.

### 3. Contenu du skill

Un bon skill contient :

- **Rôle** : rappel de l'identité de l'agent qui utilise ce skill
- **Règles absolues** : ❌/✅ — les contraintes non négociables
- **Protocole / workflow** : les étapes détaillées
- **Formats de sortie** : les structures exactes des rapports, avec exemples
- **Checklists** : les vérifications systématiques
- **Ce que tu ne fais PAS** : les anti-patterns explicites

Voir `skills/reviewer/review-protocol.md` comme exemple.
Pour des skills natives Bucket B gérant la sélection de mode et l'orchestration parallèle, voir
`skills/reviewer/reviewer-adversarial.md`, `skills/reviewer/reviewer-edge-case.md`, et
`skills/reviewer/review-merge.md` (fusion de rapports issus de sessions parallèles).

### 4. Référencer le skill dans un agent

Déterminer si le skill est Bucket A ou Bucket B (voir [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md)) :

**Bucket A** — ajouter dans `skills:` du frontmatter de l'agent :
```markdown
---
skills: [chemin/vers/mon-skill]
---
```

**Bucket B** — ajouter dans `native_skills:` du frontmatter de l'agent, et ajouter `permission: skill: allow` :
```markdown
---
permission:
  skill: allow
native_skills: [chemin/vers/mon-skill]
---
```
Ajouter également une ligne dans la section guide "## Skills disponibles" du corps de l'agent avec le déclencheur de chargement.

**Retours entre agents :** un agent lancé par un autre inclut `shared/handoff-envelope` (enveloppe commune) et son payload `<famille>/<agent>-payload` (Bucket A, `when: role=subagent`, 200 à 400 tokens, statuts dans le frontmatter `statuses:`). L'appelant n'inclut aucun format : les statuts des agents qu'il lance sont générés dans son contrat de session. Ne jamais écrire `## Retour vers <agent>` avec un nom d'agent dans une skill : le titre vient du contrat. Exemples : `skills/developer/developer-payload.md`, `skills/reviewer/reviewer-payload.md`.

---

## Paquet de session

Depuis la v5, rien n'est déployé dans le projet (`oh deploy` / `oh sync` supprimés en v5). Chaque session démarre d'un paquet de session construit au lancement hors du projet (`~/.oh/bundles/<hash>/`) à partir de son workflow : agents traduits du format hub vers le format opencode, skills, permissions, serveurs MCP.

La logique du paquet est implémentée en Go dans `cli/internal/bundle/`.

### Étapes rapides

1. Ajouter ou modifier les agents/skills dans les répertoires `agents/` et `skills/`
2. Lancer `oh bundle build <workflow>` pour construire le paquet d'un workflow qui les utilise
3. Lancer `oh bundle show <workflow> --budget` pour inspecter son contenu et son budget de contexte

---

## Conventions de contribution

### Commits

Format **Conventional Commits** obligatoire :

```
feat: ajouter l'agent <nom>
fix: corriger <problème> dans <fichier>
docs: mettre à jour <section>
chore: <maintenance>
refactor: <restructuration>
```

### Nommage des fichiers

| Type | Convention | Exemple |
|------|-----------|---------|
| Agent | `<domaine>[-<spécialité>].md` | `auditor-subagent.md` |
| Skill (dans un sous-dossier) | `<domaine>-<sujet>.md` | `audit-security.md` |

### Documentation interne

Le dossier `docs/` est organisé comme suit (pour référence lors d'ajouts de documentation) :

- `docs/architecture/` — décisions architecturales (ADR), schémas, référence agents et skills
- `docs/guides/` — guides pratiques (contribution, workflows, authoring)
- `docs/reference/` — référence CLI et configuration
- `docs/dev/` — notes techniques internes (gotchas, patterns shell)

### ADR

Toute décision architecturale significative doit être documentée dans un ADR :

```bash
touch docs/architecture/adr/<NNN>-<titre-kebab-case>.md
```

Format : voir [ADR-001](../architecture/adr/001-agent-skill-separation.fr.md) comme modèle.

### PR

Avant de soumettre une PR :

```bash
# Vérifier que le paquet de session se construit correctement
oh bundle build <workflow>

# Vérifier le contenu du paquet
oh bundle show <workflow> --budget
```

---

## Checklist avant PR

- [ ] Le fichier agent respecte la structure minimale (frontmatter + corps)
- [ ] Le skill a un frontmatter avec `name` et `description`
- [ ] Les skills Bucket A sont dans `skills:`, les skills Bucket B sont dans `native_skills:` — raisonnement documenté dans [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md)
- [ ] Si l'agent utilise `native_skills:`, `permission: skill: allow` est défini ; si c'est un coordinateur, `skill: deny` est défini
- [ ] Les permissions ctx (`ctx_search`, `ctx_batch_execute`, etc.) sont déclarées selon le type d'agent — elles ne sont PAS héritées et doivent être explicites (voir les règles frontmatter ci-dessus)
- [ ] Si l'agent a des skills natives, le corps de l'agent a une section guide "## Skills disponibles" les listant avec les déclencheurs de chargement
- [ ] L'agent est référencé dans `README.md` et `docs/architecture/agents.fr.md`
- [ ] Le skill est référencé dans `docs/architecture/skills.fr.md` avec son marqueur de bucket (A) ou (B)
- [ ] Si le skill définit un format de retour structuré : injecté dans l'agent producteur ET l'agent consommateur (toujours Bucket A)
- [ ] Si décision architecturale : un ADR est créé dans `docs/architecture/adr/`
- [ ] Le commit respecte les Conventional Commits
- [ ] `make test` passe sans erreur (ou `make test-unit` au minimum)
- [ ] `make lint` ne produit aucun warning
- [ ] `oh bundle build <workflow>` passe sans erreur pour les workflows concernés
- [ ] `oh bundle show <workflow>` ne montre aucune divergence inattendue

---

## Créer une release

> Réservé aux mainteneurs disposant d'un accès en écriture sur `main`.

### Prérequis

- Être sur la branche `main` avec un working tree propre
- [GoReleaser](https://goreleaser.com/) installé (`brew install goreleaser`)
- Accès push au dépôt distant

### Préparation du CHANGELOG

Avant de lancer la release, rédiger le contenu sous `## [Unreleased]` dans `CHANGELOG.md`.

Les entrées à inclure pour la prochaine release couvrent notamment :
- Board team kanban (5 colonnes, actions c/x/t/s wirées, labels visuels `[AI]`)
- Cycle de vie des claims (5 statuses : `planned`, `in_progress`, `review`, `blocked`, `done`)
- `oh team claim --planned` et transition automatique `planned → in_progress` via `oh start --dev` (aujourd'hui alias déprécié de `oh run ticket`)
- Sync tracker externe (`oh team sync-tracker`, package `cli/internal/tracker/`, GitLab + Jira)
- Label `agent-reviewed` auto-appliqué sur `review.ready`
- Pull async sur toutes les vues team

```markdown
## [Unreleased]

### Ajouté
- ...

### Modifié
- ...

### Corrigé
- ...
```

### Lancer la release

```bash
# Depuis le répertoire cli/
cd cli

# Prévisualiser sans publier (snapshot)
goreleaser release --snapshot --clean

# Créer la release (déclenchée par un push de tag)
git tag -a v1.2.0 -m "Release v1.2.0"
git push && git push --tags
```

GoReleaser est configuré via `.goreleaser.yml` dans le répertoire `cli/` et gère :
1. La cross-compilation du binaire `oh`
2. La génération du changelog depuis les commits
3. La création de la release GitHub avec les artefacts

### Convention des tags

Les tags suivent le format `vX.Y.Z` (annoté) :

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
```
