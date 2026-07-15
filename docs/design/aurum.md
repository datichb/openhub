# Aurum — Design System

Direction artistique du projet OpenHub (`oh`).

## Identité

| Attribut | Valeur |
|----------|--------|
| Nom | Aurum |
| Feeling | Premium & sophistiqué |
| Mots-clés | Chaleur, profondeur, précision, élégance |
| Inspiration | Dashboards financiers haut de gamme, horlogerie |

## Principes

1. **Profondeur** — Utiliser couleurs et bordures pour créer une hiérarchie visuelle claire.
2. **Espacement** — Toujours au moins 1 ligne vide entre les sections. Jamais de texte collé.
3. **Focus** — Un seul élément Gold (primaire) actif à la fois. Le regard est guidé.
4. **Cohérence** — Même pattern pour tous les wizards, boards, et dashboards.
5. **Sobriété** — Peu de couleurs simultanées. Le Gold est précieux car rare à l'écran.

## Palette

### Couleurs sémantiques

| Rôle | Nom | ANSI 256 | Hex approx. | Usage |
|------|-----|----------|-------------|-------|
| **Primary** | Gold | `178` | `#d7af00` | Titres, élément actif, bordure focus, séparateur principal |
| **Accent** | Amethyst | `99` | `#875fff` | Highlights interactifs, sélection, curseur |
| **Success** | Jade | `78` | `#5fd700` | Confirmations, étapes complétées, validations |
| **Warning** | Amber | `214` | `#ffaf00` | Avertissements, priorité medium |
| **Error** | Ruby | `196` | `#ff0000` | Erreurs, bloqué, critique |
| **Info** | Sapphire | `33` | `#0087ff` | En cours, running, liens |

### Couleurs structurelles

| Rôle | Nom | ANSI 256 | Hex approx. | Usage |
|------|-----|----------|-------------|-------|
| **Text** | Ivory | `255` | `#eeeeee` | Texte principal, texte sur fond coloré |
| **Muted** | Slate | `244` | `#808080` | Texte secondaire, descriptions, footers |
| **Surface** | Obsidian | `235` | `#262626` | Fond de title bars, panels surélevés |
| **Border** | Graphite | `240` | `#585858` | Bordures normales, séparateurs |
| **Border Active** | Gold | `178` | `#d7af00` | Bordure du panel/élément actif |

### Règles d'utilisation

- **Gold** : réservé à l'élément qui a le focus. Jamais plus d'un usage par "zone visuelle".
- **Amethyst** : pour le curseur dans les listes, la ligne sélectionnée.
- **Jade** : uniquement pour confirmer un succès (étape terminée, action réussie).
- **Slate** : texte d'aide, descriptions, footers — jamais pour du contenu principal.
- **Surface** : uniquement pour les title bars et backgrounds de sections surélevées.

## Iconographie

### Steps (wizards, progression)

| État | Icône | Fallback | Couleur |
|------|-------|----------|---------|
| Done | `●` | `●` | Jade |
| Active | `◔` | `►` | Gold |
| Pending | `○` | `○` | Slate |
| Skipped | `○` | `○` | Slate (dim) |

### Actions / Statuts

| Élément | Icône | Couleur |
|---------|-------|---------|
| Succès | `✓` | Jade |
| Erreur | `✗` | Ruby |
| Warning | `!` | Amber |
| Indicateur actif | `▸` | Gold |
| Point neutre | `·` | Slate |

### Séparateurs

| Type | Caractère | Couleur | Usage |
|------|-----------|---------|-------|
| Bold (principal) | `━` (U+2501) | Gold | Séparation header/body |
| Normal | `─` (U+2500) | Graphite | Séparation body/footer, connecteurs |
| Connecteur steps | `───` | Graphite | Entre les étapes dans la step bar |

## Bordures

| Propriété | Valeur |
|-----------|--------|
| Type | Rounded (`╭╮╯╰`) |
| Couleur normale | Graphite (`240`) |
| Couleur active/focus | Gold (`178`) |
| Padding interne | `(0, 1)` — 0 vertical, 1 horizontal |

## Espacement

| Zone | Règle |
|------|-------|
| Entre sections d'un panel | 1 ligne vide |
| Padding panel interne | 1 char left/right minimum |
| Avant/après séparateur bold | 1 ligne vide |
| Entre items d'une liste | 0 (pas d'espace entre les lignes) |
| Footer → bord inférieur | 0 (collé au bord) |

## Composants

### Wizard (alt-screen, full width)

```
╭──────────────────────────────────────────────────────────────╮
│                                                              │
│   Titre du Wizard                                            │
│                                                              │
│   ● Label 1 ─── ◔ Label 2 ─── ○ Label 3 ─── ○ Label 4      │
│                                                              │
│━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━│
│                                                              │
│   2/4 · Label de l'étape active                              │
│                                                              │
│   [Contenu du formulaire huh — pleine largeur]               │
│                                                              │
│━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━│
│   enter confirmer · esc passer · ctrl+c quitter              │
╰──────────────────────────────────────────────────────────────╯
```

- Bordure : Graphite (Gold si wizard a le focus unique)
- Titre : Bold + Gold
- Step bar : `●` Jade / `◔` Gold / `○` Slate + connecteurs `───` Graphite
- Séparateur haut : `━━━` Gold
- Label d'étape : `2/4 · Nom` Bold + Gold
- Form : couleurs par défaut de huh (inherit du terminal)
- Séparateur bas : `━━━` Graphite (subtil)
- Footer : Slate

### Sidebar (inline, pour oh init)

```
  Titre

  Étapes
  ● Label complété
  ◔ Label actif
  ○ Label à venir
  ○ Label à venir
```

- Titre : Bold + Gold
- `●` Jade, `◔` Gold, `○` Slate
- Pas de bordure (rendu inline dans le terminal)

### Title Bar (boards, dashboard)

```
┌──────────────────────────────────────────────────────────────┐
│  Titre de la vue                                   Info      │
└──────────────────────────────────────────────────────────────┘
```

- Background : Surface (235)
- Foreground : Ivory (255)
- Pas de bordure arrondie (c'est une barre, pas un panel)

### Cards / Kanban

- Bordure : Graphite (inactif) → Gold (sélectionné)
- Header de colonne : couleur sémantique (Amber=TODO, Sapphire=In Progress, Jade=Done, Ruby=Blocked)
- Contenu : Ivory
- Metadata (dates, branches) : Slate

## Anti-patterns

- Ne **jamais** utiliser Gold pour du texte long (illisible sur fond sombre).
- Ne **jamais** mélanger plus de 3 couleurs dans la même zone visuelle.
- Ne **jamais** utiliser de couleurs hardcodées (`lipgloss.Color("99")`) — toujours via `common.*`.
- Ne **jamais** coller un formulaire directement sous un titre sans espacement.
- Ne **jamais** utiliser Jade pour autre chose qu'un succès/complétion.

## Migration

Pour ajouter une couleur ou un composant au design system :
1. Ajouter la constante dans `cli/internal/tui/common/styles.go`
2. Documenter dans ce fichier (`docs/design/aurum.md`)
3. Ne jamais utiliser de `lipgloss.Color("...")` littéral dans les vues
