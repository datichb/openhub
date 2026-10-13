---
name: onboarder-phase-0
description: "Utiliser avant la phase 0 de l'onboarder : projet accessible, fichiers structurants."
---

## Phase 0 — Vérification des prérequis

### Objectif
Vérifier que les informations minimales pour démarrer l'onboarding sont disponibles.

### Ce qu'on vérifie
- Le projet est accessible (répertoire courant lisible)
- La racine du projet est identifiable (présence de fichiers structurants)
- Au moins un fichier de dépendances est présent pour détecter la stack

Phase 0 : liste la racine et lis au plus les manifestes (`go.mod`, `package.json`…) et le README. Aucun fichier source ni test : leur lecture est l'exploration, après le chargement de `onboarder-phase-1`.

### Déclencheur de pause ⏸️

Si **un ou plusieurs prérequis critiques sont manquants** :
<!-- oh:when role=entry -->

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

### Question de validation obligatoire

> **Boucle de validation Phase 0 :** protocole d'exécution (inclus) : « Démarrer (Recommandé) » (Phase 1), « Préciser le contexte » (rester en Phase 0), « Arrêter ».
<!-- /oh:when -->
<!-- oh:when role=subagent -->

- Projet illisible ou sans racine identifiable : statut `bloqué`, le manque dans ton rapport.
- Autre manque : continue avec ce qui est disponible (statut `contexte-partiel`) et liste-le dans `### Zones d'incertitude`.

Pas de validation de phase : enchaîne sur la Phase 1.
<!-- /oh:when -->
