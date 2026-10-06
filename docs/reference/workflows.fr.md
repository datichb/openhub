# Workflows livrés par le hub

> v5 : chaque cas d'usage est un workflow déclaratif (`apiVersion: oh/v1`) livré par le hub. Les fichiers sont dans `workflows/` à la racine du dépôt, embarqués dans le binaire et extraits dans `~/.oh/hub/workflows/`. Une équipe ou un projet peut les étendre (`extends`) à partir de la phase 2.

Vérifier les workflows du hub :

```bash
oh workflow validate --all          # tous les workflows du hub
oh workflow validate ticket         # un workflow du catalogue
oh workflow validate ./mon-wf.yaml  # un fichier
```

On lance un workflow avec `oh run <workflow>` ou la fiche de lancement de la TUI ; les anciennes commandes (`oh start`, `oh audit`…) en sont des alias dépréciés.

---

## Vue d'ensemble

| Workflow | Agent d'entrée | Agents | Risque | Exécution | Remplace |
|---|---|---|---|---|---|
| `ticket` | `orchestrator-dev` | `developer`, `developer-refactor`, `developer-migrator`, `reviewer`, `documentarian` (à la demande) | write | local, conteneur, distant | `oh start --dev`, `-t` |
| `feature` | `orchestrator` | `pathfinder`, `planner`, `designer`, `orchestrator-dev`, `developer*`, `reviewer`, `documentarian` (à la demande) | write | local | `oh start` (anciens modes A, B et E) |
| `quick` | `developer` | — | write | local | session rapide |
| `cadrage` | `conductor` | `pathfinder`, `planner`, `designer` | plan | local | ancien mode A sans implémentation |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, distant | `oh start --onboard` (ancien mode C) |
| `review` | `reviewer` | — | read | local, distant | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, distant | `oh audit` |
| `debug` | `debugger` | `developer` (à la demande) | write | local | `oh debug` (ancien mode D) |
| `sweep` | `conductor` | `developer`, `developer-refactor`, `developer-migrator` | write | local | `oh start --sweep` |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | au choix (`orchestrator` par défaut) | ceux que l'agent choisi peut appeler | write | local, conteneur | `oh start --agent`, session libre de la TUI |

### Niveaux de risque

| `risk` | Fichiers | Beads |
|---|---|---|
| `read` | aucune modification, shell restreint | lecture seule ; `beads.allow` obligatoire, sans commande d'écriture |
| `plan` | aucune modification, shell restreint | `beads.allow` obligatoire ; écritures permises sauf `delete` |
| `write` | modifiés | sans restriction si `beads` est absent |
| `publish` | modifiés, branches poussées, MR ouvertes | idem |

Une couche supérieure (équipe, projet) peut seulement durcir le risque : `read < plan < write < publish`.

### Préconditions

Un workflow peut déclarer des tests faits avant le lancement, dans l'emplacement de la session :

```yaml
preconditions:
  project-context:
    label: { fr: Aucun contexte projet trouvé, en: No project context found }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md, CONVENTIONS.md] }  # l'un des chemins suffit
    on_fail: suggest          # suggest (défaut) | block
    suggest: { workflow: onboarding, resume: true }
```

- `on_fail: suggest` : oh affiche le message et propose le workflow `suggest.workflow` ; l'utilisateur peut aussi continuer. Avec `resume: true`, oh propose de relancer le workflow d'origine (mêmes entrées) une fois le workflow proposé terminé.
- `on_fail: block` : le lancement est refusé.
- En distant ou sans interface : un échec `suggest` devient un avertissement, un échec `block` arrête la session.
- Chemins relatifs, sans `..`. Un patch (`extends`) modifie une précondition par son identifiant ou la retire (`disabled: true`).

`feature` et `cadrage` proposent `onboarding` quand le projet n'a ni wiki, ni `ONBOARDING.md`, ni `CONVENTIONS.md`.

---

## `ticket`

Implémente un ou plusieurs tickets Beads déjà détaillés : routage vers le bon développeur, pre-review, review, commit.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` (sélecteur : `ai-delegated`, multi-sélection) | oui | Ticket à implémenter ; plusieurs tickets sélectionnés = une session par ticket |
| `instructions` | `text` (4 000 caractères max) | non | Précisions pour la session |

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| `cp-1` Démarrer le ticket | pause | auto | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause | attend l'utilisateur |
| `cp-3` Ticket suivant | pause | auto | auto | auto |

Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Sorties : `branch`, `tickets`.

## `feature`

Réalise une feature de bout en bout. L'orchestrator choisit l'agent de planning : `pathfinder` pour une feature simple ou exploratoire, `planner` pour une feature à découper (création des tickets) ou pour classer des tickets existants. Il passe ensuite la main à `orchestrator-dev` pour l'implémentation.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | La feature en langage naturel, ou l'identifiant d'un ticket ou d'une MR GitLab |
| `tickets` | `beads-ids` | non | Tickets existants à prendre en charge (ancien mode B) |

Sans demande ni ticket, la session commence par demander quelle feature réaliser.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-0` Valider le plan (obligatoire) | pause | pause | pause |
| `cp-spec` Valider la spec UX/UI | si une spec a été produite | idem | idem |
| `cp-1` Démarrer le ticket | pause | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause |
| `cp-3` Ticket suivant | pause | auto | auto |
| `cp-feature` Récap de la feature | pause | pause | auto |

`orchestrator-dev` ne démarre qu'après `cp-0`. Mode par défaut : `semi-auto`. Sorties : `tickets`, `branch`.

L'ancienne pré-phase d'onboarding (mode C) n'est plus dans `feature` : la précondition `project-context` propose le workflow `onboarding`, puis le retour à `feature`.

## `quick`

Session de développement directe avec l'agent `developer`, sans planification ni checkpoint. Le développeur déduit le domaine de la demande et charge les standards correspondants. Il propose `feature` si la demande s'avère importante.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

## `cadrage`

Explore, planifie et spécifie une feature sans l'implémenter : seuls des tickets Beads sont créés (`risk: plan`). Le `conductor` enchaîne `pathfinder` (exploration), `planner` (découpage et création des tickets) et `designer` (spécification UX/UI si besoin).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | oui | La feature à cadrer |

Checkpoints : `cp-scope` (périmètre, avant la planification), `cp-tickets` (obligatoire : découpage validé avant la création des tickets), `cp-recap`. Sortie : `tickets`. Pour implémenter ensuite : `ticket` ou `feature` sur les tickets créés.

## `onboarding`

Découvre le projet et crée ou enrichit le wiki `docs/wiki/` (protocole `doc-wiki-protocol`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `refresh` | `bool` (défaut : non) | non | Re-découvrir le projet et enrichir le wiki existant, sans rien supprimer |
| `focus` | `text` (4 000 caractères max) | non | Modules ou sujets à approfondir |

L'onboarder peut écrire des fichiers : la limite à `docs/wiki/` (plus le `ONBOARDING.md` minimaliste de la racine) est une consigne du prompt, pas une permission. Sortie : `wiki`.

## `review`

Review en lecture seule d'une branche ou des modifications récentes.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `review_mode` | `enum` : `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` | non | Vide : le reviewer propose le choix au démarrage |
| `branch` | `branch` | non | Branche à reviewer (vide : modifications récentes) |
| `base` | `branch` (défaut : `main`) | non | Branche de base |

Les modes combinés (`standard+adversarial`, `all`) lancent des sessions `reviewer` en parallèle : le workflow déclare l'auto-délégation `reviewer: { calls: [reviewer] }`. Une auto-délégation n'est retenue que si elle est écrite dans `calls` (jamais déduite des permissions) ; elle compte pour un niveau de profondeur.

La publication d'une MR (`oh review --publish`) n'est pas un workflow : elle reste une commande d'oh.

## `review-feedback`

Applique les commentaires non résolus d'une merge request. oh récupère les discussions sur GitLab au lancement et les passe dans l'entrée `feedback`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `mr` | `string` | oui | URL ou référence de la MR |
| `branch` | `branch` | oui | Branche de la MR |
| `base` | `branch` (défaut : `main`) | non | Branche cible |
| `feedback` | `text` (70 000 caractères max) | oui | Discussions non résolues |

Checkpoints : `cp-fix` (corrections à appliquer), `cp-2` (obligatoire : commit ou correction). Sortie : `branch`.

## `audit`

Audit en lecture seule : l'`auditor` coordonne des `auditor-subagent`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `type` | `enum` : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy` (défaut : `security`) | oui | Type d'audit |
| `focus` | `text` (4 000 caractères max) | non | Modules, fichiers ou questions à cibler |

## `debug`

Diagnostic d'un bug ou d'un problème isolé par le `debugger` : rapport de diagnostic (actions d'urgence en premier) et ticket de correction. Le `developer` est disponible à la demande dans la session.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `issue` | `text` (8 000 caractères max) | non | Le problème observé ; vide : la session le demande |

Sortie : `tickets`.

## `sweep`

Atteint un objectif transverse en le découpant en sous-tâches indépendantes, lancées en parallèle par le `conductor` vers les agents développeurs, puis vérifie le résultat.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `goal` | `text` | oui | Objectif de haut niveau |
| `strategy` | `enum` : `llm` (défaut), `manual`, `by-file`, `by-package` | non | Découpage |
| `tasks` | `text` | non | Tâches, une par ligne (`manual`) |
| `include`, `exclude` | `string` | non | Motifs glob, séparés par des virgules |
| `verify` | `enum` : `none` (défaut), `tests`, `lint`, `build`, `all`, `custom` | non | Vérification finale |
| `verify_cmd` | `string` | non | Commande de vérification (`custom`) |
| `dry_run` | `bool` | non | Afficher le découpage sans rien exécuter |

Checkpoints : `cp-plan` (découpage, avant toute exécution), `cp-recap`. Différence avec l'ancien `--sweep` : les sous-tâches s'exécutent dans la même session et le même emplacement (pas un worktree par tâche) ; `--sweep-branch-prefix` et `--max-sessions` n'ont pas d'équivalent.

## `brief-enrich`

Enrichit un brief de reprise de ticket, sans interaction (session sans interface). oh fournit le brief et enregistre le résultat.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` | oui | Ticket du brief |
| `brief` | `text` (40 000 caractères max) | oui | Contenu du brief existant |

## `libre`

Session avec l'agent de ton choix, sans checkpoint : `oh run libre --agent debugger -i request="…"` (sans `--agent` : `orchestrator`). L'entrée du workflow est **au choix** (`entry.selectable: true`) : oh calcule les membres à partir de l'agent choisi et des agents qu'il peut appeler (permission `task` du catalogue, de proche en proche). Le monde reste fermé : rien d'autre n'est visible. Dans la TUI, la session libre (commande `coder`, ou « Démarrer » quand le catalogue est vide) ouvre la fiche avec l'agent par défaut.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

`entry.selectable` est un ajout additif au schéma `oh/v1` : un autre workflow peut le déclarer ; `--agent` est refusé sur un workflow dont l'entrée n'est pas au choix.

---

## Restrictions

```yaml
limits:
  budget_usd: 5                          # budget de chaque session (USD)
  models: ["eu.anthropic.claude-*"]      # modèles autorisés (motifs)
```

- Les deux sont facultatifs et s'ajoutent aux restrictions du hub, de l'équipe et du projet (`oh budget show`, guide [Sessions v5](../guides/sessions-v5.fr.md#restrictions)). La valeur la plus précise l'emporte ; une valeur imposée par l'équipe est un plafond.
- `budget_usd` est un plafond souple sur le coût que l'outil indique pour la session et ses sous-agents : l'étape qui le dépasse se termine, puis une décision `$` demande de relever le budget ou d'arrêter.
- `models` porte sur l'identifiant envoyé au fournisseur (Bedrock : `eu.anthropic.…`) ; le proxy d'identifiants refuse les autres.
- Avec `extends`, un workflow peut seulement les durcir : budget plus bas, modèles couverts par les motifs du parent.

---

## Plugins et code mode

```yaml
code_mode: true                  # défaut : false
plugins:
  - context-mode@latest          # spécification npm (paquet, version ou tag)
  - { id: "@acme/probe", options: { verbose: true } }
```

- `code_mode` : `false` (ou absent) refuse l'outil `execute` d'opencode à tous les agents de la session ; `true` le laisse disponible.
- `plugins` : chaque entrée est une spécification npm, installée par opencode au démarrage du serveur (cache `~/.cache/opencode/npm`), avec ses `options`. Les plugins s'ajoutent au plugin oh ; en conteneur, la spécification est passée telle quelle (installation dans le conteneur).
- Un plugin qui ne se charge pas n'empêche pas la session de démarrer : opencode l'ignore et le note dans son journal. Le plugin doit exporter le format V2 (`{ id, setup }`) : `context-mode`, écrit pour opencode V1, ne se charge pas sous V2.
- Les deux champs entrent dans le hash du paquet : changer de plugin ou de code mode donne un autre paquet (et un autre serveur).

---

## Gabarits de prompt

Le premier message de la session est rendu depuis `workflows/prompts/<id>.md.tmpl` (Go `text/template`) :

- les entrées sont au premier niveau (`{{ .ticket }}`), le contexte de session sous `.oh` : `project`, `location`, `mode`, `runtime`, `lang`, `workflow` ;
- chaque gabarit commence par `Mode de workflow : {{ .oh.mode }}` (contrat avec l'agent d'entrée) et `Langue de réponse : {{ .oh.lang }}` ;
- `{{ data "request" .request }}` place une entrée dans une balise de données `<oh:data name="request">…</oh:data>` : l'agent la traite comme une donnée, jamais comme une consigne. Une balise contenue dans la valeur est neutralisée. **Toute entrée `string` ou `text` passe par `data`** (vérifié par les tests) ;
- `{{ join .tickets ", " }}` joint une liste ;
- les entrées texte sont tronquées à `max_length` (20 000 caractères par défaut), avec la mention de la troncature ; un identifiant Beads invalide ou une branche ou un chemin sur plusieurs lignes est refusé ;
- une entrée absente vaut sa valeur par défaut, sinon la valeur vide de son type (`""`, `false`, `0`, liste vide), ce qui permet `{{ if .request }}`.

Le rendu est fait par `workflow.RenderPrompt` (`cli/internal/workflow/render_prompt.go`).

---

## Tests

- `TestShippedWorkflowsAreValid` : chaque workflow du dépôt est valide contre le hub, sans aucun diagnostic.
- `TestShippedWorkflowPrompts` : le prompt de chaque workflow est rendu avec toutes les entrées puis avec les seules entrées obligatoires.
- `TestShippedWorkflowTextInputsAreDelimited` : les entrées texte ne sont écrites que par `data`.
- `TestShippedWorkflowBundles` : le paquet compilé depuis chaque workflow (agents, permissions, skills, graphe, profondeur) et les skills générées sont figés par des fichiers golden (`cli/internal/bundle/testdata/golden/workflows/<id>/`).

Après une modification volontaire : `go test ./internal/bundle -run ShippedWorkflow -update`, puis relire le diff des fichiers golden.
