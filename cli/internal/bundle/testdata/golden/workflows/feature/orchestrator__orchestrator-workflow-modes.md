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

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-tickets** — Valider les tickets à créer (obligatoire) | pause | pause | pause |
| **cp-0** — Valider le plan (obligatoire) | pause | pause | pause |
| **cp-spec** — Valider la spec UX/UI | conditional | conditional | conditional |
| **cp-1** — Démarrer le ticket | pause | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause |
| **cp-3** — Ticket suivant | pause | auto | auto |
| **cp-feature** — Récap de la feature | pause | pause | auto |

- **cp-tickets** : Présenter le découpage proposé par l'agent de planning ; les tickets ne sont créés qu'après la validation. Avec des tickets existants seulement, confirmer la liste (aucune création).
- **cp-tickets** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés : la création reste possible jusqu'à ce que **cp-tickets** soit demandé de nouveau.
- **cp-0** : Afficher le tableau des tickets (ordre de traitement, agent prévu) et attendre la confirmation avant toute implémentation.
- **cp-spec** (conditional) : le designer a produit une spécification UX ou UI pour la feature
- **cp-2** déverrouille `commit`, `close` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), la fermeture d'un ticket, quel que soit le mode. Une fois passé, fais-les aussitôt, dans l'ordre : commit, puis fermeture du ticket (oh refuse la fermeture tant que le travail n'est pas commité). Le ticket fermé, elles sont de nouveau verrouillées jusqu'au prochain passage de **cp-2**. Le push revient à l'utilisateur : ne pousse pas.
- **cp-feature** : Récap global en fin de feature (tickets traités, commits, points ouverts).

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
