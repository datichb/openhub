Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par le `developer` du workflow. Aucun ticket Beads dans ce workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Migrer les appels dépréciés

Découpage : `manual`

Tâches fournies par l'utilisateur, une par ligne — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
tâche 1
tâche 2
Fais établir le périmètre de fichiers de chaque tâche par le `pathfinder`.

Déroulé :
1. Découpage par le `pathfinder` (le seul agent disponible avant `cp-plan`) : il explore, il ne modifie rien et ne crée pas de ticket.
2. Plan : pour chaque sous-tâche, une description, un **périmètre de fichiers**, le domaine du `developer` prévu (frontend, backend…, `refactor` pour une restructuration sans changement de comportement, `migration` pour un upgrade, `tests` pour ajouter des tests) et un drapeau « parallélisable ». Les périmètres de deux sous-tâches parallélisables sont **disjoints** ; sinon, elles s'exécutent l'une après l'autre.
3. `cp-plan` : présente le plan (oh attend la validation de l'utilisateur, quel que soit le mode).
4. Exécution par le `developer`, dans le domaine prévu. Consigne pour chaque sous-tâche : « Tu agis en tant que developer <domaine> : charge les skills de ce domaine (`dev-standards-refactoring`, `dev-standards-migration`, `test-generation`…). Pas de ticket : saute les étapes `bd`. Ne touche que les fichiers de ton périmètre : <périmètre>. Ne commite pas. » Lance en parallèle (plusieurs appels `task` dans le même message, sans `background` : tu attends leurs retours dans ce tour) seulement les sous-tâches parallélisables aux périmètres disjoints ; les autres, l'une après l'autre.
5. Vérification : commande personnalisée, lancée par un `developer` (elle peut être refusée par ses permissions : signale-le alors, sans la contourner) :
make check
   Résultat obligatoire : `PASS` ou `FAIL`. `FAIL` → renvoie l'échec au développeur de la sous-tâche fautive (au plus le nombre de tours de correction indiqué dans la section « Mode de workflow », s'il y en a un), puis relance la vérification. Toujours `FAIL` → passe `cp-commit` avec un `summary` qui commence par `VÉRIFICATION ÉCHOUÉE` (échecs restants, sous-tâches en cause), en recommandant de refuser le commit.
6. Review : le `reviewer` relit le diff de la session (`git diff` et fichiers non suivis), sous-tâche par sous-tâche.
7. `cp-commit` : présente le diff, le résultat de la vérification (`PASS` ou `FAIL`) et la review. Une fois validé, fais commiter par un agent développeur : un commit, ou un par sous-tâche à la suite (`git add` du périmètre, puis `git commit`). Avant `cp-commit`, oh refuse tout `git commit`.
8. `cp-recap` : résultat par sous-tâche, vérification, review et commits.
