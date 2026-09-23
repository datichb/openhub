---
name: orchestrator-dev-parallel
description: Workflow parallèle de l'orchestrator-dev — conditionnel tous modes, worktrees, pre-review et review en parallèle, sérialisation FIFO des CP en mode manuel.
---

## Workflow parallèle

Ce workflow s'applique quand les 4 critères de parallélisabilité sont vérifiés, **quel que soit le mode de workflow** (manuel, semi-auto, auto).

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
5. En cas d'échec, appliquer le protocole de recovery worktree (voir skill `orchestrator/error-recovery-protocol`) :
   - Branche déjà existante → `git worktree remove <path>` + `git branch -D <branch>` puis réessayer
   - Verrou git (`.git/index.lock`) → attendre 5s, réessayer (max 3 tentatives)
   - Échec persistant → **fallback séquentiel** pour ce ticket (le retirer du lot parallèle, le traiter après les sessions parallèles restantes)

Ne pas lancer les sessions parallèles tant que tous les worktrees actifs ne sont pas créés.

Stocker pour chaque ticket : `{ ticket_id, branch_name, worktree_path: ".worktrees/<slug>" }`

**SEULEMENT une fois tous les worktrees créés avec succès**, passer au lancement simultané.

### Lancement simultané

Invoquer N sessions `developer-*` dans le même appel — chacune reçoit son ticket, son contexte, et l'instruction TDD si applicable. Le nombre de sessions est limité par le budget de points et le plafond technique (voir critère 4 dans `orchestrator-dev-protocol`).

Quand les worktrees sont activés, chaque developer reçoit dans son prompt le chemin du worktree **déjà existant** :
> « Travaille exclusivement dans `.worktrees/<slug>/`. Le worktree et la branche `<nom-branche>` ont déjà été créés — ne pas relancer `git worktree add`. Tous tes changements doivent être faits depuis ce répertoire. »

### Attente et agrégation des résultats

Attendre les résultats de toutes les sessions. Pour chaque résultat reçu :

1. Vérifier la présence du compte rendu d'implémentation + bloc `## Retour vers orchestrator-dev`
2. Si `### Statut` = `bloqué` → traiter comme un "Ticket bloqué" (produire `## Question pour l'orchestrator` si invoqué depuis l'agent orchestrator)
3. Détecter un éventuel conflit de fichiers : si un `developer-*` a modifié un fichier déjà modifié par une autre session parallèle, signaler et passer à l'étape Pre-review+Review en priorité pour ce ticket avant les autres
4. **Si un résultat est absent ou tronqué** (pas de bloc de handoff, indicateurs de progression mais pas de clôture) → appliquer le protocole de retry/recovery (voir skill `orchestrator/error-recovery-protocol`). Si le retry échoue, traiter ce ticket en **fallback séquentiel** après les sessions parallèles restantes — ne pas bloquer les autres sessions.

### Sérialisation FIFO des CP (mode `manuel` parallèle)

> Cette section s'applique uniquement en mode `manuel`. En mode `semi-auto` et `auto`, les CP-1 et CP-3 sont automatiques — seul le CP-2 fait l'objet d'une pause (voir "CP-2 en batch conditionnel" ci-dessous).

En mode `manuel`, les CP-1 et CP-3 de chaque session parallèle sont des pauses obligatoires. Comme les sessions progressent à des vitesses différentes, ces CP arrivent de manière échelonnée.

**Protocole FIFO :**

1. **Collecte** — Maintenir une file d'attente des CP pendants, ordonnée par date d'arrivée (premier arrivé = premier traité)
2. **Notification** — Quand un CP arrive alors qu'un autre est déjà en cours de traitement, notifier l'utilisateur :
   ```
   > ⏸️ [CP en attente] <NB> CP en file : #bd-42 CP-1, #bd-43 CP-3
   ```
3. **Présentation** — Présenter le CP suivant dès que l'utilisateur a répondu au précédent
4. **Non-blocage** — Les sessions dont le CP n'a pas encore été traité restent en attente, mais les autres sessions qui n'ont pas atteint de CP continuent de progresser librement
5. **Format** — Chaque CP est présenté via un bloc `## Question CP pour l'orchestrator` (voir `orchestrator-handoff-format`)

> **Les CP-2 ne passent pas par ce mécanisme** — ils suivent toujours la logique batch/séquentielle décrite ci-dessous, y compris en mode `manuel`.

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
Le batch CP-2 est disponible dans **tous les modes** quand le parallélisme est actif — pas uniquement en mode `auto`.

### Récap global — synchronisation finale

Le récap global est produit uniquement quand **toutes** les sessions parallèles ont retourné un récap `**Type de récap :** final`. Ne pas produire le récap global tant qu'au moins une session est encore suspendue sur CP-2 ou en cours.
