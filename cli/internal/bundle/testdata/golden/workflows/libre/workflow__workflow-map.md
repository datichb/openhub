---
name: workflow-map
description: "Carte du workflow libre : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `libre`

Session avec l'agent de ton choix et les agents qu'il peut appeler, sans checkpoint (remplace oh start --agent)

- Agent d'entrée : `orchestrator`
- Risque : `write`
- Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator` | workflow (entrée) | primary | — | `debugger`, `designer`, `documentarian`, `onboarder`, `orchestrator-dev`, `pathfinder`, `planner` |
| `debugger` | workflow | primary | — | `documentarian` |
| `designer` | workflow | primary | — | — |
| `documentarian` | workflow | primary | — | — |
| `onboarder` | workflow | primary | — | — |
| `orchestrator-dev` | workflow | primary | — | `developer`, `developer-migrator`, `developer-refactor`, `documentarian`, `reviewer` |
| `pathfinder` | workflow | primary | — | `designer`, `documentarian` |
| `planner` | workflow | primary | — | `designer`, `documentarian` |
| `developer` | workflow | subagent | — | `documentarian` |
| `developer-migrator` | workflow | subagent | — | `documentarian` |
| `developer-refactor` | workflow | subagent | — | `documentarian` |
| `reviewer` | workflow | primary | — | `documentarian`, `reviewer` |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

Aucun checkpoint : avance sans pause imposée, en demandant confirmation avant toute action irréversible.

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `manuel`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
