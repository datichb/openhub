# ADR-034 — Wizard inline dans le TUI shell

## Statut

accepted

## Date

2026-09-14

## Contexte

Les flows de configuration initiale (`oh init`, `oh team init`, `oh project add`)
utilisaient deux mécanismes d'interface distincts dans le TUI :

1. **Wizards standalone** — `views.RunWizard()` crée une `tview.Application` séparée
   (alt-screen) avec un wizard multi-step complet (step bar, info panel, back navigation).
   Utilisé par `oh init` (15 steps), `oh team init` (5 steps), `oh project add` (8 steps).

2. **Modals chaînés** — Le TUI shell enchaînait `ShowInputModal` → `ShowSelectModal` →
   `ShowInlineForm` via des closures imbriquées et des `time.Sleep(50ms)` entre chaque
   modal. Utilisé par `actionTeamInit()` dans le TUI.

Ces deux approches posaient des problèmes :

- **Wizard standalone** : le switch alt-screen est brutal pour l'utilisateur — le shell
  TUI disparaît, un nouveau tview.Application prend le terminal, puis à la fin le shell
  réapparaît. L'omnibar, les toasts et tout le contexte shell sont perdus pendant le wizard.
  Pour le first-run wizard (`init_wizard.go`), le wizard tournait même *avant* la création
  du shell, empêchant toute interaction shell.

- **Modals chaînés** : l'implémentation TUI de team init ne couvrait que 3 des 5 étapes
  du CLI (manquaient config globale, notifications, policies), ne collectait que 3 champs
  d'identité sur 6 (manquaient gitlab/mattermost/tracker username), ne faisait pas de
  commit-and-push des membres, n'avait pas de back navigation, pas de step bar de
  progression, et nécessitait des `time.Sleep(50ms)` entre les modals pour éviter des
  freezes tview.

L'écart fonctionnel entre le CLI wizard (complet) et le TUI (incomplet) créait une
expérience incohérente pour les utilisateurs qui privilégiaient le TUI.

## Décision

Nous avons décidé de créer un composant **`InlineWizardView`** qui implémente l'interface
`views.View` et tourne à l'intérieur du shell TUI existant, pushé sur le router stack
via une nouvelle méthode `shell.PushView(v)`.

### Principes

1. **Réutilisation du type `WizardStep`** — le composant accepte exactement la même struct
   `WizardStep` (`wizard.go:28-75`) que le wizard standalone, permettant de partager les
   définitions de steps entre CLI et TUI.

2. **View standard** — `InlineWizardView` implémente `View` (`ID`, `Title`, `Mount`,
   `Unmount`, `HandleKey`, `StatusHints`) et suit le même cycle de vie que toutes les
   autres vues du shell. `Esc` pop naturellement le wizard via le handler global du shell.

3. **Push éphémère** — le wizard est créé dynamiquement par une action omnibar et pushé
   via `PushView` sans être pré-enregistré dans le router registry. Quand il se termine,
   l'utilisateur pop vers la vue précédente ou navigue vers une vue cible.

4. **Layout intégré** — le wizard construit son propre layout interne (StepBar + header
   + content swappable + info panel + hints bar) dans le content Flex fourni par `Mount()`.
   L'omnibar du shell reste visible et accessible.

### Composant `InlineWizardView`

```
┌─────────────────────────────────────────────────────┐
│ ● Dépôt ─── ◆ Config ─── ○ Identité ─── ○ Notifs  │  StepBar
├─────────────────────────────────────────────────────┤
│ ◆ 2/5 — Configuration globale                      │  Header
├─────────────────────────────────────────────────────┤
│ [Form / CustomView / Spinner]                       │  Contenu
├─────────────────────────────────────────────────────┤
│ ✓ Dépôt: git@gitlab.com:team/state.git             │  Info panel
├─────────────────────────────────────────────────────┤
│ ctrl+s valider · ctrl+b retour · esc passer         │  Hints
└─────────────────────────────────────────────────────┘
```

### Méthode `PushView` sur `ShellAccess`

Ajoutée à l'interface `ShellAccess` (`view.go`) et implémentée sur `Shell` (`shell.go`).
Push une vue éphémère sur le router stack sans pré-enregistrement, avec injection
automatique de `ShellAccess` si la vue implémente `shellAware`.

### Wizards intégrés

| Wizard | Commande omnibar | Steps | Fichier |
|--------|-----------------|-------|---------|
| Team init | `team init` | 6 (repo, HTTPS creds, config, identité, notifs, policies) | `tui_team_actions.go` |
| Hub init | `init` | 5 (bienvenue, provider, auth mode, credentials, premier projet) | `init_wizard.go` |
| Project add | `project add` | 8 (identité, beads, provider, provider config, agents, MCP, team, deploy) | `tui_actions.go` |

Le first-run wizard (détecté quand `DefaultProvider == ""`) est maintenant pushé
en inline après le démarrage du shell au lieu de tourner dans un tview.Application
séparé avant la création du shell.

## Conséquences

### Positives

- Parité fonctionnelle complète entre CLI et TUI pour les 3 wizards d'init
- Team init TUI couvre les 6 steps avec les 6 champs d'identité + commit-and-push
- Expérience utilisateur fluide — pas de switch alt-screen, shell chrome préservé
- Composant `InlineWizardView` réutilisable pour tout futur wizard
- Back navigation (`Ctrl+B`), step bar, info panel, validation, spinner async
- Écran résumé final avec proposition de navigation contextuelle
- Les wizards CLI standalone (`RunWizard`) restent comme fallback `--no-tui`

### Négatives / Compromis

- Deux implémentations wizard coexistent (`RunWizard` standalone + `InlineWizardView` inline)
- L'omnibar reste visible pendant le wizard (5 lignes de moins pour le contenu)
- Les steps sont définis deux fois pour team init (CLI dans `team.go`, TUI dans `tui_team_actions.go`) — un refactoring ultérieur pourrait les factoriser

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| `SuspendAndExec` + wizard standalone | Switch alt-screen brutal — le shell disparaît et réapparaît, perte de contexte |
| Enrichir les modals chaînés existants | Pas de back navigation, pas de step bar, closures imbriquées fragiles, `time.Sleep` entre modals |
| Remplacer `RunWizard` par `InlineWizardView` partout | Le CLI n'a pas de shell TUI — `RunWizard` reste nécessaire pour les commandes one-shot |

## Fichiers concernés

- `views/inline_wizard.go` — composant `InlineWizardView` (nouveau)
- `views/inline_wizard_test.go` — 7 tests unitaires (nouveau)
- `views/view.go` — ajout `PushView(v View)` à `ShellAccess`
- `shell/shell.go` — implémentation `PushView` avec injection `shellAware`
- `cmd/tui_team_actions.go` — réécriture `actionTeamInit()` en 6 steps inline
- `cmd/init_wizard.go` — ajout `buildFirstRunInlineWizard()`
- `cmd/tui.go` — first-run pushé inline après démarrage du shell
- `cmd/tui_actions.go` — ajout `actionProjectAdd()` et `buildProjectAddInlineWizard()`
- `cmd/tui_commands.go` — commandes omnibar `init` et `project.add`
