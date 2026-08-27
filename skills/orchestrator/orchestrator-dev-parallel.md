---
name: orchestrator-dev-parallel
description: Workflow parallèle de l'orchestrator-dev — mode auto conditionnel, worktrees, pre-review et review en parallèle.
---

## Workflow parallèle (mode `auto` conditionnel uniquement)

Ce workflow s'applique uniquement quand les 4 critères de parallélisabilité sont vérifiés.

### Phase 0 — Pré-création séquentielle des worktrees (si `worktree.enabled = true`)

> ⚠️ **Cette phase est obligatoire avant tout lancement parallèle quand les worktrees sont activés.**
> Les developer agents ne doivent jamais créer leurs worktrees eux-mêmes en mode parallèle — cela provoquerait une contention sur `.git/index.lock` et une perte d'isolation.

Pour chaque ticket du batch, **dans l'ordre, un par un** :

1. Calculer le nom de branche : `<type>/<ticket-id>-<description-courte>`
2. Calculer le slug : remplacer `/` par `-` → `<type>-<ticket-id>-<description-courte>`
3. Exécuter directement (via bash) :
   ```bash
   git worktree add -b <nom-branche> .worktrees/<slug>
   ```
4. Vérifier le succès de la commande avant de passer au ticket suivant
5. En cas d'échec (branche déjà existante, verrou git, etc.) : résoudre le conflit avant de continuer — ne pas lancer les sessions parallèles tant que tous les worktrees ne sont pas créés

Stocker pour chaque ticket : `{ ticket_id, branch_name, worktree_path: ".worktrees/<slug>" }`

**SEULEMENT une fois tous les worktrees créés avec succès**, passer au lancement simultané.

### Lancement simultané

Invoquer N sessions `developer-*` dans le même appel — chacune reçoit son ticket, son contexte, et l'instruction TDD si applicable. Maximum 3 sessions simultanées.

Quand les worktrees sont activés, chaque developer reçoit dans son prompt le chemin du worktree **déjà existant** :
> « Travaille exclusivement dans `.worktrees/<slug>/`. Le worktree et la branche `<nom-branche>` ont déjà été créés — ne pas relancer `git worktree add`. Tous tes changements doivent être faits depuis ce répertoire. »

### Attente et agrégation des résultats

Attendre les résultats de toutes les sessions. Pour chaque résultat reçu :

1. Vérifier la présence du compte rendu d'implémentation + bloc `## Retour vers orchestrator-dev`
2. Si `### Statut` = `bloqué` → traiter comme un "Ticket bloqué" (produire `## Question pour l'orchestrator` si invoqué depuis l'agent orchestrator)
3. Détecter un éventuel conflit de fichiers : si un `developer-*` a modifié un fichier déjà modifié par une autre session parallèle, signaler et passer à l'étape Pre-review+Review en priorité pour ce ticket avant les autres

### Pre-review et Review en parallèle

Les phases Pre-review et Review sont lancées pour chaque ticket dès que son implémentation est terminée — sans attendre les autres sessions. Les sessions Pre-review et Review pour différents tickets peuvent donc se chevaucher.

### CP-2 en batch conditionnel

Lorsque N sessions atteignent CP-2 simultanément (chacune produit `## Question pour l'orchestrator`) :

#### Étape 1 — Évaluation des verdicts

Collecter les verdicts de tous les rapports de review en attente :
- Extraire le `### Verdict` de chaque `## Question pour l'orchestrator`
- Classer les tickets en deux catégories : verdict `commit` vs verdict `corriger` ou `corriger-sécurité`

#### Étape 2 — Décision de batch ou éclatement

**Si TOUS les verdicts sont `commit`** → proposer un batch groupé :

```
> 📋 [CP-2 — Batch disponible] <NB_TICKETS> tickets prêts à commiter.
> Tous les verdicts reviewer sont `commit` — aucun problème bloquant détecté.
```

Utiliser l'outil `question` :

```
question({
  questions: [{
    header: "CP-2 — Batch de <NB_TICKETS> tickets",
    question: "<NB_TICKETS> tickets ont reçu un verdict `commit` du reviewer. Quelle action pour ce lot ?",
    options: [
      { label: "Commit tous", description: "Commiter les <NB_TICKETS> tickets en séquence avec leurs messages Conventional Commits respectifs" },
      { label: "Commit sélectif", description: "Choisir quels tickets commiter parmi les <NB_TICKETS> disponibles" },
      { label: "Voir détails", description: "Afficher le rapport de review de chaque ticket avant de décider" }
    ]
  }]
})
```

**Comportement selon la réponse :**

- **Commit tous** → pour chaque ticket du lot, dans l'ordre FIFO (ordre d'arrivée des sessions au CP-2) :
  1. Formuler le message de commit selon Conventional Commits
  2. Transmettre au developer dans le prompt de re-délégation :
     > « Crée le commit final et clos le ticket :
     > 1. `git commit -m "<type>(<scope>): <description>"`
     > 2. `bd close <ID> --reason "Implemented in commit <hash>" --suggest-next` »
  3. Passer au ticket suivant du lot
  4. Une fois tous les tickets commités, afficher le récap groupé et continuer

- **Commit sélectif** → afficher la liste des tickets du lot avec leur titre, puis utiliser l'outil `question` :
  ```
  question({
    questions: [{
      header: "Sélection des tickets à commiter",
      question: "Quels tickets commiter parmi les <NB_TICKETS> disponibles ?",
      multiple: true,
      options: [
        { label: "#<ID-1> — <titre-1>", description: "Verdict: commit" },
        { label: "#<ID-2> — <titre-2>", description: "Verdict: commit" },
        ...
      ]
    }]
  })
  ```
  → Commiter uniquement les tickets sélectionnés, les autres retournent en séquentiel standard
  → Si aucun ticket sélectionné (sélection vide), revenir au choix précédent sans action

- **Voir détails** → afficher les rapports de review complets un par un, puis passer en mode séquentiel standard :
  1. Afficher le rapport de review complet du premier ticket
  2. Poser un CP-2 unitaire (Commit / Corriger) pour ce ticket
  3. Répéter pour chaque ticket du batch, dans l'ordre FIFO
  (voir "Mode séquentiel standard" ci-dessous pour le détail)

**Si AU MOINS UN verdict est `corriger` ou `corriger-sécurité`** → éclater le batch :

```
> 📋 [CP-2 — Batch éclaté] <NB_TICKETS> tickets en attente, <NB_CORRIGER> avec verdict `corriger`.
> Traitement séquentiel : les tickets avec corrections requises seront présentés individuellement.
```

→ Passer en mode séquentiel standard.

#### Mode séquentiel standard (éclatement ou choix explicite)

- Présenter les rapports de review **un par un**, dans l'ordre d'arrivée
- Recueillir la réponse de l'utilisateur pour chaque rapport avant de passer au suivant
- Ré-invoquer chaque session via son `task_id` avec la réponse correspondante

```
> 📋 [CP-2 — Revue séquentielle] <NB_TICKETS> rapports de review en attente.
> Traitement séquentiel : rapport 1/<NB_TICKETS> affiché ci-dessus.
```

#### Rappel — CP-2 reste une pause obligatoire

Le batch ne supprime pas la validation humaine — il la regroupe pour les cas homogènes.
CP-2 reste une pause dans **tous les modes** sans exception, y compris avec le batch.

### Récap global — synchronisation finale

Le récap global est produit uniquement quand **toutes** les sessions parallèles ont retourné un récap `**Type de récap :** final`. Ne pas produire le récap global tant qu'au moins une session est encore suspendue sur CP-2 ou en cours.
