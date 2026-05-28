# Conventions Figma pour opencode-hub

Ce document définit les conventions d'organisation des fichiers Figma pour faciliter l'intégration avec les agents Scout et Planner.

## Nommage des fichiers

### Format recommandé

`[Projet] - [Feature] - [Type]`

**Exemples :**
- `MonApp - Tableau de bord - UI`
- `MonApp - Authentification - Flows`
- `MonApp - Design System`

### Mots-clés à privilégier

Pour faciliter la recherche par les agents, inclure des mots-clés explicites :
- **Nom de la feature** : Utiliser le même vocabulaire que dans les tickets Beads
- **Type de contenu** : UI, Flows, Wireframes, Prototype
- **État** : WIP, Ready, Archive

## Tags Figma

Utiliser les tags Figma pour catégoriser les fichiers :

| Tag | Usage |
|-----|-------|
| `#feature-[nom]` | Lier à une feature spécifique (ex: `#feature-dashboard`) |
| `#epic-[id]` | Lier à un epic Beads (ex: `#epic-bd-42`) |
| `#wip` | Work in progress — design non finalisé |
| `#ready-dev` | Prêt pour implémentation |
| `#archive` | Anciennes versions, ne plus utiliser |

## Organisation des pages dans un fichier

Pour faciliter l'analyse par les agents, organiser les pages de manière cohérente :

### Page 1 : Cover / Sommaire
- Vue d'ensemble du projet
- Index des features
- Statuts

### Page 2 : Flows (si applicable)
- Parcours utilisateur
- User flows
- Cas d'usage

### Page 3 : Wireframes (si applicable)
- Maquettes basse-fidélité
- Structure des pages

### Page 4 : UI Design
- Maquettes haute-fidélité
- Composants principaux
- Pages complètes

### Page 5 : States & Variants
- États visuels (hover, focus, disabled, error, loading)
- Variantes des composants
- Comportements interactifs

### Page 6 : Dev Notes
- Annotations techniques
- Spécifications fonctionnelles
- Notes pour les développeurs

## Nommage des frames

Utiliser des noms explicites et structurés :

### Format
`[Type] - [Nom] - [État]`

**Exemples :**
- `Page - Dashboard - Default`
- `Page - Dashboard - Loading`
- `Page - Dashboard - Error`
- `Component - Button - Primary`
- `Component - Button - Primary - Hover`
- `Flow - Inscription - Step 1`
- `Flow - Inscription - Step 2`

### Conventions
- **Numéroter les étapes** : Step 1, Step 2, Étape 1, Étape 2
- **Indiquer les états** : Default, Hover, Focus, Disabled, Error, Loading, Success
- **Grouper par fonctionnalité** : Utiliser des sections dans Figma

## Annotations développeur

### Dev Resources Figma

Utiliser les **Dev Resources** pour :
- Lier les frames aux tickets Beads (URL du ticket)
- Préciser les composants du design system à utiliser
- Documenter les états non visuels
- Lister les props attendues

**Exemple :**
```
Ticket Beads: https://beads.local/tickets/bd-123
Composant DSFR: DsfrButton (variante: primary)
Props: { label: string, onClick: () => void, disabled?: boolean }
États: default, hover, focus, disabled, loading
```

### Commentaires Figma

Utiliser les commentaires pour :
- Poser des questions aux designers
- Signaler des incohérences
- Tracer les décisions design

**Format recommandé :**
```
[Scout] Détection automatique :
- 5 composants identifiés
- Complexité estimée : M
- Signaux: UX ⚠️, UI ⚠️

[Planner] Ticket créé : bd-123
- Composant UserCard
- Estimation : 2h
```

## Design tokens

Publier les design tokens dans **Figma Variables** pour faciliter l'extraction par les agents.

### Structure recommandée

```
color/
  ├── primary
  ├── secondary
  ├── error
  └── success

text/
  ├── heading-1
  ├── heading-2
  ├── body
  └── caption

space/
  ├── xs
  ├── sm
  ├── md
  ├── lg
  └── xl

effect/
  ├── shadow-sm
  ├── shadow-md
  └── shadow-lg
```

### Nommage des variables
- **Préfixer par type** : `color/`, `text/`, `space/`, `effect/`
- **Utiliser des noms sémantiques** : `color/primary` plutôt que `color/blue-500`
- **Cohérence avec le code** : Utiliser les mêmes noms que dans le design system code

## Versions et historique

### Tagging des versions

Utiliser l'historique de versions Figma pour tracer les décisions importantes :

| Tag | Usage |
|-----|-------|
| `v1-ready-dev` | Version 1 prête pour dev |
| `v1-review` | Version 1 en review |
| `v2-wip` | Version 2 en cours |

### Documenter les changements

Dans la description de la version, noter :
- Quels composants ont changé
- Pourquoi (ticket Beads lié)
- Impact sur le code existant

## Composants du design system

### Organisation

Créer un fichier dédié : `[Projet] - Design System`

Pages :
1. **Tokens** : Couleurs, typographie, espacements
2. **Composants** : Boutons, inputs, cards, etc.
3. **Patterns** : Navigation, formulaires, tableaux
4. **Templates** : Pages types réutilisables

### Nommage des composants

Suivre la convention du design system en place (DSFR, Material, etc.)

**Exemples avec DSFR :**
- `DsfrButton`
- `DsfrInput`
- `DsfrModal`
- `DsfrCard`

**Composants custom :**
- Préfixer par le nom du projet : `MyAppUserCard`

### Variantes

Utiliser les variantes Figma pour les états :
- Type : primary, secondary, tertiary
- Size : sm, md, lg
- State : default, hover, focus, disabled
- Status : error, success, warning

## Bonnes pratiques

### Pour les designers

✅ **Nommer explicitement** tous les frames et composants
✅ **Utiliser les Dev Resources** pour lier aux tickets
✅ **Documenter les états** visuels (hover, focus, etc.)
✅ **Publier les tokens** dans Figma Variables
✅ **Tagger les versions** prêtes pour dev
✅ **Commenter** les décisions design importantes

❌ **Éviter les noms génériques** (Frame 1, Rectangle 2)
❌ **Ne pas mélanger** WIP et ready dans le même fichier
❌ **Ne pas oublier** de documenter les états non visuels (loading, error)

### Pour les développeurs

✅ **Consulter les Dev Resources** avant d'implémenter
✅ **Signaler** les incohérences via commentaires Figma
✅ **Lier les PRs** aux frames Figma concernés
✅ **Documenter** les divergences entre design et implémentation

❌ **Ne pas implémenter** des maquettes taggées `#wip`
❌ **Ne pas modifier** le design sans consulter l'équipe design

## Configuration pour les agents

Les agents Scout et Planner utilisent ces conventions pour :
- **Rechercher** les fichiers par nom de feature
- **Analyser** la structure des frames
- **Détecter** les signaux UX/UI automatiquement
- **Estimer** la complexité selon le nombre de composants

**Plus les conventions sont respectées, plus l'intégration est efficace.**

## Exemples complets

### Fichier : "MonApp - Dashboard - UI"

**Tags :** `#feature-dashboard`, `#ready-dev`

**Pages :**
1. Cover
2. Flows → Flow complet du dashboard (3 frames)
3. UI Design → Page Dashboard complète
4. States → Dashboard Loading, Dashboard Error, Dashboard Empty
5. Components → UserCard, MetricCard, ChartWidget

**Dev Resources :**
- Frame "Page - Dashboard - Default" → Ticket bd-456
- Frame "Component - UserCard" → Composant custom MyAppUserCard

### Fichier : "MonApp - Design System"

**Pages :**
1. Tokens → Couleurs, typo, espacements (Figma Variables)
2. Components → DsfrButton, DsfrInput, DsfrCard, etc.
3. Patterns → Navigation, Forms, Tables
4. Templates → Page vide, Page avec sidebar, Page modale

## Mise en place

### Pour un projet existant

1. **Auditer** les fichiers Figma actuels
2. **Renommer** selon les conventions (progressivement)
3. **Ajouter les tags** pertinents
4. **Organiser les pages** selon la structure recommandée
5. **Documenter** via Dev Resources et commentaires
6. **Publier** les design tokens dans Variables

### Pour un nouveau projet

1. **Créer le fichier Design System** en premier
2. **Définir les tokens** dans Figma Variables
3. **Suivre les conventions** dès le début
4. **Former l'équipe** aux conventions

---

**Dernière mise à jour :** v1.0 - Intégration initiale MCP Figma
