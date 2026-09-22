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
