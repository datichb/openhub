---
name: orchestrator-ticket-routing
description: Workflow par type de ticket de l'orchestrator — routing détaillé vers les sous-agents selon le type (feature, bug, refactor, migration, audit, doc, design).
---

## Workflow par type de ticket

### Ticket `spec-ux` ou `spec-ui`

1. **Mettre à jour todowrite** — passer le ticket Spec UX ou Spec UI courant en `in_progress` :

   **Exemple Spec UX — ticket #bd-10 démarre :**
   ```
   todowrite({
     todos: [
       { content: "Planification feature", status: "completed", priority: "high" },
       { content: "Spec UX — #bd-10 Analyse flow inscription", status: "in_progress", priority: "high" },
       { content: "Spec UI — #bd-11 Composant formulaire", status: "pending", priority: "high" },
       { content: "Audit sécurité — #bd-13 Auth", status: "pending", priority: "medium" },
       { content: "#bd-12 — Endpoint POST /users", status: "pending", priority: "high" },
       { content: "#bd-14 — Migration DB users", status: "pending", priority: "medium" }
     ]
   })
   ```

   **Exemple Spec UI — #bd-11 démarre (après #bd-10 terminé) :**
   ```
   todowrite({
     todos: [
       { content: "Planification feature", status: "completed", priority: "high" },
       { content: "Spec UX — #bd-10 Analyse flow inscription", status: "completed", priority: "high" },
       { content: "Spec UI — #bd-11 Composant formulaire", status: "in_progress", priority: "high" },
       { content: "Audit sécurité — #bd-13 Auth", status: "pending", priority: "medium" },
       { content: "#bd-12 — Endpoint POST /users", status: "pending", priority: "high" },
       { content: "#bd-14 — Migration DB users", status: "pending", priority: "medium" }
     ]
   })
   ```

2. Annoncer la phase de conception :
   > « Je délègue la spécification à `designer` / `designer` pour le ticket #<ID> — <titre>.
   > Si des questions apparaissent ici, elles viennent de cet agent et incluront leur contexte. »

3. Invoquer l'agent design avec :
   - L'ID du ticket (`bd show <ID>`)
   - Le contexte global de la feature
   - **Le marqueur de contexte d'invocation (obligatoire) :**
     > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session si une clarification critique est nécessaire, et produire le bloc ## Retour vers orchestrator en fin de session.`
   - **Le skill de parcours (obligatoire) :**
     > `[SKILL:designer/ux-subagent]` ou `[SKILL:designer/ui-subagent]` selon l'agent invoqué

4. À la réception du résultat, **détecter le type de retour** :

   **Cas A — retour final :** contient `## Retour vers orchestrator` mais **pas** de `## Question pour l'orchestrator`

   1. **Détecter la présence des blocs `## Retour intermédiaire vers orchestrator`** :
      - **Présents** → les afficher en texte dans l'ordre, AVANT le reste
      - **Absents** → continuer

   2. **Détecter la présence de la spec complète** (user flows, états, wireframes, tokens, critères d'acceptance UX/UI) :
      - **Présente** → continuer la vérification suivante
      - **Absente ou semble résumée** → demander explicitement à l'agent design de produire la spec complète avant de continuer.

   3. **Détecter la présence du bloc `## Retour vers orchestrator`** :
      - **Présent** → continuer
      - **Absent** → demander explicitement à l'agent design de produire le récapitulatif structuré avant de continuer.

   **Cas B — question montante :** contient `## Question pour l'orchestrator`
   → Afficher le `## Retour intermédiaire vers orchestrator` en texte, relayer la question via `question`, ré-invoquer avec `task_id` + réponse + marqueur `[CONTEXTE]`.

> Template de retranscription, checklist et vérifications complets → skill `posture/retranscription-coordinateur`.

**Spécificités design (Cas A) :**
- Sections critiques : `### Contraintes d'implémentation`, `### Points ouverts`
- Signaler les points ouverts avant le CP-spec, transmettre les contraintes à `orchestrator-dev`

---

   Le format attendu, les champs obligatoires et les définitions des statuts sont définis dans le skill `design/design-handoff-format` — s'y référer comme source de vérité.

   > ❌ Ne jamais résumer ni abréger la spec avant de la présenter à l'utilisateur au CP-spec.
   > ❌ Ne jamais accepter un bloc handoff sans la section `### Spec complète` — elle doit être intégrée dans le bloc.

5. [CP-spec] Afficher la spec complète dans le texte de la discussion (ne pas inclure dans l'outil `question`), puis utiliser l'outil `question` :

   ```
   question({
     questions: [{
       header: "CP-spec — Ticket #<ID>",
       question: "Spec <UX/UI> produite pour le ticket #<ID> — <titre>. Quelle suite ?",
       options: [
         { label: "Valider", description: "Transmettre la spec à orchestrator-dev pour implémentation" },
         { label: "Réviser", description: "Retourner à l'agent design avec des corrections" },
         { label: "Ignorer", description: "Abandonner ce ticket et passer au suivant" }
       ]
     }]
   })
   ```

- **Valider** → mettre à jour todowrite (ticket Spec UX/UI courant → `completed`), puis transmettre la spec validée **et les `### Contraintes d'implémentation`** à `orchestrator-dev` pour implémentation
- **Réviser** → retourner à l'agent design avec les corrections, incrémenter le compteur de révisions, nouveau CP-spec

  **Compteur de révisions :** maintenir un compteur interne par ticket spec.
  Après **3 révisions sans validation**, ne pas relancer l'agent — utiliser l'outil `question` à la place :

  ```
  question({
    questions: [{
      header: "3 révisions sans validation",
      question: "Le ticket #<ID> a subi 3 révisions sans validation. Comment procéder ?",
      options: [
        { label: "Continuer", description: "Relancer une nouvelle révision avec l'agent design" },
        { label: "Valider en l'état", description: "Accepter la spec actuelle et passer à l'implémentation" },
        { label: "Ignorer", description: "Abandonner ce ticket" }
      ]
    }]
  })
  ```

- **Ignorer** → noter le ticket comme ignoré, passer au suivant

---

### Ticket `audit`

1. **Mettre à jour todowrite** — passer le ticket audit courant en `in_progress` :

   ```
   todowrite({
     todos: [
       { content: "Planification feature", status: "completed", priority: "high" },
       { content: "Spec UX — #bd-10 <titre>", status: "completed", priority: "high" },
       { content: "Spec UI — #bd-11 <titre>", status: "completed", priority: "high" },
       { content: "Audit sécurité — #bd-13 <titre>", status: "in_progress", priority: "medium" },
       { content: "#bd-12 — <titre>", status: "pending", priority: "high" },
       { content: "#bd-14 — <titre>", status: "pending", priority: "medium" }
     ]
   })
   ```

   > Omettre les phases Spec UX/UI si absentes de la feature. La liste doit refléter exactement les tickets présents.

2. Annoncer la phase d'audit :
   > « Je délègue l'audit à `auditor` pour le ticket #<ID> — <titre>.
   > Si des questions apparaissent ici, elles viennent de cet agent et incluront leur contexte. »

3. Invoquer l'agent auditeur avec :
   - L'ID du ticket (`bd show <ID>`)
   - Le périmètre à auditer
   - **Le marqueur de contexte d'invocation (obligatoire) :**
     > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session à chaque fin de phase et produire le bloc ## Retour vers orchestrator en fin de session.`
   - **Le skill de parcours (obligatoire) :**
     > `[SKILL:auditor/auditor-subagent]`

4. À la réception du résultat, **détecter le type de retour** :

   **Cas A — retour final :** contient `## Retour vers orchestrator` mais **pas** de `## Question pour l'orchestrator`

   1. **Détecter la présence des blocs `## Retour intermédiaire vers orchestrator`** :
      - **Présents** → les afficher en texte dans l'ordre, AVANT le reste
      - **Absents** → continuer

   2. **Détecter la présence du rapport d'audit complet** (analyse narrative, observations item par item, preuves) :
      - **Présent** → continuer la vérification suivante
      - **Absent** → demander explicitement à l'agent auditeur de produire le rapport complet avant de continuer.

   3. **Détecter la présence du bloc `## Retour vers orchestrator`** :
      - **Présent** → continuer
      - **Absent** → demander explicitement à l'agent auditeur de produire le récapitulatif structuré avant de continuer.

   **Cas B — question montante :** contient `## Question pour l'orchestrator`
   → Afficher le `## Retour intermédiaire vers orchestrator` en texte, relayer la question via `question`, ré-invoquer avec `task_id` + réponse + marqueur `[CONTEXTE]`.

> Template de retranscription, checklist et vérifications complets → skill `posture/retranscription-coordinateur`.

**Spécificités auditor (Cas A) :**
- Sections critiques : `### Périmètre audité`, `### Synthèse des problèmes identifiés`, `### Risque résiduel si non corrigé`
- Adapter la question CP-audit selon le `### Statut` (corrections-requises / acceptable / bloquant)

---

   Le format attendu, les champs obligatoires et les définitions des statuts sont définis dans le skill `auditor/audit-handoff-format` — s'y référer comme source de vérité.

   > ❌ Ne jamais résumer ni filtrer le rapport avant de le présenter à l'utilisateur au CP-audit.
   > ❌ Ne jamais accepter un bloc handoff sans la section `### Rapport d'audit complet` — elle doit être intégrée dans le bloc.

5. [CP-audit] Afficher le rapport d'audit complet dans le texte de la discussion (ne pas inclure dans l'outil `question`), puis utiliser l'outil `question` :

   ```
   question({
     questions: [{
       header: "CP-audit — Ticket #<ID>",
       question: "Rapport d'audit reçu pour le ticket #<ID> — <titre>. Quelle suite ?",
       options: [
         { label: "Corriger", description: "Transmettre le rapport à orchestrator-dev pour corrections" },
         { label: "Accepter", description: "Aucune correction nécessaire — ticket audité" },
         { label: "Ignorer", description: "Abandonner ce ticket" }
       ]
     }]
   })
   ```

- **Corriger** → transmettre les `### Recommandations priorisées` **intégralement** à `orchestrator-dev` pour corrections

  Quand `orchestrator-dev` retourne son récap de corrections, utiliser l'outil `question` :

  ```
  question({
    questions: [{
      header: "Re-audit",
      question: "Corrections appliquées pour le ticket #<ID>. Relancer l'audit pour vérifier ?",
      options: [
        { label: "Oui — relancer l'audit", description: "Invoquer à nouveau l'auditeur sur le même périmètre" },
        { label: "Non", description: "Considérer le ticket corrigé sans re-vérification" }
      ]
    }]
  })
  ```

  ❌ Ne jamais déclencher le re-audit automatiquement — toujours attendre la réponse.

  **Compteur de re-audits :** maintenir un compteur interne par ticket audit.
  Après **2 re-audits sans validation**, ne pas relancer l'auditeur — utiliser l'outil `question` à la place :

  ```
  question({
    questions: [{
      header: "2 re-audits sans validation",
      question: "Le ticket #<ID> a subi 2 re-audits sans atteindre le statut acceptable. Comment procéder ?",
      options: [
        { label: "Continuer", description: "Relancer un nouveau cycle correction + re-audit" },
        { label: "Accepter en l'état", description: "Considérer les corrections suffisantes sans nouvelle vérification" },
        { label: "Ignorer", description: "Abandonner ce ticket" }
      ]
    }]
  })
  ```

- **Accepter** → mettre à jour todowrite (ticket audit courant → `completed`), noter le ticket comme audité sans corrections nécessaires
- **Ignorer** → mettre à jour todowrite (ticket audit courant → `cancelled`), noter le ticket comme ignoré

---

### Ticket `dev` (ou phase d'implémentation après spec/audit)

1. **Mettre à jour todowrite** — passer le(s) premier(s) ticket(s) dev en `in_progress` :

   ```
   todowrite({
     todos: [
       { content: "Planification feature", status: "completed", priority: "high" },
       { content: "Spec UX — #bd-10 <titre>", status: "completed", priority: "high" },
       { content: "Spec UI — #bd-11 <titre>", status: "completed", priority: "high" },
       { content: "Audit sécurité — #bd-13 <titre>", status: "completed", priority: "medium" },
       { content: "#bd-12 — Endpoint POST /users", status: "in_progress", priority: "high" },
       { content: "#bd-14 — Migration DB users", status: "pending", priority: "medium" }
     ]
   })
   ```

   > Adapter selon les phases réellement présentes. **La liste reste visible pour l'utilisateur pendant toute la durée de l'implémentation** — l'orchestrator est le seul responsable de sa mise à jour car orchestrator-dev s'exécute dans une session isolée.

2. Annoncer la délégation :
   > « Je délègue l'implémentation à `orchestrator-dev` pour les tickets : <liste des IDs>.
   > Si des questions apparaissent ici pendant l'implémentation, elles viennent d'`orchestrator-dev` ou de ses sous-agents et incluront leur contexte. »

2. Invoquer orchestrator-dev en transmettant :
   - La liste des tickets à implémenter
   - Le mode de workflow choisi en CP-0
   - Le contexte complet : specs UX/UI validées (champ `### Spec produite`) + contraintes d'implémentation (champ `### Contraintes d'implémentation`) + rapports d'audit (champ `### Recommandations priorisées`) si applicable — transmettre intégralement, sans résumer
   - Les tickets portant le label `tdd` (déjà identifiés au CP-0)
   - **Le mode de workflow sous sa forme canonique** : `manuel`, `semi-auto` ou `auto` — ne jamais transmettre le label brut de l'option d'interface (ex : `"Manuel (Recommandé)"`)

> **Vérification obligatoire avant d'invoquer `task(orchestrator-dev)` :**
> « Le prompt contient-il le mode de workflow sous forme canonique (`manuel`, `semi-auto` ou `auto`) ? Si non, l'ajouter avant d'invoquer. »

   - **Le marqueur de contexte d'invocation (obligatoire) :**
     > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois produire le bloc ## Retour vers orchestrator à la fin de ta session — sans exception, même en cas de stop, de ticket bloqué ou de session partielle.`
   - **Le skill de parcours (obligatoire) :**
     > `[SKILL:orchestrator/orchestrator-dev-subagent]`

3. orchestrator-dev pilote l'implémentation complète (developer-* → review).

4. À la réception du résultat de l'invocation, **détecter le type de retour** :

   **Cas A — retour normal** : le résultat contient `## Retour vers orchestrator` mais **pas** de bloc `## Question pour l'orchestrator` (signal que le récap est **final**)
   → **Retranscrire les champs du bloc `## Retour vers orchestrator` de manière formatée** dans le fil de discussion (voir skill `retranscription-coordinateur`) — afficher `### Détail par ticket`, `### Contexte et décisions par ticket`, `### Points d'attention globaux`. Ne jamais résumer ni omettre. Ce contenu doit être visible avant le CP-feature.
   → Lire le bloc structuré `## Retour vers orchestrator`. Le format attendu, les champs obligatoires et les définitions des statuts (`succès`, `partiel`, `bloqué`) sont définis dans le skill `orchestrator-handoff-format` — s'y référer pour le contrat exact.
    > Si le bloc structuré ne contient pas les champs requis, les demander explicitement à orchestrator-dev avant de continuer.

    → **Valider le gate de complétion** (voir section ci-dessous) avant de construire le CP-feature.

    **Cas B — question montante** : le résultat contient `## Question pour l'orchestrator`
   → Voir section ci-dessous.

---

### Validation du gate de complétion (obligatoire avant CP-feature)

Avant de construire le CP-feature, vérifier que la documentation du gate de complétion
est présente dans le rapport final d'orchestrator-dev.

**Indicateurs attendus** (dans `### Points d'attention` ou le récap global) :
- Mention des tests passés — ou justification explicite si aucun test sur le périmètre
- Mention du comportement observable conforme aux specs
- Mention de l'absence de régressions connues — ou leur documentation explicite

**Si le gate est absent ou incomplet → ne pas construire le CP-feature :**

```
question({
  questions: [{
    header: "Gate de complétion absent",
    question: "[Orchestrator — Validation gate | Feature : <nom>]\nLe rapport final d'orchestrator-dev ne documente pas le gate de complétion (3 checks : tests, comportement observable, régressions).\n\nComment procéder ?",
    options: [
      { label: "Redemander à orchestrator-dev (Recommandé)", description: "Invoquer orchestrator-dev avec task_id pour compléter le rapport" },
      { label: "Accepter et continuer", description: "Considérer le gate comme implicitement passé — risque : régressions non détectées" },
      { label: "Stop", description: "Arrêter le workflow — investigation manuelle requise" }
    ]
  }]
})
```

**Si "Redemander à orchestrator-dev"** → invoquer `task(orchestrator-dev)` avec le `task_id`
de la session précédente en précisant :
> « Le rapport final doit documenter explicitement le gate de complétion : tests passés,
> comportement observable conforme, régressions documentées ou absence justifiée. »

❌ Ne jamais construire le CP-feature sans gate documenté — sauf choix explicite "Accepter et continuer".

---

### Réception d'une question montante depuis orchestrator-dev

Quand orchestrator-dev atteint un CP à enjeu fort (CP-2, blocage 3 cycles, dépendance non résolue, ticket bloqué), il arrête sa session et remonte un bloc `## Question pour l'orchestrator`.

> ⚠️ **RAPPEL IMPÉRATIF** : Tu DOIS produire du texte de réponse (rapport, contexte, état de session) AVANT d'appeler l'outil `question`. Ne jamais appeler `question` comme première action — toujours afficher d'abord le contenu dans la discussion.

**Mise à jour todowrite à la réception de chaque bloc :**

Lire le champ `### Phase` du bloc `## Question pour l'orchestrator` pour déterminer la mise à jour :

| Phase dans le bloc | Action todowrite |
|--------------------|-----------------|
| `CP-1` (ticket sur le point de démarrer) | Ticket `#bd-XX` → `in_progress` |
| `CP-2` (ticket déjà en cours) | Aucune mise à jour (ticket déjà `in_progress`) |
| `CP-3` avec option `passer` choisie | Ticket `#bd-XX` → `cancelled` |
| Récap partiel après CP-2 commit validé | Ticket `#bd-XX` commité → `completed`, prochain ticket → `in_progress` |

> La mise à jour todowrite est effectuée **avant** d'afficher le rapport et **avant** d'appeler l'outil `question`.

**Exemple — CP-1 reçu pour #bd-14 (deuxième ticket) :**

```
todowrite({
  todos: [
    { content: "Planification feature", status: "completed", priority: "high" },
    { content: "#bd-12 — Endpoint POST /users", status: "completed", priority: "high" },
    { content: "#bd-14 — Migration DB users", status: "in_progress", priority: "medium" }
  ]
})
```

**Exemple — CP-2 commit validé pour #bd-12, CP-3 enchaîne sur #bd-14 :**

```
todowrite({
  todos: [
    { content: "Planification feature", status: "completed", priority: "high" },
    { content: "#bd-12 — Endpoint POST /users", status: "completed", priority: "high" },
    { content: "#bd-14 — Migration DB users", status: "in_progress", priority: "medium" }
  ]
})

**Comportement obligatoire :**

1. **Pour un CP-2 (rapport de review) : afficher le `### Rapport de review complet` dans le fil de conversation** avant toute autre action — l'utilisateur doit lire le rapport avant de prendre sa décision.
   - Si la section `### Rapport de review complet` est absente ou semble résumée → demander explicitement à orchestrator-dev de retransmettre le rapport intégral avant de continuer.
   - Ne jamais poser la question au CP-2 sans avoir d'abord affiché le rapport complet.

2. **Afficher le bloc `### Contexte complet` intégralement** dans la discussion — ne jamais résumer ni abréger.

3. **Poser la question à l'utilisateur** via l'outil `question`, en reprenant exactement la question et les options du bloc :

   ```
   question({
     questions: [{
       header: "[OrchestratorDev] <Phase> — #<ID>",
       question: "[OrchestratorDev — <Phase> | Ticket #<ID> — <titre>]\n<question exacte du bloc>",
       options: [
         { label: "<label-option-1>", description: "<description du bloc>" },
         { label: "<label-option-2>", description: "<description du bloc>" }
       ]
     }]
   })
   ```

4. **Ré-invoquer orchestrator-dev avec `task_id`** (valeur dans le bloc `### État de la session`) en transmettant la réponse :

   > **Transmission du mode obligatoire :** inclure toujours le mode de workflow sous sa forme canonique (`manuel`, `semi-auto` ou `auto`) dans chaque prompt de reprise — le mode n'est pas garanti d'être persisté dans la session `task_id`.

   ```
   Task(
     subagent_type: "orchestrator-dev",
     task_id: "<task_id du bloc>",
     prompt: "Réponse de l'utilisateur au CP <phase> pour le ticket #<ID> : <réponse choisie>. Mode de workflow : <valeur canonique — manuel|semi-auto|auto>. Reprendre depuis l'étape correspondante."
   )
   ```

5. **Attendre le nouveau résultat** et recommencer la détection (Cas A ou Cas B).

**Cas C — session introuvable (redémarrage d'OpenCode) :** si la ré-invocation avec `task_id` ne produit pas de résultat ou retourne une erreur indiquant que la session n'existe plus :

> ⚠️ La session `orchestrator-dev` (task_id: `<task_id>`) est introuvable — OpenCode a probablement redémarré pendant la fenêtre d'attente.

Utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Session perdue — #<ID>",
    question: "[Orchestrator — Session introuvable | Ticket #<ID> — <titre>]\nLa session orchestrator-dev a été perdue (redémarrage probable). Comment reprendre ?",
    options: [
      { label: "Relancer depuis les tickets restants (Recommandé)", description: "Invoquer une nouvelle session orchestrator-dev avec les tickets non encore traités (liste dans ### État de la session)" },
      { label: "Stop", description: "Arrêter le workflow et afficher le récap de l'état courant connu" }
    ]
  }]
})
```

- **Relancer** → invoquer `task(orchestrator-dev)` **sans `task_id`** (nouvelle session) en transmettant uniquement les tickets listés dans `**Tickets restants :**` du bloc `### État de la session` reçu, plus le mode et le marqueur `[CONTEXTE]`
- **Stop** → construire le CP-feature à partir des informations disponibles dans le dernier `### État de la session` reçu, en marquant les tickets restants comme `⏸️ Non traités — session interrompue`

> ❌ Ne jamais construire une réponse à la place de l'utilisateur.
> ❌ Ne jamais ignorer le bloc — toute question montante doit être traitée avant de continuer.
> ❌ Pour un CP-2 : ne jamais poser la question sans avoir affiché le rapport de review complet au préalable.
