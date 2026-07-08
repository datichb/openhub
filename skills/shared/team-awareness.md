---
id: team-awareness
bucket: A
scope: all
condition: team_enabled
---

# Team Awareness Protocol

Ce skill est injecté dans tous les agents lorsque les fonctionnalités d'équipe sont activées.
Il fournit les règles de base pour la collaboration via le MCP server `team`.

## Tools disponibles

| Tool | Description | Mode |
|------|-------------|------|
| `team_members` | Liste des membres de l'équipe | read |
| `team_claims` | Réservations actives (qui travaille sur quoi) | read |
| `team_wiki_list` | Pages disponibles dans le wiki partagé | read |
| `team_wiki_read` | Lire une page du wiki partagé | read |
| `team_events` | Événements récents de l'équipe | read |

## Avant de travailler sur un ticket

1. Appelle `team_claims` pour vérifier si le ticket est déjà réservé par quelqu'un
2. Si le ticket est pris par un autre membre : **informe immédiatement l'utilisateur**
3. Ne commence PAS à travailler sur un ticket déjà claimé sans confirmation explicite de l'utilisateur

## Consultation du contexte d'équipe

- Avant toute décision architecturale significative, consulte `team_wiki_list` puis `team_wiki_read` pour les pages pertinentes
- Utilise `team_events` pour comprendre l'activité récente sur le projet courant
- Utilise `team_members` si tu as besoin de savoir qui contacter pour un sujet

## Après le travail

- Le CLI émet automatiquement les événements (`session.complete`, etc.)
- Si ton travail produit un résultat qui nécessite l'attention d'un autre membre (review, décision), informe l'utilisateur pour qu'il puisse notifier l'équipe

## Règles

- Ne JAMAIS modifier le wiki partagé sans passer par `team_wiki_write` (propositions)
- Seul le `documentarian` a accès à `team_wiki_write`
- Les données team sont en lecture seule pour tous les autres agents
- Ne pas inclure de données sensibles dans les notifications
