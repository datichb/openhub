---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow feature. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
- Au plus 3 tours de correction (developer → reviewer) par ticket ; au-delà, passe le checkpoint de commit en signalant les points non résolus.

<!-- oh:when agent=orchestrator,orchestrator-dev -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=orchestrator -->
| **cp-tickets** — Valider les tickets à créer (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
| **cp-0** — Valider le plan (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
| **cp-spec** — Valider la spec UX/UI | conditional | conditional | conditional |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
| **cp-1** — Démarrer le ticket | pause | auto | auto |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
| **cp-3** — Ticket suivant | pause | auto | auto |
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
| **cp-feature** — Récap de la feature | pause | auto | auto |
<!-- /oh:when -->

<!-- oh:when agent=orchestrator -->
Séquence : `cp-tickets` [create] → `cp-0` → `cp-spec` → `cp-feature` (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
Séquence : `cp-1` → `cp-2` [commit, close] → `cp-3` (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=orchestrator -->
- **cp-tickets** : Présenter le découpage proposé par l'agent de planning ; les tickets ne sont créés qu'après la validation. Avec des tickets existants seulement, confirmer la liste (aucune création).
- **cp-tickets** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **cp-tickets** est demandé de nouveau.
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
- **cp-0** : Afficher le tableau des tickets (ordre de traitement, agent prévu) et attendre la confirmation avant toute implémentation.
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
- **cp-spec** (conditional) : le designer a produit une spécification UX ou UI pour la feature
<!-- /oh:when -->
<!-- oh:when agent=orchestrator-dev -->
- **cp-2** déverrouille `commit`, `close` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), la fermeture d'un ticket, quel que soit le mode. Une fois passé, fais-les aussitôt, dans l'ordre : commit, puis fermeture du ticket (oh refuse la fermeture tant que le travail n'est pas commité). Le travail commité, le commit est de nouveau verrouillé ; le ticket fermé, la fermeture aussi, jusqu'au prochain passage de **cp-2**. Le push revient à l'utilisateur : ne pousse pas.
<!-- /oh:when -->
<!-- oh:when agent=orchestrator -->
- **cp-feature** : Récap global en fin de feature (tickets traités, commits, points ouverts).
<!-- /oh:when -->

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
<!-- /oh:when -->
