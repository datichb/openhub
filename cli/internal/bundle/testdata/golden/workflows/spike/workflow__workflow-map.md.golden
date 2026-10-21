---
name: workflow-map
description: "Carte du workflow spike : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `spike`

Répondre à une question technique dans un temps borné — recherche sourcée, prototype jetable au besoin, décision go, clarify ou kill écrite dans le wiki

- Agent d'entrée : `pathfinder`
- Risque : `write`
- Écritures de `pathfinder` : `docs/wiki/decisions/**` (et leur commit, une fois déverrouillé)
- Commandes Beads autorisées : `show`, `list`, `search`, `query`, `count`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `pathfinder` | workflow (entrée) | primary | — | `developer` |
| `developer` | workflow | subagent | `cp-question` | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

<!-- oh:when agent=pathfinder -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=pathfinder -->
| **cp-question** — Question et critères | pause | pause | auto |
<!-- /oh:when -->
<!-- oh:when agent=pathfinder -->
| **cp-decision** — Décision du spike (obligatoire) | pause | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=pathfinder -->
Séquence : `cp-question` → `cp-decision` [commit] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=pathfinder -->
- **cp-question** : Valider la question reformulée, les critères de décision et la durée avant toute recherche ou prototype.
<!-- /oh:when -->
<!-- oh:when agent=pathfinder -->
- **cp-decision** : Présenter la décision (go, clarify ou kill) critère par critère avec ses sources, écrite sous docs/wiki/decisions/ ; oh vérifie que le prototype jetable (.oh-spike/) a été supprimé ; une fois validée, seule la décision est commitée.
- **cp-decision** déverrouille `commit` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), quel que soit le mode. Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **cp-decision** est demandé de nouveau. Le push revient à l'utilisateur : ne pousse pas.
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
- Disjoncteur : après 6 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
