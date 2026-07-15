# ADR-024 : Repository Team-State

## Statut

Accepté

## Date

2026-07-07

## Contexte

L'opencode-hub a été conçu comme un système d'orchestration mono-développeur. Avec l'adoption par des équipes de 3 à 5 développeurs, plusieurs lacunes de collaboration sont apparues :

- Pas de visibilité sur qui travaille sur quel ticket
- Pas de moyen de partager des connaissances cross-projet accessibles aux agents IA
- Pas de mécanisme de notification quand les sessions se terminent ou qu'une review est prête
- Les sessions d'agents sont isolées — pas d'accès au contexte d'équipe

Il fallait un mécanisme d'état partagé qui :
1. Ne pollue PAS le repo du projet avec des fichiers de méta-coordination
2. Ne vit PAS dans le hub (réservé aux agents/skills, pas à l'état runtime)
3. Est accessible à tous les membres de l'équipe
4. Est accessible aux agents IA pendant leurs sessions
5. Supporte les équipes multi-projets
6. Reste Git-natif (pas d'infrastructure supplémentaire)

## Décision

Introduire un **repo Git dédié** (`team-state`) géré de manière transparente par le CLI `oh`. Ce repo contient :

- `members.toml` — registre des membres de l'équipe
- `config.toml` — paramètres de notification (webhook Mattermost)
- `projects/<name>/claims/` — réservations de tickets (fichiers TOML)
- `projects/<name>/events/` — journal d'activité (JSONL mensuel)
- `wiki/` — base de connaissances cross-projet
- `wiki/.pending/` — propositions wiki en attente de validation humaine
- `reports/` — rapports d'équipe générés

Le repo est cloné dans `~/.oh/team-state/` et synchronisé automatiquement (pull avant lecture, push après écriture).

Les agents IA accèdent aux données d'équipe via un **serveur MCP** (`team-mcp`) qui lit depuis le clone local. Cela évite les problèmes d'accès filesystem et fournit une interface propre basée sur des outils.

## Alternatives envisagées

| Alternative | Raison du rejet |
|---|---|
| Fichiers dans le repo projet | Pollue l'historique du code, conflits de merge sur des fichiers non-fonctionnels |
| Fichiers dans le repo hub | Le hub est pour les définitions canoniques, pas l'état runtime |
| Git notes | API limitée, partage complexe, pas adapté aux données structurées |
| API GitLab comme backend | Crée une dépendance forte, latence à chaque opération |
| SQLite partagé via NFS | Nécessite une infrastructure, pas Git-natif |
| Branche orpheline dans le projet | Workflow exotique, pas visible en opérations normales |

## Conséquences

### Positives
- Zéro infrastructure : juste un repo Git sur GitLab/GitHub
- Trace d'audit complète : l'historique Git montre qui a fait quoi quand
- Fonctionne hors-ligne : utilise les données stale si le réseau est indisponible
- Multi-projet : un seul repo sert tous les projets
- Accessible aux IA : le serveur MCP fournit une interface propre lecture/écriture

### Négatives
- Cohérence éventuelle : les données ne sont fraîches qu'après `git pull`
- Résolution de conflits nécessaire sur les écritures concurrentes (atténué par pull-rebase-retry)
- Un repo supplémentaire à gérer (atténué : totalement transparent pour l'utilisateur)
- Un fichier par claim peut générer beaucoup de petits fichiers (acceptable pour 3-5 devs)

### Neutres
- Les membres doivent fournir l'URL du repo lors de `oh team init`
- Le repo doit être pré-créé manuellement (documenté, setup unique)
- `oh team init` crée automatiquement `config.toml` et `policies.toml` si absents (wizard adaptatif multi-étapes)

## Implémentation

- Package : `cli/internal/teamstate/`
- Serveur MCP : `cli/internal/mcp/team/`
- Notifications : `cli/internal/notify/`
- Commandes CLI : `oh team init|status|activity`, `oh claim|release`
- Skills : `skills/shared/team-awareness.md`, `skills/orchestrator/team-coordination.md`
