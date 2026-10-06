---
name: workflow-map
description: "Carte du workflow cadrage : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `cadrage`

Explorer, planifier et spécifier une feature sans l'implémenter (création des tickets Beads)

- Agent d'entrée : `conductor`
- Risque : `plan` : aucune modification de fichier ; écritures Beads limitées aux commandes autorisées
- Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `search`, `count`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `conductor` | workflow (entrée) | primary | — | `designer`, `pathfinder`, `planner` |
| `pathfinder` | workflow | subagent | — | `designer` |
| `planner` | workflow | subagent | `cp-scope` | `designer` |
| `designer` | workflow | subagent | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-scope** — Valider le périmètre | pause | pause | auto |
| **cp-tickets** — Valider le découpage (obligatoire) | pause | pause | pause |
| **cp-recap** — Récap du cadrage | pause | pause | auto |

- **cp-scope** : Présenter le rapport d'exploration et confirmer le périmètre avant la planification.
- **cp-tickets** : Les tickets ne sont créés qu'après validation du découpage par l'utilisateur.

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- N'utilise pas l'outil `question` pour valider un checkpoint ; `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents encore verrouillés).

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.

## Sorties à déclarer

Dès qu'une sortie est produite, déclare-la avec l'outil `workflow_outputs` (`type`, `value`, `id`) ; en fin de travail, rappelle chaque sortie sur une ligne `Sortie <id> : <valeur>` :
- `tickets` (beads-ids) : Tickets créés
