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
brew install datichb/openhub/oh
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

Cet assistant interactif va :

1. Demander votre langue preferee (fr/en)
2. Demander la version d'opencode a utiliser (par defaut : latest)
3. Creer la configuration `~/.oh/hub.toml`
4. S'assurer que le binaire opencode est installe (telecharge si absent)
5. Lancer l'assistant d'enregistrement de projet :
   - Nom du projet, chemin, langage, tracker
   - Configuration du fournisseur et du modele
   - Selection des agents (multi-selection parmi 18 agents)
   - Configuration des services MCP (Figma, GitLab, Google Slides)
   - Deploiement optionnel des agents/skills/config

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
oh deploy -j my-project      # projet explicite
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
oh start -j my-project       # projet explicite
oh start -a orchestrator     # utiliser un agent specifique
oh start -p "explique..."    # avec un prompt initial
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
oh doctor                    # verification de sante du systeme
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
brew upgrade oh              # mettre a jour oh lui-meme
oh upgrade opencode          # mettre a jour le binaire opencode
oh upgrade opencode 1.18.0   # fixer une version specifique
```

## Desinstallation

```bash
brew uninstall oh
rm -rf ~/.oh                 # supprimer la configuration et la base de donnees
```

## Depannage

Lancer les diagnostics :

```bash
oh doctor
```

Problemes courants :

- **opencode introuvable** — lancer `oh init` ou `oh upgrade opencode`
- **Erreurs de serveur MCP** — verifier les tokens avec `oh service setup`
- **Projet non detecte** — s'assurer d'etre dans un repertoire de projet enregistre (`oh project list`)
