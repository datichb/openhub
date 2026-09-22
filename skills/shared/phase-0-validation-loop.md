---
name: phase-0-validation-loop
description: "Boucle de validation Phase 0 — question 3 options (Démarrer/Préciser/Arrêter), routing, variante sub-agent."
bucket: B
---

# Phase 0 — Boucle de validation

## Question de validation (mode standalone)

Utiliser ce template en remplaçant `{agent_label}` et `{context_noun}` :

```
question({
  questions: [{
    header: "Démarrer l'exploration",
    question: "[{agent_label} — Phase 0 complétée | {context_noun}]\nPrérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?",
    options: [
      { label: "Démarrer (Recommandé)", description: "Passer à la Phase 1" },
      { label: "Préciser le contexte", description: "Ajouter des informations avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'analyse" }
    ]
  }]
})
```

## Routing selon la réponse

| Réponse | Action |
|---------|--------|
| **Démarrer** | Passer à la Phase 1 |
| **Préciser** | Rester en Phase 0 — intégrer les nouvelles informations, re-produire le récap, reposer la question |
| **Arrêter** | Fin de session |

## Variante sub-agent

En mode sub-agent, la validation de Phase 0 suit le mécanisme d'interruption standard (voir `shared/subagent-execution-protocol`) : produire le récap, le bloc Retour intermédiaire, le bloc Question pour l'orchestrator avec les 3 options ci-dessus, puis **terminer la session**.
