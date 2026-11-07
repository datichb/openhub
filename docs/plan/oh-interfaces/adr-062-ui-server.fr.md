> Brouillon — à relire avant d'écrire dans `docs/architecture/adr/062-ui-server.fr.md` (et sa version EN)

# ADR-062 — Serveur d'interface `uiserver`

## Statut

Proposé

## Date

2026-11-05

## Contexte

L'[ADR-061](adr-061-ui-contract-architecture.fr.md) définit un contrat d'interface (`internal/ui/contract`) et ses implémentations (`internal/appsvc`). Pour que le TUI, le navigateur et l'app macOS travaillent **en même temps sur le même état**, mis à jour en direct, ce contrat doit être servi par un processus auquel chaque interface se connecte comme client. C'est le modèle d'opencode V2 : un serveur, des clients TUI et web.

Ce qui existe déjà :

- **`oh serve`** (`cmd/serve*.go`) : 127.0.0.1, jeton et cookie, contrôle du Host (`serve_auth.go`), quelques routes `/api/v1` en lecture seule, une page de tableau de bord intégrée, et un SSE qui relit tout toutes les 5 s.
- **`ohd`** ([ADR-047](../../architecture/adr/047-session-interaction-daemon.fr.md)) : un démon par utilisateur sur `~/.oh/run/ohd.sock` (droits 0600, utilisateur du pair vérifié), démarré à la demande avec un verrou. Il héberge le proxy d'identifiants ([ADR-044](../../architecture/adr/044-credential-proxy-session-limits.fr.md)), les passerelles, les crochets du plugin et le CheckpointService, et diffuse `/v1/stream`. Il s'arrête après 10 minutes sans serveur vivant.
- **Les clients** ont des besoins différents : le TUI et l'app peuvent utiliser un socket Unix ; un navigateur a besoin de TCP.

## Décision

### 1. Un processus à part, pas dans `ohd`

`uiserver` (`internal/uiserver`, lancé par `oh ui-server`, une commande cachée) est **distinct d'`ohd`** :

- `ohd` porte des choses sensibles (identifiants, jetons de groupe, crochets) : on n'ajoute pas à sa surface d'attaque une API large, ouverte au navigateur ;
- leurs cycles de vie diffèrent : `ohd` vit tant que des serveurs opencode vivent, `uiserver` tant que des clients d'interface sont connectés ;
- `uiserver` parle à `ohd` par `daemon/client` et `daemon/api`, comme le TUI aujourd'hui (règle `archtest` existante).

### 2. Deux points d'accès

| Point d'accès | Pour qui | Sécurité |
|---|---|---|
| Socket Unix `~/.oh/run/ui.sock` | TUI, app macOS (via `WKURLSchemeHandler oh://`), ligne de commande | Droits 0600, vérification de l'UID du pair (comme `ohd`) |
| TCP `127.0.0.1:<port>` | Navigateur (`oh web`) | Jeton + cookie `HttpOnly`/`SameSite=Strict`, contrôle du `Host` et de l'`Origin`, jeton CSRF sur les écritures, CSP stricte, aucune ressource externe |

`oh web` obtient de `uiserver`, par le socket, un **lien à usage unique** (de courte durée) qui ouvre le navigateur. Le serveur l'échange contre le cookie, si bien que le jeton n'apparaît ni dans l'historique ni dans les favoris. Le port est choisi au premier démarrage puis conservé, pour qu'un favori reste valable (`oh web --at-login`).

### 3. Cycle de vie

- **Démarré à la demande** par le premier client (`client.Ensure`, avec un verrou de lancement, comme `ohd`).
- **Arrêté** après N minutes sans client connecté (par défaut 10, réglage `[ui] idle_minutes`), sauf s'il a été lancé par `--at-login`.
- **Remplacé par une nouvelle version** seulement quand aucun client n'écrit (un assistant en cours bloque le remplacement).
- **Vérification de version** à la connexion : un client plus récent que le serveur demande son remplacement, un client plus ancien reçoit le code `client_outdated`.
- **Présence et focus des clients** : chaque client envoie un battement avec son type (TUI, web, app) et l'état de son focus (`ClientPresence{ID, Kind, Focused}`). Une notification système n'est envoyée que si **aucune** interface n'est au premier plan ; sinon, elle s'affiche dans l'interface active. C'est la reprise de la « détection du vrai premier plan du TUI », restée en suspens après la 5.4.
- **Windows** : comme `ohd`, `uiserver` tourne dans le processus d'oh. Le point d'accès du navigateur reste en TCP local.

### 4. API

- **Préfixe `/api/v2`**. Les routes sont dérivées du contrat : une méthode, une route, par exemple :
  - `GET /api/v2/sessions?scope=…` ;
  - `POST /api/v2/decisions/{id}` ;
  - `POST /api/v2/sessions/{id}/stop`.
- **Flux** :
  - `GET /api/v2/events?scope=…` en SSE, avec reprise par `Last-Event-ID` sur une fenêtre bornée, sinon un événement `resync` ;
  - `GET /api/v2/sessions/{id}/feed` en SSE.
- **Assistants** : `POST /api/v2/wizards` (start), `POST /api/v2/wizards/{id}/steps/{step}` (submit), `POST /api/v2/wizards/{id}/back`, `DELETE /api/v2/wizards/{id}`. Une session d'assistant appartient au client qui l'a ouverte, et expire si elle est abandonnée.
- **Erreurs** : `{"code": "...", "params": {...}}` avec un statut HTTP cohérent. Les erreurs 5xx ne détaillent jamais l'erreur interne, comme `writeError` aujourd'hui.
- **OpenAPI 3.1 généré** à partir du registre des routes et des types du contrat, versionné dans le dépôt, avec un contrôle en CI qu'il est à jour.
- **Capacités du client** envoyées à la connexion (`ClientCaps`, [ADR-061](adr-061-ui-contract-architecture.fr.md)).

### 5. Événements

- `uiserver` s'abonne à `ohd /v1/stream`, puis le traduit en événements du contrat. Il y ajoute les siens : configuration, team-state, board, wiki, notifications.
- Les écritures d'un client produisent un événement diffusé à **tous** les clients. Le TUI et le web restent donc synchronisés.
- Les écritures concurrentes sur un même fichier utilisent la détection de conflit (celle de `config.toml` en v5.3.3), étendue au team-state et au board. Un conflit renvoie le code `conflict` avec la version actuelle.

### 6. Client HTTP et tests de contrat

- `internal/ui/client` implémente le contrat par-dessus le socket ou TCP.
- Une **suite de tests de contrat** unique s'exécute contre `appsvc` (en direct) et contre `client` + `uiserver` (en HTTP) : même comportement, mêmes codes d'erreur, mêmes événements.
- Le TUI choisit son implémentation au démarrage : client (par défaut, une fois la latence mesurée) ou direct (`--direct`, Windows, tests).

### 7. Fin de `oh serve`

`oh serve` et `/api/v1` sont remplacés par `uiserver` et `oh web`. `dashboardHTML` est retiré. Pendant une version, `oh serve` affiche un message qui renvoie vers `oh web`.

### 8. Ouverture pour plus tard

L'adresse du serveur est configurable côté client (pas seulement le socket local). Une future app « client seul », connectée à un `oh` qui tourne ailleurs, reste donc possible. Ce n'est **pas** dans le périmètre : exposer `uiserver` au réseau demanderait une vraie authentification, TLS et une revue de sécurité dédiée.

## Conséquences

### Positives

- Le TUI, le navigateur et l'app partagent un même état en direct ; une décision prise dans l'un disparaît des autres.
- `ohd` garde une surface d'attaque réduite.
- Les clients sont interchangeables, et testés par la même suite de contrat.
- Le flux d'événements remplace la relecture toutes les 5 s d'`oh serve`.

### Négatives / Compromis

- Un processus de plus à démarrer, surveiller (`oh doctor`) et mettre à jour.
- Le TUI en mode client ajoute de la latence et de nouveaux cas d'erreur (serveur absent, version différente) ; le mode direct reste nécessaire.
- La reprise du flux après une coupure est bornée : au-delà, le client doit tout relire (`resync`).
- Rupture pour les scripts qui utilisaient `/api/v1` d'`oh serve`.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Héberger l'API dans `ohd` | Surface d'attaque élargie sur le processus qui porte les identifiants ; cycles de vie incompatibles (arrêt après 10 min sans serveur opencode). |
| Un serveur par client (chaque TUI ou navigateur lance le sien) | Pas d'état partagé en direct entre les interfaces, et des écritures concurrentes sans coordination. |
| Uniquement TCP pour tous les clients | Le socket Unix avec vérification de l'UID est plus sûr pour le TUI et l'app, sans jeton à transmettre. |
| gRPC ou Connect | Pas de besoin de performance qui le justifie ; HTTP/JSON et SSE sont directement utilisables par le navigateur et faciles à inspecter. |
| WebSocket au lieu de SSE | Les flux vont du serveur vers le client ; les écritures passent par des requêtes HTTP classiques. SSE gère la reconnexion et la reprise nativement. |
