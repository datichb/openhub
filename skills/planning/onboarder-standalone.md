---
name: onboarder-standalone
description: Parcours d'exécution de l'onboarder en mode standalone (invoqué directement par l'utilisateur) — récaps en texte clair avant chaque appel question, validation via outil question à chaque phase, rapport final sans bloc handoff orchestrateur.
---

# Skill — Parcours Onboarder Standalone

> Ce skill est chargé automatiquement quand l'onboarder est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le texte de chaque phase est **directement visible** par l'utilisateur dans la discussion. La communication se fait via :
1. Le texte de réponse (récap complet de la phase)
2. L'outil `question` pour les validations et décisions

---

## Règle absolue — récap avant question

**À CHAQUE fin de phase :**

1. **TOUJOURS produire le récap en texte clair AVANT d'appeler l'outil `question`**
   - Le récap doit être affiché comme texte de réponse dans la discussion
   - Jamais intégré dans le champ `question` de l'outil
   - Jamais omis

2. **PUIS appeler l'outil `question` pour la validation**

**Séquence obligatoire :**
```
[Texte de réponse]
## [Phase X] <titre du récap>
<contenu complet du récap>

[Puis appel outil question]
question({
  questions: [{
    header: "...",
    question: "[Onboarder — Phase X | Projet : <nom>]\n<question de validation>",
    options: [...]
  }]
})
```

> ❌ **JAMAIS** : appeler `question` sans avoir d'abord affiché le récap
> ✅ **TOUJOURS** : afficher le récap en texte → puis appeler `question`

---

## Autocontrôle avant chaque appel `question`

> « Ai-je produit le récap en texte clair dans la discussion avant cet appel ? »
> - **Non** → produire le récap maintenant, puis appeler `question`
> - **Oui** → appeler `question`

---

## ✅ Checklist visuelle — AVANT CHAQUE CHECKPOINT

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai affiché le récap complet de la phase actuelle en texte dans la discussion | ⬜ |
| ✅ Le récap contient toutes les observations, découvertes et décisions de cette phase | ⬜ |
| ✅ Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| ✅ Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |

---

## Format des questions de validation (standalone)

### Phase 0 — Prérequis vérifiés
```
question({
  questions: [{
    header: "Démarrer l'exploration",
    question: "[Onboarder — Phase 0 complétée | Projet : <nom>]\nPrérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?",
    options: [
      { label: "Démarrer (Recommandé)", description: "Passer à la Phase 1 — Exploration contextuelle" },
      { label: "Préciser le contexte", description: "Ajouter des informations avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'onboarding" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Onboarder — Phase 1 complétée | Projet : <nom>]\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```

### Phase 2 — Questions complémentaires traitées
```
question({
  questions: [{
    header: "Rapport de contexte",
    question: "[Onboarder — Phase 2 complétée | Projet : <nom>]\nQuestions traitées. Passer à l'analyse approfondie (Phase 3 — Rapport de contexte) ?",
    options: [
      { label: "Passer à Phase 3 (Recommandé)", description: "Produire le rapport de contexte structuré" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations reçues" }
    ]
  }]
})
```

### Phase 3 — Rapport de contexte
```
question({
  questions: [{
    header: "Vérification des incohérences",
    question: "[Onboarder — Phase 3 complétée | Projet : <nom>]\nRapport de contexte produit. Passer à la vérification des incohérences (Phase 4) ?",
    options: [
      { label: "Passer à Phase 4 (Recommandé)", description: "Vérifier les incohérences et compléter le rapport" },
      { label: "Ajuster le rapport", description: "Modifier des éléments avant de continuer" }
    ]
  }]
})
```

### Phase 4 — Vérification des incohérences
```
question({
  questions: [{
    header: "Production du wiki",
    question: "[Onboarder — Phase 4 complétée | Projet : <nom>]\nVérifications terminées. Passer à la production du wiki (Phase 5) ?",
    options: [
      { label: "Passer à Phase 5 (Recommandé)", description: "Générer le wiki docs/wiki/ + ONBOARDING.md" },
      { label: "Revoir le rapport", description: "Ajuster le rapport avant de produire le wiki" }
    ]
  }]
})
```

---

## Format final (standalone)

Produire uniquement le rapport d'onboarding complet (voir skill `onboarder-handoff-format`), **sans** le bloc `## Retour vers orchestrator`.
