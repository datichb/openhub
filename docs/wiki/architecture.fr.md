---
page: architecture
title: Vue d'ensemble de l'architecture
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/bundle/build.go
  - docs/architecture/system-overview.fr.md
last_updated: 2026-10-06
---

> [Read in English](architecture.en.md)

# Vue d'ensemble de l'architecture

## Conception du systeme

openhub (`oh`) est un binaire Go unique qui gere les assistants IA de codage a travers les projets. L'architecture comporte trois couches :

```
+---------------------------------------------+
|              TUI Shell (tview)               |
|  Hub Mode | Project Mode | Team Mode         |
+---------------------------------------------+
|              Commandes CLI (Cobra)           |
|  init | run | bundle | team | ...           |
+---------------------------------------------+
|              Services centraux               |
|  Config | Bundle | MCP | Sessions | Storage  |
+---------------------------------------------+
```

## Packages principaux

| Package | Responsabilite |
|---------|---------------|
| `cmd/` | Definitions des commandes Cobra (points d'entree CLI) |
| `internal/config/` | Configuration hub (`hub.toml`, TOML + Viper) |
| `internal/bricks/` | Lecture des briques du hub (agents, skills, permissions, cascade des modeles, skills de stack) |
| `internal/bundle/` | Construction des paquets de session (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/workflow/` | Workflows declaratifs `oh/v1` (lecture, couches, validation, graphe de delegation, prompt) |
| `internal/sessionspec/` | Modele d'une session independant de l'outil (paquet, emplacement, runtime, provider) |
| `internal/adapters/` | Contrat oh <-> outil agentique ; `adapters/opencodev2` : adaptateur opencode V2 |
| `internal/runsvc/` | Lancement des sessions v5 (groupes de serveurs, proxy, monde ferme, ouverture des clients) |
| `internal/daemon/` | Demon `ohd` (proxy d'identifiants, supervision des sessions, decisions, notifications) |
| `internal/credproxy/` | Proxy d'identifiants LLM (jeton par groupe, vraies cles gardees sur la machine) |
| `internal/runtime/` | Environnements d'execution (local, conteneur, distant) |
| `internal/gateway/` | Passerelles du demon (Beads, MCP) pour les serveurs hors machine |
| `internal/limits/` | Restrictions des sessions (I6 : sessions actives, budgets, memoire, modeles) |
| `internal/mcp/` | 7 serveurs MCP integres (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team) |
| `internal/teamstate/` | Gestion de l'etat equipe (claims, wiki, policies, evenements, workflows d'equipe) |
| `internal/tui/` | Shell TUI (base tview, 3 modes de navigation) |
| `internal/storage/` | Base SQLite, trousseau systeme, chiffrement fichier |
| `internal/deploycleanup/` | Nettoyage des restes des anciens deploiements (`oh migrate deploy-cleanup`) |
| `internal/i18n/` | Internationalisation (FR + EN, fichiers locale JSON) |

`internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep` et `platform.SessionPlatform` ont ete supprimes en v5 (remplaces par `internal/bricks`, `internal/bundle`, les adaptateurs et `internal/runsvc`).

## Contenu embarque

Les agents, skills et permissions sont compiles dans le binaire via `go:embed` (`internal/hubcontent/`). A chaque lancement, `internal/bundle` en construit, hors du projet, le paquet de session du workflow (`~/.oh/bundles/<hash>/` : agents avec skills Bucket A integrees, skills a la demande, permissions, MCP, plugin) ; l'adaptateur en produit la config opencode (`oh deploy` supprime en v5).

## Architecture MCP

Chaque serveur MCP s'execute comme un sous-processus lance par OpenCode. Les tokens sont transmis via des variables d'environnement (jamais persistes en clair). Les serveurs communiquent via stdin/stdout en utilisant le protocole MCP. Quand la session tourne hors de la machine (conteneur, distant), les serveurs MCP d'oh du paquet passent par la passerelle MCP HTTP du demon `ohd`.

## Workflows et sessions

Chaque session est lancee depuis un workflow declaratif (`oh run <workflow>` ou la fiche de lancement de la TUI) et se suit dans la vue **Sessions** ou avec `oh session …`. Voir [Workflows livres](../reference/workflows.fr.md).

| Usage | Point d'entree | Description |
|------|---------------|-------------|
| Interactif | `oh run [workflow]` | Une session (sans argument : workflow par defaut du projet) |
| Feature | `oh run feature` | Planification, tickets, implementation orchestree |
| Tickets | `oh run ticket --tickets a,b` | Une session par ticket (un worktree par session qui ecrit, un seul serveur) ; `--one-session` pour tout regrouper |
| Sweep | `oh run sweep -i goal=<objectif>` | Decoupage d'un objectif en sous-taches, lancees en parallele dans la meme session, puis verification |
| Libre | `oh run libre --agent <id>` | Agent d'entree au choix, sans checkpoint |
| Headless | `oh run <workflow> --headless` | Non-interactif (CI/scripting) |
| Review | `oh run review` | Revue de code IA ; publication MR avec `oh review --publish` |

Les anciens modes (`oh start --parallel`, `--sweep`, `--dev`) sont des alias deprecies de ces workflows ; le moniteur et la vue de fusion du mode parallele ont ete supprimes.
