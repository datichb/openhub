# Template — Format du rapport de review

```
## Review — <nom de la branche ou titre de la PR>

### Walkthrough
| Fichier | Changement | God node | Domaine |
|---------|-----------|----------|---------|
| <fichier modifié> | <résumé 1 ligne> | <god node ou —> | <domaine ou —> |

### Résumé
<1-3 phrases : ce que fait la PR, ton évaluation globale>

### Périmètre et contexte
- **Conventions chargées :** [conventions.md ✓/✗, architecture.md ✓/✗, review-rules.md ✓/✗]
- **Standards appliqués :** [liste des dev-standards-* chargés]
- **God nodes touchés :** [liste ou "aucun"]
> ⚠️ Review sans contexte wiki — findings basés sur standards génériques uniquement. (si applicable)

### Couverture des critères d'acceptance
*(Inclure uniquement si un ticket Beads avec critères d'acceptance est disponible. Omettre sinon.)*

| Critère | Implémenté | Testé | Fichier(s) |
|---------|-----------|-------|------------|
| <critère 1 du ticket> | ✅/⚠️/❌ | ✅/❌ | `fichier:ligne` ou — |
| <critère 2 du ticket> | ✅/⚠️/❌ | ✅/❌ | `fichier:ligne` ou — |

> Légende : ✅ = vérifié dans le diff, ⚠️ = partiellement implémenté ou ambigu, ❌ = non identifié dans le diff.
> Les ❌ et ⚠️ sont repris comme findings dans les sections de corrections ci-dessous.

--- BLOC 1 : CORRECTIONS (workflow orchestrator → developer) ---

### 🔴 Critique — bloquant
<Problèmes qui doivent être résolus avant merge>

### 🟠 Majeur — à corriger
<Problèmes importants mais non-bloquants pour le merge immédiat>

### 🟡 Mineur — amélioration recommandée
<Petits écarts aux standards, nommage, lisibilité>

--- BLOC 2 : OBSERVATIONS (pour le reviewer humain, non-bloquant) ---

### 💡 Suggestions
<Idées d'amélioration, alternatives, pistes futures — sans pression>

### ✅ Points positifs
<Ce qui est bien fait — toujours inclure si pertinent>

### 🔍 Hors scope
<Problèmes existants détectés mais qui ne concernent pas cette PR>
```
