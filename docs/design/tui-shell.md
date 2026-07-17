# TUI Shell — Design Specification

> Référence design pour le shell TUI unifié d'OpenHub.
> Palette, dimensions, états visuels, composants.

## Palette unifiée (Catppuccin Mocha)

> Palette basée sur [Catppuccin Mocha](https://github.com/catppuccin/catppuccin) — éprouvée sur le fond `#1e1e2e`, teinte bleu-lavande pour une lisibilité optimale sur fond sombre.

### Backgrounds (3 niveaux de profondeur)

| Rôle | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| App | `#181825` | Mantle | Fond terminal, status bar |
| Panel | `#1e1e2e` | Base | Sidebar, cards, content panels |
| Element | `#313244` | Surface0 | Items hover, sélection, header |

### Texte (3 niveaux de contraste)

| Rôle | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| Primary | `#cdd6f4` | Text | Titres, **items de menu**, contenu principal |
| Secondary | `#a6adc8` | Subtext0 | Catégories de menu, descriptions, labels secondaires |
| Muted | `#7f849c` | Overlay1 | Placeholders, disabled, timestamps |

### Couleurs sémantiques

| Rôle | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| Accent | `#89b4fa` | Blue | Bordures focus, navigation, headers |
| Action | `#fab387` | Peach | CTA, menu actif, spinner, boutons |
| Success | `#a6e3a1` | Green | Confirmations, done |
| Warning | `#f9e2af` | Yellow | Alertes non-bloquantes |
| Error | `#f38ba8` | Red | Erreurs, blocked, critical |
| Info | `#b4befe` | Lavender | In progress, running |

### Bordures

| Rôle | Hex | Usage |
|------|-----|-------|
| Normal | `#313244` | Délimiteurs de panels (= Surface0) |
| Focus | `#89b4fa` | Panel en focus (= Blue) |
| Active | `#fab387` | Item de menu actif (= Peach) |

## Layout principal

```
┌─────────────────────────────────────────────────────────────────┐
│ [Left: logo]        [Center: breadcrumb]       [Right: meta]    │ ← Header (2 rows, bg: Elevated)
├──────────────┬──────────────────────────────────────────────────┤
│              │                                                   │
│    MENU      │              CONTENT PANEL                        │ ← Middle (flex, bg: Surface)
│   (1 part)   │               (5 parts)                          │
│              │                                                   │
├──────────────┴──────────────────────────────────────────────────┤
│ [Left: vue]       [Center: hints]                [Right: info]  │ ← StatusBar (1 row, bg: Abyss)
└─────────────────────────────────────────────────────────────────┘
```

### Dimensions

| Composant | Taille | Type |
|-----------|--------|------|
| Header | 2 rows | Fixe |
| Status bar | 1 row | Fixe |
| Sidebar (menu) | proportion 1 | Flex |
| Content | proportion 5 | Flex |
| Sidebar min width | 20 chars | — |
| Content padding | top=1, left=2, right=2 | — |

### Responsive breakpoints

| Terminal width | Comportement |
|---------------|--------------|
| >= 120 cols | Layout complet |
| 100-119 cols | Labels menu tronqués (15 chars max) |
| 80-99 cols | Menu masqué par défaut, Ctrl+N overlay |
| < 80 cols | Menu caché, breadcrumb caché, hints minimales |

## Menu (sidebar)

### Rendu

```
│                        │
│  ◆ Home                │  ← Vue active : texte Gold, bold
│                        │
│  Sessions              │  ← Catégorie expanded : Snow, bold
│    · Start             │  ← Item : Lavender, indent 4
│    · Quick             │
│    · Audit             │
│                        │  ← Ligne vide entre catégories
│  Projets               │
│  ▸ Liste               │  ← Item sous le curseur : bg Elevated, texte Snow
│    Config              │
│                        │
│  ▾ Team                │  ← Catégorie collapsed : Lavender
│  ▾ MCP                 │
│  ▾ Configuration       │
│  ▾ Système             │
│                        │
```

### États visuels

| État | Rendu | Couleur |
|------|-------|---------|
| Catégorie expanded | `Sessions` | Snow, bold |
| Catégorie collapsed | `▾ Team` | Lavender |
| Item normal | `  · Start` | Lavender |
| Item curseur (sélectionné) | `  · Start` bg Elevated | Snow, bg Elevated |
| Item = vue active | `◆ Home` | Gold, bold |
| Item disabled | `  · Deploy` | Ash |

### Interactions

| Touche | Action |
|--------|--------|
| `j` / `↓` | Item suivant |
| `k` / `↑` | Item précédent |
| `Enter` | Ouvrir vue / exécuter action |
| `l` / `→` | Expand catégorie |
| `h` / `←` | Collapse / remonter au parent |
| `Space` | Toggle expand/collapse |
| `Ctrl+N` | Toggle focus sidebar ↔ content |
| `Esc` | Retour focus content |
| `g` | Premier item |
| `G` | Dernier item |

## Header

### 3 sections (Flex horizontal)

| Section | Proportion | Alignement | Contenu |
|---------|-----------|------------|---------|
| Left | 1 | Gauche | `◆ OpenHub` (Gold diamond + Snow bold text) |
| Center | 2 | Centre | Breadcrumb : `[Ash]Sessions[-] [Ash]▸[-] [Azure]Start[-]` |
| Right | 1 | Droite | Méta-info : `[Ash]3 projets[-]` |

### Breadcrumb format

- Segments ancêtres : couleur Ash
- Séparateur : `▸` couleur Ash
- Segment actif (dernier) : couleur Azure, bold
- Exemple : `Projets ▸ my-app ▸ Configure`

## Status bar

### 3 sections (Flex horizontal)

| Section | Proportion | Alignement | Contenu |
|---------|-----------|------------|---------|
| Left | 1 | Gauche | `[Azure]◆[-] [Lavender]board[-]` (vue courante) |
| Center | 3 | Centre | `[Lavender]j/k nav  Enter ouvrir  Esc retour[-]` |
| Right | 1 | Droite | `[Ash]14:32[-]` ou info contextuelle |

### Feedback flash

Après une action réussie, la section Left passe temporairement en Jade (1.5s) :
`[Jade]✓ Déployé[-]` puis revient à l'état normal.

## Toast notifications

### Position et dimensions

- Position : haut-droite du content panel (via Grid overlay dans Pages)
- Largeur : auto (message + 4 padding)
- Hauteur : 3 rows (border + text + border)
- Durée : 2.5 secondes

### Types

| Type | Border color | Icône | Exemple |
|------|-------------|-------|---------|
| success | Jade | ✓ | `✓ Projet ajouté` |
| error | Ruby | ✗ | `✗ Échec du deploy` |
| warning | Amber | ! | `! Opencode non trouvé` |
| info | Azure | ▸ | `▸ Sync en cours...` |

### Comportement

- N'intercepte pas le focus (les inputs vont au content)
- Disparaît après timeout via `time.AfterFunc` + `RemovePage`
- Si plusieurs toasts : empiler verticalement (max 3)

## Modal de confirmation

### Layout

```
Pages overlay (resize=true, centered via Grid 3x3) :
┌───────────────────────────────────────────────────┐
│                                                    │
│       ┌─── Titre ────────────────────────┐        │
│       │                                   │        │
│       │  Message explicatif.              │        │
│       │  Détails supplémentaires.         │        │
│       │                                   │        │
│       │   [Annuler]       [Confirmer]     │        │
│       └───────────────────────────────────┘        │
│                                                    │
└───────────────────────────────────────────────────┘
```

### Styles

- Bordure modal : Azure (ou Ruby si action destructrice)
- Bouton "Annuler" : bg Elevated, texte Lavender
- Bouton "Confirmer" : bg Azure (ou Ruby si destructeur), texte Snow
- Fond derrière : le content normal reste visible

### Interactions

- `Tab` : bascule entre boutons
- `Enter` : valide le bouton sélectionné
- `Esc` : annule (= bouton Annuler)

## Icônes (canoniques, dedupliqués)

| Constante | Caractère | Usage |
|-----------|-----------|-------|
| `IconActive` | `◆` | Vue active dans le menu, spinner |
| `IconDone` | `●` | Étape complétée |
| `IconPending` | `○` | Étape future |
| `IconSuccess` | `✓` | Succès, sélection |
| `IconError` | `✗` | Erreur |
| `IconWarning` | `!` | Warning |
| `IconArrow` | `▸` | Pointeur, catégorie expanded |
| `IconCollapsed` | `▾` | Catégorie collapsed |
| `IconDot` | `·` | Bullet item de menu |
| `IconConnector` | `───` | Séparateur step bar |
| `IconGutter` | `▎` | Gutter active item (Gold) |

## Principes de design

1. **Profondeur par background** — 3 niveaux créent la hiérarchie sans séparateurs explicites
2. **Espacement** — 1 ligne vide entre sections, padding 2 sur les côtés
3. **Focus unique** — Un seul élément actif à la fois, visuellement évident
4. **Sobriété** — Max 3 couleurs sémantiques par zone visuelle
5. **Feedback immédiat** — Chaque action produit un retour visuel (toast, flash, état)
6. **Navigation prédictible** — Esc=retour, Enter=action, q=quitter
7. **Accessible sans documentation** — Raccourcis visibles dans la status bar
