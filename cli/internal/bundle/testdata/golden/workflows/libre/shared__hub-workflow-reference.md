---
name: hub-workflow-reference
description: "Agents et délégations du workflow libre. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Référence du workflow `libre`

Agent d'entrée : `orchestrator`. Seuls les agents ci-dessous existent dans cette session.

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
| `reviewer` | workflow | primary | — | `documentarian` |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.
