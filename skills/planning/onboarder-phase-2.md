---
name: onboarder-phase-2
description: Phase 2 du workflow onboarder — questions complémentaires (stratégie, conventions, zones d'ombre).
---

## Phase 2 — Questions complémentaires

### Objectif
Poser les questions de clarification identifiées en Phase 1 pour lever les zones d'ombre.

### Ce qu'on fait

1. **Regrouper TOUTES les questions** de clarification en un seul appel `question`
2. **Formuler les questions en s'appuyant sur les observations de Phase 1** — pas de questions génériques
3. **Prioriser les questions par impact** — les plus impactantes en premier (5 questions maximum)

### Types de questions à poser

#### Questions sur la stratégie projet
- L'architecture actuelle ([pattern détecté]) est-elle celle à conserver ou y a-t-il une cible de migration ?
- La dette identifiée ([X points]) est-elle connue et acceptée, ou doit-elle être priorisée ?
- Quel niveau de qualité visé ? (couverture tests, conformité RGPD/RGAA)

#### Questions sur les conventions ambiguës
- [Fichier A] utilise kebab-case, [Fichier B] utilise camelCase — quelle est la convention à suivre ?
- Certains tests sont dans `/tests`, d'autres colocalisés — où doivent-ils être créés ?
- J'ai vu `feature/XXX`, `feat/XXX`, `features/XXX` — quel format privilégier ?

#### Questions sur les zones d'ombre
- Le processus de déploiement n'est pas documenté — y a-t-il un runbook ou une procédure ?
- Combien d'environnements existent (dev, staging, prod, preview) ?
- Y a-t-il un système de feature flags actif ? Si oui, lequel ?

#### Questions sur le contexte métier (si flou)

Si le contexte métier est absent du README ou non documenté :
- Quel est le domaine d'application de ce projet ? (e-commerce, fintech, santé, SaaS, autre)
- Qui sont les utilisateurs finaux ? (clients, admins, patients, employés, etc.)
- Y a-t-il des concepts métier clés à connaître ? Si oui, existe-t-il un glossaire ?
- Y a-t-il des règles métier spécifiques documentées ailleurs (wiki, Notion, Confluence) ?

#### Questions sur la stratégie de test (si ambiguë)

Si la stratégie n'est pas claire depuis les fichiers de config :
- Quelle est la philosophie de test privilégiée ? (TDD systématique, tests après implémentation, BDD)
- Quel est le seuil de couverture visé, même s'il n'est pas configuré ? (80%, 90%, pas de cible)
- Les tests E2E sont-ils réservés aux parcours critiques ou doivent-ils être exhaustifs ?
- Les tests unitaires sont-ils obligatoires sur toute logique métier ou seulement recommandés ?

#### Questions sur Figma (si maquettes trouvées mais ambiguës)

Si des fichiers Figma existent mais leur statut n'est pas clair :
- Ces maquettes sont-elles à jour et ready-for-dev, ou encore en WIP ?
- Les design tokens Figma sont-ils la source de vérité, ou le code CSS ?
- Y a-t-il une convention de synchronisation Figma → code ? (manuelle, plugin, aucune)

### Format de la question

Afficher d'abord le contexte en texte :


Puis appeler l'outil `question` avec **une question par clarification** :


> **Si CONTEXTE = orchestrator_feature** : enrichir le champ `question` de la **première question** avec un condensé des observations Phase 1 (architecture, zones d'ombre, signaux détectés) — c'est la seule information visible dans la session parent.

**Si CONTEXTE = standalone ou orchestrator_feature :**
```
question({
  questions: [
    // Question stratégie — Architecture (avec condensé Phase 1 si orchestrateur)
    {
      header: "Architecture cible",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\n\n**Contexte de l'exploration (Phase 1) :**\n- Architecture détectée : <pattern détecté>\n- Zones d'ombre : <liste courte ou 'Aucune'>\n- Points d'attention : <liste courte ou 'Aucun'>\n\nL'architecture actuelle (<pattern détecté>) est-elle celle à conserver ?",
      options: [
        { label: "Conserver (Recommandé)", description: "L'architecture actuelle est la cible — pas de migration prévue" },
        { label: "Migration prévue", description: "Une cible de migration existe — la préciser en réponse libre" },
        { label: "À définir", description: "Pas de décision prise — à traiter comme zone d'ombre" }
      ]
    },
    // Question stratégie — Dette technique (si dette identifiée en Phase 1)
    {
      header: "Dette technique",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\nJ'ai identifié <X points> de dette technique. Cette dette est-elle connue et acceptée ?",
      options: [
        { label: "Connue et acceptée", description: "La dette est documentée et priorisée consciemment" },
        { label: "À prioriser", description: "La dette doit être traitée — la documenter dans le wiki" },
        { label: "À ignorer pour l'instant", description: "Hors périmètre — noter sans recommandation urgente" }
      ]
    },
    // Question conventions (si ambiguïté détectée en Phase 1 — adapter selon les fichiers lus)
    {
      header: "Convention de code",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\n[Fichier A] utilise <convention A>, [Fichier B] utilise <convention B>. Quelle convention suivre pour les nouvelles contributions ?",
      options: [
        { label: "<Convention A>", description: "Aligner sur la convention observée dans [Fichier A]" },
        { label: "<Convention B>", description: "Aligner sur la convention observée dans [Fichier B]" },
        { label: "Pas de préférence", description: "Documenter la coexistence sans imposer une norme" }
      ]
    },
    // Question zones d'ombre — Déploiement (si runbook absent)
    {
      header: "Processus de déploiement",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\nLe processus de déploiement n'est pas documenté. Y a-t-il un runbook ou une procédure existante ?",
      options: [
        { label: "Oui — à documenter", description: "Un runbook existe — me le transmettre pour l'intégrer au wiki" },
        { label: "CI/CD automatisée", description: "Le déploiement est entièrement automatisé via la pipeline" },
        { label: "Non documenté", description: "Pas de runbook — noter comme zone d'ombre persistante" }
      ]
    },
    // Question stratégie de test (si ambiguïté détectée en Phase 1)
    {
      header: "Stratégie de test",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\nQuelle est la philosophie de test privilégiée sur ce projet ?",
      options: [
        { label: "TDD systématique", description: "Tests écrits avant l'implémentation — obligation sur toute logique métier" },
        { label: "Tests après implémentation", description: "Tests rédigés après le code — couverture cible à préciser" },
        { label: "Pas de stratégie définie", description: "Documenter l'absence comme zone d'ombre" }
      ]
    },
    // Question Figma (uniquement si des fichiers Figma ont été trouvés en Phase 1 mais leur statut est ambigu)
    {
      header: "Statut des maquettes",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\nDes références Figma ont été trouvées. Ces maquettes sont-elles à jour et ready-for-dev ?",
      options: [
        { label: "À jour — ready-for-dev", description: "Les maquettes font foi — les intégrer comme source de vérité" },
        { label: "En WIP", description: "Maquettes en cours — ne pas s'y fier pour les specs techniques" },
        { label: "Obsolètes", description: "Maquettes dépassées — le code CSS est la source de vérité" }
      ]
    },
    // Option Skip globale en dernière position
    {
      header: "Skip questions",
      question: "[Onboarder — Phase 2 | Projet : <nom>]\nSi vous préférez ne pas répondre aux questions ci-dessus, vous pouvez passer cette étape.",
      options: [
        { label: "J'ai répondu", description: "Continuer avec mes réponses" },
        { label: "Skip toutes", description: "Passer les clarifications — l'analyse restera partielle" }
      ]
    }
  ]
})
```

> **Note :** L'option "Type your own answer" est ajoutée automatiquement par OpenCode à chaque question — ne pas la dupliquer. L'utilisateur peut toujours saisir une réponse libre si aucune option ne convient.

> **Règle d'adaptation** : N'inclure que les questions pertinentes selon ce qui a été découvert en Phase 1. Si aucune ambiguïté de convention n'a été détectée, retirer la question "Convention de code". Si aucun fichier Figma n'a été trouvé, retirer la question "Statut des maquettes". Adapter les labels des options au contenu réel observé (remplacer `<pattern détecté>`, `[Fichier A]`, `<convention A>`, etc.).


### Traitement des réponses

Les réponses sont retournées dans l'ordre des questions posées, sous forme de tableau de labels :
```
["Conserver (Recommandé)", "Connue et acceptée", "<Convention A>", "CI/CD automatisée", "TDD systématique", "À jour — ready-for-dev", "J'ai répondu"]
```

**Règles de traitement :**

| Réponse | Action |
|---------|--------|
| Label prédéfini | Utiliser directement dans le récap de fin de Phase 2 |
| Réponse libre (texte saisi) | Intégrer le texte complet dans le récap |
| "Skip toutes" (dernière question) | Marquer toutes les questions précédentes comme "non répondu" — l'analyse restera partielle |

**Mapping réponses → récap :**

```typescript
// Pseudo-code de traitement
const [architecture, dette, convention, deploiement, tests, figma, skipStatus] = reponses;

// Si l'utilisateur a choisi "Skip toutes"
if (skipStatus === "Skip toutes") {
  recapPhase2.questions.forEach(q => q.reponse = "non répondu");
  recapPhase2.zonesOmbrePersistantes.push("Questions de clarification non traitées");
} else {
  recapPhase2.questions = [
    { question: "Architecture cible", reponse: architecture },
    { question: "Dette technique", reponse: dette },
    { question: "Convention de code", reponse: convention },
    { question: "Processus de déploiement", reponse: deploiement },
    { question: "Stratégie de test", reponse: tests },
    { question: "Statut des maquettes", reponse: figma } // si applicable
  ];
}
```

> **Note :** Adapter le mapping selon les questions effectivement posées (certaines sont conditionnelles à ce qui a été détecté en Phase 1).

### Récap de fin de Phase 2


### Question de validation obligatoire


**Si CONTEXTE = standalone :**
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


**Selon la réponse (dans tous les contextes) :**
- **Passer à Phase 3** → Phase 3
- **Poser d'autres questions** → rester en Phase 2, poser de nouvelles questions, re-produire le récap
- **Revenir à Phase 1** → Phase 1 (les réponses reçues modifient le périmètre d'exploration)

---