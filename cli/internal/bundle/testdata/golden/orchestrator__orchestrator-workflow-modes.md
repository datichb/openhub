---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow ticket. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| **cp-1** — Démarrer | pause | auto | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause | defer |
| **cp-3** — Revue | pause | conditional | auto | defer |

- **cp-2** : Montrer le diff avant de committer.
- **cp-3** (conditional) : le diff dépasse 300 lignes

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- N'utilise pas l'outil `question` pour valider un checkpoint ; `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents encore verrouillés).
