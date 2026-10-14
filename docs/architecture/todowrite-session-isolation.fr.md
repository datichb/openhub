> 🇬🇧 [Read in English](todowrite-session-isolation.en.md)

# Todo list et isolation des sessions — Comportement OpenCode

Ce document décrit le comportement interne d'OpenCode pour l'outil `todowrite`,
les contraintes d'isolation qui en découlent, et les décisions d'architecture
prises pour adapter l'utilisation de la todo list dans le hub multi-agents.

> Voir aussi : [task-delegation.fr.md](task-delegation.fr.md) (mécanisme de délégation),
> [`skills/posture/coordinator-posture.md`](../../skills/posture/coordinator-posture.md) (règles d'utilisation, section « Suivi » ; `tool-todowrite` y est fusionnée depuis L5).

---

## Contexte — La découverte

Lors de la conception des améliorations de suivi de progression (v1.5.0+), une question
fondamentale s'est posée : **un sous-agent invoqué via `task` peut-il mettre à jour la
todo list visible par l'utilisateur ?**

L'investigation du code source d'OpenCode (`packages/opencode/src/tool/todo.ts`,
`packages/opencode/src/session/todo.ts`, `packages/core/src/session/sql.ts`)
a révélé une contrainte forte qui impacte directement l'architecture de responsabilité
entre agents.

> Les extraits de code de cette page ont été relevés sur une version d'opencode antérieure à la V2. oh v5 exige
> opencode V2 ; le principe (une todo list par session, sessions enfants pour `task`) reste celui décrit ici.
> Ce qui change en v5 est résumé dans [Sessions v5](#sessions-v5).

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

**Conséquence directe :** seule la todo list de l'agent de la session racine (l'agent d'entrée
du workflow) est visible dans l'interface OpenCode.

---

## Mapping agent → comportement

| Agent | Contexte d'invocation | Session | Todo list visible ? | Responsabilité |
|-------|-----------------------|---------|---------------------|----------------|
| `conductor` | Agent d'entrée (`spec`, `sweep`, `security-fix`) | Racine | ✅ Oui | Maintient la liste des étapes (une entrée par agent et par checkpoint de la carte du workflow) |
| `orchestrator` | Agent d'entrée (`libre`) | Racine | ✅ Oui | Maintient la liste des phases de la feature (planification, specs, audits, implémentation) |
| `orchestrator-dev` | Agent d'entrée (`ticket`, `implement-spec`, `review-feedback`) | Racine | ✅ Oui | Maintient la liste des tickets avec labels de phase |
| `release-manager` | Agent d'entrée (`release`) | Racine | ✅ Oui | Étapes de la release, s'il en tient une |
| `orchestrator-dev` | Invoqué via `task` depuis `orchestrator` (`libre`) | Isolée (enfant) | ❌ Non | Peut maintenir une liste interne (débogage) — non visible |
| `planner`, `pathfinder`, `onboarder`, `debugger`, `designer`, `auditor-subagent` | Invoqués via `task` depuis `orchestrator` ou `conductor` | Isolée (enfant) | ❌ Non | Pas de liste todowrite |
| `developer`, `reviewer`, `documentarian` | Invoqués via `task` depuis `orchestrator-dev` ou `conductor` | Isolée (enfant ; petit-enfant dans `libre`) | ❌ Non | Pas de liste todowrite |

Un agent qui est l'agent d'entrée de son workflow (`auditor` dans `audit`, `debugger` dans `debug` et `incident`,
`pathfinder` dans `spike`, `developer` dans `quick`…) tourne dans la session racine : sa todo list, s'il en tient
une, est visible. Une session enfant d'un fan-out (`ticket`, `sweep-task`) est une session oh à part entière : sa
todo list est celle de sa propre session racine, pas celle de la parente.

---

## Règle de responsabilité

> **L'agent de la session racine (l'agent d'entrée du workflow) est toujours
> le seul responsable de la todo list visible.**

Cette règle implique :

1. **L'orchestrator** (`libre`) maintient une liste avec **1 tâche par phase de la feature** (planification,
   spec UX, spec UI, audit, implémentation) : la granularité est volontairement haute, il suit les phases, pas les
   tickets. Il la met à jour à chaque changement de phase, à la lecture de l'enveloppe de l'agent qu'il a lancé.

2. **L'orchestrator-dev agent d'entrée** maintient sa liste avec des **labels dynamiques de phase**
   pour refléter ce qui se passe réellement dans le sous-agent developer actuellement actif.

3. **L'orchestrator-dev sous-agent** peut maintenir une liste interne pour le débogage de session,
   mais cette liste est invisible et ne remplace pas celle de l'orchestrator.

---

## Règles de mise à jour — Orchestrator (`libre`)

Quand l'orchestrator reçoit l'enveloppe d'un agent qu'il a lancé, il met à jour sa liste **avant d'afficher
l'enveloppe et avant d'appeler l'outil `question`** :

| Enveloppe reçue | Statut | Action todowrite |
|-----------------|--------|-----------------|
| `## Retour vers orchestrator` du planner ou du pathfinder | `découpage-à-valider`, `planification-complète`, `direct` | Planification → `completed`, phase suivante → `in_progress` |
| `## Retour vers orchestrator` du designer | `spec-complète` | Spec UX ou UI → `completed` |
| `## Retour vers orchestrator` d'orchestrator-dev | `en-attente` | Aucune (questions bloquantes relayées, reprise avec `task_id`) |
| `## Retour vers orchestrator` d'orchestrator-dev | `succès`, `partiel`, `bloqué` | Implémentation → `completed` (ou laissée `in_progress` avec le point bloquant) |

**Exemple — planification faite, implémentation en cours :**

```
todowrite({
  todos: [
    { content: "Planification", status: "completed", priority: "high" },
    { content: "Spec UI", status: "completed", priority: "high" },
    { content: "Implémentation", status: "in_progress", priority: "high" }
  ]
})
```

---

## Règles de mise à jour — Orchestrator-dev agent d'entrée

Quand `orchestrator-dev` est invoqué directement, mettre à jour le label de la tâche
à chaque phase clé pour que l'utilisateur sache où en est l'implémentation :

| Étape | Label | Statut |
|-------|-------|--------|
| `cp-plan` (initialisation, `implement-spec`) ou démarrage | `#bd-12 — <titre>` | `pending` |
| CP-1 démarrage | `#bd-12 — <titre> [dev]` | `in_progress` |
| Étape 4 review lancée | `#bd-12 — <titre> [review]` | `in_progress` |
| Étape 5 CP-2 en attente | `#bd-12 — <titre> [CP-2]` | `in_progress` |
| CP-2 commit validé | `#bd-12 — <titre>` | `completed` |
| CP-1 passer | `#bd-12 — <titre>` | `cancelled` |

---

## Sessions v5

- **Session racine** : créée par oh au lancement (`oh run <workflow>`). Son agent est l'agent d'entrée du workflow
  (`entry.agent`, ou `conductor` par défaut). Elle peut être ouverte dans plusieurs clients à la fois (opencode,
  navigateur) ; fermer opencode ne coupe pas la session. La todo list visible est celle de cette session.
- **Sessions enfants** : chaque `task` crée une session enfant. Le démon `ohd` la rattache à la session racine
  pour l'activité et les décisions (section « À traiter » de la vue Sessions), mais la todo list n'est pas
  partagée : elle reste propre à chaque session.
- **Suivi côté oh** : l'avancement du workflow est aussi visible hors d'opencode, dans la vue Sessions de oh et la
  fiche checkpoint (frise `✔ cp-1 → developer → ⏸ cp-2 → ○ cp-3`). Cette frise vient de la machine à états des
  checkpoints, pas de `todowrite` ([ADR-042](./adr/042-checkpoints-headless-decisions.fr.md),
  [ADR-047](./adr/047-session-interaction-daemon.fr.md)).
- **Permissions** : `todowrite` est autorisé explicitement pour `conductor`, `orchestrator`, `orchestrator-dev` et `release-manager`,
  et refusé pour `brief-enricher`.

---

## Ce qui ne change pas

- Les règles fondamentales de la posture de coordinateur s'appliquent sans exception : exactement
  1 tâche `in_progress` à la fois, mise à jour en temps réel, liste complète à chaque appel.
- Les agents `developer`, `reviewer`, `documentarian` n'utilisent pas
  `todowrite` quand ils sont invoqués en tant que sous-agents : leurs sessions sont invisibles.
- L'enveloppe de retour (`## Retour vers <appelant>`, questions bloquantes au statut `en-attente`) et le rapport
  déposé avec `workflow_report` restent le seul canal de communication entre sessions isolées.

---

## Références

- [`skills/posture/coordinator-posture.md`](../../skills/posture/coordinator-posture.md) — Règles d'utilisation (les suffixes de phase sont dans `orchestrator-dev-ticket-workflow`)
- [`skills/orchestrator/orchestrator-protocol.md`](../../skills/orchestrator/orchestrator-protocol.md) — Liste des phases de l'orchestrator (`libre`)
- [`skills/orchestrator/orchestrator-dev-protocol.md`](../../skills/orchestrator/orchestrator-dev-protocol.md) — Labels dynamiques pour orchestrator-dev agent d'entrée
- [`docs/architecture/task-delegation.fr.md`](task-delegation.fr.md) — Mécanisme de délégation via `task`, isolation des sessions, `task_id`
