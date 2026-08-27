---
name: orchestrator-recap-edge
description: Récap global de feature et gestion des cas particuliers de l'orchestrator.
---

## CP-feature — Récap global

Afficher en fin de feature (tous les tickets traités ou après un **stop**).

**Mettre à jour todowrite** — tous les tickets passent à leur statut final :

```
todowrite({
  todos: [
    { content: "Planification feature", status: "completed", priority: "high" },
    // Spec UX — une tâche par ticket avec statut final :
    { content: "Spec UX — #bd-10 <titre>", status: "completed", priority: "high" },
    // Spec UI — une tâche par ticket avec statut final :
    { content: "Spec UI — #bd-11 <titre>", status: "completed", priority: "high" },
    // Audit — une tâche par ticket avec statut final :
    { content: "Audit sécurité — #bd-13 <titre>", status: "completed", priority: "medium" },
    // Tickets dev — une tâche par ticket avec statut final :
    { content: "#bd-12 — Endpoint POST /users", status: "completed", priority: "high" },
    { content: "#bd-14 — Migration DB users", status: "completed", priority: "medium" }
  ]
})
```

> Utiliser `cancelled` pour les tickets ignorés ou abandonnés. Adapter selon les tickets réellement présents dans la feature.

### Gate de complétion — Avant le récap feature

Avant de construire le récap feature, passer les 3 checks suivants :

| Check | Vérification | Si non |
|-------|-------------|--------|
| 1 — Tests | Le récap orchestrator-dev indique que les tests passent pour tous les tickets traités | Signaler dans `### Points d'attention` |
| 2 — Spec conforme | Chaque ticket dev indique `### Critères d'acceptance couverts : tous couverts` | Lister les critères non couverts dans `### Points d'attention` |
| 3 — Pas de régression | Aucun ticket n'indique de régression non documentée | Documenter les régressions connues dans `### Points d'attention` |

> Un check en échec ne bloque pas le récap — il est documenté dans `### Points d'attention`. Signaler silencieusement = interdit.

### Review adversariale CP-feature (obligatoire)

Après le gate de complétion et **avant** de construire le récap feature, **invoquer une review adversariale** sur l'ensemble de la feature :

1. **Lancer la review adversariale** sur le diff total de la feature :
   ```
   task(subagent_type: "reviewer", prompt: "[SKILL:reviewer/reviewer-subagent] [MODE:adversarial] Review adversariale CP-feature.\nBranche feature : <feature-branch>\nDiff : git diff main..<feature-branch>\nFeature : <nom de la feature>\nTickets traités : <liste des IDs>")
   ```

2. **Proposer l'option edge-case** à l'utilisateur :
   ```
   question({
     questions: [{
       header: "Review edge-case",
       question: "[Orchestrator — CP-feature | Feature : <nom>]\nLa review adversariale est lancée. Veux-tu aussi une analyse edge-case (chemins d'exécution non gérés) ?",
       options: [
         { label: "Non (Recommandé)", description: "Review adversariale seule — suffisante pour la plupart des features" },
         { label: "Oui", description: "Ajoute une analyse edge-case en parallèle — recommandé pour les features critiques (auth, paiement, données sensibles)" }
       ]
     }]
   })
   ```

3. **Si edge-case demandé** → invoquer le reviewer avec le mode combiné :
   ```
   task(subagent_type: "reviewer", prompt: "[SKILL:reviewer/reviewer-subagent] [MODE:adversarial+edge-case] Review adversariale + edge-case CP-feature.\nBranche feature : <feature-branch>\nDiff : git diff main..<feature-branch>\nFeature : <nom de la feature>")
   ```
   > Note : si la review adversariale seule a déjà été lancée (question posée après le lancement), utiliser le résultat de la review adversariale et lancer une session edge-case additionnelle, puis fusionner les deux résultats.

4. **À la réception du rapport** :
   - Afficher le rapport adversarial (ou unifié) intégralement dans la discussion
   - Intégrer les findings adversariaux dans le `### Points d'attention` du récap feature
   - Si le verdict est `corriger` ou `corriger-sécurité` → proposer un CP de correction avant de clore la feature

> La review adversariale CP-feature est **indépendante** des reviews standard effectuées ticket par ticket. Elle analyse la cohérence globale, les interactions entre composants, et les problèmes émergents à l'échelle de la feature.

---

**Avant de construire ce récap**, vérifier que le récap global d'orchestrator-dev a bien été affiché lors de la réception du retour final (Cas A — il ne doit pas être reproduit ici une seconde fois). Puis produire le récap consolidé ci-dessous à partir du bloc structuré `## Retour vers orchestrator` :

```
## Récap feature — <nom de la feature>

### Vue d'ensemble

| ID | Titre | Phase(s) | Agent(s) | Statut |
|----|-------|----------|---------|--------|
| bd-10 | ... | Spec UX | designer | ✅ Spec validée |
| bd-11 | ... | Spec UI → Impl | designer → dev | ✅ Terminé |
| bd-12 | ... | Impl | orchestrator-dev | ✅ Terminé |
| bd-13 | ... | Audit → Impl | auditor → dev | ✅ Corrigé |

### Résumé
- **Tickets traités :** X / Y
- **Tickets ignorés :** Z
- **Phases de conception :** N specs validées
- **Audits réalisés :** M rapports (K avec corrections)

### Points d'attention
<Points soulevés en audit ou review qui méritent un suivi>

### Review adversariale CP-feature
- **Score de confiance :** XX/10
- **Findings critiques/majeurs :** <résumé des problèmes identifiés par la review adversariale>
- **Hypothèses dangereuses :** <résumé si applicable>
- **Edge-case (si activé) :** <résumé des chemins non gérés identifiés>

### Prochaines étapes suggérées
<Ce qui reste si des tickets ont été ignorés ou des blocages signalés>
```

---

## Gestion des cas particuliers

### Ticket mixte (spec + dev dans le même ticket)

Ce cas est détecté par le planner, pas par l'agent orchestrator. Si le planner signale un ticket mixte
dans son retour, utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Ticket mixte #<ID>",
    question: "Le planner a identifié que le ticket #<ID> couvre à la fois une phase de conception et une phase d'implémentation. Comment procéder ?",
    options: [
      { label: "Scinder via le planner (Recommandé)", description: "Demander au planner de créer deux tickets : Spec <UX/UI> et Implémentation" },
      { label: "Traiter comme indiqué par le planner", description: "Utiliser l'agent prévu par le planner tel quel" }
    ]
  }]
})
```

### Agent prévu non spécifié par le planner

Si le retour du planner ne contient pas le champ `Agent prévu` pour un ticket, demander
explicitement au planner de compléter l'information avant de continuer.

> ❌ Ne jamais tenter de déterminer l'agent soi-même en analysant le ticket

---

### Agent requis non disponible

Quand un agent identifié pour un ticket n'est pas déployé dans le projet (invocation refusée
ou agent absent de `.opencode/agents/`), ne jamais silencieusement basculer vers un autre agent.

**Référence — table de substitution :**

| Agent manquant | Substitut proposé | Limitation |
|----------------|-------------------|------------|
| `auditor-security` | `developer` (domaine security) | Pas de rapport structuré OWASP — analyse ad hoc uniquement |
| `auditor-accessibility` | `developer` (domaine frontend) | Pas de rapport WCAG/RGAA — vérifications basiques uniquement |
| `auditor-architecture` | `developer` (domaine fullstack) | Pas d'analyse SOLID/couplage structurée — revue partielle |
| `auditor-performance` | `developer` (domaine fullstack) | Pas de rapport Web Vitals/N+1 — analyse ad hoc |
| `auditor-privacy` | *(aucun substitut)* | — |
| `auditor-ecodesign` | *(aucun substitut)* | — |
| `auditor-observability` | *(aucun substitut)* | — |
| `designer` | *(aucun substitut)* | — |
| `designer` | *(aucun substitut)* | — |

> **Note :** L'agent `developer` listé comme substitut est invoqué **via `orchestrator-dev`** avec le domaine approprié, jamais directement par l'orchestrator. L'orchestrator délègue à `orchestrator-dev` qui route ensuite vers `developer` avec le bon domaine.

**Si un substitut existe**, utiliser l'outil `question` avec 3 options :

```
question({
  questions: [{
    header: "Agent manquant — #<ID>",
    question: "[Orchestrator — Routing | Ticket #<ID> — <titre>]\nL'agent `<agent-id>` est requis mais n'est pas déployé sur ce projet. Comment procéder ?",
    options: [
      { label: "Déployer l'agent (Recommandé)", description: "Tape `!oc deploy opencode <PROJECT_ID>` ici pour déployer sans quitter OpenCode, puis réponds pour reprendre" },
      { label: "Utiliser <substitut>", description: "<Limitation de couverture>" },
      { label: "Ignorer ce ticket", description: "Passer au ticket suivant — noté comme ignoré dans le récap" }
    ]
  }]
})
```

**Si aucun substitut n'existe**, utiliser l'outil `question` avec 2 options :

```
question({
  questions: [{
    header: "Agent manquant — #<ID>",
    question: "[Orchestrator — Routing | Ticket #<ID> — <titre>]\nL'agent `<agent-id>` est requis mais n'est pas déployé sur ce projet, et aucun substitut n'est disponible. Comment procéder ?",
    options: [
      { label: "Déployer l'agent (Recommandé)", description: "Tape `!oc deploy opencode <PROJECT_ID>` ici pour déployer sans quitter OpenCode, puis réponds pour reprendre" },
      { label: "Ignorer ce ticket", description: "Passer au ticket suivant — noté comme ignoré dans le récap" }
    ]
  }]
})
```

**Comportement selon le choix :**

- **Déployer l'agent** → afficher le bloc d'instructions, puis attendre la confirmation avant de reprendre :

  > Pour déployer `<agent-id>` sans quitter OpenCode :
  > 1. Tape `!oc deploy opencode <PROJECT_ID>` dans ce chat
  > 2. Réponds ici une fois le déploiement terminé pour reprendre le workflow

- **Utiliser le substitut** → router vers l'agent de substitution via `orchestrator-dev` en signalant
  explicitement la limitation dans le compte rendu d'étape et dans le récap global CP-feature
- **Ignorer** → noter le ticket comme ignoré, continuer avec le suivant
