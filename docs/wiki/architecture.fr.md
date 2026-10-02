---
page: architecture
title: Vue d'ensemble de l'architecture
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/deploy/deploy.go
  - docs/architecture/system-overview.fr.md
last_updated: 2026-10-02
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
|  init | start | deploy | review | team | ... |
+---------------------------------------------+
|              Services centraux               |
|  Config | Deploy | MCP | Platform | Storage  |
+---------------------------------------------+
```

## Packages principaux

| Package | Responsabilite |
|---------|---------------|
| `cmd/` | Definitions des commandes Cobra (points d'entree CLI) |
| `internal/config/` | Configuration hub (`hub.toml`, TOML + Viper) |
| `internal/deploy/` | Moteur de deploiement transactionnel (agents -> opencode.json) |
| `internal/mcp/` | 7 serveurs MCP integres (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team) |
| `internal/opencode/` | Integration OpenCode (sessions, abstraction plateforme) |
| `internal/parallel/` | Coordination de sessions paralleles (worktrees, recovery, merge) |
| `internal/sweep/` | Mode sweep (decomposition d'objectifs, verification) |
| `internal/teamstate/` | Gestion de l'etat equipe (claims, wiki, policies, evenements) |
| `internal/tui/` | Shell TUI (base tview, 3 modes de navigation) |
| `internal/storage/` | Base SQLite, trousseau systeme, chiffrement fichier |
| `internal/workflow/` | Definitions de workflows et validation des permissions |
| `internal/i18n/` | Internationalisation (FR + EN, fichiers locale JSON) |

## Contenu embarque

Les agents, skills et permissions sont compiles dans le binaire via `go:embed` (`internal/hubcontent/`). La commande `oh deploy` les extrait et les transforme au format `opencode.json` pour le projet cible.

## Architecture MCP

Chaque serveur MCP s'execute comme un sous-processus lance par OpenCode. Les tokens sont transmis via des variables d'environnement (jamais persistes en clair). Les serveurs communiquent via stdin/stdout en utilisant le protocole MCP.

## Modes de session

| Mode | Point d'entree | Description |
|------|---------------|-------------|
| Interactif | `oh start` | Session TUI unique |
| Dev | `oh start --dev` | Workflow dev orchestre avec tickets |
| Parallele | `oh start --parallel` | N sessions concurrentes dans des worktrees |
| Sweep | `oh start --sweep` | Decomposition d'objectif + execution parallele |
| Headless | `oh start --headless` | Non-interactif (CI/scripting) |
| Review | `oh review` | Revue de code IA avec publication MR optionnelle |
