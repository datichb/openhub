> [Read in English](039-declarative-workflows-oh-v1.en.md)

# ADR-039 — Workflows déclaratifs `oh/v1`

## Statut

Accepté

## Date

2026-10-05

## Contexte

Jusqu'à la v4, `oh` n'avait qu'**un seul workflow**, codé en dur (`BaseWorkflow`, `WorkflowDefinition`), que le hub, l'équipe et le projet pouvaient seulement surcharger (surcharges empilées, [ADR-033](./033-config-cascade-enforcement.fr.md)). Les autres usages passaient par des « stratagèmes » :

- **commandes ou options dédiées** : `oh start --dev`, `--onboard`, `oh audit`, `oh review`, `oh debug`, `--sweep`, `--parallel` ;
- **modes A à E** décrits dans le prompt de l'orchestrateur ;
- **modes `manuel` / `semi-auto` / `auto`** de l'orchestrateur dev ([ADR-006](./006-orchestrator-configurable-mode.fr.md)) ;
- **checkpoints `[CP-x]`** écrits dans la prose des agents ([ADR-003](./003-orchestrator-checkpoints.fr.md)) ;
- **carte des agents et des délégations** tenue à la main dans une skill ([ADR-018](./018-hub-workflow-reference.fr.md)).

Rien de tout cela ne pouvait être validé, versionné ni partagé, et tous les agents du hub étaient visibles dans chaque session.

Décisions D5 (workflows en YAML), D6 (orchestration hybride), O1 (profondeur calculée), O7 (sorties typées), O9 (niveau workflow des modèles) et O11 (entrées délimitées).

## Décision

### 1. Format et schéma figé

Un workflow est un document YAML `apiVersion: oh/v1`, `kind: Workflow`. Le schéma est **figé** (`cli/internal/workflow/schema.go`, type `Spec`) ; il n'évolue que par ajouts additifs (champ facultatif, nouvelle valeur d'énumération). Champs :

- **identité** : `id`, `version` (gérée par la publication), `category`, `label`, `description` (textes simples ou par langue) ;
- **héritage** : `extends`, `enforce` ;
- **sécurité** : `risk` (`read` < `plan` < `write` < `publish`), `isolation` (`strict` | `standard`), `code_mode` ;
- **déroulé** : `entry` (`agent`, `selectable`), `inputs`, `prompt` (`template` ou `text`), `agents` (`role`, `mode`, `after`, `calls`), `checkpoints` (`label`, comportement par `mode`, `condition`, `remote`, `mandatory`, `disabled`), `modes`, `circuit_breaker`, `preconditions` ;
- **ressources** : `models`, `skills` (`extra`, `deny`), `plugins`, `mcp`, `beads.allow` ;
- **exécution et limites** : `runtime` (`default`, `allowed`), `outputs`, `limits` (`budget_usd`, `models`).

`inputs`, `agents`, `checkpoints` et `preconditions` sont des cartes ordonnées : l'ordre de déclaration sert à la fiche de lancement et à l'ordre des checkpoints.

Ajouts additifs faits après le gel : `risk: plan` et `preconditions` (workflows livrés), `enforce` (verrous d'une couche, [ADR-040](./040-workflows-team-state-governance.fr.md)), `limits.models` (restrictions I6), `entry.selectable` (workflow `libre`, [ADR-048](./048-opencode-v1-abandonment.fr.md)).

### 2. Lecture stricte et diagnostics

- Champs inconnus, clés en double et valeurs mal typées sont refusés. Toutes les erreurs du fichier sont rapportées, avec ligne et colonne.
- Un diagnostic a la forme `{Severity, Code, Path, Source, Pos, Message, Hint}`, avec des messages traduits (`workflow.diag.<code>`). Il pointe vers le document qui a posé la valeur.
- Commande : `oh workflow validate <fichier|id> [--all] [--json]`.

### 3. Couches et résolution

- Couches : `hub` < `team` < `project` < options de session (jamais enregistrées). Les brouillons sont une source, pas une couche ([ADR-040](./040-workflows-team-state-governance.fr.md)).
- **Patch par `extends`** :
  - c'est la présence d'un champ dans le fichier qui décide du remplacement, pas sa valeur ;
  - les cartes sont fusionnées par clé, les listes et les textes sont remplacés en bloc ;
  - un agent est retiré par `role: disabled`, un checkpoint par `disabled: true`.
- Un workflow de même id dans une couche plus spécifique **doit étendre** celui de la couche inférieure la plus proche (`shadow_without_extends`, `must_extend_nearest`).
- **Champs de sécurité** (`risk`, `isolation`, `runtime.allowed`, checkpoints obligatoires, `beads.allow`, `limits`) : ils ne peuvent que se durcir. Un assouplissement est une **erreur bloquante**, jamais ignoré en silence.
- Champs verrouillés par `enforce:` : une couche plus spécifique qui les écrit est en erreur.
- Chaque valeur résolue garde son **origine** (`oh workflow show <id> --origin`, éditeur de la TUI).

### 4. Validation

En plus du schéma, la validation vérifie notamment :

- ids en kebab-case ;
- agents présents dans le catalogue ;
- graphe sans cycle, et agents `workflow` atteignables depuis l'entrée ;
- agent d'entrée primaire ;
- cibles de `after` et de `calls` valides ;
- variables du gabarit ⊆ entrées déclarées ;
- `risk: read` sans agent qui écrit ni écriture Beads ; `risk: plan` sans modification de fichier, avec `beads.allow` obligatoire et sans `delete` ;
- `remote` permis ⇒ aucun checkpoint qui attend l'utilisateur avec `remote: forbid` ;
- fermeture des skills (`requires:`) résoluble, avec des identifiants uniques ;
- `isolation: strict` ⇒ adaptateur à isolation complète.

### 5. Graphe et profondeur

- Le **graphe de délégation** vient de `calls` quand il est écrit, sinon de la permission `task` de l'agent, restreinte aux membres du workflow.
- L'auto-délégation est possible si elle est explicite (`calls: [<soi-même>]`, sessions `reviewer` parallèles de `review`).
- La **profondeur maximale** du graphe est rendue en `experimental.subagent_depth` (O1 ; opencode V2 limite la délégation à 1 niveau par défaut).

### 6. Orchestration hybride

- Par défaut, l'entrée est **`conductor`**, un agent générique sans écriture ni shell. Il suit la carte du workflow générée et délègue selon `after` et les checkpoints. Un workflow peut aussi désigner un agent d'entrée dédié (`entry.agent`).
- Les skills d'enchaînement sont **générées depuis le YAML** dans le paquet (`workflow/workflow-map`, `orchestrator-workflow-modes`, `hub-workflow-reference`) : elles ne sont plus maintenues à la main.
- `orchestrator.md` ne contient plus de modes A à E.

### 7. Prompts

- Gabarits Go `text/template` (`workflows/prompts/<id>.md.tmpl`), entrées au premier niveau, contexte de session sous `.oh`.
- Les entrées texte passent par `data` : balises `<oh:data name="…">`, balises internes neutralisées, troncature à `max_length` (20 000 caractères par défaut, O11).
  - *Révision du 08/10/2026 (O11, anomalie A27)* : seules les entrées calculées depuis une source externe (`from:`) restent dans des balises ; une entrée saisie par l'utilisateur est sa demande et est écrite telle quelle (tronquée, balises neutralisées). Dans des balises, une demande courte et impérative (« Dis seulement X ») était prise pour une injection.
- Le prompt contient toujours la ligne `Mode de workflow : <mode>`. Il se termine par la liste des checkpoints à signaler avant chaque agent verrouillé.

### 8. Modes et checkpoints

- Chaque checkpoint a un comportement par mode : `pause`, `auto`, `skip` ou `conditional`. `mandatory` interdit un comportement moins strict dans une couche plus spécifique.
- `modes.default` et `modes.allowed` encadrent le choix. Le mode est fixé au lancement et n'est plus demandé par l'agent.
- L'exécution des checkpoints est décrite dans [ADR-042](./042-checkpoints-headless-decisions.fr.md).

### 9. Workflows livrés et commandes

- Livrés par le hub (`workflows/`, embarqués) : `feature`, `ticket`, `cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `quick`, `sweep`, `brief-enrich`, `libre`.
- Lancement : `oh run <workflow> [--input k=v] [--mode] [--runtime] [--location] [--tickets a,b] [--headless] [--recap] [--draft]`, fiche de lancement générée dans la TUI, bloc « Démarrer » (épinglés, récents). Les anciennes commandes sont des alias dépréciés de `oh run`.
- Le **niveau workflow** s'ajoute à la cascade des modèles : workflow·agent > workflow > projet… (O9).
- **Sorties typées** (`branch`, `merge_request`, `beads-ids`, `path`), déclarées par l'agent avec l'outil MCP `workflow_outputs`. Elles proposent « Enchaîner avec… » en fin de session (O7).
- **Préconditions** déclaratives (`path_exists`) : `suggest` propose de lancer d'abord un autre workflow puis de revenir, `block` refuse le lancement.

## Conséquences

### Positives

- Chaque usage est un workflow lisible, validé avant le lancement, versionné et extensible par équipe ou par projet sans toucher au hub.
- Le paquet ne contient que les membres du workflow : monde fermé par usage ([ADR-043](./043-session-bundle-deploy-removal.fr.md)).
- La sécurité ne peut pas être assouplie par une couche plus spécifique ; l'origine de chaque valeur est visible.
- La carte du workflow vue par les agents est toujours celle du YAML.
- Tests golden des paquets et des prompts de chaque workflow livré.

### Négatives / Compromis

- Le schéma figé contraint les évolutions : tout changement incompatible demande une discussion explicite. L'ajout d'`entry.selectable` par la piste 3.E reste à confirmer.
- La restriction des écritures d'un agent ne va pas plus loin que les permissions : « `docs/wiki/` seulement » (`onboarding`) est une consigne de prompt.
- La `condition` d'un checkpoint `conditional` n'est pas évaluée par oh : le checkpoint demande toujours l'utilisateur.
- `enforce:` ne porte que sur les champs de premier niveau.
- Les skills générées sont en français, quelle que soit la langue de la session.
- Les modèles peuvent ignorer les consignes générées : en recette, Haiku n'appelait pas `workflow_checkpoint`, d'où le rappel en fin de prompt. Des gabarits citent encore comme « à charger » des skills qui sont inlinées (anomalie Q3-1).
- Les anciennes surcharges de `hub.toml` sont archivées mais pas chargées : il n'existe pas de couche « hub locale ».

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Garder le workflow unique et ses surcharges empilées (ADR-033) | Un seul enchaînement possible ; les autres usages restent des stratagèmes, et une surcharge ne peut ni ajouter un workflow ni changer l'agent d'entrée. |
| Décrire les workflows en Go dans le binaire | Ni équipe ni projet ne pourraient en créer ; chaque changement demanderait une version d'oh. |
| Garder les modes dans le prompt de l'orchestrateur (modes A à E) | Ni validable ni vérifiable ; tous les agents restent visibles ; le modèle choisit lui-même son chemin. |
| Moteur d'exécution complet dans oh (chaque étape lancée par oh) | Perd la souplesse de l'orchestration par un agent ; oh se limite à contrôler les points de passage (checkpoints, verrous `after`). |
| Couches libres de tout assouplir | Une équipe ou un projet pourrait retirer un checkpoint obligatoire ou élargir `beads.allow` sans que personne ne le voie. |
| TOML ou JSON | Moins lisibles pour des gabarits et des textes longs ; le YAML garde commentaires et ordre, ce qui sert à l'éditeur. |
