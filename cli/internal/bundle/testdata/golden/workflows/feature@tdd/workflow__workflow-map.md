---
name: workflow-map
description: "Carte du workflow feature : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `feature`

Réaliser une feature de bout en bout (exploration ou planification, design, implémentation, review)

- Agent d'entrée : `orchestrator`
- Risque : `write`
- Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator` | workflow (entrée) | primary | — | `designer`, `orchestrator-dev`, `pathfinder`, `planner`, `reviewer` |
| `pathfinder` | workflow | subagent | — | `designer` |
| `planner` | workflow | subagent | — | `designer` |
| `designer` | workflow | subagent | — | — |
| `orchestrator-dev` | workflow | subagent | `cp-0` | `developer`, `documentarian`, `reviewer` |
| `developer` | workflow | subagent | `cp-1` | `documentarian` |
| `reviewer` | workflow | subagent | — | — |
| `documentarian` | independent | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

<!-- oh:when agent=orchestrator,orchestrator-dev,developer -->
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
<!-- oh:when agent=developer -->
| **cp-seams** — Points d'injection et tests prévus | pause | pause | auto |
<!-- /oh:when -->
<!-- oh:when agent=developer -->
| **cp-red** — Tests en échec | auto | auto | auto |
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
<!-- oh:when agent=developer -->
Séquence : `cp-seams` → `cp-red` (entre crochets, les opérations que le checkpoint déverrouille).
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
<!-- oh:when agent=developer -->
- **cp-seams** : Méthode TDD, avant d'écrire un test : présenter les points d'injection, les interfaces et les tests prévus (comportement attendu, cas limites). Une fois par ticket.
<!-- /oh:when -->
<!-- oh:when agent=developer -->
- **cp-red** : Méthode TDD, tests écrits et avant le code : oh vérifie que les tests échouent pour la bonne raison et que les changements depuis cp-seams sont des tests (et les seules amorces nécessaires). Une fois par ticket.
- **cp-red** : oh lance lui-même la vérification avant de le laisser passer : les tests doivent échouer, à l'exécution et non à la compilation (le code doit construire) ; les changements depuis **cp-seams** doivent toucher au moins un fichier de test (les autres fichiers modifiés sont signalés : n'y mets que les amorces nécessaires, sans implémentation). Refusé : corrige, puis rappelle-le.
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

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
- Au plus 3 tours de correction (developer → reviewer) par ticket ; au-delà, passe le checkpoint de commit en signalant les points non résolus.

## Sorties à déclarer

Dès qu'une sortie est produite, déclare-la avec l'outil `workflow_outputs` (`type`, `value`, `id`) ; en fin de travail, rappelle chaque sortie sur une ligne `Sortie <id> : <valeur>` :
- `tickets` (beads-ids) : Tickets de la feature
- `branch` (branch) : Branche de travail
