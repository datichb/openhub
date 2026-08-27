---
name: orchestrator-dev-edge-cases
description: Gestion des cas particuliers de l'orchestrator-dev — drift détecté, review échouée, conflits, pannes agent.
---

## Gestion des cas particuliers

### Ticket avec dépendance non résolue

**En mode standalone** → utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Dépendance non résolue",
    question: "Le ticket #<ID> dépend de #<ID-parent> qui n'est pas encore terminé. Comment procéder ?",
    options: [
      { label: "Attendre", description: "Suspendre ce ticket jusqu'à la résolution du ticket parent" },
      { label: "Traiter le parent d'abord", description: "Réorganiser pour traiter #<ID-parent> avant #<ID>" },
      { label: "Continuer quand même", description: "Ignorer la dépendance et démarrer l'implémentation maintenant" }
    ]
  }]
})
```

**En mode invoqué depuis l'orchestrator** → produire le bloc `## Question pour l'orchestrator` et arrêter :

> ⚠️ **Si CONTEXTE = orchestrator_feature** : ajouter le bloc `## Retour vers orchestrator` **immédiatement après** le bloc `## Question pour l'orchestrator`, avant de clore la session. Les deux blocs sont émis ensemble.

```
---

## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** Dépendance non résolue

### Contexte complet
Le ticket #<ID> dépend de #<ID-parent> — <titre du ticket parent>.
Statut du ticket parent : <bd show <ID-parent> — statut et description>

### Question en attente
Le ticket #<ID> dépend de #<ID-parent> qui n'est pas encore terminé. Comment procéder ?

### Options disponibles
- `Attendre` : Suspendre ce ticket jusqu'à la résolution du ticket parent
- `Traiter le parent d'abord` : Réorganiser pour traiter #<ID-parent> avant #<ID>
- `Continuer quand même` : Ignorer la dépendance et démarrer l'implémentation maintenant

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

### Ticket sans agent identifiable

Utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Agent non identifié",
    question: "Aucun agent clairement identifié pour le ticket #<ID>. Quel agent utiliser ?",
    options: [
      { label: "developer (domaine fullstack — Recommandé)", description: "Agent généraliste — couvre les cas ambigus front + back" },
      { label: "Préciser manuellement", description: "Indiquer le domaine à utiliser dans la réponse libre" }
    ]
  }]
})
```

### Blocage après 3 cycles de review

**En mode standalone** → afficher les problèmes persistants, puis utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Blocage après 3 cycles",
    question: "Le ticket #<ID> a subi 3 cycles de review sans résolution. Une intervention manuelle est recommandée. Comment procéder ?",
    options: [
      { label: "Continuer", description: "Tenter un nouveau cycle de correction" },
      { label: "Passer ce ticket", description: "Ignorer ce ticket et passer au suivant" }
    ]
  }]
})
```

### Détection de ping-pong reviewer

Si le reviewer retourne **les mêmes findings** sur le même ticket lors de **2 cycles consécutifs**, ne pas re-déléguer automatiquement au developer.

**Critère de détection :** les champs `corrections requises` du bloc de handoff reviewer contiennent des libellés identiques (ou quasi-identiques) à ceux du cycle précédent.

**Action obligatoire :** escalade immédiate à l'utilisateur, même en mode `semi-auto` ou `auto` :

```
question({
  questions: [{
    header: "Ping-pong détecté",
    question: "Le reviewer signale les mêmes problèmes depuis 2 cycles sur #<ID>.\nLe developer n'arrive pas à corriger ces points :\n\n<liste des findings répétés>\n\nUne intervention manuelle est nécessaire.",
    options: [
      { label: "Reprendre manuellement ce ticket", description: "Vous corrigez vous-même avant de relancer la review" },
      { label: "Passer ce ticket", description: "Ignorer et continuer avec les tickets suivants" },
      { label: "Arrêter la session", description: "Terminer le workflow ici" }
    ]
  }]
})
```

Ne JAMAIS lancer un 3ème cycle sur les mêmes findings sans validation explicite de l'utilisateur.

**En mode invoqué depuis l'orchestrator** → produire le bloc `## Question pour l'orchestrator` et arrêter :

> ⚠️ **Si CONTEXTE = orchestrator_feature** : ajouter le bloc `## Retour vers orchestrator` **immédiatement après** le bloc `## Question pour l'orchestrator`, avant de clore la session. Les deux blocs sont émis ensemble.

```
---

## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** Blocage 3 cycles

### Contexte complet
**Problèmes persistants (non résolus après 3 cycles) :** <liste des points signalés à chaque cycle sans résolution>

**Historique des cycles :**
- Cycle 1 : verdict <commit|corriger> — <synthèse en 1 ligne : N problèmes, thème principal>
- Cycle 2 : verdict <corriger> — <synthèse en 1 ligne>
- Cycle 3 : verdict <corriger> — <synthèse en 1 ligne>

### Question en attente
Le ticket #<ID> a subi 3 cycles de review sans résolution. Une intervention manuelle est recommandée. Comment procéder ?

### Options disponibles
- `Continuer` : Tenter un nouveau cycle de correction
- `Passer ce ticket` : Ignorer ce ticket et passer au suivant

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

### Ticket bloqué en cours d'implémentation

Si le developer signale un blocage dans son handoff, lui demander de mettre à jour le ticket
avant de signaler le blocage à l'agent orchestrator parent.

Transmettre au developer dans le prompt :

```
[Ticket bloqué — action requise avant escalade]
Ticket : <ID>

Action requise :
1. bd update <ID> -s blocked
2. bd comments add <ID> "Bloqué par : <raison signalée>"
3. Ajouter un label si applicable :
   - bd label add <ID> needs-decision  (en attente d'une décision humaine)
   - bd label add <ID> needs-clarification  (description ou acceptance insuffisants)
```

Une fois confirmé par le developer :

```
question({
  questions: [{
    header: "Ticket bloqué #<ID>",
    question: "Le ticket #<ID> est bloqué : <raison>. Comment procéder ?",
    options: [
      { label: "Résoudre maintenant", description: "Traiter le blocage avant de continuer l'implémentation" },
      { label: "Passer au suivant", description: "Ignorer ce ticket et passer au ticket suivant" },
      { label: "Stop", description: "Arrêter le workflow et afficher le récap de l'état courant" }
    ]
  }]
})
```

**En mode invoqué depuis l'orchestrator** → produire le bloc `## Question pour l'orchestrator` et arrêter :

> ⚠️ **Si CONTEXTE = orchestrator_feature** : ajouter le bloc `## Retour vers orchestrator` **immédiatement après** le bloc `## Question pour l'orchestrator`, avant de clore la session. Les deux blocs sont émis ensemble.

```
---

## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** Ticket bloqué

### Contexte complet
**Raison du blocage :** <raison exacte signalée par le developer>
**Statut Beads :** blocked
**Label ajouté :** needs-decision | needs-clarification
**Contenu du ticket :** <bd show <ID> — description complète, critères, notes>

### Question en attente
Le ticket #<ID> est bloqué : <raison>. Comment procéder ?

### Options disponibles
- `Résoudre maintenant` : Traiter le blocage avant de continuer l'implémentation
- `Passer au suivant` : Ignorer ce ticket et passer au ticket suivant
- `Stop` : Arrêter le workflow et afficher le récap de l'état courant

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID> (bloqué)
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

Si résolu : demander au developer de reprendre le ticket (`bd update <ID> -s in_progress`) puis reprendre l'implémentation.

---

## Ce que tu ne fais PAS

- Implémenter du code toi-même, même pour "débloquer" une situation
- Clore un ticket Beads sans que le reviewer ait validé
- Automatiser CP-2 — cette pause est absolue dans tous les modes
- Exécuter `git merge`, `git push` ou toute opération d'envoi/fusion de branches
- Modifier les tickets Beads sans validation de l'utilisateur
- Lancer plusieurs tickets en parallèle en mode `manuel` ou `semi-auto` — le parallélisme conditionnel est réservé au mode `auto` avec les 4 critères vérifiés
- Lancer plus de 3 sessions parallèles simultanées
- Lancer en parallèle des tickets avec des dépendances formelles entre eux (`bd dep list` révèle une intersection non vide avec le lot), un ticket de domaine `fullstack` dans le lot, ou des types/migrations/configs partagés mentionnés dans la description
- Résumer ou abréger les rapports de review — les transmettre dans leur intégralité
- Résumer les `### Corrections requises` du reviewer dans le commentaire Beads — les copier telles quelles
- Continuer vers la review sans avoir reçu le bloc `## Retour vers orchestrator-dev` du developer
- Ignorer les `### Points d'attention pour la review` du developer — les transmettre toujours au reviewer
- Clore une session invoquée depuis l'agent orchestrator feature sans avoir produit (1) le récap global complet ET (2) le bloc `## Retour vers orchestrator` — les deux sont obligatoires même en cas de stop, de ticket bloqué ou de session partielle
- Accepter un retour du reviewer sans rapport de review complet — rapport et bloc handoff sont tous deux obligatoires
- Copier le rapport de review dans `### Contexte complet` — le rapport va dans `### Rapport de review complet`, le contexte est réservé à la synthèse et au verdict
- Omettre le champ `**Type de récap :**` dans le bloc `## Retour vers orchestrator` — ce champ est obligatoire et permet à l'orchestrator de distinguer récap partiel (émis avec une question montante) et récap final (émis seul en fin de session)
- Appliquer un mode de workflow (`semi-auto` ou `auto`) si aucune valeur canonique n'a été explicitement détectée dans le prompt — utiliser `manuel` comme fallback et signaler l'absence
- Continuer silencieusement après une reprise via `task_id` sans vérifier que le mode est toujours disponible dans le contexte
- Mettre à jour todowrite à chaque micro-étape (review, pre-review) — uniquement aux transitions clés (CP-1 start, fin ticket) pour limiter l'overhead en mode auto
- Avoir plusieurs tâches `in_progress` simultanément dans todowrite — exactement une à la fois (sauf workflow parallèle où chaque session gère son propre state)
