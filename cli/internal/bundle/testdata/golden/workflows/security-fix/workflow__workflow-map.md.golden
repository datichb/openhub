---
name: workflow-map
description: "Carte du workflow security-fix : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `security-fix`

Corriger les constats d'un audit de sécurité — choix des constats, correction avec test de non-régression, nouvelle analyse ciblée avant le commit

- Agent d'entrée : `conductor`
- Risque : `write`
- Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `conductor` | workflow (entrée) | primary | — | `auditor-subagent`, `developer` |
| `developer` | workflow | subagent | `cp-triage` | — |
| `auditor-subagent` | workflow | subagent | `developer` | — |

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
| **cp-triage** — Constats à corriger (obligatoire) | pause | pause | auto |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-rescan** — Nouvelle analyse | pause | auto | auto |
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
Séquence : `cp-triage` [create] → `cp-rescan` → `cp-2` [commit] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=conductor -->
- **cp-triage** : Présenter les constats reçus, numérotés, avec leur gravité et leur emplacement ; la réponse dit lesquels corriger dans cette session. Une fois validé, chaque constat écarté devient un ticket, avec la raison du tri.
- **cp-triage** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **cp-triage** est demandé de nouveau.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-rescan** : Vérifié par oh, sans pause en semi-auto, avant le commit — les tests passent et au moins un test a été écrit ou modifié ; le verdict de l'auditeur sur chaque constat corrigé accompagne le passage, un constat encore présent repart au developer.
- **cp-rescan** : oh lance lui-même la vérification avant de le laisser passer : les changements doivent toucher au moins un fichier de test. Refusé : corrige, puis rappelle-le.
<!-- /oh:when -->
<!-- oh:when agent=conductor -->
- **cp-2** : Présenter les corrections, leurs tests de non-régression et le verdict de la nouvelle analyse avant le commit.
- **cp-2** : oh lance lui-même la vérification avant de le laisser passer : les changements doivent toucher au moins un fichier de test. Refusé : corrige, puis rappelle-le.
- **cp-2** déverrouille `commit` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), quel que soit le mode. Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **cp-2** est demandé de nouveau. Le push revient à l'utilisateur : ne pousse pas.
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
- Disjoncteur : après 8 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.
- Au plus 2 tours de correction (developer → reviewer) par ticket ; au-delà, passe le checkpoint de commit en signalant les points non résolus.
