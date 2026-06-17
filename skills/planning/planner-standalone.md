---
name: planner-standalone
description: Parcours d'exécution du planner en mode standalone (invoqué directement par l'utilisateur) — récaps en texte clair avant chaque appel question, validation via outil question à chaque phase, sans blocs handoff orchestrateur.
---

# Skill — Parcours Planner Standalone

> Ce skill est chargé automatiquement quand le planner est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

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
<contenu complet du récap — observations, découvertes, décisions>

[Puis appel outil question]
question({
  questions: [{
    header: "...",
    question: "[Planner — Phase X | Feature : <nom>]\n<question de validation>",
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

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai affiché le récap complet de la phase actuelle en texte dans la discussion | ⬜ |
| ✅ Le récap contient toutes les observations, découvertes et décisions de cette phase | ⬜ |
| ✅ Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| ✅ Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et produire le contenu manquant MAINTENANT.**

---

## Format des questions de validation (standalone)

### Phase 0 — Prérequis vérifiés
```
question({
  questions: [{
    header: "Démarrer l'exploration",
    question: "[Planner — Phase 0 complétée | Feature : <nom>]\nPrérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?",
    options: [
      { label: "Démarrer (Recommandé)", description: "Passer à la Phase 1 — Exploration contextuelle" },
      { label: "Préciser le contexte", description: "Ajouter des informations avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'analyse" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle (signal design détecté)
```
question({
  questions: [{
    header: "Délégation design",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté>\n- Tests existants : <état>\n- Signal <UX/UI> détecté : <raison concrète>\n- Zones d'ombre : <liste courte>\n\nComment procéder ?",
    options: [
      { label: "Phase 1.5 — Délégation design (Recommandé)", description: "Invoquer <ux-designer/ui-designer> avant de planifier" },
      { label: "Skip design — Phase 2", description: "Passer aux questions complémentaires sans spec design" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de décider" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle (aucun signal design)
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté>\n- Tests existants : <état>\n- Aucun signal design détecté\n- Zones d'ombre : <liste courte ou 'Aucune'>\n\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```

### Phase 1.5 — Délégation design terminée
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1.5 complétée | Feature : <nom>]\nSpecs design intégrées. Passer aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les specs design reçues" }
    ]
  }]
})
```

### Phase 2 — Questions complémentaires traitées
```
question({
  questions: [{
    header: "Plan hiérarchique",
    question: "[Planner — Phase 2 complétée | Feature : <nom>]\nQuestions traitées. Passer à l'analyse approfondie (Phase 3 — Plan hiérarchique) ?",
    options: [
      { label: "Passer à Phase 3 (Recommandé)", description: "Démarrer la décomposition en epics et tickets" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations reçues" }
    ]
  }]
})
```

### Phase 3 — Plan hiérarchique
```
question({
  questions: [{
    header: "Validation du plan",
    question: "[Planner — Phase 3 complétée | Feature : <nom>]\nEst-ce que ce découpage vous convient ? Souhaitez-vous modifier, ajouter ou supprimer des éléments avant que je crée les tickets ?",
    options: [
      { label: "Valider le plan (Recommandé)", description: "Passer à la détection des cas particuliers (Phase 4)" },
      { label: "Modifier le plan", description: "Apporter des modifications au découpage" },
      { label: "Revenir à Phase 2", description: "Reposer des questions avant de finaliser le plan" }
    ]
  }]
})
```

### Phase 4 — Cas particuliers
```
question({
  questions: [{
    header: "Création des tickets",
    question: "[Planner — Phase 4 complétée | Feature : <nom>]\nDétection des cas particuliers terminée. Passer à la création des tickets dans Beads (Phase 5) ?",
    options: [
      { label: "Créer les tickets (Recommandé)", description: "Passer à la Phase 5 — Création dans Beads" },
      { label: "Vérifier d'autres cas", description: "Rester en Phase 4 pour vérifier d'autres cas particuliers" },
      { label: "Revenir à Phase 3", description: "Revoir le plan après détection de cas particuliers critiques" }
    ]
  }]
})
```

### Phase 5.5 — Délégation ai-delegated
```
question({
  questions: [{
    header: "Délégation ai-delegated",
    question: "[Planner — Phase 5.5 | Feature : <nom>]\nSouhaitez-vous déléguer certains tickets à l'agent IA (label ai-delegated) ?",
    options: [
      { label: "Non", description: "Aucun ticket délégué à l'IA" },
      { label: "Oui — certains tickets", description: "Indiquer les IDs dans la réponse libre" },
      { label: "Oui — tous les tickets éligibles", description: "Déléguer tous les tickets sans dépendance bloquante" }
    ]
  }]
})
```

### Phase 6 — Vérification finale
```
question({
  questions: [{
    header: "Validation finale",
    question: "[Planner — Phase 6 complétée | Feature : <nom>]\nLes tickets correspondent-ils à vos attentes ? Souhaitez-vous des ajustements ?",
    options: [
      { label: "Oui — c'est bon", description: "Planning terminé" },
      { label: "Ajustements à faire", description: "Apporter des modifications aux tickets créés" }
    ]
  }]
})
```

### Retour en arrière
```
question({
  questions: [{
    header: "Retour à Phase X",
    question: "[Planner — Retour en arrière | Feature : <nom>]\n<raison du retour>. Revenir à la Phase X pour <action> ?",
    options: [
      { label: "Oui, revenir à Phase X", description: "<ce qui sera fait en Phase X>" },
      { label: "Non, continuer", description: "Poursuivre avec l'information disponible" }
    ]
  }]
})
```

---

## Format final (Phase 6)

Produire uniquement le récapitulatif de planification complet (voir section Phase 6 du skill `planner-workflow`), **sans** le bloc `## Retour vers orchestrator`.

**Selon la réponse à la validation finale :**
- **C'est bon** → Fin de session
- **Ajustements** → rester en Phase 6, appliquer les ajustements via `bd update`, re-produire le récap
