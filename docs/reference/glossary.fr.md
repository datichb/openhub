> [Read in English](glossary.en.md)

# Glossaire

Reference rapide des termes utilises dans la documentation OpenHub.

---

### Adaptateur

Composant d'oh qui traduit un [SessionSpec](#sessionspec) dans le format d'un outil (aujourd'hui opencode V2 : configuration, agents, permissions, serveur `opencode serve`) et qui verifie le [monde ferme](#monde-ferme) a chaque demarrage (`Attest`). Voir [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.fr.md).

### Agent

Un role IA specialise (fichier Markdown avec frontmatter YAML) qui determine le persona, les permissions, les skills et les regles de delegation d'une session IA. Les agents sont organises en 7 familles : planning, developer, auditor, quality, design, documentation, utility. Voir [Architecture des agents](../architecture/agents.fr.md).

### Brique / Catalogue de briques

Une brique est un agent ou une skill que les workflows peuvent utiliser. Le catalogue de briques reunit celles du hub et celles de l'equipe (`catalog/agents`, `catalog/skills` du team-state). TUI : commande `bricks`. Voir [Workflows d'equipe](../guides/team-workflows.fr.md#catalogue-de-briques-déquipe).

### Brouillon / Publication

Un brouillon est la version non publiee d'un workflow d'equipe ou de projet, propre a un membre (`workflows/drafts/<membre>/`), valide a l'enregistrement et lancable en local (`oh run <id> --draft`). La publication (`oh workflow publish`, tout membre) le revalide, cree une nouvelle version et met a jour `workflows.lock` ; hors ligne, elle est mise en file d'attente. Voir [Workflows d'equipe](../guides/team-workflows.fr.md#brouillons-publication-historique).

### Beads

Le systeme de suivi de tickets leger integre a OpenHub. Les tickets Beads (`bd-1`, `bd-2`, ...) sont crees par l'agent planner et consommes par l'orchestrator-dev pendant les workflows d'implementation. Necessite l'outil CLI `bd`. Voir [Reference modele Beads](beads-model.fr.md).

### Bucket A (Skills inline)

Skills listes dans le tableau `skills:` du frontmatter d'un agent. Ils sont injectes directement dans le prompt systeme de l'agent a la construction du paquet de session -- toujours presents dans le contexte de l'agent. A opposer au [Bucket B](#bucket-b-skills-natifs).

### Bucket B (Skills natifs)

Skills listes dans le tableau `native_skills:` du frontmatter d'un agent. Ils sont livres en fichiers separes dans le paquet de session et charges a la demande via l'outil `skill` pendant une session. Ils etendent les capacites d'un agent sans consommer de contexte de base. A opposer au [Bucket A](#bucket-a-skills-inline).

### Checkpoint (CP)

Un point de passage d'un [workflow](#workflow) (`checkpoints:`, ex. `cp-0`, `cp-2`) ou l'utilisateur valide, corrige ou donne une autre consigne avant la suite. Son comportement depend du [mode de workflow](#mode-de-workflow) (`pause`, `auto`, `skip`, `conditional`) ; un checkpoint `mandatory` ne peut pas etre assoupli. Il est tenu a **3 niveaux** : le prompt genere (skill `workflow/workflow-map`), l'outil MCP `workflow_checkpoint` (permission `ask` validee par oh) et le plugin oh ; la machine a etats est dans oh. Une pause devient une [decision](#decision) ⏸. Voir [schema `oh/v1`](workflow-schema.fr.md#checkpoints) et [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md).

### Conductor

Agent d'entree generique d'un workflow quand `entry.agent` est absent : sans ecriture ni shell, il suit la carte du workflow generee et delegue aux autres agents selon `after` et les checkpoints (workflows `cadrage`, `sweep`). Voir [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.fr.md).

### Claim

En mode equipe, un claim est une assignation de ticket : un membre de l'equipe "claim" un ticket pour signaler qu'il travaille dessus. Les claims empechent le travail en double entre membres. Gere via `oh team claim` / `oh team release`. Voir [Configuration equipe](../guides/team-setup.fr.md).

### Decision

Ce qu'une session attend de l'utilisateur : ⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur ou coupe-circuit. La premiere reponse gagne, d'ou qu'elle vienne (TUI, opencode, navigateur, `oh session approve|answer`). Visible dans la section « A traiter » de la vue Sessions et dans `oh session inbox`. Voir [Sessions v5](../guides/sessions-v5.fr.md#checkpoints).

### Demon ohd

Processus d'arriere-plan d'oh (`oh daemon status|stop`) : [proxy d'identifiants](#proxy-didentifiants), supervision des sessions, flux, [decisions](#decision), notifications systeme et [passerelles](#passerelle-beads--mcp). Sous Windows, il tourne dans le processus oh. Voir [ADR-047](../architecture/adr/047-session-interaction-daemon.fr.md).

### Deploy (Deploiement, supprime en v5)

Anciennement, le processus de copie des agents, skills, permissions et configuration depuis le hub central (`~/.oh/`) vers le repertoire `.opencode/` d'un projet cible (`oh deploy` / `oh sync`, supprimes en v5). Remplace par le paquet de session : chaque session demarre d'un paquet construit au lancement hors du projet (`~/.oh/bundles/<hash>/`) a partir de son workflow ; l'inspecter avec `oh bundle show <workflow>`. Les restes des anciens deploiements sont supprimes par `oh migrate deploy-cleanup`. Voir [Demarrage rapide](../guides/getting-started.fr.md#paquet-de-session).

### Environnement d'execution (runtime)

L'endroit ou tourne le serveur d'une session : `local` (la machine), `container` (Colima, Podman ou Docker, une image par projet, paquet monte en lecture seule) ou `remote` (GitLab CI, projet `oh-runner`). Autorise par `runtime.allowed` du workflow ; ordre du choix : option > projet > Reglages > workflow. Voir [ADR-045](../architecture/adr/045-execution-environments.fr.md), [Conteneur](../guides/container.fr.md), [Execution distante](../guides/remote-runners.fr.md).

### Espace solo

Un team-state local, sans depot distant (`~/.oh/teams/<id>/`), qui permet a un projet sans equipe d'avoir ses propres workflows. Cree par `oh team init --solo`, partage plus tard avec `oh team promote --remote <url>`. Voir [Workflows d'equipe](../guides/team-workflows.fr.md#projet-sans-équipe--espace-solo).

### Groupe de serveur

L'ensemble des sessions servies par un meme serveur `opencode serve` : meme version du [paquet de session](#paquet-de-session), meme projet, meme [environnement d'execution](#environnement-dexecution-runtime). Chaque groupe recoit son propre jeton du [proxy d'identifiants](#proxy-didentifiants). Voir [Sessions v5](../guides/sessions-v5.fr.md#fonctionnement).

### Hub

Le repertoire d'installation central (`~/.oh/`) contenant le fichier de configuration (`hub.toml`), la base de donnees des projets (`oh.db`), les agents et skills embarques (`hub/`), et les cles de chiffrement. Le hub est cree par `oh init` et sert de source de verite unique pour tous les projets.

### Hub Content

La collection de definitions d'agents et de protocoles de skills embarquee dans le binaire `oh` a la compilation via `go:embed`. Extraite dans `~/.oh/hub/` au premier lancement. Mise a jour quand vous mettez a jour `oh`.

### Living Wiki (Wiki vivant)

Un systeme de documentation structure genere par l'agent [onboarder](#onboarder) lors de la decouverte d'un projet. Il consiste en des pages Markdown interconnectees avec des tags de confiance, des god nodes (concepts a haute connectivite), et un enrichissement incremental. Voir [Architecture du wiki vivant](../architecture/living-wiki.fr.md).

### MCP Server (Model Context Protocol)

Un service qui expose des outils externes aux agents IA via le protocole JSON-RPC sur stdio. OpenHub inclut 7 serveurs MCP integres : Figma, GitHub, GitLab, Google Slides, Jira, Linear et Team. Voir [Reference services](services.fr.md).

### Monde ferme

Regle de v5 : une session ne voit que les agents et skills de son [paquet](#paquet-de-session) ; les agents natifs d'opencode et la configuration personnelle sont masques. Verifie a chaque demarrage (`Attest`) ; en cas d'echec, la session ne demarre pas. Voir [ADR-041](../architecture/adr/041-closed-world-isolation.fr.md).

### Mode (Agent)

Un agent peut fonctionner selon deux modes :
- **Primary (principal)** : lance directement par l'utilisateur via `oh run <workflow>` ou le TUI (agent d'entree du workflow). Possede sa propre session.
- **Subagent (sous-agent)** : invoque par un autre agent via l'outil `task`. S'execute dans la session de l'agent parent.

### Mode Equipe

Mode de navigation du TUI focalise sur une equipe active. L'omnibar affiche uniquement les commandes equipe (team board, status, policies) et les commandes globales. Active automatiquement si une seule equipe est configuree, ou manuellement via `Ctrl+T` / selection depuis le Hub Home. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Hub

Mode de navigation par defaut du TUI, affichant tous les projets et equipes. Active automatiquement si plusieurs projets ou equipes sont configures. Permet de selectionner un projet ou une equipe pour basculer en mode focalise. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode Projet

Mode de navigation du TUI focalise sur un projet actif. L'omnibar affiche uniquement les commandes projet (sessions, board, config projet) et les commandes globales. Active automatiquement si un seul projet est configure, ou manuellement via `Ctrl+T` / selection depuis le Hub Home. Voir [Usage TUI](../guides/tui-usage.fr.md).

### Onboarder

Un agent principal de la famille planning qui explore un codebase existant, detecte la stack technique, identifie les risques et produit un [wiki vivant](#living-wiki-wiki-vivant). Invoque avec `oh run onboarding`.

### OpenCode

Le runtime sous-jacent de l'agent de code IA (binaire separe) qu'OpenHub orchestre. OpenCode gere la conversation LLM reelle, l'execution des outils et la persistance des sessions. oh demande opencode V2 (>= 2.0.0), installe avec son propre outil ; V1 n'est plus pris en charge (`oh doctor` le verifie). Voir le [guide de migration v5](../guides/migration-v5.fr.md).

### Orchestrator (Orchestrateur)

L'agent coordinateur principal. Recoit les demandes utilisateur, delegue aux agents specialises (planner, designer, developer, auditor, reviewer, debugger, documentarian), et gere le workflow via les [checkpoints](#checkpoint-cp). Ne realise jamais d'analyse ou de code directement.

### Orchestrator-dev

Un agent coordinateur specialise pour les workflows d'implementation. Gere le cycle de vie des tickets Beads : selectionne les tickets, route vers le domaine [developer](../architecture/agents.fr.md) approprie, declenche la review et gere le merge. Voir [Workflows](../guides/workflows.fr.md).

### Paquet de session

Le dossier immuable et hache (`~/.oh/bundles/<hash>/`) construit au lancement depuis le [workflow](#workflow), hors du projet : agents, skills, permissions, MCP, plugins, modele, carte du workflow. Une reprise repart du meme paquet. Inspecter : `oh bundle show <workflow>`. Remplace le [deploiement](#deploy-deploiement-supprime-en-v5). Voir [ADR-043](../architecture/adr/043-session-bundle-deploy-removal.fr.md).

### Passerelle Beads / MCP

Services du [demon ohd](#demon-ohd). La **passerelle Beads** execute sur la machine les commandes `bd` d'une session, locale ou en conteneur (faux `bd` en tete du `PATH` de la session ou dans le conteneur, liste blanche `beads.allow`) ; en distant, un instantane part avec la session et le journal est rejoue au retour (`oh session resolve`). La **passerelle MCP** sert en HTTP les serveurs MCP d'oh du paquet, les tokens restant sur la machine. Voir [ADR-046](../architecture/adr/046-beads-gateways.fr.md).

### Permission Profile (Profil de permissions)

Un fichier YAML (dans `permissions/`) qui definit quels outils un agent peut utiliser (`bash`, `read`, `edit`, `write`, `glob`, `grep`, `webfetch`, `websearch`, `skill`, `task`). Trois profils integres : `coordinator` (lecture seule), `developer-rw` (acces complet), `readonly-code` (code en lecture seule).

### Proxy d'identifiants

Service du [demon ohd](#demon-ohd) par lequel passent les appels LLM des sessions : chaque [groupe de serveur](#groupe-de-serveur) recoit un jeton `ohs_…`, les vraies cles restent sur la machine ; le proxy applique la liste blanche de chemins et de modeles et signe les requetes Bedrock (SigV4). Voir [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

### Provider (Fournisseur)

Le backend LLM qui sert les reponses du modele IA. Providers supportes : Amazon Bedrock, Anthropic (API directe), OpenRouter, GitHub Copilot. Configure via `oh init` ou `oh provider setup`. Voir [Guide providers](../guides/providers.fr.md).

### Restrictions I6

Limites facultatives des sessions, desactivees par defaut : sessions actives max, budget par session et par jour (USD), plafond memoire, liste de modeles. Cascade hub (`[limits]`) → equipe (recommande / impose) → projet → workflow (`limits:`). Gerees par `oh budget show|set|unset|raise` et les Reglages › Restrictions. Voir [Sessions v5](../guides/sessions-v5.fr.md#restrictions) et [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

### SessionSpec

La description d'une session independante de l'outil (agents, skills, permissions, MCP, modele, workflow) dont le [paquet de session](#paquet-de-session) est la forme enregistree ; l'[adaptateur](#adaptateur) la traduit pour opencode. Voir [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.fr.md).

### Skill (Competence)

Un document de protocole Markdown qui fournit des connaissances specifiques a un domaine, des workflows ou des instructions comportementales a un agent. Les skills sont categorises en [Bucket A](#bucket-a-skills-inline) (toujours charges) ou [Bucket B](#bucket-b-skills-natifs) (a la demande). Voir [Architecture des skills](../architecture/skills.fr.md).

### Stack Skills

Protocoles de skills specifiques a un framework (ex. `dev-standards-react`, `dev-standards-golang`) qui sont ajoutes dynamiquement au paquet de session en fonction de la stack technique detectee du projet cible (langages Go, TypeScript, Python, Rust, Java, Ruby ; frameworks Next.js, Nuxt, React, Vue, Express, Django, FastAPI, Rails ; Vitest, Jest, Docker, CI). Situes dans `skills/developer/stacks/`.

### Target Project (Projet cible)

Un codebase enregistre avec OpenHub via `oh project add`. Les sessions y sont lancees avec `oh run <workflow>` ; rien n'est deploye dans le projet (les paquets de session sont dans `~/.oh/bundles/`). Plusieurs projets peuvent etre enregistres simultanement.

### Team-state Repository (Depot d'etat equipe)

Un depot Git partage par les membres de l'equipe pour synchroniser l'etat de collaboration : claims, identites membres, wiki, evenements, policies, patterns et credentials. Cree avec `oh team init`. Voir [Configuration equipe](../guides/team-setup.fr.md).

### TUI (Terminal User Interface)

Le tableau de bord terminal interactif lance en executant `oh` sans arguments. Fournit une navigation visuelle pour les projets, sessions, board equipe, configuration, et plus. Construit avec tview (huh sert encore a quelques invites en ligne, hors TUI). Voir [Usage TUI](../guides/tui-usage.fr.md).

### Mode de Workflow

Fixe au lancement (`--mode`, fiche de lancement), il determine le comportement de chaque [checkpoint](#checkpoint-cp), declare par le workflow pour chaque mode :
- **Manuel** : la plupart des checkpoints s'arretent pour validation
- **Semi-auto** (defaut des workflows livres) : les checkpoints courants passent seuls ; les points de decision cles (ex. `cp-0`, `cp-2`) s'arretent
- **Auto** : tout passe seul sauf les checkpoints qui restent en `pause` (ex. `cp-0` validation du plan, `cp-2` commit)

Un workflow peut restreindre les modes (`modes.allowed`). Voir [schema `oh/v1`](workflow-schema.fr.md#modes).

### Workflow

La description declarative d'un cas d'usage (YAML `apiVersion: oh/v1`) : agent d'entree, agents, checkpoints, entrees, ressources, environnements et limites. 12 workflows livres par le hub (`feature`, `ticket`, `quick`, `cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `sweep`, `brief-enrich`, `libre`), extensibles par l'equipe et le projet (`extends`, la securite ne pouvant que se durcir). Lance par `oh run <workflow>`. Voir [Workflows livres](workflows.fr.md) et [schema `oh/v1`](workflow-schema.fr.md).

### Worktree

Un Git worktree utilise pour isoler les sessions IA en parallele. Chaque worktree obtient sa propre branche et son repertoire de travail, permettant a plusieurs agents de travailler simultanement sans conflits. Gere via `oh worktree` ou `oh run <workflow> --location new`. Voir [Guide worktree](../worktree.md).
