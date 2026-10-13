---
name: audit-protocol-light
description: "Utiliser pour rédiger ou lire un rapport d'audit de domaine : structure, criticités, format des findings, score /10."
---

# Skill — Protocole d'Audit (light)
<!-- oh:when role=subagent -->

## Règles absolues

❌ Tu ne modifies JAMAIS un fichier du projet audité
❌ Tu ne crées JAMAIS de fichiers dans le projet audité
✅ Si tu es incertain, tu formules en question plutôt qu'en affirmation
✅ Tu restes factuel : chaque finding est accompagné d'une référence de fichier/ligne
✅ Tu priorises par impact réel : un problème critique doit remonter même si peu fréquent
- Un même problème répété : signale le pattern une fois et liste les occurrences ; jamais de liste exhaustive sans priorisation.

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

## Section "Enrichissements wiki proposés"

**Tous les rapports d'audit** se terminent par cette section, avant l'enveloppe de retour : le coordinateur la regroupe pour l'enrichissement des documents vivants.
<!-- oh:when mcp=wiki -->
Avec l'outil `wiki_propose`, après le rapport complet (jamais pendant l'analyse), propose chaque découverte selon `living-docs-enrichment` : la section cite alors leurs `wp-…`. Sans l'outil, ou si la proposition n'est pas enregistrée, elle suit le format ci-dessous.
<!-- /oh:when -->

```
### Enrichissements wiki proposés

**Bonnes pratiques identifiées :**
- <pratique — fichier ou composant où elle est observée>

**Patterns à généraliser :**
- <pattern — où il est appliqué, où il pourrait être répliqué>

**Documentation manquante ou obsolète :**
- <ce qui manque ou doit être mis à jour>
```

Rubrique vide : « Aucune bonne pratique notable identifiée », « Aucun pattern à généraliser identifié », « Documentation à jour ». Exemple de pattern : « Validation Zod systématique avant insertion en base (appliquée dans users, à répliquer sur orders, products) ».

---
<!-- /oh:when -->

## Niveaux de criticité

| Niveau | Définition |
|--------|-----------|
| 🔴 Critique | Faille exploitable, violation légale grave, perte de données possible, blocage total d'accès |
| 🟠 Majeur | Violation d'une norme de référence, N+1 à fort impact, absence de gestion d'erreur critique, dette significative sur module central |
| 🟡 Mineur | Non-conformité partielle, manque de test sur chemin secondaire, nommage ou structure perfectibles |
| 💡 Suggestion | Alternative plus performante, extraction en composant réutilisable, amélioration DX |

Un seul 🔴 Critique suffit à déconseiller la mise en production, quel que soit le score global.

## Scoring

| Score | Appréciation |
|-------|-------------|
| 9-10  | Excellent — conforme, robuste, bien maintenu |
| 7-8   | Bon — quelques points d'amélioration non bloquants |
| 5-6   | Passable — des problèmes majeurs à corriger |
| 3-4   | Insuffisant — des problèmes critiques bloquants |
| 0-2   | Critique — mise en production déconseillée |
<!-- oh:when role=entry -->

## Lecture d'un rapport de domaine

Le rapport d'un domaine, rédigé par le sous-agent selon ce protocole, te donne, pour la consolidation : le score /10 et son appréciation, le nombre de findings par criticité, le plan d'action priorisé, le périmètre non couvert et les `### Enrichissements wiki proposés` (identifiants `wp-…` déjà en file, ou les découvertes elles-mêmes quand l'outil manquait). Tu ne réécris pas ses findings : cite le rapport par son identifiant `rp-…`.
<!-- /oh:when -->
