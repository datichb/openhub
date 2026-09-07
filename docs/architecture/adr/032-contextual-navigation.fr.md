# ADR-032 — Navigation contextuelle Équipe/Projet

## Statut

proposed

## Date

2026-09-07

## Contexte

Le hub supporte à la fois des projets solo (sans team) et des projets en équipe.
Actuellement, toutes les vues et commandes omnibar sont accessibles depuis partout,
ce qui crée plusieurs problèmes d'expérience utilisateur :

- **Confusion des boards** : 3 entrées "Board" dans l'omnibar (Board Projet, Board Équipe,
  doublon Home) sans distinction claire de leur rôle.
- **Commandes non pertinentes** : les commandes team (Board Équipe, Team Status, Policies,
  Patterns) sont visibles même quand aucune team n'est configurée. Inversement, le Board
  Projet (beads) est visible même sans projet actif.
- **Deux systèmes de tickets déconnectés** : le board team (claims du tracker externe) et le
  board projet (beads, sous-tâches de développement) n'ont aucun lien alors qu'ils sont
  complémentaires. Un ticket tracker devrait pouvoir être décomposé en sous-tâches beads
  pour le développement.
- **Le `ProjectModeView` existe** comme concept de contexte projet mais n'est pas
  complètement abouti. Il n'y a pas d'équivalent "Team Mode".

Le `resolveRepo()` du TUI utilise `resolvedTeamConfig(a, project)` qui requiert un `TeamID`
sur le projet, alors que le CLI `sync-tracker` utilise directement `a.Config.ActiveTeam()`.
Cette asymétrie fait que les tickets synchronisés par le CLI n'apparaissent pas dans le TUI.

## Décision

Nous avons décidé d'introduire **3 modes de navigation** dans le TUI, avec sélection
automatique basée sur la configuration :

### Modes

| Mode | Activation | Vues disponibles | Board |
|------|-----------|-------------------|-------|
| **Hub** | Défaut (multi-team ou multi-projet sans team) | Home, Settings, Providers, MCP, Projects, Teams | Aucun (choisir d'abord) |
| **Équipe** | 1 team configurée, ou sélection explicite | Board Équipe, Team Status, Activity, Policies, Patterns, Team Detail, Briefs | Board Équipe |
| **Projet** | 1 projet actif, ou sélection explicite | Board Projet, Config Projet, Worktree, Sessions, Merge | Board Projet |

### Sélection automatique

- Si 1 seule team configurée et pas de projet solo → **mode Équipe** par défaut
- Si pas de team et 1 seul projet → **mode Projet** par défaut
- Si multi-team ou mix team/solo → **mode Hub** (choix via Home)

### Filtrage omnibar

Les commandes omnibar sont filtrées selon le mode actif :
- En mode Équipe : seules les commandes team + les commandes globales (Settings, etc.)
- En mode Projet : seules les commandes projet + les commandes globales
- En mode Hub : toutes les commandes

### Lien Tracker ↔ Beads

Chaque ticket beads peut référencer un ticket source via un champ `source_ticket`
(ex: `gitlab:693`). Sur le board team, un badge `[3/7 tasks]` affiche l'avancement
des sous-tâches beads liées. Sur le board projet, un label identifie le ticket tracker
source.

### Navigation entre modes

- `Ctrl+T` : basculer en mode Équipe (ou choisir la team si plusieurs)
- `Ctrl+P` : omnibar (filtrée par mode)
- Home : retour au mode Hub

## Conséquences

### Positives

- Interface épurée : seules les commandes pertinentes sont visibles
- Moins de confusion : un seul board visible selon le contexte
- Le lien tracker ↔ beads donne une vision complète du cycle de développement
- La sélection automatique évite un choix inutile quand la config est simple
- Compatible avec l'architecture existante (ProjectModeView, shell, omnibar)

### Négatives / Compromis

- Refactoring du shell TUI : le shell doit maintenir un `activeMode` et filtrer les commandes
- Le lien tracker ↔ beads nécessite une convention de nommage ou un champ dédié sur les tickets beads
- Les utilisateurs habitués à l'omnibar flat devront s'adapter au filtrage
- Le mode Hub (multi-team) est un écran de choix supplémentaire avant d'accéder aux vues

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Board unique fusionné (tracker + beads) | Trop complexe — les deux systèmes ont des modèles de données différents. Un ticket tracker a un assignee et un status GitLab, un ticket beads a des sous-tâches et une complexité. La fusion créerait une abstraction fragile. |
| Omnibar flat avec tags de filtrage | Ne résout pas le problème de fond — les commandes non pertinentes restent listées, juste filtrables. L'UX reste confuse. |
| Supprimer le board projet (beads uniquement via agents) | Le board projet est utile pour la visibilité locale du développement. Les agents l'utilisent pour le suivi des sous-tâches. Le supprimer perdrait cette visibilité. |

## Fichiers concernés

### Phase 1 (immédiat — masquage basique)

- `cli/cmd/tui_commands.go` — labels distincts, masquage par config
- `cli/cmd/tui_team_board.go` — fallback `ActiveTeam()` dans `resolveRepo()`
- `cli/internal/tui/v2/views/teamboard_view.go` — fix Mount empty-state
- `cli/internal/tui/v2/views/home.go` — suppression doublons omnibar

### Phase 2 (moyen terme — lien tracker ↔ beads)

- `cli/internal/beads/beads.go` — champ `source_ticket` sur les tickets beads
- `cli/internal/tui/v2/views/teamboard_view.go` — badge `[N/M tasks]` par ticket
- `cli/internal/tui/v2/views/board_view.go` — label source tracker sur les tickets

### Phase 3 (long terme — navigation contextuelle)

- `cli/internal/tui/v2/shell/shell.go` — `activeMode` (hub/team/project), filtrage commandes
- `cli/internal/tui/v2/views/home.go` — sélection de mode, auto-détection
- `cli/cmd/tui_commands.go` — champ `Mode` sur les commandes, filtrage dynamique
- `cli/internal/tui/v2/views/view.go` — interface `ModeAware` optionnelle sur les vues
