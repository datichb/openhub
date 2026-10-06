# ADR-033 — Matrice de cascade et d'enforcement des configurations

## Statut

accepted — **Évolué par [ADR-040](./040-workflows-team-state-governance.fr.md)**

## Date

2026-09-10

## Contexte

OpenHub utilise trois niveaux de configuration empilés :

- **Team** (repo git partagé) — source autoritaire pour les règles d'équipe
- **Hub** (`hub.toml` local) — surcharges personnelles du membre
- **Projet** (SQLite local) — surcharges spécifiques à un projet

Chaque domaine de configuration (MCP, Tracker, Workflow, Models...) implémente sa
propre logique de cascade avec des sémantiques différentes. Certains champs peuvent
être **enforced** par l'équipe (le membre ne peut pas surcharger), d'autres sont de
simples **recommandations** (surchargeable librement).

Un audit complet a révélé :

- 5 champs d'enforcement dans le modèle de données
- 2 d'entre eux étaient du code mort (définis mais jamais wirés)
- 1 vue projet (ProjectMCPView) ne bloquait pas l'édition des champs enforced
- La sémantique de cascade variait entre les domaines sans documentation centralisée

Cette ADR formalise la matrice complète de cascade et d'enforcement comme référence.

## Décision

Nous adoptons une matrice formelle documentant, pour chaque domaine et champ :

1. **L'ordre de cascade** (qui a priorité sur qui)
2. **Les champs enforceable** (quelle équipe peut imposer)
3. **L'effet de l'enforcement** (blocage résolution + blocage TUI)
4. **Les valeurs par défaut** quand aucun niveau ne définit la valeur

### Principe d'affichage : nil = "non configuré"

Toute valeur pointeur nil (`*bool`, `*int`) ou champ optionnel vide affiche
**"non configuré"** dans la TUI — jamais "désactivé", "(vide)" ou "défaut système"
quand l'utilisateur n'a rien configuré explicitement.

Pour les champs avec héritage implicite au niveau projet, le texte ajoute
une parenthèse précisant la source : `"non configuré (hérite du hub)"`.
Au niveau hub (plus haut niveau), pas de parenthèse.

`formatFieldValue` reçoit le `configField` complet et utilise `Scope` pour
déterminer la parenthèse d'héritage. Le type `CfgFieldTriDefault` a été supprimé
au profit de `CfgFieldTriBool` universel.

### Patterns de cascade par domaine

| Pattern | Domaines | Mécanisme |
|---------|---------|-----------|
| Hub-only (pas de cascade) | CLI, Opencode, Deploy, Worktree | Valeurs simples, pas d'héritage |
| Hub → Projet (2 niveaux) | Provider (aws_profile, region, auth) | Projet surcharge le hub ; vide = hériter |
| Team(rec) → Hub → Projet (3 niveaux, recommandation) | Models (default, families, agents) | Team recommande, hub surcharge, projet surcharge |
| Team(src) → Hub(override) (2 niveaux, pointeur nil-inherit) | Tracker local (enabled, auto_sync, push_labels, auto_plan) | Hub utilise `*bool` ; nil = utiliser la valeur team |
| Team(src) → Projet(override) (2 niveaux) | Tracker connexion (url, project, token, pattern) | Projet surcharge les valeurs team |
| Team(rec/enf?) → Projet → Hub → Team(rec) → Défaut (5 niveaux) | MCP services (enabled, url) | Team peut enforcer `enabled` et `url` |
| Base ← Hub ← Team ← Projet (overlay additif) | Workflow (overrides empilés) | Chaque niveau ajoute des overrides ; team `Enforced` bloque le projet |
| Team-only (pas de cascade) | Notification, Takeover, Parallel, Claim | Infrastructure partagée |

### Champs enforceable

| Champ | Struct | Effet sur la résolution | Effet TUI |
|-------|--------|-------------------------|-----------|
| `MCP.EnabledEnforced` | `SharedMCPConfig` | Impose la valeur team, ignore hub/projet | Lock icon + édition bloquée |
| `MCP.URLEnforced` | `SharedMCPConfig` | Impose l'URL team, ignore hub/projet | Lock icon + édition bloquée |
| `Workflow.Enforced` | `WorkflowTeamConfig` | Ignore les overrides projet | Vue en lecture seule + 🔒 |
| `Tracker.TypeEnforced` | `TrackerConfig` | Bloque l'édition TUI du type tracker | Grayed + toast |
| `Tracker.PushLabelsEnforced` | `TrackerConfig` | Impose la valeur team, ignore le local override | Grayed + toast + résolution |

### Champs explicitement NON enforceable

| Domaine | Raison |
|---------|--------|
| Models (default, families, agents) | Toujours recommandation — chaque membre/projet peut surcharger |
| MCP Token / WriteEnabled | Données personnelles/sécurité — jamais partagées via le team |
| Worktree, CLI, Opencode, Deploy | Hub-only — pas de dimension team |

### Hub Tracker *bool : tri-state uniforme

Les champs Tracker hub (`*bool`) sont toujours tri-state via `CfgFieldTriBool` :

- `""` (nil) = non configuré
- `"true"` = activé
- `"false"` = désactivé

Pas de distinction solo/team — le comportement est identique. Le `Scope: ScopeHub`
empêche l'ajout de parenthèse d'héritage (le hub est le plus haut niveau).

## Conséquences

### Positives

- Documentation centralisée de la cascade — référence unique pour les développeurs
- Tous les champs d'enforcement sont wirés de bout en bout (résolution + TUI)
- Le code mort (`TypeEnforced`, `PushLabelsEnforced`) est activé
- La vue projet MCP affiche correctement les lock icons pour les champs enforced
- L'utilisateur hub peut revenir à "utiliser la valeur équipe" pour les champs tracker

### Négatives / Compromis

- Complexité inhérente : 8 patterns de cascade différents
- Le tracker hub n'a toujours pas d'enforcement (TypeEnforced/PushLabelsEnforced ne bloquent que la TUI, pas la résolution pour TypeEnforced)
- Les models sont non-enforceable par design — un team lead ne peut pas imposer un modèle

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Cascade unique pour tous les domaines | Les sémantiques sont trop différentes (overlay workflow vs winner-takes-all MCP vs nil-inherit tracker) |
| Supprimer TypeEnforced et PushLabelsEnforced | Cohérence : tous les champs d'enforcement du modèle de données doivent être wirés |
| Auto-save pour les vues team | Le git push (2-7s) est trop coûteux pour un auto-save par mutation |
| Afficher "désactivé" pour nil | Mensonger — l'utilisateur n'a jamais fait ce choix. "Non configuré" est la vraie information |

## Fichiers concernés

- `teamstate/teamconfig.go` — helpers `IsTypeEnforced()`, `IsPushLabelsEnforced()`
- `tracker/resolve_config.go` — enforcement PushLabelsEnforced dans la résolution
- `views/team_detail_view.go` — grayed + toggles enforcement
- `views/settings_view.go` — trackerBoolField dynamique
- `cmd/tui_views.go` — wiring ResolveMCPSource + WorkflowView
- `views/config_field.go` — `CfgFieldTriBool` universel, `formatFieldValue(f, val)` avec annotations d'héritage par Scope
