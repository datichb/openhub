# ADR-026 : Redesign TUI Omnibar-First

- **Statut :** Accepté
- **Date :** 2026-07-20
- **Décideurs :** benjamin.datiche

## Contexte et problème

Le TUI avait accumulé trop de mécanismes d'interaction :
- Menu sidebar hiérarchique (TreeView avec navigation vim)
- Command palette (overlay Ctrl+P)
- 6 types de modals (input, password, select, multi-select, scrollable, confirm)
- Launchers de session (modal spécialisé)
- Header avec breadcrumb
- Status bar avec hints contextuels
- Messages flash dans la status bar
- Breakpoints responsive changeant le layout
- Raccourcis spécifiques par vue (non standardisés)

Cela créait une surcharge cognitive : les utilisateurs ne pouvaient pas prédire si sélectionner un élément du menu allait naviguer vers une vue, ouvrir un modal, ou exécuter directement une action.

## Facteurs de décision

- Power users utilisant l'outil quotidiennement et mémorisant les raccourcis
- Mix équilibré d'actions (sessions, projets, config, système) — pas de workflow dominant
- Volonté d'une interface comme opencode : aérée, simple, point d'interaction unique
- Inspiration fzf/Telescope : recherche fuzzy comme pattern d'interaction principal

## Options considérées

1. **Simplifier l'existant** — Garder sidebar + palette, réduire les items
2. **Tabs/sections** — Remplacer l'arbre par une navigation horizontale
3. **Omnibar-first** — Input unique remplace menu, palette, header, status bar

## Décision

Option choisie : **Omnibar-first** — redesign complet supprimant le menu sidebar, le header, la status bar et le système de modals au profit d'un omnibar persistant en bas de l'écran.

### Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                      CONTENU (plein écran)                        │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  [hints contextuels / saisie de commande]                    │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

### Décisions clés

1. **Registre plat de commandes** remplace le menu hiérarchique. Toutes les actions sont des commandes avec ID, alias et métadonnées fuzzy-searchable.
2. **L'omnibar remplace 5 composants** : menu sidebar, command palette, header (breadcrumb), status bar (hints), session launchers.
3. **Prompts inline** remplacent les modals overlay. Les inputs, selects et contenus scrollables apparaissent dans la zone de contenu.
4. **4 raccourcis globaux seulement** : `Ctrl+P`, `Esc`, `Ctrl+Q`, `Tab` — tout le reste via omnibar ou touches contextuelles.
5. **Toute touche imprimable active l'omnibar** si la vue courante ne la consomme pas — accès zéro-friction aux commandes.
6. **Les vues gardent `HandleKey`** pour les raccourcis contextuels (j/k, a/d, h/l) — l'omnibar ne capture que les runes non-gérées.

### Conséquences

**Positives :**
- Modèle mental radicalement plus simple (un input pour toutes les actions)
- Espace écran maximum pour le contenu
- Pattern d'interaction cohérent sur toutes les fonctionnalités
- Courbe d'apprentissage plus basse (tapez ce que vous voulez, le fuzzy match le trouve)
- Plus facile d'ajouter de nouvelles commandes (pas de restructuration d'arbre)

**Négatives :**
- Perte de découvrabilité du menu visible (atténué par l'omnibar qui affiche toutes les commandes sur requête vide)
- Les vues nécessitant des workflows multi-modals complexes (team init) utilisent des prompts inline chaînés
- Les power users ayant mémorisé la structure du menu doivent réapprendre (coût faible vu la simplification)

## Composants supprimés

- Package `v2/menu/` (répertoire entier)
- `shell/header.go`
- `shell/statusbar.go`
- `shell/palette.go`
- `shell/launcher.go`
- `shell/flash.go`
- `shell/responsive.go`

## Composants ajoutés

- `shell/command.go` — Struct Command + CommandRegistry avec recherche fuzzy
- `shell/omnibar.go` — Widget d'input persistant avec overlay de suggestions

## Liens

- Remplace : [ADR-025 : TUI Unified Shell](./025-tui-unified-shell.fr.md)
