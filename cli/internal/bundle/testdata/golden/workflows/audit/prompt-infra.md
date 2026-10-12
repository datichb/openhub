Mode de workflow : manuel
Langue de réponse : fr

Réalise un audit infra du projet. Focus : infrastructure (revue IaC, sécurité, coût, drift ; outils d'analyse statique lancés après confirmation, aucune application).

Audit en lecture seule : aucun fichier n'est modifié et aucun ticket n'est créé ; les corrections proposées sont listées dans le rapport.
Le `documentarian` est absent de ce workflow en lecture seule : les enrichissements du wiki sont listés dans `### Enrichissements wiki proposés` du rapport, à appliquer ensuite avec `oh run libre --agent documentarian`.
