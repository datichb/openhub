# ADR-028 : Synchronisation bidirectionnelle avec le tracker externe

## Statut

Accepted

## Date

2026-07-28

## Contexte

Après l'implémentation du board d'équipe avec le cycle de vie des claims (ADR-024), deux lacunes subsistaient :
- Les claims n'entraient dans le board que via `oh claim` manuel — aucune visibilité sur les issues GitLab/Jira assignées aux membres de l'équipe
- Les changements de statut des claims (ex. : issue fermée sur GitLab) n'étaient pas répercutés sur le board sans intervention manuelle
- Les serveurs MCP GitLab et Jira existants fournissaient un accès API mais étaient limités à l'usage par les agents — aucun composant ne synchronisait les claims avec les trackers externes

De plus :
- Les équipes avaient déjà configuré les tokens GitLab/Jira pour les serveurs MCP ; dupliquer ces credentials pour une fonctionnalité de sync aurait constitué une régression en termes de sécurité et d'expérience utilisateur
- La synchronisation devait être sûre pour des membres de l'équipe concurrents (plusieurs membres ayant le board ouvert simultanément)

## Décision

Introduire un package `cli/internal/tracker/` qui :
1. Définit une interface `Tracker` avec des implémentations pour GitLab et Jira
2. Réutilise les credentials MCP existants (`[mcp.gitlab]` / `[mcp.jira]` dans `hub.toml`) — aucun nouveau stockage de credentials
3. Exécute un moteur de réconciliation (`Engine.Run()`) qui :
   - **Direction pull (tracker → claims)** : issue fermée → claim `done` ; issue rouverte → claim `in_progress` ; labels du tracker reflétés sur le claim
   - **Direction push (claims → tracker, opt-in)** : labels hub (`agent-reviewed`, `hub:done`) poussés vers l'issue si `push_labels = true` et `write_enabled` est défini sur le serveur MCP
   - **Auto-plan** : issues assignées à des membres de l'équipe sans claim → claim `planned` créé automatiquement (limité par `max_auto_plan_per_member`)
4. Utilise `~/.oh/sync-state.json` (local, absent du git team-state) pour stocker `last_sync_at` par projet — transmis comme `updated_after` aux APIs des trackers pour des récupérations incrémentales
5. Effectue un seul `CommitAndPush` par cycle de synchronisation (commit groupé) pour minimiser les conditions de concurrence
6. Est déclenché : à l'ouverture de la vue équipe (si `auto_sync = true`), sur la touche `r`, et via la commande CLI `oh team sync-tracker`

Jira utilise `statusCategory.key` (`"done"` = fermé) plutôt que les noms d'état des issues — cela fonctionne universellement pour les workflows personnalisés.

## Alternatives Considérées

| Alternative | Raison du rejet |
|---|---|
| Stockage séparé des credentials dans config.toml du team-state | Régression sécurité ; les utilisateurs ont déjà configuré les tokens pour MCP |
| Synchronisation webhook en temps réel | Nécessite une infrastructure (récepteur webhook) ; surdimensionné pour des équipes de 3-5 personnes |
| Sync intégrée dans le serveur MCP | Les serveurs MCP sont limités aux sessions agents ; la sync doit s'exécuter depuis le CLI indépendamment |
| CommitAndPush par ticket | Trop d'allers-retours git ; profil de concurrence moins bon qu'un commit groupé unique |
| Polling toutes les 5s (même que le timer du board) | Risque de dépassement des limites de taux GitLab/Jira ; un intervalle de 5 minutes + à la demande est suffisant |

## Conséquences

### Positives
- Aucun nouveau stockage de credentials — réutilise la config MCP du hub
- Les équipes voient les issues assignées sur GitLab dans le board sans claiming manuel
- Le statut des claims reste synchronisé avec le tracker sans mises à jour manuelles
- Une interface unique prend en charge GitLab et Jira (extensible à d'autres)
- La sync concurrente est sûre : commit groupé + idempotence via `ErrClaimExists`

### Négatives / Compromis
- La sync est à cohérence éventuelle — le board peut être obsolète jusqu'au prochain cycle de synchronisation
- L'auto-plan peut créer des claims indésirables si `max_auto_plan_per_member` n'est pas bien calibré
- La direction push nécessite un opt-in (`push_labels = true` + `write_enabled`) — non automatique
- Le mapping `statusCategory.key` pour Jira couvre la majorité des cas mais peut manquer des cas limites avec des catégories de statut personnalisées
