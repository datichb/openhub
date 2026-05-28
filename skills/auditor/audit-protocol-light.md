---
name: audit-protocol-light
description: Protocole d'audit allégé — format de rapport structuré, niveaux de criticité, format des findings individuels et scoring /10. Version compacte d'audit-protocol, injectée dans les sous-agents spécialisés.
---

# Skill — Protocole d'Audit (light)

## Règles absolues

❌ Tu ne modifies JAMAIS un fichier du projet audité (sauf ONBOARDING.md et CONVENTIONS.md via `living-docs-enrichment`)
❌ Tu ne crées JAMAIS de fichiers dans le projet audité
✅ Si tu es incertain, tu formules en question plutôt qu'en affirmation
✅ Tu restes factuel : chaque finding est accompagné d'une référence de fichier/ligne
✅ Tu priorises par impact réel : un problème critique doit remonter même si peu fréquent

---

## Format du rapport d'audit

Toujours produire le rapport dans cette structure, dans cet ordre.
Omettre les sections vides (ne pas écrire "Aucun" si rien à signaler).

```
## Audit [DOMAINE] — <nom du projet ou du périmètre audité>

### Résumé exécutif
<3-5 phrases : périmètre analysé, score global, problèmes les plus critiques, tendance générale>

### Score global
<NOTE> /10 — <Appréciation courte>

### 🔴 Critique — action immédiate requise
<Problèmes qui exposent un risque grave : sécurité, perte de données, inaccessibilité totale>

### 🟠 Majeur — à corriger dans le sprint
<Problèmes importants qui dégradent significativement la qualité ou la conformité>

### 🟡 Mineur — amélioration recommandée
<Petits écarts, manques partiels, points perfectibles>

### 💡 Suggestion — bonne pratique
<Recommandations proactives, pistes d'amélioration futures>

### ✅ Points positifs
<Ce qui est bien fait — toujours inclure si pertinent>

### 📋 Plan d'action priorisé
<Liste numérotée des actions à entreprendre, de la plus urgente à la moins urgente>
```

---

## Niveaux de criticité

| Niveau | Définition |
|--------|-----------|
| 🔴 Critique | Faille exploitable, violation légale grave, perte de données possible, blocage total d'accès |
| 🟠 Majeur | Violation d'une norme de référence, N+1 à fort impact, absence de gestion d'erreur critique, dette significative sur module central |
| 🟡 Mineur | Non-conformité partielle, manque de test sur chemin secondaire, nommage ou structure perfectibles |
| 💡 Suggestion | Alternative plus performante, extraction en composant réutilisable, amélioration DX |

Un seul 🔴 Critique suffit à déconseiller la mise en production, quel que soit le score global.

---

## Format des findings individuels

```
**[CRITICITÉ]** `chemin/vers/fichier:ligne` — <titre court>

Référence : <norme, article, règle (ex: OWASP A03, WCAG 1.4.3, RGPD art.5)>

<Explication en 1-3 phrases : quel est le problème et pourquoi c'est important>

<Recommandation concrète si possible>
```

**Exemple :**
```
**[🔴 Critique]** `src/controllers/auth.controller.ts:34` — Injection SQL possible

Référence : OWASP A03:2021 — Injection

La requête SQL est construite par concaténation directe du paramètre `username` sans
échappement ni paramétrage. Un attaquant peut exfiltrer ou modifier la base de données.

Recommandation : utiliser des requêtes paramétrées ou un ORM avec bindings automatiques.
```

---

## Scoring

| Score | Appréciation |
|-------|-------------|
| 9-10  | Excellent — conforme, robuste, bien maintenu |
| 7-8   | Bon — quelques points d'amélioration non bloquants |
| 5-6   | Passable — des problèmes majeurs à corriger |
| 3-4   | Insuffisant — des problèmes critiques bloquants |
| 0-2   | Critique — mise en production déconseillée |

---

## Ce que tu ne fais PAS

- Modifier, créer ou supprimer des fichiers dans le projet audité (sauf ONBOARDING.md et CONVENTIONS.md via le skill `living-docs-enrichment`)
- Répéter le même finding sur chaque occurrence — signaler le pattern une fois et lister les occurrences
- Présenter une liste exhaustive sans priorisation — toujours hiérarchiser par impact

---

## Section "Découvertes à documenter"

**Tous les rapports d'audit** doivent se terminer par cette section standardisée avant le bloc de handoff.
Cette section permet au coordinateur `auditor` de consolider les découvertes pour enrichissement des documents vivants.

**Format standardisé :**

```
### Découvertes à documenter

**Bonnes pratiques identifiées :**
- <pratique 1 — fichier/composant où elle a été observée>
- <pratique 2>

<Écrire "Aucune bonne pratique notable identifiée" si vide>

**Patterns à généraliser :**
- <pattern 1 — où il est appliqué + où il pourrait être répliqué>
- <pattern 2>

<Écrire "Aucun pattern à généraliser identifié" si vide>

**Documentation manquante ou obsolète :**
- <doc 1 — ce qui manque ou doit être mis à jour>
- <doc 2>

<Écrire "Documentation à jour" si vide>
```

**Exemples concrets :**

```
### Découvertes à documenter

**Bonnes pratiques identifiées :**
- Gestion d'erreur exhaustive avec codes HTTP appropriés dans `src/controllers/users.controller.ts`
- Tests de sécurité systématiques sur tous les endpoints d'authentification

**Patterns à généraliser :**
- Validation Zod systématique avant insertion BDD (appliqué dans users, à répliquer sur orders, products)
- Pattern decorator @RateLimit sur endpoints publics (appliqué sur /auth, absent sur /api/search)

**Documentation manquante ou obsolète :**
- Politique de gestion des secrets non documentée (détection de `.env` committé)
- ONBOARDING.md ne mentionne pas la procédure de rotation des tokens JWT
```

---

## Étape post-rapport — Enrichissement des documents vivants

**Après avoir produit le rapport d'audit complet**, appliquer le skill `living-docs-enrichment`
si disponible (injecté dans l'agent) :

1. Identifier les découvertes techniques ou fonctionnelles à capitaliser
2. Si des enrichissements pertinents existent → proposer l'enrichissement à l'utilisateur
3. Si aucun enrichissement pertinent → afficher `> 💾 Documents vivants : aucune nouvelle découverte à capitaliser.`

> Cette étape est **toujours post-rapport** — jamais pendant l'analyse.
> Elle est **toujours conditionnée** à une confirmation explicite de l'utilisateur.
