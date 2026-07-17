# ADR-025 : Shell TUI unifié avec navigation intégrée

## Statut

Accepté

## Date

2026-07-17

## Contexte

Le CLI `oh` dispose de 50+ commandes réparties en 14 domaines fonctionnels et de 6 vues TUI plein écran indépendantes (dashboard, board, teamboard, wizard, picker, parallel). Chaque vue lance sa propre `tview.Application` — il est impossible de naviguer entre vues sans quitter le processus. Le sidebar existant est un stub préparé pour devenir un menu navigable.

Problèmes identifiés :
1. Pas de routeur/navigation — chaque commande lance une app indépendante
2. Pas de menu de découverte — l'utilisateur doit connaître les commandes CLI
3. Deux systèmes de thème incompatibles (`v2/theme` tcell vs `common/` lipgloss)
4. Pas de feedback visuel (pas de toast, pas de modal, pas de breadcrumb)
5. Le sidebar est un stub avec un seul item statique

## Décision

Créer un shell TUI unifié accessible via `oh` sans arguments, fondé sur :

### Architecture technique

- **Interface `View`** : contrat commun pour toutes les vues (`ID()`, `Title()`, `Mount()`, `Unmount()`, `StatusHints()`, `HandleKey()`)
- **Router à pile** : navigation push/pop/replace avec historique (back via Esc)
- **Shell unique** : une seule `tview.Application` persistante gérant header, menu, content, status bar
- **Pages overlay** : wrapper `tview.Pages` pour modals et toasts au-dessus du contenu
- **Menu hiérarchique** : `tview.TreeView` dans le sidebar avec catégories expand/collapse

### Design unifié

- **Palette unique** : package `tui/theme/` avec hex constants → dérivations tcell ET lipgloss
- **Dual-accent** : Azure (#64a0ff) pour structure/navigation, Gold (#e0a030) pour actions/CTA
- **Header 3-sections** : logo | breadcrumb | méta-info
- **StatusBar 3-sections** : vue courante | hints contextuels | info globale

### Intégration opencode

- **Phase 1** : Process switching via `app.Suspend()` → exec opencode → resume
- **Phase 2** : Panel monitoring via API REST `opencode serve` (headless)

### Point d'entrée

- `oh` sans arguments en terminal interactif → lance le shell TUI
- Les commandes individuelles (`oh board`, `oh start`, etc.) restent fonctionnelles en standalone
- Flag `--no-tui` force le comportement CLI classique

## Alternatives envisagées

| Alternative | Raison du rejet |
|---|---|
| Garder les vues indépendantes | Pas de navigation, pas de découvrabilité, UX fragmentée |
| Migrer tout vers BubbleTea | Refonte totale, perte des 6 vues tview existantes |
| Utiliser tmux/zellij pour multiplexer | Dépendance externe, configuration complexe pour l'utilisateur |
| Intégrer opencode comme sous-vue tview | Impossible — opencode est un binaire TypeScript avec son propre TUI |
| Commande `oh tui` dédiée | Moins découvrable que `oh` sans args |

## Conséquences

### Positives
- UX unifiée : navigation fluide entre toutes les vues sans quitter
- Découvrabilité : le menu expose toutes les fonctionnalités disponibles
- Cohérence visuelle : une seule palette, un seul design system
- Feedback : toast, modal, breadcrumb améliorent la compréhension utilisateur
- Rétrocompatible : les commandes standalone restent fonctionnelles

### Négatives
- Complexité accrue du code TUI (router, lifecycle, overlay)
- Deux modes d'accès à maintenir (CLI direct + shell TUI)
- Surface de test plus large (SimScreen pour les widgets)

### Neutres
- Le layout existant (`layout.Build()`) est conservé pour les vues standalone
- Les vues huh/BubbleTea (floating prompt, quick) restent en mode inline séparé
- La Phase 2 opencode (monitoring REST) n'est pas bloquante pour le lancement

## Implémentation

- Package thème unifié : `cli/internal/tui/theme/`
- Shell : `cli/internal/tui/v2/shell/`
- Router : `cli/internal/tui/v2/router/`
- Menu : `cli/internal/tui/v2/menu/`
- Interface View : `cli/internal/tui/v2/views/view.go`
- Point d'entrée : `cli/cmd/root.go` (RunE par défaut)
