---
name: workflow-map
description: "Carte du workflow debug : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `debug`

Diagnostiquer un bug ou un problème isolé (rapport de diagnostic, ticket de correction)

- Agent d'entrée : `debugger`
- Risque : `write`
- Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `debugger` | workflow (entrée) | primary | — | `documentarian` |
| `developer` | independent | primary | — | `documentarian` |
| `documentarian` | independent | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

<!-- oh:when agent=debugger,developer -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=debugger -->
| **cp-ticket** — Créer le ticket de correction (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=developer -->
| **cp-commit** — Commit (obligatoire) | pause | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=debugger -->
Séquence : `cp-ticket` [create] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->
<!-- oh:when agent=developer -->
Séquence : `cp-commit` [commit, close] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=debugger -->
- **cp-ticket** : Présenter le diagnostic et le ticket de correction à créer ; le ticket n'est créé qu'après la validation.
- **cp-ticket** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **cp-ticket** est demandé de nouveau.
<!-- /oh:when -->
<!-- oh:when agent=developer -->
- **cp-commit** : Présenter le résumé des changements (fichiers, tests) avant le commit, puis la fermeture du ticket s'il y en a un.
- **cp-commit** déverrouille `commit`, `close` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), la fermeture d'un ticket, quel que soit le mode. Une fois passé, fais-les aussitôt, dans l'ordre : commit, puis fermeture du ticket (oh refuse la fermeture tant que le travail n'est pas commité). Le travail commité, le commit est de nouveau verrouillé ; le ticket fermé, la fermeture aussi, jusqu'au prochain passage de **cp-commit**. Le push revient à l'utilisateur : ne pousse pas.
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

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `manuel`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.

## Sorties à déclarer

Dès qu'une sortie est produite, déclare-la avec l'outil `workflow_outputs` (`type`, `value`, `id`) ; en fin de travail, rappelle chaque sortie sur une ligne `Sortie <id> : <valeur>` :
- `tickets` (beads-ids) : Tickets de correction
