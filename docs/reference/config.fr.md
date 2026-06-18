# Référence de configuration

---

## `config/hub.json`

Configuration globale du hub. Créé par `oc install` et modifiable manuellement.

### Structure complète

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan"]
  },
}
```

### Référence des clés

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `version` | string | — | Version du hub (lue par `oc version`) |
| `default_provider` | object | — | Configuration du provider LLM par défaut pour tous les projets |
| `default_provider.name` | string | `"anthropic"` | Nom du provider (`anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`) |
| `default_provider.api_key` | string | `""` | Clé API du provider (masquée en affichage, auto-ignorée par git si définie) |
| `default_provider.base_url` | string | `""` | URL de base customisée (optionnel pour litellm et autres) |
| `default_provider.model` | string | `""` | Modèle IA par défaut pour ce provider (si vide : fallback à `opencode.model`) |
| `opencode.model` | string | — | Modèle IA injecté dans `opencode.json` des projets déployés (si `default_provider.model` est vide) |
| `opencode.disabled_native_agents` | array | `[]` | Agents natifs OpenCode désactivés par défaut (`build`, `plan`, `general`, `explore`, `pathfinder`) — surchargeables par projet via `- Disable agents :` dans `projects.md` |

### Exemple minimal (OpenCode uniquement)

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

### Exemple avec provider par défaut configuré

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "mammouth",
    "api_key": "sk-xxx...",
    "base_url": "https://api.mammouth.ai/v1",
    "model": "claude-opus-4-5"
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

### Exemple complet

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "sk-ant-xxx...",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

---

## `projects/projects.md`

Registre local des projets. **Ignoré par git** — chaque développeur maintient
le sien. Créé automatiquement depuis `projects/projects.example.md` au premier
`oc install` ou `oc init`.

### Format

```markdown
## PROJECT_ID
- Nom : Nom lisible du projet
- Stack : Stack technique (ex: Vue 3 + Laravel)
- Board Beads : Identifiant du board Beads
- Tracker : jira | gitlab | none
- Labels : label1, label2, label3
- Langue : english        # optionnel — si absent : agents en français par défaut
- Agents : all            # optionnel — all (défaut) ou liste CSV d'agent-ids
- Modes : agent-id:mode,agent-id:mode  # optionnel — override des modes primary/subagent par agent
- Disable agents : plan,build  # optionnel — surcharge hub.json pour ce projet
```

### Exemple

```markdown
## MON-APP
- Nom : Mon Application
- Stack : Vue 3 + Laravel 10
- Board Beads : MON-APP
- Tracker : jira
- Labels : feature, fix, front, back

## API-GATEWAY
- Nom : API Gateway
- Stack : Node.js + Fastify
- Board Beads : API-GATEWAY
- Tracker : none
- Labels : feature, fix, api
- Langue : english
- Agents : orchestrator,orchestrator-dev,developer-backend,developer-api
- Modes : developer-backend:primary,developer-api:primary
```

### Règles

- `PROJECT_ID` : lettres, chiffres, `-` et `_` uniquement — pas d'espaces ni de slashes
- `Tracker` : `jira`, `gitlab` ou `none`
- `Langue` : optionnel — valeur libre (ex: `english`, `spanish`) — si absent, les agents s'expriment en français
- `Agents` : optionnel — `all` ou CSV d'identifiants d'agents — filtré au déploiement
- `Modes` : optionnel — CSV de paires `agent-id:mode` — surcharge le frontmatter des agents. Modes : `primary`, `subagent`. Laisser vide pour revenir aux valeurs frontmatter.
- `Disable agents` : optionnel — CSV d'agents natifs OpenCode à désactiver (`build`, `plan`, `general`, `explore`, `pathfinder`) — surcharge `opencode.disabled_native_agents` de `hub.json`. Vide = utiliser le défaut hub.
- Ce fichier est **local** — ne jamais le committer

---

## `projects/projects.example.md`

Template versionné pour `projects.md`. Copié automatiquement en `projects/projects.md`
si ce fichier est absent.

Modifier ce template pour définir la structure de projet par défaut de votre équipe.

---

## `projects/paths.local.md`

Associe chaque `PROJECT_ID` à un chemin local sur la machine du développeur.
**Ignoré par git.**

### Format

```
PROJECT_ID=/chemin/absolu/vers/le/projet
```

### Exemple

```
MON-APP=~/workspace/mon-app
API-GATEWAY=/home/user/projets/api-gateway
AUTRE-APP=~/dev/autre-app
```

### Règles

- Un `PROJECT_ID` par ligne
- Chemins absolus ou avec `~` (expansé par le shell)
- Ne pas committer ce fichier — chaque développeur a ses propres chemins locaux

## `projects/api-keys.local.md`

Stocke les clés API et modèles configurés par projet via `oc config`.
**Ignoré par git** — ne jamais committer ce fichier.

### Format

```ini
[PROJECT_ID]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-...

[AUTRE-PROJET]
model=claude-sonnet-4-5
provider=mammouth
api_key=sk-bRf...
base_url=https://api.mammouth.ai/v1

[PROJET-GITHUB]
model=claude-sonnet-4-5
provider=github-models
api_key=ghp_xxx...
base_url=https://models.inference.ai.azure.com
```

### Clés disponibles par section

| Clé | Requis | Description |
|-----|--------|-------------|
| `model` | oui | Modèle IA (ex: `claude-opus-4-5`, `claude-haiku-4-5`) |
| `provider` | oui | `anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`, ou `litellm` |
| `api_key` | oui | Clé API — jamais affichée en clair |
| `base_url` | non | URL de base (recommandé pour `mammouth`, `github-models`, `bedrock`, `ollama`, et requis pour `litellm` générique) |

### Providers supportés

| Provider | Requis API Key | Base URL défaut | Description |
|----------|----------------|-----------------|-------------|
| `anthropic` | oui | — | API Anthropic directe |
| `mammouth` | oui | `https://api.mammouth.ai/v1` | Proxy OpenAI-compatible (FR-hosted) |
| `github-models` | oui | `https://models.inference.ai.azure.com` | GitHub Models API |
| `bedrock` | oui | — (spécifique AWS) | AWS Bedrock |
| `ollama` | non | `http://localhost:11434/v1` | LLM local compatible OpenAI |
| `litellm` | oui | ⚠️ requis | Proxy litellm générique (custom) |

### Effets lors du déploiement

Lors d'un `oc deploy <PROJECT_ID>`, si une entrée existe pour le projet :

- `opencode.json` et `.opencode/` sont ajoutés au `.git/info/exclude` du projet cible **avant** l'écriture du fichier (exclusion locale, invisible pour les autres devs)
- `opencode.json` est régénéré avec le bloc `provider` complet
- Le fichier est créé avec les permissions `600`

Si `PROJECT_ID` est défini sans clé API (ou après un `oc config unset`), `opencode.json` est
également régénéré pour retirer tout ancien bloc `provider`.

Pour OpenCode, la clé est injectée comme `ANTHROPIC_API_KEY` au moment du `oc start` (Anthropic uniquement).

---

## `oc config` — commande CLI

Gère les entrées de `projects/api-keys.local.md` ainsi que la configuration des providers LLM au niveau du hub (`config/hub.json`).

### Sous-commandes

```
oc config set [PROJECT_ID] [options]   Créer ou mettre à jour une configuration (projet ou hub)
oc config get <PROJECT_ID>             Afficher la configuration (clé masquée)
oc config list [--providers]           Lister toutes les configurations, ou les providers du catalogue
oc config unset <PROJECT_ID>           Supprimer une configuration
oc config init-providers [--force]     Initialiser les fichiers switcher dans config/providers/
```

### Options de `oc config set`

| Option | Description |
|--------|-------------|
| `--model <model>` | Modèle IA |
| `--provider <provider>` | `anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`, ou `litellm` |
| `--api-key <key>` | Clé API (si omis : saisie masquée interactive) |
| `--base-url <url>` | Base URL (optionnel pour la plupart des providers) |
| `--family-model <model>` | Modèle IA pour les agents de type `family` |
| `--agent-model <model>` | Modèle IA pour les agents |

**Comportement selon les arguments :**

- `oc config set <PROJECT_ID>` — interactif, configure le provider et la clé pour ce projet
- `oc config set` (sans `PROJECT_ID`) — wizard interactif de configuration du provider **hub**
- `oc config set --provider anthropic --api-key sk-...` — configure le provider hub en mode non-interactif
- `oc config set --provider bedrock` — provider hub sans clé API
- `oc config set --model claude-opus-4` — met à jour uniquement le modèle par défaut du hub
- `oc config set --provider p --api-key k --model m` — configure provider, clé et modèle hub en une commande

`oc config list --providers` liste tous les providers du catalogue avec leur statut hub.

`oc config init-providers` crée `config/providers/` et génère les fichiers JSON utilisés par `ocp` (`mammouth.json`, `copilot.json`, `openrouter.json`, `ollama.json`, `bedrock.json`) ainsi que `config/providers/.gitignore`. Sans `--force`, les fichiers existants ne sont pas écrasés.

### Exemple

```sh
# Wizard interactif hub (provider par défaut)
./oc.sh config set

# Configurer le provider hub en ligne de commande
./oc.sh config set --provider anthropic --api-key sk-ant-...

# Provider hub sans clé API (ex. Bedrock)
./oc.sh config set --provider bedrock

# Mettre à jour uniquement le modèle hub
./oc.sh config set --model claude-opus-4

# Configuration projet interactive
./oc.sh config set MON-PROJET

# Configuration projet en ligne de commande
./oc.sh config set MON-PROJET --model claude-opus-4-5 --provider anthropic

# Avec MammouthAI
./oc.sh config set MON-PROJET --provider mammouth --api-key sk-xxx

# Lister les providers du catalogue
./oc.sh config list --providers

# Initialiser les fichiers switcher ocp
./oc.sh config init-providers

# Vérifier
./oc.sh config get MON-PROJET

# Supprimer
./oc.sh config unset MON-PROJET
```

---

## `oc project` — gestion des projets CLI

Gère les entrées du registre de projets dans `projects/projects.md` et `projects/paths.local.md`.

### Sous-commandes

```
oc project rename <OLD_ID> <NEW_ID>      Renomme un projet dans tous les fichiers registre
oc project move <PROJECT_ID> <path>      Change le chemin local d'un projet
oc project configure [PROJECT_ID]        Reconfigure les champs d'un projet existant
```

### `oc project configure`

Wizard interactif pour mettre à jour n'importe quel champ de `projects.md` pour un projet existant.
Si aucun `PROJECT_ID` n'est fourni, une liste numérotée interactive s'affiche.

Pour chaque champ, la valeur actuelle est affichée. Appuyer sur **Entrée** pour la conserver.

| Champ | Valeurs | Description |
|-------|---------|-------------|
| `Stack` | texte libre | Technologies utilisées (ex: `Vue 3 + Laravel`) |
| `Tracker` | `none` \| `jira` \| `gitlab` | Tracker externe |
| `Labels` | CSV | Labels Beads (ex: `feature,fix,front,back`) |
| `Langue` | texte libre | Langue des agents (`english`, `spanish` — absent = français) |
| `Disable agents` | CSV | Agents natifs OpenCode à désactiver (`build`, `plan`, `general`, `explore`, `pathfinder`) — `none` pour vider |
| `MCP` | `all` \| `none` \| CSV | Serveurs MCP à activer |
| `Worktree` | `enabled` \| `disabled` | Activer les git worktrees pour le travail en parallèle |
| `Worktree auto cleanup` | `true` \| `false` | Supprimer automatiquement les worktrees mergés *(affiché uniquement si Worktree est activé)* |
| `Worktree base branch` | nom de branche | Branche de base pour le cleanup (défaut : `main`) *(affiché uniquement si Worktree est activé)* |

> Note : les champs `Agents` et `Modes` ont des commandes dédiées — utiliser `oc agent select` et `oc agent mode`.

### Exemples

```sh
# Wizard interactif (liste de projets)
./oc.sh project configure

# Configurer un projet spécifique
./oc.sh project configure MON-APP

# Renommer un projet
./oc.sh project rename MON-APP MON-APP-V2

# Déplacer un projet vers un nouveau chemin
./oc.sh project move MON-APP ~/workspace/mon-app-new
```

---

## `ocp` — switcher interactif de providers

Fonction shell injectée dans `~/.zshrc` par le hub. Permet de lancer opencode
en choisissant le provider à utiliser, tout en conservant la logique complète de `oc start`
(déploiement des agents, mode `--dev`, sync Beads, onboarding, etc.).

Requiert que `config/providers/` soit initialisé via `oc config init-providers`.

### Usage

```sh
ocp                          # picker interactif du provider (fzf ou select natif)
ocp mammouth                 # lancer avec mammouth (picker interactif du projet)
ocp mammouth openhub    # lancer le projet openhub avec mammouth
ocp bedrock MON-APP --dev    # mode --dev avec bedrock
ocp --list                   # lister les providers disponibles
```

### Comportement

`ocp <provider> [args...]` est équivalent à :
```sh
./oc.sh start --provider <provider> [args...]
```

Le flag `--provider` override le provider effectif pour la génération de `opencode.json` —
les modèles par agent sont préfixés et aliasés selon le provider sélectionné (voir [Préfixage provider](model-resolution.fr.md#préfixage-provider-dans-opencodejson)).

### Installation

La fonction est injectée automatiquement dans `~/.zshrc` lors de `oc install`.
Pour l'ajouter manuellement ou la mettre à jour, relancer `oc install` ou copier
le bloc entre les délimiteurs `# >>> opencode providers switcher (ocp) >>>` / `# <<< opencode providers switcher (ocp) <<<`.

---

## `opencode.json`

Fichier de configuration OpenCode à la racine d'un projet cible.
Créé par `oc deploy` — **régénéré si une clé API est configurée, si `PROJECT_ID` est
défini (pour retirer un ancien bloc provider), ou si le fichier est absent** ; conservé tel quel sinon.

### Contenu sans clé API

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "agent": {
    "auditor-subagent": { "mode": "subagent" },
    "developer-backend": { "mode": "subagent" },
    "build": { "disable": true },
    "plan": { "disable": true }
  }
}
```

Le bloc `"agent":` liste :
- les agents dont le mode effectif est `subagent`
- les agents natifs OpenCode désactivés (`"disable": true`) — définis dans `hub.json → opencode.disabled_native_agents` et surchargeables par projet dans `projects.md` via `- Disable agents :`

Les agents `primary` non désactivés sont absents — OpenCode les considère visibles par défaut.
Si aucun agent n'a de configuration spéciale, le bloc `"agent":` est omis.

### Contenu avec clé Anthropic

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-opus-4-5",
  "provider": {
    "anthropic": {
      "apiKey": "sk-ant-..."
    }
  }
}
```

### Contenu avec litellm / proxy compatible OpenAI

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "provider": {
    "litellm": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "apiKey": "sk-bRf...",
        "baseURL": "https://api.mammouth.ai/v1"
      }
    }
  }
}
```

Le modèle est résolu par priorité :
1. `projects/api-keys.local.md` → clé `model` du projet (si `PROJECT_ID` défini)
2. Variable d'env `$OPENCODE_MODEL`
3. `config/hub.json` → clé `opencode.model`
4. Fallback : `claude-sonnet-4-5`

> Si une clé API est injectée, ce fichier **ne doit pas être commité** dans le projet cible
> (ajouté automatiquement au `.git/info/exclude` du projet par `oc deploy` — exclusion locale, invisible pour les autres devs).
> Sans clé API, le fichier **peut être commité**.

---

## `.gitignore` du hub

Fichiers et dossiers ignorés par git dans le hub lui-même :

```gitignore
config/hub.json             # si default_provider.api_key est définie (auto-ajouté)
projects/projects.md        # registre local des projets
projects/paths.local.md     # chemins locaux
projects/api-keys.local.md  # clés API par projet
.opencode/node_modules/     # dépendances OpenCode
.opencode/bun.lock
.opencode/package.json
skills/external/            # skills téléchargés via oc skills add
```

---

## Variables d'environnement

Le hub ne définit pas de variables d'environnement obligatoires.
Les credentials pour les trackers (Jira, GitLab) sont stockés localement
par `bd config set` — jamais dans des fichiers versionnés.

Les variables suivantes sont lues par les scripts du hub si elles sont présentes dans l'environnement :

| Variable | Défaut | Obligatoire | Description |
|----------|--------|-------------|-------------|
| `OPENCODE_HUB_DIR` | `~/.openhub` | non | Répertoire d'installation du hub. Utilisé par `install.sh` et `uninstall.sh` pour choisir l'emplacement d'installation. Exemple : `OPENCODE_HUB_DIR=~/tools/oc bash install.sh` |
| `OPENCODE_MODEL` | *(cascade hub.json)* | non | Modèle LLM à utiliser. Niveau 2 de la cascade de résolution du modèle (après la config projet `api-keys.local.md`, avant `hub.json`). Exemple : `OPENCODE_MODEL=claude-opus-4-5` |
| `AWS_BEARER_TOKEN_BEDROCK` | — | non | Token d'authentification AWS Bedrock. Injecté automatiquement par `oc start` depuis la config projet ou hub quand le provider effectif est `bedrock` — ne pas définir manuellement sauf cas avancé. |
| `HUB_DIR` | *(dossier du dépôt)* | non | Override du répertoire racine du hub en runtime. Auto-détecté depuis l'emplacement de `oc.sh` si absent. Utile pour les tests ou les configurations multi-hub. |
