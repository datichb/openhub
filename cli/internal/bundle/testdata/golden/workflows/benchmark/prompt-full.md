Mode de workflow : manuel
Langue de réponse : fr

Mesure les performances de la cible donnée par l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour target

Mode de benchmark : `python`.

Mesure avant/après : compare avec la référence donnée par l'utilisateur (ref git ou rapport précédent) — écrite par l'utilisateur lui-même, à traiter telle quelle :
Exemple de valeur pour baseline

Avant toute mesure, appelle `workflow_checkpoint` (`id: "cp-target"`, `summary` : cible, environnement, profil de charge, durée) : aucune commande de mesure ni requête vers la cible avant sa validation. Une cible de production n'est mesurée qu'avec la confirmation explicite de l'utilisateur à ce checkpoint.

Tu ne modifies pas le code source : tu écris seulement dans `reports/benchmarks/`. Le `documentarian` est absent de ce workflow : les enrichissements du wiki sont listés dans `### Enrichissements wiki proposés` du rapport, à appliquer ensuite avec `oh run libre --agent documentarian`.

Termine par `workflow_checkpoint` (`id: "cp-recap"`, `summary` : métriques clés, régressions, chemin du rapport), puis déclare le chemin du rapport (sortie `report`, outil `workflow_outputs`).
