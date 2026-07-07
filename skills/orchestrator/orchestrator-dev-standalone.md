---
name: orchestrator-dev-standalone
description: Parcours d'exécution de l'agent orchestrator dev en mode standalone (invoqué directement par l'utilisateur) — CP-0 demande le mode, tous les CPs posent les questions via l'outil question, todo list visible par l'utilisateur.
---

# Skill — Parcours Orchestrator-dev Standalone

> Ce skill est chargé automatiquement quand l'orchestrator-dev est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, tous les checkpoints posent les questions **directement via l'outil `question`**. La todo list est visible par l'utilisateur en temps réel.

---

## CP-0 — Initialisation standalone

Afficher les tickets à traiter, demander le mode de workflow via les blocs question du skill `orchestrator-workflow-modes`.

**Initialiser todowrite** avec 1 tâche par ticket (toutes en `pending`) avec les labels de phase :

| Moment | Mise à jour du label | Statut |
|--------|---------------------|--------|
| CP-0 (initialisation) | `#bd-12 — <titre>` | `pending` |
| CP-1 démarrage | `#bd-12 — <titre> [dev]` | `in_progress` |
| Étape 4 — review lancée | `#bd-12 — <titre> [review]` | `in_progress` |
| Étape 5 — CP-2 en attente | `#bd-12 — <titre> [CP-2]` | `in_progress` |
| CP-2 commit validé | `#bd-12 — <titre>` | `completed` |
| CP-1 passer / ticket ignoré | `#bd-12 — <titre>` | `cancelled` |

---

## Comportement des CPs (standalone)

### CP-1 — Démarrer le ticket
Poser la question via l'outil `question` :
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

### Branche dédiée
Poser la question via l'outil `question` :
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

### CP-2 — Commit ou corriger ?
Afficher le rapport de review intégralement dans le texte **avant** d'appeler `question`. Utiliser les labels dynamiques selon le verdict du reviewer :
```
question({
  questions: [{
    header: "CP-2 — Ticket #<ID>",
    question: "Le rapport de review est affiché ci-dessus. Quelle suite pour le ticket #<ID> ?",
    options: [
      { label: "<Commit | Commit (Recommandé — aucun problème bloquant)>", description: "Formuler le message Conventional Commits et demander au developer de commiter" },
      { label: "<Corriger | Corriger (Recommandé — X problèmes à résoudre)>", description: "Retourner le ticket au developer avec les retours du reviewer" }
    ]
  }]
})
```

### CP-3 — Ticket suivant ou stop ?
```
question({
  questions: [{
    header: "CP-3 — Suite",
    question: "Ticket #<ID> terminé. Continuer avec le ticket suivant ?",
    options: [
      { label: "Ticket suivant (Recommandé)", description: "Passer au ticket #<ID-suivant> — <titre>" },
      { label: "Stop", description: "Arrêter le workflow et afficher le récap global" }
    ]
  }]
})
```

### CHANGELOG (feature/fix)
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

---

## Récap global (fin de session standalone)

Produire le récap global en texte clair. **Ne pas** produire le bloc `## Retour vers orchestrator`.
