> [Read in English](overview.en.md)

# Vue d'ensemble de l'architecture

> Voir aussi : [Glossaire](../reference/glossary.fr.md) pour les définitions des termes, [Guide de migration v5](../guides/migration-v5.fr.md) pour ce qui a changé depuis la v4.

## Vue système

`oh` est la **tour de contrôle**, opencode la **cabine** : `oh` choisit le workflow, construit le paquet de session, démarre un serveur opencode par groupe et suit les sessions ; opencode exécute les agents et sert d'interface de conversation. Fermer la fenêtre d'opencode n'arrête pas la session.

```mermaid
%% OpenHub -- System Overview (v5)
%% oh is the control tower: it resolves a workflow, builds a session bundle,
%% starts one opencode server per group and supervises the sessions through
%% its daemon (credential proxy, gateways, decisions, MCP servers of oh).
%% See ADR-038 to ADR-053.

flowchart LR
    U["fa:fa-user User"] -->|"oh run &lt;workflow&gt;<br/>TUI · oh session …"| CLI["fa:fa-terminal oh<br/>CLI + TUI"]

    subgraph Hub ["~/.oh/ (hub)"]
        TOML["hub.toml"]
        DB["oh.db<br/>projects · sessions · decisions · usage"]
        HC["hub/<br/>agents · skills · workflows"]
    end

    TS[("team-state (git)<br/>team / project workflows<br/>catalog · claims")]

    CLI -->|reads| Hub
    CLI -->|"resolves workflow<br/>(hub &lt; team &lt; project)"| TS
    CLI -->|builds| BUN["~/.oh/bundles/&lt;hash&gt;/<br/>session bundle (immutable)"]
    CLI -->|"Render · Launch · Attest<br/>(opencode V2 adapter)"| OC

    subgraph RT ["Execution environment: local · container · remote"]
        OC["fa:fa-robot opencode serve<br/>one server per group<br/>closed world"]
        S1["session 1"]
        S2["session 2"]
        OC --- S1
        OC --- S2
    end

    BUN -->|"config + agents + skills + plugin"| OC

    subgraph D ["ohd daemon (on the machine)"]
        PX["credential proxy<br/>ohs_ tokens · allow-lists"]
        GW["gateways<br/>Beads · MCP"]
        SUP["supervision<br/>events · decisions · sleep"]
    end

    CLI <-->|socket| D
    OC -->|"LLM calls (session token)"| PX
    PX -->|"real key (keychain)"| LLM["fa:fa-cloud LLM provider<br/>Bedrock · Anthropic · OpenRouter"]
    OC <-->|"SSE events · permissions"| SUP
    OC <-->|"bd · oh MCP servers"| GW
    GW --> BD[("Beads (bd)<br/>on the machine")]

    subgraph MCP ["oh MCP servers"]
        WF["workflow<br/>checkpoints · outputs · reports"]
        WIKI["wiki (project with docs/wiki/)<br/>index · read · propose · check"]
        INT["gitlab · github · figma<br/>jira · linear · gslides · team"]
    end

    OC <-->|"tool calls"| MCP
    CLIENT["opencode client<br/>iTerm2 · Terminal.app · tmux · browser"] -->|"--server · -s &lt;id&gt;"| OC
```

> Source du diagramme : [`docs/diagrams/system-overview.mermaid`](../diagrams/system-overview.mermaid)

## Flux d'une session

```
oh run <workflow> (ou « Démarrer » dans la TUI)
  1. Résolution du workflow     hub < équipe < projet (< brouillon) < options de session
  2. Fiche / --recap            entrées, mode, environnement, emplacement, avertissements
  3. Paquet de session          ~/.oh/bundles/<hash>/ (immuable, partagé par le groupe)
  4. Serveur de groupe          un `opencode serve` par (paquet, projet, environnement)
  5. Vérification               monde fermé (Attest) : sinon la session ne démarre pas
  6. Session                    créée par l'API (identifiant choisi par oh), prompt initial
  7. Ouverture                  iTerm2 → Terminal.app → tmux → navigateur → terminal courant
  8. Suivi et décisions         démon ohd : flux, ⏸ ? ! $ ✗, notifications, veille après 5 min
  9. Fin et suite               sorties relevées par oh, suite proposée (next:), jamais lancée seule
```

Plus rien n'est écrit dans le projet : `oh deploy` et `oh sync` sont supprimés en v5 (ce sont des alias qui expliquent la migration ; les restes des anciens déploiements se retirent avec `oh migrate deploy-cleanup`).

Cycle de vie d'une session : [`session-lifecycle.mermaid`](../diagrams/session-lifecycle.mermaid) ; guide utilisateur : [Sessions v5](../guides/sessions-v5.fr.md).

## Concepts fondamentaux

### Hub

Le **hub** (`openhub`) est le dépôt central qui contient les sources des agents (`agents/`), des skills (`skills/`) et des workflows livrés (`workflows/`). Ils sont embarqués dans le binaire (`internal/hubcontent`) et extraits dans `~/.oh/hub/`. C'est la source de vérité : on édite ici, jamais dans les projets.

### Workflow

Un **workflow** est un fichier YAML déclaratif (`apiVersion: oh/v1`) qui décrit un cas d'usage : agent d'entrée, agents membres et leur ordre (`after`, `calls`), checkpoints et leur comportement par mode (`manuel`, `semi-auto`, `auto`), entrées, sorties typées, ressources (skills, MCP, plugins, Beads autorisés), risque (`read`, `plan`, `write`, `publish`), environnements autorisés et restrictions (`limits`). Le hub en livre 24 : `spec`, `implement-spec`, `ticket`, `quick`, `sweep`, `review`, `review-feedback`, `audit`, `security-fix`, `debug`, `incident`, `spike`, `retro`, `triage`, `benchmark`, `infra-change`, `onboarding`, `doc-sync`, `skill-refresh`, `release`, `brief-enrich`, `libre`, et deux workflows que lance oh dans un fan-out (utilisables aussi à la main), `merge-conflicts` et `sweep-task`. `feature` et `cadrage` ne sont plus des workflows mais des **alias** (`aliases:`) : `oh run feature` (comme `oh start`) et `oh run cadrage` lancent `spec`, `oh run feature --tickets …` lance `implement-spec`. L'agent générique `conductor` sert d'entrée quand aucun agent dédié n'est indiqué (`spec`, `sweep`, `security-fix`).

Les workflows se résolvent **par couches** : hub, puis équipe et projet (dans le dépôt team-state), puis options de session. Une couche plus spécifique étend la précédente (`extends`) ; la sécurité ne peut que se durcir, et une couche peut verrouiller des champs (`enforce`). Chaque valeur résolue garde son origine (`oh workflow show --origin`).

Voir [Workflows livrés](../reference/workflows.fr.md), [Workflows d'équipe](../guides/team-workflows.fr.md), [carte des workflows](../diagrams/workflow-scenarios-map.mermaid) et [résolution par couches](../diagrams/config-resolution.mermaid). Décisions : [ADR-039](./adr/039-declarative-workflows-oh-v1.fr.md), [ADR-040](./adr/040-workflows-team-state-governance.fr.md).

### Composition des workflows

Un workflow ne lance jamais un autre workflow. Il déclare ses **sorties**, que oh **relève lui-même** quand il le peut (`record:` : tickets créés ou fermés par la passerelle Beads, spec vérifiée à un checkpoint, dernier rapport déposé, fichiers écrits sous des chemins, entrée du lancement), et les workflows qui peuvent le suivre (`next:`, avec une condition `when: { outputs, equals }` et des entrées `session.output(<sortie>)`). À la fin de la session, oh **propose** la suite, sans la lancer : à la fermeture du client (`oh run`), avec `oh session next <id>` et en tête de « Enchaîner avec… » dans la TUI ; la suite lancée avec `--parent <session>` lit les sorties de cette session. Enchaînements livrés : `spec` → `implement-spec` (tickets, spec, méthode), `audit` (`security`) → `security-fix` (contenu du rapport), `spike` (verdict `go`) et `incident` → `spec` (page d'origine `source`), `incident` → `ticket` (tickets des actions).

```mermaid
%% OpenHub -- Workflow composition (v5, R5 and branchements)
%% A workflow declares the workflows that may follow it (`next:`): oh proposes
%% them when their condition holds (`when: outputs`, `equals`) and fills their
%% inputs from the outputs it recorded itself (`record:`), never from what an
%% agent declares. oh never launches the next workflow: it is offered at the end
%% of `oh run`, by `oh session next <id>` and first in "Chain with…" of the TUI,
%% and started with `--parent <session>`.

flowchart LR
    AUD["<b>audit</b><br/>records findings (report)<br/>and type (input)"]
    SECFIX["<b>security-fix</b><br/>findings = content of the report<br/>records deferred (tickets created)"]
    INC["<b>incident</b><br/>records postmortem (written)<br/>and actions (tickets created)"]
    SPIKE["<b>spike</b><br/>records decision (written)<br/>and verdict (written, match)"]
    SPEC["<b>spec</b> (aliases cadrage, feature)<br/>records spec (checked spec),<br/>tickets (created), method (input)"]
    IMPL["<b>implement-spec</b><br/>records tickets (closed)"]
    TICK["<b>ticket</b>"]
    OH{{"oh proposes, never launches:<br/>end of oh run · oh session next · TUI"}}

    AUD ==>|"type = security<br/>findings"| SECFIX
    SPIKE ==>|"verdict = go<br/>source = decision"| SPEC
    INC ==>|"source = postmortem"| SPEC
    INC ==>|"ticket = actions"| TICK
    SPEC ==>|"tickets, spec, method"| IMPL
```

> Source du diagramme : [`docs/diagrams/workflow-composition.mermaid`](../diagrams/workflow-composition.mermaid)

Voir [Composition](../reference/workflow-schema.fr.md#composition-next-et-aliases), [Sorties relevées par oh](../reference/workflow-schema.fr.md#sorties-relevées-par-oh-record) et la [carte des workflows](../diagrams/workflow-scenarios-map.mermaid).

### Paquet de session

À chaque lancement, `internal/bundle` construit le **paquet de session** `~/.oh/bundles/<hash>/` à partir du workflow résolu : agents membres (skills Bucket A intégrées), skills à la demande (Bucket B, skills de stack, `skills.extra`, skills générées depuis le YAML comme `workflow/workflow-map` ; chaque agent n'en voit que son catalogue, voir [Catalogue par agent](skills.fr.md#catalogue-par-agent)), permissions, graphe de délégation et profondeur maximale, serveurs MCP (dont les serveurs `workflow` et `wiki` d'oh) et plugin oh. Chaque agent y reçoit un **contrat de session** généré (son rôle, ses appelants et les agents qu'il peut lancer, ses checkpoints, ses écritures, l'outil `question` ou non : voir [Agents](agents.fr.md#les-agents-dans-une-session-v5)), le texte filtré pour son rôle (`when:`, voir [Variantes par rôle](skills.fr.md#variantes-par-rôle-et-par-session-when)) et les instructions du projet à son niveau (`full`, `index`, `none` : voir [Instructions du projet par niveau](agents.fr.md#instructions-du-projet-par-niveau)). Le paquet est **immuable** et adressé par son hash : les sessions d'un même groupe le partagent, une reprise repart du même paquet.

Commandes : `oh bundle show <workflow> [--budget]`, `oh bundle build <workflow>`. Décision : [ADR-043](./adr/043-session-bundle-deploy-removal.fr.md).

### Monde fermé

Une session ne voit **que** les agents et skills de son paquet : agents natifs d'opencode désactivés, skills intégrées refusées, agents hors du workflow invisibles. Ce n'est pas réglable. L'adaptateur le vérifie à chaque démarrage de serveur (`Attest` : agents, skills et MCP réellement visibles, règles effectives par agent) ; en cas d'écart, la session ne démarre pas. Décision : [ADR-041](./adr/041-closed-world-isolation.fr.md).

### Groupe de serveur

Un **groupe** = (version du paquet, projet, environnement d'exécution). Chaque groupe a son propre `opencode serve`, partagé par ses sessions ; chaque session a son emplacement (dossier du projet ou worktree), son environnement, ses règles de permissions et son prompt. Plusieurs tickets lancés ensemble (`oh run ticket --tickets a,b`) donnent N sessions dans un seul serveur, avec un worktree par session qui écrit. Un serveur inactif sans décision en attente se met en veille après 5 minutes ; la reprise redémarre le serveur sur le même paquet.

### Adaptateur d'outil

La logique reste dans `oh` ; un **adaptateur** par outil traduit un modèle neutre (`SessionSpec`, `BundleSpec`) dans le format de l'outil et pilote son serveur : `Render`, démarrage du serveur, création de session par l'API, `Attest`, événements, décisions, contrôle (interrompre, changer de modèle, compacter, fork), résultats, export/import. Seul l'adaptateur **opencode V2** existe (`internal/adapters/opencodev2`) ; opencode V1 n'est plus pris en charge. Rien de propre à un outil (nom, identifiants de fournisseur, variables, API, chemins, agents natifs) n'apparaît hors de son adaptateur : le reste d'oh passe par l'interface et des capacités neutres, une seule racine de composition (`cmd/v5_adapters.go`) choisit l'adaptateur, et un test d'architecture (`internal/archtest`) l'impose (D19). Décisions : [ADR-038](./adr/038-sessionspec-tool-adapters.fr.md) (remplace en partie l'[ADR-036](./adr/036-platform-abstraction-layer.fr.md)), [ADR-048](./adr/048-opencode-v1-abandonment.fr.md), [ADR-049](./adr/049-tool-independence-architecture-guard.fr.md).

### Démon `ohd`

Le démon (`oh daemon status|stop`, lancé à la demande) tourne sur la machine et héberge :

- le **proxy d'identifiants LLM** : opencode ne reçoit qu'un jeton de groupe (`ohs_…`) ; la vraie clé reste dans le trousseau ; liste blanche des chemins d'inférence et des modèles, signature SigV4 pour les profils AWS, comptage de l'usage ;
- la **supervision** des sessions : flux d'événements SSE, décisions en attente, état, notifications système, mise en veille, réapplication de l'environnement de session aux sous-sessions ;
- les **passerelles** : passerelle Beads (faux `bd`, dans tous les environnements) et, hors machine, passerelle MCP HTTP pour les serveurs MCP d'oh du paquet ;
- les **restrictions** optionnelles (désactivées par défaut) : sessions actives max, budget par session et journalier, plafond mémoire, liste de modèles (`oh budget`).

Sous Windows, le démon tourne dans le processus `oh`. Décisions : [ADR-044](./adr/044-credential-proxy-session-limits.fr.md), [ADR-047](./adr/047-session-interaction-daemon.fr.md).

### Checkpoints et décisions

Un **checkpoint** est un point de contrôle du workflow (`cp-1`, `cp-2`…), avec un comportement par mode (`pause`, `auto`, `skip`, `conditional`). Il est tenu à trois niveaux : le prompt généré (carte du workflow), l'outil MCP `workflow_checkpoint` en permission `ask` (validé par l'API des permissions) et le plugin oh. La machine à états est dans `oh` (`CheckpointService`) : les agents verrouillés par `after:` restent refusés tant que le checkpoint n'est pas passé, et un coupe-circuit arrête les délégations en boucle.

oh ne croit pas l'agent sur parole :

- **Déverrouillages** (`unlocks:`) : `commit`, `close`, `create`, `apply`, `tag`, `mr` sont refusés à tous les agents tant que le checkpoint qui les ouvre n'est pas passé, et la fenêtre se referme ensuite (au checkpoint obligatoire suivant, ou une fois le travail commité ; `unlock_again` rouvre le commit une seconde fois, dans `incident`). Aucun workflow livré ne déverrouille `push`.
- **Vérification par oh** (`verify:`) : avant d'ouvrir les déverrouillages, oh lance lui-même le build, les tests et le typecheck de la stack (ou une commande du workflow), vérifie une spec (`verify.spec` : glossaire, points de test, découpage) ou relève et vérifie un découpage (`tasks:` de `sweep`) ; un échec refuse le checkpoint. Les preuves de l'agent (`evidence`) sont recoupées avec les commandes observées.
- **Ordre et auteur** : `after:` sur un checkpoint impose l'ordre (`spec`), `by:` dit quel agent le passe, et le budget de pauses (CP-R2) borne ce qui arrête l'utilisateur en semi-auto.
- **Écritures déclarées** (`agents.<id>.writes`) : un agent en lecture seule écrit et commite seul ses chemins de documentation (la spec du planner, le postmortem du debugger, la décision du pathfinder), après le checkpoint qui les ouvre (`writes_after`).

La carte de décision montre la vérification, les preuves, les rapports déposés et les propositions du wiki. Voir [Checkpoints](../reference/checkpoints.fr.md).

Les décisions (⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur) se prennent depuis n'importe quel client : vue Sessions et boîte « À traiter » de la TUI, `oh session approve|answer|dismiss`, interface opencode ou navigateur. La première réponse gagne. Décision : [ADR-042](./adr/042-checkpoints-headless-decisions.fr.md) (remplace l'[ADR-003](./adr/003-orchestrator-checkpoints.fr.md)).

### Rapports et brouillons

Un agent lancé par un autre rend une **enveloppe** courte et dépose son rapport complet dans oh avec `workflow_report` (`rp-<n>`, auteur fixé par oh, borné, vérifié par empreinte) ; son appelant le lit avec `workflow_report_get`, la carte du checkpoint le montre et `oh session report` le donne. Un agent peut aussi recevoir un **brouillon de session** (`agents.<id>.scratch`), un dossier hors du dépôt que seuls lui et ses lecteurs déclarés voient (la reproduction du debugger dans `debug`). Voir [Délégation inter-agents](task-delegation.fr.md#enveloppe-de-retour-et-rapport-déposé-l5) et le [schéma](../reference/workflow-schema.fr.md#scratch-brouillons-de-session).

### Wiki documentaire vivant

Un projet doté de `docs/wiki/` reçoit le serveur MCP `wiki` (`wiki_index`, `wiki_read`, `wiki_propose`, `wiki_pending`, `wiki_check`, `wiki_apply`). Les agents **proposent** leurs découvertes (`wiki_propose`) au lieu d'écrire le wiki ; l'utilisateur accepte, modifie ou refuse chaque proposition sur la carte du checkpoint de commit (`cp-wiki` d'`onboarding`, `cp-sync` de `doc-sync`, `cp-2`, `cp-commit`…), et oh applique les acceptées. `wiki_check` vérifie le wiki et calcule la dérive d'un changement (axe documentation du reviewer). Voir [Wiki documentaire vivant](living-wiki.fr.md).

### Fan-out (expérimental)

Avec `[fanout] experimental = true`, `implement-spec -i fanout=true` lance une session enfant `ticket` par ticket et `sweep -i fanout=true` une session enfant `sweep-task` par sous-tâche relevée, chacune dans sa branche et son worktree. oh les pilote (`oh fanout drive`), les merge dans une branche d'intégration (le `merger` sur un conflit qu'il ne résout pas), puis le `cp-merge` obligatoire de la parente, vérifié par oh sur cette branche, remplace le checkpoint de commit des enfants (substitution de contrôle, CP-R3). Budget global, tour de correction et nettoyage des branches `oh/fanout/…` : voir [Fan-out](../guides/fan-out.fr.md) et l'[ADR-053](./adr/053-fanout-control-substitution.fr.md).

```mermaid
%% OpenHub -- Fan-out (experimental, ADR-053)
%% `-i fanout=true` with `[fanout] experimental = true`: implement-spec (one
%% `ticket` child per ticket) and sweep (one `sweep-task` child per subtask
%% recorded by oh at cp-plan). oh drives the children, merges them and checks
%% the result; the commit checkpoint of the children is replaced by the
%% mandatory cp-merge of the parent (control substitution, CP-R3). No session
%% of the fan-out pushes, merges, closes a ticket or moves an oh/fanout branch.

flowchart TD
    P["<b>parent session</b><br/>implement-spec or sweep · -i fanout=true"]
    D["<b>oh fan-out driver</b><br/>oh fanout drive · oh fanout status"]
    I[("integration branch<br/>oh/fanout/&lt;session&gt;/integration")]
    C1["<b>child session</b> ticket / sweep-task<br/>own branch and worktree"]
    C2["<b>child session</b> …<br/>1 to 3 at a time, after its dependencies"]
    M["<b>merge-conflicts</b><br/>merger, result checked by oh"]
    R{"<b>cp-merge</b> (mandatory, pause in every mode)<br/>verification by oh on the integration branch"}
    FIX["fix round (fanout.fix)<br/>quick session, role fixer,<br/>its cp-commit approved by the user"]
    F["<b>cp-final</b> / <b>cp-recap</b><br/>then oh cleans the oh/fanout/… worktrees and branches"]

    P -->|"cp-plan passed (tickets or recorded tasks)"| D
    D -->|"creates"| I
    D -->|"launches"| C1
    D -->|"launches"| C2
    C1 -->|"commit checkpoint without pause (delegated),<br/>verified by oh, commit"| D
    C2 --> D
    D -->|"merge --no-ff"| I
    D -.->|"conflict oh does not resolve"| M
    M -.-> I
    D -->|"all children done: table to the parent,<br/>review of the whole by the reviewer"| R
    R -->|"approved: oh merges into the session branch<br/>and closes the merged tickets"| F
    R -.->|"refused with an instruction"| FIX
    FIX -.->|"cp-merge again (limits.max_fix_rounds)"| R
    D -.->|"global budget [fanout] budget_usd"| P
```

> Source du diagramme : [`docs/diagrams/fanout-flow.mermaid`](../diagrams/fanout-flow.mermaid)

### Environnements d'exécution

| Environnement | Où tourne opencode | Ce qui reste sur la machine |
|---|---|---|
| ⌂ local | processus `opencode serve` sur la machine | tout |
| ▣ conteneur | un conteneur par groupe (Colima, Podman ou Docker CLI) ; image = Dockerfile de dev du projet + couche oh ; paquet monté en lecture seule | clés (proxy), Beads (passerelle), serveurs MCP d'oh (passerelle) |
| ☁ distant | job GitLab CI du projet `oh-runner` (`oh runner run`) | clés de la machine ; Beads : instantané en entrée, journal en sortie rejoué localement (`oh session resolve`) |

L'environnement se choisit au lancement (`--runtime`, sinon config du projet, Réglages, workflow), dans la limite de `runtime.allowed`. Voir [topologie](../diagrams/execution-topology.mermaid), [Conteneur](../guides/container.fr.md), [Runners distants](../guides/remote-runners.fr.md). Décisions : [ADR-045](./adr/045-execution-environments.fr.md), [ADR-046](./adr/046-beads-gateways.fr.md).

### team-state

Le **team-state** est le dépôt git partagé de l'équipe : membres, policies, claims, événements, wiki, et désormais les workflows d'équipe et de projet (`workflows/published`, brouillons, historique, `workflows.lock`) et le catalogue de briques d'équipe (`catalog/`). Tout membre peut publier (brouillon → publication validée, file hors ligne). Un projet sans équipe a un **espace solo** (team-state local sans remote, `oh team init --solo`, promouvable par `oh team promote`). Décision : [ADR-040](./adr/040-workflows-team-state-governance.fr.md).

### Agent

Un **agent** est un fichier Markdown (`agents/<famille>/<id>.md`) qui définit l'identité d'un rôle : qui il est, ce qu'il fait, ce qu'il ne fait pas, ses permissions et ses skills. 18 agents en 9 familles (planning, developer, auditor, infra, delivery, quality, design, documentation, utility). Voir [agents.fr.md](./agents.fr.md).

### Skill

Un **skill** est un bloc de protocole injectable (format de rapport, checklist, règles). Deux chemins de livraison :

| Chemin | Champ frontmatter | Quand chargé |
|--------|------------------|-------------|
| **Bucket A — Inline** | `skills: [...]` | Toujours — intégré au corps de l'agent à la construction du paquet |
| **Bucket B — À la demande** | `native_skills: [...]` | Le modèle le charge depuis les `skills/` du paquet via l'outil `skill` |

Voir [skills.fr.md](./skills.fr.md), [flux de livraison](../diagrams/skill-injection-flow.mermaid), [ADR-001](./adr/001-agent-skill-separation.fr.md), [ADR-010](./adr/010-hybrid-skills-architecture.fr.md).

### Serveur MCP

Les **serveurs MCP** d'oh sont des implémentations Go natives (`cli/internal/mcp/`), lancées par `oh mcp serve <nom>` (stdio JSON-RPC) : `figma`, `gitlab`, `gslides`, `github`, `jira`, `linear`, `team`, `workflow` (état, checkpoints, sorties et rapports, ajouté à tout paquet de workflow) et `wiki` (wiki du projet, ajouté quand le projet a un `docs/wiki/`). Les serveurs activés pour le projet sont placés dans le paquet au lancement ; le champ `mcp:` d'un workflow les filtre. Hors machine, ils sont servis par la passerelle MCP du démon. Les jetons restent dans le trousseau de la machine.

### Ajout de briques

Les agents et les skills hors hub viennent du catalogue d'équipe (`catalog/` du team-state, ou d'un espace solo), versionné et partagé ; `oh brick` l'édite et le publie avec la gouvernance des workflows et le scelle dans `catalog.lock` ([ADR-054](./adr/054-team-brick-catalogue.fr.md)), `oh skill check` le contrôle. L'ancien registre de skills communautaires (`oh skill add|list|remove|search`, `~/.oh/skills/`) est déconnecté en v5 ([ADR-051](./adr/051-community-skills-disconnection.fr.md)).

### Observabilité

Le registre des sessions (`oh.db` : sessions, décisions, usage par session et par jour) alimente `oh metrics`, `oh serve`, la vue Sessions et `oh session results` (coût, tokens, modèle, fichiers modifiés, branche, description de MR). La table `agent_events` (une ligne par agent d'une session : agent d'entrée et chaque sous-agent, avec statut, durée, tokens, coût et skills chargées) est alimentée par le démon et donne le tableau par agent de `oh metrics`, `oh serve` et de la vue Métriques.

---

## Exemple — `oh run feature` : `spec` puis `implement-spec`

`feature` est un alias : la première session (`spec`) écrit la spec et crée les tickets, la seconde (`implement-spec`), proposée par oh, les implémente. Le mode est fixé au lancement (`--mode` ou fiche) ; chaque checkpoint passe par `workflow_checkpoint`, et oh décide s'il attend l'utilisateur.

```mermaid
sequenceDiagram
    participant U as Utilisateur (oh ou opencode)
    participant OH as oh (CheckpointService)
    participant C as conductor
    participant PL as planner
    participant OD as orchestrator-dev
    participant DEV as developer
    participant R as reviewer

    U->>OH: oh run feature -m "…" (alias de spec, semi-auto)
    OH->>C: session spec
    C->>PL: Entretien (grilling, questions bloquantes)
    PL-->>C: Enveloppe : reformulation, vocabulaire (rapport rp-1)
    C->>OH: workflow_checkpoint cp-understanding
    OH->>U: ⏸ cp-understanding — compréhension validée ?
    U-->>OH: Valider
    Note over OH: writes_after : la spec ne s'écrit qu'après cp-understanding
    C->>PL: Écrire la spec (docs/wiki/specs/<slug>.md) et le découpage
    C->>OH: workflow_checkpoint cp-converge (vérifié par oh, sans pause)
    C->>OH: workflow_checkpoint cp-spec (obligatoire, verify.spec)
    OH->>U: ⏸ cp-spec — spec validée ? (déverrouille commit)
    U-->>OH: Valider
    C->>PL: Commiter la spec seule
    C->>OH: workflow_checkpoint cp-tickets (obligatoire)
    OH->>U: ⏸ cp-tickets — découpage validé ? (déverrouille create)
    U-->>OH: Valider
    C->>PL: Créer les tickets
    Note over OH: sorties relevées : spec, tickets, method
    OH->>U: Suite proposée : implement-spec (oh session next)
    U->>OH: oh run implement-spec --parent <session spec>
    OH->>OD: session implement-spec (tickets et spec relevés)
    OD->>OH: workflow_checkpoint cp-plan (obligatoire)
    OH->>U: ⏸ cp-plan — ordre des tickets ?
    U-->>OH: Valider
    loop Pour chaque ticket
        OD->>DEV: Implémentation (tests inclus), après cp-1
        DEV-->>OD: Enveloppe (rapport rp-…)
        OD->>R: Review
        R-->>OD: Verdict
        OD->>OH: workflow_checkpoint cp-2 (obligatoire)
        Note over OH: verify : build, tests, typecheck de la stack
        OH->>U: ⏸ cp-2 — commit ou correction ? (déverrouille commit, close)
        U-->>OH: Valider / Corriger d'abord / Autre consigne
    end
    OD->>R: Review de l'ensemble des commits
    OD->>OH: workflow_checkpoint cp-final
```

Avec `-i fanout=true` (expérimental), chaque ticket devient une session enfant et un seul `cp-merge` remplace les `cp-2` (voir [Fan-out](#fan-out-expérimental)). Détail des deux workflows : [Workflows livrés](../reference/workflows.fr.md#spec).

---

## Principes de design

### 1. Séparation identité / protocole

L'agent définit **qui** il est, le skill définit **comment** il travaille. Bucket A (toujours actif, inline) pour les protocoles obligatoires, Bucket B (à la demande) pour les contextes de domaine.

→ [ADR-001](./adr/001-agent-skill-separation.fr.md), [ADR-010](./adr/010-hybrid-skills-architecture.fr.md)

### 2. Le workflow est l'unité de lancement

L'enchaînement des agents, les checkpoints et les ressources sont déclarés dans le workflow, pas codés dans les agents. Un agent ne connaît que les agents de son workflow.

→ [ADR-039](./adr/039-declarative-workflows-oh-v1.fr.md)

### 3. Monde fermé, vérifié

Le modèle ne voit que le paquet de sa session ; c'est vérifié à chaque démarrage.

→ [ADR-041](./adr/041-closed-world-isolation.fr.md), [ADR-043](./adr/043-session-bundle-deploy-removal.fr.md)

### 4. Checkpoints explicites, décidés par oh

Les étapes critiques demandent une décision de l'utilisateur selon le mode ; la machine à états est dans `oh`, pas dans le prompt, et les agents verrouillés ne peuvent pas être appelés avant leur checkpoint. Ce qui peut l'être, oh le vérifie lui-même plutôt que de croire l'agent : tests avant un commit, spec et découpage, sorties relevées, merges d'un fan-out.

→ [ADR-042](./adr/042-checkpoints-headless-decisions.fr.md)

### 5. Les secrets et Beads restent sur la machine

opencode ne reçoit jamais de clé LLM ni de jeton d'intégration ; Beads n'est jamais copié dans un conteneur ou un job (passerelle, instantané et journal).

→ [ADR-044](./adr/044-credential-proxy-session-limits.fr.md), [ADR-046](./adr/046-beads-gateways.fr.md), [ADR-019](./adr/019-agent-security-model.fr.md)

### 6. La logique reste dans oh

CLI, TUI et futur `oh serve` passent par les mêmes services (`internal/services/…`, `runsvc`, `daemon`) ; l'outil d'IA est derrière un adaptateur.

→ [ADR-038](./adr/038-sessionspec-tool-adapters.fr.md)

### 7. Séparation des responsabilités de qualité

Implémenter et diagnostiquer sont confiés à des agents différents (developer, debugger) ; les tests sont écrits par le developer et vérifiés par le reviewer. Les agents d'audit, de revue et de design n'écrivent pas dans le projet. Le wiki (`docs/wiki/`) est créé par l'`onboarder` et tenu à jour par le `documentarian` (`doc-sync`) ; les autres agents proposent (`wiki_propose`) et oh applique ce que l'utilisateur accepte. Un agent en lecture seule n'écrit que les chemins de documentation que son workflow lui ouvre (`writes:`) (voir [Wiki documentaire vivant](./living-wiki.fr.md)).

→ [ADR-004](./adr/004-qa-debugger-separation.fr.md), [ADR-013](./adr/013-developer-agent-consolidation.fr.md)

---

## Décisions d'architecture v5

| ADR | Décision | Remplace / fait évoluer |
|---|---|---|
| [038](./adr/038-sessionspec-tool-adapters.fr.md) | Modèle neutre `SessionSpec` et adaptateurs d'outil | remplace en partie 036 (`platform` garde `Credentials` et `StatsProvider`) |
| [039](./adr/039-declarative-workflows-oh-v1.fr.md) | Workflows déclaratifs `oh/v1` | remplace 006, 018 |
| [040](./adr/040-workflows-team-state-governance.fr.md) | Workflows dans le team-state, gouvernance, espace solo | fait évoluer 024, 029, 033 |
| [041](./adr/041-closed-world-isolation.fr.md) | Monde fermé et vérification d'isolation | fait évoluer 019 |
| [042](./adr/042-checkpoints-headless-decisions.fr.md) | Checkpoints à trois niveaux et décisions sans interface | remplace 003 ; fait évoluer 019 (coupe-circuit) |
| [043](./adr/043-session-bundle-deploy-removal.fr.md) | Paquet de session et suppression du déploiement par projet | remplace 011 ; fait évoluer 008, 010, 012 |
| [044](./adr/044-credential-proxy-session-limits.fr.md) | Proxy d'identifiants LLM et restrictions des sessions | fait évoluer 019, 021, 033 |
| [045](./adr/045-execution-environments.fr.md) | Environnements d'exécution : local, conteneur, distant | — |
| [046](./adr/046-beads-gateways.fr.md) | Beads sur la machine et passerelles | — |
| [047](./adr/047-session-interaction-daemon.fr.md) | Interaction avec les sessions, multi-session, démon `ohd` | fait évoluer 012 (worktree automatique) |
| [048](./adr/048-opencode-v1-abandonment.fr.md) | Abandon d'opencode V1 | déprécie 014 |
| [049](./adr/049-tool-independence-architecture-guard.fr.md) | Indépendance vis-à-vis de l'outil et garde-fou d'architecture | fait évoluer 038 |
| [050](./adr/050-session-context-capability.fr.md) | État de session évolutif par capacité de l'adaptateur | fait évoluer 047 |
| [051](./adr/051-community-skills-disconnection.fr.md) | Déconnexion du registre de skills communautaires | remplace en partie 027 (marketplace de skills) |
| [052](./adr/052-beads-identity-provenance.fr.md) | Identité et provenance Beads : `--readonly`, auteur, événements de provenance | fait évoluer 046 |
| [053](./adr/053-fanout-control-substitution.fr.md) | Fan-out : sessions enfants pilotées par oh et substitution de contrôle (CP-R3) | — |
| [054](./adr/054-team-brick-catalogue.fr.md) | Catalogue des briques d'équipe : `oh brick`, gouvernance et intégrité (`catalog.lock`) | fait évoluer 040, suite de 051 |

Tous les ADR : [`docs/architecture/adr/`](./adr/).

---

## Structure des fichiers

```
openhub/
├── agents/              ← Définitions des rôles (18 agents, 9 familles)
├── skills/              ← Protocoles : Bucket A (inline) + Bucket B (à la demande)
├── workflows/           ← Workflows livrés (oh/v1) + gabarits de prompt
├── cli/                 ← Binaire Go (oh)
│   ├── cmd/             ← Commandes Cobra et câblage de la TUI
│   └── internal/
│       ├── workflow/    ← Schéma oh/v1, résolution par couches, validation, rendu des prompts
│       ├── services/    ← Services partagés CLI/TUI : workflow, session, checkpoint, report, wiki, fanout, remote
│       ├── bundle/      ← Construction des paquets de session (~/.oh/bundles/<hash>/)
│       ├── bricks/      ← Briques : assemblage des agents, skills, permissions, cascade des modèles
│       ├── sessionspec/ ← Modèle neutre (SessionSpec, BundleSpec)
│       ├── adapters/    ← Interface d'adaptateur + opencodev2 (rendu, serveur, Attest, plugin)
│       │   └── opencodev2/deploycleanup/ ← Nettoyage des anciens déploiements
│       ├── runsvc/      ← Lancement : groupes de serveurs, sessions, worktrees, veille, reprise
│       ├── daemon/      ← Démon ohd : supervision, décisions, flux, notifications
│       ├── credproxy/   ← Proxy d'identifiants LLM
│       ├── gateway/     ← Passerelles Beads et MCP
│       ├── runtime/     ← Environnements d'exécution (local, container)
│       ├── remote/      ← Contrat machine ↔ job GitLab CI, pipeline généré, client GitLab
│       ├── limits/      ← Restrictions des sessions (I6)
│       ├── teamstate/   ← Dépôt team-state (claims, workflows, catalogue, solo)
│       ├── mcp/         ← Serveurs MCP natifs (dont workflow, wiki et team)
│       ├── storage/     ← SQLite (oh.db), trousseau, chiffrement de fichiers
│       ├── tui/         ← TUI tview/tcell (shell, vues, widgets)
│       ├── evalrun/     ← Évaluations du comportement avec un vrai modèle (oh eval)
│       └── …            ← beads, config, i18n, prompt, tracker, worktree, termlaunch…
├── evals/               ← Cas d'évaluation (oh eval, make test-eval)
├── docs/                ← Documentation (bilingue fr/en)
└── ~/.oh/               ← Données utilisateur (hors dépôt)
    ├── hub.toml · oh.db ← Configuration et registre
    ├── hub/             ← Contenu du hub extrait (agents, skills, workflows)
    ├── bundles/<hash>/  ← Paquets de session (immuables)
    ├── sessions/<id>/   ← Environnement statique, restrictions, résultats, rapports, brouillons, artefacts distants
    ├── servers/<groupe>/ ← Données opencode du groupe (XDG_DATA_HOME), URL du proxy
    └── teams/<id>/      ← Espaces solo
```

**Plateformes :** macOS et Linux (amd64, arm64) ; Windows en local uniquement (démon dans le processus oh, pas de conteneur ni de distant).
