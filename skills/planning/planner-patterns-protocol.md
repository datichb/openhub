---
id: planner-patterns-protocol
bucket: B
agent: planner
---

# Planner Patterns Protocol

Ce skill definit comment le planner utilise la bibliotheque de patterns
pour accelerer et ameliorer la decomposition des tickets.

## Consultation avant decomposition

AVANT de commencer la phase de decomposition d'un ticket :

1. Identifier les tags pertinents du ticket (ex: backend, api, crud, migration, frontend, etc.)
2. Appeler `team_patterns_list` avec ces tags
3. Si un ou plusieurs patterns matchent (>= 2 tags en commun) :
   - Appeler `team_patterns_read` pour obtenir le detail du meilleur match
   - UTILISER comme base de decomposition
   - ADAPTER au contexte specifique du ticket (ne pas copier aveuglement)
   - MENTIONNER dans le plan : "Base: pattern [nom], adapte pour [contexte]"
4. Si aucun match : decomposer normalement depuis zero

## Adaptation du pattern

Un pattern est un point de depart, pas un template rigide. Adapter signifie :
- Ajouter des etapes specifiques au contexte (auth, i18n, etc.)
- Supprimer des etapes non pertinentes
- Ajuster les estimations de complexite
- Modifier les dependances selon l'architecture du projet

## Proposition de nouveaux patterns

Apres un planning reussi (tous les tickets issus du plan completes avec succes) :

1. Evaluer si la decomposition est suffisamment generique pour etre reutilisee
2. Si oui, appeler `team_patterns_propose` avec :
   - `name` : slug descriptif (ex: "api-integration-external")
   - `tags` : tags generiques (pas le nom du projet ou du ticket)
   - `complexity` : complexite moyenne des tickets du pattern
   - `content` : la decomposition generalisee (sans references au ticket specifique)
3. Le pattern sera cree en `validated=false`, en attente de validation humaine

## Criteres pour proposer un pattern

Un pattern merite d'etre propose si :
- La decomposition couvre >= 4 etapes
- Les etapes sont suffisamment generiques pour s'appliquer a d'autres tickets
- Le type de travail est recurrent (CRUD, integration, migration, etc.)
- Le plan a ete execute avec succes (evidence que la decomposition fonctionne)

NE PAS proposer si :
- La decomposition est trop specifique a un ticket unique
- Le plan a echoue ou necessite de nombreuses corrections
- Un pattern similaire existe deja (verifier d'abord)

## Integration dans le workflow du planner

```
Phase 1 - Cadrage :
  1. Analyser le ticket
  2. Identifier les tags
  3. Consulter team_patterns_list
  4. Si match : charger le pattern

Phase 2 - Decomposition :
  1. Si pattern charge : l'adapter au contexte
  2. Si pas de pattern : decomposer depuis zero
  3. Generer les beads (tickets enfants)

Phase 7 - Cloture (post-execution reussie) :
  1. Evaluer si le plan merite d'etre un pattern
  2. Si oui : team_patterns_propose
```
