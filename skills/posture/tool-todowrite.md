---
name: tool-todowrite
description: Utilisation de l'outil todowrite d'OpenCode — quand et comment suivre la progression des tâches dans une session. Couvre le seuil des 3 étapes, la mise à jour en temps réel et la différence avec Beads.
---

# Skill — Outil `todowrite` (OpenCode)

## Rôle

Ce skill est la **source de vérité unique** pour l'utilisation de l'outil `todowrite` d'OpenCode.
Il définit quand et comment utiliser cet outil pour suivre la progression des tâches
au sein d'une session de travail.

Les protocoles `orchestrator` et `orchestrator-dev` référencent ce skill — aucune duplication
de ces règles ne doit exister ailleurs.

---

## Schéma de l'outil

L'outil `todowrite` accepte un paramètre **`todos`** qui est un **tableau** de tâches.

```
todowrite({
  todos: [
    {
      content: "...",      // description de la tâche (obligatoire)
      status: "...",       // état de la tâche (obligatoire)
      priority: "..."      // priorité (obligatoire)
    }
  ]
})
```

### Paramètres d'une tâche

| Paramètre | Requis | Type | Valeurs | Description |
|-----------|--------|------|---------|-------------|
| `content` | ✅ | string | — | Description concise et actionnable de la tâche |
| `status` | ✅ | string | `pending`, `in_progress`, `completed`, `cancelled` | État courant de la tâche |
| `priority` | ✅ | string | `high`, `medium`, `low` | Niveau de priorité |

### États disponibles

| État | Signification |
|------|---------------|
| `pending` | Tâche planifiée, non encore démarrée |
| `in_progress` | Tâche en cours de traitement |
| `completed` | Tâche terminée avec succès |
| `cancelled` | Tâche abandonnée (hors scope, bloquée définitivement, devenue obsolète) |

### Priorités

| Priorité | Usage |
|----------|-------|
| `high` | Critique pour la session — à traiter en premier |
| `medium` | Important mais pas bloquant — ordre normal |
| `low` | Secondaire — peut être reporté si le temps manque |

---

## Contrainte fondamentale

**Exactement une tâche `in_progress` à la fois.**

Cette contrainte reflète le principe de focus : on termine ce qu'on a commencé avant
de passer à autre chose. Si plusieurs tâches sont simultanément `in_progress`,
c'est un signe de dispersion.

> ❌ Ne jamais avoir 0 ou 2+ tâches `in_progress` en même temps (sauf au tout début
> de session quand aucune tâche n'a encore démarré, ou à la fin quand toutes sont terminées).

---

## Mise à jour en temps réel

La liste de tâches doit être mise à jour **à chaque transition d'état** — pas en batch
à la fin de la session.

| Événement | Action attendue |
|-----------|-----------------|
| Nouvelle tâche identifiée | `todowrite` avec la tâche ajoutée en `pending` |
| Démarrage d'une tâche | `todowrite` avec la tâche passée en `in_progress` |
| Fin d'une tâche | `todowrite` avec la tâche passée en `completed` |
| Abandon d'une tâche | `todowrite` avec la tâche passée en `cancelled` |
| Découverte d'une sous-tâche | `todowrite` avec la nouvelle tâche ajoutée |

> ✅ Chaque appel à `todowrite` remplace la liste complète — toujours envoyer
> l'ensemble des tâches (celles terminées incluses), pas seulement le delta.

---

## Quand utiliser `todowrite`

Utiliser `todowrite` quand la demande de l'utilisateur implique **3 étapes distinctes ou plus**.

### Exemples de cas d'utilisation

| Demande | Nombre d'étapes | Utiliser `todowrite` ? |
|---------|-----------------|------------------------|
| "Implémente la feature d'authentification JWT" | 5+ (analyse, backend, tests, review...) | ✅ Oui |
| "Refactorise le service UserService" | 3+ (analyse, refacto, tests) | ✅ Oui |
| "Ajoute une validation sur le champ email" | 3 (validation, test, intégration) | ✅ Oui |
| "Corrige le typo dans le README" | 1 | ❌ Non |
| "Explique comment fonctionne ce composant" | 0 (informatif) | ❌ Non |
| "Quelle est la version de Node utilisée ?" | 0 (informatif) | ❌ Non |

### Règle des 3 étapes

Le seuil de **3 étapes distinctes** est le critère de décision :

- **< 3 étapes** → exécuter directement sans `todowrite`
- **≥ 3 étapes** → initialiser `todowrite` avec la liste des étapes planifiées

> Ce seuil évite la surcharge cognitive pour les tâches simples tout en garantissant
> le suivi pour les tâches complexes.

---

## Quand NE PAS utiliser `todowrite`

| Situation | Raison |
|-----------|--------|
| Requête informationnelle | Pas d'action à suivre — réponse directe attendue |
| Tâche triviale (1-2 étapes) | Overhead supérieur au bénéfice |
| Question de clarification | En attente de réponse utilisateur — pas encore de tâche |
| Lecture ou exploration seule | Pas de modification, pas de suivi nécessaire |

> ❌ Ne jamais utiliser `todowrite` pour tracker des réflexions internes ou des hypothèses
> — uniquement pour les **actions concrètes** à réaliser.

---

## Différence avec Beads

| Aspect | `todowrite` | Beads |
|--------|-------------|-------|
| **Portée** | Session courante uniquement | Persistant entre sessions |
| **Granularité** | Étapes techniques d'une tâche | Tickets métier (feature, bug, task) |
| **Visibilité** | Visible dans la session OpenCode | Stocké dans `.beads/`, versionné |
| **Cycle de vie** | Disparaît à la fin de la session | Workflow complet (open → review → closed) |
| **Qui crée** | L'agent automatiquement | Le `planner` ou l'utilisateur |

### Complémentarité

`todowrite` et Beads sont **complémentaires, pas concurrents** :

- Un **ticket Beads** représente une unité de travail métier traçable
- Les **tâches `todowrite`** représentent les étapes techniques pour implémenter ce ticket

**Exemple concret :**

```
Ticket Beads : #bd-42 — "Implémenter l'endpoint POST /users"

Tâches todowrite pour ce ticket :
1. [pending]      Lire les critères d'acceptance du ticket
2. [in_progress]  Créer le DTO de requête UserCreateDto
3. [pending]      Implémenter le controller avec validation
4. [pending]      Implémenter le service UserService.create()
5. [pending]      Écrire les tests unitaires
6. [pending]      Écrire le test d'intégration
7. [pending]      Passer le ticket en review
```

> Le ticket Beads est clos à la fin du workflow complet.
> Les tâches `todowrite` sont consommées au fur et à mesure de la session.

---

## Exemples d'utilisation

### Initialisation d'une session

```
todowrite({
  todos: [
    { content: "Analyser le ticket #bd-42", status: "pending", priority: "high" },
    { content: "Créer le DTO UserCreateDto", status: "pending", priority: "high" },
    { content: "Implémenter UserController.create()", status: "pending", priority: "high" },
    { content: "Implémenter UserService.create()", status: "pending", priority: "medium" },
    { content: "Écrire les tests unitaires", status: "pending", priority: "medium" },
    { content: "Passer en review", status: "pending", priority: "low" }
  ]
})
```

### Démarrage de la première tâche

```
todowrite({
  todos: [
    { content: "Analyser le ticket #bd-42", status: "in_progress", priority: "high" },
    { content: "Créer le DTO UserCreateDto", status: "pending", priority: "high" },
    { content: "Implémenter UserController.create()", status: "pending", priority: "high" },
    { content: "Implémenter UserService.create()", status: "pending", priority: "medium" },
    { content: "Écrire les tests unitaires", status: "pending", priority: "medium" },
    { content: "Passer en review", status: "pending", priority: "low" }
  ]
})
```

### Transition entre tâches

```
todowrite({
  todos: [
    { content: "Analyser le ticket #bd-42", status: "completed", priority: "high" },
    { content: "Créer le DTO UserCreateDto", status: "in_progress", priority: "high" },
    { content: "Implémenter UserController.create()", status: "pending", priority: "high" },
    { content: "Implémenter UserService.create()", status: "pending", priority: "medium" },
    { content: "Écrire les tests unitaires", status: "pending", priority: "medium" },
    { content: "Passer en review", status: "pending", priority: "low" }
  ]
})
```

### Abandon d'une tâche devenue obsolète

```
todowrite({
  todos: [
    { content: "Analyser le ticket #bd-42", status: "completed", priority: "high" },
    { content: "Créer le DTO UserCreateDto", status: "completed", priority: "high" },
    { content: "Implémenter UserController.create()", status: "completed", priority: "high" },
    { content: "Implémenter UserService.create()", status: "completed", priority: "medium" },
    { content: "Écrire les tests unitaires", status: "completed", priority: "medium" },
    { content: "Ajouter la validation email (déjà existante)", status: "cancelled", priority: "low" },
    { content: "Passer en review", status: "in_progress", priority: "low" }
  ]
})
```

---

## Mauvais usages à éviter

### Ne pas fragmenter à l'excès

```
// ❌ Mauvais — trop granulaire
todowrite({
  todos: [
    { content: "Ouvrir le fichier user.service.ts", status: "pending", priority: "high" },
    { content: "Ajouter l'import de UserDto", status: "pending", priority: "high" },
    { content: "Écrire la signature de la méthode", status: "pending", priority: "high" },
    { content: "Écrire le corps de la méthode", status: "pending", priority: "high" },
    { content: "Sauvegarder le fichier", status: "pending", priority: "high" }
  ]
})

// ✅ Bon — niveau de granularité approprié
todowrite({
  todos: [
    { content: "Implémenter UserService.create()", status: "pending", priority: "high" }
  ]
})
```

### Ne pas utiliser pour des réponses simples

```
// ❌ Mauvais — question simple, pas besoin de todowrite
User: "Quelle est la structure du projet ?"
Agent: todowrite({ todos: [{ content: "Analyser la structure", status: "in_progress", priority: "high" }] })

// ✅ Bon — répondre directement
User: "Quelle est la structure du projet ?"
Agent: "Le projet suit une architecture hexagonale avec les dossiers suivants..."
```

### Ne pas oublier de mettre à jour

```
// ❌ Mauvais — tâche terminée mais toujours in_progress
// (le code a été écrit mais la liste n'a pas été mise à jour)

// ✅ Bon — mise à jour immédiate après chaque étape
```

---

## Règles récapitulatives

| Règle | ✅ / ❌ |
|-------|--------|
| Utiliser pour les demandes avec 3+ étapes distinctes | ✅ |
| Maintenir exactement 1 tâche `in_progress` à la fois | ✅ |
| Mettre à jour en temps réel à chaque transition | ✅ |
| Envoyer la liste complète à chaque appel (pas de delta) | ✅ |
| Utiliser des descriptions concises et actionnables | ✅ |
| Utiliser pour des requêtes informationnelles | ❌ |
| Utiliser pour des tâches triviales (< 3 étapes) | ❌ |
| Fragmenter en micro-étapes (ouvrir fichier, sauvegarder...) | ❌ |
| Avoir 0 ou 2+ tâches `in_progress` simultanément | ❌ |
| Faire un batch de mises à jour en fin de session | ❌ |

---

## Usage par type d'agent

### Contrainte d'isolation des sessions

> ⚠️ Dans OpenCode, chaque agent invoqué via l'outil `task` dispose de sa propre
> session isolée avec son propre `session_id`. La todo list est stockée dans une
> table SQLite clé par `(session_id, position)` — un sous-agent ne peut donc
> **jamais** mettre à jour la liste de son parent.
>
> **Conséquence directe :** seul l'agent de plus haut niveau invoqué directement
> par l'utilisateur maintient une liste visible dans l'interface OpenCode.
> Les listes des sous-agents sont invisibles pour l'utilisateur.

### Tableau de responsabilité

| Agent | Contexte d'invocation | Liste visible ? | Responsabilité |
|-------|-----------------------|-----------------|----------------|
| `orchestrator` | Invoqué par l'utilisateur | ✅ Oui | Maintient la liste des phases + 1 tâche par ticket dev |
| `orchestrator-dev` | Invoqué directement par l'utilisateur | ✅ Oui | Maintient la liste des tickets avec phase en suffixe |
| `orchestrator-dev` | Invoqué via `task` depuis `orchestrator` | ❌ Non (session isolée) | Sa liste est interne — l'orchestrator gère la liste visible |
| `developer-*`, `reviewer`… | Invoqués via `task` | ❌ Non | Sous-agents — todowrite non utilisé |

> **Référence architecture :** voir `docs/architecture/todowrite-session-isolation.fr.md`
> pour l'analyse complète du comportement interne d'OpenCode et le schéma de visibilité.

---

### Suffixes de phase — orchestrator-dev standalone

Quand `orchestrator-dev` est invoqué **directement par l'utilisateur** (sa liste est visible),
mettre à jour le label de la tâche à chaque phase clé pour refléter l'état réel en cours.

| Phase | Label de la tâche | Statut |
|-------|-------------------|--------|
| En attente | `#bd-12 — Titre` | `pending` |
| Implémentation démarrée | `#bd-12 — Titre [dev]` | `in_progress` |
| QA en cours | `#bd-12 — Titre [QA]` | `in_progress` |
| Review en cours | `#bd-12 — Titre [review]` | `in_progress` |
| En attente décision CP-2 | `#bd-12 — Titre [CP-2]` | `in_progress` |
| Terminé (commit validé) | `#bd-12 — Titre` | `completed` |
| Ignoré (skip ou cancelled) | `#bd-12 — Titre` | `cancelled` |

**Exemple de séquence :**

```
// CP-1 démarrage
todowrite({ todos: [
  { content: "#bd-12 — Endpoint POST /users [dev]", status: "in_progress", priority: "high" },
  { content: "#bd-13 — Migration DB users", status: "pending", priority: "high" }
]})

// QA activé
todowrite({ todos: [
  { content: "#bd-12 — Endpoint POST /users [QA]", status: "in_progress", priority: "high" },
  { content: "#bd-13 — Migration DB users", status: "pending", priority: "high" }
]})

// Review lancée
todowrite({ todos: [
  { content: "#bd-12 — Endpoint POST /users [review]", status: "in_progress", priority: "high" },
  { content: "#bd-13 — Migration DB users", status: "pending", priority: "high" }
]})

// CP-2 en attente de décision
todowrite({ todos: [
  { content: "#bd-12 — Endpoint POST /users [CP-2]", status: "in_progress", priority: "high" },
  { content: "#bd-13 — Migration DB users", status: "pending", priority: "high" }
]})

// Commit validé → completed
todowrite({ todos: [
  { content: "#bd-12 — Endpoint POST /users", status: "completed", priority: "high" },
  { content: "#bd-13 — Migration DB users", status: "in_progress", priority: "high" }
]})
```

> Ces suffixes ne s'appliquent qu'en mode standalone. En mode sous-agent (invoqué depuis
> l'orchestrator), la liste est dans une session isolée et invisible pour l'utilisateur —
> la maintenir reste utile pour le débogage de session mais non obligatoire.
