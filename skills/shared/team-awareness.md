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
| `team_policies` | Règles d'équipe actives (conventions, limites, patterns interdits) | read |
| `team_takeover_brief` | Brief de reprise d'un ticket transféré (contexte du prédécesseur) | read |
| `team_patterns_list` | Liste des patterns de décomposition documentés par l'équipe | read |
| `team_patterns_read` | Lire le contenu complet d'un pattern (structure, dépendances, variantes) | read |
| `team_patterns_propose` | Proposer un nouveau pattern à la bibliothèque (documentarian uniquement) | write |
| `team_wiki_write` | Proposer une entrée au wiki partagé (documentarian uniquement) | write |

> **Note :** Les outils en mode `write` ne sont disponibles que pour le `documentarian`.
> Les autres agents n'y ont pas accès et ne doivent pas tenter de les appeler.

## Statuts des claims

Quand tu consultes `team_claims`, chaque réservation a un statut :

- `planned` — ticket réservé, pas encore démarré. Si l'utilisateur courant est propriétaire de ce ticket, `oh start --dev` le transitionnera automatiquement en `in_progress`.
- `in_progress` — travail en cours actif
- `review` — travail terminé, en attente de review humaine
- `blocked` — bloqué sur une dépendance externe
- `done` — terminé, sera nettoyé automatiquement

## Labels connus

Certains labels sur les claims ont une signification particulière :

- `agent-reviewed` — un agent IA a déjà effectué une passe de review sur ce ticket. Si ce label est visible, **informe l'utilisateur avant de démarrer une nouvelle review** pour éviter le travail en doublon.
- `needs-human-review` — l'agent a signalé un besoin d'attention humaine
- D'autres labels peuvent être synchronisés depuis GitLab/Jira (informatifs uniquement)

## Sync automatique depuis le tracker

Certains claims peuvent avoir été créés automatiquement en statut `planned` par la synchronisation tracker (quand un ticket GitLab/Jira a été assigné à un membre de l'équipe). Ces claims sont légitimes — les traiter exactement comme un claim `planned` créé manuellement.

## Avant de travailler sur un ticket

1. Appelle `team_claims` pour vérifier si le ticket est déjà réservé par quelqu'un
2. Si le ticket est pris par un autre membre : **informe immédiatement l'utilisateur**
3. Ne commence PAS à travailler sur un ticket déjà claimé sans confirmation explicite de l'utilisateur

## Consultation du contexte d'équipe

- Avant toute décision architecturale significative, consulte `team_wiki_list` puis `team_wiki_read` pour les pages pertinentes
- Utilise `team_events` pour comprendre l'activité récente sur le projet courant
- Utilise `team_members` si tu as besoin de savoir qui contacter pour un sujet
- Si un `team_takeover_brief` existe pour le ticket courant, le consulter pour charger le contexte du prédécesseur (voir skill `takeover-context-protocol`)
- En mode parallèle, le skill `parallel-coordination` est injecté automatiquement avec les règles spécifiques

## Après le travail

- Le CLI émet automatiquement les événements (`session.complete`, etc.)
- Si ton travail produit un résultat qui nécessite l'attention d'un autre membre (review, décision), informe l'utilisateur pour qu'il puisse notifier l'équipe

## Règles

- Ne JAMAIS modifier le wiki partagé sans passer par `team_wiki_write` (propositions)
- Seul le `documentarian` a accès à `team_wiki_write`
- Les données team sont en lecture seule pour tous les autres agents
- Ne pas inclure de données sensibles dans les notifications

## En cas d'indisponibilité des outils team

Si les outils `team_*` ne sont pas disponibles dans la session courante
(MCP server non connecté, team non configurée, erreur de connexion) :

1. **Informer l'utilisateur** : "Les données d'équipe ne sont pas disponibles dans cette session."
2. **Continuer normalement** sans les données d'équipe — ne pas bloquer le workflow
3. Ne pas inventer de données team (claims, membres, policies) en l'absence de réponse des outils
4. Si l'utilisateur demande explicitement une action team : recommander `oh team status` pour diagnostiquer

## Team Policies

Si l'équipe a configuré des policies (`team_policies`), consulter le skill
`team-policies-enforcement` pour connaître les règles d'application par agent.
En début de session, appeler `team_policies` pour charger les règles actives.
