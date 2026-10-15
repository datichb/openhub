> [Read in English](notifications.en.md)

# Notifications — Guide de configuration

## Présentation

openhub peut envoyer des notifications vers les plateformes de messagerie d'équipe lorsque des événements clés surviennent pendant les sessions IA. Les notifications sont envoyées vers Slack, Discord, Mattermost ou Microsoft Teams via des webhooks entrants.

### Plateformes supportées

| Plateforme | Format du payload | Nom du bot | Redirection de canal |
|------------|-------------------|------------|---------------------|
| Slack | `{"text", "username"}` | Oui | Non |
| Discord | `{"content", "username"}` | Oui | Non |
| Mattermost | `{"text", "username", "channel"}` | Oui | Oui |
| Microsoft Teams | MessageCard (`@type: MessageCard`) | Non | Non |

---

## Événements

Tous les événements d'équipe ne déclenchent pas de notifications. Le tableau ci-dessous indique quels événements sont automatiquement envoyés :

| Événement | Type | Auto-notifie | Description |
|-----------|------|:---:|-------------|
| Revue prête | `review.ready` | Oui | Revue IA terminée, MR prête pour revue humaine |
| Revue approuvée | `review.approved` | Oui | Le relecteur humain a approuvé la MR |
| Revue rejetée | `review.rejected` | Oui | Le relecteur humain a rejeté la MR |
| Proposition wiki | `wiki.proposal` | Oui | L'agent a proposé une mise à jour de page wiki |
| Notification personnalisee | `custom.notification` | Oui | L'agent a envoyé un message personnalisé via `team_notify` |
| Session terminée | `session.complete` | Non | Session IA terminée (journalisé uniquement) |
| Ticket pris en charge | `claim.taken` | Non | L'agent a pris en charge un ticket (journalisé uniquement) |
| Conflit de prise en charge | `claim.conflict` | Non | Deux agents ont tenté de prendre le même ticket (journalisé uniquement) |
| Ticket transféré | `claim.transferred` | Non | Ticket transféré entre agents (journalisé uniquement) |
| Ticket libéré | `claim.released` | Non | L'agent a libéré un ticket (journalisé uniquement) |
| Wiki accepté | `wiki.accepted` | Non | Proposition wiki acceptée (journalisé uniquement) |
| Wiki rejeté | `wiki.rejected` | Non | Proposition wiki rejetée (journalisé uniquement) |
| Constat d'audit | `audit.finding` | Non | L'audit a rapporté des constats (journalisé uniquement) |

> **Note :** Les événements « journalisé uniquement » sont enregistrés dans le journal d'événements du team-state et visibles dans l'historique des notifications du TUI, mais ne sont pas envoyés vers les plateformes externes.

---

## Configuration

Les notifications sont configurées dans le fichier `config.toml` du dépôt **team-state** (PAS dans `hub.toml`).

### Destination unique

```toml
[notification]
enabled = true
type = "slack"                    # slack | discord | mattermost | teams
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
```

### Destinations multiples

```toml
[notification]
enabled = true
bot_name = "OpenHub"              # Valeur par defaut pour toutes les destinations

[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."

[[notification.destinations]]
type = "mattermost"
webhook_url = "https://mattermost.example.com/hooks/..."
channel = "#dev-ai"               # Mattermost uniquement
bot_name = "MattermostBot"        # Redefinition par destination
```

### Configuration par plateforme

**Slack :** Créez un [Incoming Webhook](https://api.slack.com/messaging/webhooks) dans les paramètres de votre espace de travail Slack.

**Discord :** Dans les paramètres du canal > Intégrations > Webhooks > Nouveau Webhook. Copiez l'URL du webhook.

**Mattermost :** Dans la Console Système > Intégrations > Incoming Webhooks. Vous pouvez optionnellement spécifier une redefinition de `channel`.

**Microsoft Teams :** Créez un [workflow Power Automate](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) ou utilisez un connecteur de webhook entrant legacy.

---

## Processus d'installation

1. **Pendant `oh team init`** — L'assistant interactif propose la configuration des notifications comme étape optionnelle
2. **Manuellement** — Editez directement `~/.oh/team-state/config.toml`
3. **Commitez** — Poussez la configuration pour la partager avec l'équipe :
   ```bash
   cd ~/.oh/team-state && git add config.toml && git commit -m "feat: ajout des notifications" && git push
   ```

---

## Test

```bash
oh team notify test                      # Envoyer une notification de test
oh team notify test --message "Bonjour!" # Message de test personnalise
```

---

## Intégration TUI

Les notifications apparaissent également dans le TUI :

- **Notifications toast** — Apparaissent en temps réel pour tous les événements
- **Historique des notifications** — Tapez `notifications` (ou `notif`, `logs`, `messages`, `toasts`) dans l'omnibar
- **Persistance** — Les 50 dernières notifications sont stockées dans `~/.oh/notifications.jsonl`
- **Erreurs** — Les notifications de niveau erreur sont écrites sur stderr

---

## Dépannage

### Notifications non envoyées

1. Vérifiez que `enabled = true` est présent dans `[notification]`
2. Vérifiez que l'URL du webhook est correcte et accessible
3. Testez avec `oh team notify test`
4. Consultez les logs : les URLs de webhook sont masquées dans les logs pour des raisons de sécurité (`httplog.WithMaskURL`)

### Mauvais format de plateforme

Chaque plateforme attend un format de payload spécifique. Si vous obtenez `400 Bad Request`, vérifiez que le champ `type` correspond bien à la plateforme du webhook.

### Canal Mattermost introuvable

Le champ `channel` doit inclure le préfixe `#` (par exemple `#dev-ai`). S'il est omis, le canal par défaut du webhook est utilisé.

---

## Limitations connues

- **Messages de notification en français uniquement** — Toutes les chaînes de formatage des événements sont actuellement codées en dur en français. L'internationalisation des messages de notification est prévue mais pas encore implémentée.
- **Pas de filtrage par événement** — Tous les événements auto-notifies sont envoyés à toutes les destinations. Le filtrage par événement ou par destination n'est pas supporté.
- **Pas de renvoi en cas d'échec** — Si un appel webhook échoue, l'erreur est journalisée mais la notification n'est pas renvoyee.
- **Timeout de 10 secondes** — Les appels HTTP de webhook expirent après 10 secondes.

---

## Ressources

- [Guide de configuration d'équipe](team-setup.fr.md)
- [Référence du serveur MCP Team](../reference/mcp-team.fr.md) — outil `team_notify`
- [Référence de configuration](../reference/config.fr.md)

---

## Support

```bash
oh team notify test      # Tester l'envoi de notifications
oh team status           # Verifier la configuration d'equipe
```

Pour signaler un problème : [GitHub Issues](https://github.com/datichb/openhub/issues)
