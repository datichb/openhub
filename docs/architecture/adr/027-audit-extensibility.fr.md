# ADR-027 : Registre dynamique, télémétrie des agents et marketplace de skills

## Statut

Accepté

## Date

2026-07-22

## Contexte

Un audit architectural a identifié trois lacunes dans la plateforme `oh` qui limitaient son extensibilité et son adoption à long terme :

1. **Listes de plugins et de serveurs MCP codées en dur** — les plugins et serveurs MCP étaient enregistrés statiquement à la compilation, empêchant les contributeurs externes et les membres d'équipe d'ajouter des intégrations sans recompiler le binaire.

2. **Aucune observabilité sur l'exécution des agents** — il n'existait aucun mécanisme pour suivre les métriques de qualité par agent (taux de réussite, consommation de tokens, latence, coût). Les décisions d'amélioration se prenaient sans données, rendant l'optimisation de la qualité aveugle.

3. **Aucun mécanisme de partage de skills** — les skills étaient des fichiers locaux sans chemin de découverte ni de distribution, empêchant la réutilisation des connaissances entre équipes et limitant l'adoption communautaire.

## Décision

Cinq systèmes complémentaires ont été conçus et implémentés pour combler ces lacunes :

### 1. Registre dynamique de plugins

**Emplacement :** `internal/plugin/registry.go`

Découvre les plugins installés par l'utilisateur en scannant `~/.oh/plugins/<name>/manifest.json` au démarrage. Chaque manifest déclare le nom, la version, le chemin du binaire et l'enregistrement des hooks. Les plugins ne nécessitent plus de modifications au niveau du code source de `oh`.

### 2. Registre dynamique MCP

**Emplacement :** `internal/mcp/mcpregistry/`

Découvre les serveurs MCP installés par l'utilisateur en scannant `~/.oh/mcp/<name>/manifest.json` au démarrage. Chaque manifest déclare le nom du serveur, le chemin du binaire, les variables d'environnement et le transport (stdio). Permet `oh mcp serve <name>` pour tout serveur installé sans codage en dur.

### 3. Télémétrie des agents

**Emplacement :** table SQLite `agent_events` — migrations v13–v16

Enregistre les métriques d'exécution par agent après chaque session :
- Nom de l'agent et identifiant de session
- Résultat succès/échec
- Durée (secondes)
- Nombre de tokens (entrée + sortie)
- Coût estimé (USD)
- Type d'erreur en cas d'échec

Expose les tendances agrégées via `oh agent stats` et le tableau de bord web.

### 4. Marketplace de skills

**Emplacement :** `internal/skillregistry/` — CLI : `oh skill add/list/remove/search`

Un index de skills communautaire hébergé à une URL connue. Les skills sont installés dans `~/.oh/skills/<name>/` et découverts automatiquement au démarrage. Commandes :
- `oh skill search <query>` — rechercher dans l'index
- `oh skill add <name>` — installer un skill
- `oh skill list` — lister les skills installés
- `oh skill remove <name>` — désinstaller un skill

### 5. Tableau de bord web

**Emplacement :** commande `oh serve` — API REST + SPA intégrée

Lance un serveur HTTP sur `127.0.0.1` (port aléatoire ou `--port`) servant :
- `GET /api/agents` — résumé de télémétrie des agents
- `GET /api/sessions` — liste des sessions récentes
- `GET /api/plugins` — plugins installés
- Une application monopage intégrée pour la supervision via navigateur

## Alternatives envisagées

| Alternative | Raison du rejet |
|---|---|
| Plugins Go natifs (bibliothèques partagées `.so`) | Instabilité ABI entre versions Go ; nécessite un environnement de build identique ; non portable entre OS |
| Service de télémétrie externe (ex. collecteur OpenTelemetry) | Brise la posture zéro-dépendance ; nécessite un accès réseau ; configuration complexe pour un outil local-first |
| Distribution de skills par Git (submodules) | Friction élevée pour les contributeurs ; pas de mécanisme de découverte ; complexité de gestion des versions |
| Métriques intégrées dans les tables SQLite existantes | Conflits de schéma avec les données existantes ; plus difficile à interroger et agréger séparément |

## Conséquences

### Positives

- **Extensibilité sans recompilation** : les plugins, serveurs MCP et skills peuvent être ajoutés en déposant des fichiers dans `~/.oh/` sans compiler depuis les sources
- **Tendances de qualité observables** : la télémétrie permet des décisions basées sur les données pour les prompts des agents, la sélection de modèles et le contrôle des coûts
- **Écosystème communautaire** : le marketplace de skills crée un chemin de partage du travail de prompt engineering entre équipes
- **Tableau de bord local-first** : supervision sans services externes ni dépendances SaaS

### Négatives

- **Contrat de stabilité du manifest** : le format `manifest.json` pour les plugins et serveurs MCP doit être traité comme une API publique — les breaking changes nécessitent un chemin de migration
- **Croissance du stockage de télémétrie** : ~2 Ko par session ajoutés à la base de données SQLite ; pour les utilisateurs intensifs (100+ sessions/jour), cela représente ~70 Mo/mois (acceptable, mais justifie un élagage périodique)
- **Confiance dans le marketplace** : les skills communautaires s'exécutent avec les mêmes permissions que les skills intégrés — un mécanisme de validation ou de sandboxing devrait être ajouté avant le lancement public

### Neutres

- Les plugins et serveurs MCP enregistrés statiquement restent fonctionnels ; le registre dynamique est additif
- Le tableau de bord web est une commande opt-in (`oh serve`), pas un daemon en arrière-plan

## Implémentation

- Registre dynamique de plugins : `internal/plugin/registry.go`
- Registre dynamique MCP : `internal/mcp/mcpregistry/`
- Table de télémétrie des agents : migrations SQLite v13–v16
- Registre de skills : `internal/skillregistry/`
- Commandes CLI de skills : `cli/cmd/skill.go`
- Tableau de bord web : `cli/cmd/serve.go` + `internal/dashboard/`

## En lien avec

- ADR-014 — Plugin context-mode
- ADR-024 — Dépôt team-state
- ADR-025 — Shell TUI unifié
