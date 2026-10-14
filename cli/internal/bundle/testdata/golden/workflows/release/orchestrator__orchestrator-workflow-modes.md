---
name: orchestrator-workflow-modes
description: "Modes de workflow et comportement des checkpoints du workflow release. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Modes de workflow et checkpoints

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.

<!-- oh:when agent=release-manager -->
## Checkpoints (dans l'ordre)

Seuls les checkpoints que tu passes figurent ici (contrat de session).

| Checkpoint | manuel | semi-auto |
|---|---|---|
<!-- oh:when agent=release-manager -->
| **cp-release** — Release (obligatoire) | pause | pause |
<!-- /oh:when -->

<!-- oh:when agent=release-manager -->
Séquence : `cp-release` [commit, tag, mr] (entre crochets, les opérations que le checkpoint déverrouille).
<!-- /oh:when -->

<!-- oh:when agent=release-manager -->
- **cp-release** : Valider la version et sa raison, la section du CHANGELOG, les notes et le message du tag (et la description de la MR si elle est demandée) ; le commit, le tag et la MR restent refusés avant.
- **cp-release** déverrouille `commit`, `tag`, `mr` : avant son passage, oh refuse à tous les agents de la session `git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull), la création d'un tag (`git tag`, `git push --tags`), l'ouverture d'une merge request (`glab mr create`, `gh pr create`), quel que soit le mode. Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **cp-release** est demandé de nouveau. Le tag vient après le commit : oh le refuse tant que la release n'est pas commitée. La création du tag et l'ouverture de la merge request se referment au checkpoint obligatoire suivant, ou quand **cp-release** est demandé de nouveau. Le push (branche et tags) revient à l'utilisateur : ne pousse pas.
<!-- /oh:when -->

Passage d'un checkpoint : appelle l'outil `workflow_checkpoint` avec `id` (identifiant du tableau) et `summary` (ce qui est fait, ce qui va suivre, points d'attention), **avant** de lancer l'étape suivante. C'est oh qui applique le comportement du mode :
- `pause` (et `conditional`) : l'appel attend la validation de l'utilisateur (depuis oh ou l'interface). S'il est refusé, ne continue pas : suis la consigne reçue (corriger, préciser), puis rappelle `workflow_checkpoint` pour le même checkpoint.
- `auto` : l'appel passe aussitôt ; continue.
- `skip` : le checkpoint est ignoré dans ce mode, ne l'appelle pas.
- La réponse de l'outil peut contenir une consigne de l'utilisateur : applique-la.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.
- Les checkpoints passent **uniquement** par `workflow_checkpoint` : n'utilise jamais l'outil `question` ni un message texte pour poser ou valider un checkpoint (« CP-2 : commit ou corriger ? »). oh ferme une telle question sans effet et le checkpoint reste à passer. `workflow_status` donne l'état courant (checkpoints passés, prochain checkpoint, agents et opérations encore verrouillés).
<!-- /oh:when -->
