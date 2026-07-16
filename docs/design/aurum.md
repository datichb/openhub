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

1. **Profondeur par le fond** — 3 niveaux de luminosité pour créer la hiérarchie (terminal → panel → element). Pas de séparateurs explicites.
2. **Espacement** — Toujours au moins 1 ligne vide entre les sections. Le vide crée la structure.
3. **Focus** — Un seul élément Copper (primaire) actif à la fois. Le regard est guidé.
4. **Floating** — Les panels "flottent" au-dessus du terminal grâce à un fond plus clair et des bordures quasi-invisibles.
5. **Sobriété** — Peu de couleurs simultanées. Les bordures sont des murmures, pas des cris.

## Palette

### Couleurs sémantiques

| Rôle | Nom | Hex | Usage |
|------|-----|-----|-------|
| **Primary** | Copper | `#e8a838` | Élément actif, label d'étape, bordure focus |
| **Accent** | Amethyst | `#a78bfa` | Highlights interactifs, sélection, curseur |
| **Success** | Jade | `#5fd787` | Confirmations, étapes complétées |
| **Warning** | Amber | `#ffaf5f` | Avertissements, priorité medium |
| **Error** | Ruby | `#ff5f5f` | Erreurs, bloqué, critique |
| **Info** | Sapphire | `#5fafff` | En cours, running, liens |

### Couleurs de texte

| Rôle | Nom | Hex | Usage |
|------|-----|-----|-------|
| **Text** | Ivory | `#e8e8e8` | Titres, texte principal |
| **Subtle** | Lavender | `#8585a0` | Footer, help text, keybinds |
| **Muted** | Ash | `#6e6e82` | Descriptions, metadata, timestamps |

### Couleurs de profondeur (3 niveaux)

| Niveau | Rôle | Hex | Usage |
|--------|------|-----|-------|
| 0 | Terminal | (hérité) | Fond du terminal de l'utilisateur |
| 1 | Panel (Surface) | voir variante | Fond du cadre extérieur (flotte sur le terminal) |
| 2 | Element (SurfaceElem) | voir variante | Zone active : step bar, formulaires, cards |

### Couleurs de bordure

| Rôle | Hex | Usage |
|------|-----|-------|
| Border (panel) | voir variante | Bordure extérieure quasi-invisible |
| BorderElem | voir variante | Bordure intérieure très subtile |
| BorderActive | `#e8a838` | = Primary (élément avec le focus) |

## Variantes de thème

### Mocha (actif)

Fond neutre avec soupçon de chaleur. Universel, s'adapte à tous les terminaux.

| Rôle | Hex | Description |
|------|-----|-------------|
| Surface | `#1e1e2e` | Panel bg |
| SurfaceElem | `#323248` | Element bg (surélevé) |
| Border | `#262636` | Bordure panel (quasi-invisible) |
| BorderElem | `#3c3c52` | Bordure element (très subtile) |

### Bleu Nuit (alternatif)

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
| Done | `●` | `●` | Jade |
| Active | `◔` | `►` | Copper |
| Pending | `○` | `○` | Ash |
| Skipped | `○` | `○` | Ash (dim) |

### Actions / Statuts

| Élément | Icône | Couleur |
|---------|-------|---------|
| Succès | `✓` | Jade |
| Erreur | `✗` | Ruby |
| Warning | `!` | Amber |
| Indicateur actif | `▸` | Copper |
| Point neutre | `·` | Ash |

### Connecteurs

| Type | Caractère | Couleur | Usage |
|------|-----------|---------|-------|
| Entre steps | `───` | Ash | Step bar horizontal |

## Bordures

| Propriété | Valeur |
|-----------|--------|
| Type | Rounded (`╭╮╯╰`) — outer ET inner |
| Couleur outer | Border (quasi-invisible) |
| Couleur inner | BorderElem (très subtile) |
| Couleur focus | BorderActive (= Copper) |

## Espacement

| Zone | Règle |
|------|-------|
| Entre sections d'un panel | 1 ligne vide |
| Padding panel interne | 1 char left/right minimum |
| Entre step bar et form | 1 ligne vide |
| Footer → bord inférieur | 1 ligne vide au-dessus et en-dessous |

## Composants

### Wizard (alt-screen, floating panels)

```
╭────────────────────────────────────────────────────────────╮  ← border quasi-invisible
│                                                            │  ← Surface (panel bg)
│   Titre du Wizard                                          │  ← Ivory bold
│                                                            │
│  ╭──────────────────────────────────────────────────────╮  │
│  │                                                      │  │  ← SurfaceElem + BorderElem
│  │  ● Label ─── ◔ Label ─── ○ Label ─── ○ Label        │  │  ← step bar
│  │                                                      │  │
│  │  N/M · Label de l'étape                              │  │  ← Copper bold
│  │                                                      │  │
│  │  [Contenu du formulaire — pleine largeur]            │  │
│  │                                                      │  │
│  ╰──────────────────────────────────────────────────────╯  │
│                                                            │
│   enter confirmer · esc passer · ctrl+c quitter            │  ← Lavender
│                                                            │
╰────────────────────────────────────────────────────────────╯
```

### Sidebar (inline, pour oh init)

```
  Titre                    ← Copper bold

  Étapes
  ● Label complété         ← Jade
  ◔ Label actif            ← Copper
  ○ Label à venir          ← Ash
```

Pas de bordure (rendu inline dans le terminal). Pas de fond (pas de contrôle background en mode inline).

### Title Bar (boards, dashboard)

- Background : Surface
- Foreground : Ivory
- Pas de bordure arrondie (barre, pas un panel)

### Cards / Kanban

- Fond : SurfaceElem
- Bordure : BorderElem (inactif) → BorderActive (sélectionné)
- Header de colonne : couleur sémantique (Amber=TODO, Sapphire=In Progress, Jade=Done, Ruby=Blocked)
- Contenu : Ivory
- Metadata : Ash

## Anti-patterns

- Ne **jamais** utiliser de séparateurs `━━━` ou `───` pour délimiter des zones — utiliser le changement de fond.
- Ne **jamais** utiliser Copper pour du texte long (illisible). Réservé aux labels courts, icônes, bordures focus.
- Ne **jamais** mélanger plus de 3 couleurs sémantiques dans la même zone visuelle.
- Ne **jamais** utiliser de couleurs hardcodées (`lipgloss.Color("99")`) — toujours via `common.*`.
- Ne **jamais** coller un formulaire directement sous un titre sans espacement.
- Les bordures sont des **murmures** : quasi-invisibles, même famille que le fond.

## Migration

Pour ajouter une couleur ou un composant au design system :
1. Ajouter la constante dans `cli/internal/tui/common/styles.go`
2. Documenter dans ce fichier (`docs/design/aurum.md`)
3. Ne jamais utiliser de `lipgloss.Color("...")` littéral dans les vues
4. Utiliser true color hex (`#rrggbb`) — lipgloss gère le fallback automatiquement
