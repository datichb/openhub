Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par le `developer` du workflow. Aucun ticket Beads dans ce workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour goal

Découpage : `by-package`
Une sous-tâche par package : fais lister les packages par le `pathfinder` (dossiers du projet, lecture seule).

Motifs à inclure (données) :
Exemple de valeur pour include

Motifs à exclure (données) :
Exemple de valeur pour exclude

Déroulé :
1. Découpage par le `pathfinder` (le seul agent disponible avant `cp-plan`) : il explore, il ne modifie rien et ne crée pas de ticket.
2. Plan : pour chaque sous-tâche, une description, un **périmètre de fichiers**, le domaine du `developer` prévu (frontend, backend…, `refactor` pour une restructuration sans changement de comportement, `migration` pour un upgrade, `tests` pour ajouter des tests) et un drapeau « parallélisable ». Les périmètres de deux sous-tâches parallélisables sont **disjoints** ; sinon, elles s'exécutent l'une après l'autre.
3. Mode plan seulement : affiche le plan, **n'appelle pas `cp-plan`** et ne lance aucun agent développeur. Termine par `cp-recap`, avec le plan comme résumé.
