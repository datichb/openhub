---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow infra-change. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `manuel`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-plan** — Plan | pause | pause | pause |
| **cp-apply** — Application (obligatoire) | pause | pause | pause |

Séquence : `cp-plan` → `cp-apply` [apply] (entre crochets, les opérations que le checkpoint déverrouille).

- **cp-plan** : Présenter la cible vérifiée, le plan (créations, modifications, destructions), le delta de coût, les risques et le retour arrière.
- **cp-apply** : Confirmer la cible et le plan validé avant de l'appliquer ; les modifications de l'infrastructure restent refusées avant.
- **cp-apply** déverrouille `apply` : avant son passage, oh refuse à tous les agents de la session les modifications d'infrastructure (`terraform apply`, `kubectl apply`…), quel que soit le mode.

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
