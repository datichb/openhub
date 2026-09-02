---
name: tui-patterns
description: Patterns de design specifiques aux interfaces TUI (terminal) -- contraintes terminales, patterns d'interaction clavier-first, catalogue des widgets Aurum/OpenHub, anti-patterns TUI. Reference obligatoire quand le designer travaille sur le TUI OpenHub.
---

# Patterns TUI -- reference

Les TUI ne sont pas des interfaces web en monochrome. Elles ont leurs propres contraintes,
patterns et forces. Ce skill couvre les regles specifiques au terminal.

---

## Contraintes terminales fondamentales

### Grille de caracteres

Tout est aligne sur une grille de caracteres (colonnes x lignes). Pas de pixels, pas de
sous-pixel, pas de positions fractionnaires. Un caractere = une cellule.
- Largeurs en **colonnes** (typiquement 80-200+ colonnes)
- Hauteurs en **lignes** (typiquement 24-60+ lignes)
- Un widget occupe un nombre entier de colonnes/lignes

### Couleurs

- **True color** (24-bit) : supporte par la plupart des terminaux modernes (iTerm2, kitty, Wezterm, Windows Terminal)
- **256 couleurs** : fallback pour les terminaux plus anciens
- Le design system Aurum utilise Catppuccin Mocha (true color) comme palette
- **Jamais se fier uniquement a la couleur** : toujours doubler avec icone Unicode ou texte

### Pas de souris fiable

La souris fonctionne dans certains terminaux mais pas tous. Les interactions doivent
fonctionner **uniquement au clavier**. La souris est un bonus, pas une dependance.

### Navigation clavier-first

Le clavier est le moyen d'interaction principal. Chaque ecran doit etre entierement
utilisable au clavier. Les raccourcis doivent etre decouvreables (affichees dans la
status bar ou le help overlay).

### Taille terminal variable

Le terminal peut etre redimensionne a tout moment. Les layouts doivent s'adapter.
Definir des seuils minimum (ex : 80 colonnes, 24 lignes) en dessous desquels un
message "terminal trop petit" s'affiche.

---

## Patterns TUI d'OpenHub

### Omnibar (Command Palette)

Pattern central d'OpenHub. L'omnibar en bas de l'ecran sert a la fois de :
- **Barre de hints** (mode passif) : affiche les raccourcis clavier contextuels
- **Command palette** (mode actif, Ctrl+P) : recherche fuzzy dans toutes les commandes
- **Saisie rapide** : taper un caractere active l'omnibar avec ce caractere pre-rempli

**Regles :**
- Position fixe en bas (5 lignes)
- Le contenu principal occupe tout le reste de l'ecran
- Les suggestions flottent au-dessus de l'omnibar
- Escape ferme l'omnibar et revient au contenu
- Les vues peuvent injecter des commandes contextuelles via `CommandProvider`

### Stack-based Router (navigation entre vues)

Navigation par pile : Push (aller vers), Pop (revenir), Replace (remplacer la vue courante).

**Regles :**
- Escape = Pop (retour)
- Chaque vue a un cycle de vie Mount/Unmount
- Pas de navigation directe entre vues soeurs -- toujours via le router
- Le titre de la vue courante est visible (pour l'orientation)

### Overlay system (modals TUI)

Les modals utilisent `tview.Pages` avec une grille 3x3 : fond assombri sur les 8
cellules peripheriques, contenu dans la cellule centrale.

**Regles :**
- Deux couches : `inline-overlay` (modal primaire) + `sub-overlay` (sous-modal)
- Jamais plus de 2 niveaux d'overlay
- Clic sur le fond assombri ferme les overlays simples (pas les formulaires)
- Escape ferme le dernier overlay

### Toast notifications

Notifications temporaires en haut a droite. 4 niveaux : Success, Error, Warning, Info.

**Regles :**
- Auto-dismiss (duree configurable)
- Non bloquantes (ne prennent pas le focus)
- Maximum 1 toast visible a la fois
- Historique persistant dans un ring buffer JSONL

### Inline Forms (formulaires modaux)

Formulaires dans un overlay centre. 5 types de champs : Text, Password, Select,
MultiSelect, Bool.

**Regles :**
- Tab pour naviguer entre les champs
- Enter pour valider / ouvrir un sous-menu
- Escape pour annuler le formulaire (avec confirmation si donnees saisies)
- Select : fleches gauche/droite pour cycler inline, Enter pour la sous-modal si 5+ options
- MultiSelect : Space ou x pour toggler les options

### FilterableList (liste filtrable)

Liste avec champ de filtre integre. Le filtre est toujours visible et actif.

**Regles :**
- La saisie filtre en temps reel (fuzzy)
- Fleches haut/bas pour naviguer dans la liste filtree
- Enter pour selectionner
- Le nombre de resultats est affiche

### SectionedList (liste par sections)

Liste avec des headers de section non selectionnables et navigation par section.

**Regles :**
- Les headers de section sont visuellement distincts (gras, couleur differente)
- `{` et `}` pour sauter entre les sections
- j/k ou fleches pour la navigation item par item
- Les headers sont sautes automatiquement lors de la navigation

### StepBar (barre de progression wizard)

Indicateur horizontal de progression pour les processus multi-etapes.

**Regles :**
- Etapes passees en couleur Success
- Etape courante en couleur Accent (focus)
- Etapes futures en couleur Muted
- Labels courts (1-2 mots par etape)
- Maximum 5-7 etapes visibles

---

## Tokens Aurum -- quick reference

### Arriere-plans (3 niveaux de profondeur)

| Token | Hex | Usage |
|-------|-----|-------|
| `BgApp` | #181825 (Mantle) | Fond le plus profond -- shell, barre d'etat |
| `BgPanel` | #1e1e2e (Base) | Surface principale -- contenu, panneaux |
| `BgElement` | #313244 (Surface0) | Elements eleves -- cards, boutons, headers |

### Texte (3 niveaux de contraste)

| Token | Hex | Usage |
|-------|-----|-------|
| `TextPrimary` | #cdd6f4 | Titres, contenu principal |
| `TextSecondary` | #a6adc8 | Descriptions, texte secondaire |
| `TextMuted` | #7f849c | Placeholders, texte desactive |

### Couleurs semantiques

| Token | Hex | Couleur | Usage |
|-------|-----|---------|-------|
| `Accent` | #89b4fa | Blue | Focus structurel, selection |
| `Action` | #fab387 | Peach | CTA, elements actifs |
| `Success` | #a6e3a1 | Green | Confirmations, succes |
| `Warning` | #f9e2af | Yellow | Alertes non bloquantes |
| `Error` | #f38ba8 | Red | Erreurs, danger |
| `Info` | #b4befe | Lavender | En cours, informationnel |

### Bordures

| Token | Hex | Usage |
|-------|-----|-------|
| `BorderNormal` | #313244 | Bordure par defaut |
| `BorderFocus` | #89b4fa (= Accent) | Bordure element focusse |
| `BorderActive` | #fab387 (= Action) | Bordure element actif |

### Icones Unicode

| Icone | Caractere | Usage |
|-------|-----------|-------|
| `IconDone` | `checkmark` | Etape completee, succes |
| `IconPending` | `circle` | Etape en attente |
| `IconCurrent` | `arrow` | Etape courante |
| `IconError` | `cross` | Erreur |
| `IconWarning` | `warning triangle` | Avertissement |
| `IconInfo` | `info circle` | Information |

---

## Regles de layout TUI

### Espacement

- **Padding horizontal du contenu :** 2 colonnes (gauche et droite)
- **Padding vertical haut :** 1 ligne
- **Separateurs de section :** 1 ligne vide
- **Indentation sous-elements :** 2 espaces

### Responsive terminal

- Detecter la taille du terminal a chaque resize
- **Seuil minimum :** 80 colonnes x 24 lignes
- En dessous : message "Terminal trop petit" avec les dimensions requises
- Adapter les layouts :
  - Large (120+ col) : sidebar + contenu + panneau info
  - Medium (80-119 col) : contenu + omnibar
  - Small (< 80 col) : contenu seul, simplifie

### Focus management

- Un seul element focusse a la fois (visible par bordure/couleur `BorderFocus`)
- Tab / Shift+Tab pour cycler entre les zones de focus
- Le focus ne doit jamais etre invisible -- si aucun element visuel n'a le focus, c'est un bug
- Restaurer le focus apres fermeture d'un overlay (revenir a l'element qui l'a ouvert)

---

## Anti-patterns TUI

| Anti-pattern | Probleme | Correction |
|--------------|----------|------------|
| **Trop de couleurs** | Interface carnaval, fatigue visuelle | Palette restreinte (Aurum : 6 semantiques + 3 textes + 3 fonds) |
| **Scroll horizontal** | Le terminal ne gere pas bien le scroll horizontal | Wrap le texte, tronquer avec ellipsis, adapter au viewport |
| **Menus profonds (3+ niveaux)** | Impossible de naviguer efficacement au clavier | Flat navigation + omnibar search |
| **Texte tronque sans indication** | L'utilisateur ne sait pas qu'il manque du contenu | Ellipsis (`...`) + tooltip ou expand |
| **Manque de raccourcis clavier** | L'utilisateur expert ne peut pas accelerer | Raccourcis pour toutes les actions frequentes, affiches dans le help |
| **ASCII art excessif** | Decoratif, consomme de l'espace utile | Unicode sobre pour les bordures et icones. Pas de logo ASCII |
| **Animations lourdes** | Scintillement, consommation CPU | Animations subtiles (spinner braille, shimmer sobre) |
| **Couleur seule pour le sens** | Inaccessible si monochrome ou daltonien | Icone + texte + couleur pour chaque etat |
| **Modal dans modal dans modal** | Confusion de contexte, perte de l'etat | Maximum 2 niveaux d'overlay |
| **Pas de confirmation sur destructif** | Actions irreversibles sans filet | Confirmation dialog avant suppression/reset |

---

## Catalogue des vues OpenHub (27 vues)

Reference rapide des vues existantes pour verifier l'existant avant de specifier :

| Vue | ID | Description |
|-----|----|-------------|
| Home | `home` | Tableau de bord principal |
| Board | `board` | Vue kanban Beads |
| Team Board | `team.board` | Board d'equipe |
| Teams | `teams` | Liste des equipes |
| Team Detail | `team.detail` | Detail d'une equipe |
| Team Activity | `team.activity` | Flux d'activite |
| Team Status | `team.status` | Statut de l'equipe |
| Team Briefs | `team.briefs` | Takeover/briefs |
| Team Patterns | `team.patterns` | Patterns d'equipe |
| Team Policies | `team.policies` | Politiques |
| Settings | `settings` | Parametres |
| Secrets | `secrets` | Gestion des secrets |
| Projects | `projects.list` | Liste des projets |
| Project Config | `project.config` | Configuration projet |
| Project Mode | `project.mode` | Mode du projet |
| Models | `models` | Modeles IA |
| Provider | `provider` | Configuration provider |
| Plugins | `plugins` | Gestion plugins |
| MCP | `mcp` | Serveurs MCP |
| Metrics | `metrics` | Metriques |
| Merge | `merge` | Merge/integration |
| Worktrees | `worktrees` | Gestion worktrees |
| Parallel | `parallel` | Execution parallele |
| Notifications | `notifications` | Historique notifications |
| Status | `status` | Statut systeme |
| Doctor | `doctor` | Diagnostic |
| Help | `help` | Aide/documentation |
