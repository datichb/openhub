# Git Worktrees — Guide utilisateur

> **Historique (v5).** Ce guide décrit les worktrees du temps des scripts Bash : blocs `Worktree` de `projects.md`, `oh worktree create`, `oh worktree status` et `oh worktree list [PROJECT_ID]`, qui n'existent plus. Aujourd'hui, `oh worktree` propose `list`, `add`, `remove` et `cleanup` ([référence](../reference/cli-tools.fr.md#git-worktree)), les réglages sont dans la section `[worktree]` de `hub.toml` ([configuration](../reference/config.fr.md)), et `oh run <workflow> --location new` ou `oh run ticket --tickets …` créent un worktree par session ([workflows](../reference/cli-workflows.fr.md#oh-run), [fan-out](../guides/fan-out.fr.md)). La suite de cette page est conservée pour l'historique.

Les git worktrees permettent de travailler sur plusieurs branches simultanément sans `git stash` ou `git checkout`. Chaque worktree est un répertoire indépendant sur lequel une branche distincte est checkoutée.

---

## Prérequis

- **git 2.5+** (disponible sur toute installation moderne)
- Activer les worktrees pour le projet concerné (voir [Configuration](#configuration))

---

## Configuration

### Activation via `oh init`

Lors de l'initialisation d'un nouveau projet (`oh init`), une question interactive propose d'activer les worktrees :

```
Activer les git worktrees pour le travail en parallèle ? [y/N]
Activer le nettoyage automatique des worktrees mergés ? [Y/n]
```

### Activation manuelle dans `projects.md`

Ajouter les champs suivants dans le bloc du projet :

```markdown
## MON-APP
- Nom : Mon Application
- Stack : TypeScript
- Tracker : none
- Worktree : enabled
- Worktree auto cleanup : true
- Worktree base branch : main
```

| Champ | Valeurs | Description |
|-------|---------|-------------|
| `Worktree` | `enabled` / `disabled` | Active ou désactive les worktrees pour ce projet |
| `Worktree auto cleanup` | `true` / `false` | Supprime les worktrees mergés au démarrage de chaque session |
| `Worktree base branch` | Nom de branche | Branche de référence pour le cleanup (défaut : `main`) |

---

## Commandes `oh worktree`

### `oh worktree list [PROJECT_ID]`

Liste les worktrees actifs du projet avec leur branche et leur statut (mergé ou non).

```
$ oh worktree list MON-APP

  ../mon-app-feat-bd-42    feat/bd-42
  ../mon-app-fix-auth      fix/auth-null-check  [merged]
```

### `oh worktree create BRANCH [PROJECT_ID]`

Crée un worktree pour la branche donnée. Si la branche n'existe pas encore, elle est créée.

```
$ oh worktree create feat/nouvelle-feature MON-APP
```

Le worktree est créé dans un dossier frère du dépôt : `../mon-app-feat-nouvelle-feature/` (`<projet>-<slug de la branche>`).

### `oh worktree remove BRANCH [PROJECT_ID]`

Supprime un worktree (avec confirmation interactive).

```
$ oh worktree remove feat/bd-42 MON-APP
Supprimer le worktree 'feat/bd-42' ? [Y/n]
```

### `oh worktree cleanup [PROJECT_ID]`

Supprime tous les worktrees dont la branche est mergée dans la branche de base.

```
$ oh worktree cleanup MON-APP
◆  Suppression worktree mergé : fix/auth-null-check
◆  1 worktree(s) mergé(s) supprimé(s)
```

### `oh worktree status [PROJECT_ID]`

Affiche un résumé : worktrees actifs, mergés, configuration du projet.

```
$ oh worktree status MON-APP
  Worktree activé    enabled
  Auto-cleanup       true
  Base branch        main

Worktrees actifs : 2
Worktrees mergés : 1
→ Lancer 'oh worktree cleanup' pour supprimer les worktrees mergés
```

---

## Lancer une session dans un worktree : `--location new`

`oh run <workflow> --location new` lance la session dans un nouveau worktree (la branche vient de l'entrée `branch` du workflow quand il en a une : `-i branch=feat/ma-feature`). Idéal pour travailler sur une nouvelle feature pendant que votre branche principale est en développement. (`oh start --worktree`, retiré en 5.1, faisait de même.)

```bash
oh run feature -m "Ajoute l'export CSV" --location new -i branch=feat/ma-feature -p MON-APP
```

**Comportement :**
- Si le worktree pour cette branche n'existe pas → il est créé
- La session OpenCode est lancée dans le répertoire du worktree
- Un workflow qui écrit (`ticket`, `libre`…) travaille déjà dans son propre worktree : voir [`oh run`](../reference/cli-workflows.fr.md#oh-run)

**Après la session :**
```bash
# Commiter et pousser depuis le worktree
git -C ../mon-app-feat-ma-feature add . && git -C ../mon-app-feat-ma-feature commit -m "feat: ..."

# Une fois la PR mergée, nettoyer
oh worktree remove feat/ma-feature MON-APP
# ou
oh worktree cleanup MON-APP
```

---

## Plusieurs tickets en parallèle

`oh run ticket --tickets bd-1,bd-2` lance une session par ticket, chacune dans son worktree. L'ancien `oh start --parallel` (retiré en 5.1) lançait l'`orchestrator-dev` dans un worktree dédié `parallel/YYYYMMDD-HHMMSS`.

---

## Isolation via worktrees en mode `auto` (orchestrator-dev)

Quand `Worktree: enabled` est configuré pour le projet, l'`orchestrator-dev` utilise automatiquement `git worktree` à l'étape 1b (création de branche) :

- Au lieu de `git checkout -b <branche>`, il crée le dossier frère `../<projet>-<slug>/`
- Chaque `developer-*` délégué travaille dans son répertoire isolé
- Aucun risque de conflit filesystem entre sessions parallèles
- À CP-2 après commit validé, la suppression du worktree est proposée

---

## Cycle de vie d'un worktree

```
oh init / config         →  Worktree: enabled activé dans projects.md
                                     │
         ┌───────────────────────────┴─────────────────────┐
         │                                                   │
oh run ticket --tickets …        oh run … --location new
(orchestrator-dev isolé)         (session libre isolée)
         │                                                   │
   workflow complet                   développement libre
   impl → QA → review → CP-2              │
         │                             commit + push
      commit validé                       │
         │                             PR créée
      PR / merge                          │
         └──────────────────┬────────────┘
                            │
               oh worktree cleanup
               (ou auto au prochain lancement)
               → git worktree remove + prune
```

---

## Auto-cleanup

Quand `Worktree auto cleanup: true` est configuré, chaque lancement de session appelle `worktree_cleanup_merged` avant de lancer la session. Les worktrees dont la branche est mergée dans `Worktree base branch` sont automatiquement supprimés.

**Condition de suppression :** la branche doit être listée par `git branch --merged <base_branch>`.

---

## Limitations et bonnes pratiques

- **Un seul worktree par branche** : git interdit de checker la même branche dans deux worktrees simultanément
- **Migrations de base de données** : les migrations doivent être coordonnées manuellement entre worktrees (risque de conflits de schéma)
- **Fichiers de configuration globaux** (`tsconfig.json`, `package.json` racine) : les modifier dans un worktree n'impacte pas les autres — prévision d'une resync manuelle
- **Dépendances npm** : chaque worktree partage `node_modules/` via le dépôt principal — pas besoin de `npm install` dans chaque worktree
- **Maximum recommandé** : 3 worktrees actifs simultanément (limite opérationnelle de l'`orchestrator-dev`)
