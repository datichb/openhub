# Template — Présentations Marp par type

Le frontmatter de chaque template est remplacé par le `marpFrontmatter` du branding entreprise quand il existe.

## Tech demo / feature

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

## Product pitch / stakeholders

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

## Retrospective / bilan

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

## Onboarding / formation

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
