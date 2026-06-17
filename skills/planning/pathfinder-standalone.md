---
name: pathfinder-standalone
description: Parcours d'exécution du pathfinder en mode standalone (invoqué directement par l'utilisateur) — récap en texte clair avant question, outil question pour les pauses, rapport final sans bloc handoff orchestrateur.
---

# Skill — Parcours Pathfinder Standalone

> Ce skill est chargé automatiquement quand le pathfinder est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le texte est **directement visible** par l'utilisateur. La communication se fait via :
1. Le texte de réponse (rapport, contexte, récap)
2. L'outil `question` pour les clarifications et pauses

---

## Règle absolue — récap avant question

**Avant tout appel à l'outil `question` :**

1. **TOUJOURS afficher le contexte en texte clair** dans la discussion avant d'appeler `question`
2. **PUIS** appeler l'outil `question`

> ❌ **JAMAIS** : appeler `question` sans avoir d'abord affiché le contexte
> ✅ **TOUJOURS** : afficher le contexte → puis appeler `question`

### Format standard pour une pause avec question

```markdown
## ⏸️ Pause — <sujet>

<Contexte de la pause : ce qui a été observé, ce qui pose question, impact sur la suite>

**Options disponibles :**
- <Option A> → <conséquence>
- <Option B> → <conséquence>
```

Puis appeler l'outil `question`.

---

## Format final (standalone)

Produire le rapport pathfinder complet (voir skill `pathfinder-handoff-format` pour le format exact), **sans** le bloc `## Retour vers orchestrator`.

Après le rapport, si des tickets doivent être créés, demander la confirmation via l'outil `question` :

```
question({
  questions: [{
    header: "Créer les tickets",
    question: "Créer les tickets suggérés dans Beads ?",
    options: [
      { label: "Oui — créer (Recommandé)", description: "Créer les tickets du draft de plan" },
      { label: "Non", description: "Laisser la création au planner (si escalade recommandée)" }
    ]
  }]
})
```
