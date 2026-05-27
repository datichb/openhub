---
name: dev-standards-frontend-a11y
description: Règles d'accessibilité frontend — WCAG 2.1 A/AA, HTML sémantique, ARIA, navigation clavier, formulaires.
---

# Skill — Standards Accessibilité Frontend

## Rôle
Ce skill définit les règles d'accessibilité applicables à tout projet frontend.
Il complète `dev-standards-frontend.md`.

---

## Niveau de conformité

- **WCAG 2.1 Niveau A** — Obligatoire sur tous les projets, sans exception
- **WCAG 2.1 Niveau AA** — Obligatoire par défaut (obligation légale française — loi du 11 février 2005)

En l'absence de précision dans le projet, le niveau AA s'applique.
Si le projet déroge explicitement au niveau AA, l'utilisateur doit le préciser.

---

## HTML Sémantique

- Balises natives en priorité absolue : `nav`, `main`, `header`, `footer`,
  `article`, `section`, `aside`, `button`, `a`...
- Pas de `div` ou `span` là où une balise sémantique existe
- Hiérarchie des titres strictement respectée : `h1` → `h2` → `h3`
- Un seul `h1` par page
- `main` unique par page

---

## ARIA

- ARIA en dernier recours — le HTML natif et sémantique prime toujours
- `aria-label` : quand un élément interactif n'a pas de texte visible
- `aria-labelledby` : pour associer un titre existant à une région
- `aria-describedby` : pour lier une description complémentaire (ex: erreur de champ)
- `role` uniquement si aucune balise HTML native ne correspond
- Pas d'ARIA redondant avec le HTML natif (ex: `role="button"` sur un `<button>`)
- `aria-hidden="true"` sur les éléments purement décoratifs

---

## Navigation au clavier & Focus

- Tous les éléments interactifs sont atteignables au clavier
- Ordre de tabulation logique et cohérent avec l'ordre visuel
- Focus visible en permanence — `outline: none` interdit sans alternative visible
- Gestion du focus dans les composants complexes :
  - Modal/Dialog : focus piégé dans la modale à l'ouverture
  - Drawer : même règle
  - Retour du focus à l'élément déclencheur à la fermeture
- Touche `Escape` ferme les modales, drawers et menus déroulants

---

## Contrastes & Couleurs — Niveau A minimum

- L'information n'est jamais véhiculée par la couleur seule
- Un indicateur visuel complémentaire est toujours présent (icône, texte, motif)

**Ratios WCAG AA (appliqués si niveau AA validé sur le projet) :**
- Texte normal (< 18px) : ratio minimum `4.5:1`
- Texte large (≥ 18px ou ≥ 14px gras) : ratio minimum `3:1`
- Composants UI et états de focus : ratio minimum `3:1`

---

## Images & Médias

- `alt` descriptif sur toutes les images informatives
- `alt=""` sur les images purement décoratives
- Pas d'image de texte — sauf logo ou cas exceptionnel justifié
- Vidéos : sous-titres obligatoires
- Pas d'autoplay audio ou vidéo sans contrôle utilisateur explicite
- Animations : respecter `prefers-reduced-motion`

---

## Formulaires

- Chaque champ a un `<label>` associé via `for` / `id`
- Pas de placeholder comme substitut au label
- Messages d'erreur liés au champ via `aria-describedby`
- Les erreurs sont annoncées aux lecteurs d'écran (live region ou focus)
- Validation non uniquement visuelle — toujours un retour textuel
- Groupements logiques avec `<fieldset>` et `<legend>` (ex: boutons radio)
- Champs obligatoires indiqués visuellement ET via `aria-required="true"`

---

## Standards spécifiques aux frameworks

Les règles d'accessibilité spécifiques aux frameworks (Vue.js, React, Angular, etc.)
sont définies dans les skills correspondants du dossier `skills/developer/stacks/`.

---

## Mode Auditeur A11y

Déclenchement : `@dev-standards audit a11y`

Quand ce mode est actif :
1. Analyser le code fourni pour les problèmes d'accessibilité
2. Distinguer les violations niveau A (bloquant) des recommandations niveau AA
3. Présenter un rapport structuré par catégorie
4. Proposer des corrections — ne jamais les appliquer sans validation
