---
page: architecture
title: Vue d'ensemble de l'architecture
confidence: CONFIRMED
sources:
  - cli/cmd/root.go
  - cli/internal/bundle/build.go
  - cli/internal/runsvc/service.go
  - docs/architecture/overview.fr.md
last_updated: 2026-10-06
---

> [Read in English](architecture.en.md)

# Vue d'ensemble de l'architecture

## Conception du systeme

openhub (`oh`) est un binaire Go unique qui lance et pilote les sessions de codage assistees par IA a travers les projets. `oh` est la **tour de controle**, opencode V2 la **cabine** : oh choisit le workflow, construit le paquet de session, demarre un serveur opencode par groupe et suit les sessions ; opencode execute les agents et sert d'interface de conversation. Fermer opencode n'arrete pas une session.

```
+---------------------------------------------+
|              TUI Shell (tview)               |
|  Demarrer | Sessions | Workflows | Reglages  |
+---------------------------------------------+
|              Commandes CLI (Cobra)           |
|  init | run | workflow | bundle | session    |
+---------------------------------------------+
|   Services partages (CLI, TUI, oh serve)     |
|  Workflow | Session | Checkpoint | Remote    |
+---------------------------------------------+
|  Bundle | RunService | Adaptateur opencode V2|
+---------------------------------------------+
|  Demon ohd : proxy d'identifiants,           |
|  supervision, decisions, passerelles, limites|
+---------------------------------------------+
```

## Flux d'une session

1. **Resolution du workflow** : hub < equipe < projet (< brouillon) < options de session.
2. **Fiche de lancement / `--recap`** : entrees, mode, environnement, emplacement, avertissements.
3. **Paquet de session** : `~/.oh/bundles/<hash>/`, immuable, partage par le groupe.
4. **Serveur de groupe** : un `opencode serve` par (version du paquet, projet, environnement).
5. **Verification** : monde ferme (`Attest`) ; sinon la session ne demarre pas.
6. **Session** : creee par l'API (identifiant choisi par oh), prompt initial.
7. **Ouverture** : iTerm2 → Terminal.app → tmux → navigateur → terminal courant.
8. **Suivi et decisions** : demon `ohd` (⏸ ? ! $ ✗), notifications, veille apres 5 minutes d'inactivite.

Plus rien n'est ecrit dans le projet (`oh deploy` et `oh sync` sont des alias qui affichent un message de migration).

## Packages principaux

| Package | Responsabilite |
|---------|---------------|
| `cmd/` | Definitions des commandes Cobra (points d'entree CLI), cablage de la TUI ; `cmd/oh-bd` : faux `bd` des conteneurs et des jobs distants |
| `internal/config/` | Configuration hub (`hub.toml`, TOML + Viper) |
| `internal/hubcontent/` | Contenu du hub embarque (agents, skills, permissions, workflows), extrait dans `~/.oh/hub/` |
| `internal/bricks/` | Lecture des briques du hub (agents, skills, permissions, cascade des modeles, skills de stack) |
| `internal/workflow/` | Workflows declaratifs `oh/v1` (lecture, couches, validation, graphe de delegation, prompt) |
| `internal/services/` | Services partages CLI/TUI : `workflow`, `session`, `checkpoint`, `remote` |
| `internal/bundle/` | Construction des paquets de session (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/sessionspec/` | Modele d'une session independant de l'outil (paquet, emplacement, runtime, provider) |
| `internal/adapters/` | Contrat oh <-> outil agentique ; `adapters/opencodev2` : adaptateur opencode V2 (rendu, serveur, `Attest`, plugin) |
| `internal/runsvc/` | Lancement des sessions v5 (groupes de serveurs, jetons du proxy, worktrees, veille, reprise) |
| `internal/daemon/` | Demon `ohd` (proxy d'identifiants, supervision des sessions, decisions, notifications) |
| `internal/credproxy/` | Proxy d'identifiants LLM (jeton `ohs_` par groupe, vraies cles gardees sur la machine) |
| `internal/gateway/` | Passerelles du demon (Beads, MCP) pour les serveurs hors machine |
| `internal/runtime/` | Environnements d'execution : local et `runtime/container` |
| `internal/remote/` | Contrat machine <-> job GitLab CI (manifeste, enveloppe, pipeline genere, client GitLab) |
| `internal/limits/` | Restrictions des sessions (I6 : sessions actives, budgets, memoire, modeles) |
| `internal/mcp/` | 8 serveurs MCP integres (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team, Workflow) |
| `internal/teamstate/` | Gestion de l'etat equipe (claims, wiki, policies, evenements, workflows d'equipe, catalogue, espace solo) |
| `internal/tui/` | Shell TUI (base tview, modes hub / projet / equipe) |
| `internal/storage/` | Base SQLite (`oh.db`), trousseau systeme, chiffrement fichier |
| `internal/deploycleanup/` | Nettoyage des restes des anciens deploiements (`oh migrate deploy-cleanup`) |
| `internal/i18n/` | Internationalisation (FR + EN, fichiers locale JSON) |

`internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/plugin` et `platform.SessionPlatform` ont ete supprimes en v5 (remplaces par `internal/bricks`, `internal/bundle`, les adaptateurs et `internal/runsvc`).

## Contenu embarque

Les agents, skills, permissions et workflows livres sont compiles dans le binaire via `go:embed` (`internal/hubcontent/`). A chaque lancement, `internal/bundle` en construit, hors du projet, le paquet de session du workflow (`~/.oh/bundles/<hash>/` : agents membres avec skills Bucket A integrees, skills a la demande, permissions, graphe de delegation, MCP, plugin oh) ; l'adaptateur en produit la config opencode.

## Monde ferme

Une session ne voit que les agents et les skills de son paquet : agents natifs d'opencode desactives, skills integrees refusees, agents hors du workflow invisibles. L'adaptateur le verifie a chaque demarrage de serveur (`Attest`) ; en cas d'ecart, la session ne demarre pas. Voir [ADR-041](../architecture/adr/041-closed-world-isolation.fr.md).

## Secrets et demon

Le demon `ohd` heberge le proxy d'identifiants : opencode ne recoit qu'un jeton de groupe (`ohs_…`), la vraie cle reste dans le trousseau (listes blanches de chemins et de modeles, SigV4 pour les profils AWS). Le socket du demon est en 0600 avec verification de l'UID du pair ; les routes qui emettent des jetons exigent une capacite (`X-Oh-Capability`). Sous Windows, le demon tourne dans le processus oh. Voir [SECURITY](../../SECURITY.fr.md) et [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

## Architecture MCP

Les serveurs MCP d'oh actives pour le projet sont declares dans le paquet de session sous la forme `oh mcp serve <nom>` (stdio). Chaque serveur lit lui-meme son jeton dans le trousseau systeme (`--token-key` : seul le nom de la cle est dans le paquet) ; les jetons ne sont jamais ecrits dans le paquet ni dans la config opencode. Le serveur `workflow` (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) est ajoute a tout paquet de workflow. Quand la session tourne hors de la machine (conteneur), les serveurs MCP d'oh tournent sur la machine, derriere la passerelle MCP HTTP du demon `ohd`.

## Environnements d'execution

| Runtime | Ou tourne opencode | Ce qui reste sur la machine |
|---------|--------------------|-----------------------------|
| local | `opencode serve` sur la machine | tout |
| conteneur | un conteneur par groupe (Colima, Podman, Docker CLI), image de dev du projet + couche oh, paquet en lecture seule | cles (proxy), Beads (passerelle), serveurs MCP d'oh (passerelle) |
| distant | job GitLab CI du projet `oh-runner` | cles ; Beads : instantane en entree, journal en sortie, rejoue par `oh session resolve` |

Voir [ADR-045](../architecture/adr/045-execution-environments.fr.md) et [ADR-046](../architecture/adr/046-beads-gateways.fr.md).

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
