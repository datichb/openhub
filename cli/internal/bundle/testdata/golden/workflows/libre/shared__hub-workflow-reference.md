---
name: hub-workflow-reference
description: "Agents et délégations du workflow libre. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Référence du workflow `libre`

Agent d'entrée : `orchestrator`. Seuls les agents ci-dessous existent dans cette session.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator` | workflow (entrée) | primary | — | `debugger`, `designer`, `onboarder`, `orchestrator-dev`, `pathfinder`, `planner`, `reviewer` |
| `debugger` | workflow | subagent | — | — |
| `designer` | workflow | subagent | — | — |
| `onboarder` | workflow | subagent | — | — |
| `orchestrator-dev` | workflow | subagent | — | `developer`, `documentarian`, `reviewer` |
| `pathfinder` | workflow | subagent | — | `designer` |
| `planner` | workflow | subagent | — | `designer` |
| `reviewer` | workflow | subagent | — | `reviewer` |
| `developer` | workflow | subagent | — | — |
| `documentarian` | workflow | subagent | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.
