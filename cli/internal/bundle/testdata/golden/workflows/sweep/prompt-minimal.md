Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par les agents développeurs du workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour goal

Découpage : `llm`
Analyse l'objectif et découpe-le en sous-tâches indépendantes ; fais explorer le code par un `developer` si nécessaire.

Règles :
- Chaque sous-tâche a une description et un périmètre de fichiers ; l'agent qui l'exécute ne modifie que les fichiers de son périmètre.
- `cp-plan` : présente la liste des sous-tâches avant toute exécution.
- Lance les sous-tâches indépendantes en parallèle (plusieurs appels `task` dans le même message), avec l'agent développeur adapté.
- Vérification après les sous-tâches : aucune.
- `cp-recap` : résultat par sous-tâche et résultat de la vérification.
