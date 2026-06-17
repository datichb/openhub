# ADR-012 — Git Worktrees pour l'isolation du travail en parallèle

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
- `oc start --worktree [BRANCH]` : session OpenCode isolée dans un nouveau worktree sur une branche donnée, sans lien Beads obligatoire. Permet de développer une feature en parallèle de la branche courante.
- `oc start --parallel` : lance l'`orchestrator-dev` dans un worktree dédié avec un lot de tickets `ai-delegated`. Le workflow complet est préservé (implémentation → QA → review → CP-2).

### Axe 3 — Gestion du cycle de vie

- Les worktrees sont stockés dans `.worktrees/<slug>/` à la racine du projet
- `.worktrees/` est automatiquement ajouté à `.git/info/exclude` (jamais à `.gitignore`)
- Un auto-cleanup optionnel (`worktree.auto_cleanup = true`) supprime les worktrees dont la branche est mergée au démarrage de toute session
- La commande `oc worktree` expose le cycle de vie complet (list, create, remove, cleanup, status)

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
| `scripts/cmd-worktree.sh` | Nouvelle commande `oc worktree` |
| `scripts/cmd-start.sh` | Flags `--parallel` et `--worktree` |
| `scripts/cmd-init.sh` | Configuration interactive worktree à l'init |
| `scripts/adapters/opencode.adapter.sh` | `.worktrees/` dans `.git/info/exclude` |
| `scripts/lib/project.sh` | Getters `get_project_worktree_*` |
| `scripts/lib/session-state.sh` | Champ `worktree_path` |
| `agents/planning/orchestrator-dev.md` | Permissions `git worktree *` |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Étape 1b conditionnelle |
| `skills/orchestrator/orchestrator-workflow-modes.md` | Documentation worktrees |
| `oc.sh` | Routing `worktree)` |
