---
name: tui-patterns
description: "Utiliser quand la spec porte sur une interface terminal (TUI OpenHub) : contraintes, clavier d'abord, widgets, anti-patterns."
---

# Patterns TUI — référence

Les TUI ne sont pas des interfaces web en monochrome. Elles ont leurs propres contraintes,
patterns et forces. Ce skill couvre les règles spécifiques au terminal.

---

## Contraintes terminales fondamentales

### Grille de caractères

Tout est aligné sur une grille de caractères (colonnes x lignes). Pas de pixels, pas de
sous-pixel, pas de positions fractionnaires. Un caractère = une cellule.
- Largeurs en **colonnes** (typiquement 80-200+ colonnes)
- Hauteurs en **lignes** (typiquement 24-60+ lignes)
- Un widget occupe un nombre entier de colonnes/lignes

### Couleurs

- **True color** (24-bit) : supporté par la plupart des terminaux modernes (iTerm2, kitty, Wezterm, Windows Terminal)
- **256 couleurs** : fallback pour les terminaux plus anciens
- Le design system Aurum utilise Catppuccin Mocha (true color) comme palette
- **Ne jamais se fier uniquement à la couleur** : toujours doubler avec une icône Unicode ou du texte

### Pas de souris fiable

La souris fonctionne dans certains terminaux mais pas tous. Les interactions doivent
fonctionner **uniquement au clavier**. La souris est un bonus, pas une dépendance.

### Navigation clavier-first

Le clavier est le moyen d'interaction principal. Chaque écran doit être entièrement
utilisable au clavier. Les raccourcis doivent être découvrables (affichés dans la
status bar ou le help overlay).

### Taille terminal variable

Le terminal peut être redimensionné à tout moment. Les layouts doivent s'adapter.
Définir des seuils minimum (ex : 80 colonnes, 24 lignes) en dessous desquels un
message "terminal trop petit" s'affiche.

---

## Patterns TUI d'OpenHub

### Omnibar (Command Palette)

Pattern central d'OpenHub. L'omnibar en bas de l'écran sert à la fois de :
- **Barre de hints** (mode passif) : affiche les raccourcis clavier contextuels
- **Command palette** (mode actif, Ctrl+P) : recherche fuzzy dans toutes les commandes
- **Saisie rapide** : taper un caractère active l'omnibar avec ce caractère pré-rempli

**Règles :**
- Position fixe en bas (5 lignes)
- Le contenu principal occupe tout le reste de l'écran
- Les suggestions flottent au-dessus de l'omnibar
- Escape ferme l'omnibar et revient au contenu
- Les vues peuvent injecter des commandes contextuelles via `CommandProvider`

### Stack-based Router (navigation entre vues)

Navigation par pile : Push (aller vers), Pop (revenir), Replace (remplacer la vue courante).

**Règles :**
- Escape = Pop (retour)
- Chaque vue a un cycle de vie Mount/Unmount
- Pas de navigation directe entre vues sœurs — toujours via le router
- Le titre de la vue courante est visible (pour l'orientation)

### Overlay system (modals TUI)

Les modals utilisent `tview.Pages` avec une grille 3x3 : fond assombri sur les 8
cellules périphériques, contenu dans la cellule centrale.

**Règles :**
- Deux couches : `inline-overlay` (modal primaire) + `sub-overlay` (sous-modal)
- Jamais plus de 2 niveaux d'overlay
- Un clic sur le fond assombri ferme les overlays simples (pas les formulaires)
- Escape ferme le dernier overlay

### Toast notifications

Notifications temporaires en haut à droite. 4 niveaux : Success, Error, Warning, Info.

**Règles :**
- Auto-dismiss (durée configurable)
- Non bloquantes (ne prennent pas le focus)
- Maximum 1 toast visible à la fois
- Historique persistant dans un ring buffer JSONL

### Inline Forms (formulaires modaux)

Formulaires dans un overlay centré. 5 types de champs : Text, Password, Select,
MultiSelect, Bool.

**Règles :**
- Tab pour naviguer entre les champs
- Enter pour valider / ouvrir un sous-menu
- Escape pour annuler le formulaire (avec confirmation si des données ont été saisies)
- Select : flèches gauche/droite pour cycler inline, Enter pour la sous-modal si 5+ options
- MultiSelect : Space ou x pour basculer les options

### FilterableList (liste filtrable)

Liste avec champ de filtre intégré. Le filtre est toujours visible et actif.

**Règles :**
- La saisie filtre en temps réel (fuzzy)
- Flèches haut/bas pour naviguer dans la liste filtrée
- Enter pour sélectionner
- Le nombre de résultats est affiché

### SectionedList (liste par sections)

Liste avec des headers de section non sélectionnables et navigation par section.

**Règles :**
- Les headers de section sont visuellement distincts (gras, couleur différente)
- `{` et `}` pour sauter entre les sections
- j/k ou flèches pour la navigation item par item
- Les headers sont sautés automatiquement lors de la navigation

### StepBar (barre de progression wizard)

Indicateur horizontal de progression pour les processus multi-étapes.

**Règles :**
- Étapes passées en couleur Success
- Étape courante en couleur Accent (focus)
- Étapes futures en couleur Muted
- Labels courts (1-2 mots par étape)
- Maximum 5-7 étapes visibles

---

## Tokens Aurum — quick reference

### Arrière-plans (3 niveaux de profondeur)

| Token | Hex | Usage |
|-------|-----|-------|
| `BgApp` | #181825 (Mantle) | Fond le plus profond — shell, barre d'état |
| `BgPanel` | #1e1e2e (Base) | Surface principale — contenu, panneaux |
| `BgElement` | #313244 (Surface0) | Éléments surélevés — cards, boutons, headers |

### Texte (3 niveaux de contraste)

| Token | Hex | Usage |
|-------|-----|-------|
| `TextPrimary` | #cdd6f4 | Titres, contenu principal |
| `TextSecondary` | #a6adc8 | Descriptions, texte secondaire |
| `TextMuted` | #7f849c | Placeholders, texte désactivé |

### Couleurs sémantiques

| Token | Hex | Couleur | Usage |
|-------|-----|---------|-------|
| `Accent` | #89b4fa | Blue | Focus structurel, sélection |
| `Action` | #fab387 | Peach | CTA, éléments actifs |
| `Success` | #a6e3a1 | Green | Confirmations, succès |
| `Warning` | #f9e2af | Yellow | Alertes non bloquantes |
| `Error` | #f38ba8 | Red | Erreurs, danger |
| `Info` | #b4befe | Lavender | En cours, informationnel |

### Bordures

| Token | Hex | Usage |
|-------|-----|-------|
| `BorderNormal` | #313244 | Bordure par défaut |
| `BorderFocus` | #89b4fa (= Accent) | Bordure de l'élément focusé |
| `BorderActive` | #fab387 (= Action) | Bordure de l'élément actif |

### Icônes Unicode

| Icône | Caractère | Usage |
|-------|-----------|-------|
| `IconDone` | `checkmark` | Étape complétée, succès |
| `IconPending` | `circle` | Étape en attente |
| `IconCurrent` | `arrow` | Étape courante |
| `IconError` | `cross` | Erreur |
| `IconWarning` | `warning triangle` | Avertissement |
| `IconInfo` | `info circle` | Information |

---

## Règles de layout TUI

### Espacement

- **Padding horizontal du contenu :** 2 colonnes (gauche et droite)
- **Padding vertical haut :** 1 ligne
- **Séparateurs de section :** 1 ligne vide
- **Indentation des sous-éléments :** 2 espaces

### Responsive terminal

- Détecter la taille du terminal à chaque resize
- **Seuil minimum :** 80 colonnes x 24 lignes
- En dessous : message "Terminal trop petit" avec les dimensions requises
- Adapter les layouts :
  - Large (120+ col) : sidebar + contenu + panneau info
  - Medium (80-119 col) : contenu + omnibar
  - Small (< 80 col) : contenu seul, simplifié

### Focus management

- Un seul élément focusé à la fois (visible par bordure/couleur `BorderFocus`)
- Tab / Shift+Tab pour cycler entre les zones de focus
- Le focus ne doit jamais être invisible — si aucun élément visuel n'a le focus, c'est un bug
- Restaurer le focus après la fermeture d'un overlay (revenir à l'élément qui l'a ouvert)

---

## Anti-patterns TUI

| Anti-pattern | Problème | Correction |
|--------------|----------|------------|
| **Trop de couleurs** | Interface carnaval, fatigue visuelle | Palette restreinte (Aurum : 6 sémantiques + 3 textes + 3 fonds) |
| **Scroll horizontal** | Le terminal gère mal le scroll horizontal | Revenir à la ligne, tronquer avec ellipsis, adapter au viewport |
| **Menus profonds (3+ niveaux)** | Impossible de naviguer efficacement au clavier | Navigation à plat + recherche dans l'omnibar |
| **Texte tronqué sans indication** | L'utilisateur ne sait pas qu'il manque du contenu | Ellipsis (`...`) + tooltip ou expand |
| **Manque de raccourcis clavier** | L'utilisateur expert ne peut pas accélérer | Raccourcis pour toutes les actions fréquentes, affichés dans l'aide |
| **ASCII art excessif** | Décoratif, consomme de l'espace utile | Unicode sobre pour les bordures et icônes. Pas de logo ASCII |
| **Animations lourdes** | Scintillement, consommation CPU | Animations subtiles (spinner braille, shimmer sobre) |
| **Couleur seule pour le sens** | Inaccessible en monochrome ou pour les daltoniens | Icône + texte + couleur pour chaque état |
| **Modal dans modal dans modal** | Confusion de contexte, perte de l'état | Maximum 2 niveaux d'overlay |
| **Pas de confirmation sur destructif** | Actions irréversibles sans filet | Confirmation dialog avant suppression/reset |

---

## Catalogue des vues OpenHub

Référence rapide des vues existantes (`cli/internal/tui/v2/views`) pour vérifier l'existant avant de spécifier :

| Vue | ID | Description |
|-----|----|-------------|
| Home | `home` | Tableau de bord principal |
| Board | `board` | Vue kanban Beads |
| Sessions | `sessions` | Sessions de travail en cours |
| Launch | `launch.<workflow>` | Formulaire de lancement d'un workflow |
| Workflows | `workflows` | Catalogue des workflows |
| Workflow Editor | `workflow.edit.<ref>` | Édition d'un workflow (historique : `workflow.history.<ref>`, publication : `workflow.publish.<ref>`) |
| Team Mode | `team.mode` | Accueil du mode équipe |
| Team Board | `team.board` | Board d'équipe |
| Teams | `teams` | Liste des équipes |
| Team Detail | `team.detail` | Détail d'une équipe |
| Team Activity | `team.activity` | Flux d'activité |
| Team Status | `team.status` | Statut de l'équipe |
| Team Briefs | `team.briefs` | Takeover/briefs |
| Team Patterns | `team.patterns` | Patterns d'équipe |
| Team Policies | `team.policies` | Politiques |
| Team Wiki | `team.wiki` | Wiki d'équipe |
| Team Models / MCP | `team.models`, `team.mcp` | Modèles et serveurs MCP de l'équipe |
| Settings | `settings` | Paramètres |
| Secrets | `secrets` | Gestion des secrets |
| Projects | `projects.list` | Liste des projets |
| Project Config | `project.config` | Configuration projet |
| Project Mode | `project.mode` | Mode du projet |
| Project Agents / Models / MCP | `project.agents`, `project.models`, `project.mcp` | Agents, modèles et serveurs MCP du projet |
| Models | `models` | Modèles IA |
| Provider | `provider` | Configuration provider |
| MCP | `mcp` | Serveurs MCP |
| Metrics | `metrics` | Métriques |
| Worktrees | `worktrees` | Gestion des worktrees |
| Notifications | `notifications` | Historique des notifications |
| Status | `status` | Statut système |
| Doctor | `doctor` | Diagnostic |
| Help | `help` | Aide/documentation |
