> [Read in English](046-beads-gateways.en.md)

# ADR-046 — Beads sur la machine et passerelles

## Statut

Accepté

## Date

2026-10-05

## Contexte

Beads (`bd`) est la source des tickets du projet, et sa base vit sur la machine. Les serveurs MCP d'oh (gitlab, figma, jira, gslides, linear, github, team, workflow) lisent leurs jetons dans le trousseau de la machine. En local, l'agent les appelle directement.

En conteneur et à distance, cela ne marche plus :

- la base Beads n'est pas dans l'image ;
- copier les jetons dans le conteneur casserait la règle « les secrets restent sur la machine » (D10) ;
- un job distant ne peut pas joindre la machine.

Il fallait aussi qu'un agent ne puisse faire dans Beads que ce que son workflow autorise (`beads.allow`, [ADR-039](./039-declarative-workflows-oh-v1.fr.md)).

Décision D9 : Beads reste toujours sur la machine.

- **local** : `bd` direct ;
- **conteneur** : faux `bd` qui passe par une passerelle d'oh ;
- **distant** : instantané en entrée, journal en sortie, rejoué localement avec détection des conflits.

## Décision

### 1. Passerelle Beads (conteneur)

- **Faux `bd`** (`cmd/oh-bd`, Go, compilé pour `linux/{amd64,arm64}`, embarqué dans l'image du projet). Il envoie `argv`, le dossier courant et, seulement pour `--stdin` ou `-`, l'entrée standard, en `POST $OH_GATEWAY_URL/beads/v1/exec` avec `Authorization: Bearer $OH_GATEWAY_TOKEN`. Il relaie stdout, stderr et le code retour.
- **Passerelle** (`internal/gateway`, dans le démon, sur les écoutes du proxy, préfixe `/oh-gateway`) :
  - **liste blanche** = `beads.allow` du workflow. Sans bloc `beads:`, lecture seule (`show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`) ; une liste vide refuse tout ;
  - options globales qui changent de base ou de dossier refusées partout dans la commande (`--db`, `-C`, `--global`…) ;
  - dossier courant et chemins de fichiers traduits vers la machine et limités aux dossiers de la session, liens résolus ; paquet et données du groupe exclus ;
  - le vrai `bd` est lancé sans shell, dans le dossier de la session (délai de 2 min, 8 Mio par flux).
- **Jetons** `ohg_…` :
  - un par session et par sous-session, transmis par l'environnement de session ;
  - le démon n'en garde que l'empreinte (`~/.oh/run/gateway.json`, 0600) ;
  - valides tant que la session est ouverte et éveillée, révoqués avec le groupe ;
  - opencode 2.0.20 ne transmet pas l'environnement de session aux sous-agents : le démon l'applique à chaque sous-session qu'il rattache, avec un jeton propre.
- En local, `bd` reste direct.

### 2. Passerelle MCP

- Pour un runtime hors machine, l'adaptateur déclare chaque serveur du paquet de forme `oh mcp serve <nom>` en `type: remote` :
  - URL `…/oh/v1/hooks/mcp/<nom>` ;
  - en-tête `Authorization: Bearer {env:<variable du jeton du proxy>}`, développé par opencode : le jeton n'est jamais écrit dans la config ;
  - le paquet et son hash ne changent pas.
- Le démon lance la commande du serveur **sur la machine**, relue dans le paquet immuable, avec l'exécutable oh courant. C'est ce processus qui lit son jeton dans le trousseau, comme en local.
- **Pont générique stdio ↔ HTTP** :
  - transport « streamable HTTP » avec réponses JSON ;
  - identifiants de requête réécrits, pour plusieurs clients par processus ;
  - un processus par groupe et par serveur, relancé s'il meurt et arrêté avec le groupe.
- Un appel dont le `_meta` désigne une session d'un autre groupe est refusé (403). Le serveur `workflow` ([ADR-042](./042-checkpoints-headless-decisions.fr.md)) fonctionne ainsi à l'identique en conteneur.
- Les autres serveurs MCP locaux restent dans le conteneur.

### 3. Distant : instantané et journal

- **Avant l'envoi** :
  - réservation (`bd update --claim`, puis claim team-state ; annulée si l'envoi échoue) ;
  - instantané des tickets concernés, de leurs dépendances et de leurs enfants (`bd show --json`), avec la **révision** de chaque ticket ;
  - l'instantané voyage dans l'enveloppe de session ([ADR-045](./045-execution-environments.fr.md)).
- **Dans le job**, le faux `bd` en mode journal (`OH_BD_MODE=journal`) est derrière la passerelle du démon du job : `beads.allow` et les options refusées s'appliquent sans changement.
  - Les lectures sont servies par l'instantané.
  - Les écritures sont enregistrées dans `journal.jsonl` (`beadswire.JournalEntry` : ordre, argv, dossier, entrée standard) et appliquées à la copie.
  - `create` renvoie un identifiant provisoire `pending-<n>`.
- **Au retour**, `oh session resolve <id>` rejoue le journal :
  - chaque entrée est revérifiée sur la machine (liste blanche, options, identifiants de la session) ;
  - **conflit** = ticket écrit par la session dont la révision actuelle diffère de celle de l'instantané ;
  - par ticket : garder la version locale, appliquer la distante, ou fusionner les notes ;
  - rien n'est appliqué sans résolution ni confirmation ;
  - les identifiants provisoires sont remplacés ; le rejeu est relançable (`replay.json`).
- **Pendant le pipeline**, le démon entretient le bail Beads (`bd heartbeat`) des tickets réservés.

## Conséquences

### Positives

- Une seule base Beads, sur la machine, quel que soit l'environnement d'exécution.
- La liste blanche du workflow s'applique en conteneur et à distance, par le même code.
- Aucun jeton dans le conteneur : `env` ne montre que `OH_GATEWAY_URL` et `ohg_…`. Les serveurs MCP d'oh fonctionnent sans modification.
- À distance, aucun changement Beads n'est perdu ni appliqué en silence.

### Négatives / Compromis

- `bd` interactif (`edit`, `create-form`) est indisponible en conteneur et à distance (pas de terminal).
- Passerelle MCP : pas de notifications serveur → client, corps de requête limité à 1 Mio, stderr des serveurs gardé seulement à la mort du processus.
- La seconde écoute Linux, qui sert aussi les passerelles, n'est testée qu'en unitaire. Les MCP gitlab/team avec de vrais jetons n'ont pas été testés en conteneur.
- À distance, les serveurs MCP qui ont besoin du trousseau de la machine sont indisponibles (BL-18) ; seul `workflow` fonctionne.
- Le rejeu repose sur la révision que donne `bd show` (vérifiée sur bd 1.3.1) : un changement de format de Beads le casserait.
- Le rattachement des sous-sessions laisse une course possible : un premier shell lancé avant que le démon ne voie la sous-session n'aurait pas l'environnement. Elle n'a pas été observée.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Monter la base Beads dans le conteneur | Accès complet, sans liste blanche ; impossible à distance. |
| Copier les jetons MCP dans le conteneur | Contraire à D10 : les secrets seraient lisibles par l'agent. |
| Réécrire chaque serveur MCP en HTTP | Un pont générique couvre tous les serveurs d'oh sans les modifier. |
| Passer le jeton de la passerelle dans l'environnement du serveur | Le shell de session n'hérite pas de l'environnement du serveur (F30) ; il passe par l'environnement de session. |
| Appliquer directement les écritures du job distant à la base | Le job ne joint pas la machine, et une écriture concurrente serait écrasée sans le savoir. |
