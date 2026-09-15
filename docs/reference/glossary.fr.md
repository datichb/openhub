> [Read in English](glossary.en.md)

# Glossaire

Reference rapide des termes utilises dans la documentation OpenHub.

---

### Agent

Un role IA specialise (fichier Markdown avec frontmatter YAML) qui determine le persona, les permissions, les skills et les regles de delegation d'une session IA. Les agents sont organises en 7 familles : planning, developer, auditor, quality, design, documentation, utility. Voir [Architecture des agents](../architecture/agents.fr.md).

### Beads

Le systeme de suivi de tickets leger integre a OpenHub. Les tickets Beads (`bd-1`, `bd-2`, ...) sont crees par l'agent planner et consommes par l'orchestrator-dev pendant les workflows d'implementation. Necessite l'outil CLI `bd`. Voir [Reference modele Beads](beads-model.fr.md).

### Bucket A (Skills inline)

Skills listes dans le tableau `skills:` du frontmatter d'un agent. Ils sont injectes directement dans le prompt systeme de l'agent au deploiement -- toujours presents dans le contexte de l'agent. A opposer au [Bucket B](#bucket-b-skills-natifs).

### Bucket B (Skills natifs)

Skills listes dans le tableau `native_skills:` du frontmatter d'un agent. Ils sont deployes en fichiers separes et charges a la demande via l'outil `skill` pendant une session. Ils etendent les capacites d'un agent sans consommer de contexte de base. A opposer au [Bucket A](#bucket-a-skills-inline).

### Checkpoint (CP)

Un point de pause predefini dans un workflow d'orchestrateur ou l'utilisateur peut valider, rejeter ou modifier le travail avant de continuer. Les checkpoints sont numerotes (CP-0 a CP-7) et leur comportement depend du [mode de workflow](#mode-de-workflow). Voir [Delegation de taches](../architecture/task-delegation.fr.md).

### Claim

En mode equipe, un claim est une assignation de ticket : un membre de l'equipe "claim" un ticket pour signaler qu'il travaille dessus. Les claims empechent le travail en double entre membres. Gere via `oh team claim` / `oh team release`. Voir [Configuration equipe](../guides/team-setup.fr.md).

### Deploy (Deploiement)

Le processus de copie des agents, skills, permissions et configuration depuis le hub central (`~/.oh/`) vers le repertoire `.opencode/` d'un projet cible. Le deploiement est transactionnel (5 phases, rollback en cas d'erreur). Execute avec `oh deploy`. Voir [Demarrage rapide](../guides/getting-started.fr.md#deployer-les-agents-et-skills).

### Hub

Le repertoire d'installation central (`~/.oh/`) contenant le fichier de configuration (`hub.toml`), la base de donnees des projets (`oh.db`), les agents et skills embarques (`hub/`), et les cles de chiffrement. Le hub est cree par `oh init` et sert de source de verite unique pour tous les projets.

### Hub Content

La collection de definitions d'agents et de protocoles de skills embarquee dans le binaire `oh` a la compilation via `go:embed`. Extraite dans `~/.oh/hub/` au premier lancement. Mise a jour quand vous mettez a jour `oh`.

### Living Wiki (Wiki vivant)

Un systeme de documentation structure genere par l'agent [onboarder](#onboarder) lors de la decouverte d'un projet. Il consiste en des pages Markdown interconnectees avec des tags de confiance, des god nodes (concepts a haute connectivite), et un enrichissement incremental. Voir [Architecture du wiki vivant](../architecture/living-wiki.fr.md).

### MCP Server (Model Context Protocol)

Un service qui expose des outils externes aux agents IA via le protocole JSON-RPC sur stdio. OpenHub inclut 7 serveurs MCP integres : Figma, GitHub, GitLab, Google Slides, Jira, Linear et Team. Voir [Reference services](services.fr.md).

### Mode (Agent)

Un agent peut fonctionner selon deux modes :
- **Primary (principal)** : lance directement par l'utilisateur via `oh start` ou le TUI. Possede sa propre session.
- **Subagent (sous-agent)** : invoque par un autre agent via l'outil `task`. S'execute dans la session de l'agent parent.

### Mode Equipe

Mode de navigation du TUI focalise sur une equipe active. L'omnibar affiche uniquement les commandes equipe (team board, status, policies) et les commandes globales. Active automatiquement si une seule equipe est configuree, ou manuellement via `Ctrl+T` / selection depuis le Hub Home. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Hub

Mode de navigation par defaut du TUI, affichant tous les projets et equipes. Active automatiquement si plusieurs projets ou equipes sont configures. Permet de selectionner un projet ou une equipe pour basculer en mode focalise. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Projet

Mode de navigation du TUI focalise sur un projet actif. L'omnibar affiche uniquement les commandes projet (sessions, board, deploy, config projet) et les commandes globales. Active automatiquement si un seul projet est configure, ou manuellement via `Ctrl+T` / selection depuis le Hub Home. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Onboarder

Un agent principal de la famille planning qui explore un codebase existant, detecte la stack technique, identifie les risques et produit un [wiki vivant](#living-wiki-wiki-vivant). Invoque avec `oh start --onboard`.

### OpenCode

Le runtime sous-jacent de l'agent de code IA (binaire separe) qu'OpenHub orchestre. OpenCode gere la conversation LLM reelle, l'execution des outils et la persistance des sessions. Telecharge automatiquement par `oh init` ou `oh start`.

### Orchestrator (Orchestrateur)

L'agent coordinateur principal. Recoit les demandes utilisateur, delegue aux agents specialises (planner, designer, developer, auditor, reviewer, debugger, documentarian), et gere le workflow via les [checkpoints](#checkpoint-cp). Ne realise jamais d'analyse ou de code directement.

### Orchestrator-dev

Un agent coordinateur specialise pour les workflows d'implementation. Gere le cycle de vie des tickets Beads : selectionne les tickets, route vers le domaine [developer](#developer) approprie, declenche la review et gere le merge. Voir [Workflows](../guides/workflows.fr.md).

### Permission Profile (Profil de permissions)

Un fichier YAML (dans `permissions/`) qui definit quels outils un agent peut utiliser (`bash`, `read`, `edit`, `write`, `glob`, `grep`, `webfetch`, `websearch`, `skill`, `task`). Trois profils integres : `coordinator` (lecture seule), `developer-rw` (acces complet), `readonly-code` (code en lecture seule).

### Provider (Fournisseur)

Le backend LLM qui sert les reponses du modele IA. Providers supportes : Amazon Bedrock, Anthropic (API directe), OpenRouter, GitHub Copilot. Configure via `oh init` ou `oh provider setup`. Voir [Guide providers](../guides/providers.fr.md).

### Skill (Competence)

Un document de protocole Markdown qui fournit des connaissances specifiques a un domaine, des workflows ou des instructions comportementales a un agent. Les skills sont categorises en [Bucket A](#bucket-a-skills-inline) (toujours charges) ou [Bucket B](#bucket-b-skills-natifs) (a la demande). Voir [Architecture des skills](../architecture/skills.fr.md).

### Stack Skills

Protocoles de skills specifiques a un framework (ex. `dev-standards-react`, `dev-standards-golang`) qui sont injectes dynamiquement au deploiement en fonction de la stack technique detectee du projet cible. Situes dans `skills/developer/stacks/`.

### Target Project (Projet cible)

Un codebase enregistre avec OpenHub via `oh project add`. Le deploiement copie le contenu du hub dans le repertoire `.opencode/` du projet. Plusieurs projets peuvent etre enregistres simultanement.

### Team-state Repository (Depot d'etat equipe)

Un depot Git partage par les membres de l'equipe pour synchroniser l'etat de collaboration : claims, identites membres, wiki, evenements, policies, patterns et credentials. Cree avec `oh team init`. Voir [Configuration equipe](../guides/team-setup.fr.md).

### TUI (Terminal User Interface)

Le tableau de bord terminal interactif lance en executant `oh` sans arguments. Fournit une navigation visuelle pour les projets, sessions, board equipe, configuration, et plus. Construit avec le framework Charm/BubbleTea. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode de Workflow

Determine le comportement des [checkpoints](#checkpoint-cp) pendant une session d'orchestrateur :
- **Manuel** : tous les checkpoints s'arretent pour validation utilisateur
- **Semi-auto** : la plupart des checkpoints procedent automatiquement ; les points de decision cles (CP-0, CP-2) s'arretent
- **Auto** : tous les checkpoints procedent automatiquement sauf la validation initiale du plan (CP-0)

### Worktree

Un Git worktree utilise pour isoler les sessions IA en parallele. Chaque worktree obtient sa propre branche et son repertoire de travail, permettant a plusieurs agents de travailler simultanement sans conflits. Gere via `oh worktree` ou `oh start -w`. Voir [Guide worktree](../worktree.md).
