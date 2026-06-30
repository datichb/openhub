> 🇬🇧 [Read in English](todowrite-session-isolation.en.md)

# Todo list et isolation des sessions — Comportement OpenCode

Ce document décrit le comportement interne d'OpenCode pour l'outil `todowrite`,
les contraintes d'isolation qui en découlent, et les décisions d'architecture
prises pour adapter l'utilisation de la todo list dans le hub multi-agents.

> Voir aussi : [task-delegation.fr.md](task-delegation.fr.md) (mécanisme de délégation),
> [`skills/posture/tool-todowrite.md`](../../skills/posture/tool-todowrite.md) (règles d'utilisation).

---

## Contexte — La découverte

Lors de la conception des améliorations de suivi de progression (v1.5.0+), une question
fondamentale s'est posée : **un sous-agent invoqué via `task` peut-il mettre à jour la
todo list visible par l'utilisateur ?**

L'investigation du code source d'OpenCode (`packages/opencode/src/tool/todo.ts`,
`packages/opencode/src/session/todo.ts`, `packages/core/src/session/sql.ts`)
a révélé une contrainte forte qui impacte directement l'architecture de responsabilité
entre agents.

---

## Architecture interne — Comment OpenCode stocke la todo list

### Stockage SQLite par session

La todo list est persistée dans une table SQLite `todo` avec le schéma suivant :

```typescript
export const TodoTable = sqliteTable("todo", {
  session_id: text().notNull().references(() => SessionTable.id, { onDelete: "cascade" }),
  content:    text().notNull(),
  status:     text().notNull(),
  priority:   text().notNull(),
  position:   integer().notNull(),
}, (table) => [
  primaryKey({ columns: [table.session_id, table.position] }),
])
```

**Points clés :**

- La clé primaire est `(session_id, position)` — chaque ligne appartient à une session précise
- Toutes les lectures et écritures utilisent `WHERE session_id = ?`
- Une mise à jour est un **remplacement complet** : DELETE toutes les lignes pour `session_id`, puis INSERT les nouvelles
- Les todos sont cascade-supprimés quand la session est supprimée

### Isolation stricte par session

Quand l'outil `todowrite` est appelé, il utilise `ctx.sessionID` comme clé :

```typescript
// todo.ts — execute()
yield* todo.update({ sessionID: ctx.sessionID, todos: params.todos })
```

Ce `ctx.sessionID` est **l'ID de la session courante de l'agent** — pas une session globale ou partagée.

---

## Le mécanisme Task crée une session isolée

Quand un agent parent invoque l'outil `task`, OpenCode crée **une nouvelle session** :

```typescript
// task.ts
const nextSession = session ?? (yield* sessions.create({
  parentID: ctx.sessionID,     // lien hiérarchique — navigable dans le TUI
  title: params.description + ` (@${next.name} subagent)`,
  agent: next.name,
}))

yield* ops.prompt({
  sessionID: nextSession.id,   // ← le sous-agent utilise CET ID, pas celui du parent
})
```

Le `parentID` crée un lien hiérarchique navigable dans le TUI OpenCode, mais **ne crée
aucun partage d'état** entre les sessions. C'est purement une relation de référence.

---

## Schéma de visibilité par niveau d'invocation

```
Utilisateur
    │
    ▼
Agent A (session_id = A)  →  todo list A  ← VISIBLE par l'utilisateur
    │
    │ task("agent-b", ...)
    ▼
Agent B (session_id = B)  →  todo list B  ← ISOLÉE, invisible
    │
    │ task("agent-c", ...)
    ▼
Agent C (session_id = C)  →  todo list C  ← ISOLÉE, invisible
```

**Conséquence directe :** seule la todo list de l'agent de plus haut niveau invoqué
directement par l'utilisateur est visible dans l'interface OpenCode.

---

## Mapping agent → comportement

| Agent | Contexte d'invocation | Session | Todo list visible ? | Responsabilité |
|-------|-----------------------|---------|---------------------|----------------|
| `orchestrator` | Invoqué par l'utilisateur | Principale | ✅ Oui | Maintient la liste des tickets (1 tâche par ticket) |
| `orchestrator-dev` | Invoqué directement par l'utilisateur | Principale | ✅ Oui | Maintient la liste des tickets avec labels de phase |
| `orchestrator-dev` | Invoqué via `task` depuis `orchestrator` | Isolée (enfant) | ❌ Non | Peut maintenir une liste interne (débogage) — non visible |
| `planner`, `pathfinder`, `onboarder`, `auditor`, `debugger`, `designer` | Invoqués via `task` depuis `orchestrator` | Isolée (enfant) | ❌ Non | Pas de liste todowrite |
| `developer-*`, `reviewer`, `qa-engineer`, `documentarian` | Invoqués via `task` depuis `orchestrator-dev` | Isolée (petit-enfant) | ❌ Non | Pas de liste todowrite |

---

## Règle de responsabilité

> **L'agent actif dans la session directement ouverte par l'utilisateur est toujours
> le seul responsable de la todo list visible.**

Cette règle implique :

1. **L'orchestrator feature** maintient une liste avec **1 tâche par ticket** (pas une tâche
   agrégée "Implémentation — N tickets"). Il met à jour cette liste à chaque checkpoint reçu
   d'`orchestrator-dev` via les blocs `## Question pour l'orchestrator`.

2. **L'orchestrator-dev standalone** maintient sa liste avec des **labels dynamiques de phase**
   pour refléter ce qui se passe réellement dans le sous-agent developer actuellement actif.

3. **L'orchestrator-dev sous-agent** peut maintenir une liste interne pour le débogage de session,
   mais cette liste est invisible et ne remplace pas la responsabilité de l'orchestrator feature.

---

## Règles de mise à jour — Orchestrator feature

Quand l'orchestrator feature reçoit un bloc de retour d'`orchestrator-dev`, il met à jour
sa liste **avant d'afficher le rapport et avant d'appeler l'outil `question`** :

| Bloc reçu | Phase dans le bloc | Action todowrite |
|-----------|--------------------|-----------------|
| `## Question pour l'orchestrator` | `CP-1` (ticket démarre) | Ticket `#bd-XX` → `in_progress` |
| `## Question pour l'orchestrator` | `CP-QA`, `CP-2` | Aucune (déjà `in_progress`) |
| `## Question pour l'orchestrator` | `CP-3` si ticket sauté | Ticket `#bd-XX` → `cancelled` |
| `## Retour vers orchestrator` (partiel, après CP-2 commit) | — | Ticket commité → `completed`, prochain → `in_progress` si applicable |
| `## Retour vers orchestrator` (final) | — | Tous les tickets restants → statut final (`completed` / `cancelled`) |

**Exemple — état après CP-2 commit validé pour #bd-12, CP-3 en mode semi-auto vers #bd-14 :**

```
todowrite({
  todos: [
    { content: "Planification feature", status: "completed", priority: "high" },
    { content: "#bd-12 — Endpoint POST /users", status: "completed", priority: "high" },
    { content: "#bd-14 — Migration DB users", status: "in_progress", priority: "medium" }
  ]
})
```

---

## Règles de mise à jour — Orchestrator-dev standalone

Quand `orchestrator-dev` est invoqué directement, mettre à jour le label de la tâche
à chaque phase clé pour que l'utilisateur sache où en est l'implémentation :

| Étape | Label | Statut |
|-------|-------|--------|
| CP-0 (initialisation) | `#bd-12 — <titre>` | `pending` |
| CP-1 démarrage | `#bd-12 — <titre> [dev]` | `in_progress` |
| Étape 3.3 QA activé | `#bd-12 — <titre> [QA]` | `in_progress` |
| Étape 4 review lancée | `#bd-12 — <titre> [review]` | `in_progress` |
| Étape 5 CP-2 en attente | `#bd-12 — <titre> [CP-2]` | `in_progress` |
| CP-2 commit validé | `#bd-12 — <titre>` | `completed` |
| CP-1 passer | `#bd-12 — <titre>` | `cancelled` |

---

## Ce qui ne change pas

- Les règles fondamentales de `tool-todowrite.md` s'appliquent sans exception : exactement
  1 tâche `in_progress` à la fois, mise à jour en temps réel, liste complète à chaque appel.
- Les agents `developer-*`, `reviewer`, `qa-engineer`, `documentarian` n'utilisent pas `todowrite` —
  ils sont toujours invoqués en tant que sous-agents et leurs sessions sont invisibles.
- Le mécanisme de handoff inter-agents (`## Retour vers <parent>`, `## Question pour <parent>`)
  reste le seul canal de communication entre sessions isolées.

---

## Références

- [`skills/posture/tool-todowrite.md`](../../skills/posture/tool-todowrite.md) — Règles d'utilisation et suffixes de phase
- [`skills/orchestrator/orchestrator-protocol.md`](../../skills/orchestrator/orchestrator-protocol.md) — Règles de mise à jour pour l'orchestrator feature
- [`skills/orchestrator/orchestrator-dev-protocol.md`](../../skills/orchestrator/orchestrator-dev-protocol.md) — Labels dynamiques pour orchestrator-dev standalone
- [`docs/architecture/task-delegation.fr.md`](task-delegation.fr.md) — Mécanisme de délégation via `task`, isolation des sessions, `task_id`
