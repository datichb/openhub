---
name: figma-ui-designer-protocol
description: Protocole d'intégration Figma pour l'agent UI Designer — lecture du design system existant, extraction des tokens de design, inventaire des composants, détection des incohérences visuelles. Charger via native_skills avant de spécifier des composants ou tokens pour un projet avec Figma.
bucket: B
---

# Skill — Figma UI Designer Protocol

## Rôle

Ce skill guide l'agent UI Designer dans l'exploitation des fichiers Figma pour :
- Lire le design system existant avant de spécifier
- Extraire les tokens de design (couleurs, typographie, spacing, élévations)
- Inventorier les composants disponibles
- Détecter les incohérences visuelles entre maquettes et design system

---

## Quand charger ce skill

Charger ce skill (via l'outil `skill`) dès qu'une des conditions suivantes est remplie :
- La demande porte sur des composants UI, tokens, ou un design system
- Un lien Figma ou un nom de fichier est fourni dans le prompt
- L'onboarder a détecté un design system Figma dans le projet

---

## Workflow d'exploration Figma

### Étape 1 — Rechercher le fichier design system

Stratégie progressive :

**Tentative 1 :** `search_figma_files("design system")` ou `search_figma_files(<nom du projet>)`

**Tentative 2 :** `search_figma_files("components")` ou `search_figma_files("tokens")`

**Si aucun résultat :**
```
question({
  questions: [{
    header: "Design system Figma",
    question: "[UI Designer | Projet : <nom>]\nAucun fichier design system Figma trouvé pour les termes : [terme1], [terme2].\nComment procéder ?",
    options: [
      { label: "Fournir l'URL ou le nom", description: "Préciser le fichier Figma du design system" },
      { label: "Pas de design system Figma", description: "Continuer sans Figma — baser la spec sur le code existant" }
    ]
  }]
})
```

### Étape 2 — Explorer la structure

1. `get_figma_file(<file_key>)` — obtenir la structure complète
2. Identifier les pages clés :
   - Page "Tokens" ou "Foundations" — couleurs, typographie, spacing
   - Page "Components" ou "UI Kit" — composants disponibles
   - Pages de maquettes — composants en contexte

### Étape 3 — Extraire le design system

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

### Étape 4 — Détecter les incohérences

Signaler activement :
- Composants qui utilisent des valeurs hardcodées plutôt que des tokens
- Variantes manquantes (ex : bouton sans état disabled)
- Incohérences cross-composants (spacing différent pour des éléments de même niveau)
- Composants dupliqués avec des designs légèrement différents

### Étape 5 — Intégrer dans la spec UI

Les données Figma deviennent la référence dans la spec :

- **Tokens** : "La couleur `color.primary.500` correspond à `#3B82F6` dans le design system Figma"
- **Composants existants** : "Utiliser le composant Button/Primary/Large existant — variante déjà définie en Figma"
- **Incohérences** : "Le composant Card utilise `8px` de border-radius dans certaines maquettes et `12px` dans d'autres — à normaliser"

---

## Règles

✅ Toujours lire le design system existant avant de spécifier de nouveaux tokens ou composants
✅ Réutiliser les tokens existants — ne pas créer de doublons
✅ Référencer les composants Figma par leur nom exact dans la spec
✅ Signaler toutes les incohérences détectées — même hors scope immédiat
❌ Ne pas proposer de nouveaux tokens sans avoir vérifié qu'ils n'existent pas déjà
❌ Ne pas ignorer les états manquants (disabled, error, empty) — les signaler systématiquement
