---
name: websearch-design-patterns
description: Protocole de recherche web pour les agents design — patterns UI/UX, composants, accessibilité, design systems de référence.
---

# Recherche Design Patterns via websearch

## Quand chercher

Déclencher une recherche web design quand :

1. **Conception de composant complexe** — datepicker, modal, formulaire multi-étapes, navigation imbriquée
2. **Problème UX inconnu** — pas de pattern évident pour le besoin utilisateur
3. **Vérification accessibilité** — conformité WCAG 2.2, support lecteur d'écran, navigation clavier
4. **Tendances visuelles** — langage design actuel, dark mode, design tokens
5. **Responsive / mobile-first** — patterns de navigation mobile, breakpoints, zones d'accessibilité tactile
6. **Conflit de sources** — guidance contradictoire nécessitant des données récentes ou quantitatives

## Sources de référence par domaine

### Design systems & composants

| Source | Spécialité |
|--------|-----------|
| Material Design 3 (m3.material.io) | Système complet Google, tokens, composants |
| Ant Design (ant.design) | Bibliothèque composants enterprise |
| Radix UI (radix-ui.com) | Primitives non-stylées, accessibles |
| shadcn/ui (ui.shadcn.com) | Composants Tailwind + Radix |
| Chakra UI (chakra-ui.com) | Composants React accessibles |
| Component Gallery (component.gallery) | Exemples multi-systèmes |

### Accessibilité

| Source | Spécialité |
|--------|-----------|
| WCAG 2.2 Quick Reference (w3.org/WAI/WCAG22/quickref) | Standards officiels |
| WebAIM (webaim.org) | Outils et guides pratiques |
| Inclusive Components (inclusive-components.design) | Patterns accessibles détaillés |
| A11y Project (a11yproject.com) | Checklist et ressources communautaires |

### UX patterns & research

| Source | Spécialité |
|--------|-----------|
| Nielsen Norman Group (nngroup.com) | Recherche UX empirique, études utilisateurs |
| Baymard Institute (baymard.com) | UX e-commerce, études quantitatives |
| Smashing Magazine (smashingmagazine.com) | Articles techniques design/front |
| A List Apart (alistapart.com) | Standards web, accessibilité |

### Inspiration visuelle

| Source | Spécialité |
|--------|-----------|
| Dribbble (dribbble.com) | Tendances visuelles (⚠️ pas de validation UX) |
| Behance (behance.net) | Études de cas complètes |
| Awwwards (awwwards.com) | Sites primés, innovation |
| Mobbin (mobbin.com) | Patterns apps mobiles réelles |

## Stratégie de recherche

### Principes généraux

- **Rechercher avant de concevoir** — ne pas réinventer un pattern existant
- **Prioriser les sources avec données** — études A/B, recherche utilisateur > opinions > tendances Dribbble
- **Vérifier la date** — le design UX évolue ; privilégier les sources < 2 ans
- **Croiser les sources** — minimum 2 sources concordantes avant d'adopter un pattern
- **Séparer mobile et desktop** — les patterns diffèrent, rechercher spécifiquement

### Catégories de recherche

| Type | Objectif | Sources prioritaires |
|------|----------|---------------------|
| Component patterns | Structure, variantes, états | Material, Ant, Radix |
| Interaction patterns | Flows, feedback, transitions | NNG, Baymard |
| Accessibility patterns | WCAG, ARIA, clavier, lecteur d'écran | W3C, WebAIM, Inclusive Components |
| Visual patterns | Tendances, inspiration, branding | Dribbble, Awwwards, Behance |
| Responsive patterns | Breakpoints, adaptation, touch | NNG, Material, Mobbin |

### Gestion des conflits

Quand les sources se contredisent :
1. Vérifier les dates de publication (récent > ancien)
2. Privilégier la méthodologie (A/B test > opinion d'expert > article blog)
3. Considérer le contexte (site contenu ≠ app métier ≠ e-commerce)
4. En cas de doute persistant → recommander un test utilisateur

## Critères d'évaluation

Tout pattern retenu doit être évalué sur :

| Critère | Standard minimum |
|---------|-----------------|
| **Accessibilité** | WCAG 2.2 AA — contraste 4.5:1 texte, 3:1 UI, navigation clavier, compatible lecteur d'écran |
| **Responsiveness** | Fonctionne sur mobile (320px), tablet (768px), desktop (1024px+) |
| **Performance** | Pas d'animation bloquant le main thread, images optimisées, lazy loading |
| **Cohérence design system** | Constructible avec les tokens existants, aligné avec les composants du projet |
| **Evidence** | Supporté par au moins une source fiable (recherche ou design system majeur) |

## Format de synthèse attendu

Après recherche, produire une recommandation structurée :

```markdown
## Recommandation Design : {Composant/Pattern}

**Problème** : {Description du défi UX/UI}
**Pattern retenu** : {Nom du pattern}

### Justification
- {Finding 1 avec source} (ex: NNG — réduction de 35% du temps de tâche)
- {Finding 2 avec source}
- {Finding 3 avec source}

### Spécifications accessibilité
- {Exigence WCAG applicable}
- {Pattern ARIA requis}
- {Comportement clavier attendu}

### Variantes responsive
- Mobile : {Adaptation}
- Desktop : {Adaptation}

### Références
1. {Source 1} — {URL}
2. {Source 2} — {URL}

### Notes d'implémentation
- Composant : {nom fichier}
- Tokens utilisés : {liste}
- Dépendances : {ou "aucune"}
```

## Checklist pré-validation

Avant de finaliser un design :

- [ ] Pattern recherché dans les sources autoritatives
- [ ] Conformité WCAG 2.2 AA vérifiée
- [ ] Comportement responsive validé (mobile/tablet/desktop)
- [ ] Navigation clavier et lecteur d'écran considérés
- [ ] Performance évaluée (animations, images, embeds)
- [ ] Alignement design system confirmé
- [ ] Rationale et sources documentés
- [ ] Notes d'implémentation incluses pour les développeurs
