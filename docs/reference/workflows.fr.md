# Workflows livrés par le hub

> v5 : chaque cas d'usage est un workflow déclaratif (`apiVersion: oh/v1`) livré par le hub. Les fichiers sont dans `workflows/` à la racine du dépôt, embarqués dans le binaire et extraits dans `~/.oh/hub/workflows/`. Une équipe ou un projet peut les étendre (`extends`) à partir de la phase 2.

Vérifier les workflows du hub :

```bash
oh workflow validate --all          # tous les workflows du hub
oh workflow validate ticket         # un workflow du catalogue
oh workflow validate ./mon-wf.yaml  # un fichier
```

Le lancement par `oh run <workflow>` et la fiche de lancement de la TUI arrivent avec la suite de la phase 1 ; d'ici là, `oh start` et les commandes existantes restent les points d'entrée.

---

## Vue d'ensemble

| Workflow | Agent d'entrée | Agents | Risque | Exécution | Remplace |
|---|---|---|---|---|---|
| `ticket` | `orchestrator-dev` | `developer`, `developer-refactor`, `developer-migrator`, `reviewer`, `documentarian` (à la demande) | write | local, conteneur, distant | `oh start --dev`, `-t` |
| `feature` | `orchestrator` | `pathfinder`, `planner`, `designer`, `orchestrator-dev`, `developer*`, `reviewer`, `documentarian` (à la demande) | write | local | `oh start` (anciens modes A, B et E) |
| `quick` | `developer` | — | write | local | session rapide |

Les autres workflows (`cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `sweep`, `brief-enrich`) suivent.

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

L'ancienne pré-phase d'onboarding (mode C) n'est plus dans `feature` : le workflow `onboarding` la remplace.

## `quick`

Session de développement directe avec l'agent `developer`, sans planification ni checkpoint. Le développeur déduit le domaine de la demande et charge les standards correspondants. Il propose `feature` si la demande s'avère importante.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

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
