# Git Worktrees — Guide utilisateur

Les git worktrees permettent de travailler sur plusieurs branches simultanément sans `git stash` ou `git checkout`. Chaque worktree est un répertoire indépendant sur lequel une branche distincte est checkoutée.

---

## Prérequis

- **git 2.5+** (disponible sur toute installation moderne)
- Activer les worktrees pour le projet concerné (voir [Configuration](#configuration))

---

## Configuration

### Activation via `oc init`

Lors de l'initialisation d'un nouveau projet (`oc init`), une question interactive propose d'activer les worktrees :

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

## Commandes `oc worktree`

### `oc worktree list [PROJECT_ID]`

Liste les worktrees actifs du projet avec leur branche et leur statut (mergé ou non).

```
$ oc worktree list MON-APP

  .worktrees/feat-bd-42    feat/bd-42
  .worktrees/fix-auth      fix/auth-null-check  [merged]
```

### `oc worktree create BRANCH [PROJECT_ID]`

Crée un worktree pour la branche donnée. Si la branche n'existe pas encore, elle est créée.

```
$ oc worktree create feat/nouvelle-feature MON-APP
```

Le worktree est créé dans `.worktrees/feat-nouvelle-feature/`.

### `oc worktree remove BRANCH [PROJECT_ID]`

Supprime un worktree (avec confirmation interactive).

```
$ oc worktree remove feat/bd-42 MON-APP
Supprimer le worktree 'feat/bd-42' ? [Y/n]
```

### `oc worktree cleanup [PROJECT_ID]`

Supprime tous les worktrees dont la branche est mergée dans la branche de base.

```
$ oc worktree cleanup MON-APP
◆  Suppression worktree mergé : fix/auth-null-check
◆  1 worktree(s) mergé(s) supprimé(s)
```

### `oc worktree status [PROJECT_ID]`

Affiche un résumé : worktrees actifs, mergés, configuration du projet.

```
$ oc worktree status MON-APP
  Worktree activé    enabled
  Auto-cleanup       true
  Base branch        main

Worktrees actifs : 2
Worktrees mergés : 1
→ Lancer 'oc worktree cleanup' pour supprimer les worktrees mergés
```

---

## Mode `oc start --worktree`

Lance une session OpenCode isolée dans un nouveau worktree. Idéal pour travailler sur une nouvelle feature pendant que votre branche principale est en développement.

```bash
# Branche spécifiée directement
oc start --worktree feat/ma-feature MON-APP

# Branche demandée interactivement
oc start --worktree MON-APP
```

**Comportement :**
- Si le worktree pour cette branche n'existe pas → il est créé
- Si le worktree existe déjà → il est réutilisé
- La session OpenCode est lancée dans le répertoire du worktree
- Pas de lien Beads obligatoire — développement libre
- L'auto-cleanup est exécuté avant le lancement si activé

**Après la session :**
```bash
# Commiter et pousser depuis le worktree
git -C .worktrees/feat-ma-feature add . && git -C .worktrees/feat-ma-feature commit -m "feat: ..."

# Une fois la PR mergée, nettoyer
oc worktree remove feat/ma-feature MON-APP
# ou
oc worktree cleanup MON-APP
```

---

## Mode `oc start --parallel`

Lance l'`orchestrator-dev` dans un worktree dédié avec un lot de tickets `ai-delegated`. Le workflow complet est préservé : implémentation → QA → review → CP-2.

```bash
oc start --parallel MON-APP
```

**Comportement :**
1. Charge les tickets `ai-delegated` disponibles
2. Crée un worktree sur une branche `parallel/YYYYMMDD-HHMMSS`
3. Lance l'`orchestrator-dev` dans ce worktree
4. Le workflow standard s'applique (modes manuel / semi-auto / auto, CP-2 obligatoire)

**Différence avec `--dev` :**
- `--dev` : session dans le répertoire principal du projet
- `--parallel` : session dans un worktree isolé — permet d'avoir une session `--dev` en cours ET une session `--parallel` simultanément

---

## Isolation via worktrees en mode `auto` (orchestrator-dev)

Quand `Worktree: enabled` est configuré pour le projet, l'`orchestrator-dev` utilise automatiquement `git worktree` à l'étape 1b (création de branche) :

- Au lieu de `git checkout -b <branche>`, il crée `.worktrees/<slug>/`
- Chaque `developer-*` délégué travaille dans son répertoire isolé
- Aucun risque de conflit filesystem entre sessions parallèles
- À CP-2 après commit validé, la suppression du worktree est proposée

---

## Cycle de vie d'un worktree

```
oc init / config         →  Worktree: enabled activé dans projects.md
                                     │
oc deploy                →  .worktrees/ ajouté à .git/info/exclude
                                     │
         ┌───────────────────────────┴─────────────────────┐
         │                                                   │
oc start --parallel              oc start --worktree BRANCH
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
               oc worktree cleanup
               (ou auto au prochain oc start)
               → git worktree remove + prune
```

---

## Auto-cleanup

Quand `Worktree auto cleanup: true` est configuré, chaque `oc start` (quelle que soit sa variante) appelle `worktree_cleanup_merged` avant de lancer la session. Les worktrees dont la branche est mergée dans `Worktree base branch` sont automatiquement supprimés.

**Condition de suppression :** la branche doit être listée par `git branch --merged <base_branch>`.

---

## Limitations et bonnes pratiques

- **Un seul worktree par branche** : git interdit de checker la même branche dans deux worktrees simultanément
- **Migrations de base de données** : les migrations doivent être coordonnées manuellement entre worktrees (risque de conflits de schéma)
- **Fichiers de configuration globaux** (`tsconfig.json`, `package.json` racine) : les modifier dans un worktree n'impacte pas les autres — prévision d'une resync manuelle
- **Dépendances npm** : chaque worktree partage `node_modules/` via le dépôt principal — pas besoin de `npm install` dans chaque worktree
- **Maximum recommandé** : 3 worktrees actifs simultanément (limite opérationnelle de l'`orchestrator-dev`)
