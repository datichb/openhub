---
name: reviewer-handoff-format
description: Source de vérité pour le format de retour du reviewer vers orchestrator-dev. Définit le bloc structuré unique à produire en fin de review quand invoqué depuis orchestrator-dev. Le rapport de review complet est intégré dans le bloc. Injecté dans le reviewer et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff reviewer → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `reviewer` vers `orchestrator-dev`.
Il est injecté dans le `reviewer` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task`),
ton **seul output** est le bloc `## Retour vers orchestrator-dev` défini ci-dessous.

**Format de sortie :** aucun texte avant, après ou en dehors de ce bloc. Le rapport de review complet est **intégré dans le bloc** (section `### Rapport complet`), pas produit séparément en texte libre.

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** reviewer
**Ticket :** #<ID> — <titre>
**Branche :** <nom de la branche reviewée>

### Verdict
`commit` | `corriger` | `corriger-sécurité`

### Synthèse des problèmes

| Sévérité | Nombre | Résumé |
|----------|--------|--------|
| 🔴 Critique | <N> | <résumé 1 ligne — ex : "Injection SQL sur endpoint /users"> |
| 🟠 Majeur | <N> | <résumé> |
| 🟡 Mineur | <N> | <résumé> |
| 💡 Suggestion | <N> | <résumé> |

<"Aucun problème identifié — review propre" si le verdict est `commit` sans réserves>

### Corrections requises
<Pour chaque problème Critique ou Majeur — format actionnable pour le developer :>
- `[🔴 CRITIQUE] [SCORE: X/5]` `<fichier:ligne>` — <action concrète à réaliser>
- `[🟠 MAJEUR] [SCORE: X/5]` `<fichier:ligne>` — <action concrète>
<"Aucune correction requise" si verdict = `commit`>
<Ces corrections sont copiées VERBATIM dans les commentaires Beads du ticket>

### Suggestions (non-bloquant)
<Pour chaque problème Mineur ou Suggestion — pour information, le developer peut les ignorer :>
- `[🟡 MINEUR] [SCORE: X/5]` `<fichier:ligne>` — <suggestion>
- `[💡 SUGGESTION] [SCORE: X/5]` `<fichier:ligne>` — <suggestion>
<"Aucune suggestion" si non applicable>
<Ces suggestions ne sont PAS copiées dans les commentaires Beads — elles sont dans le rapport complet>

### Routing recommandé
`retour-initial` | `developer-security`
<`retour-initial` = ticket retourne au developer du même domaine pour correction>
<`developer-security` = ticket nécessite un developer domaine security>

### Rapport complet

## Review — <nom de la branche ou titre de la PR>

### Walkthrough
| Fichier | Changement | God node | Domaine |
|---------|-----------|----------|---------|

### Résumé
<évaluation globale — verdict justifié, qualité d'ensemble, respect des conventions>

### Périmètre et contexte
- **Conventions chargées :** [conventions.md ✓/✗, architecture.md ✓/✗, review-rules.md ✓/✗]
- **Standards appliqués :** [liste des dev-standards-* chargés]
- **God nodes touchés :** [liste ou "aucun"]

--- BLOC 1 : CORRECTIONS ---

### 🔴 Critique — bloquant
<si applicable — chaque finding avec : localisation, score, description, impact, correction attendue>

### 🟠 Majeur — à corriger
<si applicable — même format que critique>

### 🟡 Mineur — amélioration recommandée
<si applicable>

--- BLOC 2 : OBSERVATIONS ---

### 💡 Suggestions
<si applicable>

### ✅ Points positifs
<toujours inclure si pertinent — bonne pratique observée, code élégant, test bien couvert>

### 🔍 Hors scope
<observations pertinentes mais hors du périmètre de cette review — pour information uniquement>

### Statut
`approuvé` | `corrections-requises` | `bloquant-sécurité`
```

**Définitions du verdict :**

| Verdict | Condition |
|---------|-----------|
| `commit` | Code prêt à être commité — aucun problème Critique ou Majeur |
| `corriger` | Corrections nécessaires avant commit — au moins un problème Critique ou Majeur |
| `corriger-sécurité` | Corrections de sécurité nécessaires — problème Critique de type sécurité |

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `approuvé` | Verdict = `commit` |
| `corrections-requises` | Verdict = `corriger` |
| `bloquant-sécurité` | Verdict = `corriger-sécurité` |

---

## Règles pour le producteur (reviewer)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator-dev`** — aucun texte avant ou après
- **Le rapport complet est DANS le bloc** (section `### Rapport complet`) — ne pas le produire séparément en texte libre
- **Toujours inclure `### Rapport complet`** même si la review ne trouve aucun problème (review propre) — le rapport minimal comporte `### Walkthrough`, `### Résumé`, `### Périmètre et contexte` et `### ✅ Points positifs`
- **`### Corrections requises`** contient UNIQUEMENT les findings 🔴 et 🟠 — copiés VERBATIM dans les commentaires Beads. Chaque correction doit être précise, actionnable, et porter son score de confiance
- **`### Suggestions (non-bloquant)`** contient les findings 🟡 et 💡 — ils restent dans le rapport mais ne sont PAS transmis comme corrections obligatoires dans Beads
- **`### Routing recommandé`** détermine vers quel developer le ticket est renvoyé — `developer-security` uniquement pour les problèmes de sécurité nécessitant une expertise spécifique

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire le rapport comme texte libre avant le bloc — il est DANS le bloc
> ❌ Ne jamais résumer le rapport dans `### Rapport complet` — il doit être exhaustif

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du reviewer

1. **Lire le `### Verdict`** pour décider de la suite :
   - `commit` → continuer vers CP-2 (commit)
   - `corriger` ou `corriger-sécurité` → cycle de correction

2. **Au CP-2** : copier intégralement la section `### Rapport complet` du bloc dans le `## Question pour l'orchestrator > ### Rapport de review complet` pour transmission à l'orchestrator (l'utilisateur doit voir le rapport avant de décider).

3. **Transmettre les `### Corrections requises`** au developer via `bd comments add <ID>` si correction choisie. Ne PAS transmettre les `### Suggestions (non-bloquant)` comme corrections — elles sont informatives.

4. **Utiliser le `### Routing recommandé`** pour déterminer quel agent developer ré-invoquer.

5. **Si le bloc est absent ou si `### Rapport complet` est absent** → demander explicitement au reviewer de produire le bloc complet.

> ❌ Ne jamais passer au CP-2 sans `### Rapport complet` dans le bloc — le rapport est nécessaire pour la décision utilisateur.
> ❌ Ne jamais résumer le rapport quand il est transmis à l'orchestrator — le copier tel quel.
> ❌ Ne jamais transformer les `### Suggestions (non-bloquant)` en corrections obligatoires.
