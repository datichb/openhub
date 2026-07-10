> [Read in English](getting-started.en.md)

# Demarrage rapide

Ce guide couvre l'installation, la configuration initiale et l'utilisation quotidienne du CLI `oh`.

## Prerequisites

| Outil | Usage | Requis |
|-------|-------|--------|
| **git** | Controle de version | Oui |
| **opencode** | Agent IA de code | Auto-telecharge par `oh init` / `oh start` |
| **bd** | Gestionnaire de tickets Beads | Non (pour le mode `--dev` et `oh board`) |

Aucun besoin de Node.js, jq, sqlite3, bun ou Python. Le binaire Go est autonome.

## Installation

**Homebrew (recommande) :**

```bash
brew install datichb/tap/openhub
```

**Script curl :**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

**Depuis les sources :**

```bash
cd cli && go install .
```

## Configuration initiale

```bash
oh init
```

Cet assistant interactif en 3 etapes va :

**[1/3] Configuration du hub :**
- Afficher un preambule avec les prerequis (provider, tokens MCP)
- Demander votre langue preferee (fr/en)
- Demander la version d'opencode (par defaut : latest)
- Choisir le provider LLM par defaut (Bedrock, Anthropic, OpenRouter, GitHub Copilot)
- Detecter automatiquement les credentials existantes et proposer de les utiliser ou d'en configurer de nouvelles

**[2/3] Serveurs MCP (optionnel) :**
- Proposer de configurer des services MCP (Figma, GitLab, Google Slides)
- Pour chaque service selectionne : demander le token et le stocker dans le keychain
- Les services sans token sont ignores (configurables plus tard via `oh mcp setup`)

**[3/3] Premier projet (optionnel) :**
- Proposer d'enregistrer un premier projet
- Si oui : lance l'assistant de projet (nom, chemin, langage, agents, MCP)
- Si non : l'initialisation est terminee (`oh project add` disponible plus tard)

Le hub content (agents et skills) est extrait automatiquement dans `~/.oh/hub/` depuis le binaire.

## Enregistrer un projet

Pour ajouter d'autres projets apres l'initialisation :

```bash
oh project add
```

Ou de maniere non-interactive :

```bash
oh project add --name my-app --path ~/workspace/my-app --language typescript --tracker github
```

## Deployer les agents et skills

Deployer les agents, skills et la configuration partagee dans un projet :

```bash
oh deploy                    # detection automatique du projet depuis le repertoire courant
oh deploy -p my-project      # projet explicite
oh deploy --check            # verifier si le deploiement est necessaire (code retour 1 si obsolete)
oh deploy --diff             # afficher les changements prevus
```

Cela genere :

- `.opencode/agents/*.md` — definitions des agents
- `.opencode/skills/*/SKILL.md` — protocoles de skills
- `opencode.json` — fournisseur, modele, MCP, permissions

## Lancer une session

```bash
oh start                     # detection auto du projet, affiche le recap, confirme puis lance
oh start -p my-project       # projet explicite
oh start -a orchestrator     # utiliser un agent specifique
oh start -m "explique..."    # avec un prompt initial
oh start --dev               # mode dev : choisir epics/tickets
oh start --onboard           # creer le wiki du projet
oh start -y                  # passer la confirmation
oh start -r <session-id>     # reprendre une session precedente
```

Le flux de demarrage :

1. Resout le projet (depuis le repertoire courant ou le flag `--project`)
2. Resout le fournisseur et le token d'authentification
3. Detecte la stack du projet (langage/framework)
4. Affiche un recap de configuration detaille
5. Attend la confirmation (Entree ou `--yes` pour passer)
6. Lance opencode

## Demarrage rapide (sans recap)

```bash
oh quick                     # detection auto du projet, lancement immediat
```

## Commandes quotidiennes

```bash
oh sync --all                # synchroniser agents/skills vers tous les projets
oh status                    # afficher le statut du hub et du projet courant
oh doctor                    # verification de sante du systeme (verifie aussi les credentials provider)
oh provider setup            # configurer les credentials provider
oh metrics                   # metriques d'utilisation et couts
oh dashboard                 # tableau de bord TUI interactif
oh board                     # kanban (necessite bd)
```

## Workflow de developpement

```bash
oh start --dev               # choisir epic/ticket, lance orchestrator-dev
oh start --dev --label bug   # filtrer les tickets par label
oh audit --type security     # audit de code
oh review                    # revue de code
oh debug --issue "crash on login"  # session de debogage
```

## Gestion des worktrees

```bash
oh start -w feature/login    # cree un worktree et lance dedans
oh worktree list             # lister les worktrees actifs
oh worktree cleanup          # supprimer les worktrees merges
```

## Configuration

```bash
oh config list               # afficher toute la configuration
oh config set opencode.default_provider anthropic
oh config language fr        # passer en francais
oh config websearch enable   # activer la recherche web pour les agents
```

## Mise a jour

```bash
brew upgrade openhub          # mettre a jour oh lui-meme
oh upgrade opencode          # mettre a jour le binaire opencode
oh upgrade opencode 1.18.0   # fixer une version specifique
```

## Desinstallation

```bash
brew uninstall openhub
rm -rf ~/.oh                 # supprimer la configuration et la base de donnees
```

## Depannage

Lancer les diagnostics :

```bash
oh doctor
```

Problemes courants :

- **opencode introuvable** — lancer `oh init` ou `oh upgrade opencode`
- **Credentials provider manquantes** — lancer `oh provider setup`
- **Erreurs de serveur MCP** — verifier les tokens avec `oh service setup --project <id>`
- **Projet non detecte** — s'assurer d'etre dans un repertoire de projet enregistre (`oh project list`)
