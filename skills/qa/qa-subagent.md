---
name: qa-subagent
description: Parcours d'exécution du QA engineer en mode sous-agent (invoqué via task depuis orchestrator-dev) — rapport de couverture complet obligatoire suivi du bloc Retour vers orchestrator-dev. Le rapport précède toujours le bloc handoff.
---

# Skill — Parcours QA Sous-agent

> Ce skill est chargé quand le qa-engineer est invoqué via `task` depuis orchestrator-dev. L'orchestrator-dev injecte `[SKILL:qa/qa-subagent]` dans le prompt.

## Principe fondamental

Quand le qa-engineer est invoqué via `task`, il doit toujours produire :
1. **Le rapport QA complet** (jamais résumé, jamais omis)
2. **Le bloc `## Retour vers orchestrator-dev`** (obligatoire, vient après le rapport)

---

## Comportement sous-agent

1. Exécuter le workflow QA complet (voir skill `qa-protocol`)
2. Écrire les tests manquants directement dans le projet
3. Produire le rapport de couverture structuré au format défini dans `qa-protocol`
4. Conclure avec le bloc `## Retour vers orchestrator-dev` (voir skill `qa-handoff-format`)

---

## Règle absolue

> ❌ Ne jamais produire le bloc handoff seul — le rapport complet est la condition préalable
> ❌ Ne jamais résumer le rapport
> ✅ Rapport complet PUIS bloc handoff, dans cet ordre

---

## Format final

```markdown
## Rapport QA — <nom de la branche ou ticket #ID>

### Résumé
<périmètre analysé, état couverture avant/après, points d'attention>

### Tests écrits
<tableau des fichiers de test, type, cas couverts>

### Couverture estimée
<tableau module / avant / après / gaps>

### ⚠️ Zones non testables identifiées
<si applicable>

### 💡 Suggestions
<si applicable>

---

## Retour vers orchestrator-dev

<contenu défini dans le skill qa-handoff-format>
```
