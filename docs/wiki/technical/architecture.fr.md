---
updated: 2026-10-15
confidence: confirmed
agents: [onboarder]
---

> [Read in English](architecture.en.md)

# Vue d'ensemble de l'architecture

Sources de la page — `CONFIRMÉ` · onboarder · 2026-10-06 · cli/cmd/root.go, cli/internal/bundle/build.go, cli/internal/runsvc/service.go, docs/architecture/overview.fr.md

## Conception du système

openhub (`oh`) est un binaire Go unique qui lance et pilote les sessions de codage assistées par IA à travers les projets. `oh` est la **tour de contrôle**, opencode V2 la **cabine** : oh choisit le workflow, construit le paquet de session, démarre un serveur opencode par groupe et suit les sessions ; opencode exécute les agents et sert d'interface de conversation. Fermer opencode n'arrête pas une session.

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

1. **Résolution du workflow** : hub < équipe < projet (< brouillon) < options de session.
2. **Fiche de lancement / `--recap`** : entrées, mode, environnement, emplacement, avertissements.
3. **Paquet de session** : `~/.oh/bundles/<hash>/`, immuable, partage par le groupe.
4. **Serveur de groupe** : un `opencode serve` par (version du paquet, projet, environnement).
5. **Vérification** : monde fermé (`Attest`) ; sinon la session ne démarre pas.
6. **Session** : créée par l'API (identifiant choisi par oh), prompt initial.
7. **Ouverture** : iTerm2 → Terminal.app → tmux → navigateur → terminal courant.
8. **Suivi et décisions** : démon `ohd` (⏸ ? ! $ ✗), notifications, veille après 5 minutes d'inactivité.

Plus rien n'est écrit dans le projet (`oh deploy` et `oh sync` sont des alias qui affichent un message de migration).

## Packages principaux

| Package | Responsabilité |
|---------|---------------|
| `cmd/` | Définitions des commandes Cobra (points d'entrée CLI), cablage de la TUI ; `cmd/oh-bd` : faux `bd` des conteneurs et des jobs distants |
| `internal/config/` | Configuration hub (`hub.toml`, TOML + Viper) |
| `internal/hubcontent/` | Contenu du hub embarqué (agents, skills, permissions, workflows), extrait dans `~/.oh/hub/` |
| `internal/bricks/` | Lecture des briques du hub (agents, skills, permissions, cascade des modèles, skills de stack) |
| `internal/workflow/` | Workflows déclaratifs `oh/v1` (lecture, couches, validation, graphe de délégation, prompt) |
| `internal/services/` | Services partagés CLI/TUI : `workflow`, `session`, `checkpoint`, `remote` |
| `internal/bundle/` | Construction des paquets de session (workflow -> `~/.oh/bundles/<hash>/`) |
| `internal/sessionspec/` | Modèle d'une session indépendant de l'outil (paquet, emplacement, runtime, provider) |
| `internal/adapters/` | Contrat oh <-> outil agentique ; `adapters/opencodev2` : adaptateur opencode V2 (rendu, serveur, `Attest`, plugin) |
| `internal/runsvc/` | Lancement des sessions v5 (groupes de serveurs, jetons du proxy, worktrees, veille, reprise) |
| `internal/daemon/` | Démon `ohd` (proxy d'identifiants, supervision des sessions, décisions, notifications) |
| `internal/credproxy/` | Proxy d'identifiants LLM (jeton `ohs_` par groupe, vraies clés gardées sur la machine) |
| `internal/gateway/` | Passerelles du démon (Beads, MCP) pour les serveurs hors machine |
| `internal/runtime/` | Environnements d'exécution : local et `runtime/container` |
| `internal/remote/` | Contrat machine <-> job GitLab CI (manifeste, enveloppe, pipeline généré, client GitLab) |
| `internal/limits/` | Restrictions des sessions (I6 : sessions actives, budgets, mémoire, modèles) |
| `internal/mcp/` | 8 serveurs MCP intégrés (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team, Workflow) |
| `internal/teamstate/` | Gestion de l'état équipe (claims, wiki, policies, événements, workflows d'équipe, catalogue, espace solo) |
| `internal/tui/` | Shell TUI (base tview, modes hub / projet / équipe) |
| `internal/storage/` | Base SQLite (`oh.db`), trousseau système, chiffrement fichier |
| `internal/adapters/opencodev2/deploycleanup/` | Nettoyage des restes des anciens déploiements (`oh migrate deploy-cleanup`) |
| `internal/i18n/` | Internationalisation (FR + EN, fichiers locale JSON) |

`internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/plugin` et `platform.SessionPlatform` ont été supprimés en v5 (remplacés par `internal/bricks`, `internal/bundle`, les adaptateurs et `internal/runsvc`).

## Contenu embarqué

Les agents, skills, permissions et workflows livrés sont compilés dans le binaire via `go:embed` (`internal/hubcontent/`). À chaque lancement, `internal/bundle` en construit, hors du projet, le paquet de session du workflow (`~/.oh/bundles/<hash>/` : agents membres avec skills Bucket A intégrées, skills à la demande, permissions, graphe de délégation, MCP, plugin oh) ; l'adaptateur en produit la config opencode.

## Monde fermé

Une session ne voit que les agents et les skills de son paquet : agents natifs d'opencode désactivés, skills intégrées refusées, agents hors du workflow invisibles. L'adaptateur le vérifie à chaque démarrage de serveur (`Attest`) ; en cas d'écart, la session ne démarre pas. Voir [ADR-041](../../architecture/adr/041-closed-world-isolation.fr.md).

## Secrets et démon

Le démon `ohd` héberge le proxy d'identifiants : opencode ne reçoit qu'un jeton de groupe (`ohs_…`), la vraie clé reste dans le trousseau (listes blanches de chemins et de modèles, SigV4 pour les profils AWS). Le socket du démon est en 0600 avec vérification de l'UID du pair ; les routes qui émettent des jetons exigent une capacité (`X-Oh-Capability`). Sous Windows, le démon tourne dans le processus oh. Voir [SECURITY](../../../SECURITY.fr.md) et [ADR-044](../../architecture/adr/044-credential-proxy-session-limits.fr.md).

## Architecture MCP

Les serveurs MCP d'oh actives pour le projet sont déclarés dans le paquet de session sous la forme `oh mcp serve <nom>` (stdio). Chaque serveur lit lui-meme son jeton dans le trousseau système (`--token-key` : seul le nom de la clé est dans le paquet) ; les jetons ne sont jamais écrits dans le paquet ni dans la config opencode. Le serveur `workflow` (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) est ajouté à tout paquet de workflow. Quand la session tourne hors de la machine (conteneur), les serveurs MCP d'oh tournent sur la machine, derrière la passerelle MCP HTTP du démon `ohd`.

## Environnements d'exécution

| Runtime | Où tourne opencode | Ce qui reste sur la machine |
|---------|--------------------|-----------------------------|
| local | `opencode serve` sur la machine | tout |
| conteneur | un conteneur par groupe (Colima, Podman, Docker CLI), image de dev du projet + couche oh, paquet en lecture seule | clés (proxy), Beads (passerelle), serveurs MCP d'oh (passerelle) |
| distant | job GitLab CI du projet `oh-runner` | clés ; Beads : instantané en entrée, journal en sortie, rejoué par `oh session resolve` |

Voir [ADR-045](../../architecture/adr/045-execution-environments.fr.md) et [ADR-046](../../architecture/adr/046-beads-gateways.fr.md).

## Workflows et sessions

Chaque session est lancée depuis un workflow déclaratif (`oh run <workflow>` ou la fiche de lancement de la TUI) et se suit dans la vue **Sessions** ou avec `oh session …`. Voir [Workflows livrés](../../reference/workflows.fr.md).

| Usage | Point d'entrée | Description |
|------|---------------|-------------|
| Interactif | `oh run [workflow]` | Une session (sans argument : workflow par défaut du projet) |
| Feature | `oh run feature` | Planification, tickets, implémentation orchestree |
| Tickets | `oh run ticket --tickets a,b` | Une session par ticket (un worktree par session qui écrit, un seul serveur) ; `--one-session` pour tout regrouper |
| Sweep | `oh run sweep -i goal=<objectif>` | Découpage d'un objectif en sous-taches, lancées en parallèle dans la même session, puis vérification |
| Libre | `oh run libre --agent <id>` | Agent d'entrée au choix, sans checkpoint |
| Headless | `oh run <workflow> --headless` | Non-interactif (CI/scripting) |
| Review | `oh run review` | Revue de code IA ; publication MR avec `oh review --publish` |

Les anciens modes (`oh start --parallel`, `--sweep`, `--dev`) sont des alias dépréciés de ces workflows ; le moniteur et la vue de fusion du mode parallèle ont été supprimés.
