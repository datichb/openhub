---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow quick. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `manuel`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.

## Checkpoints (dans l'ordre)

Aucun checkpoint : avance sans pause imposée, en demandant confirmation avant toute action irréversible.
