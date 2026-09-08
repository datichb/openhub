# Aurum — Design System

Direction artistique du projet OpenHub (`oh`).

## Identité

| Attribut | Valeur |
|----------|--------|
| Nom | Aurum |
| Feeling | Premium & sophistiqué, flottant |
| Mots-clés | Chaleur, profondeur, précision, élégance, floating |
| Inspiration | Arc Browser, Raycast, Warp — panneaux flottants sur fond sombre |

## Principes

1. **Profondeur par le fond** — 3 niveaux de luminosité pour créer la hiérarchie (terminal → panel → element). Le fond est le principal outil de séparation visuelle.
2. **Espacement** — Toujours au moins 1 ligne vide entre les sections. Le vide crée la structure.
3. **Focus** — Un seul élément accentué actif à la fois. Le regard est guidé.
4. **Floating** — Les panels "flottent" au-dessus du terminal grâce à un fond plus clair et des bordures quasi-invisibles.
5. **Sobriété** — Peu de couleurs simultanées. Les bordures sont des murmures, pas des cris. Les séparateurs structurels (colonnes, sections) restent subtils — même famille de couleur que le fond.

## Palette

Base : **Catppuccin Mocha** — palette prouvée sur fond `#1e1e2e`, excellente lisibilité, large adoption.

### Couleurs sémantiques — système dual-accent

Le design utilise deux accents complémentaires :
- **Accent** (Blue) pour les éléments structurels : bordures focus, navigation, sélection
- **Action** (Peach) pour les éléments interactifs : CTA, menu actif, spinners

| Rôle | Nom code | Hex | Usage |
|------|----------|-----|-------|
| **Accent** | Blue | `#89b4fa` | Bordure focus, navigation, sélection, badges projet |
| **Action** | Peach | `#fab387` | CTA actif, menu item actif, bordure active, spinners |
| **Success** | Green | `#a6e3a1` | Confirmations, étapes complétées, badges beads complets |
| **Warning** | Yellow | `#f9e2af` | Avertissements, priorité medium, colonne TODO |
| **Error** | Red | `#f38ba8` | Erreurs, bloqué, critique, priorité P0 |
| **Info** | Lavender | `#b4befe` | En cours, running, colonne VALIDATION |

### Couleurs de texte

| Rôle | Nom code | Hex | Usage |
|------|----------|-----|-------|
| **Primary** | Text | `#cdd6f4` | Titres, contenu principal, texte des cards |
| **Secondary** | Subtext0 | `#a6adc8` | Labels, descriptions, flèches de navigation |
| **Muted** | Overlay1 | `#7f849c` | Metadata, timestamps, IDs, placeholders |

### Couleurs de profondeur (3 niveaux)

| Niveau | Rôle | Hex | Usage |
|--------|------|-----|-------|
| 0 | App (Mantle) | `#181825` | Fond terminal, arrière-plan global |
| 1 | Panel (Base) | `#1e1e2e` | Surface : colonnes, sidebar, zone de contenu |
| 2 | Element (Surface0) | `#313244` | Surélevé : cards, sélection, header bar |

### Couleurs de bordure

| Rôle | Hex | Usage |
|------|-----|-------|
| BorderNormal | `#313244` | Bordures de panel discrètes |
| BorderCard | `#45475a` | Bordures de cards et séparateurs de colonnes — subtiles |
| BorderFocus | `#89b4fa` | = Accent (élément avec le focus) |
| BorderActive | `#fab387` | = Action (menu item actif) |

## Variantes de thème

### Mocha (actif)

Palette Catppuccin Mocha — fond neutre lavande-gris. Universel, s'adapte à tous les terminaux.

| Rôle | Hex | Constante code | Description |
|------|-----|----------------|-------------|
| Surface (Panel) | `#1e1e2e` | `BgPanelHex` | Fond de colonne, sidebar, zone de contenu |
| SurfaceElem (Card) | `#313244` | `BgCardHex` | Fond des cards surélevées |
| Border | `#313244` | `BorderNormalHex` | Bordure panel (quasi-invisible) |
| BorderCard | `#45475a` | `BorderCardHex` | Bordure card + séparateurs (subtile) |

### Bleu Nuit (alternatif — non implémenté)

Teinte indigo dans le noir. Plus distinctif, plus "nuit étoilée".

| Rôle | Hex | Description |
|------|-----|-------------|
| Surface | `#1a1a2e` | Panel bg |
| SurfaceElem | `#303050` | Element bg (surélevé) |
| Border | `#222238` | Bordure panel (quasi-invisible) |
| BorderElem | `#3a3a5a` | Bordure element (très subtile) |

## Iconographie

### Steps (wizards, progression)

| État | Icône | Fallback | Couleur |
|------|-------|----------|---------|
| Done | `●` | `●` | Success/Green |
| Active | `◔` | `►` | Action/Peach |
| Pending | `○` | `○` | Muted |
| Skipped | `○` | `○` | Muted (dim) |

### Actions / Statuts

| Élément | Icône | Couleur |
|---------|-------|---------|
| Succès | `✓` | Success/Green |
| Erreur | `✗` | Error/Red |
| Warning | `!` | Warning/Yellow |
| Indicateur actif | `▸` | Action/Peach |
| Point neutre | `·` | Muted |

### Connecteurs

| Type | Caractère | Couleur | Usage |
|------|-----------|---------|-------|
| Entre steps | `───` | Muted | Step bar horizontal |

## Bordures

| Propriété | Valeur |
|-----------|--------|
| Type | Rounded (`╭╮╯╰`) — cards et panels |
| Couleur card (repos) | BorderCard `#45475a` (subtile, visible sur BgPanel) |
| Couleur card (focus) | BorderFocus `#89b4fa` (= Accent/Blue) |
| Couleur panel | BorderNormal `#313244` (quasi-invisible) |
| Couleur active | BorderActive `#fab387` (= Action/Peach) |
| Séparateur colonne | `│` en BorderCard — trait vertical entre colonnes kanban |

## Espacement

| Zone | Règle |
|------|-------|
| Entre sections d'un panel | 1 ligne vide |
| Padding panel interne | 1 char left/right minimum |
| Entre step bar et form | 1 ligne vide |
| Footer → bord inférieur | 1 ligne vide au-dessus et en-dessous |
| Entre cards kanban | 1 ligne vide |
| Marge latérale card | 1 char left/right (card indentée dans la colonne) |
| Header colonne → première card | 1 ligne vide (après le séparateur `─`) |

## Composants

### Wizard (alt-screen, floating panels)

```
╭────────────────────────────────────────────────────────────╮  ← border quasi-invisible
│                                                            │  ← Surface (panel bg)
│   Titre du Wizard                                          │  ← Primary bold
│                                                            │
│  ╭──────────────────────────────────────────────────────╮  │
│  │                                                      │  │  ← SurfaceElem + BorderElem
│  │  ● Label ─── ◔ Label ─── ○ Label ─── ○ Label        │  │  ← step bar
│  │                                                      │  │
│  │  N/M · Label de l'étape                              │  │  ← Action bold
│  │                                                      │  │
│  │  [Contenu du formulaire — pleine largeur]            │  │
│  │                                                      │  │
│  ╰──────────────────────────────────────────────────────╯  │
│                                                            │
│   enter confirmer · esc passer · ctrl+c quitter            │  ← Secondary
│                                                            │
╰────────────────────────────────────────────────────────────╯
```

### Sidebar (inline, pour oh init)

```
  Titre                    ← Action bold
 
  Étapes
  ● Label complété         ← Success/Green
  ◔ Label actif            ← Action/Peach
  ○ Label à venir          ← Muted
```

Pas de bordure (rendu inline dans le terminal). Pas de fond (pas de contrôle background en mode inline).

### Title Bar (dashboard)

- Background : Surface (BgPanel)
- Foreground : Primary
- Pas de bordure arrondie (barre, pas un panel)
- Note : les boards kanban n'utilisent plus de title bar — le header de colonne est intégré dans le widget `CardColumn`

### Cards / Kanban (`widgets/cardcolumn.go`)

Chaque ticket est rendu comme une **card individuelle** avec bordure rounded,
fond surélevé, et espacement. Les colonnes n'ont pas de bordure propre — la
structure est donnée par le header coloré, les séparateurs verticaux, et la
profondeur de fond (Panel vs Card).

#### Structure d'une colonne

```
 ◄ IN PROGRESS (3) ►          ← header : titleColor bold + compteur
 ─────────────────────         ← séparateur horizontal (BorderCard)
                               ← ligne vide
 ╭───────────────────╮         ← bordure card (BorderCard ou BorderFocus si sélectionnée)
 │ P1 · Fix auth bug │         ← ligne 1 : priorité colorée + titre (Primary)
 │ BD-001 · bug      │         ← ligne 2 : ID + type (Muted)
 │ ← gitlab-42       │         ← ligne 3 : metadata — ref externe, assignee, labels (Muted)
 ╰───────────────────╯
                               ← espacement (1 ligne vide)
 ╭───────────────────╮
 │ P2 · Add search   │
 │ BD-003 · feature  │
 │ @benjamin         │
 ╰───────────────────╯
```

#### Tokens visuels

| Élément | Token | Valeur |
|---------|-------|--------|
| Fond colonne | BgPanel | `#1e1e2e` |
| Fond card | BgCard | `#313244` |
| Bordure card (repos) | BorderCard | `#45475a` |
| Bordure card (focus) | BorderFocus | `#89b4fa` |
| Header colonne | titleColor (sémantique) | Yellow=TODO, Blue=In Progress, etc. |
| Texte titre | Primary | `#cdd6f4` |
| Texte metadata | Muted | `#7f849c` |
| Séparateur vertical | `│` en BorderCard | `#45475a` |
| Flèches navigation | `◄ ►` en Secondary | `#a6adc8` (colonne focusée uniquement) |

#### Card — 3 lignes de contenu

| Ligne | Contenu (BoardView) | Contenu (TeamBoardView) |
|-------|---------------------|------------------------|
| 1 | Priorité colorée + titre | Badge projet + titre + badge beads |
| 2 | ID · type | ID · priorité |
| 3 | Réf. externe (si présente) | @assignee · labels (filtrés) |

La ligne 3 est optionnelle : si vide, la card fait 2 lignes de contenu au lieu de 3.

#### Navigation

- **h/l** (ou ←/→) : déplacer le focus entre colonnes
- **j/k** (ou ↑/↓) : naviguer entre cards dans la colonne
- **Flèches `◄ ►`** dans le header : apparaissent uniquement sur la colonne focusée, indiquent les colonnes adjacentes
- **Séparateurs `│`** entre colonnes : toujours visibles, délimitent les colonnes
- **Scroll vertical** automatique quand les cards dépassent la hauteur

#### Implémentation

Widget : `cli/internal/tui/v2/widgets/cardcolumn.go` — custom `tview.Primitive`.

4 boards utilisent ce widget de façon identique :
- `board.go` / `board_view.go` — board personnel (projet)
- `teamboard.go` / `teamboard_view.go` — board équipe (team-state)

## Anti-patterns

- Ne **jamais** utiliser de séparateurs lourds (`━━━`, couleurs vives) pour délimiter des zones. Les séparateurs subtils en `BorderCard` (`│`, `─`) sont acceptés pour la structure (colonnes kanban, sections).
- Ne **jamais** utiliser la couleur Action (Peach) pour du texte long (illisible). Réservé aux labels courts, icônes, bordures actives.
- Ne **jamais** mélanger plus de 3 couleurs sémantiques dans la même zone visuelle.
- Ne **jamais** utiliser de couleurs hardcodées (`lipgloss.Color("99")`) — toujours via `theme.*`.
- Ne **jamais** coller un formulaire directement sous un titre sans espacement.
- Les bordures sont des **murmures** : quasi-invisibles, même famille que le fond.

## Composants avancés

### Thème huh "Aurum" (`theme/huh.go`)

Tous les formulaires huh utilisent un thème personnalisé qui applique la palette :
- Bordure active : Action/Peach (`BorderActive`)
- Titre : Action bold
- Sélecteur : Accent/Blue
- Selected : Success/Green (`✓`)
- Erreurs : Error/Red
- Curseur : Success/Green
- Bouton focus : Action bg + Surface fg
- Bouton blur : SurfaceElem bg + Primary fg

Usage : `theme.NewForm(groups...)` au lieu de `huh.NewForm(groups...)`.

### Summary Card (`components/summary/`)

Carte récapitulative affichée après un wizard alt-screen ou un flow multi-étapes :

```
╭──────────────────────────────────────────╮
│  ✓ Team Setup Complete                   │
│                                          │
│  Repo      git@gitlab.com:team/state     │
│  Member    benjamin (lead)               │
│  Policies  3 active                      │
│                                          │
│  Run `oh team status` for details        │
╰──────────────────────────────────────────╯
```

- Bordure : BorderCard
- Icône : couleur sémantique (Success/Error/Warning)
- Labels : Secondary
- Valeurs : Primary
- Footer : Muted italic

### Floating Prompt (`components/floating/`)

Panel inline (pas alt-screen) pour les prompts simples — style "command palette" Raycast :

```
╭──────────────────────────────────╮
│  ◔ Quick Launch                  │
│                                  │
│  ┃ Choose your project           │
│  ┃ > my-api — /home/dev/my-api  │
│  ┃   frontend — /home/dev/front │
│                                  │
│  enter · confirm  esc · cancel   │
╰──────────────────────────────────╯
```

- Bordure : BorderCard
- Titre : Action bold
- Contenu : formulaire huh avec thème Aurum
- Footer : Muted

Usage : `floating.Run(floating.Config{Title: "...", Form: form})`

Fallback automatique vers `form.Run()` si `UseRichTUI()` retourne false.

### Wizard Spinner

Pendant l'exécution d'un `OnDone` dans le wizard, un spinner Dot animé (Action/Peach) remplace la zone du formulaire :

```
  ⠋ Enregistrement du profil...
```

Le champ `StepConfig.Processing` définit le message affiché.

## Détection terminal (`common/detect.go`)

`UseRichTUI()` retourne false si :
- `--no-tui` flag passé
- `stdin` n'est pas un terminal (pipe)
- `CI=true`
- `TERM=dumb`
- `OH_RICH_TUI=0`

Les composants FloatingPrompt et le wizard utilisent cette détection pour fallback.

## Migration

Pour ajouter une couleur ou un composant au design system :
1. Ajouter la constante dans `cli/internal/tui/theme/colors.go`
2. Dériver les valeurs tcell dans `cli/internal/tui/theme/tcell.go`
3. Dériver les valeurs lipgloss dans `cli/internal/tui/theme/lipgloss.go` (si nécessaire)
4. Documenter dans ce fichier (`docs/design/aurum.md`)
5. Ne jamais utiliser de `lipgloss.Color("...")` littéral dans les vues — toujours via `theme.*`
6. Utiliser true color hex (`#rrggbb`) — lipgloss gère le fallback automatiquement
7. Pour les formulaires, toujours utiliser `theme.NewForm()` (thème Aurum appliqué)
8. Pour les prompts simples inline, préférer `floating.Run()` à `form.Run()`
