---
name: reviewer-subagent
description: Parcours d'exécution du reviewer en mode sous-agent (invoqué via task depuis orchestrator-dev ou orchestrator feature) — rapport de review complet obligatoire suivi du bloc Retour vers orchestrator-dev. Supporte les modes standard (ticket), adversarial (CP-feature), et combiné adversarial + edge-case (CP-feature avec option).
---

# Skill — Parcours Reviewer Sous-agent

> Ce skill est chargé quand le reviewer est invoqué via `task` depuis orchestrator-dev ou orchestrator feature. L'invocateur injecte `[SKILL:reviewer/reviewer-subagent]` dans le prompt.

## Principe fondamental

Quand le reviewer est invoqué via `task`, il doit toujours produire :
1. **Le rapport de review complet** (jamais résumé, jamais omis)
2. **Le bloc `## Retour vers orchestrator-dev`** (obligatoire, vient après le rapport)

---

## Détection du mode d'invocation

Le mode est déterminé par les tags présents dans le prompt :

| Tag présent | Mode | Contexte |
|-------------|------|----------|
| `[MODE:standard]` ou aucun tag MODE | Standard | Review de ticket (orchestrator-dev, Étape 4) |
| `[MODE:adversarial]` | Adversarial | CP-feature (orchestrator) |
| `[MODE:adversarial+edge-case]` | Adversarial + Edge-case combiné | CP-feature avec option edge-case |

---

## Comportement par mode

### Mode Standard (review de ticket — par défaut)

1. Exécuter le workflow de review complet (voir skill `review-protocol`)
2. Produire le rapport structuré complet au format défini dans `review-protocol`
3. Conclure avec le bloc `## Retour vers orchestrator-dev` (voir skill `reviewer-handoff-format`)

### Mode Adversarial (CP-feature)

1. Charger le skill `reviewer-adversarial` via l'outil `skill`
2. Exécuter la revue adversariale sur le diff complet feature (`git diff main..<feature-branch>`)
3. Produire le rapport au format `## Revue Adversariale — <périmètre>`
4. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur les findings adversariaux

### Mode Adversarial + Edge-case combiné (CP-feature avec option)

Pour garantir l'isolation contextuelle, orchestrer des sessions parallèles :

1. **Lancer les sessions en parallèle** via l'outil `task` :
   ```
   // Session 1 — Adversarial (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [SKILL:reviewer/reviewer-standalone-single] Revue adversariale de la feature <branche>. git diff main..<branche>")

   // Session 2 — Edge-case (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:edge-case] [SKILL:reviewer/reviewer-standalone-single] Analyse edge-case de la feature <branche>. git diff main..<branche>")
   ```

2. **Récupérer les rapports bruts** de chaque session
3. **Fusionner** en chargeant le skill `review-merge` et en lui fournissant les rapports
4. Produire le rapport unifié final
5. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur le rapport unifié post-fusion

---

## Règle absolue

> ❌ Ne jamais produire le bloc handoff seul — le rapport complet est la condition préalable
> ❌ Ne jamais résumer le rapport pour aller plus vite
> ✅ Rapport complet PUIS bloc handoff, dans cet ordre

---

## Format final (mode standard)

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

## Format final (mode adversarial ou combiné)

```markdown
## Revue Adversariale — <feature-branch>
<ou ## Review unifiée — <feature-branch> si mode combiné>

<rapport selon le format du mode activé>

---

## Retour vers orchestrator-dev

<contenu défini dans le skill reviewer-handoff-format>
```

> Un rapport sans problèmes comporte au minimum `### Résumé` et `### ✅ Points positifs`.
