---
name: orchestrator-dev-ticket-workflow
description: Workflow ticket par ticket de l'orchestrator-dev — étapes 1a à 6 (présentation, branche, délégation, pre-review, review, décision, compte rendu).
---

## Workflow ticket par ticket

### Étape 1a — Présentation du ticket + CP-1

Afficher le ticket :

```
## Ticket #<ID> — <titre>

**Priorité :** P<PRIORITE> | **Type :** <type> | **Agent :** <developer-xxx>

**Description :**
<description du ticket>

**Critères d'acceptance :**
<liste des critères>

**Notes :**
<notes et contraintes>

---
```

**Selon le mode :**

- **`manuel`** → pause CP-1 :

  **Si CONTEXTE = orchestrator_feature (mode `manuel`) :**

  Produire dans cet ordre et terminer la session :

  ````markdown
  ## Question pour l'orchestrator

  **Agent :** orchestrator-dev
  **Ticket :** #<ID> — <titre>
  **Phase :** CP-1

  ### Contexte
  Prêt à démarrer l'implémentation du ticket #<ID> — <titre>.
  <Description courte du ticket issue de bd show>

  ### Question en attente
  Démarrer l'implémentation du ticket #<ID> — <titre> ?

  ### Options disponibles
  - `demarrer` — Déléguer l'implémentation à <developer-xxx>
  - `voir-detail` — Afficher le contenu complet du ticket (bd show <ID>)
  - `passer` — Ignorer ce ticket et passer au suivant
  - `stop` — Arrêter le workflow

  ### État de la session
  **Tickets traités :** [bd-XX ✅, ...]
  **En cours :** bd-<ID>
  **Tickets restants :** [bd-YY, bd-ZZ, ...]
  **task_id :** <task_id de la session en cours>
  ````

  Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

  → **TERMINER LA SESSION**

  **Instruction de reprise :** "Réponse CP-1 ticket #<ID> : [option choisie]. Reprendre depuis CP-1."

  **Sinon (mode `manuel` standalone)** → pause CP-1 via l'outil `question` :

  ```
  question({
    questions: [{
      header: "CP-1 — Ticket #<ID>",
      question: "Démarrer l'implémentation du ticket #<ID> — <titre> ?",
      options: [
        { label: "Oui — démarrer", description: "Déléguer l'implémentation à <developer-xxx>" },
        { label: "Voir le détail", description: "Afficher le contenu complet du ticket via bd show <ID>" },
        { label: "Passer", description: "Ignorer ce ticket et passer au suivant" },
        { label: "Stop", description: "Arrêter le workflow et afficher le récap de l'état courant" }
      ]
    }]
  })
  ```
  - **Oui — démarrer** → mettre à jour todowrite (ticket en `in_progress`) → passer à l'étape 1b
  - **Voir le détail** → exécuter `bd show <ID>`, afficher la sortie intégrale, puis re-poser CP-1 (boucle — l'utilisateur peut demander le détail autant de fois que nécessaire)
  - **Passer** → mettre à jour todowrite (ticket en `cancelled`) → ticket ignoré, ticket suivant
  - **Stop** → aller directement à la section **Récap global — Fin de session** (afficher le récap et produire le bloc `## Retour vers orchestrator` si CONTEXTE = orchestrator_feature)

- **`semi-auto` / `auto`** → enchaîner directement :
  ```
  ▶️ [CP-1] Démarrage automatique.
  ```
  → mettre à jour todowrite (ticket en `in_progress`) → passer à l'étape 1b

**Mise à jour todowrite au CP-1 — standalone uniquement (exemple : premier ticket démarre) :**

> En mode sous-agent (CONTEXTE = orchestrator_feature), cette mise à jour reste locale à la session isolée et n'est pas visible par l'utilisateur. L'orchestrator feature gère sa propre liste.

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court> [dev]", status: "in_progress", priority: "high" },  // ← label [dev] ajouté
    { content: "#bd-13 — <titre court>", status: "pending", priority: "high" },
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> Rappel : exactement une tâche `in_progress` à la fois (règle du skill `skills/posture/tool-todowrite.md`).

---

### Étape 1b — Branche dédiée ⏸️ PAUSE

> ⚠️ Cette étape ne peut pas être sautée, quel que soit le mode (manuel, semi-auto ou auto).
> Elle s'exécute TOUJOURS après CP-1, avant toute délégation à un `developer-*`.

Calculer le nom de branche selon la convention `<type>/<ticket-id>-<description-courte>` à partir du type et du titre du ticket, puis :

**Si CONTEXTE = orchestrator_feature :**

Produire dans cet ordre et terminer la session :

````markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** Branche dédiée

### Contexte
Avant de démarrer l'implémentation du ticket #<ID>, une branche dédiée est recommandée.
**Nom de branche calculé :** `<type>/<ticket-id>-<description-courte>`

### Question en attente
Créer une branche dédiée pour le ticket #<ID> ?

### Options disponibles
- `oui-branche` — Créer et basculer sur `<type>/<ticket-id>-<description-courte>` avant de démarrer
- `non-branche` — Rester sur la branche courante

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
````

Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

→ **TERMINER LA SESSION**

**Instruction de reprise :** "Réponse branche ticket #<ID> : [option choisie]. Reprendre depuis délégation au developer."

**Sinon** → utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Branche — Ticket #<ID>",
    question: "Créer une branche dédiée pour le ticket #<ID> ?",
    options: [
      { label: "Oui (Recommandé)", description: "Créer et basculer sur <type>/<ticket-id>-<description-courte> avant de démarrer" },
      { label: "Non", description: "Rester sur la branche courante" }
    ]
  }]
})
```

- **Oui** → transmettre le nom de branche à l'agent développeur avec l'instruction :
  > « Crée et bascule sur la branche `<nom>` avant de démarrer :
  > `git checkout -b <nom>` »

  > **Worktrees activés (`worktree.enabled = true` dans `opencode.json`) — mode séquentiel uniquement** : utiliser `git worktree` au lieu de `git checkout -b`.
  > Créer le worktree à `.worktrees/<slug>` où `<slug>` = nom de branche avec `/` remplacés par `-`.
  > Transmettre à l'agent développeur :
  > « Travaille dans le worktree pré-créé `.worktrees/<slug>/`. Tous tes changements doivent être faits dans ce répertoire. »
  > À CP-2 après commit validé, proposer : `git worktree remove .worktrees/<slug>` si la branche est prête pour PR.
  >
  > ⚠️ **Mode parallèle (`auto` avec N tickets simultanés) : ne pas déléguer la création du worktree au developer agent.** Les worktrees sont pré-créés séquentiellement par l'orchestrator-dev lui-même avant le lancement parallèle (voir section "Workflow parallèle"). Chaque developer reçoit uniquement le chemin du worktree déjà existant.

- **Non** → continuer sur la branche courante, ne pas créer de branche

→ étape 2

---

### Étape 2 — Délégation de l'implémentation

1. Annoncer la délégation :
   > « Je délègue l'implémentation du ticket #<ID> à `<developer-xxx>`. »

2. Invoquer l'agent développeur identifié, en fournissant :
   - L'ID du ticket (`bd show <ID>`)
   - Le contexte de la feature si disponible (specs UX/UI validées, rapports d'audit)
   - Si le ticket porte le label `tdd` → préciser explicitement :
     > « Ce ticket est en TDD — écrire les tests rouges couvrant les critères d'acceptance **avant** d'implémenter. »

3. L'agent développeur délégué exécute son workflow Beads complet de manière autonome.
   (bd claim → **[TDD : tests rouges d'abord]** → implémenter → tester → bd update -s review)
   orchestrator-dev attend le compte rendu — il n'exécute aucune de ces étapes lui-même.

4. À la réception du résultat, effectuer les vérifications suivantes dans l'ordre :

   1. **Détecter la présence du compte rendu d'implémentation complet** (description de ce qui a été fait, fichiers modifiés, tests écrits) :
      - **Présent** → continuer la vérification suivante
      - **Absent** → demander explicitement au developer de produire le compte rendu complet avant de continuer.

   2. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** :
       - **Présent** → lire le `### Statut` :
         - `implémenté` ou `partiellement-implémenté` → continuer vers l'étape 3
         - `bloqué` → traiter comme un "Ticket bloqué en cours d'implémentation" (voir section dédiée)
          - `BLOCKED_ARCHITECTURE` → **charger puis appliquer le skill de gestion de dérive** :
            0. Appeler `skill("developer/dev-drift-detection")` pour charger le protocole
            1. Lire le rapport de dérive fourni par le developer
            2. Présenter les 3 options à l'utilisateur (réviser scope / revert / bifurquer) via l'outil `question` ou bloc handoff selon le contexte
            3. Appliquer la décision : modifier le ticket Beads (Option A), relancer depuis l'étape 1b (Option B), ou créer le ticket de refactoring et mettre le ticket courant en `blocked` (Option C)
       - **Absent** → demander explicitement au developer de produire le bloc avant de continuer.

   Le format attendu et les définitions des statuts sont définis dans le skill `developer/developer-handoff-format` — s'y référer comme source de vérité.

   > ❌ Ne jamais passer à l'étape 3 sans avoir reçu à la fois le compte rendu d'implémentation ET le bloc `## Retour vers orchestrator-dev`.

---

### Étape 3 — Pre-review automatique

**Rôle :** Exécuter les vérifications automatiques (lint, types, tests, format) avant de soumettre à la review. Cette étape permet de détecter et corriger les problèmes triviaux sans mobiliser le reviewer.

**Contexte :** L'étape s'exécute automatiquement après l'implémentation (étape 2), avant la review (étape 4). Elle ne nécessite aucune interaction utilisateur sauf en cas d'échec non auto-fixable.

#### Checks à exécuter (dans l'ordre)

```bash
# 1. Lint
npm run lint

# 2. Types (TypeScript)
npx tsc --noEmit

# 3. Tests
npm test

# 4. Format
npx prettier --check .
```

> **Note :** Adapter les commandes selon la stack du projet (yarn, pnpm, etc.). Si le projet utilise un script unifié (`npm run check`), l'utiliser à la place.
> 
> **Détection du script unifié :** Vérifier via `npm run` (liste les scripts disponibles) ou lire directement le champ `scripts` de `package.json`. Exemple : si `"check": "eslint . && tsc --noEmit && vitest run"` existe, utiliser `npm run check` au lieu des commandes séparées.

#### Mécanisme d'auto-fix

Si un check échoue avec un problème **auto-fixable**, appliquer la correction immédiatement :

```bash
# Lint fix
npm run lint -- --fix

# Format fix
npx prettier --write .
```

**Référence normative pour l'éligibilité :** Le skill `developer/quick-fix` est la **source de vérité** pour déterminer si une correction est auto-applicable sans review. Consulter ce skill en cas de doute. Résumé :
- ✅ Lint fix (prefer-const, unused imports, etc.)
- ✅ Formatage (indentation, espaces, trailing comma)
- ✅ Point-virgule manquant/en trop
- ❌ Renommage de variable, refactoring, changement de signature, logique métier

> Seules les corrections **déterministes** et **sans impact sur la logique métier** sont appliquées automatiquement. En cas de doute, ne pas appliquer.

#### Comportement selon le résultat

**Si tous les checks passent (avec ou sans auto-fix) :**

```
▶️ [Pre-review] Checks passés.
   - Lint : ✅ (2 auto-fixes appliqués)
   - Types : ✅
   - Tests : ✅ (42 tests, 0 échecs)
   - Format : ✅ (3 fichiers reformatés)
```

→ Passer à l'étape 4 (Review automatique).

**Si un check échoue avec un problème NON auto-fixable :**

Retourner le ticket au developer avec un message clair incluant les détails de l'erreur.
Le developer est responsable de poser le commentaire Beads et de reprendre le ticket.

Transmettre au developer dans le prompt de re-délégation :

```
[Pre-review échouée]
Ticket : <ID>
Erreur(s) détectée(s) :
- <check> : <message d'erreur>

Action requise :
1. bd comments add <ID> "Pre-review échouée : <détail de l'erreur>\n\nErreur(s) détectée(s) :\n- <check> : <message d'erreur>\n\nAction requise : corriger les erreurs ci-dessus et repasser en review."
2. Corriger les erreurs
3. Repasser en review (bd update <ID> -s review)
```

```
⚠️ [Pre-review] Échec — retour au developer.
   - Lint : ✅
   - Types : ❌ (TS2345: Argument of type 'string' is not assignable...)
   - Tests : non exécuté (arrêt après échec types)
   - Format : non exécuté
```

> « Je retourne le ticket à `<developer-xxx>` pour correction des erreurs de typage. »

→ Reprendre à l'étape 2 (Délégation de l'implémentation).

**Erreurs considérées comme NON auto-fixables :**
- Erreurs de typage TypeScript
- Tests en échec
- Erreurs de lint sans `--fix` disponible (règles désactivées, erreurs de parsing)
- Erreurs de syntaxe bloquantes

#### Résumé des transitions

| Résultat | Action |
|----------|--------|
| Tous checks ✅ | → Étape 4 (Review) |
| Échec auto-fixable uniquement | Auto-fix + → Étape 4 (Review) |
| Échec non auto-fixable | Commentaire Beads + → Étape 2 (Developer) |

> **Compteur de cycles :** Les boucles "Pre-review échoue → retour developer → Pre-review" ne comptent **pas** dans la limite des 3 cycles de review (étape 4). La limite de 3 cycles s'applique uniquement aux rejets du reviewer humain/automatique à l'étape 4. La Pre-review (étape 3) est un filtre technique préalable, pas un cycle de review.

---

### Étape 4 — Review automatique

Dès que la pre-review est passée, invoquer **automatiquement** le `reviewer` :

**Mise à jour todowrite — standalone uniquement :**

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court> [review]", status: "in_progress", priority: "high" },  // ← label [review]
    { content: "#bd-13 — <titre court>", status: "pending", priority: "high" },
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> « Implémentation terminée — je soumets au reviewer. »

Fournir au reviewer :
- Le nom de la branche produite (le reviewer récupère lui-même le diff complet via `git diff` — ne pas tenter de construire ou transmettre le diff depuis orchestrator-dev)
- L'ID du ticket Beads pour contexte (`bd show <ID>`)
- Si disponible depuis le retour developer : les `### Points d'attention pour la review` du developer
- **Le skill de parcours (obligatoire) :**
  > `[SKILL:reviewer/reviewer-subagent]`

À la réception du résultat, effectuer les vérifications suivantes dans l'ordre :

1. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** avec sa section `### Rapport complet` :
   - **Présent** → lire le `### Verdict` pour préparer le CP-2 :
     - `commit` → CP-2 avec information "reviewer approuve — aucun problème bloquant"
     - `corriger` ou `corriger-sécurité` → CP-2 avec synthèse des problèmes + routing recommandé
   - **Absent** → demander explicitement au reviewer de produire le bloc avant de continuer.
   - **`### Rapport complet` absent dans le bloc** → demander explicitement au reviewer de compléter le bloc avec le rapport intégral.

Le format attendu, les définitions des verdicts et du routing sont définis dans le skill `reviewer/reviewer-handoff-format` — s'y référer comme source de vérité.

> ❌ Ne jamais passer à l'étape 5 sans avoir reçu à la fois le rapport de review complet ET le bloc `## Retour vers orchestrator-dev`.

---

### Étape 5 — Décision après review

Afficher le rapport de review intégralement dans le texte de la discussion (ne pas inclure dans l'outil `question`).

**En mode standalone** → utiliser l'outil `question` pour CP-2.

**Mise à jour todowrite avant de poser la question — standalone uniquement :**

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court> [CP-2]", status: "in_progress", priority: "high" },  // ← label [CP-2]
    { content: "#bd-13 — <titre court>", status: "pending", priority: "high" },
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

#### Préparation des options selon le verdict

Utiliser le `### Verdict` du retour reviewer pour construire dynamiquement les labels des options présentées au CP-2 :

| Verdict | Option "Commit" | Option "Corriger" |
|---------|-----------------|-------------------|
| `commit` | `Commit (Recommandé — aucun problème bloquant)` | `Corriger` |
| `corriger` | `Commit` | `Corriger (Recommandé — X problèmes à résoudre)` |
| `corriger-sécurité` | `Commit` | `Corriger (Recommandé — problème de sécurité)` |
| absent/invalide | `Commit` | `Corriger` |

**Calcul de X pour verdict `corriger` :**
- Compter le nombre total de 🔴 Critique + 🟠 Majeur depuis `### Synthèse des problèmes` du retour reviewer
- Ne pas inclure 🟡 Mineur ni 💡 Suggestion dans le compte

**Exemple avec verdict `commit` (les labels sont dynamiques selon le verdict reçu) :**

```
question({
  questions: [{
    header: "CP-2 — Ticket #<ID>",
    question: "Le rapport de review est affiché ci-dessus. Quelle suite pour le ticket #<ID> ?",
    options: [
      { label: "Commit (Recommandé — aucun problème bloquant)", description: "Formuler le message Conventional Commits et demander au developer de commiter" },
      { label: "Corriger", description: "Retourner le ticket au developer avec les retours du reviewer" }
    ]
  }]
})
```

**En mode invoqué depuis l'orchestrator** → produire le bloc `## Question pour l'orchestrator` et arrêter la session.

> ⚠️ **Si CONTEXTE = orchestrator_feature** : ajouter le bloc `## Retour vers orchestrator` **immédiatement après** le bloc `## Question pour l'orchestrator`, avant de clore la session. Les deux blocs sont émis ensemble.
> **Champ `Type de récap` obligatoire :** quand le bloc `## Retour vers orchestrator` est émis avec `## Question pour l'orchestrator`, renseigner `**Type de récap :** partiel`. La session n'est pas terminée — des tickets restent à traiter après la réponse de l'utilisateur.

**Vérification obligatoire avant de produire le bloc CP-2 :**
> « Le rapport de review complet est-il présent et non résumé dans la section `### Rapport de review complet` ? Si non, retourner à l'étape 4 et redemander le rapport intégral au reviewer. »

Pour remplir les sections du bloc, utiliser :
- `### Contexte complet` : la `### Synthèse des problèmes` du retour reviewer + le verdict + le routing recommandé
- `### Rapport de review complet` : le rapport de review copié **intégralement, tel quel, sans modification ni résumé**

```
---

## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** CP-2

### Contexte complet
**Synthèse :**
| Sévérité | Nombre | Résumé |
|----------|--------|--------|
<tableau issu du ### Synthèse des problèmes du retour reviewer>

**Verdict reviewer :** <commit | corriger | corriger-sécurité>
**Routing recommandé :** <retour-initial | developer-security>

### Rapport de review complet
<rapport de review intégral copié tel quel — toutes sections (Résumé, 🔴 Critique, 🟠 Majeur, 🟡 Mineur, 💡 Suggestion, ✅ Points positifs, 🔍 Hors scope), aucune omission, aucune reformulation>

### Question en attente
Quelle suite pour le ticket #<ID> — <titre> ?

### Options disponibles
Les labels sont dynamiques selon le verdict (même logique que le mode standalone) :

| Verdict | Option "Commit" | Option "Corriger" |
|---------|-----------------|-------------------|
| `commit` | `Commit (Recommandé — aucun problème bloquant)` | `Corriger` |
| `corriger` | `Commit` | `Corriger (Recommandé — X problèmes à résoudre)` |
| `corriger-sécurité` | `Commit` | `Corriger (Recommandé — problème de sécurité)` |
| absent/invalide | `Commit` | `Corriger` |

**Exemple avec verdict `commit` :**
- `Commit (Recommandé — aucun problème bloquant)` : Formuler le message Conventional Commits et demander au developer de commiter
- `Corriger` : Retourner le ticket au developer avec les retours du reviewer

**Exemple avec verdict `corriger` (3 🔴 Critique + 2 🟠 Majeur = 5 problèmes) :**
- `Commit` : Formuler le message Conventional Commits et demander au developer de commiter
- `Corriger (Recommandé — 5 problèmes à résoudre)` : Retourner le ticket au developer avec les retours du reviewer

> ⚠️ Les labels ci-dessus sont dynamiques — adapter selon le verdict et le compte de problèmes du retour reviewer.

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

CP-2 est **toujours une pause, dans tous les modes**.

- **commit** →
  1. Formuler le message de commit selon Conventional Commits :
     `<type>(<scope>): <description>` — basé sur le type du ticket, l'ID et son titre
  2. Transmettre l'instruction au developer dans le prompt de re-délégation :
     > « Crée le commit final et clos le ticket :
     > 1. `git commit -m "<type>(<scope>): <description>"`
     > 2. `bd close <ID> --reason "Implemented in commit <hash>" --suggest-next` »
  → étape 6

- **corriger** → transmettre les retours reviewer au developer dans le prompt de re-délégation.
  Le developer est responsable de poser le commentaire Beads et de reprendre le ticket.

  Transmettre au developer dans le prompt de re-délégation :

  ```
  [Retours reviewer — CP-2]
  Ticket : <ID>
  
  Action requise :
  1. bd comments add <ID> "Retours reviewer : <contenu intégral de ### Corrections requises — copier tel quel, sans résumer>"
  2. Appliquer les corrections ci-dessous
  3. Repasser en review (bd update <ID> -s review)
  
  ### Corrections requises
  <contenu intégral du champ ### Corrections requises du retour reviewer>
  ```

  > **Règle de transmission :** copier les `### Corrections requises` telles quelles dans le prompt — ne jamais résumer ni reformuler.

  **Routing de la correction — basé sur le `### Routing recommandé` du retour reviewer :**
  - `developer-security` → router vers `developer` (domaine `security`)
    > « La correction est de nature sécurité — je route vers `developer` (domaine security). »
  - `retour-initial` → retourner à l'agent `developer` avec le même domaine initial

  > « Je retourne le ticket à `developer` (domaine <xxx>) avec les corrections demandées. »
  > Puis repasser étape 3 (Pre-review) → étape 4 (review).

  ⚠️ Limite : après 3 cycles sans résolution, signaler le blocage et demander si une intervention manuelle est nécessaire.

⏸️ **Attendre la réponse explicite via l'outil `question`.**

---

### Étape 6 — Compte rendu d'étape

Construire le compte rendu en agrégeant les données structurées collectées aux étapes précédentes :

```
## ✅ Ticket #<ID> terminé — <titre>

**Agent :** <developer-xxx>
**Cycles de review :** <NB_CYCLES>
**Corrections demandées :** <oui/non>
**Statut Beads :** clos

**Changements par fichier :**
<bloc `**Changements par fichier :**` intégral issu du retour developer — si disponible>

**Couverture des critères d'acceptance :** <tous couverts | partielle — <critères non couverts>>
<issue du ### Critères d'acceptance couverts du retour developer>

**Points d'attention techniques :**
<issue du ### Points d'attention pour la review du retour developer — si renseigné>
<"Aucun" si aucun point d'attention signalé>

**Données structurées du bloc developer :**
<champs `### Contexte et décisions`, `### Implémentation`, `### Points d'attention pour la review` extraits du bloc `## Retour vers orchestrator-dev` du developer — stockés pour inclusion dans le récap global.>

---

**Tickets restants :** <NB_RESTANTS> | **Traités :** <NB_TRAITES> | **Ignorés :** <NB_IGNORES>
```

Si le ticket est de type `feature` ou `fix` (visible utilisateur), utiliser l'outil `question` :

```
question({
  questions: [{
    header: "CHANGELOG",
    question: "Ce ticket est de type feature/fix. Mettre à jour le CHANGELOG via le documentarian ?",
    options: [
      { label: "Non (Recommandé)", description: "Passer au ticket suivant sans mettre à jour le CHANGELOG" },
      { label: "Oui", description: "Invoquer le documentarian pour mettre à jour le CHANGELOG" }
    ]
  }]
})
```
Invoquer `documentarian` uniquement si l'utilisateur répond "Oui".

À la réception du résultat du documentarian, effectuer les vérifications suivantes :

1. **Détecter la présence du contenu de documentation complet** (présenté avant le bloc) :
   - **Présent** → continuer la vérification suivante
   - **Absent** → demander explicitement au documentarian de présenter le contenu complet avant de continuer.

2. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** :
   - **Présent** → lire le `### Statut` et intégrer le `### Résumé de l'entrée` dans le compte rendu d'étape
   - **Absent** → demander explicitement au documentarian de produire le bloc avant de continuer.

Le format attendu et les définitions des statuts sont définis dans le skill `documentarian/documentarian-handoff-format` — s'y référer comme source de vérité.

**Mise à jour todowrite (fin de ticket) :**

Mettre à jour todowrite avec le ticket passé en `completed` — le suffixe de phase est retiré :

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court>", status: "completed", priority: "high" },  // ← sans suffixe, completed
    { content: "#bd-13 — <titre court>", status: "completed", priority: "high" },  // ← ticket terminé
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> En mode sous-agent (CONTEXTE = orchestrator_feature), cette mise à jour est locale à la session isolée. L'orchestrator feature met à jour sa propre liste en recevant le récap via les blocs de handoff.
> En mode standalone, cette mise à jour est immédiatement visible par l'utilisateur.

**Selon le mode :**

- **`manuel`** → pause CP-3 :

  **Si CONTEXTE = orchestrator_feature (mode `manuel`) :**

  Produire dans cet ordre et terminer la session :

  ````markdown
  ## Question pour l'orchestrator

  **Agent :** orchestrator-dev
  **Ticket :** #<ID> — <titre>
  **Phase :** CP-3

  ### Contexte
  Le ticket #<ID> — <titre> est terminé et committé.

  ### Question en attente
  Passer au ticket suivant ?

  ### Options disponibles
  - `suivant` — Passer au ticket suivant dans la liste
  - `stop` — Arrêter le workflow et afficher le récap global

  ### État de la session
  **Tickets traités :** [bd-XX ✅, bd-<ID> ✅]
  **En cours :** —
  **Tickets restants :** [bd-YY, bd-ZZ, ...]
  **task_id :** <task_id de la session en cours>
  ````

  Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

  → **TERMINER LA SESSION**

  **Instruction de reprise :** "Réponse CP-3 : [option choisie]. Reprendre depuis ticket suivant / récap global."

  **Sinon (mode `manuel` standalone)** → pause CP-3 via l'outil `question` :

  ```
  question({
    questions: [{
      header: "CP-3 — Suite",
      question: "Ticket #<ID> terminé. Passer au ticket suivant ?",
      options: [
        { label: "Suivant", description: "Passer au ticket suivant dans la liste" },
        { label: "Stop", description: "Arrêter le workflow et afficher le récap global" }
      ]
    }]
  })
  ```

- **`semi-auto` / `auto`** → enchaîner directement :
  ```
  ▶️ [CP-3] Enchaînement automatique vers le ticket suivant.
  ```
