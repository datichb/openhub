# Référence CLI — commandes `oc`

Toutes les commandes disponibles via le point d'entrée `oc.sh` (alias recommandé : `oc`).

---

## Synopsis global

```
oc <commande> [sous-commande] [options] [arguments]
```

---

## `oc install`

Installe les outils et crée la structure du hub.

```bash
oc install
```

**Comportement :**
**Comportement :**
- Vérifie et **demande confirmation** avant d'installer chaque dépendance (Node.js, opencode, Beads, bun)
- Si `config/hub.json` existe déjà, demande confirmation avant d'écraser

---

## `oc uninstall`

Désinstalle openhub et nettoie les artefacts créés lors de l'installation.

```bash
oc uninstall
# équivalent à :
bash ~/.openhub/uninstall.sh
```

**Comportement :**

Guide la désinstallation en 4 étapes, toutes optionnelles et avec confirmation explicite :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets (`.opencode/agents/`, `opencode.json`, `.opencode/agents/`) | `[y/N]` |
| 2 | Supprimer le hub (`~/.openhub`) | `[y/N]` |
| 3 | Retirer l'alias `oc` et les exports bun du fichier rc shell | `[Y/n]` |
| 4 | Désinstaller les outils système : `opencode`, `beads`, `bun` (séparément) | `[y/N]` |

> `jq` et `node` ne sont pas proposés à la désinstallation (usage général, risque de casser d'autres outils).
>
> Un backup `.bak` est créé automatiquement avant toute modification du fichier rc.

---

## `oc deploy`

Génère les fichiers agents pour un projet. Quand un `PROJECT_ID` est fourni, **détecte automatiquement la stack du projet** et injecte les skills spécifiques correspondants dans les agents developer (en plus de leurs skills déclarés statiquement).

```bash
oc deploy [PROJECT_ID]
oc deploy --check [PROJECT_ID]
oc deploy --diff  [PROJECT_ID]
```

**Arguments :**

| Argument | Valeurs | Description |
|----------|---------|-------------|
| `[PROJECT_ID]` | ID d'un projet enregistré | Optionnel — déploie au niveau du hub si absent (pas de détection de stack) |

**Options :**

| Option | Description |
|--------|-------------|
| `--check` | Vérifie si les agents **et les skills** sont à jour sans déployer |
| `--diff` | Compare les sources avec les fichiers déployés ; propose le déploiement si un écart est détecté |

**Détection de stack :**

Quand `PROJECT_ID` est fourni, `oc deploy` lit les fichiers de dépendances du projet (`package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, fichiers d'infrastructure, etc.) pour détecter la stack active. Les skills correspondants dans `skills/developer/stacks/` sont ensuite injectés dans les agents developer selon le mapping de `config/stack-skills.json`.

Ainsi, un agent `developer-frontend` déployé sur un projet React/Vitest/Playwright recevra automatiquement `dev-standards-react`, `dev-standards-vitest` et `dev-standards-playwright` — sans aucun changement de configuration d'agent.

**Exemples :**

```bash
oc deploy                       # déploie au niveau du hub (pas de détection de stack)
oc deploy MON-APP               # déploie les agents dans MON-APP (avec détection de stack)
oc deploy --check               # vérifie les agents du hub
oc deploy --check MON-APP       # vérifie les agents de MON-APP
oc deploy --diff MON-APP        # affiche le diff sources → déployés pour MON-APP
```

**Sorties générées :**

| Cible | Fichiers générés |
|-------|-----------------|
| `opencode` | `.opencode/agents/*.md` (Phase 1) + `.opencode/skills/<name>/SKILL.md` (Phase 2) + `opencode.json` (Phase 3, si clé API ou PROJECT_ID défini) |

**Codes de sortie `--check` :**
- `0` : agents et skills tous à jour
- `1` : au moins un agent ou une skill est obsolète ou manquant(e)

> Un spinner animé (`⠋⠙⠹…`) est affiché pendant le déploiement.

---

## `oc sync`

Redéploie les agents sur tous les projets enregistrés ayant un chemin local défini.

```bash
oc sync [--dry-run]
```

**Options :**

| Option | Description |
|--------|-------------|
| `--dry-run` | Vérifie la fraîcheur sans déployer (équivalent à `oc deploy --check` sur chaque projet) |

**Exemples :**

```bash
oc sync             # redéploie sur tous les projets
oc sync --dry-run   # vérifie sans déployer
```

---

## `oc start`

Lance l'outil par défaut dans le répertoire d'un projet.

```bash
oc start [PROJECT_ID] [prompt]
         [--dev [--label <label>] [--assignee <user>]]
         [--onboard [--refresh]]
         [--parallel]
         [--worktree [<branche>]]
         [--agent <nom>]
         [--provider <p>]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |
| `[prompt]` | Prompt de démarrage passé à l'outil |

**Options :**

| Option | Description |
|--------|-------------|
| `--dev` | Mode développement — charge les tickets `ai-delegated` ouverts dans le prompt de démarrage. Effectue un `sync pull` tracker automatique avant le lancement. Lance dans le répertoire principal du projet. |
| `--dev --label <label>` | Comme `--dev`, mais filtre les tickets ayant le label `<label>` |
| `--dev --assignee <user>` | Comme `--dev`, mais filtre les tickets assignés à `<user>` |
| `--onboard` | Injecte un prompt de découverte projet pour onboarder l'agent sur le codebase |
| `--onboard --refresh` | Réinitialise le contexte d'onboarding avant de relancer la découverte |
| `--parallel` | Lance un `orchestrator-dev` dans un **worktree isolé** (`parallel/TIMESTAMP`) pour traiter plusieurs tickets `ai-delegated` simultanément en mode `auto`. À utiliser quand plusieurs tickets indépendants sont prêts. Requiert `Worktree: enabled` dans `projects.md`. |
| `--worktree [<branche>]` | Ouvre une **session libre dans un worktree isolé** sur une branche nommée. Si la branche est omise, elle est demandée interactivement. Idéal pour démarrer un développement indépendant en parallèle d'une session en cours. Crée le worktree s'il n'existe pas, le réutilise sinon. |
| `--agent <nom>` | Force l'agent de démarrage (ex : `orchestrator`, `developer-fullstack`) |
| `--provider <p>` | Surcharge le provider LLM pour cette session (ex : `anthropic`, `openai`). Régénère `opencode.json` si les agents sont déjà déployés. |

> **Exclusivités mutuelles :** `--dev`, `--parallel` et `--worktree` sont mutuellement exclusifs entre eux, et tous incompatibles avec `--onboard`. `--label` et `--assignee` sont mutuellement exclusifs. `--refresh` requiert `--onboard`.

> **Choisir entre `--dev`, `--parallel` et `--worktree` :**
> - `--dev` : session séquentielle dans le repo principal, tickets traités un par un
> - `--parallel` : orchestrator multi-tickets dans un worktree dédié (mode auto, isolation filesystem)
> - `--worktree` : session libre sur une branche isolée, sans lien Beads obligatoire — pour un développement parallèle indépendant

**Exemples :**

```bash
oc start                                        # sélection interactive du projet
oc start MON-APP                                # lance l'outil dans MON-APP
oc start MON-APP "explique l'architecture"      # avec prompt de démarrage
oc start MON-APP --dev                          # charge les tickets ai-delegated
oc start MON-APP --dev --label ai-delegated     # filtre par label
oc start MON-APP --dev --assignee alice         # filtre par assignee
oc start MON-APP --onboard                      # prompt de découverte projet
oc start MON-APP --onboard --refresh            # redécouverte avec reset du contexte
oc start MON-APP --parallel                     # orchestrator multi-tickets en worktree isolé
oc start MON-APP --worktree feat/ma-feature     # session libre sur branche isolée
oc start MON-APP --worktree                     # session libre, nom de branche demandé interactivement
oc start MON-APP --agent developer-fullstack    # force l'agent de démarrage
oc start MON-APP --provider openai              # surcharge le provider LLM
```

**Rendu au lancement :**

```
◆  MON-APP
│  Chemin     /Users/alice/workspace/mon-app
│  Cible      opencode
│
│  → Nouveau sur ce projet ? Invoke l'agent onboarder
│    "Onboarde-toi sur ce projet"
│  → Ou lance directement : ./oc.sh start --onboard MON-APP
│
└  Lancement de opencode…
```

> Avertit dans le bloc contextuel si les agents ne sont pas déployés (`◆` jaune) ou si `.beads/` est absent.

---

## `oc audit`

Lance un audit IA sur un projet en invoquant l'agent `auditor` (et son sous-agent spécialisé si `--type` est précisé).

```bash
oc audit [PROJECT_ID] [--type <type>]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |

**Options :**

| Option | Valeurs | Description |
|--------|---------|-------------|
| `--type <type>` | `security`, `accessibility`, `architecture`, `ecodesign`, `observability`, `performance`, `privacy` | Cible l'audit sur un domaine précis. Si absent : audit global via `auditor` |

**Comportement :**

1. **Validation** — vérifie que le `--type` est parmi les 7 domaines reconnus (si fourni)
2. **Résolution projet** — normalise l'ID et résout le chemin local
3. **Vérification projects.md** — si le projet a une sélection d'agents restrictive (pas `all`), vérifie que `auditor` (et `auditor-<type>` si précisé) sont inclus :
   - Si manquants → propose de les ajouter + redéployer
   - Si refus → affiche les agents audit physiquement déployés et propose un menu de sélection
4. **Vérification déploiement physique** — si le dossier agents est absent ou si les fichiers manquent, propose `oc deploy`
5. **Lancement** — construit le prompt de bootstrap et ouvre l'outil avec `--agent auditor` (ou l'agent sélectionné)

**Exemples :**

```bash
oc audit                          # sélection interactive du projet, audit global
oc audit MON-APP                  # audit global sur MON-APP
oc audit MON-APP --type security  # audit sécurité uniquement
oc audit MON-APP --type privacy   # audit RGPD/privacy uniquement
```

**Prompt injecté :**

```
Effectue un audit complet du projet.

Projet : MON-APP
Chemin : /Users/alice/workspace/mon-app
Périmètre : audit security uniquement.   ← présent seulement si --type

Workflow :
1. Annoncer le périmètre et la méthodologie de l'audit
2. Explorer les fichiers pertinents selon le type d'audit
3. Identifier et classifier les points d'attention (🔴 critiques, 🟠 importants, 🟡 améliorations)
4. Produire le rapport d'audit structuré avec recommandations priorisées
```

> Pour un audit complet multi-domaines, invoquer l'agent `auditor` directement sans `--type`.

---

## `oc review`

Lance une code review IA sur une branche en invoquant l'agent `reviewer` avec le nom de la branche dans le prompt — le reviewer récupère lui-même le diff via `git diff`.

```bash
oc review [PROJECT_ID] [--branch <branche>]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |

**Options :**

| Option | Description |
|--------|-------------|
| `--branch <branche>` | Branche à reviewer. Si absent : utilise la branche git courante du projet |

**Comportement :**

1. **Résolution de la branche** — si `--branch` non fourni, détecte la branche courante via `git branch --show-current` dans le répertoire du projet
2. **Git fetch** — exécute `git fetch` pour mettre à jour les refs distantes ; si échoue (pas de réseau, auth), demande confirmation avant de continuer
3. **Pull de la branche de base** — exécute `git pull --ff-only origin <base>` où `<base>` est lu depuis `- Worktree base branch :` dans `projects.md` (défaut : `main`) ; si échoue (branche divergée), demande confirmation
4. **Vérification projects.md** — si le projet a une sélection d'agents restrictive (pas `all`), vérifie que `reviewer` est inclus :
   - Si manquant → propose de l'ajouter + redéployer
5. **Vérification déploiement physique** — si le dossier agents est absent ou si `reviewer.md` manque, propose `oc deploy`
6. **Instruction diff** — injecte la commande exacte `git diff <base>...<branche>` dans le prompt ; l'agent l'exécute lui-même et analyse le résultat progressivement, évitant le débordement de la fenêtre de contexte sur les grosses branches
7. **Lancement** — ouvre l'outil avec `--agent reviewer` et le prompt contenant l'instruction diff

**Exemples :**

```bash
oc review                              # sélection interactive du projet, branche courante
oc review MON-APP                      # review de la branche courante de MON-APP
oc review MON-APP --branch feat/login  # review de la branche feat/login
```

**Prompt injecté :**

```
Effectue une code review de la branche `feat/login`.

Projet : MON-APP
Chemin : /Users/alice/workspace/mon-app
Branche reviewée : feat/login
Branche de base  : main

→ Lire CONVENTIONS.md à la racine du projet avant la review   ← si le fichier existe

Pour obtenir le diff, exécute :
  git diff main...feat/login

Workflow :
1. Si CONVENTIONS.md existe à la racine → le lire pour appliquer les conventions réelles du projet
2. Exécuter `git diff main...feat/login` pour obtenir les changements
3. Analyser le diff selon la checklist systématique du skill review-protocol
4. Produire le rapport structuré par sévérité : Critique → Majeur → Mineur → Suggestion → Points positifs
```

> L'agent `reviewer` ne modifie aucun fichier — il produit uniquement un rapport d'analyse.
> L'agent récupère lui-même le diff via `git diff` — cela évite le débordement de la fenêtre de contexte sur les grosses branches.
> Pour un diff vide (branche à jour avec la branche de base), l'agent le détecte et le signale.
> La branche de base utilisée pour le diff est lue depuis `- Worktree base branch :` dans `projects.md` (défaut : `main`).

---

## `oc conventions`

Génère ou met à jour le fichier `CONVENTIONS.md` à la racine d'un projet en
invoquant l'agent `onboarder` en mode conventions.

```bash
oc conventions [PROJECT_ID] [--force]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |

**Options :**

| Option | Description |
|--------|-------------|
| `--force` | Écrase `CONVENTIONS.md` sans demander confirmation s'il existe déjà |

**Comportement :**

1. Résout le projet (interactif si `PROJECT_ID` absent)
2. Si `CONVENTIONS.md` existe déjà dans le projet → affiche la date de génération et demande confirmation avant d'écraser (sauf `--force`)
3. Injecte le prompt de bootstrap conventions et ouvre l'outil avec l'agent `onboarder`
4. L'agent explore la codebase, détecte les conventions réelles (9 catégories) et génère `CONVENTIONS.md`
5. Ajoute `CONVENTIONS.md` au `.git/info/exclude` du projet s'il n'y est pas déjà (exclusion locale, invisible pour les autres devs)

**Exemples :**

```bash
oc conventions                   # sélection interactive du projet
oc conventions MON-APP           # génère CONVENTIONS.md pour MON-APP
oc conventions MON-APP --force   # regénère sans confirmation
```

**Fichier généré :**

`CONVENTIONS.md` documente les conventions réelles observées dans la codebase :
formatage, nommage, architecture, tests, Git, gestion d'erreurs, sécurité,
performance, et conventions spécifiques. Ce fichier est lu par tous les agents
développeurs et qualité en début de session pour coder en respectant les
conventions du projet plutôt que les standards génériques.

> `CONVENTIONS.md` est exclu via `.git/info/exclude` — il reste local au poste de travail, invisible pour les autres devs.
> Pour le régénérer après une évolution du projet : `oc conventions MON-APP --force`.

---

## `oc debug`

Lance une session de diagnostic de bug sur un projet en invoquant l'agent `debugger`.

```bash
oc debug [PROJECT_ID]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |

**Comportement :**

1. **Résolution projet** — normalise l'ID et résout le chemin local
2. **Vérification projects.md** — si le projet a une sélection d'agents restrictive (pas `all`), vérifie que `debugger` est inclus :
   - Si manquant → propose de l'ajouter + redéployer
3. **Vérification déploiement physique** — si le dossier agents est absent ou si `debugger.md` manque, propose `oc deploy`
4. **Lancement** — construit le prompt de bootstrap et ouvre l'outil avec `--agent debugger`

**Exemples :**

```bash
oc debug               # sélection interactive du projet
oc debug MON-APP       # lance le debugger sur MON-APP
```

**Rendu au lancement :**

```
◆  oc debug  MON-APP
│  Chemin        /Users/alice/workspace/mon-app
│  Cible         opencode
│  Agent         debugger
│
└  Lancement de opencode…
```

> L'agent `debugger` analyse le bug décrit, explore la codebase et produit un diagnostic structuré avec hypothèses et corrections recommandées.

---

## `oc init`

Enregistre un projet dans le hub. Guide l'utilisateur en **6 étapes numérotées** et affiche un récapitulatif coloré à la fin.

```bash
oc init [PROJECT_ID] [chemin]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Identifiant unique du projet (lettres, chiffres, `-`, `_`) |
| `[chemin]` | Chemin absolu ou `~`-expansé vers le répertoire du projet |

**Wizard interactif :**

| Étape | Contenu |
|-------|---------|
| 1 — Informations projet | PROJECT_ID, chemin, vérification/création du dossier, nom, stack, labels, tracker |
| 2 — Beads & tracker | `bd init`, upstream Git, configuration tracker |
| 3 — Agents | Sélection des agents et des agents natifs OpenCode à désactiver |
| 4 — Services MCP | Sélection des intégrations MCP à activer pour ce projet (`none` par défaut) |
| 5 — Fournisseur LLM | Configuration d'un provider spécifique au projet (surcharge le hub) |
| 6 — Déploiement | Proposition de déploiement immédiat |

> La création du dossier a lieu en **fin d'étape 1** — Beads est ainsi garanti accessible dès l'étape 2.

> **Étape 4 — Services MCP :** Par défaut, aucun MCP n'est déployé (opt-in). Répondre `Y` ouvre un sélecteur multi-choix listant les services disponibles depuis `config/services.json`. La sélection est persistée dans `- MCP :` de `projects/projects.md` et appliquée à chaque `oc deploy`. Pour modifier la sélection plus tard, éditer `projects.md` directement ou relancer `oc init`.

**Rendu wizard :**

```
◆  Initialisation d'un projet
│
│
◇  Étape 1/6 — Informations projet
│
│  PROJECT_ID (ex: MON-APP) :
│  ...
│
◇  Étape 2/6 — Beads & tracker
│
│  ...
│
◇  Étape 4/6 — Services MCP
│
│  Activer des intégrations MCP pour ce projet ? [y/N] :
```

**Récapitulatif final :**

```
┌─ MON-APP initialisé ──────────────────────────────┐
│  Chemin       /Users/alice/workspace/mon-app       │
│  Nom          Mon Application                      │
│  Stack        Vue 3 + Laravel                      │
│  Tracker      jira                                 │
│  Beads        ◆ initialisé                         │
│  MCP          figma-mcp                            │
│                                                    │
│  Prochain → ./oc.sh start MON-APP                  │
└────────────────────────────────────────────────────┘

└  Projet MON-APP prêt — ./oc.sh start MON-APP
```

**Exemples :**

```bash
oc init                              # mode interactif complet
oc init MON-APP ~/workspace/mon-app  # pré-remplit ID et chemin (questions restantes interactives)
```

---

## `oc status`

Affiche un tableau de bord de l'état de tous les projets enregistrés.

```bash
oc status [--short]
```

**Options :**

| Option | Description |
|--------|-------------|
| `--short` / `-s` | Vue compacte : tableau id / chemin / statut (équivalent à l'ancien `oc list`) |

**Sans option — vue détaillée.** Pour chaque projet, vérifie :
- Chemin local accessible
- Beads initialisé (`.beads/`)
- Clé API configurée (provider + modèle)
- Tracker configuré
- Agents déployés pour la cible par défaut

**Exemple de sortie détaillée :**

```
  MON-APP
    ·  Chemin : /Users/alice/workspace/mon-app
    ✔  Beads initialisé
    ✔  API configurée (anthropic / claude-sonnet-4-5)
    ·  Tracker : aucun
    ✔  Agents déployés (opencode) : 12 fichier(s)
```

**Exemples :**

```bash
oc status          # vue détaillée de tous les projets
oc status --short  # liste compacte (id, chemin, statut)
```

---

## `oc project`

Opérations sur les projets enregistrés : renommage et déplacement.

```bash
oc project rename <OLD_ID> <NEW_ID>
oc project move   <PROJECT_ID> <nouveau_chemin>
```

### `oc project rename`

Renomme un projet dans **tous les fichiers registre** (`projects.md`, `paths.local.md`, `api-keys.local.md`).

```bash
oc project rename MON-APP MON-APP-V2
```

- Demande confirmation avant toute modification
- Met à jour les trois fichiers de façon atomique
- Rappelle de redéployer les agents après le renommage si nécessaire

### `oc project move`

Change le chemin local d'un projet dans `paths.local.md`.

```bash
oc project move MON-APP ~/workspace/mon-app-nouveau
```

- Accepte les chemins avec `~` et les chemins relatifs (résolus depuis `$PWD`)
- Avertit si le dossier de destination n'existe pas encore (peut continuer quand même)

**Exemples :**

```bash
oc project rename OLD-NAME NEW-NAME           # renomme dans tous les registres
oc project move MON-APP ~/workspace/mon-app   # met à jour le chemin local
```

---

## `oc remove`

Supprime un projet du registre (avec confirmation).

```bash
oc remove <PROJECT_ID> [--clean]
```

**Options :**

| Option | Description |
|--------|-------------|
| `--clean` | Supprime également les fichiers agents déployés dans le répertoire du projet (`.opencode/agents/`, `opencode.json`) |

**Exemples :**

```bash
oc remove MON-APP           # retire du registre uniquement
oc remove MON-APP --clean   # retire du registre + nettoie les fichiers déployés
```

> Demande confirmation dans les deux cas. Retire aussi l'entrée de `paths.local.md` et `api-keys.local.md`.

---

## `oc update`

Met à jour les **outils installés** : opencode, Beads (`bd`) et les skills externes enregistrés.

```bash
oc update
```

> Ne met pas à jour les scripts du hub lui-même. Pour cela, utiliser `oc upgrade`.

---

## `oc upgrade`

Met à jour les **sources du hub lui-même** (`git pull` sur le repo local). Avec un argument de version optionnel, bascule sur un tag de release spécifique.

```bash
oc upgrade              # pull le dernier main
oc upgrade v1.1.0       # checkout du tag v1.1.0
```

Après une mise à jour réussie, propose de relancer `oc sync` pour redéployer les agents sur tous les projets enregistrés.

> **Résumé de la distinction :**
> - `oc update` → met à jour les outils installés (opencode, bd, skills externes)
> - `oc upgrade` → met à jour les scripts et agents du hub via git

---

## `oc version`

Affiche la version du hub (lue depuis `config/hub.json`).

```bash
oc version
```

---

## `oc config`

Gère les clés API et les modèles IA par projet, ainsi que la configuration des providers LLM au niveau du hub. Les données projet sont stockées dans `projects/api-keys.local.md` (non versionné) ; la configuration hub dans `config/hub.json`.

```bash
oc config <sous-commande> [options]
```

| Sous-commande | Description |
|---------------|-------------|
| `set [PROJECT_ID] [options]` | Configure la clé API, le modèle et le provider (projet ou hub) |
| `get <PROJECT_ID>` | Affiche la configuration d'un projet (clé masquée) |
| `list [--providers]` | Liste toutes les configurations enregistrées, ou tous les providers du catalogue |
| `unset <PROJECT_ID>` | Supprime la configuration d'un projet (avec confirmation) |
| `init-providers [--force]` | Initialise les fichiers de configuration switcher dans `config/providers/` |

**Options de `oc config set` :**

| Option | Description |
|--------|-------------|
| `--model <modèle>` | Modèle IA (défaut : `claude-sonnet-4-5`) |
| `--provider <provider>` | Provider LLM — en mode interactif, un menu numéroté est proposé depuis le catalogue `providers.json` |
| `--api-key <clé>` | Clé API (saisie masquée en mode interactif) |
| `--base-url <url>` | URL de base (providers compatibles OpenAI) |
| `--family-model <modèle>` | Modèle IA pour les agents de type `family` |
| `--agent-model <modèle>` | Modèle IA pour les agents |

**Comportement de `oc config set` selon les arguments :**

- **`oc config set <PROJECT_ID>`** — interactif, configure le provider et la clé pour ce projet
- **`oc config set`** (sans `PROJECT_ID`) — wizard interactif de configuration du provider **hub** (équivalent à l'ancien `oc provider set-default`)
- **`oc config set --provider anthropic --api-key sk-...`** — configure le provider hub en mode non-interactif
- **`oc config set --provider bedrock`** — provider hub sans clé API (ex. Bedrock avec auth AWS)
- **`oc config set --model claude-opus-4`** — met à jour uniquement le modèle par défaut du hub
- **`oc config set --provider p --api-key k --model m`** — configure provider, clé et modèle hub en une commande

> Après un `set` avec `PROJECT_ID`, propose de re-déployer les agents dans le projet si le chemin est connu.

**`oc config list --providers` :**

Liste tous les providers du catalogue avec leur statut de configuration au niveau du hub.

**`oc config init-providers [--force]` :**

Crée le dossier `config/providers/` et génère les fichiers JSON utilisés par `ocp` : `mammouth.json`, `copilot.json`, `openrouter.json`, `ollama.json`, `bedrock.json`. Crée également `config/providers/.gitignore` pour protéger les clés API. Sans `--force`, les fichiers existants ne sont pas écrasés.

**Exemples :**

```bash
oc config set                                         # wizard interactif hub (provider par défaut)
oc config set --provider anthropic --api-key sk-ant-... # configure le provider hub
oc config set --provider bedrock                      # provider hub sans clé API
oc config set --model claude-opus-4                   # met à jour le modèle hub uniquement
oc config set MON-APP                                 # mode interactif pour MON-APP
oc config set MON-APP --model claude-opus-4-5 --provider anthropic --api-key sk-ant-...
oc config set MON-APP --provider litellm --api-key sk-... --base-url https://api.example.com/v1
oc config get MON-APP                                 # affiche la config (clé masquée)
oc config list                                        # liste toutes les entrées projet
oc config list --providers                            # liste tous les providers du catalogue
oc config unset MON-APP                               # supprime (avec confirmation)
oc config init-providers                              # initialise les fichiers switcher ocp
oc config init-providers --force                      # réinitialise tous les fichiers switcher
```

---

## `oc agent`

Gère les agents canoniques du hub.

```bash
oc agent <sous-commande>
```

| Sous-commande | Description |
|---------------|-------------|
| `list` | Liste tous les agents avec leur id, label et skills |
| `create` | Crée un nouvel agent (workflow interactif) |
| `edit <id>` | Modifie les skills et métadonnées d'un agent existant |
| `info <id>` | Affiche le détail complet d'un agent (frontmatter + corps) |
| `select <PROJECT_ID>` | Choisit les agents à déployer pour un projet |
| `mode <PROJECT_ID>` | Affiche / overrides les modes `primary`/`subagent` par projet |
| `validate [agent-id]` | Valide la cohérence des agents (champs requis, skills existants, unicité des id) |
| `deploy <agent-id> [PROJECT_ID]` | Déploie **un seul agent** |
| `discover <PROJECT_ID>` | Découvre les agents existants du projet et propose de les intégrer |

### `oc agent create` — workflow interactif

1. **Identifiant** — slug unique (ex: `reviewer`)
2. **Label** — nom court affiché dans l'outil (ex: `CodeReviewer`)
3. **Description** — phrase courte décrivant le rôle
4. **Skills** — sélecteur interactif ↑↓/espace avec panneau de description
5. **Corps** — si `opencode` est disponible, proposition de génération automatique via `opencode run`
6. **Prévisualisation** — affichage du fichier `.md` complet avant écriture
7. **Confirmation** — `Y/n` pour créer le fichier

### `oc agent validate`

```bash
oc agent validate             # valide tous les agents canoniques
oc agent validate <agent-id>  # valide uniquement l'agent spécifié
```

Vérifie pour chaque agent :
- Champs requis présents (`id`, `label`, `description`, `skills`)
- Unicité de l'`id` sur l'ensemble des agents
- `mode` valide (`primary` | `subagent` | `all`) si présent
- Tous les skills référencés existent (local ou externe)

Retourne le code 1 si au moins une erreur est détectée.

### `oc agent deploy`

```bash
oc agent deploy <agent-id>                # déploie dans le hub
oc agent deploy <agent-id> <PROJECT_ID>   # déploie dans le projet
```

Déploie **un seul agent** sans tout redéployer. Utile après modification d'un agent ou d'un skill.

- Applique la détection de langue du projet (si configurée)

**Exemples :**

```bash
oc agent deploy planner            # déploie planner dans le hub
oc agent deploy planner MON-APP    # déploie planner dans MON-APP uniquement
```

### `oc agent discover`

```bash
oc agent discover <PROJECT_ID>
```

Scanne `.opencode/agents/` du projet, détecte les agents **non générés par le hub**, résout leur similarité sémantique avec les agents hub, et propose interactivement de les intégrer.

**Deux modes d'intégration :**

| Mode | Comportement | Exemple |
|------|-------------|---------|
| `substitute` | L'agent projet **remplace** l'agent hub correspondant lors du deploy | `my-planner.md` remplace `planner` du hub |
| `complement` | L'agent projet **s'ajoute** en plus des agents hub | `custom-agent.md` coexiste avec tous les agents hub |

**Résolution de similarité (3 niveaux) :**
1. Match exact d'ID (ex: `planner` → `planner`)
2. Lookup dans `config/agent-aliases.json` (ex: `plan` → `planner`, `frontend` → `developer-frontend`)
3. Normalisation avancée avec strip des préfixes courants (`dev-`, `my-`, `agent-`)

**Persistance :** les choix sont écrits dans le champ `External agents` de `projects.md` :
```markdown
- External agents : .opencode/agents/my-planner.md:substitute:planner|.opencode/agents/custom.md:complement
```

**Comportement au deploy :** `oc deploy PROJECT_ID` déclenche automatiquement la découverte si de nouveaux agents non-hub sont présents (sauf en mode non-interactif `OC_NON_INTERACTIVE=1`).

**Exemples :**

```bash
oc agent discover MON-APP          # découverte interactive
oc deploy MON-APP                  # découverte automatique + deploy
```

> Le sélecteur interactif (agents) utilise l'écran alternatif (`smcup`/`rmcup`) — le contenu du terminal parent est intégralement préservé à la fermeture.
> `oc agent keytest` est disponible pour diagnostiquer les terminaux où la navigation ne fonctionne pas (non documenté dans le help, taper `oc agent keytest`).

---

## `oc skills`

Gère les skills externes téléchargés via context7.

```bash
oc skills <sous-commande>
```

| Sous-commande | Description |
|---------------|-------------|
| `search <query>` | Recherche des skills disponibles |
| `add /owner/repo [name]` | Ajoute un skill externe |
| `list` | Liste tous les skills (locaux + externes) |
| `update [name]` | Met à jour un skill externe (ou tous si absent) |
| `info /owner/repo` | Prévisualise les skills disponibles dans un dépôt |
| `used-by <skill>` | Liste les agents qui utilisent ce skill |
| `sync` | Re-télécharge tous les skills externes (utile après clone) |
| `remove <name>` | Supprime un skill externe |
| `validate [name]` | Valide la cohérence des skills (frontmatter, sources) |

### `oc skills validate`

```bash
oc skills validate          # valide tous les skills (locaux + externes)
oc skills validate <name>   # valide uniquement le skill spécifié
```

Vérifie pour chaque fichier skill `.md` :
- Champs frontmatter requis présents (`name`, `description`)
- Cohérence entre le champ `name` et le nom de fichier
- Pour les skills externes : présence de leur source dans `.sources.json`

Retourne le code 1 si au moins une erreur est détectée.

---

## `oc beads`

Gère l'intégration Beads (`bd`) dans les projets enregistrés.

```bash
oc beads <sous-commande>
```

| Sous-commande | Description |
|---------------|-------------|
| `status [PROJECT_ID]` | Vérifie Beads sur tous les projets (ou un seul) |
| `init <PROJECT_ID>` | Initialise `.beads/` dans le projet |
| `list <PROJECT_ID>` | Liste les tickets ouverts du projet |
| `show <PROJECT_ID> <TICKET_ID>` | Affiche le détail d'un ticket |
| `create <PROJECT_ID> [titre] [--label <l>] [--type <t>] [--desc <d>]` | Crée un ticket dans le projet |
| `open <PROJECT_ID>` | Affiche le chemin pour utiliser `bd` manuellement |
| `sync <PROJECT_ID> [options]` | Synchronise avec un tracker externe |
| `tracker status <PROJECT_ID>` | Affiche le statut de connexion au tracker |
| `tracker setup <PROJECT_ID>` | Configure le tracker (interactif) |
| `tracker switch <PROJECT_ID>` | Change de provider (jira ↔ gitlab ↔ none) |
| `tracker set-sync-mode <PROJECT_ID> [mode]` | Définit la direction de sync par défaut pour le projet |

### `oc beads create`

```bash
oc beads create <PROJECT_ID> [titre] [--label <label>] [--type <type>] [--desc <description>]
```

| Argument / Option | Description |
|-------------------|-------------|
| `<PROJECT_ID>` | Projet dans lequel créer le ticket |
| `[titre]` | Titre du ticket — mode interactif si absent |
| `--label <label>` | Étiquette du ticket |
| `--type <type>` | Type de ticket (`feature`, `fix`, `chore`, …) |
| `--desc <description>` | Description longue |

**Exemples :**

```bash
oc beads create MON-APP                                              # mode interactif
oc beads create MON-APP "Ajouter la gestion des rôles"              # titre direct
oc beads create MON-APP "Fix race condition" --type fix --label bug  # avec flags
```

**Options de `oc beads sync` :**

| Option | Description |
|--------|-------------|
| `pull` | Importe seulement depuis le tracker (surcharge le `Sync mode` du projet) |
| `push` | Exporte seulement vers le tracker (surcharge le `Sync mode` du projet) |
| `--dry-run` | Simule sans modifier |

> La direction par défaut de `oc beads sync` est contrôlée par le champ `Sync mode` dans `projects.md`
> (défini avec `oc beads tracker set-sync-mode <PROJECT_ID>`). Valeur par défaut : `bidirectional`.
> Un flag CLI prend toujours le dessus sur le mode configuré.

> `oc start` avertit automatiquement si `.beads/` n'est pas présent dans le projet.

### `oc beads board` — Board kanban terminal

Affiche un tableau kanban dans le terminal avec 4 colonnes : **OPEN**, **IN PROGRESS**, **REVIEW**, **BLOCKED**.
Aucune dépendance externe — pur shell + `bd`.

```bash
oc beads board [PROJECT_ID] [--watch] [--interval <sec>]
```

| Option | Description |
|--------|-------------|
| `[PROJECT_ID]` | Projet à afficher (auto-découverte depuis le répertoire courant si absent) |
| `--watch` | Mode rafraîchissement automatique (Ctrl+C pour quitter) |
| `--interval <sec>` | Intervalle entre les rafraîchissements en secondes (défaut : 5) |

**Exemples :**

```bash
oc beads board MON-APP              # affiche le board une fois
oc beads board MON-APP --watch      # rafraîchissement en direct toutes les 5s
oc beads board MON-APP --watch --interval 10   # rafraîchissement toutes les 10s
```

> Le board s'adapte à la largeur du terminal. Les titres de tickets sont tronqués si nécessaire.
> Badges de priorité : **P0** en rouge, P1 en jaune, P2/P3 en grisé.

---

## `oc service`

Gestion des services et intégrations externes via MCP. Voir la [référence complète](services.fr.md).

```bash
oc service [setup|status|list|remove] [nom-du-service]
```

**Exemples :**

```bash
oc service list                     # liste les services disponibles
oc service setup figma              # configure Figma (wizard interactif)
oc service status                   # état de tous les services
oc service remove gitlab            # supprime la config GitLab

# Aliases raccourcis
oc figma setup                      # = oc service setup figma
oc gitlab status                    # = oc service status gitlab
```
> Couleurs des bordures de colonnes : grisé (open), bleu (in progress), jaune (review), rouge (blocked).

---

## `oc metrics`

Affiche les métriques de vélocité, coûts et usage du hub OpenCode.

```bash
oc metrics                  # 7 derniers jours (défaut)
oc metrics --period today   # aujourd'hui seulement
oc metrics --period week    # 7 derniers jours
oc metrics --period month   # 30 derniers jours
```

**Sources de données :**

| Source | Données collectées | Prérequis |
|--------|-------------------|-----------|
| `~/.local/share/opencode/opencode.db` | Sessions, coûts, tokens, modèles, agents | `sqlite3` |
| `bd list` par projet | Tickets par statut | `bd` (optionnel) |
| `.opencode/metrics.jsonl` | Vélocité workflow (rétrocompat.) | — |
| `~/.claude/context-mode/sessions/` | Tokens économisés par context-mode | context-mode (optionnel) |
| `rtk gain` | Tokens économisés par RTK | RTK 0.42+ (optionnel) |

**Sections affichées (dans l'ordre) :**

1. **Vue globale** : sessions actives (dont créées), coût exact de la période (basé sur les steps exécutés — inclut les sessions multi-jours), tokens input/output, cache write/read, cache hit rate + économies estimées
   - **Économies plugins** *(si context-mode ou RTK installé)* : tokens économisés, dollars économisés et réduction de contexte. La période correspond au filtre `--period` pour context-mode ; RTK affiche toujours ses statistiques globales.
2. **Coût total** : lifetime (toutes sessions) + breakdown Aujourd'hui / 7 jours / 30 jours par steps. La ligne correspondant au `--period` actif est mise en évidence. Toujours affiché avec les 3 fenêtres fixes quelle que soit la période choisie.
3. **Coût** : sous-sections par projet, par agent, par modèle (vue fusionnée)
4. **Activité** : répartition des sessions par catégorie d'usage (code, exploration, planification, review, debug, conversation) avec coût et pourcentage
5. **Sessions récentes** : 5 dernières sessions avec titre, agent, coût et date
6. **Tickets par projet** *(si `bd` disponible)* : compteurs par statut pour chaque projet Beads
7. **Vélocité workflow** *(si `metrics.jsonl` présent)* : tickets complétés, temps moyen, cycles review

> **Note sur le coût exact** : les valeurs de coût sont calculées à partir des steps (`part.step-finish`) par date d'exécution, pas par date de création de session. Une session démarrée hier mais toujours active aujourd'hui contribue au coût de la journée en cours (`--period today`).

**Options :**

| Option | Description |
|--------|-------------|
| `--period today` | Données de la journée en cours uniquement |
| `--period week` | 7 derniers jours (défaut) |
| `--period month` | 30 derniers jours |

**Comportement si `sqlite3` absent :** Message d'aide affiché, exit 0 (non bloquant). Les sections Tickets et Vélocité restent disponibles.

**Prérequis :** `sqlite3` (natif macOS — `sudo apt-get install sqlite3` sur Linux)

---

## `oc dashboard`

Affiche une vue synthétique du hub OpenCode — conçu pour un check rapide quotidien (10 secondes). Les informations financières apparaissent en premier.

```bash
oc dashboard
```

**Sources de données :**

| Source | Données collectées | Prérequis |
|--------|-------------------|-----------|
| `~/.local/share/opencode/opencode.db` | Budget sessions, sessions récentes | `sqlite3` |
| `bd list` par projet | Tickets actifs, bloqués, complétés | `bd` (optionnel) |
| `.opencode/session-state.json` | Session orchestrateur active (rétrocompat.) | `jq` |
| `~/.claude/context-mode/sessions/` | Tokens économisés par context-mode (lifetime) | context-mode (optionnel) |
| `rtk gain` | Tokens économisés par RTK (global) | RTK 0.42+ (optionnel) |

**Sections affichées (dans l'ordre) :**

1. **Budget** : coût exact par steps (aujourd'hui depuis minuit calendaire / cette semaine / ce mois), sessions actives (dont créées), cache hit rate inline, et ligne **Total lifetime**. Les coûts sont basés sur les steps exécutés — une session ouverte la veille mais toujours active contribue au coût d'aujourd'hui.
2. **Économies IA** *(si context-mode ou RTK installé)* : tokens économisés (lifetime), dollars économisés, réduction de contexte. Silencieusement absente si aucun plugin installé.
3. **Session active** *(si orchestrateur en cours via `oc start`)* : agent, ticket courant, action, heure de démarrage
4. **Projets** : pour chaque projet Beads — ticket en cours et compteurs par statut (✅ / 🔄 / ⏳ / 🚫)
5. **Sessions récentes** : 5 dernières sessions (titre, agent, coût, date)

**Comportement si `sqlite3` absent :** Les sections Budget et Sessions récentes affichent un message d'aide. Les sections Projets (bd) et Économies IA restent disponibles.

**Comportement si `bd` absent :** La section Projets affiche un message d'aide. Les autres sections restent disponibles.

**Prérequis :** `sqlite3` pour les sections coûts/sessions ; `bd` (optionnel) pour les tickets

> Pour voir les métriques détaillées sur une période personnalisée, utiliser `oc metrics --period month`.

---

## `oc optimize`

Analyse les patterns de gaspillage de tokens du hub et produit un rapport avec un grade global.

```bash
oc optimize                      # 30 derniers jours (défaut)
oc optimize --period week        # 7 derniers jours
oc optimize --period today       # aujourd'hui
oc optimize --project T-SRU      # filtré sur un projet
```

**9 analyses déterministes (sans LLM) :**

| Analyse | Niveau | Signal détecté |
|---------|--------|---------------|
| MCP servers inutilisés | Critique | Server déployé mais 0 appels sur la période |
| Sessions sans modification | Critique/Warning | Sessions coûteuses sans aucun edit/write de fichier |
| Ratio Read/Edit faible | Critique/Warning | < 1.0 → critique, < 2.0 → warning (idéal ≥ 2.0) |
| Taux d'erreurs élevé | Critique/Warning | > 25% → critique, > 10% → warning |
| Fichiers re-lus excessivement | Warning | Même fichier lu 5+ fois dans une session |
| Délégation intensive | Info | > 40% de tool calls = `task` |
| Skills Bucket B inutilisées | Warning | Aucun chargement via `skill` sur la période |
| Sessions pure conversation | Info | Sessions sans aucun tool call |
| Cache hit rate faible | Warning | < 30% — tokens input rechargés inutilement |

**Grading :**

| Grade | Score (critiques × 3 + warnings) |
|-------|----------------------------------|
| A | 0 |
| B | 1–2 |
| C | 3–5 |
| D | 6–9 |
| E | 10–14 |
| F | 15+ |

Chaque finding inclut une description et une correction suggérée prête à copier-coller.

**Prérequis :** `sqlite3`

**Options :**

| Option | Description |
|--------|-------------|
| `--period today\|week\|month` | Période d'analyse (défaut : 30 jours) |
| `--project PROJECT_ID` | Filtrer sur un seul projet |

---

## `oc yield`

Corrèle les sessions OpenCode avec les commits git pour mesurer le rendement réel.

```bash
oc yield                      # 7 derniers jours (défaut)
oc yield --period today       # aujourd'hui
oc yield --period month       # 30 derniers jours
oc yield --project T-SRU      # filtré sur un projet
```

**Classification de chaque session :**

| Catégorie | Définition | Coût affiché |
|-----------|-----------|-------------|
| **Productive** | Au moins un commit git dans les 24h suivant la session | En vert |
| **Abandonnée** | Aucun commit dans la fenêtre de 24h | En jaune |
| **Revertée** | Commit trouvé mais son message commence par `Revert` | En rouge |

**Résolution worktrees :** les sessions dans des worktrees git (ex: `~/workspace/t-sru-2/`) sont automatiquement associées au dépôt principal via `git rev-parse --show-toplevel`.

**Prérequis :** `sqlite3` + `git`

**Options :**

| Option | Description |
|--------|-------------|
| `--period today\|week\|month` | Période d'analyse (défaut : 7 jours) |
| `--project PROJECT_ID` | Filtrer sur un seul projet |

> Sessions abandonnées coûteuses → utiliser `oc optimize` pour identifier les causes.
