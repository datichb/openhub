---
id: team-coordination
bucket: B
agent: orchestrator-dev
---

# Team Coordination Protocol

Ce skill fournit les règles de coordination d'équipe pour l'orchestrator-dev.
Il est chargé à la demande lorsque le contexte d'équipe est pertinent.

## Vérification des claims avant le travail

Avant de commencer l'implémentation d'un ticket :

1. Appelle `team_claims` avec le projet courant
2. Si le ticket demandé est déjà claimé par un autre membre :
   - Informe l'utilisateur : "Ce ticket est actuellement pris par [membre] depuis [date]"
   - Demande confirmation avant de continuer
   - Suggère de contacter le membre ou de choisir un autre ticket
3. Si le ticket n'est pas claimé : le CLI gère automatiquement le claim

## Sélection de tickets

Quand l'utilisateur demande un ticket à travailler sans spécifier lequel :

1. Appelle `team_claims` pour voir les tickets déjà pris
2. Suggère uniquement les tickets **non claimés**
3. Affiche les claims actifs pour contexte : "Alice travaille sur X, Benjamin sur Y"

## Contexte inter-tickets

Si un travail en cours est lié à un ticket claimé par un autre membre :

1. Appelle `team_events` pour voir l'activité récente sur le ticket lié
2. Mentionne la dépendance à l'utilisateur
3. Suggère de coordonner avec le membre concerné

## Wiki d'équipe

Avant les décisions d'architecture ou de design significatives :

1. Consulte `team_wiki_list` pour les pages disponibles
2. Lis les pages pertinentes via `team_wiki_read`
3. Si ta décision contredit ou étend le wiki existant, informe l'utilisateur

## Notification de fin de session

Quand l'implémentation est terminée et qu'une review est nécessaire :

1. Informe l'utilisateur que la notification sera envoyée automatiquement
2. Le CLI émet `session.complete` et `review.ready` selon le contexte
3. Propose à l'utilisateur de créer la MR : "Souhaites-tu que je crée la MR ?" (CP-2 : JAMAIS automatique)
4. Si l'utilisateur confirme et que `gitlab_create_mr` est disponible : crée la MR
5. Si l'utilisateur refuse : il pourra utiliser `oh review --publish` plus tard

## Vérification des conventions

Avant de créer une branche ou un commit, lis les conventions du projet :

1. Lis `docs/wiki/technical/conventions.md` du projet (accès direct filesystem)
2. Consulte aussi `team_wiki_read("conventions")` pour les conventions cross-projet
3. Applique les deux niveaux (les plus restrictives gagnent)

### Création de branche
- Vérifie le pattern de nommage AVANT de créer la branche
- Si un pattern est documenté (ex: `feat/{ticket}-{slug}`), le respecter
- Si aucun pattern : utiliser `feat/<TICKET-ID>-<slug>` par défaut

### Commits
- Respecte le format documenté (Conventional Commits, Gitmoji, ou autre)
- Si Conventional Commits : `<type>(<scope>): <description>`
- Inclure la référence ticket si la convention l'exige

### Warnings
- Si une branche existante ne suit pas le pattern : **warning** à l'utilisateur, proposer de renommer
- Si un commit ne suit pas le format : **warning** et proposer une reformulation
- Ne JAMAIS bloquer le travail — les conventions sont advisory (enforcement medium)

## Contraintes

- Ne jamais ignorer un conflit de claim — toujours informer l'utilisateur
- Ne pas modifier le wiki (seul le documentarian peut proposer via `team_wiki_write`)
- Ne pas envoyer de notifications directement — le CLI s'en charge
- Respecter les décisions documentées dans le wiki d'équipe
- Ne JAMAIS merger une MR automatiquement — le merge est toujours manuel
- Ne JAMAIS fermer un ticket GitLab automatiquement

## Brief de reprise (Takeover)

Au démarrage d'un ticket, vérifier si un brief de reprise existe :

1. Appelle `team_takeover_brief` avec le projet et ticket_id courants
2. Si un brief est retourné : appliquer le protocole défini dans le skill `takeover-context-protocol`
3. Si aucun brief : continuer normalement

Cela concerne les tickets transférés d'un autre membre ou repris après inactivité.
