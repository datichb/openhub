---
name: orchestrator-recap-edge
description: "Utiliser en fin de feature ou sur un cas d'erreur : review adversariale finale, récap de feature, cas particuliers."
---

## Récap global de la feature

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

### Review adversariale de fin de feature (obligatoire)

Après le gate de complétion et **avant** de construire le récap feature, **invoquer une review adversariale** sur l'ensemble de la feature :

1. **Lancer la review adversariale** sur le diff total de la feature :
   ```
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] Review adversariale de fin de feature.\nBranche feature : <branche de travail>\nDiff : git diff <base>...HEAD\nFeature : <nom de la feature>\nTickets traités : <liste des IDs>")
   ```

2. **Proposer l'option edge-case** à l'utilisateur :
   ```
   question({
     questions: [{
       header: "Review edge-case",
       question: "[Orchestrator — Review edge-case | Feature : <nom>]\nLa review adversariale est lancée. Veux-tu aussi une analyse edge-case (chemins d'exécution non gérés) ?",
       options: [
         { label: "Non (Recommandé)", description: "Review adversariale seule — suffisante pour la plupart des features" },
         { label: "Oui", description: "Ajoute une analyse edge-case en parallèle — recommandé pour les features critiques (auth, paiement, données sensibles)" }
       ]
     }]
   })
   ```

3. **Si edge-case demandé** → invoquer le reviewer avec le mode combiné :
   ```
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial+edge-case] Review adversariale + edge-case de fin de feature.\nBranche feature : <branche de travail>\nDiff : git diff <base>...HEAD\nFeature : <nom de la feature>")
   ```
   > Note : si la review adversariale seule a déjà été lancée (question posée après le lancement), utiliser le résultat de la review adversariale et lancer une session edge-case additionnelle, puis fusionner les deux résultats.

4. **À la réception du rapport** :
   - Afficher le rapport adversarial (ou unifié) intégralement dans la discussion
   - Intégrer les findings adversariaux dans le `### Points d'attention` du récap feature
   - Si le verdict est `corriger` ou `corriger-sécurité` → rappeler `orchestrator-dev` avec les corrections (un ticket de correction par sujet) : chaque correction repasse par son propre `cp-2` (redemandé par orchestrator-dev), puis le récap

> `<branche de travail>` : ligne `Branche de travail` du premier message, sinon la branche courante ; `<base>` : la branche de départ de la feature si elle est connue (retour d'orchestrator-dev, demande de l'utilisateur), sinon écrire `Base : à déterminer` et laisser le reviewer la trouver (branche par défaut du dépôt) — jamais `main` en dur.
>
> La review adversariale de fin de feature est **indépendante** des reviews standard effectuées ticket par ticket. Elle analyse la cohérence globale, les interactions entre composants, et les problèmes émergents à l'échelle de la feature.

---

**Avant de construire ce récap**, vérifier que l'enveloppe finale d'orchestrator-dev a bien été affichée à sa réception (son rapport n'est pas reproduit ici une seconde fois). Puis produire le récap consolidé ci-dessous à partir de l'enveloppe et du rapport d'orchestrator-dev (`workflow_report_get`) :

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

### Review adversariale de fin de feature
- **Score de confiance :** XX/10
- **Findings critiques/majeurs :** <résumé des problèmes identifiés par la review adversariale>
- **Hypothèses dangereuses :** <résumé si applicable>
- **Edge-case (si activé) :** <résumé des chemins non gérés identifiés>

### Bloc rétrospectif (si >= 3 tickets traités)
*(Omettre cette section si la feature a < 3 tickets — pas assez de données pour une retro.)*

**Métriques :**
- Cycles de review : <total, min, max, moyenne par ticket>
- Findings reviewer : <total 🔴, total 🟠, total 🟡 — toutes reviews confondues>
- Tickets avec drift : <nombre — raisons principales>

**Patterns récurrents :**
- <pattern 1 : ex "3 tickets sur 5 ont eu des findings 🟠 sur la gestion d'erreur — conventions insuffisantes ?">
- <pattern 2 : ex "Le drift a été détecté sur les 2 tickets touchant le module auth — architecture fragile ?">
*(Si aucun pattern notable : "Aucun pattern récurrent identifié.")*

**Recommandations pour l'équipe :**
- <reco 1 : ex "Renforcer la couverture des cas d'erreur dans les conventions projet">
- <reco 2 : ex "Le module auth nécessite un audit d'architecture dédié">
*(Si rien à recommander : "Aucune recommandation — la feature s'est déroulée sans friction notable.")*

> **Capitalisation :** Ce bloc se propose comme enrichissement wiki vers `docs/wiki/technical/retro-log.md`
> avec `wiki_propose` (mécanisme `living-docs-enrichment` standard). L'écriture wiki est un bonus —
> sans l'outil, le bloc reste visible dans la discussion.

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

Quand un agent identifié pour un ticket n'est pas disponible dans la session (invocation refusée :
il ne fait pas partie du workflow lancé), ne jamais silencieusement basculer vers un autre agent.

**Référence — table de substitution :**

| Agent manquant | Substitut proposé | Limitation |
|----------------|-------------------|------------|
| `auditor` (domaine security) | `developer` (domaine security) | Pas de rapport structuré OWASP — analyse ad hoc uniquement |
| `auditor` (domaine accessibility) | `developer` (domaine frontend) | Pas de rapport WCAG/RGAA — vérifications basiques uniquement |
| `auditor` (domaine architecture) | `developer` (domaine fullstack) | Pas d'analyse SOLID/couplage structurée — revue partielle |
| `auditor` (domaine performance) | `developer` (domaine fullstack) | Pas de rapport Web Vitals/N+1 — analyse ad hoc |
| `auditor` (domaine privacy) | *(aucun substitut)* | — |
| `auditor` (domaine ecodesign) | *(aucun substitut)* | — |
| `auditor` (domaine observability) | *(aucun substitut)* | — |
| `designer` | *(aucun substitut)* | — |

> **Note :** L'agent `developer` listé comme substitut est invoqué **via `orchestrator-dev`** avec le domaine approprié, jamais directement par l'orchestrator. L'orchestrator délègue à `orchestrator-dev` qui route ensuite vers `developer` avec le bon domaine.

**Si un substitut existe**, utiliser l'outil `question` avec 3 options :

```
question({
  questions: [{
    header: "Agent manquant — #<ID>",
    question: "[Orchestrator — Routing | Ticket #<ID> — <titre>]\nL'agent `<agent-id>` est requis mais ne fait pas partie de ce workflow. Comment procéder ?",
    options: [
      { label: "Ajouter l'agent au workflow (Recommandé)", description: "Ajouter l'agent au workflow (oh workflow edit), puis relancer la session : il n'est pas disponible dans celle-ci" },
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
    question: "[Orchestrator — Routing | Ticket #<ID> — <titre>]\nL'agent `<agent-id>` est requis mais ne fait pas partie de ce workflow, et aucun substitut n'est disponible. Comment procéder ?",
    options: [
      { label: "Ajouter l'agent au workflow (Recommandé)", description: "Ajouter l'agent au workflow (oh workflow edit), puis relancer la session : il n'est pas disponible dans celle-ci" },
      { label: "Ignorer ce ticket", description: "Passer au ticket suivant — noté comme ignoré dans le récap" }
    ]
  }]
})
```

**Comportement selon le choix :**

- **Ajouter l'agent au workflow** → afficher le bloc d'instructions, noter le ticket comme en attente
  dans le récap, puis continuer avec les tickets suivants :

  > `<agent-id>` n'est pas dans le paquet de cette session :
  > 1. Ajoute-le au workflow (`oh workflow edit <workflow>`, section `agents`) ou lance un workflow qui le contient
  > 2. Relance une session pour ce ticket (`oh run <workflow> --tickets <ID>`)

- **Utiliser le substitut** → router vers l'agent de substitution via `orchestrator-dev` en signalant
  explicitement la limitation dans le compte rendu d'étape et dans le récap global de la feature
- **Ignorer** → noter le ticket comme ignoré, continuer avec le suivant
