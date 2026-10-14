---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow implement-spec. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
- Au plus 3 tours de correction (developer → reviewer) par ticket ; au-delà, passe le checkpoint de commit en signalant les points non résolus.

<!-- oh:when agent=orchestrator-dev -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=orchestrator-dev -->
| **cp-plan** — Valider le plan (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
| **cp-merge** — Valider les merges (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
| **cp-final** — Revue de l'ensemble et récap | pause | pause | auto |
<!-- /oh:when -->

<!-- oh:when agent=orchestrator-dev -->
Séquence : `cp-plan` → `cp-merge` [merge] → `cp-final` (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=orchestrator-dev -->
- **cp-plan** : Avant toute implémentation, présenter le tableau des tickets (ordre de traitement selon les dépendances puis la priorité, agent prévu, tickets écartés et pourquoi).
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
- **cp-merge** : Fan-out : les sessions enfants terminées et mergées par oh dans la branche d'intégration, la revue de l'ensemble faite — présenter le tableau des enfants (état, merge, conflits résolus par le merger) et le verdict de la revue. oh vérifie la branche d'intégration ; validé, oh la merge dans la branche de la session et ferme les tickets mergés. Remplace le cp-2 de chaque enfant.
- **cp-merge** déverrouille `merge` : avant son passage, oh refuse à tous les agents de la session le merge des branches (`git merge`, `git pull`), quel que soit le mode. Une fois validé, oh merge lui-même la branche d'intégration dans la branche de la session (et ferme les tickets des enfants mergés, s'il y en a) : aucun agent ne merge, ne commite, ne pousse ni ne ferme de ticket.
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
- **cp-final** : Tous les tickets traités (ou arrêt), la revue de l'ensemble faite — présenter son verdict, les tickets traités et leurs commits, les points ouverts ; l'utilisateur décide d'un tour de correction ou de s'arrêter.
<!-- /oh:when -->

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
<!-- /oh:when -->
