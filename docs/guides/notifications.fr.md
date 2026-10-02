> [Read in English](notifications.en.md)

# Notifications — Guide de configuration

## Presentation

openhub peut envoyer des notifications vers les plateformes de messagerie d'equipe lorsque des evenements cles surviennent pendant les sessions IA. Les notifications sont envoyees vers Slack, Discord, Mattermost ou Microsoft Teams via des webhooks entrants.

### Plateformes supportees

| Plateforme | Format du payload | Nom du bot | Redirection de canal |
|------------|-------------------|------------|---------------------|
| Slack | `{"text", "username"}` | Oui | Non |
| Discord | `{"content", "username"}` | Oui | Non |
| Mattermost | `{"text", "username", "channel"}` | Oui | Oui |
| Microsoft Teams | MessageCard (`@type: MessageCard`) | Non | Non |

---

## Evenements

Tous les evenements d'equipe ne declenchent pas de notifications. Le tableau ci-dessous indique quels evenements sont automatiquement envoyes :

| Evenement | Type | Auto-notifie | Description |
|-----------|------|:---:|-------------|
| Revue prete | `review.ready` | Oui | Revue IA terminee, MR prete pour revue humaine |
| Revue approuvee | `review.approved` | Oui | Le relecteur humain a approuve la MR |
| Revue rejetee | `review.rejected` | Oui | Le relecteur humain a rejete la MR |
| Proposition wiki | `wiki.proposal` | Oui | L'agent a propose une mise a jour de page wiki |
| Notification personnalisee | `custom.notification` | Oui | L'agent a envoye un message personnalise via `team_notify` |
| Session terminee | `session.complete` | Non | Session IA terminee (journalise uniquement) |
| Ticket pris en charge | `claim.taken` | Non | L'agent a pris en charge un ticket (journalise uniquement) |
| Conflit de prise en charge | `claim.conflict` | Non | Deux agents ont tente de prendre le meme ticket (journalise uniquement) |
| Ticket transfere | `claim.transferred` | Non | Ticket transfere entre agents (journalise uniquement) |
| Ticket libere | `claim.released` | Non | L'agent a libere un ticket (journalise uniquement) |
| Wiki accepte | `wiki.accepted` | Non | Proposition wiki acceptee (journalise uniquement) |
| Wiki rejete | `wiki.rejected` | Non | Proposition wiki rejetee (journalise uniquement) |
| Constat d'audit | `audit.finding` | Non | L'audit a rapporte des constats (journalise uniquement) |

> **Note :** Les evenements « journalise uniquement » sont enregistres dans le journal d'evenements du team-state et visibles dans l'historique des notifications du TUI, mais ne sont pas envoyes vers les plateformes externes.

---

## Configuration

Les notifications sont configurees dans le fichier `config.toml` du depot **team-state** (PAS dans `hub.toml`).

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

**Slack :** Creez un [Incoming Webhook](https://api.slack.com/messaging/webhooks) dans les parametres de votre espace de travail Slack.

**Discord :** Dans les parametres du canal > Integrations > Webhooks > Nouveau Webhook. Copiez l'URL du webhook.

**Mattermost :** Dans la Console Systeme > Integrations > Incoming Webhooks. Vous pouvez optionnellement specifier une redefinition de `channel`.

**Microsoft Teams :** Creez un [workflow Power Automate](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) ou utilisez un connecteur de webhook entrant legacy.

---

## Processus d'installation

1. **Pendant `oh team init`** — L'assistant interactif propose la configuration des notifications comme etape optionnelle
2. **Manuellement** — Editez directement `~/.oh/team-state/config.toml`
3. **Commitez** — Poussez la configuration pour la partager avec l'equipe :
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

## Integration TUI

Les notifications apparaissent egalement dans le TUI :

- **Notifications toast** — Apparaissent en temps reel pour tous les evenements
- **Historique des notifications** — Tapez `notifications` (ou `notif`, `logs`, `messages`, `toasts`) dans l'omnibar
- **Persistance** — Les 50 dernieres notifications sont stockees dans `~/.oh/notifications.jsonl`
- **Erreurs** — Les notifications de niveau erreur sont ecrites sur stderr

---

## Depannage

### Notifications non envoyees

1. Verifiez que `enabled = true` est present dans `[notification]`
2. Verifiez que l'URL du webhook est correcte et accessible
3. Testez avec `oh team notify test`
4. Consultez les logs : les URLs de webhook sont masquees dans les logs pour des raisons de securite (`httplog.WithMaskURL`)

### Mauvais format de plateforme

Chaque plateforme attend un format de payload specifique. Si vous obtenez `400 Bad Request`, verifiez que le champ `type` correspond bien a la plateforme du webhook.

### Canal Mattermost introuvable

Le champ `channel` doit inclure le prefixe `#` (par exemple `#dev-ai`). S'il est omis, le canal par defaut du webhook est utilise.

---

## Limitations connues

- **Messages de notification en francais uniquement** — Toutes les chaines de formatage des evenements sont actuellement codees en dur en francais. L'internationalisation des messages de notification est prevue mais pas encore implementee.
- **Pas de filtrage par evenement** — Tous les evenements auto-notifies sont envoyes a toutes les destinations. Le filtrage par evenement ou par destination n'est pas supporte.
- **Pas de renvoi en cas d'echec** — Si un appel webhook echoue, l'erreur est journalisee mais la notification n'est pas renvoyee.
- **Timeout de 10 secondes** — Les appels HTTP de webhook expirent apres 10 secondes.

---

## Ressources

- [Guide de configuration d'equipe](team-setup.fr.md)
- [Reference du serveur MCP Team](../reference/mcp-team.fr.md) — outil `team_notify`
- [Reference de configuration](../reference/config.fr.md)

---

## Support

```bash
oh team notify test      # Tester l'envoi de notifications
oh team status           # Verifier la configuration d'equipe
```

Pour signaler un probleme : [GitHub Issues](https://github.com/datichb/openhub/issues)
