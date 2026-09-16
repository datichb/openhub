> [Read in English](tutorial.en.md)

# Tutoriel -- Vos 5 premieres minutes avec OpenHub

Ce tutoriel pratique vous guide pas a pas pour installer OpenHub, configurer votre premier projet et lancer votre premiere session assistee par IA. Chaque etape inclut la sortie terminal attendue pour que vous puissiez verifier que tout fonctionne.

**Ce que vous saurez faire apres ce tutoriel :**
- Installer le CLI `oh`
- Configurer un fournisseur LLM (Anthropic ou Bedrock)
- Enregistrer un projet
- Deployer les agents et skills
- Lancer votre premiere session IA

**Temps necessaire :** ~5 minutes.

> **Prerequis :** Vous avez besoin de `git` installe et d'une cle API pour au moins un fournisseur LLM. Voir le [guide de choix de provider](#quel-provider-choisir) ci-dessous.

---

## Quel provider choisir ?

```mermaid
flowchart TD
    Start([Je dois choisir un provider]) --> Q1{Avez-vous un<br/>compte AWS avec<br/>acces Bedrock ?}
    Q1 -->|Oui| Bedrock[Amazon Bedrock<br/>Enterprise, facturation a l'usage]
    Q1 -->|Non| Q2{Avez-vous une<br/>cle API Anthropic ?}
    Q2 -->|Oui| Anthropic[Anthropic Direct<br/>Le plus simple, paiement par token]
    Q2 -->|Non| Q3{Voulez-vous un<br/>acces multi-modeles ?}
    Q3 -->|Oui| OpenRouter[OpenRouter<br/>Plusieurs modeles, une seule cle API]
    Q3 -->|No| Copilot[GitHub Copilot<br/>Utiliser votre abonnement Copilot existant]
```

| Provider | Complexite setup | Ce qu'il vous faut avant de commencer |
|----------|-----------------|---------------------------------------|
| **Anthropic** | Faible | Une cle API depuis [console.anthropic.com](https://console.anthropic.com) |
| **Amazon Bedrock** | Moyenne | Un compte AWS avec l'acces aux modeles Bedrock active |
| **OpenRouter** | Faible | Une cle API depuis [openrouter.ai](https://openrouter.ai) |
| **GitHub Copilot** | Faible | Un abonnement GitHub Copilot actif |

> **Recommandation pour les nouveaux utilisateurs :** Commencez avec **Anthropic** (le plus simple). Vous pourrez changer de provider plus tard avec `oh provider setup`.

---

## Etape 1 -- Installer OpenHub

**macOS/Linux (Homebrew) :**

```bash
brew install datichb/tap/openhub
```

**macOS/Linux (curl) :**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh
```

**Depuis les sources (necessite Go 1.22+) :**

```bash
cd cli && go install .
```

Verifier l'installation :

```bash
oh version
```

Sortie attendue :

```
oh v4.0.1 (go1.26.4, darwin/arm64)
```

---

## Etape 2 -- Initialiser le Hub

```bash
oh init
```

L'assistant interactif vous guide en 3 phases. Voici ce qui vous attend :

### [1/3] Configuration du hub

```
Bienvenue dans OpenHub !

Cet assistant va configurer votre hub central de developpement assiste par IA.
Vous aurez besoin de :
  - Une cle API pour votre fournisseur LLM (Anthropic, Bedrock, OpenRouter ou GitHub Copilot)
  - (Optionnel) Des tokens pour les integrations MCP (Figma, GitLab, Google Slides)

? Choisissez votre langue :
    English
  > Francais

? Version d'OpenCode a installer : (latest)

? Fournisseur LLM par defaut :
    Amazon Bedrock
  > Anthropic (API directe)
    OpenRouter
    GitHub Copilot
```

**Si vous avez choisi Anthropic :**

```
? Cle API Anthropic : sk-ant-••••••••
  Cle API stockee dans le trousseau systeme.
```

**Si vous avez choisi Amazon Bedrock (alternative) :**

```
? Mode d'authentification :
  > Bearer token (API Gateway / LiteLLM)
    Profil AWS (Bedrock natif)

? Bearer token : ••••••••
  Token stocke dans le trousseau systeme.
```

### [2/3] Serveurs MCP (optionnel)

```
? Configurer les integrations MCP ?
  [x] GitLab
  [ ] Figma
  [ ] Google Slides

? Token d'acces personnel GitLab : glpat-••••••••
  Token stocke dans le trousseau systeme.
  Activer le mode ecriture (creer des MR, ajouter des notes) ? (o/N)
```

> Passez cette etape si vous n'avez pas encore de tokens. Vous pourrez les configurer plus tard avec `oh mcp setup`.

### [3/3] Premier projet (optionnel)

```
? Enregistrer un projet maintenant ? (O/n) O
? Nom du projet : my-app
? Chemin du projet : ~/workspace/my-app
? Langage principal : typescript

  Projet 'my-app' enregistre.
```

```
Hub initialise dans ~/.oh/
  hub.toml ........... configuration
  oh.db .............. registre de projets
  hub/ ............... agents & skills (extraits du binaire)

Lancez 'oh deploy' dans votre repertoire de projet pour commencer.
```

---

## Etape 3 -- Deployer les agents et skills

Naviguez vers votre projet et deployer :

```bash
cd ~/workspace/my-app
oh deploy
```

Sortie attendue :

```
Deploiement vers my-app...

  Phase 1/5  Agents .............. 19 agents deployes
  Phase 2/5  Skills .............. 47 skills assembles
  Phase 3/5  Configuration ....... opencode.json ecrit
  Phase 4/5  Config agents ....... modeles resolus (sonnet-4-6)
  Phase 5/5  Serveurs MCP ........ 2 serveurs configures

Deploye dans .opencode/ (0.3s)
```

Cela cree un repertoire `.opencode/` dans votre projet contenant les definitions d'agents, les protocoles de skills et la configuration.

---

## Etape 4 -- Lancer votre premiere session

```bash
oh start
```

Sortie attendue :

```
  Projet       my-app
  Chemin       ~/workspace/my-app
  Provider     anthropic
  Modele       claude-sonnet-4-6
  Agent        orchestrator
  Stack        typescript, react, next.js
  MCP          gitlab

Appuyez sur Entree pour demarrer (ou Ctrl+C pour annuler)...
```

Appuyez sur Entree. OpenCode se lance avec l'agent configure et vous pouvez commencer a travailler :

```
> Explique l'architecture de ce projet et suggere des ameliorations.
```

L'orchestrateur analysera votre codebase, deleguera potentiellement a des agents specialises (planner, designer, developer), et fournira des resultats structures.

---

## Etape 5 -- Explorer la suite

Vous avez maintenant un setup OpenHub fonctionnel. Voici vos prochaines etapes :

| Ce que vous voulez faire | Commande | Guide |
|--------------------------|----------|-------|
| Comprendre tous les agents et leur fonctionnement | - | [Vue d'ensemble architecture](../architecture/overview.fr.md) |
| Configurer les parametres avances | `oh config list` | [Guide de configuration](configuration-guide.fr.md) |
| Executer un workflow feature complet | `oh start -a orchestrator` | [Workflows](workflows.fr.md) |
| Auditer votre code (securite/perf) | `oh audit --type security` | [Workflows](workflows.fr.md#scenario-2) |
| Configurer la collaboration d'equipe | `oh team init` | [Configuration equipe](team-setup.fr.md) |
| Explorer le tableau de bord TUI | `oh` (sans arguments) | [Usage TUI](tui-usage.fr.md) |
| Chercher la definition d'un terme | - | [Glossaire](../reference/glossary.fr.md) |

---

## Reference rapide

```
oh init                 # assistant de configuration initiale
oh deploy               # deployer agents/skills dans le projet courant
oh start                # lancer une session IA (lancement rapide)
oh start --recap        # lancer une session IA (avec récap + confirmation)
oh start --dev          # mode dev : choisir des tickets a implementer
oh start --onboard      # decouvrir et documenter un codebase
oh doctor               # diagnostiquer les problemes
oh status               # afficher l'etat du hub et du projet
```

---

**Suite :** Lisez le [guide de demarrage](getting-started.fr.md) pour la reference complete des commandes, ou plongez dans les [Workflows](workflows.fr.md) pour des scenarios reels.
