> [Read in English](glossary.en.md)

# Glossaire

Référence rapide des termes utilisés dans la documentation OpenHub.

---

### Adaptateur

Composant d'oh qui traduit un [SessionSpec](#sessionspec) dans le format d'un outil (aujourd'hui opencode V2 : configuration, agents, permissions, serveur `opencode serve`) et qui vérifie le [monde fermé](#monde-fermé) à chaque démarrage (`Attest`). Voir [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.fr.md).

### Agent

Un rôle IA spécialisé (fichier Markdown avec frontmatter YAML) qui détermine le persona, les permissions, les skills et les règles de délégation d'une session IA. 18 agents, organisés en 9 familles : planning, developer, auditor, infra, delivery, quality, design, documentation, utility. Voir [Architecture des agents](../architecture/agents.fr.md).

### Brique / Catalogue de briques

Une brique est un agent ou une skill que les workflows peuvent utiliser. Le catalogue de briques reunit celles du hub et celles de l'équipe (`catalog/agents`, `catalog/skills` du team-state). TUI : commande `bricks`. Voir [Workflows d'équipe](../guides/team-workflows.fr.md#catalogue-de-briques-déquipe).

### Brouillon / Publication

Un brouillon est la version non publiée d'un workflow d'équipe ou de projet, propre à un membre (`workflows/drafts/<membre>/`), validé à l'enregistrement et lançable en local (`oh run <id> --draft`). La publication (`oh workflow publish`, tout membre) le revalide, crée une nouvelle version et met à jour `workflows.lock` ; hors ligne, elle est mise en file d'attente. Voir [Workflows d'équipe](../guides/team-workflows.fr.md#brouillons-publication-historique).

### Beads

Le système de suivi de tickets léger intégré à OpenHub. Les tickets Beads (`bd-1`, `bd-2`, ...) sont créés par l'agent planner et consommés par l'orchestrator-dev pendant les workflows d'implémentation. Nécessite l'outil CLI `bd`. Voir [Référence modèle Beads](beads-model.fr.md).

### Bucket A (Skills inline)

Skills listes dans le tableau `skills:` du frontmatter d'un agent. Ils sont injectés directement dans le prompt système de l'agent à la construction du paquet de session -- toujours présents dans le contexte de l'agent. À opposer au [Bucket B](#bucket-b-skills-natifs).

### Bucket B (Skills natifs)

Skills listes dans le tableau `native_skills:` du frontmatter d'un agent. Ils sont livrés en fichiers séparés dans le paquet de session et chargés à la demande via l'outil `skill` pendant une session. Ils étendent les capacités d'un agent sans consommer de contexte de base. À opposer au [Bucket A](#bucket-a-skills-inline).

### Checkpoint (CP)

Un point de passage d'un [workflow](#workflow) (`checkpoints:`, ex. `cp-plan`, `cp-2`) ou l'utilisateur valide, corrige ou donne une autre consigne avant la suite. Son comportement dépend du [mode de workflow](#mode-de-workflow) (`pause`, `auto`, `skip`, `conditional`) ; un checkpoint `mandatory` ne peut pas être assoupli. Il est tenu à **3 niveaux** : le prompt généré (skill `workflow/workflow-map`), l'outil MCP `workflow_checkpoint` (permission `ask` validée par oh) et le plugin oh ; la machine à états est dans oh. Une pause devient une [décision](#décision) ⏸. Voir [schéma `oh/v1`](workflow-schema.fr.md#checkpoints) et [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md).

### Conductor

Agent d'entrée générique d'un workflow quand `entry.agent` est absent : sans écriture ni shell, il suit la carte du workflow générée et délègue aux autres agents selon `after` et les checkpoints (workflows `spec`, `sweep`). Voir [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.fr.md).

### Claim

En mode équipe, un claim est une assignation de ticket : un membre de l'équipe "claim" un ticket pour signaler qu'il travaille dessus. Les claims empêchent le travail en double entre membres. Géré via `oh team claim` / `oh team release`. Voir [Configuration équipe](../guides/team-setup.fr.md).

### Décision

Ce qu'une session attend de l'utilisateur : ⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur, ⊘ coupe-circuit. La première réponse gagne, d'où qu'elle vienne (TUI, opencode, navigateur, `oh session approve|answer`). Visible dans la section « À traiter » de la vue Sessions et dans `oh session inbox`. Voir [Sessions v5](../guides/sessions-v5.fr.md#checkpoints).

### Démon ohd

Processus d'arrière-plan d'oh (`oh daemon status|stop`) : [proxy d'identifiants](#proxy-didentifiants), supervision des sessions, flux, [décisions](#décision), notifications système et [passerelles](#passerelle-beads--mcp). Sous Windows, il tourne dans le processus oh. Voir [ADR-047](../architecture/adr/047-session-interaction-daemon.fr.md).

### Deploy (Déploiement, supprimé en v5)

Anciennement, le processus de copie des agents, skills, permissions et configuration depuis le hub central (`~/.oh/`) vers le répertoire `.opencode/` d'un projet cible (`oh deploy` / `oh sync`, supprimés en v5). Remplacé par le paquet de session : chaque session démarre d'un paquet construit au lancement hors du projet (`~/.oh/bundles/<hash>/`) à partir de son workflow ; l'inspecter avec `oh bundle show <workflow>`. Les restes des anciens déploiements sont supprimés par `oh migrate deploy-cleanup`. Voir [Démarrage rapide](../guides/getting-started.fr.md#paquet-de-session).

### Environnement d'exécution (runtime)

L'endroit où tourne le serveur d'une session : `local` (la machine), `container` (Colima, Podman ou Docker, une image par projet, paquet monté en lecture seule) ou `remote` (GitLab CI, projet `oh-runner`). Autorisé par `runtime.allowed` du workflow ; ordre du choix : option > projet > Réglages > workflow. Voir [ADR-045](../architecture/adr/045-execution-environments.fr.md), [Conteneur](../guides/container.fr.md), [Exécution distante](../guides/remote-runners.fr.md).

### Espace solo

Un team-state local, sans dépôt distant (`~/.oh/teams/<id>/`), qui permet à un projet sans équipe d'avoir ses propres workflows. Créé par `oh team init --solo`, partage plus tard avec `oh team promote --remote <url>`. Voir [Workflows d'équipe](../guides/team-workflows.fr.md#projet-sans-équipe--espace-solo).

### Groupe de serveur

L'ensemble des sessions servies par un même serveur `opencode serve` : même version du [paquet de session](#paquet-de-session), même projet, même [environnement d'exécution](#environnement-dexécution-runtime). Chaque groupe reçoit son propre jeton du [proxy d'identifiants](#proxy-didentifiants). Voir [Sessions v5](../guides/sessions-v5.fr.md#fonctionnement).

### Hub

Le répertoire d'installation central (`~/.oh/`) contenant le fichier de configuration (`hub.toml`), la base de données des projets (`oh.db`), les agents et skills embarqués (`hub/`), et les clés de chiffrement. Le hub est créé par `oh init` et sert de source de vérité unique pour tous les projets.

### Hub Content

La collection de définitions d'agents et de protocoles de skills embarquee dans le binaire `oh` à la compilation via `go:embed`. Extraite dans `~/.oh/hub/` au premier lancement. Mise à jour quand vous mettez à jour `oh`.

### Living Wiki (Wiki vivant)

Un système de documentation structurée générée par l'agent [onboarder](#onboarder) lors de la découverte d'un projet. Il consiste en des pages Markdown interconnectees avec des tags de confiance, des god nodes (concepts à haute connectivité), et un enrichissement incrémental. Voir [Architecture du wiki vivant](../architecture/living-wiki.fr.md).

### MCP Server (Model Context Protocol)

Un service qui expose des outils externes aux agents IA via le protocole JSON-RPC sur stdio. OpenHub inclut 7 serveurs MCP intégrés : Figma, GitHub, GitLab, Google Slides, Jira, Linear et Team. Voir [Référence services](services.fr.md).

### Monde fermé

Règle de v5 : une session ne voit que les agents et skills de son [paquet](#paquet-de-session) ; les agents natifs d'opencode et la configuration personnelle sont masqués. Vérifié à chaque démarrage (`Attest`) ; en cas d'échec, la session ne démarre pas. Voir [ADR-041](../architecture/adr/041-closed-world-isolation.fr.md).

### Mode (Agent)

Un agent peut fonctionner selon deux modes :
- **Primary (principal)** : lancé directement par l'utilisateur via `oh run <workflow>` ou le TUI (agent d'entrée du workflow). Possède sa propre session.
- **Subagent (sous-agent)** : invoqué par un autre agent via l'outil `task`. S'exécute dans la session de l'agent parent.

### Mode Équipe

Mode de navigation du TUI focalisé sur une équipe active. L'omnibar affiche uniquement les commandes équipe (team board, status, policies) et les commandes globales. Active automatiquement si une seule équipe est configurée, ou manuellement via `Ctrl+T` / sélection depuis le Hub Home. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Hub

Mode de navigation par défaut du TUI, affichant tous les projets et équipes. Active automatiquement si plusieurs projets ou équipes sont configurés. Permet de sélectionner un projet ou une équipe pour basculer en mode focalisé. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Projet

Mode de navigation du TUI focalisé sur un projet actif. L'omnibar affiche uniquement les commandes projet (sessions, board, config projet) et les commandes globales. Active automatiquement si un seul projet est configuré, ou en choisissant le projet depuis le Hub Home (`Esc` revient au Hub, `Ctrl+T` passe en mode équipe). Voir [Usage TUI](../guides/tui-usage.fr.md).

### Onboarder

Un agent principal de la famille planning qui explore un codebase existant, détecte la stack technique, identifie les risques et produit un [wiki vivant](#living-wiki-wiki-vivant). Invoqué avec `oh run onboarding`.

### OpenCode

Le runtime sous-jacent de l'agent de code IA (binaire séparé) qu'OpenHub orchestre. OpenCode gère la conversation LLM réelle, l'exécution des outils et la persistance des sessions. oh demande opencode V2 (>= 2.0.0), installé avec son propre outil ; V1 n'est plus pris en charge (`oh doctor` le vérifie). Voir le [guide de migration v5](../guides/migration-v5.fr.md).

### Orchestrator (Orchestrateur)

L'agent coordinateur principal. Reçoit les demandes utilisateur, délègue aux agents spécialisés (planner, designer, developer, auditor, reviewer, debugger, documentarian), et gère le workflow via les [checkpoints](#checkpoint-cp). Ne réalise jamais d'analyse ou de code directement.

### Orchestrator-dev

Un agent coordinateur spécialisé pour les workflows d'implémentation. Gère le cycle de vie des tickets Beads : sélectionne les tickets, route vers le domaine [developer](../architecture/agents.fr.md) approprié, déclenche la review et gère le merge. Voir [Workflows](../guides/workflows.fr.md).

### Paquet de session

Le dossier immuable et hache (`~/.oh/bundles/<hash>/`) construit au lancement depuis le [workflow](#workflow), hors du projet : agents, skills, permissions, MCP, plugins, modèle, carte du workflow. Une reprise repart du même paquet. Inspecter : `oh bundle show <workflow>`. Remplace le [déploiement](#deploy-déploiement-supprimé-en-v5). Voir [ADR-043](../architecture/adr/043-session-bundle-deploy-removal.fr.md).

### Passerelle Beads / MCP

Services du [démon ohd](#démon-ohd). La **passerelle Beads** exécute sur la machine les commandes `bd` d'une session, locale ou en conteneur (faux `bd` en tête du `PATH` de la session ou dans le conteneur, liste blanche `beads.allow`) ; en distant, un instantané part avec la session et le journal est rejoué au retour (`oh session resolve`). La **passerelle MCP** sert en HTTP les serveurs MCP d'oh du paquet, les tokens restant sur la machine. Voir [ADR-046](../architecture/adr/046-beads-gateways.fr.md).

### Permission Profile (Profil de permissions)

Un fichier YAML (dans `permissions/`) qui définit quels outils un agent peut utiliser (`bash`, `read`, `edit`, `write`, `glob`, `grep`, `webfetch`, `websearch`, `skill`, `task`). Quatre profils intégrés : `coordinator` (lecture seule), `developer-rw` (accès complet), `readonly-code` (code en lecture seule), `readonly-ops` (opérations d'infrastructure sans modification de fichier).

### Proxy d'identifiants

Service du [démon ohd](#démon-ohd) par lequel passent les appels LLM des sessions : chaque [groupe de serveur](#groupe-de-serveur) reçoit un jeton `ohs_…`, les vraies clés restent sur la machine ; le proxy applique la liste blanche de chemins et de modèles et signe les requêtes Bedrock (SigV4). Voir [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

### Provider (Fournisseur)

Le backend LLM qui sert les réponses du modèle IA. Providers supportés : Amazon Bedrock, Anthropic (API directe), OpenRouter, GitHub Copilot. Configuré via `oh init` ou `oh provider setup`. Voir [Guide providers](../guides/providers.fr.md).

### Restrictions I6

Limites facultatives des sessions, désactivées par défaut : sessions actives max, budget par session et par jour (USD), plafond mémoire, liste de modèles. Cascade hub (`[limits]`) → équipe (recommandé / imposé) → projet → workflow (`limits:`). Gérées par `oh budget show|set|unset|raise` et les Réglages › Restrictions. Voir [Sessions v5](../guides/sessions-v5.fr.md#restrictions) et [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

### SessionSpec

La description d'une session indépendante de l'outil (agents, skills, permissions, MCP, modèle, workflow) dont le [paquet de session](#paquet-de-session) est la forme enregistrée ; l'[adaptateur](#adaptateur) la traduit pour opencode. Voir [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.fr.md).

### Skill (Compétence)

Un document de protocole Markdown qui fournit des connaissances spécifiques à un domaine, des workflows ou des instructions comportementales à un agent. Les skills sont catégorisés en [Bucket A](#bucket-a-skills-inline) (toujours chargés) ou [Bucket B](#bucket-b-skills-natifs) (à la demande). Voir [Architecture des skills](../architecture/skills.fr.md).

### Stack Skills

Protocoles de skills spécifiques à un framework (ex. `dev-standards-react`, `dev-standards-golang`) qui sont ajoutés dynamiquement au paquet de session en fonction de la stack technique détectée du projet cible (langages Go, TypeScript, Python, Rust, Java, Ruby ; frameworks Next.js, Nuxt, React, Vue, Express, Django, FastAPI, Rails ; Vitest, Jest, Docker, CI). Situes dans `skills/developer/stacks/`.

### Target Project (Projet cible)

Un codebase enregistré avec OpenHub via `oh project add`. Les sessions y sont lancées avec `oh run <workflow>` ; rien n'est déployé dans le projet (les paquets de session sont dans `~/.oh/bundles/`). Plusieurs projets peuvent être enregistrés simultanément.

### Team-state Repository (Dépôt d'état équipe)

Un dépôt Git partage par les membres de l'équipe pour synchroniser l'état de collaboration : claims, identités membres, wiki, événements, policies, patterns et credentials. Créé avec `oh team init`. Voir [Configuration équipe](../guides/team-setup.fr.md).

### TUI (Terminal User Interface)

Le tableau de bord terminal interactif lancé en exécutant `oh` sans arguments. Fournit une navigation visuelle pour les projets, sessions, board équipe, configuration, et plus. Construit avec tview (huh sert encore à quelques invites en ligne, hors TUI). Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode de Workflow

Fixé au lancement (`--mode`, fiche de lancement), il détermine le comportement de chaque [checkpoint](#checkpoint-cp), déclaré par le workflow pour chaque mode :
- **Manuel** : la plupart des checkpoints s'arrêtent pour validation
- **Semi-auto** (défaut des workflows livrés) : les checkpoints courants passent seuls ; les points de décision clés (ex. `cp-plan`, `cp-2`) s'arrêtent
- **Auto** : tout sauf les décisions obligatoires — seuls s'arrêtent les checkpoints obligatoires (`mandatory`) et ceux qui déverrouillent une action (ex. `cp-plan` validation du plan, `cp-2` commit)

Un workflow peut restreindre les modes (`modes.allowed`). Voir [schéma `oh/v1`](workflow-schema.fr.md#modes).

### Workflow

La description declarative d'un cas d'usage (YAML `apiVersion: oh/v1`) : agent d'entrée, agents, checkpoints, entrées, ressources, environnements et limites. 21 workflows livrés par le hub (`ticket`, `quick`, `spec`, `implement-spec`, `infra-change`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `incident`, `security-fix`, `spike`, `release`, `benchmark`, `sweep`, `doc-sync`, `skill-refresh`, `brief-enrich`, `merge-conflicts`, `libre` ; `feature` et `cadrage` sont des alias de `spec`), extensibles par l'équipe et le projet (`extends`, la sécurité ne pouvant que se durcir). Lancé par `oh run <workflow>`. Voir [Workflows livrés](workflows.fr.md) et [schéma `oh/v1`](workflow-schema.fr.md).

### Worktree

Un Git worktree utilisé pour isoler les sessions IA en parallèle. Chaque worktree obtient sa propre branche et son répertoire de travail, permettant à plusieurs agents de travailler simultanément sans conflits. Géré via `oh worktree` ou `oh run <workflow> --location new`. Voir [Guide worktree](../worktree.md).
