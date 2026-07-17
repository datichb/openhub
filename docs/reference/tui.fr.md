# Référence TUI — OpenHub

> Documentation complète de l'interface terminal interactive (TUI) d'OpenHub.

## Lancement

```bash
oh          # Lance le TUI (terminal interactif détecté)
oh --no-tui # Force le mode CLI classique
```

Le TUI se lance automatiquement quand `oh` est exécuté sans sous-commande dans un terminal interactif. Variables qui désactivent le TUI : `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

## Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Logo]              [Breadcrumb]                    [Meta]       │ Header (2 rows)
├──────────────┬──────────────────────────────────────────────────┤
│   MENU       │              CONTENU                             │ Middle (flex)
│   (sidebar)  │              (vue active)                        │
├──────────────┴──────────────────────────────────────────────────┤
│ [Vue]         [Raccourcis contextuels]              [Info]       │ Status bar (1 row)
└─────────────────────────────────────────────────────────────────┘
```

## Raccourcis globaux

| Touche | Action |
|--------|--------|
| `Ctrl+Q` | Quitter le TUI |
| `Ctrl+P` | Ouvrir la command palette (recherche fuzzy) |
| `Ctrl+N` | Basculer le focus entre menu et contenu |
| `?` | Ouvrir la vue Aide |
| `Esc` | Retour à la vue précédente |

## Navigation du menu

| Touche | Action |
|--------|--------|
| `j` / `↓` | Item suivant |
| `k` / `↑` | Item précédent |
| `Enter` | Ouvrir la vue / exécuter l'action |
| `l` / `→` | Déplier une catégorie |
| `h` / `←` | Replier une catégorie / remonter |
| `Space` | Basculer déplier/replier |
| `g` | Premier item |
| `G` | Dernier item |

## Vues disponibles

### Home

Vue d'accueil avec message de bienvenue et activité récente.

### Sessions

| Item | Action |
|------|--------|
| Start | Sélecteur : Standard / Dev / Onboard → lance opencode |
| Audit | Sélecteur : Sécurité / Performance / Architecture / A11y / Éco / Observabilité |
| Review | Sélecteur : Standard / Adversarial / Edge cases / Complète |
| Debug | Prompt description du problème → lance opencode debugger |
| Quick | Lance opencode directement |
| Parallel | Vue monitoring des sessions parallèles |

### Projets

| Item | Raccourcis | Action |
|------|-----------|--------|
| Kanban | `h/l` colonnes, `r` refresh | Board kanban du projet actif |
| Liste | `a` ajouter, `d` supprimer | Liste des projets avec détail |
| Deploy | — | Déploie agents/skills/MCP sur le projet actif |
| Sync | — | Synchronise tous les projets |

### Team

| Item | Raccourcis | Action |
|------|-----------|--------|
| Kanban équipe | `h/l` colonnes, `r` refresh | Board kanban d'équipe |
| Status | `r` refresh | Statut équipe, config, activité |
| Worktrees | `a` ajouter, `d` supprimer, `r` refresh | Gestion git worktrees |

### Configuration

| Item | Raccourcis | Action |
|------|-----------|--------|
| Hub | `Enter` modifier | Table clé/valeur éditable (hub.toml) |
| MCP | `e` enable, `d` disable, `t` token | Gestion serveurs MCP |

### Système

| Item | Raccourcis | Action |
|------|-----------|--------|
| Status | `r` refresh | Info hub, config, provider, projets |
| Doctor | `r` re-check | Vérification système (OS, git, opencode, config, DB) |
| Métriques | `7` semaine, `3` mois, `a` tout | Stats depuis opencode.db |
| Mise à jour | — | Met à jour le binaire opencode |
| Aide | — | Raccourcis et documentation |

## Command Palette (Ctrl+P)

Recherche fuzzy sur toutes les commandes disponibles :
- Taper pour filtrer
- `↑`/`↓` ou `Tab`/`Shift+Tab` pour naviguer
- `Enter` pour exécuter
- `Esc` pour fermer

## Sessions opencode

Quand une session est lancée (Start, Audit, Review, Debug, Quick) :
1. Le TUI se met en pause
2. opencode prend le contrôle du terminal
3. À la fermeture d'opencode, le TUI reprend exactement où il en était
4. Un toast confirme la fin de session

## Responsive

| Largeur terminal | Comportement |
|-----------------|--------------|
| ≥ 120 cols | Layout complet |
| 100-119 cols | Labels menu tronqués |
| 80-99 cols | Menu visuellement réduit |
| < 80 cols | Breadcrumb masqué |
