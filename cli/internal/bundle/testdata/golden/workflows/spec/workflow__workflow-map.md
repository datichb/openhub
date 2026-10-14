---
name: workflow-map
description: "Carte du workflow spec : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `spec`

Comprendre une demande, écrire sa spec dans le wiki (glossaire, critères et points de test) et la découper en tickets Beads, sans l'implémenter

- Agent d'entrée : `conductor`
- Risque : `plan` : aucune modification de fichier hors des chemins de documentation ci-dessous ; écritures Beads limitées aux commandes autorisées
- Écritures de `planner` : `docs/wiki/specs/**`, à partir de `cp-understanding` (et leur commit, une fois déverrouillé)
- Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `conductor` | workflow (entrée) | primary | — | `designer`, `pathfinder`, `planner` |
| `pathfinder` | workflow | subagent | — | `designer` |
| `planner` | workflow | subagent | — | `designer` |
| `designer` | workflow | subagent | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

<!-- oh:when agent=conductor -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=conductor -->
| **cp-understanding** — Valider la compréhension | pause | pause | auto |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-converge** — Convergence de la spec et du découpage (après `cp-understanding`, refusé avant) | auto | auto | auto |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-spec** — Valider la spec (obligatoire) (après `cp-converge`, refusé avant) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-tickets** — Valider le découpage (obligatoire) (après `cp-spec`, refusé avant) | pause | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
Séquence : `cp-understanding` → `cp-converge` → `cp-spec` [commit] → `cp-tickets` [create] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
- **cp-understanding** : Présenter la reformulation de la demande par le planner, son vocabulaire et les réponses aux questions, avant l'écriture de la spec.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-converge** : Vérifié par oh, sans pause, avant de montrer la spec — glossaire, points de test, chaque critère couvert par une tranche du découpage et aucune tranche hors spec ; un écart est rendu au planner.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-spec** : oh vérifie la spec écrite sous docs/wiki/specs/ (glossaire, chaque critère avec son point de test, découpage qui les couvre) ; une fois validée, le planner la commite.
- **cp-spec** déverrouille `commit` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), quel que soit le mode. Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **cp-spec** est demandé de nouveau. Le push revient à l'utilisateur : ne pousse pas.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-tickets** : Les tickets (une tranche du découpage chacun) ne sont créés qu'après validation du découpage par l'utilisateur.
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

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
