# Référence CLI — commandes `oc`

Toutes les commandes disponibles via le point d'entrée `oc.sh` (alias recommandé : `oc`).

---

## Synopsis global

```
oc <commande> [sous-commande] [options] [arguments]
```

---

## `oc install`

Installe les outils, crée la structure du hub et configure les cibles actives.

```bash
oc install
```

**Comportement :**
- Interactif — propose un menu de sélection des cibles
- Vérifie et **demande confirmation** avant d'installer chaque dépendance (Node.js, opencode, Beads, bun)
- Si `config/hub.json` existe déjà, demande confirmation avant d'écraser

**Options de cible :**

| Choix | Cibles configurées |
|-------|--------------------|
| 1 (défaut) | OpenCode |
| 2 | OpenCode |
| 3 | Tout |

---

## `oc uninstall`

Désinstalle opencode-hub et nettoie les artefacts créés lors de l'installation.

```bash
oc uninstall
# équivalent à :
bash ~/.opencode-hub/uninstall.sh
```

**Comportement :**

Guide la désinstallation en 4 étapes, toutes optionnelles et avec confirmation explicite :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets (`.opencode/agents/`, `opencode.json`, `.opencode/agents/`) | `[y/N]` |
| 2 | Supprimer le hub (`~/.opencode-hub`) | `[y/N]` |
| 3 | Retirer l'alias `oc` et les exports bun du fichier rc shell | `[Y/n]` |
| 4 | Désinstaller les outils système : `opencode`, `beads`, `bun` (séparément) | `[y/N]` |

> `jq` et `node` ne sont pas proposés à la désinstallation (usage général, risque de casser d'autres outils).
>
> Un backup `.bak` est créé automatiquement avant toute modification du fichier rc.

---

## `oc deploy`

Génère les fichiers agents pour une cible dans un projet. Quand un `PROJECT_ID` est fourni, **détecte automatiquement la stack du projet** et injecte les skills spécifiques correspondants dans les agents developer (en plus de leurs skills déclarés statiquement).

```bash
oc deploy <target> [PROJECT_ID]
oc deploy --check [target] [PROJECT_ID]
oc deploy --diff  [target] [PROJECT_ID]
```

**Arguments :**

| Argument | Valeurs | Description |
|----------|---------|-------------|
| `<target>` | `opencode`, `opencode`, `all` | Cible à déployer |
| `[PROJECT_ID]` | ID d'un projet enregistré | Optionnel — déploie au niveau du hub si absent (pas de détection de stack) |

**Options :**

| Option | Description |
|--------|-------------|
| `--check` | Vérifie si les fichiers sont à jour sans déployer |
| `--diff` | Compare les sources avec les fichiers déployés ; propose le déploiement si un écart est détecté |

**Détection de stack :**

Quand `PROJECT_ID` est fourni, `oc deploy` lit les fichiers de dépendances du projet (`package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, fichiers d'infrastructure, etc.) pour détecter la stack active. Les skills correspondants dans `skills/developer/stacks/` sont ensuite injectés dans les agents developer selon le mapping de `config/stack-skills.json`.

Ainsi, un agent `developer-frontend` déployé sur un projet React/Vitest/Playwright recevra automatiquement `dev-standards-react`, `dev-standards-vitest` et `dev-standards-playwright` — sans aucun changement de configuration d'agent.

**Exemples :**

```bash
oc deploy opencode              # déploie OpenCode au niveau du hub (pas de détection de stack)
oc deploy opencode MON-APP      # déploie OpenCode dans MON-APP (avec détection de stack)
oc deploy all MON-APP           # déploie toutes les cibles actives dans MON-APP
oc deploy --check               # vérifie toutes les cibles actives (hub)
oc deploy --check opencode      # vérifie OpenCode (hub)
oc deploy --check all MON-APP   # vérifie toutes les cibles pour MON-APP
oc deploy --diff all MON-APP    # affiche le diff sources → déployés pour MON-APP
```

**Sorties générées :**

| Cible | Fichiers générés |
|-------|-----------------|
| `opencode` | `.opencode/agents/*.md` + `opencode.json` (régénéré si une clé API ou un PROJECT_ID est défini) |
| `opencode` | `.opencode/agents/*.md` |

**Codes de sortie `--check` :**
- `0` : tout est à jour
- `1` : au moins un fichier est obsolète ou manquant

> Un spinner animé (`⠋⠙⠹…`) est affiché pendant le déploiement de chaque cible.

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
oc start [PROJECT_ID] [prompt] [--dev [--label <label>] [--assignee <user>]] [--onboard]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | ID du projet — sélection interactive si absent |
| `[prompt]` | Prompt de démarrage passé à l'outil |

**Options :**

| Option | Description |
|--------|-------------|
| `--dev` | Mode développement — charge les tickets `ai-delegated` ouverts dans le prompt de démarrage. Effectue un sync tracker `--pull-only` automatique avant le lancement. |
| `--dev --label <label>` | Comme `--dev`, mais filtre les tickets ayant le label `<label>` |
| `--dev --assignee <user>` | Comme `--dev`, mais filtre les tickets assignés à `<user>` |
| `--onboard` | Injecte un prompt de découverte projet pour onboarder l'agent sur le codebase |

> `--dev` et `--onboard` sont mutuellement exclusifs. `--label` et `--assignee` sont mutuellement exclusifs.

**Exemples :**

```bash
oc start                                        # sélection interactive du projet
oc start MON-APP                                # lance l'outil dans MON-APP
oc start MON-APP "explique l'architecture"      # avec prompt de démarrage
oc start MON-APP --dev                          # charge les tickets ai-delegated
oc start MON-APP --dev --label ai-delegated     # filtre par label
oc start MON-APP --dev --assignee alice         # filtre par assignee
oc start MON-APP --onboard                      # prompt de découverte projet
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

Lance une code review IA sur une branche en invoquant l'agent `reviewer` avec le diff complet injecté dans le prompt.

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
2. **Vérification projects.md** — si le projet a une sélection d'agents restrictive (pas `all`), vérifie que `reviewer` est inclus :
   - Si manquant → propose de l'ajouter + redéployer
3. **Vérification déploiement physique** — si le dossier agents est absent ou si `reviewer.md` manque, propose `oc deploy`
4. **Génération du diff** — exécute `git diff main...<branche>` et injecte le résultat complet dans le prompt de bootstrap
5. **Lancement** — ouvre l'outil avec `--agent reviewer` et le prompt contenant le diff

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
Commande diff utilisée : git diff main...feat/login

→ Lire CONVENTIONS.md à la racine du projet avant la review   ← si le fichier existe

--- DIFF ---

diff --git a/src/auth/login.ts b/src/auth/login.ts
...

--- FIN DU DIFF ---

Workflow :
1. Si CONVENTIONS.md existe à la racine → le lire pour appliquer les conventions réelles du projet
2. Analyser le diff ci-dessus selon la checklist systématique du skill review-protocol
3. Produire le rapport structuré par sévérité : Critique → Majeur → Mineur → Suggestion → Points positifs
```

> L'agent `reviewer` ne modifie aucun fichier — il produit uniquement un rapport d'analyse.
> Pour un diff vide (branche à jour avec `main`), le prompt l'indique explicitement.

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

## `oc init`

Enregistre un projet dans le hub. Guide l'utilisateur en **5 étapes numérotées** et affiche un récapitulatif coloré à la fin.

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
| 3 — Agents & cibles | Sélection des agents, des cibles de déploiement, et des agents natifs OpenCode à désactiver |
| 4 — Fournisseur LLM | Configuration d'un provider spécifique au projet (surcharge le hub) |
| 5 — Déploiement | Proposition de déploiement immédiat |

> La création du dossier a lieu en **fin d'étape 1** — Beads est ainsi garanti accessible dès l'étape 2.

**Rendu wizard :**

```
◆  Initialisation d'un projet
│
│
◇  Étape 1/5 — Informations projet
│
│  PROJECT_ID (ex: MON-APP) :
│  ...
│
◇  Étape 2/5 — Beads & tracker
│
│  ...
```

**Récapitulatif final :**

```
┌─ MON-APP initialisé ──────────────────────────────┐
│  Chemin       /Users/alice/workspace/mon-app       │
│  Nom          Mon Application                      │
│  Stack        Vue 3 + Laravel                      │
│  Tracker      jira                                 │
│  Beads        ◆ initialisé                         │
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
| `--clean` | Supprime également les fichiers agents déployés dans le répertoire du projet (`.opencode/agents/`, `opencode.json`, `.opencode/agents/` selon les cibles actives) |

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

Gère les clés API et les modèles IA par projet. Les données sont stockées dans `projects/api-keys.local.md` (non versionné).

```bash
oc config <sous-commande> [options]
```

| Sous-commande | Description |
|---------------|-------------|
| `set <PROJECT_ID> [options]` | Configure la clé API, le modèle et le provider pour un projet |
| `get <PROJECT_ID>` | Affiche la configuration d'un projet (clé masquée) |
| `list` | Liste toutes les configurations enregistrées |
| `unset <PROJECT_ID>` | Supprime la configuration d'un projet (avec confirmation) |

**Options de `oc config set` :**

| Option | Description |
|--------|-------------|
| `--model <modèle>` | Modèle IA (défaut : `claude-sonnet-4-5`) |
| `--provider <provider>` | Provider LLM — en mode interactif, un menu numéroté est proposé depuis le catalogue `providers.json` |
| `--api-key <clé>` | Clé API (saisie masquée en mode interactif) |
| `--base-url <url>` | URL de base (providers compatibles OpenAI) |

> Sans options, `set` est interactif — propose les valeurs actuelles comme défaut et affiche un menu numéroté des providers disponibles.
> Après un `set`, propose de re-déployer les agents dans le projet si le chemin est connu.

**Exemples :**

```bash
oc config set MON-APP                                 # mode interactif
oc config set MON-APP --model claude-opus-4-5 --provider anthropic --api-key sk-ant-...
oc config set MON-APP --provider litellm --api-key sk-... --base-url https://api.example.com/v1
oc config get MON-APP                                 # affiche la config (clé masquée)
oc config list                                        # liste toutes les entrées
oc config unset MON-APP                               # supprime (avec confirmation)
```

---

## `oc provider`

Gère les providers LLM au niveau **hub** (configuration globale partagée par tous les projets).

```bash
oc provider <sous-commande>
```

| Sous-commande | Description |
|---------------|-------------|
| `list` | Liste tous les providers du catalogue avec leur statut hub |
| `set-default` | Configure le provider par défaut du hub (interactif) |

> Pour configurer le provider d'un **projet spécifique**, utiliser `oc config set <PROJECT_ID>`.

**Exemples :**

```bash
oc provider list         # liste tous les providers disponibles
oc provider set-default  # wizard interactif pour choisir le provider hub
```

---

## `oc agent`

Gère les agents canoniques du hub.

```bash
oc agent <sous-commande>
```

| Sous-commande | Description |
|---------------|-------------|
| `list` | Liste tous les agents avec leur id, label et targets |
| `create` | Crée un nouvel agent (workflow interactif) |
| `edit <id>` | Modifie les skills et métadonnées d'un agent existant |
| `info <id>` | Affiche le détail complet d'un agent (frontmatter + corps) |
| `select <PROJECT_ID>` | Choisit les agents à déployer pour un projet |
| `mode <PROJECT_ID>` | Affiche / overrides les modes `primary`/`subagent` par projet |
| `validate [agent-id]` | Valide la cohérence des agents (champs requis, skills existants, targets valides, unicité des id) |
| `deploy <agent-id> [PROJECT_ID]` | Déploie **un seul agent** sur les cibles actives (ou celles du projet) |

### `oc agent create` — workflow interactif

1. **Identifiant** — slug unique (ex: `reviewer`)
2. **Label** — nom court affiché dans l'outil (ex: `CodeReviewer`)
3. **Description** — phrase courte décrivant le rôle
4. **Cibles** — sélecteur interactif ↑↓/espace : `opencode`, `opencode`
5. **Skills** — sélecteur interactif ↑↓/espace avec panneau de description
6. **Corps** — si `opencode` est disponible, proposition de génération automatique via `opencode run`
7. **Prévisualisation** — affichage du fichier `.md` complet avant écriture
8. **Confirmation** — `Y/n` pour créer le fichier

### `oc agent validate`

```bash
oc agent validate             # valide tous les agents canoniques
oc agent validate <agent-id>  # valide uniquement l'agent spécifié
```

Vérifie pour chaque agent :
- Champs requis présents (`id`, `label`, `description`, `targets`, `skills`)
- Unicité de l'`id` sur l'ensemble des agents
- `mode` valide (`primary` | `subagent` | `all`) si présent
- Toutes les cibles dans `targets` reconnues (`opencode`, `opencode`)
- Tous les skills référencés existent (local ou externe)

Retourne le code 1 si au moins une erreur est détectée.

### `oc agent deploy`

```bash
oc agent deploy <agent-id>                # déploie sur les cibles actives du hub
oc agent deploy <agent-id> <PROJECT_ID>   # déploie sur les cibles configurées du projet
```

Déploie **un seul agent** sans tout redéployer. Utile après modification d'un agent ou d'un skill.

- Respecte les cibles du projet si `PROJECT_ID` est fourni (sinon cibles actives du hub)
- Vérifie que l'agent supporte la cible avant de déployer
- Applique la détection de langue du projet (si configurée)

**Exemples :**

```bash
oc agent deploy planner            # déploie planner dans le hub
oc agent deploy planner MON-APP    # déploie planner dans MON-APP uniquement
```

> Le sélecteur interactif (agents, cibles) utilise l'écran alternatif (`smcup`/`rmcup`) — le contenu du terminal parent est intégralement préservé à la fermeture.
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
| `--pull-only` | Importe seulement depuis le tracker (surcharge le `Sync mode` du projet) |
| `--push-only` | Exporte seulement vers le tracker (surcharge le `Sync mode` du projet) |
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
> Couleurs des bordures de colonnes : grisé (open), bleu (in progress), jaune (review), rouge (blocked).
