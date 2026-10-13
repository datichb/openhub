---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow cadrage. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.

<!-- oh:when agent=conductor -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=conductor -->
| **cp-scope** — Valider le périmètre | pause | pause | auto |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-tickets** — Valider le découpage (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-recap** — Récap du cadrage | pause | pause | auto |
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
Séquence : `cp-scope` → `cp-tickets` [create] → `cp-recap` (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
- **cp-scope** : Présenter le rapport d'exploration et confirmer le périmètre avant la planification.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-tickets** : Les tickets ne sont créés qu'après validation du découpage par l'utilisateur.
- **cp-tickets** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **cp-tickets** est demandé de nouveau.
<!-- /oh:when -->

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
<!-- /oh:when -->
