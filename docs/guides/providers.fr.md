> 🇬🇧 [Read in English](providers.en.md)

# Support multi-fournisseurs LLM

OpenCode Hub supporte plusieurs fournisseurs LLM, vous permettant de choisir la meilleure solution selon vos besoins. Ce guide explique comment configurer et utiliser les différents fournisseurs.

## Vue d'ensemble

### Fournisseurs supportés

| Fournisseur | Type | Cibles | Credential | URL de base par défaut |
|-------------|------|--------|------------|------------------------|
| **Anthropic** | Natif | OpenCode, OpenCode | Clé API | N/A |
| **MammouthAI** | OpenAI-compatible (litellm) | OpenCode | Clé API | `https://api.mammouth.ai/v1` |
| **GitHub Models** | OpenAI-compatible (litellm) | OpenCode | Clé API | `https://models.inference.ai.azure.com` |
| **AWS Bedrock** | Natif (`amazon-bedrock`) | OpenCode | Bearer token | N/A |
| **Ollama** | OpenAI-compatible (litellm) | OpenCode | Optionnel | `http://localhost:11434/v1` |
| **GitHub Copilot** | Natif (`github-copilot`) | OpenCode | OAuth (aucune clé API) | N/A |

### Notes importantes

- **Limitation OpenCode** : OpenCode ne supporte que le fournisseur `anthropic` (contrainte architecturale). L'utilisation d'autres fournisseurs déclenchera un avertissement.
- **Priorité des modèles** : Les modèles sont résolus dans cet ordre : 1) Config projet → 2) Hub par défaut → 3) Variable d'env → 4) Hub opencode.model → 5) Fallback par défaut

## Niveaux de configuration

OpenCode Hub supporte la configuration du provider à deux niveaux :

### 1. Niveau Hub (par défaut pour tous les projets)

Définir un provider appliqué à tous les projets par défaut :

```bash
./oc.sh provider set-default
```

Vous serez invité à :
- Sélectionner un provider (1-5 ou ignorer)
- Fournir les credentials API (si requis)
- Saisir optionnellement une URL de base personnalisée

La configuration est stockée dans `config/hub.json` dans le bloc `default_provider` :

```json
{
  "default_provider": {
    "name": "mammouth",
    "api_key": "sk-xxx...",
    "base_url": "https://api.mammouth.ai/v1",
    "model": ""
  }
}
```

**Note** : Si une clé API est configurée, `config/hub.json` est automatiquement ajouté au `.gitignore`.

### 2. Niveau Projet (surcharge par projet)

Configurer un provider différent pour un projet spécifique :

```bash
./oc.sh init MY-PROJECT
# ou
./oc.sh config set MY-PROJECT
```

Lors de `oc init`, vous serez invité à configurer un provider optionnel au niveau projet (étape 4).

Lors de `oc config set`, vous pouvez spécifier `--provider` et les flags associés :

```bash
./oc.sh config set MY-PROJECT --provider github-models --api-key sk-xxx
```

La configuration au niveau projet est stockée dans `projects/api-keys.local.md` (non commité dans git) :

```
[MY-PROJECT]
provider=github-models
api_key=sk-xxx...
base_url=https://models.inference.ai.azure.com
model=claude-opus
```

## Référence des commandes

### `oc provider list`

Affiche tous les providers disponibles avec leur statut (par défaut, configuré, cibles supportées) :

```bash
./oc.sh provider list
```

Exemple de sortie :
```
Fournisseurs LLM disponibles

Anthropic (direct) ◆ (hub default)
  API Anthropic directe pour Claude models
  Cibles: ["opencode", "opencode"]

MammouthAI
  Proxy OpenAI-compatible vers Claude (FR-hosted)
  Cibles: ["opencode"]
  Base URL: https://api.mammouth.ai/v1

...
```

### `oc provider set-default`

Configure interactivement le fournisseur par défaut du hub :

```bash
./oc.sh provider set-default
```

Vous serez invité à :
1. Sélectionner un fournisseur
2. Saisir les credentials (saisie masquée pour la sécurité)
3. Optionnellement saisir une URL de base personnalisée

La configuration est écrite dans `config/hub.json` **et `opencode.json` est régénéré immédiatement** — pas besoin de lancer `oc deploy` manuellement.

> **Note :** La configuration du provider par projet se gère via `oc config set <PROJECT_ID>` — voir `./oc.sh config set --help` ou la [référence de configuration](../reference/config.fr.md).

## Guides de configuration par fournisseur

### Anthropic (par défaut)

**Cibles supportées** : OpenCode, OpenCode

1. Obtenez votre clé API depuis [console.anthropic.com](https://console.anthropic.com)
2. Lancez `./oc.sh provider set-default` ou `./oc.sh config set <PROJECT_ID>`
3. Choisissez "Anthropic" et entrez votre clé API

### MammouthAI

**Cibles supportées** : OpenCode

MammouthAI est un proxy OpenAI-compatible hébergé en France, compatible avec les modèles Anthropic.

1. Obtenez votre clé API depuis [mammouth.ai](https://mammouth.ai)
2. Lancez `./oc.sh provider set-default`
3. Choisissez "MammouthAI" (option 2)
4. Entrez votre clé API (l'URL de base par défaut sera utilisée : `https://api.mammouth.ai/v1`)

```bash
# Ou via config :
./oc.sh config set MY-PROJECT --provider mammouth --api-key sk-xxx
```

### GitHub Models

**Cibles supportées** : OpenCode

GitHub Models fournit un accès à divers modèles via l'API GitHub/Copilot.

1. Obtenez votre token depuis [github.com/settings/tokens](https://github.com/settings/tokens)
2. Lancez `./oc.sh provider set-default`
3. Choisissez "GitHub Models" (option 3)
4. Entrez votre token GitHub
5. Surchargez optionnellement l'URL de base (par défaut : `https://models.inference.ai.azure.com`)

```bash
# Ou via config :
./oc.sh config set MY-PROJECT \
  --provider github-models \
  --api-key ghp_xxx \
  --base-url https://models.inference.ai.azure.com
```

### AWS Bedrock

**Cibles supportées** : OpenCode

AWS Bedrock utilise le **provider natif `amazon-bedrock`** intégré à OpenCode. Il requiert un **bearer token Bedrock** (clé à long terme générée depuis la console Amazon Bedrock).

**Fonctionnement :**
- Le bearer token est stocké dans `config/hub.json` (jamais dans `opencode.json`)
- `opencode.json` est généré avec un bloc `amazon-bedrock` vide
- Au lancement via `oc start`, le token est injecté automatiquement comme `AWS_BEARER_TOKEN_BEDROCK`

1. Générez un bearer token depuis la [console Amazon Bedrock](https://console.aws.amazon.com/bedrock/) sous **API Keys**
2. Demandez l'accès aux modèles dans le **Model catalog**
3. Lancez `./oc.sh provider set-default`
4. Choisissez "AWS Bedrock (natif)" et entrez votre bearer token

Le `opencode.json` généré ressemblera à :
```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-5",
  "provider": {
    "amazon-bedrock": {}
  }
}
```

Au lancement, `oc start` injecte :
```bash
AWS_BEARER_TOKEN_BEDROCK=<token> opencode
```

```bash
# Ou configurer par projet :
./oc.sh config set MON-PROJET --provider bedrock --api-key <bearer-token>
```

### Ollama (Local)

**Cibles supportées** : OpenCode

Ollama vous permet d'exécuter des LLM localement.

1. Installez Ollama depuis [ollama.ai](https://ollama.ai)
2. Démarrez le serveur Ollama : `ollama serve`
3. Lancez `./oc.sh provider set-default`
4. Choisissez "Ollama" (option 5)
5. L'URL de base par défaut (`http://localhost:11434/v1`) sera utilisée

```bash
# Ou via config :
./oc.sh config set MY-PROJECT \
  --provider ollama \
  --base-url http://localhost:11434/v1
```

Note : Ollama ne requiert pas de clé API, mais une peut être définie pour des couches d'authentification personnalisées.

### GitHub Copilot

**Cibles supportées** : OpenCode

GitHub Copilot utilise une **authentification OAuth** — aucune clé API n'est requise. L'authentification se fait via `opencode auth`, qui ouvre un flux OAuth avec GitHub directement.

**Prérequis :** disposer d'un abonnement GitHub Copilot actif.

**Fonctionnement :**
- Pas de clé API à gérer — le token OAuth est géré par OpenCode
- Le `opencode_prefix` est `github-copilot` dans la configuration générée
- `config/hub.json` n'a pas besoin de champ `api_key` pour ce provider

**Configuration :**

1. Authentifiez-vous via `opencode auth` (une seule fois, peut être fait avant ou après la configuration du hub) :

```bash
opencode auth
# Suit le flux OAuth GitHub — ouvre un navigateur pour autoriser
```

2. Configurez GitHub Copilot comme provider par défaut du hub :

```bash
./oc.sh provider set-default
# → Choisissez "GitHub Copilot"
```

3. Ou configurez-le pour un projet spécifique :

```bash
./oc.sh config set MON-PROJET --provider github-copilot
# Pas de --api-key requis
```

Le `opencode.json` généré ressemblera à :

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "github-copilot/claude-sonnet-4.5",
  "provider": {
    "github-copilot": {}
  }
}
```

> **Note technique :** Le hub accepte à la fois `claude-sonnet-4-5` (format interne) et `claude-sonnet-4.5` (format API GitHub Copilot). Les `model_aliases` du provider effectuent la transformation automatiquement.

**Modèles disponibles via GitHub Copilot :**

| Nom interne (hub) | Nom API GitHub Copilot |
|-------------------|------------------------|
| `claude-sonnet-4-5` | `claude-sonnet-4.5` |
| `claude-sonnet-4-5-v2` | `claude-sonnet-4.5-v2` |
| `claude-opus-4` | `claude-opus-4` |
| `claude-haiku-3-5` | `claude-haiku-3.5` |

> Ces noms de modèles sont automatiquement transformés depuis le format interne du hub (ex. `claude-sonnet-4-5`) grâce aux `model_aliases`.

**Remarque :** contrairement aux autres providers, `config/hub.json` ne contient pas de champ `api_key` pour GitHub Copilot. Le fichier `hub.json` est toujours gitignored par sécurité.

## Flux de travail

### Utiliser différents providers pour différents projets

```bash
# Définir Anthropic comme provider par défaut du hub
./oc.sh provider set-default
# → Choisir Anthropic

# Surcharger un projet spécifique avec GitHub Models
./oc.sh config set MY-PYTHON-PROJECT --provider github-models --api-key ghp_xxx

# Un autre projet utilise MammouthAI
./oc.sh config set MY-JS-PROJECT --provider mammouth --api-key sk-xxx
```

### Changer de provider

Pour modifier une configuration de provider :

```bash
# Pour le provider par défaut du hub :
./oc.sh provider set-default

# Pour un projet :
./oc.sh config set MY-PROJECT
# → Suivre les invites pour mettre à jour le provider/clé/modèle
```

### Utiliser Ollama en local pour le développement

```bash
# Démarrer Ollama (dans un terminal séparé) :
ollama serve

# Configurer votre projet pour utiliser Ollama :
./oc.sh config set MY-PROJECT --provider ollama

# Déployer et démarrer :
./oc.sh deploy all MY-PROJECT
./oc.sh start MY-PROJECT
```

## Sécurité

- **Clés API** : Toutes les clés API sont stockées dans des fichiers locaux (ajoutés au `.gitignore`) et ne sont jamais commitées dans git.
- **Masquage** : Lors de la consultation des configurations, les clés API sont masquées pour n'afficher que les 8 premiers caractères.
- **Spécifique à l'environnement** : Chaque environnement peut avoir des configurations de provider différentes.

### Fichiers avec secrets

Les fichiers suivants contiennent des credentials et ne sont **jamais commités dans git** :

| Fichier | Pourquoi gitignored |
|---------|---------------------|
| `config/hub.json` | Contient l'`api_key` / bearer token — toujours gitignored |
| `opencode.json` | Généré par `adapter_deploy`, reflète la config provider locale — toujours gitignored |
| `projects/api-keys.local.md` | Clés API par projet — toujours gitignored par conception |

Un template sans secret est commité dans `config/hub.json.example`. Au premier lancement (ou après un clone), `hub.json` est créé automatiquement depuis ce template s'il n'existe pas.

```bash
# Après un clone, lancez cette commande pour configurer votre provider :
./oc.sh provider set-default
```

## Dépannage

### "Provider not supported"

Si vous voyez cette erreur, assurez-vous d'utiliser l'un des providers supportés :
- `anthropic`
- `mammouth`
- `github-models`
- `bedrock`
- `ollama`
- `github-copilot`

### OpenCode affiche un avertissement "provider not supported"

C'est un comportement attendu. OpenCode ne supporte que Anthropic. Si vous avez besoin d'utiliser OpenCode :
1. Configurez une clé API Anthropic au niveau hub, ou
2. Surchargez votre projet pour utiliser le provider `anthropic`

### Modèle introuvable / Erreurs API

1. Vérifiez que votre clé API est correcte : `./oc.sh config get <PROJECT_ID>`
2. Vérifiez que l'URL de base est correcte pour votre provider
3. Assurez-vous que le service provider est en cours d'exécution (notamment pour Ollama)
4. Testez votre clé API directement avec la CLI ou l'API du provider

### Les changements de provider ne sont pas appliqués

Après `oc provider set-default`, `opencode.json` est automatiquement régénéré — aucune étape manuelle nécessaire.

Pour les changements au niveau projet (`oc config set`), redéployez :

```bash
./oc.sh deploy all MON-PROJET
```

## Commandes associées

- `./oc.sh config set` — Gérer la configuration du provider et du modèle au niveau projet
- `./oc.sh config get` — Afficher la configuration effective d'un projet
- `./oc.sh deploy all` — Déployer les agents avec la configuration provider actuelle
- `./oc.sh start` — Démarrer OpenCode avec le provider configuré
- `./oc.sh init` — Initialiser un nouveau projet (inclut l'étape de configuration du provider)
