---
name: reviewer-subagent
description: Parcours d'exécution du reviewer en mode sous-agent (invoqué via task depuis orchestrator-dev) — rapport de review complet obligatoire suivi du bloc Retour vers orchestrator-dev. Le rapport précède toujours le bloc handoff.
---

# Skill — Parcours Reviewer Sous-agent

> Ce skill est chargé quand le reviewer est invoqué via `task` depuis orchestrator-dev. L'orchestrator-dev injecte `[SKILL:reviewer/reviewer-subagent]` dans le prompt.

## Principe fondamental

Quand le reviewer est invoqué via `task`, il doit toujours produire :
1. **Le rapport de review complet** (jamais résumé, jamais omis)
2. **Le bloc `## Retour vers orchestrator-dev`** (obligatoire, vient après le rapport)

---

## Comportement sous-agent

1. Exécuter le workflow de review complet (voir skill `review-protocol`)
2. Produire le rapport structuré complet au format défini dans `review-protocol`
3. Conclure avec le bloc `## Retour vers orchestrator-dev` (voir skill `reviewer-handoff-format`)

---

## Règle absolue

> ❌ Ne jamais produire le bloc handoff seul — le rapport complet est la condition préalable
> ❌ Ne jamais résumer le rapport pour aller plus vite
> ✅ Rapport complet PUIS bloc handoff, dans cet ordre

---

## Format final

```markdown
## Review — <nom de la branche ou titre de la PR>

### Résumé
<évaluation globale>

### 🔴 Critique — bloquant
<si applicable>

### 🟠 Majeur — à corriger
<si applicable>

### 🟡 Mineur — amélioration recommandée
<si applicable>

### 💡 Suggestion — optionnel
<si applicable>

### ✅ Points positifs
<toujours inclure si pertinent>

### 🔍 Hors scope
<si applicable>

---

## Retour vers orchestrator-dev

<contenu défini dans le skill reviewer-handoff-format>
```

> Un rapport sans problèmes comporte au minimum `### Résumé` et `### ✅ Points positifs`.
