---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow incident. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 6 lancements `task` consécutifs sans interaction de l'utilisateur, oh suspend les délégations jusqu'à ce que l'utilisateur intervienne. Si un lancement est refusé, fais le point et attends.

<!-- oh:when agent=debugger -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
<!-- oh:when agent=debugger -->
| **cp-mitigated** — Atténuation (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=debugger -->
| **cp-postmortem** — Postmortem (obligatoire) | pause | pause | pause |
<!-- /oh:when -->
<!-- oh:when agent=debugger -->
| **cp-actions** — Actions (obligatoire) | pause | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=debugger -->
Séquence : `cp-mitigated` → `cp-postmortem` [commit] → `cp-actions` [create] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=debugger -->
- **cp-mitigated** : Présenter le diagnostic rapide et la mesure d'atténuation (retour arrière, désactivation, configuration, ou correction de code déjà appliquée dans l'arbre de travail, vérifiée par oh) ; l'utilisateur décide de l'appliquer, ou confirme qu'elle est en place.
<!-- /oh:when -->
<!-- oh:when agent=debugger -->
- **cp-postmortem** : Présenter le postmortem sans blâme écrit sous docs/wiki/postmortems/ ; une fois validé, le correctif d'atténuation (s'il y en a un) puis le postmortem sont commités, chacun dans son commit.
- **cp-postmortem** déverrouille `commit` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), quel que soit le mode. Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **cp-postmortem** est demandé de nouveau. Le push revient à l'utilisateur : ne pousse pas.
<!-- /oh:when -->
<!-- oh:when agent=debugger -->
- **cp-actions** : Les actions du postmortem (prévenir, détecter, atténuer plus vite) ne deviennent des tickets qu'après validation de la liste par l'utilisateur.
- **cp-actions** déverrouille `create` : avant son passage, oh refuse à tous les agents de la session la création de tickets, quel que soit le mode. Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **cp-actions** est demandé de nouveau.
<!-- /oh:when -->

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
<!-- /oh:when -->
