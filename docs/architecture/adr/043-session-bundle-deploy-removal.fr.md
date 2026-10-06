> [Read in English](043-session-bundle-deploy-removal.en.md)

# ADR-043 — Paquet de session et suppression du déploiement par projet

## Statut

Accepté

## Date

2026-10-05

## Contexte

Jusqu'à la v4, `oh deploy` écrivait le hub dans chaque projet : `.opencode/agents/`, `.opencode/skills/`, des clés d'`opencode.json`, `.opencode/.deploy-state`, `.opencode/context-manifest.json` et `.opencode/team.json`. `oh sync` resynchronisait, et chaque worktree recevait des liens vers ces fichiers (`EnsureWorktreeConfig`). Les ADR [008](./008-stack-skills-dynamic-injection.fr.md), [010](./010-hybrid-skills-architecture.fr.md), [011](./011-external-agents-per-project.fr.md) et [016](./016-execution-path-skills.fr.md) décrivent des mécanismes appliqués à ce moment-là.

Ce modèle posait plusieurs problèmes :

- **un seul monde par dossier** : toutes les sessions du projet voyaient les 20 agents et les ~184 skills du hub, et le choix du « workflow » reposait sur des consignes dans les prompts ;
- **écritures dans les fichiers de l'utilisateur**, écarts entre ce qui était déployé et le hub, conflits avec les agents propres au projet (ADR-011) ;
- impossible de faire tourner deux workflows différents en même temps dans le même dossier ;
- opencode V2 lit tout ce qu'il trouve dans le projet (`instructions` compris) et le déploiement ne pouvait plus garantir ce que le modèle voit.

Décisions D4 et D14 : un paquet par session, construit hors du projet, et plus aucun déploiement.

## Décision

### 1. Paquet de session

`bundle.Build` (`internal/bundle`) compile un paquet à partir d'un **workflow résolu** (obligatoire : il n'y a plus de paquet sans workflow). Le paquet contient :

- **les agents membres du workflow**, assemblés : skills inlinées, puis instructions du projet intégrées au corps de l'agent (`ONBOARDING.md`, `CONVENTIONS.md`, `.claude/CLAUDE.md`, `[deploy].instruction_files`). oh n'écrit jamais `AGENTS.md`.
- **les skills à la demande**, avec leur fermeture `requires:` (cycles, dépendances introuvables et identifiants en double sont des erreurs au build). Leurs annexes (`annexes:`) sont copiées à côté du `SKILL.md`.
- **les skills générées depuis le YAML du workflow** (carte du workflow, checkpoints, règles de mode) et les skills de stack détectées dans le projet.
- **les permissions neutres**, le graphe de délégation et la profondeur maximale.
- **les MCP du projet**, filtrés par le champ `mcp:` du workflow. Le serveur MCP `workflow` d'oh est ajouté d'office.
- **les plugins déclarés, le Code Mode et le modèle par défaut**.

### 2. Emplacement et hash

- `~/.oh/bundles/<hash>/` : `bundle.json`, `agents/<id>.md`, `skills/<id>/SKILL.md` et leurs annexes. Fichiers en lecture seule (0444) ; un agent autorisé à écrire ne peut pas modifier le paquet sans demande.
- Hash = SHA-256 du `bundle.json` canonique (sans chemins) et du contenu des fichiers. La compilation est idempotente : même entrée, même dossier. Les chemins de la machine sont des variables développées au démarrage du serveur (`{{oh.bundle}}`, `{{oh.bin}}`, [ADR-038](./038-sessionspec-tool-adapters.fr.md)).
- La config rendue n'est pas écrite dans le paquet : elle est passée par `OPENCODE_CONFIG_CONTENT`. Le plugin oh est installé dans le dossier de données du groupe (`~/.oh/servers/<groupe>/`). Les données propres à une session sont dans `~/.oh/sessions/<id>/`.
- La clé de groupe de serveur contient le hash du paquet : deux workflows (ou deux versions) n'ont jamais le même serveur.
- `oh bundle build|show <workflow> [--budget] [--json]` construit ou décrit un paquet ; `oh skill budget` en est un alias.

### 3. Suppression du déploiement

- `internal/deploy` est supprimé. Ce qui sert encore au paquet passe dans `internal/bricks` : frontmatter et assemblage des agents, bases de permissions, cascade des modèles, skills de stack, fichiers d'instructions.
- `oh deploy` et `oh sync` restent pendant v5.x comme des commandes qui expliquent la migration (code de sortie 0).
- Supprimés aussi :
  - `Project.Agents` (migrations v36, puis v37) ;
  - les étapes Agents et Déploiement des assistants ;
  - `[deploy].disable_native_agents` (le monde fermé est imposé, voir [ADR-041](./041-closed-world-isolation.fr.md)) ;
  - `EnsureWorktreeConfig` ;
  - la section Déploiement du mode projet de la TUI.
- La vue `project.agents` devient le **Catalogue des briques**, en lecture seule : agents et skills, origine, coût estimé, workflows qui les livrent.
- Les agents propres à une équipe ou à un projet passent par le catalogue de briques du team-state ([ADR-040](./040-workflows-team-state-governance.fr.md)). `.opencode/agents` n'est plus lu (`OPENCODE_DISABLE_PROJECT_CONFIG=1`).

### 4. Nettoyage des projets : `oh migrate deploy-cleanup`

`oh migrate deploy-cleanup [-p] [--dry-run] [--diff] [--yes] [--json]` (`internal/deploycleanup`) traite chaque projet enregistré :

- il supprime `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json` et `.opencode/team.json` ;
- il retire d'`opencode.json` **seulement** une clé d'un type qu'écrivait `oh deploy` **et** dont la valeur est encore celle du `config_snapshot` de `.deploy-state`. Une clé modifiée depuis est gardée et signalée. Sans `.deploy-state`, le fichier n'est pas touché. L'ordre des clés est conservé ;
- il affiche un diff et demande une confirmation (`--yes` sans terminal).

Le nettoyage est proposé une fois au démarrage, CLI et TUI (écran dédié, commande `cleanup`), et Doctor signale les restes.

## Conséquences

### Positives

- Monde fermé par session : `cadrage` présente 4 agents et 30 skills au modèle, contre 20 agents et ~184 skills avec l'ancien déploiement complet.
- Plus aucune écriture dans le projet de l'utilisateur, hors `oh migrate deploy-cleanup` avec diff et confirmation.
- Plusieurs workflows et plusieurs sessions peuvent tourner sur le même projet, chacun avec son paquet.
- Paquet reproductible et immuable : le conteneur le monte en lecture seule, le job distant le télécharge par son hash ([ADR-045](./045-execution-environments.fr.md)).
- Plus de dérive entre le hub et ce qui est déployé : une session utilise toujours les briques au moment de son lancement.

### Négatives / Compromis

- Aucune purge des paquets n'est implémentée (le plan prévoyait 30 jours sans référence) : `~/.oh/bundles/` grossit.
- Toute modification d'une brique change le hash : nouveau paquet, nouveau groupe et nouveau serveur au lancement suivant. Une session en cours garde son paquet.
- Les agents placés à la main dans `.opencode/agents` ne sont plus chargés : il faut les migrer dans le catalogue de l'équipe ou d'un espace solo.
- Le nettoyage est prudent : une clé d'`opencode.json` modifiée après le déploiement reste, et les liens `.opencode` des worktrees créés par l'ancien `EnsureWorktreeConfig` ne sont pas parcourus.
- Le budget de tokens d'un paquet est une estimation (≈ 4 caractères par token) ; la mesure réelle est au backlog (BL-14).
- Les ADR 008, 010, 011 et 016 décrivent leur mécanisme dans le cadre du déploiement ; le comportement actuel se lit dans le paquet.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Garder `oh deploy` en filtrant par workflow | Toujours un seul monde par dossier, toujours des écritures dans le projet, et deux sessions de workflows différents se gêneraient. |
| Déployer dans un worktree par session | Coûteux, toujours dans les fichiers de l'utilisateur, et ne couvre pas le dossier principal du projet. |
| Paquet dans le projet (`.oh/` ignoré par git) | Écriture dans le dépôt, risque de commit, et opencode lirait ce qu'il trouve dans le projet. |
| Injecter agents et skills par l'API `instructions` par session (S8) | Expérimental, ne couvre ni les agents ni les permissions : à l'étude seulement (P1-T29). |
| Retirer d'`opencode.json` toutes les clés qu'oh a pu écrire | Le snapshot contient aussi les clés de l'utilisateur : risque de supprimer sa configuration. |
| Supprimer `oh deploy` sans commande de remplacement | Les scripts et habitudes existants échoueraient sans explication. |
