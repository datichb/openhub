# ADR-012 — Git Worktrees pour l'isolation du travail en parallèle

**Date :** 2026-06-04
**Statut :** Accepté *(mis à jour 2026-07-22)*
**Auteurs :** openhub

---

## Contexte

Le hub dispose d'un mécanisme de parallélisme conditionnel en mode `auto` : l'`orchestrator-dev` peut traiter jusqu'à 3 tickets simultanément via l'outil `task` (ADR-006). Cependant, ce parallélisme est **purement sémantique** — tous les agents `developer-*` partagent le même working tree. L'isolation repose sur des conventions (domaines distincts, dependency graph) mais reste vulnérable à des conflits filesystem inattendus.

Par ailleurs, les développeurs souhaitent parfois travailler sur une nouvelle feature sans perturber leur branche courante en cours de développement — ce qui impose un `git stash` / `git checkout` intrusif.

---

## Décision

Introduire le support de **`git worktree`** dans openhub selon deux axes complémentaires :

### Axe 1 — Isolation du mode auto (complément)

Quand `worktree.enabled = true` dans la config du projet, l'étape 1b de l'`orchestrator-dev` utilise `git worktree add` au lieu de `git checkout -b`. Chaque ticket reçoit un répertoire dédié `.worktrees/<slug>/`. Les agents `developer-*` travaillent dans leur répertoire isolé — sans risque de conflit filesystem entre sessions parallèles.

Le mode auto existant reste inchangé pour les projets où `worktree.enabled` est absent ou `disabled`.

### Axe 2 — Sessions parallèles libres (nouveau mode)

Deux nouvelles commandes :
- `oh start --worktree [BRANCH]` : session OpenCode isolée dans un nouveau worktree sur une branche donnée, sans lien Beads obligatoire. Permet de développer une feature en parallèle de la branche courante.
- `oh start --parallel` : lance l'`orchestrator-dev` dans un worktree dédié avec un lot de tickets `ai-delegated`. Le workflow complet est préservé (implémentation → QA → review → CP-2).

### Axe 3 — Gestion du cycle de vie

- Les worktrees sont stockés dans `.worktrees/<slug>/` à la racine du projet
- `.worktrees/` est automatiquement ajouté à `.git/info/exclude` (jamais à `.gitignore`)
- Un auto-cleanup optionnel (`worktree.auto_cleanup = true`) supprime les worktrees dont la branche est mergée au démarrage de toute session
- La commande `oh worktree` expose le cycle de vie complet (list, create, remove, cleanup, status)

---

## Conséquences

### Positives

- Isolation filesystem réelle entre agents parallèles → zéro conflit de fichiers
- Développement multi-features sans stash/checkout intrusif
- Le mode auto sans worktrees reste identique → compatibilité ascendante totale
- Auto-cleanup préserve la propreté du répertoire de travail

### Négatives / risques

- Dépendance à `git worktree` (disponible depuis git 2.5, mars 2015 — risque négligeable)
- Multiplication des répertoires `.worktrees/` si l'auto-cleanup est désactivé
- Les worktrees partagent l'index git du dépôt principal — les opérations sur les branches sensibles (merge, rebase) doivent être faites avec précaution

### Impact sur les permissions agents

L'`orchestrator-dev` reçoit quatre permissions bash supplémentaires :
```yaml
"git worktree add *": allow
"git worktree remove *": allow
"git worktree list": allow
"git worktree prune": allow
```

Ces permissions sont limitées aux opérations worktree — `git push`, `git merge` restent interdits (principe du moindre privilège conservé).

### Impact sur le session-state

Le champ `worktree_path` est ajouté aux entrées `tickets[]` et `current_ticket` dans `session-state.json` pour permettre au dashboard de l'afficher. Ce champ est optionnel (`null` si pas de worktree).

---

## Alternatives considérées

| Alternative | Rejetée car |
|-------------|-------------|
| Répertoires temporaires (`/tmp/`) | Hors du dépôt git — impossible de commiter depuis là |
| Sous-modules git | Complexité excessive, cycle de vie couplé au dépôt parent |
| `git stash` automatique | Intrusif, perd le contexte de travail courant |
| Branches uniquement (sans worktrees) | Ne résout pas l'isolation filesystem |

---

## Fichiers concernés

| Fichier | Changement |
|---------|-----------|
| `scripts/lib/worktree.sh` | Nouvelle lib — toutes les opérations worktree |
| `scripts/cmd-worktree.sh` | Nouvelle commande `oh worktree` |
| `scripts/cmd-start.sh` | Flags `--parallel` et `--worktree` |
| `scripts/cmd-init.sh` | Configuration interactive worktree à l'init |
| `scripts/adapters/opencode.adapter.sh` | `.worktrees/` dans `.git/info/exclude` |
| `scripts/lib/project.sh` | Getters `get_project_worktree_*` |
| `scripts/lib/session-state.sh` | Champ `worktree_path` |
| `agents/planning/orchestrator-dev.md` | Permissions `git worktree *` |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Étape 1b conditionnelle |
| `skills/orchestrator/orchestrator-workflow-modes.md` | Documentation worktrees |
| `oc.sh` | Routing `worktree)` |

---

## Addendum — Mise à jour juillet 2026

L'implémentation Go livrée diffère sur plusieurs points du design initial décrit ci-dessus. Cet addendum documente l'état réel du code.

### A. Répertoires frères au lieu de `.worktrees/<slug>/`

**Design initial :** les worktrees étaient stockés dans `.worktrees/<slug>/` à la racine du projet, avec ajout automatique de `.worktrees/` à `.git/info/exclude`.

**Implémentation réelle :** les worktrees sont créés en **répertoires frères** du projet principal :

```
/home/user/myrepo/              ← projet principal
/home/user/myrepo-feat-bd42/    ← worktree pour feat/BD-42
/home/user/myrepo-fix-auth/     ← worktree pour fix/auth
```

Le chemin est calculé par `SiblingPath(projectPath, branch)` dans `cli/internal/worktree/worktree.go`. Cette approche rend la manipulation de `.git/info/exclude` inutile (les répertoires frères sont hors de l'arbre du projet), donc la fonction `EnsureExclude` a été supprimée.

**Justification :** les répertoires frères n'apparaissent jamais dans `git status` du projet principal, éliminant le besoin de gestion d'exclude. Ils restent accessibles via des chemins relatifs prévisibles.

### B. Implémentation Go au lieu de scripts shell

**Design initial :** l'implémentation reposait sur des scripts shell (`scripts/lib/worktree.sh`, etc.).

**Implémentation réelle :** toute la logique est dans le package Go `cli/internal/worktree/worktree.go`. Les fichiers shell référencés dans le tableau "Fichiers concernés" original n'existent plus.

**Fichiers réels :**

| Fichier | Rôle |
|---------|------|
| `cli/internal/worktree/worktree.go` | Package core — toutes les opérations |
| `cli/internal/worktree/worktree_test.go` | Tests unitaires et d'intégration |
| `cli/cmd/worktree.go` | Sous-commandes `oh worktree` |
| `cli/cmd/start.go` | Flags `--worktree` pour `oh start` |
| `cli/cmd/start_parallel_mode.go` | Mode `oh start --parallel` |
| `cli/internal/parallel/coordinator.go` | Création séquentielle de worktrees |
| `cli/internal/parallel/config.go` | Config parallèle (dont `cleanup_completed_worktrees`) |
| `cli/internal/tui/v2/views/worktree_view.go` | Vue TUI de gestion |
| `cli/internal/config/config.go` | Struct `WorktreeConfig` |

### C. `CleanupMerged` : safe-by-default

Le cleanup de worktrees est maintenant **prudent par défaut** : sans le flag `-f`, les worktrees ayant des modifications non commitées sont ignorés (skippés) plutôt que détruits. La signature a évolué :

```go
// Avant
func CleanupMerged(projectPath, baseBranch string) ([]string, error)

// Après
func CleanupMerged(projectPath, baseBranch string, force bool) (CleanupResult, error)
// CleanupResult.Removed : branches supprimées
// CleanupResult.Skipped : branches mergées mais conservées (dirty, force=false)
```

L'auto-cleanup de `oh start` utilise `force=false`. La commande `oh worktree cleanup -f` utilise `force=true`.

### D. Vérification de l'existence en remote avant création

`ResolveOrCreate` vérifie maintenant si la branche existe sur le remote (`origin`) avant de tenter de la créer. Si elle existe, un `git fetch` est effectué et le worktree pointe sur la branche remote — permettant la reprise de travail sur une branche déjà poussée.

```
1. Worktree déjà enregistré localement → réutiliser
2. Branche existe sur origin → fetch + checkout
3. Branche existe localement → checkout
4. Aucun des cas → créer nouvelle branche
```

### E. Convention de nommage de branche configurable

Le pattern de nommage des branches (auparavant hardcodé à `feat/%s` dans le coordinator parallèle) est maintenant :

1. Configurable via `[worktree].branch_pattern` dans `hub.toml`
2. Détecté automatiquement à l'onboarding (`oh init`) par heuristique sur les branches existantes
3. Géré par `worktree.BranchName(pattern, ticketID)` qui valide que le pattern contient `%s`

### F. Cleanup de worktrees après session parallèle

Le `Coordinator` supporte maintenant l'option `cleanup_completed_worktrees` dans sa config : quand activée, les worktrees des sessions **terminées avec succès** sont supprimés à l'arrêt. Les sessions en échec sont intentionnellement conservées pour inspection post-mortem.


**Date :** 2026-06-04
**Statut :** Accepté
**Auteurs :** openhub

---

## Contexte

Le hub dispose d'un mécanisme de parallélisme conditionnel en mode `auto` : l'`orchestrator-dev` peut traiter jusqu'à 3 tickets simultanément via l'outil `task` (ADR-006). Cependant, ce parallélisme est **purement sémantique** — tous les agents `developer-*` partagent le même working tree. L'isolation repose sur des conventions (domaines distincts, dependency graph) mais reste vulnérable à des conflits filesystem inattendus.

Par ailleurs, les développeurs souhaitent parfois travailler sur une nouvelle feature sans perturber leur branche courante en cours de développement — ce qui impose un `git stash` / `git checkout` intrusif.

---

## Décision

Introduire le support de **`git worktree`** dans openhub selon deux axes complémentaires :

### Axe 1 — Isolation du mode auto (complément)

Quand `worktree.enabled = true` dans la config du projet, l'étape 1b de l'`orchestrator-dev` utilise `git worktree add` au lieu de `git checkout -b`. Chaque ticket reçoit un répertoire dédié `.worktrees/<slug>/`. Les agents `developer-*` travaillent dans leur répertoire isolé — sans risque de conflit filesystem entre sessions parallèles.

Le mode auto existant reste inchangé pour les projets où `worktree.enabled` est absent ou `disabled`.

### Axe 2 — Sessions parallèles libres (nouveau mode)

Deux nouvelles commandes :
- `oh start --worktree [BRANCH]` : session OpenCode isolée dans un nouveau worktree sur une branche donnée, sans lien Beads obligatoire. Permet de développer une feature en parallèle de la branche courante.
- `oh start --parallel` : lance l'`orchestrator-dev` dans un worktree dédié avec un lot de tickets `ai-delegated`. Le workflow complet est préservé (implémentation → QA → review → CP-2).

### Axe 3 — Gestion du cycle de vie

- Les worktrees sont stockés dans `.worktrees/<slug>/` à la racine du projet
- `.worktrees/` est automatiquement ajouté à `.git/info/exclude` (jamais à `.gitignore`)
- Un auto-cleanup optionnel (`worktree.auto_cleanup = true`) supprime les worktrees dont la branche est mergée au démarrage de toute session
- La commande `oh worktree` expose le cycle de vie complet (list, create, remove, cleanup, status)

---

## Conséquences

### Positives

- Isolation filesystem réelle entre agents parallèles → zéro conflit de fichiers
- Développement multi-features sans stash/checkout intrusif
- Le mode auto sans worktrees reste identique → compatibilité ascendante totale
- Auto-cleanup préserve la propreté du répertoire de travail

### Négatives / risques

- Dépendance à `git worktree` (disponible depuis git 2.5, mars 2015 — risque négligeable)
- Multiplication des répertoires `.worktrees/` si l'auto-cleanup est désactivé
- Les worktrees partagent l'index git du dépôt principal — les opérations sur les branches sensibles (merge, rebase) doivent être faites avec précaution

### Impact sur les permissions agents

L'`orchestrator-dev` reçoit quatre permissions bash supplémentaires :
```yaml
"git worktree add *": allow
"git worktree remove *": allow
"git worktree list": allow
"git worktree prune": allow
```

Ces permissions sont limitées aux opérations worktree — `git push`, `git merge` restent interdits (principe du moindre privilège conservé).

### Impact sur le session-state

Le champ `worktree_path` est ajouté aux entrées `tickets[]` et `current_ticket` dans `session-state.json` pour permettre au dashboard de l'afficher. Ce champ est optionnel (`null` si pas de worktree).

---

## Alternatives considérées

| Alternative | Rejetée car |
|-------------|-------------|
| Répertoires temporaires (`/tmp/`) | Hors du dépôt git — impossible de commiter depuis là |
| Sous-modules git | Complexité excessive, cycle de vie couplé au dépôt parent |
| `git stash` automatique | Intrusif, perd le contexte de travail courant |
| Branches uniquement (sans worktrees) | Ne résout pas l'isolation filesystem |

---

## Fichiers concernés

| Fichier | Changement |
|---------|-----------|
| `scripts/lib/worktree.sh` | Nouvelle lib — toutes les opérations worktree |
| `scripts/cmd-worktree.sh` | Nouvelle commande `oh worktree` |
| `scripts/cmd-start.sh` | Flags `--parallel` et `--worktree` |
| `scripts/cmd-init.sh` | Configuration interactive worktree à l'init |
| `scripts/adapters/opencode.adapter.sh` | `.worktrees/` dans `.git/info/exclude` |
| `scripts/lib/project.sh` | Getters `get_project_worktree_*` |
| `scripts/lib/session-state.sh` | Champ `worktree_path` |
| `agents/planning/orchestrator-dev.md` | Permissions `git worktree *` |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Étape 1b conditionnelle |
| `skills/orchestrator/orchestrator-workflow-modes.md` | Documentation worktrees |
| `oc.sh` | Routing `worktree)` |
