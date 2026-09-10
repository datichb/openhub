> [Lire en francais](inter-agent-interruption.fr.md)

---
title: Inter-agent session interruption mechanism
description: Reference guide for the mechanism that allows agents invoked via task to send intermediate recaps and questions back to the parent agent, bypassing the technical limitation of the task tool which only returns the last message.
---

# Inter-agent session interruption mechanism

## Why this mechanism

When an agent is invoked via the `task` tool, only the **last text message** from the child session is returned to the parent agent. Everything the agent displays during the session (phase recaps, pause contexts, intermediate results) is invisible to the parent agent and to the user in the parent session.

The problem is particularly acute for agents with a phased workflow (planner, onboarder, auditor, debugger) or with validation checkpoints (orchestrator-dev): the user sees nothing until the child session completes entirely, which can take several minutes.

## Solution principle

Instead of using the `question` tool (which pauses the child session but remains invisible to the parent agent), agents in `orchestrator_feature` mode:

1. Produce an `## Intermediate return to orchestrator` block containing the phase recap or current context
2. Produce a `## Question for the orchestrator` block containing the question, options, and `task_id`
3. **Terminate their session**

The parent orchestrator:
1. Receives these blocks in the final message of the child session
2. Displays the intermediate block as text in the conversation
3. Relays the question to the user via the `question` tool
4. Re-invokes the agent with `task_id` + the answer — the agent reloads its complete history and continues

## Agents implementing this mechanism

| Agent | Granularity | Interruption type |
|-------|-------------|-------------------|
| **orchestrator-dev** | High-stakes CPs (CP-2, blocking, blocked ticket) + intermediate CPs (CP-1, CP-3, branch) | Systematic and ad hoc |
| **planner** | End of each phase (0 to 5) + ad hoc pauses | Systematic |
| **pathfinder** | Critical clarification detected | Ad hoc only |
| **onboarder** | End of each phase (0 to 4) + ad hoc pauses | Systematic |
| **auditor** (coordinator) | End of each phase (0 to 3) + ad hoc pauses | Systematic |
| **debugger** | End of each phase + confirmations for irreversible actions | Systematic |
| **designer** | Critical clarification (design system, insufficient user information, unspecified mode) | Ad hoc only |

## Block format

### `## Intermediate return to orchestrator` block

```markdown
## Intermediate return to orchestrator

**Agent:** <agent name>
**Phase:** X — <phase title>
**task_id:** <current sessionID>

**Summary:** <2-3 sentences about what was done in this phase>
**Key points:** <short list — important discoveries, decisions, blockers>
```

### `## Question for the orchestrator` block

```markdown
## Question for the orchestrator

**Phase:** X
**task_id:** <current sessionID>

**Context:** <explanation of why this question>

**Question:** <exact question text>

**Options:**
- `<option-label-a>` — <description>
- `<option-label-b>` — <description>

**Resume instruction:** "Answer Phase X <agent>: [option]. Resume from <interruption point>."
```

### orchestrator-dev variant: `## Question pour l'orchestrator` (without accent)

orchestrator-dev uses the variant without accent. Both blocks are semantically equivalent. The orchestrator must detect both.

## Invocation context detection

Agents detect their invocation context via the marker in the prompt:

```
[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session...
```

This marker is injected by the orchestrator agent in each `task(...)` invocation.

### Behavior by context

| Context | `question` call | Structured blocks | Session termination |
|---------|----------------|-------------------|---------------------|
| **standalone** | Used normally | Not produced | Session stays open |
| **orchestrator_feature** | Forbidden | Produced at each checkpoint | Mandatory after the blocks |

## Session resumption with task_id

The `task_id` is the OpenCode session ID — a persistent session with its complete message history.

### Resumption flow

```
1. Agent (e.g.: planner Phase 1) produces the blocks + terminates
   → task_result contains ## Intermediate return + ## Question for the orchestrator (task_id: "sess_abc")

2. Orchestrator displays the intermediate recap as text

3. Orchestrator asks the question via question() → user answers

4. Orchestrator re-invokes:
   task(
     subagent_type: "planner",
     task_id: "sess_abc",
     prompt: "Answer Phase 1 planner: phase-2. Resume from Phase 2."
   )

5. The planner reloads the complete history of "sess_abc"
   → Receives the new message with the answer
   → Continues from Phase 2 with all previous context
```

### Risk: session not found

If OpenCode restarts between the upstream question and the re-invocation, the session may no longer exist.

The orchestrator must handle this case: if the re-invocation with `task_id` does not produce a coherent result, offer the user to restart from the beginning.

## Implementing this mechanism in a new agent

### 1. Add invocation context detection

In the agent's workflow skill, add at the beginning of the file:

```markdown
### Invocation context detection

At startup, detect if the prompt contains `[CONTEXTE] Invoqué depuis l'orchestrateur feature`. If yes:
- Store **CONTEXT = orchestrator_feature**
- Confirm: `[<agent>] Context detected: interruption mode active.`
```

### 2. Add the block format

Define the block format for each checkpoint:

```markdown
### Return format — ABSOLUTE RULE (orchestrator_feature)

At EACH checkpoint:
1. Produce the recap as text
2. Produce ## Intermediate return to orchestrator
3. Produce ## Question for the orchestrator
4. TERMINATE THE SESSION
```

### 3. Modify each call to the question tool

For each `question({...})` call:

```markdown
**If CONTEXT = standalone:**
```
question({...})  [unchanged]
```

**If CONTEXT = orchestrator_feature:**
```markdown
## Intermediate return to orchestrator
...
## Question for the orchestrator
...
```
→ TERMINATE THE SESSION
```

### 4. Add to the agent file

In `agents/<family>/<agent>.md`, add an "Invocation context" section (see `agents/planning/pathfinder.md` for an example).

### 5. Update orchestrator-protocol.md

In the agent's invocation section in `skills/orchestrator/orchestrator-protocol.md`:
- Add the `[CONTEXTE]` marker in the invocation
- Add the "Receiving an upstream question from <agent>" section
- Update the transcription templates

### 6. Update retranscription-coordinateur.md

Add new rows in the "Rules by return type" table.

## Orchestrator side: receiving and relaying

Upon receiving each result from a `task` invocation, the orchestrator agent must:

1. **Detect the return type**:
   - Contains `## Question for the orchestrator` (or `## Question pour l'orchestrator`) → upstream question
   - Contains `## Retour vers orchestrator` without an upstream question → final return

2. **For a final return**:
   - Display the `## Intermediate return to orchestrator` blocks as text, in order (if present)
   - Transcribe the fields from the structured `## Retour vers orchestrator` block in a formatted manner (all sections: integrated report, tables, status — see `retranscription-coordinateur` skill)
   - Then only call `question` for the user checkpoint

3. **For an upstream question**:
   - Display the `## Intermediate return to orchestrator` as text
   - Read the `## Question for the orchestrator` — retrieve question, options, task_id
   - Relay the question via `question()`
   - Re-invoke with task_id + answer + [CONTEXTE] marker
   - Repeat until the final return

## Known limitations

- **Session not found**: if OpenCode restarts, the `task_id` may no longer be valid. See "Risk: session not found" above.
- **Mode transmission**: for orchestrator-dev, the workflow mode (manual/semi-auto/auto) must be retransmitted in each re-invocation via `task_id`.
- **Block accumulation**: if multiple phases end without a question (automatic transition), the intermediate blocks accumulate in the final message — the orchestrator agent must display all of them in order.
