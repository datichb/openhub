---
name: doc-slides
description: "Utiliser pour produire une présentation Marp : branding, structure par type, compilation HTML/PDF."
annexes: [templates/marp-slide-templates.md]
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
❌ Tu ne choisis JAMAIS le thème ou la structure sans avoir établi le contexte minimal (voir « Questions de contexte »)
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
    → Faire choisir le template à l'utilisateur (voir « Questions de contexte »)
    → Appeler get_template_branding({ presentationId: <ID choisi> })
SINON (MCP non configuré) :
  → Passer à l'Étape 0 (comportement standard avec thèmes Marp natifs)
```

### Injection du branding dans le frontmatter

Quand un branding est disponible, utiliser le `marpFrontmatter` retourné par `get_template_branding`
**à la place** du frontmatter standard (ne pas utiliser `theme: default|gaia|uncover`) : un bloc
`marp: true`, `paginate: true` et `style: |` qui porte le CSS du branding (fond, couleurs, police).

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

Avant de générer, explorer avec `glob` (et `ls`) :

- slides existants : `**/presentations/**/*.md`, `**/slides/**/*.md`, le dossier `docs/presentations/` ;
- thème Marp custom : `**/*marp*.css`, `**/theme*/*.css`, `.marprc*`, `marp.config.*`.

Lire un fichier de slides existant s'il en existe un pour détecter :
- Le thème utilisé (`theme: default | gaia | uncover | custom`)
- La structure type (nombre de slides, patterns de contenu)
- L'emplacement standard dans le projet

---

## Questions de contexte

Il te faut le type de présentation (démo technique, pitch stakeholders, retrospective, onboarding),
et, avec un branding Google Slides sans template par défaut, le template à utiliser.

<!-- oh:when role=entry -->
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
<!-- /oh:when -->
<!-- oh:when role=subagent -->
Sous-agent : déduis le type de la demande et du contexte, note-le comme hypothèse dans ton rapport ; seul un type impossible à déduire (ou le template de branding à choisir) devient une question bloquante.
<!-- /oh:when -->

Si l'audience, le nombre de slides souhaité ou un thème particulier ne sont pas précisés,
utiliser les valeurs par défaut des templates et le signaler.

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

Avant d'écrire le fichier, lis avec `read` le template du type retenu dans
`templates/marp-slide-templates.md` (tech demo, product pitch, retrospective, onboarding)
et remplis-le avec le contenu du projet.

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

## Après la génération — compilation

Le fichier `.md` est le livrable. Tu ne lances pas Marp (ton shell ne le permet pas) : donne à
l'utilisateur les commandes pour compiler, et signale que la compilation attend sa confirmation.

<!-- capability: user -->
```bash
# HTML
marp docs/presentations/[fichier].md -o docs/presentations/[fichier].html
# ou avec npx
npx @marp-team/marp-cli docs/presentations/[fichier].md -o docs/presentations/[fichier].html

# PDF (Chromium ou Chrome requis)
marp --pdf docs/presentations/[fichier].md -o docs/presentations/[fichier].pdf
```

Sans Marp installé :

<!-- capability: user -->
```
| Option | Commande / Lien |
|--------|-----------------|
| npx (Node.js requis) | `npx @marp-team/marp-cli [fichier].md -o [fichier].html` |
| Installation globale | `npm install -g @marp-team/marp-cli` |
| VS Code | Extension "Marp for VS Code" (prévisualisation + export) |
| En ligne | https://web.marp.app (coller le contenu Markdown) |
```
