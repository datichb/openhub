> [Read in English](057-gitlab-token-per-server.en.md)

# ADR-057 — Jeton GitLab strict par serveur

## Statut

Accepté — évolue l'[ADR-028](028-tracker-sync-bidirectional.fr.md) (le tracker réutilise les jetons GitLab du MCP), complète l'[ADR-029](029-multi-team-support.fr.md) (`[[teams]]`) et l'[ADR-030](030-enforced-recommended-config.fr.md) (réglages d'équipe recommandés) ; prolonge le principe « les secrets restent sur la machine » de l'[ADR-046](046-beads-gateways.fr.md)

## Date

2026-11-02

## Contexte

oh parle à GitLab de trois façons : le dépôt d'équipe (team-state, par git), le tracker (synchro des issues, board, `oh mr create`) et le serveur MCP `gitlab` des agents. Les jetons viennent de plusieurs endroits : la variable `GITLAB_TOKEN`, la clé du tracker (`tracker_token_key`), le jeton du projet, celui de l'équipe, celui du MCP du hub, et l'ancienne clé du tracker. Une cascade choisissait le premier trouvé, **sans regarder le serveur**.

Quand le dépôt d'équipe et le tracker sont sur deux GitLab différents (par exemple un GitLab interne et gitlab.com), le jeton du dépôt d'équipe était envoyé au tracker : réponse 401, puis « token invalide ou expiré » sans dire d'où venait le jeton, quelle clé ni quel serveur. Au-delà du 401, **c'est une fuite de secret** : un jeton émis par un serveur est présenté à un autre.

L'[ADR-028](028-tracker-sync-bidirectional.fr.md) avait fait réutiliser au tracker les jetons déjà configurés pour le MCP, d'où la cascade. `tracker_token_key` ne se réglait en outre qu'avec `oh team config`, et le champ « Jeton du tracker » de team-detail affichait une valeur par défaut, écrivait la clé MCP du hub et avait une portée fausse.

## Décision

**Chaque source de jeton GitLab est rattachée à un serveur, et une source ne répond que pour le serveur qu'on appelle.** Une seule cascade (`tracker.ResolveGitLabToken`, appelée par `services/team.GitLabToken`) sert toutes les commandes : synchro, board, `oh mr create`, vérification d'identité, `oh team config status`, MCP.

### 1. Le serveur d'une source

| Source | Serveur |
|---|---|
| `GITLAB_TOKEN` | **à déclarer** : `OH_GITLAB_URL`, sinon `GITLAB_URL` |
| `tracker_token_key` et l'ancienne clé du tracker | `tracker_token_url` s'il est déclaré (à côté de la clé, au même niveau) ; sinon **implicite** : le tracker (`tracker_url` du niveau de la clé, à défaut l'effectif) |
| jeton du projet, jeton MCP du hub | l'URL du MCP GitLab (`[mcp.gitlab] url`, projet, ou URL recommandée par l'équipe) ; sans URL, **implicite** : `https://gitlab.com`, là où va le serveur MCP |
| jeton de l'équipe | `[[teams]] gitlab_url` s'il est déclaré (dépôt joint par un alias SSH, où l'hôte n'est pas un serveur) ; sinon l'hôte de `state_repo` |

Une source dont le serveur n'est ni déclaré ni implicite n'est **jamais** utilisée : seul `GITLAB_TOKEN` seul reste strict. Un serveur déclaré l'emporte sur l'implicite.

### 2. Comparer des serveurs

`tracker.ServerOf` réduit une URL, un remote git ou un hôte nu à son identité : hôte en minuscules sans utilisateur ni point final ni chemin, port seulement s'il n'est pas celui du web (80, 443), schéma ignoré. Le port d'un remote `ssh://` ou `git://` est celui de git, pas de l'API : il est écarté ; `user@hôte:chemin` se lit comme l'hôte. `tracker.SameServer` compare deux identités ; deux valeurs vides ou illisibles ne sont pas le même serveur. Deux GitLab de même hôte qui ne diffèrent que par un préfixe de chemin sont donc le même serveur (limite assumée).

### 3. Déclarer et corriger

- `tracker_token_url` se règle avec `tracker_token_key` (`oh init`, `oh team rejoin`, `oh team config`, team-detail, `oh config set`), dans `hub.toml` ou dans la config partagée de l'équipe. Une clé de `hub.toml` n'hérite pas du serveur déclaré à côté de celui de l'équipe.
- `[[teams]] gitlab_url` (`--gitlab-url`) pour un alias SSH. Les assistants ne remplissent plus la clé tout seuls : vide = garder, `-` = effacer.
- Sans jeton pour le serveur appelé, la commande s'arrête avec le serveur, les sources lues (clé, vide, pour un autre serveur, serveur non déclaré, mention « implicite »), la façon de déclarer la clé et la commande à lancer. Un refus 401 ou 403 (`TokenRejectedError`) dit la source, la clé et le serveur.

## Conséquences

### Positives

- Un jeton n'est plus jamais envoyé à un autre serveur que le sien : le 401 et la fuite disparaissent ensemble.
- Une seule cascade, un seul endroit à lire : `oh team config status`, la synchro et `oh mr create` donnent la même réponse.
- Les messages disent quoi corriger.

### Négatives / compromis

- **Rupture de la 5.1** : `GITLAB_TOKEN` demande `OH_GITLAB_URL` (ou `GITLAB_URL`). Les clés du tracker et les jetons MCP continuent de marcher pour leur serveur implicite ; il suffit de déclarer `tracker_token_url` ou `mcp.gitlab.url` quand ce serveur est un autre. Une configuration qui marchait « par chance » avec un jeton envoyé au bon serveur doit désormais le déclarer.
- Un réglage de plus (`tracker_token_url`, `gitlab_url`), propre à la machine ou partagé par l'équipe.
- Les serveurs implicites sont une commodité, pas une garantie : une clé réglée à côté d'un `tracker_url` ne répond pas pour un autre serveur.
- Pour Jira, `tracker_token_url` n'a pas de sens : le champ est masqué dans la TUI.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Garder la cascade sans serveur et améliorer seulement les messages | Le jeton resterait présenté au mauvais serveur |
| Essayer chaque jeton jusqu'à un succès | Présente un secret à un serveur qui ne l'a pas émis ; bruit dans les journaux d'audit du serveur |
| Déduire le serveur du jeton (préfixe `glpat-`) | Le préfixe ne dit pas l'instance |
| Exiger partout une déclaration explicite | Casse les configurations simples (un seul GitLab) pour rien ; d'où les serveurs implicites |
| Fusionner jeton d'équipe et jeton du tracker | Deux serveurs, deux comptes possibles : le cas qui a motivé la décision |
