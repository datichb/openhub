---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow ticket. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, fais une pause et demande s'il faut continuer, faire une revue ou arrêter. Le compteur repart de zéro à chaque échange avec l'utilisateur.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| **cp-1** — Démarrer le ticket | pause | auto | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause | defer |
| **cp-3** — Ticket suivant | pause | auto | auto | auto |

- **cp-1** : Présenter le ticket et l'agent prévu avant de lancer l'implémentation.
- **cp-2** : Après la review, présenter le résultat et demander s'il faut committer ou corriger.
- **cp-3** : Passer au ticket suivant ou s'arrêter.

Comportements :
- `pause` : arrête-toi, présente l'état et demande la validation de l'utilisateur avec l'outil `question` ; ne continue jamais sans réponse.
- `auto` : annonce le passage du checkpoint en une ligne, puis continue.
- `skip` : le checkpoint est ignoré dans ce mode.
- `conditional` : évalue la condition ; si elle est remplie, comporte-toi comme `pause`, sinon comme `auto`.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
