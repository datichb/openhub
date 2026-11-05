# ADR-028 : Synchronisation bidirectionnelle avec le tracker externe

## Statut

Accepted — **Évolué par [ADR-057](./057-gitlab-token-per-server.fr.md)** (jeton GitLab strict par serveur)

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
   - **Direction pull (tracker → claims)** : issue fermée → claim `done` ; issue rouverte → claim `in_progress` ; labels du tracker reflétés sur le claim. Le statut n'est réécrit **que si l'issue a changé depuis le passage précédent** (voir l'amendement 5.3.3)
   - **Direction push (claims → tracker, opt-in)** : labels hub (`agent-reviewed`, `hub:done`) poussés vers l'issue si `push_labels = true` et `write_enabled` est défini sur le serveur MCP ; depuis la 5.3.3, le statut du hub l'est aussi, sous la forme du label rattaché à sa colonne
   - **Auto-plan** : issues assignées à des membres de l'équipe sans claim → claim créé automatiquement dans la colonne que son statut désigne, sans plafond par membre (le membre est borné par la limite de travail en cours `max_in_progress_per_member`, vérifiée là où un ticket est pris ou démarré, jamais par la synchro ; elle remplace l'ancienne `max_auto_plan_per_member`)
4. Utilise `~/.oh/sync-state.json` (local, absent du git team-state) pour stocker `last_sync_at` par projet — transmis comme `updated_after` aux APIs des trackers pour des récupérations incrémentales
5. Effectue un seul `CommitAndPush` par cycle de synchronisation (commit groupé) pour minimiser les conditions de concurrence
6. Est déclenché : par le démon toutes les `sync_interval_minutes` et à l'ouverture du board (si `auto_sync = true`, un seul membre par intervalle : voir l'amendement 5.4.0), sur la touche `r`, et via la commande CLI `oh team sync-tracker`

Jira utilise `statusCategory.key` (`"done"` = fermé) plutôt que les noms d'état des issues — cela fonctionne universellement pour les workflows personnalisés.

## Amendement 5.3.3 : le tracker ne réécrit que ce qui a changé

Le texte ci-dessus laissait lire la synchro comme « le statut du claim = le statut mappé de l'issue, à chaque passage ». Sur GitLab, une issue ouverte sans label de statut donne toujours `in_progress` : une carte passée en review, validation, bloqué ou fait dans le hub (board, `oh mr create`, verdict, fin de session) y revenait à la synchro suivante de n'importe quel membre. Aucune direction ne repoussait le statut du hub vers le tracker. Les labels posés par le hub (`session-ended`, `review`, `ready-for-review`) étaient aussi retirés, car tout ce qui n'était pas `agent-reviewed`, `needs-human-review` ou `hub:done` passait pour un miroir du tracker.

- Chaque claim garde l'état de l'issue (`tracker_state`) et ses labels (`tracker_labels`) vus au dernier passage. Le statut du claim n'est réécrit par le mapping que si l'état, le nom de statut ou les labels rattachés à une colonne (`label_status_mapping`) ont changé depuis ; la règle « bloqué d'abord, puis le plus avancé » choisit alors le statut. Un claim sans cette mémoire est mémorisé au premier passage sans rien écraser, sauf une issue fermée dont le claim n'est pas terminal (et une réouverture d'un claim terminal que le claim savait fermée).
- Un label n'est retiré d'un claim que s'il était sur l'issue au passage précédent et n'y est plus.
- Avec `push_labels` (et `write_enabled`), quand le statut du hub diffère de celui de l'issue sans que l'issue ait changé, le label rattaché à la colonne du claim est posé et les labels rattachés à d'autres colonnes sont retirés. Sans label rattaché à la colonne, rien n'est poussé. Sans `push_labels`, rien n'est écrit dans le tracker.
- Une synchro dont la poussée vers le team-state échoue renvoie l'erreur et n'enregistre pas `sync-state.json`.

Les champs sont facultatifs dans le TOML d'un claim : un `oh` plus ancien les ignore, un claim d'avant 5.3.3 se lit sans eux.

## Amendement 5.4.0 : la synchronisation automatique, un membre par intervalle

Jusqu'à la 5.3, `auto_sync` et `sync_interval_minutes` étaient lus pour l'affichage seulement : la promesse du point 6 n'était pas tenue. Elle l'est depuis la 5.4.0 :

- **Le démon** de chaque membre a une tâche `tracker-sync` qui regarde chaque minute les équipes dont `auto_sync` est activé et lance `Engine.RunAuto` quand l'intervalle de l'équipe est écoulé ; le board lance la même synchro à son ouverture. Une synchro demandée (`Engine.Run`) reste immédiate.
- **Élection par bail.** Les membres ont chacun leur démon, un seul synchronise par intervalle. La synchro écrit `sync.lock` à la racine du team-state (`member`, `acquired_at`, `expires_at` = maintenant + l'intervalle) **dans son propre commit**. `RunAuto` lit le bail (d'abord dans le clone, sans réseau, puis dans la transaction, après le pull) : bail en cours → `LeaseHeldError`, rien n'est écrit. Deux membres qui partent ensemble sont départagés par le push : le second voit son push refusé, rejoue la transaction et trouve le bail du premier. Un bail expiré est repris par la première synchro qui passe (le membre disparu ne bloque pas plus d'un intervalle). Le bail n'est pris que par une synchro qui est allée au bout (pas de jeton refusé) et seulement si `auto_sync` est activé. Une synchro demandée prend le bail sans l'attendre.
- **Coupe-circuit.** Trois refus (HTTP 401) du même jeton en suivant arrêtent la synchro automatique ; le compte est dans `sync-state.json` (donc d'un processus à l'autre) avec l'empreinte du jeton, et un autre jeton le remet à zéro. Un 403 est une erreur du projet de tracker, pas du jeton.
- **Les claims.** Seuls les projets de l'équipe sont liés à son tracker ; une issue a un seul claim par projet de tracker ; l'identifiant suit `ticket_pattern` ; les libérations faites par la synchro elle-même (nettoyage des claims faits, expirés) sont nommées dans le corps de son commit (`Released-claim:`) pour que ni une autre machine ni l'auto-plan ne les recréent.

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
- L'auto-plan met toute issue assignée sur le board : un membre qui en a beaucoup les voit toutes ; `max_in_progress_per_member` ne limite que ce qu'il peut prendre ou démarrer
- La direction push nécessite un opt-in (`push_labels = true` + `write_enabled`) — non automatique
- Le mapping `statusCategory.key` pour Jira couvre la majorité des cas mais peut manquer des cas limites avec des catégories de statut personnalisées
