---
name: onboarder-phase-0
description: Phase 0 du workflow onboarder — vérification des prérequis (projet accessible, fichiers structurants détectés).
---

## Phase 0 — Vérification des prérequis

### Objectif
Vérifier que les informations minimales pour démarrer l'onboarding sont disponibles.

### Ce qu'on vérifie
- Le projet est accessible (répertoire courant lisible)
- La racine du projet est identifiable (présence de fichiers structurants)
- Au moins un fichier de dépendances est présent pour détecter la stack

### Déclencheur de pause ⏸️

Si **un ou plusieurs prérequis critiques sont manquants** :

**Si CONTEXTE = standalone :**
```
[Texte de réponse]
## ⏸️ Phase 0 — Prérequis manquants

Pour démarrer l'onboarding, j'ai besoin de :
1. <élément manquant 1>
2. <élément manquant 2>

**Impact :** Sans ces éléments, [conséquence].

[Puis appel outil question]
question({
  questions: [{
    header: "Prérequis manquants",
    question: "[Onboarder — Phase 0 : Prérequis | Projet]\nPour démarrer l'onboarding, j'ai besoin de :\n<liste>\n\nComment procéder ?",
    options: [
      { label: "Fournir les informations", description: "Préciser les éléments manquants" },
      { label: "Continuer quand même", description: "Démarrer avec les informations disponibles — le rapport sera partiel" }
    ]
  }]
})
```


### Récap de fin de Phase 0


### Question de validation obligatoire

> **Boucle de validation Phase 0 :** voir skill `shared/phase-0-validation-loop` pour le template question et le routing. Paramètres : `{agent_label}` = Onboarder, `{context_noun}` = Projet.