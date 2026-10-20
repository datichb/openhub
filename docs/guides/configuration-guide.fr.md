> [Read in English](configuration-guide.en.md)

# Guide de configuration

Ce guide vous accompagne pas à pas dans la configuration d'OpenHub, depuis le setup minimal jusqu'à la configuration équipe avancée. Pour la référence exhaustive de chaque clé de configuration, voir [Référence configuration](../reference/config.fr.md).

**Ce que couvre ce guide :**

1. [Setup solo minimal](#1-setup-solo-minimal) -- Demarrez en 2 minutes
2. [Configuration du provider](#2-configuration-du-provider) -- Configurer votre fournisseur LLM
3. [Enregistrement de projets](#3-enregistrement-de-projets) -- Enregistrer et configurer des projets
4. [Serveurs MCP](#4-serveurs-mcp) -- Connecter des outils externes
5. [Configuration équipe](#5-configuration-équipe) -- Collaboration multi-utilisateurs
6. [Configuration avancée](#6-configuration-avancée) -- Personnaliser les workflows, fichiers d'instructions, worktrees

---

## 1. Setup Solo Minimal

Après avoir exécuté `oh init`, votre configuration hub vit dans `~/.oh/hub.toml`. La configuration minimale viable ne requiert que 3 paramètres :

```toml
# ~/.oh/hub.toml -- configuration minimale
[cli]
language = "fr"              # "en" ou "fr"

[llm]
default_provider = "anthropic"   # votre fournisseur LLM
```

C'est tout. Tout le reste a des valeurs par défaut sensibles. Les credentials du provider sont stockés dans votre trousseau OS (pas dans ce fichier).

### Emplacements des fichiers

| Chemin | Usage |
|--------|-------|
| `~/.oh/hub.toml` | Configuration du hub |
| `~/.oh/oh.db` | Registre de projets (SQLite) |
| `~/.oh/hub/` | Agents et skills embarqués |
| `~/.oh/secrets.enc` | Secrets chiffrés (fallback trousseau) |
| `~/.oh/bundles/<hash>/` | Paquets de session (agents, skills, config) construits au lancement — plus rien n'est déployé dans `<projet>/.opencode/` |

---

## 2. Configuration du Provider

### Choisir un provider

```mermaid
flowchart TD
    Start([Choisir un provider]) --> Q1{Compte AWS<br/>avec Bedrock ?}
    Q1 -->|Oui| Bedrock[Amazon Bedrock]
    Q1 -->|Non| Q2{Cle API<br/>Anthropic ?}
    Q2 -->|Oui| Anthropic[Anthropic Direct]
    Q2 -->|Non| Q3{Acces<br/>multi-modeles ?}
    Q3 -->|Oui| OpenRouter[OpenRouter]
    Q3 -->|Non| Copilot[GitHub Copilot]
```

### Configurer Anthropic (le plus simple)

```bash
oh provider setup
# Selectionnez "Anthropic (API directe)"
# Entrez votre cle API depuis console.anthropic.com
```

La clé est stockée dans votre trousseau OS. Pour vérifier :

```bash
oh doctor
```

Paramètre hub.toml :

```toml
[llm]
default_provider = "anthropic"
```

### Configurer Amazon Bedrock

Bedrock supporte deux modes d'authentification :

**Bearer token (API Gateway / LiteLLM) :**

```bash
oh provider setup
# Selectionnez "Amazon Bedrock"
# Selectionnez "Bearer token"
# Entrez votre token
```

```toml
[llm]
default_provider = "bedrock"

[provider.bedrock]
auth_mode = "bearer"
```

**Profil AWS (Bedrock natif) :**

```bash
oh provider setup
# Selectionnez "Amazon Bedrock"
# Selectionnez "Profil AWS"
# Entrez le nom du profil et la region
```

```toml
[llm]
default_provider = "bedrock"

[provider.bedrock]
auth_mode = "profile"
aws_profile = "my-profile"
aws_region = "us-east-1"
```

### Configurer OpenRouter

```bash
oh provider setup
# Selectionnez "OpenRouter"
# Entrez votre cle API depuis openrouter.ai
```

```toml
[llm]
default_provider = "openrouter"
```

### Configurer GitHub Copilot

```bash
oh provider setup
# Selectionnez "GitHub Copilot"
# Suivez le flux OAuth
```

```toml
[llm]
default_provider = "github-copilot"
```

### Override provider par projet

Un projet peut utiliser un provider différent du défaut du hub :

```bash
oh project configure --provider bedrock
```

### Configuration des modèles

```bash
# Definir le modele par defaut pour tous les agents
oh config model default anthropic/claude-sonnet-4-6

# Definir un modele pour une famille d'agents
oh config model family planning anthropic/claude-opus-4-6

# Definir un modele pour un agent specifique
oh config model agent planner anthropic/claude-opus-4-6

# Voir la configuration modele actuelle
oh config model show
```

Dans `hub.toml` :

```toml
[models]
default = "claude-sonnet-4-6"

[models.families]
planning = "claude-opus-4-6"

[models.agents]
reviewer = "claude-opus-4-6"
```

La résolution de modèle suit une **cascade à 10 niveaux** (premier match gagne), precedee du bloc `models:` du workflow de la session :

```
Workflow agent > Workflow defaut >
Projet agent > Projet famille > Projet defaut >
Hub agent > Hub famille > Hub defaut >
Team agent > Team famille > Team defaut >
Plancher frontmatter agent
```

Voir [Référence résolution de modèles](../reference/model-resolution.fr.md) pour les détails.

---

## 3. Enregistrement de Projets

### Ajouter un projet

```bash
oh project add
```

L'assistant interactif demande : nom, chemin, langage.

Non-interactif :

```bash
oh project add --name my-app --path ~/workspace/my-app --language typescript
```

### Lister les projets

```bash
oh project list
```

### Paquet de session d'un projet

Plus rien n'est déployé dans le projet (`oh deploy` supprimé en v5) : chaque session démarre d'un paquet construit au lancement à partir de son workflow.

```bash
cd ~/workspace/my-app
oh bundle show <workflow>                  # projet detecte depuis le repertoire courant
oh bundle show <workflow> -p my-project    # projet explicite
oh bundle show <workflow> --budget         # budget de contexte par agent/skill
oh bundle build <workflow>                 # construire sans lancer
```

```mermaid
flowchart LR
    W[Workflow] --> A[Agents<br/>skills Bucket A integrees]
    H[Hub ~/.oh/] --> A
    H --> S[Skills a la demande]
    H --> P[Permissions]
    H --> M[Modeles<br/>resolus par agent]
    H --> MCP[MCP<br/>config serveurs]
    A --> D[~/.oh/bundles/hash/]
    S --> D
    P --> D
    M --> D
    MCP --> D
```

### Paramètres par projet

```bash
oh project configure --provider bedrock       # override provider
oh project configure --model claude-opus-4-6  # override modele
oh project configure --language fr            # override langue
```

---

## 4. Serveurs MCP

OpenHub inclut 7 serveurs MCP intégrés pour l'intégration d'outils externes :

| Serveur | Usage | Token requis |
|---------|-------|-------------|
| **GitLab** | Issues, MR, pipelines | PAT GitLab |
| **GitHub** | Issues, PR, Actions | PAT GitHub |
| **Figma** | Fichiers design, composants | PAT Figma |
| **Jira** | Issues, projets, transitions | Token API Jira |
| **Linear** | Issues via GraphQL | Clé API Linear |
| **Google Slides** | Analyse de présentations | OAuth Google |
| **Team** | État équipe, claims, wiki | (pas de token -- données locales) |

### Configurer les serveurs MCP

```bash
oh mcp setup                 # assistant interactif
oh mcp list                  # lister tous les serveurs et leur statut
oh mcp enable gitlab         # activer un serveur
oh mcp disable figma         # desactiver un serveur
oh mcp status                # statut detaille de tous les serveurs
```

Dans `hub.toml` :

```toml
[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true
url = "https://gitlab.mycompany.com"

[mcp.figma]
enabled = true
token_key = "openhub.mcp.figma.token"

[mcp.jira]
enabled = false
```

### Override MCP par projet

Les serveurs MCP peuvent être activés/désactivés par projet (`oh mcp enable|setup`). À chaque lancement, les paramètres hub combinés aux overrides projet sont placés dans le paquet de session — aucun redéploiement nécessaire.

---

## 5. Configuration Équipe

Les fonctionnalités équipe permettent à plusieurs développeurs de collaborer via un dépôt Git partage ("team-state repo") qui synchronise les claims, policies, wiki et événements.

> **Utilisateur solo ?** Passez cette section entièrement. Les fonctionnalités équipe sont optionnelles et n'affectent pas l'usage solo.

### Configuration initiale

**Étape 1 -- Créer un dépôt team-state :**

Créez un nouveau dépôt Git (sur n'importe quel hébergeur Git) pour contenir l'état équipe.

**Étape 2 -- Initialiser les fonctionnalités équipe :**

```bash
oh team init
```

L'assistant vous guide à travers :
1. **Connexion au dépôt** -- URL de votre dépôt team-state
2. **Configuration globale** -- Paramètres par défaut pour tous les membres
3. **Identité** -- Votre member ID (utilisé pour les claims)
4. **Notifications** -- Destinations webhook (Slack, Discord, Teams, Mattermost)
5. **Policies** -- Règles de revue de code, nommage de branches, conventions de commits

Dans `hub.toml` :

```toml
[[teams]]
id = "my-team"
name = "Mon Equipe"
enabled = true
state_repo = "git@github.com:myorg/team-state.git"
state_path = "~/.oh/team-state/my-team"
member_id = "alice"
```

### Support multi-equipe

Vous pouvez appartenir à plusieurs équipes. Chaque équipe a son propre dépôt :

```bash
oh teams list                # lister toutes les equipes
oh teams add --repo git@... --member-id alice
oh teams remove <team-id>
```

### Policies d'équipe

Les policies définissent des règles applicables à l'équipe :

```bash
oh policies list             # afficher les policies actives
oh policies check            # valider le projet courant contre les policies
oh policies add              # ajouter une nouvelle policy
```

### Conventions d'équipe

```bash
oh conventions check         # valider le projet contre les conventions d'equipe
```

### Notifications

Les notifications sont configurées dans le fichier de configuration team-state (pas dans `hub.toml`) :

```toml
# team-state config.toml
[notification]
enabled = true

[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/..."
channel = "#dev-ai"
bot_name = "openhub"
```

Plateformes supportées : Slack, Discord, Microsoft Teams, Mattermost.

### Claims et collaboration

```bash
oh team claim <ticket-id>                    # revendiquer un ticket
oh team release <ticket-id>                  # liberer un claim
oh team claim transfer <ticket-id> --to bob  # transferer un claim
oh team status                               # afficher l'etat de l'equipe
oh team activity --today                     # activite du jour
oh team board                                # kanban d'equipe
```

### Takeover briefs

Lors de la reprise du travail de quelqu'un d'autre :

```bash
oh takeover-brief show <ticket-id>   # voir le contexte de reprise
oh takeover-brief list               # lister les briefs disponibles
oh run brief-enrich --headless -i ticket=<ticket-id>   # enrichir avec l'analyse du code
```

`oh takeover-brief enrich <ticket-id>` reste un alias déprécié de `oh run brief-enrich --headless`.

### Bibliothèque de patterns

Partager des patterns réutilisables au sein de l'équipe :

```bash
oh patterns list             # lister les patterns d'equipe
oh patterns show <name>      # voir un pattern
oh patterns add              # proposer un nouveau pattern
oh patterns validate         # valider les patterns
```

---

## 6. Configuration Avancée

### Personnaliser les workflows

Le bloc `[workflow]` de `hub.toml` n'existe plus en v5 : il est migré automatiquement vers un workflow `implement-spec` du team-state (voir [Workflows d'équipe › Migration des anciennes surcharges](team-workflows.fr.md#migration-des-anciennes-surcharges-de-workflow-v5)). Pour adapter un workflow (checkpoints, mode, modèles, limites), créez une couche d'équipe ou de projet :

```bash
oh workflow list              # workflows disponibles (toutes les couches)
oh workflow show feature      # workflow resolu, avec l'origine des valeurs
oh workflow edit feature      # brouillon, puis oh workflow publish
```

Voir [Workflows d'équipe](team-workflows.fr.md) et la [Référence des workflows](../reference/workflows.fr.md).

### Fichiers d'instructions

`disable_native_agents` a été supprimé en v5 (les agents d'une session sont ceux de son workflow). Seuls les fichiers d'instructions supplémentaires restent, intégrées à chaque agent des paquets de session :

```toml
[deploy]
instruction_files = ["docs/ARCHITECTURE.md"]
```

### Configuration worktree

```toml
[worktree]
auto_cleanup = true          # oh gc supprime periodiquement les worktrees crees par oh (propres, pousses, inutilises)
base_branch = "main"         # branche de base pour les nouveaux worktrees
branch_pattern = "oh/%s"     # pattern de nommage de branche (%s = nom worktree)
```

### Intégration tracker

Configurer la synchronisation tracker externe pour les tickets Beads :

```toml
[tracker]
enabled = true
auto_sync = true             # synchroniser en fin de session
push_labels = true           # pousser les labels vers le tracker externe
```

### Paramètres OpenCode

```toml
[llm]
default_provider = "bedrock"   # opencode V2 est installé avec son propre outil (v5)
```

### Gestion des secrets

```bash
oh secrets set <key>           # stocker un secret dans le trousseau (valeur demandee)
oh secrets get <key>           # recuperer un secret
oh secrets list                # lister les cles de secrets stockes
oh secrets delete <key>        # supprimer un secret
```

### Plugins

`oh plugin` (plugins globaux d'opencode V1, dont RTK) est supprimé en v5. Les plugins se déclarent par workflow (`plugins:`) ; voir [Workflows livrés › Plugins et code mode](../reference/workflows.fr.md#plugins-et-code-mode).

---

## Precedence de Configuration

Quand plusieurs sources définissent le même paramètre, la résolution suit cet ordre (priorité la plus haute en premier) :

```mermaid
flowchart TD
    A[Flag CLI] --> B[Override projet]
    B --> C[Hub hub.toml]
    C --> D[Policy team ENFORCED]
    D --> E[Defaut team RECOMMENDED]
    E --> F[Defaut systeme]
    style A fill:#e74c3c,color:white
    style F fill:#95a5a6,color:white
```

Les policies **ENFORCED** de l'équipe overrident tous les paramètres locaux. Les policies **RECOMMENDED** de l'équipe servent de defauts qui peuvent être overrides localement.

---

**Suite :** Explorez les [Workflows](workflows.fr.md) pour des scénarios d'usage réels, ou consultez la [Référence configuration](../reference/config.fr.md) pour la liste exhaustive de toutes les clés.
