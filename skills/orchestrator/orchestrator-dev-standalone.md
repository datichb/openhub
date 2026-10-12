---
name: orchestrator-dev-standalone
description: Parcours d'exécution de l'agent orchestrator dev comme agent d'entrée de la session (workflow ticket, review-feedback) — récap des tickets au démarrage (mode fixé au lancement), checkpoints par workflow_checkpoint, confirmations hors checkpoint par l'outil question, todo list visible par l'utilisateur.
---

# Skill — Parcours Orchestrator-dev agent d'entrée

> Partie à suivre quand tu es **agent d'entrée** ou lancé directement par l'utilisateur (section « Rôle dans cette session » de ton prompt, générée par oh).

## Principe fondamental

Chaque checkpoint listé dans la table des checkpoints de la carte du workflow (section « Modes de workflow et checkpoints ») passe **uniquement** par l'outil `workflow_checkpoint` (`id` + `summary`). C'est oh qui applique le mode : l'appel attend la validation de l'utilisateur (`pause`), passe aussitôt (`auto`) ou n'est pas à faire (`skip`). Validation reçue = suite prévue (validation de `cp-2` = commit puis fermeture) ; refus = suivre la consigne reçue, puis rappeler `workflow_checkpoint` pour le même checkpoint. oh ferme sans effet une question qui nomme un checkpoint, et refuse `git commit` et la fermeture du ticket tant que `cp-2` n'est pas validé.

L'outil `question` ne sert qu'aux confirmations **hors checkpoint** (branche dédiée, CHANGELOG), sans jamais nommer un checkpoint. N'appelle que les checkpoints de la table : un checkpoint décrit ci-dessous mais absent de la table n'existe pas dans cette session. La todo list est visible par l'utilisateur en temps réel.

---

## Démarrage

Afficher les tickets à traiter. Le mode est donné par la ligne `Mode de workflow : <mode>` du premier message : ne le redemande pas.

**Initialiser todowrite** avec 1 tâche par ticket (toutes en `pending`) avec les labels de phase :

| Moment | Mise à jour du label | Statut |
|--------|---------------------|--------|
| Démarrage | `#bd-12 — <titre>` | `pending` |
| Ticket démarré (`cp-1` passé) | `#bd-12 — <titre> [dev]` | `in_progress` |
| Review lancée | `#bd-12 — <titre> [review]` | `in_progress` |
| `cp-2` en attente | `#bd-12 — <titre> [validation]` | `in_progress` |
| Commit fait | `#bd-12 — <titre>` | `completed` |
| Ticket ignoré | `#bd-12 — <titre>` | `cancelled` |

---

## Checkpoints

### `cp-1` — Démarrer le ticket
```
workflow_checkpoint({ id: "cp-1", summary: "Ticket #<ID> — <titre> : <résumé en 2 lignes>. Agent prévu : <developer-xxx>." })
```

### Branche dédiée (confirmation hors checkpoint)
Sauf si le premier message contient une ligne `Branche de travail : <branche>` (session déjà sur sa branche : ne rien demander), poser la question via l'outil `question` :
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

### `cp-2` — Commit ou correction
Afficher le rapport de review intégralement dans le texte, puis :
```
workflow_checkpoint({ id: "cp-2", summary: "Ticket #<ID> — verdict du reviewer : <commit | corriger | corriger-sécurité>. <synthèse des problèmes>. Validation = commit puis fermeture ; refus = correction." })
```
Validé → relancer aussitôt le developer : commit Conventional Commits, puis fermeture du ticket. Refusé → retourner le ticket au developer avec la consigne reçue et les retours du reviewer, nouvelle review, puis rappeler `cp-2`.

### `cp-3` — Ticket suivant
```
workflow_checkpoint({ id: "cp-3", summary: "Ticket #<ID> terminé (commit <sha court>). Suivant : #<ID-suivant> — <titre>." })
```

### CHANGELOG (feature/fix, confirmation hors checkpoint)
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

## Récap global (fin de session)

Produire le récap global en texte clair. **Ne pas** produire le bloc `## Retour vers orchestrator`.
