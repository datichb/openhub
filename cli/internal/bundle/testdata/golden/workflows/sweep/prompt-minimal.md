Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par les agents développeurs du workflow. Aucun ticket Beads dans ce workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour goal

Découpage : `llm`
Fais analyser l'objectif et explorer le code par le `pathfinder` (lecture seule), qui propose les sous-tâches.

Déroulé :
1. Découpage par le `pathfinder` (le seul agent disponible avant `cp-plan`) : il explore, il ne modifie rien et ne crée pas de ticket.
2. Plan : pour chaque sous-tâche, une description, un **périmètre de fichiers**, l'agent développeur prévu et un drapeau « parallélisable ». Les périmètres de deux sous-tâches parallélisables sont **disjoints** ; sinon, elles s'exécutent l'une après l'autre.
3. `cp-plan` : présente le plan (oh attend la validation de l'utilisateur, quel que soit le mode).
4. Exécution, avec l'agent développeur prévu. Consigne pour chaque sous-tâche : « Pas de ticket : saute les étapes `bd`. Ne touche que les fichiers de ton périmètre : <périmètre>. Ne commite pas. » Lance en parallèle (plusieurs appels `task` dans le même message) seulement les sous-tâches parallélisables aux périmètres disjoints ; les autres, l'une après l'autre.
5. Vérification : aucune.
6. Review : le `reviewer` relit le diff de la session (`git diff` et fichiers non suivis), sous-tâche par sous-tâche.
7. `cp-commit` : présente le diff, le résultat de la vérification (`PASS` ou `FAIL`) et la review. Une fois validé, fais commiter par un agent développeur : un commit, ou un par sous-tâche à la suite (`git add` du périmètre, puis `git commit`). Avant `cp-commit`, oh refuse tout `git commit`.
8. `cp-recap` : résultat par sous-tâche, vérification, review et commits.
