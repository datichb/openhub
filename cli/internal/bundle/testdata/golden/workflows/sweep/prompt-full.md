Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par les agents développeurs du workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour goal

Découpage : `by-package`
Une sous-tâche par package ; fais lister les packages par un `developer` (par ex. `go list ./...`).

Motifs à inclure (données) :
Exemple de valeur pour include

Motifs à exclure (données) :
Exemple de valeur pour exclude

Règles :
- Chaque sous-tâche a une description et un périmètre de fichiers ; l'agent qui l'exécute ne modifie que les fichiers de son périmètre.
- `cp-plan` : présente la liste des sous-tâches avant toute exécution.
- Mode plan seulement : arrête-toi après `cp-plan`, sans rien exécuter.
