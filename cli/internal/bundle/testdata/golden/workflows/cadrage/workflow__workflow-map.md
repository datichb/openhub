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

Comportements :
- `pause` : arrête-toi, présente l'état et demande la validation de l'utilisateur avec l'outil `question` ; ne continue jamais sans réponse.
- `auto` : annonce le passage du checkpoint en une ligne, puis continue.
- `skip` : le checkpoint est ignoré dans ce mode.
- `conditional` : évalue la condition ; si elle est remplie, comporte-toi comme `pause`, sinon comme `auto`.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, fais une pause et demande s'il faut continuer, faire une revue ou arrêter. Le compteur repart de zéro à chaque échange avec l'utilisateur.

## Sorties à déclarer

En fin de travail, annonce chaque sortie produite sur une ligne `Sortie <id> : <valeur>` :
- `tickets` (beads-ids) : Tickets créés
