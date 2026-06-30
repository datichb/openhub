---
name: figma-deep-protocol
description: Protocole d'exploration Figma approfondie pour l'agent Designer — modes ux, ui et ux+ui. Couvre l'extraction UX (flows, frictions, états manquants) et UI (tokens complets, inventaire composants, incohérences). Charger via native_skills quand des fichiers Figma sont disponibles pour une spec.
bucket: B
---

# Skill — Figma Deep Protocol

## Rôle

Ce skill guide l'exploration Figma approfondie pour la production de specs UX et UI. Il fusionne les protocoles UX et UI en un seul skill selon le mode actif.

---

## Quand charger ce skill

Charger ce skill (via l'outil `skill`) dès qu'une des conditions suivantes est remplie :
- La demande mentionne des écrans, maquettes, ou composants UI existants
- Un lien Figma ou un nom de fichier Figma est fourni dans le prompt
- L'onboarder a détecté Figma dans le projet
- Le mode est `ux`, `ui`, ou `ux+ui` et des fichiers Figma existent

---

## Étape commune — Recherche et accès au fichier

### Recherche progressive

**Tentative 1 :** `search_figma_files(<nom du projet ou de la feature>)`

**Tentative 2 (si aucun résultat) :** `search_figma_files(<terme alternatif — nom du ticket, composant, ou feature>)`

**Si aucun résultat :**
```
question({
  questions: [{
    header: "Fichiers Figma",
    question: "[Designer | Feature : <nom>]\nAucun fichier Figma trouvé pour les termes : [terme1], [terme2].\nComment procéder ?",
    options: [
      { label: "Fournir l'URL ou le nom exact", description: "Préciser le fichier Figma à analyser" },
      { label: "Pas de maquettes Figma", description: "Continuer sans données Figma" }
    ]
  }]
})
```

### Exploration de la structure

1. `get_figma_file(<file_key>)` — obtenir la structure complète du fichier
2. Identifier les pages pertinentes selon le mode actif (voir sections ci-dessous)

---

## Section UX — Extraction pour mode ux

### Pages pertinentes

Identifier et prioriser :
- Pages "Flows", "User Journey", "Wireframes", "Parcours"
- Pages de feature spécifique (ex : "Inscription", "Panier", "Dashboard")

### Extraction UX

Pour chaque page pertinente : `get_figma_file_nodes(<file_key>, <node_ids>)`

| Information | Comment l'extraire |
|-------------|-------------------|
| **User flows existants** | Lire la séquence des frames dans chaque page |
| **Points de friction visibles** | Chercher les états d'erreur, loaders, messages vides |
| **Navigation** | Identifier les liens entre frames, back buttons, modals |
| **Formulaires** | Lister les champs, labels, messages de validation présents |
| **États manquants** | Comparer l'état "happy path" avec les états d'erreur / vide / chargement |

### Intégration dans la spec UX

Les observations Figma enrichissent directement la spec UX :

- **Contexte existant** : "Les maquettes Figma montrent X — la spec s'appuie sur cette base"
- **Frictions détectées** : "L'écran de confirmation (frame Y) ne gère pas l'état d'erreur réseau"
- **Incohérences** : "Le flow Figma saute l'étape de validation — à clarifier avec le Product Owner"

### Règles UX

✅ Toujours lire les maquettes avant de spécifier pour une feature qui en a
✅ Signaler explicitement les frictions détectées dans Figma
✅ Référencer les frames Figma dans la spec (ex : "Frame 'Confirmation — Error state'")
❌ Ne pas reproduire les frictions existantes dans la spec — les corriger
❌ Ne pas inventer des flows non présents dans Figma sans le préciser

---

## Section UI — Extraction pour mode ui

### Pages pertinentes

Identifier et prioriser :
- Page "Design System", "Tokens", "Foundations" — couleurs, typographie, spacing
- Page "Components", "UI Kit", "Library" — composants disponibles
- Pages de maquettes — composants en contexte

### Extraction UI

Pour chaque page pertinente : `get_figma_file_nodes(<file_key>, <node_ids>)`

#### Tokens à extraire

| Catégorie | Ce qu'on cherche |
|-----------|-----------------|
| **Couleurs** | Palettes, couleurs sémantiques (primary, error, success, warning), niveaux de surface |
| **Typographie** | Font families, tailles, weights, line heights par niveau hiérarchique |
| **Spacing** | Échelle de spacing (4px, 8px, 16px...) et usage sémantique |
| **Border radius** | Valeurs par taille de composant |
| **Ombres** | Niveaux d'élévation |

#### Inventaire des composants

Pour chaque composant identifié, documenter :
- Nom et variantes disponibles (taille, état, couleur)
- États couverts (default, hover, focus, disabled, error, loading)
- Props attendues (implicites depuis Figma)

### Détection des incohérences

Signaler activement :
- Composants qui utilisent des valeurs hardcodées plutôt que des tokens
- Variantes manquantes (ex : bouton sans état disabled)
- Incohérences cross-composants (spacing différent pour des éléments de même niveau)
- Composants dupliqués avec des designs légèrement différents

### Intégration dans la spec UI

Les données Figma deviennent la référence dans la spec :

- **Tokens** : "La couleur `color.primary.500` correspond à `#3B82F6` dans le design system Figma"
- **Composants existants** : "Utiliser le composant Button/Primary/Large existant — variante déjà définie en Figma"
- **Incohérences** : "Le composant Card utilise `8px` de border-radius dans certaines maquettes et `12px` dans d'autres — à normaliser"

### Règles UI

✅ Toujours lire le design system existant avant de spécifier de nouveaux tokens ou composants
✅ Réutiliser les tokens existants — ne pas créer de doublons
✅ Référencer les composants Figma par leur nom exact dans la spec
✅ Signaler toutes les incohérences détectées — même hors scope immédiat
❌ Ne pas proposer de nouveaux tokens sans avoir vérifié qu'ils n'existent pas déjà
❌ Ne pas ignorer les états manquants (disabled, error, empty) — les signaler systématiquement
