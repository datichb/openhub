---
name: workflow-map
description: "Carte du workflow feature : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `feature`

Réaliser une feature de bout en bout (exploration ou planification, design, implémentation, review)

- Agent d'entrée : `orchestrator`
- Risque : `write`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator` | workflow (entrée) | primary | — | `designer`, `documentarian`, `orchestrator-dev`, `pathfinder`, `planner` |
| `pathfinder` | workflow | subagent | — | `designer`, `documentarian` |
| `planner` | workflow | subagent | — | `designer`, `documentarian` |
| `designer` | workflow | subagent | — | — |
| `orchestrator-dev` | workflow | subagent | `cp-0` | `developer`, `developer-migrator`, `developer-refactor`, `documentarian`, `reviewer` |
| `developer` | workflow | subagent | `cp-1` | `documentarian` |
| `developer-refactor` | workflow | subagent | `cp-1` | `documentarian` |
| `developer-migrator` | workflow | subagent | `cp-1` | `documentarian` |
| `reviewer` | workflow | subagent | — | `documentarian` |
| `documentarian` | independent | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-0** — Valider le plan (obligatoire) | pause | pause | pause |
| **cp-spec** — Valider la spec UX/UI | conditional | conditional | conditional |
| **cp-1** — Démarrer le ticket | pause | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause |
| **cp-3** — Ticket suivant | pause | auto | auto |
| **cp-feature** — Récap de la feature | pause | pause | auto |

- **cp-0** : Afficher le tableau des tickets (ordre de traitement, agent prévu) et attendre la confirmation avant toute implémentation.
- **cp-spec** (conditional) : le designer a produit une spécification UX ou UI pour la feature
- **cp-feature** : Récap global en fin de feature (tickets traités, commits, points ouverts).

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
- `tickets` (beads-ids) : Tickets de la feature
- `branch` (branch) : Branche de travail
