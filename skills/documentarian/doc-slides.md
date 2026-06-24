---
name: doc-slides
description: Protocole de génération de présentations Marp — format source Markdown, structure par type de présentation, bonnes pratiques de slides, détection et compilation HTML/PDF.
---

# Skill — Présentations Marp

## Rôle

Tu génères des présentations au format **Marp** (Markdown Presentation Ecosystem).
Le fichier source est un fichier `.md` standard avec un frontmatter spécial.
Tu explores toujours avant de créer — s'il existe déjà des slides dans le projet,
tu t'adaptes à leur format et leur emplacement.

---

## Règles absolues

❌ Tu ne génères JAMAIS un fichier de slides sans avoir exploré l'existant au préalable
❌ Tu ne compiles JAMAIS en HTML/PDF sans confirmation explicite de l'utilisateur
❌ Tu ne choisis JAMAIS le thème ou la structure sans avoir posé les questions de contexte minimales
✅ Explorer → branding entreprise → proposer → confirmer → écrire → proposer la compilation
✅ Si des slides existent déjà dans le projet, s'adapter à leur format et emplacement
✅ Le fichier `.md` Marp est toujours livrable, même sans compilation

---

## Étape -1 — Branding entreprise (avant tout)

**Avant** l'exploration et le choix du thème, vérifier si un template d'entreprise est disponible.

### Détection du MCP Google Slides

Tenter d'appeler l'outil `get_template_branding` (disponible si le MCP `gslides-mcp` est configuré).

```
// Priorité 1 : GOOGLE_SLIDES_TEMPLATE_ID est injecté automatiquement
//              dans l'environnement si configuré pour ce projet (oc gslides setup --project <id>)
// Priorité 2 : demander l'ID à l'utilisateur si le MCP est disponible mais sans template par défaut
```

### Workflow branding

```
SI l'outil get_template_branding est accessible :
  templateId = process.env.GOOGLE_SLIDES_TEMPLATE_ID  // injecté par opencode depuis la config projet
  SI templateId défini :
    → Appeler get_template_branding({ presentationId: templateId })
    → Stocker le champ cssTheme retourné
    → Stocker le champ marpFrontmatter retourné (prêt à l'emploi)
  SINON :
    → Appeler list_presentations() pour afficher les templates accessibles
    → Demander à l'utilisateur quel template utiliser via question()
    → Appeler get_template_branding({ presentationId: <ID choisi> })
SINON (MCP non configuré) :
  → Passer à l'Étape 0 (comportement standard avec thèmes Marp natifs)
```

### Injection du branding dans le frontmatter

Quand un branding est disponible, utiliser le `marpFrontmatter` retourné par `get_template_branding`
**à la place** du frontmatter standard (ne pas utiliser `theme: default|gaia|uncover`) :

```markdown
---
marp: true
paginate: true
style: |
  section {
    background: #1a1a2e;
    color: #ffffff;
    font-family: 'Montserrat', Arial, sans-serif;
  }
  h1, h2, h3 {
    color: #e94560;
  }
  a {
    color: #e94560;
  }
  section.lead {
    background: #e94560;
    color: #1a1a2e;
  }
  section.lead h1 {
    color: #1a1a2e;
  }
---
```

> **Note** : le champ `theme` est omis intentionnellement — Marp applique le CSS directement.
> Le `marpFrontmatter` retourné par `get_template_branding` contient déjà le bloc complet.

### Signaler le branding utilisé

Indiquer à l'utilisateur le template détecté :
```
✓ Branding appliqué : "[templateName]" (ID: [presentationId])
  Couleurs : fond [backgroundColor], accent [accentColor], texte [textColor]
  Police : [fontFamily]
```

---

## Étape 0 — Exploration obligatoire

Avant de générer, explorer :

```bash
# Slides existants
find . -name "*.md" -path "*/presentations/*" 2>/dev/null | head -10
find . -name "*.md" -path "*/slides/*" 2>/dev/null | head -10
ls docs/presentations/ 2>/dev/null || echo "Pas de docs/presentations/"

# Vérifier si un thème Marp custom est déjà défini
find . -name "*.css" -path "*marp*" -o -name "*.css" -path "*theme*" 2>/dev/null | head -5
find . -name ".marprc*" -o -name "marp.config.*" 2>/dev/null | head -3
```

Lire un fichier de slides existant s'il en existe un pour détecter :
- Le thème utilisé (`theme: default | gaia | uncover | custom`)
- La structure type (nombre de slides, patterns de contenu)
- L'emplacement standard dans le projet

---

## Questions de contexte

Si la demande ne précise pas ces éléments, les demander via l'outil `question` :

```
question({
  questions: [{
    header: "Contexte de la présentation",
    question: "Quelques précisions pour structurer la présentation :",
    options: [
      { label: "Démo technique / feature", description: "Présenter une fonctionnalité ou un système à une équipe technique" },
      { label: "Product pitch / stakeholders", description: "Présenter un produit, une roadmap ou des résultats à des décideurs" },
      { label: "Retrospective / bilan", description: "Synthèse d'une sprint, d'un projet ou d'un incident" },
      { label: "Onboarding / formation", description: "Initier une équipe ou un nouveau membre à un sujet" }
    ]
  }]
})
```

Si l'audience, le nombre de slides souhaité ou un thème particulier ne sont pas précisés,
utiliser les valeurs par défaut des templates ci-dessous et le signaler.

---

## Format source Marp

### Frontmatter de base

```markdown
---
marp: true
theme: default
paginate: true
---
```

### Directives globales (dans le frontmatter)

| Directive | Valeurs | Effet |
|-----------|---------|-------|
| `theme` | `default`, `gaia`, `uncover` | Thème visuel |
| `paginate` | `true` / `false` | Numéros de page |
| `backgroundColor` | couleur CSS | Couleur de fond global |
| `color` | couleur CSS | Couleur de texte global |
| `size` | `16:9`, `4:3` | Format de slide (défaut : 16:9) |

### Directives locales (dans une slide spécifique)

```markdown
<!-- _class: lead -->       ← classe CSS sur cette slide uniquement
<!-- _backgroundColor: #1a1a2e -->
<!-- _paginate: false -->   ← désactiver la pagination sur cette slide
```

### Séparateurs

```markdown
---          ← nouvelle slide
```

### Incrément (build step)

```markdown
- Item visible
- Item visible

<!--
- Item caché (affiché à l'étape suivante en mode présentation)
-->
```

---

## Templates par type de présentation

### Tech demo / feature

```markdown
---
marp: true
theme: default
paginate: true
---

<!-- _class: lead -->
<!-- _paginate: false -->

# [Titre de la feature]

**[Équipe / Auteur]** · [Date]

---

## Contexte

> [Problème ou besoin initial en 1-2 phrases]

- [Contrainte ou enjeu clé 1]
- [Contrainte ou enjeu clé 2]

---

## Solution

[Description en 1 phrase]

```[langage]
// Exemple de code représentatif (< 15 lignes)
```

---

## Architecture

[Schéma ou liste structurée — éviter les phrases]

```
[Composant A] → [Composant B] → [Composant C]
```

---

## Démo

> [Ce qu'on va montrer]

1. [Étape 1]
2. [Étape 2]
3. [Résultat attendu]

---

## Résultats / Métriques

| Avant | Après |
|-------|-------|
| [valeur] | [valeur] |

---

<!-- _class: lead -->

## Next steps

- [ ] [Action 1]
- [ ] [Action 2]

**Questions ?**
```

---

### Product pitch / stakeholders

```markdown
---
marp: true
theme: gaia
paginate: true
---

<!-- _class: lead -->
<!-- _paginate: false -->

# [Nom du produit / feature]

[Tagline en 1 phrase]

**[Auteur]** · [Date]

---

## Problème

> [Douleur utilisateur en 1 phrase]

[2-3 bullets factuels avec données si possible]

---

## Solution

[Description orientée valeur — pas technique]

---

## Impact

| Indicateur | Valeur |
|------------|--------|
| [KPI 1] | [Valeur] |
| [KPI 2] | [Valeur] |

---

## Roadmap

| Phase | Périmètre | Timing |
|-------|-----------|--------|
| Phase 1 | [scope] | [date] |
| Phase 2 | [scope] | [date] |

---

<!-- _class: lead -->

## Décision demandée

> [Ce qu'on attend des participants]

**Questions ?**
```

---

### Retrospective / bilan

```markdown
---
marp: true
theme: default
paginate: true
---

<!-- _class: lead -->
<!-- _paginate: false -->

# Retrospective — [Sprint / Projet]

[Période] · [Équipe]

---

## Ce qui a bien fonctionné

- [Point positif 1]
- [Point positif 2]
- [Point positif 3]

---

## Ce qui peut s'améliorer

- [Point d'amélioration 1]
- [Point d'amélioration 2]

---

## Actions décidées

| Action | Responsable | Échéance |
|--------|-------------|----------|
| [action] | [qui] | [quand] |

---

## Métriques du sprint

| Indicateur | Valeur |
|------------|--------|
| Vélocité | [pts] |
| Bugs ouverts | [n] |
| Taux de complétion | [%] |
```

---

### Onboarding / formation

```markdown
---
marp: true
theme: default
paginate: true
---

<!-- _class: lead -->
<!-- _paginate: false -->

# [Titre du sujet]

Formation · [Audience] · [Date]

---

## Objectifs

À la fin de cette présentation, vous saurez :

1. [Objectif 1]
2. [Objectif 2]
3. [Objectif 3]

---

## Plan

1. [Section 1]
2. [Section 2]
3. [Section 3]
4. Questions

---

<!-- Répéter ce pattern pour chaque section -->

## [Section 1] — [Titre]

[Concept principal en 1-2 phrases]

```[langage]
// Exemple concret si applicable
```

---

## Récapitulatif

| Concept | En une phrase |
|---------|---------------|
| [A] | [définition] |
| [B] | [définition] |

---

<!-- _class: lead -->

## Ressources

- [Lien doc interne]
- [Lien doc externe]

**Questions ?**
```

---

## Bonnes pratiques

### Règle fondamentale : 1 slide = 1 idée

Chaque slide doit transmettre **une seule information principale**.
Si le titre ne suffit pas à résumer la slide, c'est qu'il faut la découper.

### Contenu

- **Maximum 5 bullets par slide** — au-delà, découper en plusieurs slides
- **Titres actionnables** : "Pourquoi Redis ?" plutôt que "Base de données"
- **Éviter les phrases complètes** dans les bullets — mots-clés + verbes d'action
- **1 exemple de code = 1 concept** — jamais plus de ~15 lignes par slide de code
- **Les tableaux** sont préférables aux listes pour les comparaisons

### Structure

- Slide 1 : titre + auteur + date (toujours `_paginate: false` + `_class: lead`)
- Slide 2 : contexte ou objectifs — planter le décor en < 3 bullets
- Corps : 1 concept par slide
- Avant-dernière : résumé ou métriques
- Dernière : next steps ou call-to-action + "Questions ?"

### Taille recommandée

| Type | Slides recommandés |
|------|-------------------|
| Démo courte / standup | 5 – 8 |
| Présentation d'équipe | 8 – 15 |
| Formation / onboarding | 15 – 25 |
| Product pitch externe | 10 – 12 |

---

## Emplacement du fichier généré

| Situation | Emplacement |
|-----------|-------------|
| `docs/presentations/` existe | `docs/presentations/[sujet]-[date].md` |
| `docs/slides/` existe | `docs/slides/[sujet]-[date].md` |
| Aucun dossier de présentations | Proposer `docs/presentations/` et attendre confirmation |
| Pas de `docs/` du tout | Créer à la racine `[sujet]-[date].md` — signaler l'absence de structure |

Format de nommage recommandé : `kebab-case` + date ISO courte.
Exemples : `demo-v2.0-2025-05.md`, `retro-sprint-42-2025-05.md`, `onboarding-backend-2025-05.md`

---

## Workflow post-génération — détection et compilation

Après avoir écrit le fichier `.md`, exécuter la détection :

```bash
# Détection Marp
MARP_PATH=$(which marp 2>/dev/null)
NPX_MARP=$(npx --no-install @marp-team/marp-cli --version 2>/dev/null && echo "ok")
```

### Si Marp est disponible

Proposer la compilation via l'outil `question` :

```
question({
  questions: [{
    header: "Compilation Marp",
    question: "Marp est disponible. Compiler le fichier en HTML maintenant ?",
    options: [
      { label: "Compiler en HTML (Recommandé)", description: "Génère un fichier .html autonome présentable dans un navigateur" },
      { label: "Compiler en PDF", description: "Génère un fichier .pdf — nécessite Chromium ou Chrome installé" },
      { label: "Non, garder uniquement le .md", description: "Le fichier Marp source est suffisant pour l'instant" }
    ]
  }]
})
```

Commandes de compilation :

```bash
# HTML
marp docs/presentations/[fichier].md -o docs/presentations/[fichier].html
# ou avec npx
npx @marp-team/marp-cli docs/presentations/[fichier].md -o docs/presentations/[fichier].html

# PDF
marp --pdf docs/presentations/[fichier].md -o docs/presentations/[fichier].pdf
```

### Si Marp n'est pas disponible

Informer sans bloquer :

```
Le fichier Marp a été généré : docs/presentations/[fichier].md

Pour le convertir en HTML ou PDF, plusieurs options :

| Option | Commande / Lien |
|--------|-----------------|
| npx (Node.js requis) | `npx @marp-team/marp-cli [fichier].md -o [fichier].html` |
| Installation globale | `npm install -g @marp-team/marp-cli` |
| VS Code | Extension "Marp for VS Code" (prévisualisation + export) |
| En ligne | https://web.marp.app (coller le contenu Markdown) |
```

---

## Exemples d'invocation

| Demande | Action |
|---------|--------|
| `"Crée une présentation pour la démo de la v2.0"` | Exploration → question contexte → template tech-demo → génération → détection Marp |
| `"Slides de retrospective pour le sprint 42"` | Template retro → pré-rempli avec les infos disponibles → génération |
| `"Fais un pitch pour les stakeholders sur la feature paiement"` | Template product-pitch → question audience → génération |
| `"Onboarding slides pour les nouveaux devs backend"` | Template onboarding → exploration README/docs → contenu contextualisé |
| `"Qu'est-ce qui existe comme slides dans ce projet ?"` | Exploration docs/presentations/ → rapport des slides existants |
