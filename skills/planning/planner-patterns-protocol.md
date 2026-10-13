---
name: planner-patterns-protocol
description: "Utiliser en phase 3 du planner, avec le serveur MCP team : réutiliser la bibliothèque de patterns pour décomposer."
mcp: team
---

# Planner Patterns Protocol

Comment le planner utilise la bibliothèque de patterns de l'équipe pour accélérer et améliorer la décomposition.

## Consultation avant décomposition (phase 3)

1. Identifier les tags pertinents de la feature (ex : backend, api, crud, migration, frontend)
2. Appeler `team_patterns_list` avec ces tags
3. Si un ou plusieurs patterns matchent (>= 2 tags en commun) :
   - Appeler `team_patterns_read` pour obtenir le détail du meilleur match
   - L'utiliser comme base de décomposition
   - ADAPTER au contexte specifique du ticket (ne pas copier aveuglement)
   - Le mentionner dans le plan : "Base: pattern [nom], adapté pour [contexte]"
4. Sinon : décomposer depuis zéro (tracer bullet et découpage vertical, `planner-phase-3-4`)

Adapter un pattern : ajouter les étapes propres au contexte (auth, i18n…), retirer les étapes non pertinentes, ajuster les estimations et les dépendances à l'architecture du projet.

## Proposition de nouveaux patterns (après exécution réussie)

Quand tous les tickets issus du plan sont terminés avec succès, et que la décomposition couvre >= 4 étapes génériques d'un travail récurrent (CRUD, intégration, migration…) :

1. Vérifier qu'aucun pattern similaire n'existe
2. Appeler `team_patterns_propose` avec `name` (slug descriptif, ex : "api-integration-external"), `tags` (génériques, sans nom de projet ou de ticket), `complexity` (moyenne des tickets), `content` (la décomposition généralisée)
3. Le pattern est créé en `validated=false`, en attente de validation humaine

NE PAS proposer si :
- La décomposition est trop specifique a un ticket unique
- Le plan a echoue ou necessite de nombreuses corrections
- Un pattern similaire existe deja
