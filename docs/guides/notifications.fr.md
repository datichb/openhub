> [Read in English](notifications.en.md)

# Notifications — Guide de configuration

## Trois canaux, un seul réglage personnel

oh vous prévient de trois façons, qui ne se confondent pas :

| Canal | Qui le voit | Où se règle-t-il ? |
|-------|-------------|--------------------|
| **Notification système** (centre de notifications de macOS, `notify-send`…) | vous, même si oh est fermé : c'est le **démon** qui l'affiche | `hub.toml` `[notifications]`, Réglages › Notifications |
| **Toast de la TUI** (et son historique, `~/.oh/notifications.jsonl`) | vous, quand la TUI est ouverte | le même réglage |
| **Webhook d'équipe** (Mattermost, Slack, Discord, Teams) | toute l'équipe | `config.toml` du team-state, `[notification]` |

Les deux premiers sont **personnels** (votre `hub.toml`) ; le troisième est **partagé** (le dépôt team-state). Ce guide décrit les trois.

---

## Qui est prévenu, de quoi, et comment (réglages par défaut)

● activé par défaut · ○ disponible, désactivé par défaut.

| Événement | Pour | Système | Toast | Webhook | Mention | Dédup / rappel |
|---|---|:-:|:-:|:-:|:-:|---|
| Checkpoint / question / permission | moi | ● (sauf session attachée) | ● | ○ | ○ | notification « mise en veille avec une décision en attente » ; la décision est reprise au réveil ; rappel unique au redémarrage du démon |
| Budget épuisé | moi | ● | ● | ○ | — | une fois par session et par plafond, pas à chaque étape |
| Échec ou plantage de session | moi | ● | ● | ○ | — | une notification par session et par 10 minutes |
| Identifiant refusé, jeton expiré (LLM, tracker, git), serveur SSH inconnu | moi | ● avec la cause | ● | — | — | une par clé |
| Étape terminée (session non attachée) | moi | ○ | ● | — | — | regroupé |
| Session ou workflow terminé | moi | ● | ● | ○ `session.complete` | — | — |
| Session distante prête ou en échec | moi | ● | ● | — | — | — |
| MR ou revue prête | l'équipe et le relecteur | — | ○ | ● | ● relecteur | — |
| Verdict de revue | l'équipe et l'auteur | — | ○ | ● | ● auteur | — |
| Réservation transférée à moi | le destinataire | ○ | ● | ○ | ● | — |
| Réservation prise, libérée, en conflit | l'équipe | — | — | ○ | — | — |
| Réservation expirée ou bientôt | le propriétaire | ● | ● | ○ | ● | une par réservation |
| Synchro du tracker | moi | — | récapitulatif | — | — | un échec répété n'est dit qu'une fois |
| Wiki proposé, accepté, rejeté | l'équipe et l'auteur | — | ○ | ● | ○ | même chemin dans la TUI et en CLI |
| Configuration d'équipe modifiée (`config_changed`) | l'équipe | — | ○ | ○ | — | — |

> Les colonnes « Toast » des événements d'équipe (revue, wiki, réservation, configuration) disent ce que **vous** pouvez choisir de voir sur votre écran ; l'annonce à l'équipe passe par le webhook.

---

## Vos réglages : `hub.toml` `[notifications]`

Visibles dans **Réglages › Notifications** ; **rechargés à chaud** : le démon et la TUI relisent `hub.toml` quand il change, sans redémarrage.

```toml
[notifications]
system = true              # notifications système du démon (faux : seuls les toasts restent)
sound = true               # son du système avec la notification
quiet_hours = "22:00-08:00" # pas de notification système ni de son (peut passer minuit) ; les toasts restent
when_attached = false      # notifier aussi une session à laquelle un terminal est attaché
when_tui_focused = true    # notifier aussi quand une TUI est ouverte (oh ne voit pas quelle fenêtre est devant)

[notifications.events]     # system | toast | off, par type d'événement
step_done = "system"
claim = "toast"
```

Valeurs de `events.<type>` : `system` (notification système **et** toast), `toast` (toast seulement), `off` (rien). Types : `checkpoint`, `question`, `permission`, `sleep`, `budget`, `error`, `credential`, `step_done`, `session_complete`, `remote`, `memory`, `review`, `verdict`, `claim_transferred`, `claim`, `claim_expired`, `tracker_sync`, `wiki`, `config_changed`. Seuls les choix différents du défaut sont écrits.

L'ancien `[session] notify = "off"` éteint toujours les notifications système tant que `[notifications] system` n'est pas écrit.

Une notification système par famille (décisions, budget, erreurs, alertes, fin d'étape…) : une famille ne remplace pas l'autre dans le centre de notifications. Les alertes disent leur **cause** dans le titre (« Clé Bedrock refusée par AWS », « Accès au team-state refusé », « Serveur SSH inconnu »…), une notification par clé.

### Décision en attente, TUI fermée

La veille d'un serveur reste à `idle_sleep_minutes` (5 min). Si une décision (permission, question, checkpoint) attend à ce moment, oh la **garde** dans « À traiter » (marquée en veille), vous prévient (« mise en veille avec une décision en attente ») et, au réveil de la session :

- si l'outil repose la demande, elle redevient une décision normale ;
- sinon la demande est perdue avec le serveur : oh la clôt et **demande à l'agent de la reposer** (une consigne mise en file).

Tant que la session dort, répondre à la décision est refusé avec un message qui dit de reprendre la session (`oh session resume`). Les décisions encore ouvertes depuis plus d'une minute au démarrage du démon (et de moins de 24 h) sont rappelées **une fois**.

---

## Le webhook d'équipe

### Événements envoyés

Chaque destination reçoit, par défaut, la **revue** (`review.*`), le **wiki** (`wiki.*`) et les messages des agents (`custom.notification`). Les autres événements sont envoyés si la destination les demande avec `events` : un type (`claim.transferred`), une famille (`claim.*`) ou `*`.

| Événement | Type | Par défaut |
|-----------|------|:---:|
| Revue prête / approuvée / rejetée | `review.ready`, `review.approved`, `review.rejected` | Oui |
| Wiki proposé / accepté / rejeté | `wiki.proposal`, `wiki.accepted`, `wiki.rejected` | Oui |
| Message d'un agent | `custom.notification` | Oui |
| Réservation prise / libérée / transférée / en conflit | `claim.taken`, `claim.released`, `claim.transferred`, `claim.conflict` | Non |
| Réservation bientôt expirée / expirée | `claim.expiring`, `claim.expired` | Non |
| Session terminée | `session.complete` | Non |
| Constat d'audit | `audit.finding` | Non |
| Workflow publié, restauré, archivé | `workflow.published`, `workflow.restored`, `workflow.archived` | Non |
| Configuration d'équipe modifiée | `config_changed` | Non |

Tout événement écrit dans l'activité de l'équipe est annoncé **une fois, quand sa transaction est validée**, quel que soit l'auteur (une commande, le démon, un agent, la TUI). Une MR créée par un agent (`gitlab_create_mr`) annonce `review.ready` comme `oh mr create`.

### Mentions

Le relecteur demandé (`oh mr create --reviewer`), l'auteur d'une revue ou d'une proposition de wiki, le membre à qui un ticket est transféré, le propriétaire d'une réservation qui expire sont **mentionnés** avec le `mattermost_username` de leur fiche (`oh team profile`), sur Mattermost et Slack. Sur Discord et Teams le texte donne leur nom, sans mention. `mention = false` les coupe. Une personne sans `mattermost_username` n'est pas mentionnée.

### Configuration

Dans le `config.toml` du team-state (pas dans `hub.toml`) :

```toml
[notification]
enabled = true                    # sans cet interrupteur, rien n'est envoyé
type = "slack"                    # slack | discord | mattermost | teams ; déduit du webhook s'il manque
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
events = ["review.*", "wiki.*", "claim.transferred"]   # facultatif
mention = true                    # facultatif (vrai par défaut)
```

Plusieurs destinations, chacune avec ses `events` et son `mention` :

```toml
[notification]
enabled = true
bot_name = "OpenHub"

[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "mattermost"
webhook_url = "https://mattermost.example.com/hooks/..."
channel = "dev-ai"
events = ["*"]
```

Sans terminal : `oh team config set notification.events "review.*, claim.*"`, `oh team config set notification.destinations.0.mention false`. Dans la TUI : détail de l'équipe › Notifications (publié par Ctrl+S).

**Le webhook est un secret** : il est écrit dans le dépôt team-state, donc masqué dans tous les écrans, récapitulatifs, événements et messages d'erreur ; `oh doctor` avertit si ce dépôt est lisible par tout le monde.

### Configuration par plateforme

| Plateforme | Format du payload | Nom du bot | Canal | Mention |
|------------|-------------------|------------|-------|---------|
| Slack | `{"text", "username", "link_names"}` | Oui | Non | `@nom` |
| Discord | `{"content", "username"}` (aucune mention analysée) | Oui | Non | Non |
| Mattermost | `{"text", "username", "channel"}` | Oui | Oui | `@nom` |
| Microsoft Teams | MessageCard | Non | Non | Non |

**Slack :** créez un [Incoming Webhook](https://api.slack.com/messaging/webhooks). **Discord :** canal › Intégrations › Webhooks. **Mattermost :** Console Système › Intégrations › Incoming Webhooks. **Teams :** un [workflow Power Automate](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) ou un connecteur entrant.

### Texte et fiabilité

- Le texte est écrit **dans la langue de l'expéditeur**, avec les noms des membres et des projets (pas leurs identifiants) et les symboles de l'activité (`✔ ✗ ●`), valables sur toutes les plateformes.
- Les destinations sont jointes **en parallèle** (8 s au plus chacune).
- Un message qui n'atteint pas son webhook (serveur arrêté, réseau coupé, 5xx, 429) est **gardé** dans `~/.oh/notify/outbox/` ; le démon le renvoie après 1 min, 5 min, 15 min, 1 h, 3 h puis 6 h, puis abandonne et le dit dans l'historique des notifications. Un webhook révoqué (404, 403…) n'est pas renvoyé : l'erreur est dite tout de suite.
- Les erreurs ne contiennent jamais l'adresse du webhook.

---

## L'outil MCP `notify` des agents

Un agent peut écrire à l'équipe avec `team_notify`. Le message est **signé** (membre et projet), limité à 1000 caractères et à 5 messages par 10 minutes, et ne peut pas appeler `@channel`, `@all`, `@here`. Si les notifications sont désactivées, sans destination, ou en échec, l'outil **répond une erreur** : l'agent ne croit plus avoir prévenu l'équipe.

---

## Test et diagnostic

```bash
oh team notify test                      # envoie un message de test aux destinations
oh team notify test --message "Bonjour!" # message personnalisé
oh doctor                                # contrôle « Notifications »
```

`oh doctor` vérifie l'outil de notification système (oh ne peut pas savoir si le système l'autorise : si rien ne s'affiche, autorisez les notifications de votre terminal), signale les messages qui attendent un nouvel essai, et que le serveur de chaque webhook répond (sans y envoyer de message).

L'historique (`notifications` dans l'omnibar) garde les 500 dernières lignes de `~/.oh/notifications.jsonl` : les toasts, les synchros du tracker faites par le démon, les réservations qui expirent, les webhooks abandonnés. Un même message n'est pas affiché ni enregistré deux fois en 5 secondes.

---

## Dépannage

1. Rien n'arrive sur le webhook : `enabled = true` ? un `type` et un `webhook_url` ? l'événement est-il dans `events` (revue et wiki seulement par défaut) ? `oh team notify test`, puis `oh doctor`.
2. Pas de notification système : `[notifications] system`, `quiet_hours`, le type d'événement (`events.<type>` à `system`), une session attachée (`when_attached`), une TUI ouverte (`when_tui_focused`), et l'autorisation du système.
3. `400 Bad Request` : le `type` ne correspond pas à la plateforme du webhook.
4. Canal Mattermost introuvable : le champ `channel` est le nom du canal.

---

## Ressources

- [Guide de configuration d'équipe](team-setup.fr.md)
- [Référence du serveur MCP Team](../reference/mcp-team.fr.md) — outil `team_notify`
- [Référence de configuration](../reference/config.fr.md)

Pour signaler un problème : [GitHub Issues](https://github.com/datichb/openhub/issues)
