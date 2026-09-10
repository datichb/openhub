> [Lire en francais](authoring-skills.fr.md)

> 🏗️ This guide covers the **qualitative methodology** for writing skills: TDD, SDO, anti-patterns, governance.
> For structural design (agent vs skill, buckets, checklists), see [authoring.fr.md](authoring.fr.md).

# Guide — Skill authoring methodology

---

## 1 — Skill types

Before writing a skill, identify its type. The type determines the expected format and the pitfalls to avoid.

| Type | Role | Dominant format | Examples |
|------|------|----------------|---------|
| **Technical** | Procedural workflow — guides the agent step by step | Numbered phases, templates, self-checks | `debugger-workflow`, `planner-workflow`, `auditor-workflow` |
| **Pattern** | Recurring constraint — enforces a behavior at all times | ✅/❌ rules, violation examples, triggers | `posture/tool-question`, `posture/coordination-only` |
| **Reference** | Catalog / lookup — consulted on demand | Tables, structured lists, source of truth | `shared/hub-workflow-reference`, `developer/dev-standards-*` |
| **Discipline** | Cross-cutting behavioral norm — shapes the overall posture | Principles, absolute prohibitions, examples | `posture/expert-posture`, `posture/retranscription-coordinateur` |

> A skill can be hybrid (e.g.: Technical + Reference), but one type always dominates. Identify the dominant type to choose the main format.

---

## 2 — TDD for skills

Directly writing a skill often produces content that is too vague, circumventable, or incomplete. TDD forces you to start from the expected behavior, not the rule.

### RED — Write the test scenario first

Before writing a single line of skill, draft in natural language:

**Nominal case:**
> "When [triggering situation], the agent must [precise and observable behavior]."

Example:
> "When the agent receives a handoff without a documented completion gate, it must block the CP-feature construction and ask a question via the `question` tool."

**Rationalization case:**
> "If the agent can justify [behavior to avoid] by saying [plausible rationalization], the skill must explicitly prohibit it."

Examples:
> "If the agent says 'the gate is implicitly passed since the implementation is finished', the skill must prohibit this assumption."
> "If the agent says 'the question takes too long, I'll continue', the skill must prohibit bypassing."

---

### GREEN — Minimal skill that passes the scenario

Write only what is necessary to cover the RED cases. Rules:

- ✅ Cover the nominal case with an actionable rule
- ✅ Cover each rationalization case with an explicit prohibition
- ❌ Do not anticipate untested cases — they will come at REFACTOR
- ❌ Do not write "just in case" content — each line must respond to a scenario

---

### REFACTOR — Close the loopholes

After having a GREEN skill, re-read by asking for each rule:
> "How could a model respect the letter of this rule while violating its spirit?"

Each identified loophole → add a row in the **rationalization table** and an additional rule if necessary.

**Systematic additions at REFACTOR:**
1. Rationalization table (see section 4)
2. Red Flags list — list of signals indicating the agent is deviating
3. Mandatory self-checks if the skill is of Technical type

---

## 3 — SDO — Skill Description Optimization

The `description:` field in the frontmatter is used by OpenCode to dynamically select skills. A bad description = skill never loaded at the right time.

### SDO criteria

**Rich and discriminating description**

The description must allow OpenCode to distinguish this skill from all others.

❌ Too generic:
```yaml
description: Best practices guide for debugging
```

✅ Discriminating:
```yaml
description: Mandatory completion gate before any DONE — 3 checks (tests pass,
  observable behavior conforms, regressions documented). Blocking if absent. Load
  in orchestrator-protocol before CP-feature construction.
```

**Keyword coverage**

Include synonyms and alternative phrasings that the user or agent might use.

Example for a completion skill:
- "completion gate" AND "verification before DONE" AND "3 checks" AND "CP-feature"

**Token efficiency**

- ≤ 2 sentences in `description:` — beyond that, the description is truncated or ignored
- Condense without losing discriminating keywords
- Put the most important keywords first

**Cross-references**

If two skills could be confused, the description should point to the other:
```yaml
description: Transcription protocol for coordinators — verbatim display rules
  for agent results. Complementary to posture/coordination-only (tool restrictions).
```

---

## 4 — Rationalization table

Standard template to include in the REFACTOR section of any operational skill:

```markdown
## Rationalization table

| At-risk rationalization | What prohibits it |
|------------------------|-------------------|
| "[Rationalization phrasing 1]" | Section X — "[Exact rule that prevents it]" |
| "[Rationalization phrasing 2]" | Section Y — "[Exact rule that prevents it]" |
| "[Rationalization phrasing 3]" | ❌ explicit rule line Z |
```

**Frequent rationalizations to systematically test:**

| Rationalization | Typical phrasing |
|-----------------|-----------------|
| Implicit is sufficient | "The result implies that X, so I don't need to verify" |
| Path optimization | "I can skip this step if the context is clear" |
| Favorable interpretation | "The rule doesn't say I can't do Y" |
| Urgency | "In this case, the usual rules don't apply" |
| Silent delegation | "I'll assume the previous step was done" |

---

## 5 — Anti-patterns

### Narrative without actionable rule

❌ Problem:
```markdown
It is important to always check the tests before finishing.
Code quality is a shared responsibility.
```

✅ Correct:
```markdown
Before any `DONE`, execute the following 3 checks — if one fails, block:
1. Tests pass (or documented justification)
2. Observable behavior conforms to the spec
3. No undocumented regressions
```

---

### Multi-language dilution

A bilingual FR/EN skill in the same file forces the model to average the two instructions — rules weaken each other.

❌ Problem:
```markdown
Ne jamais résumer le rapport. / Never summarize the report.
```

✅ Correct: one skill per target language, or skill in English with technical terms only (without bilingual narrative prose).

---

### Generic labels in `description:`

❌:
```yaml
description: Best practices guide
description: Standard protocol
description: Instructions for the agent
```

✅: See SDO criteria — always cite the trigger + the imposed behavior.

---

### Code in flowcharts

Inline code blocks in a Mermaid or ASCII flowchart are sometimes executed by the model instead of being read as documentation.

❌:
```
flowchart TD
  A["bd create 'Title' --json"] --> B["T_ID=$(echo $T | jq '.id')"]
```

✅: Pseudo-code in natural language in flowcharts; actual code in separate ` ```bash ``` ` blocks.

---

### Over-specification

Specifying every micro-decision kills the model's judgment on uncovered cases.

❌: 800-line skill that covers every imaginable edge case → the model no longer retains the important rules.

✅: Skill focused on the 5-10 critical rules + rationalization table for edge cases. Uncovered cases benefit from general judgment.

---

## 6 — Final validation checklist

Before merging or declaring a skill complete:

**Content**
- [ ] RED scenario written (nominal case + rationalization cases)
- [ ] Minimal GREEN skill that passes the scenario
- [ ] REFACTOR performed — loopholes closed
- [ ] Rationalization table present (if operational skill)
- [ ] Anti-patterns verified (narrative, bilingual, generic, flowchart code, over-spec)

**SDO**
- [ ] `description:` ≤ 2 sentences, discriminating keywords, cross-refs if needed
- [ ] `bucket:` filled in (A or B)
- [ ] `name:` matches the file path

**Integration**
- [ ] Bucket A: skill in `skills:` of the relevant agents + `skills.fr.md` matrix updated
- [ ] Bucket B: skill in `native_skills:` of the relevant agents + trigger documented
- [ ] `docs/architecture/skills.fr.md` — new entry in the correct domain
- [ ] If new agent created: `skills/shared/hub-workflow-reference.md` updated

---

## 7 — Governance rule

> **Any new agent added to the hub must include an update to `skills/shared/hub-workflow-reference.md`.**
>
> This skill is the source of truth for the agent catalog. Without this update, the planner and orchestrator do not know the new agent and cannot route to it.

For the complete structural checklist (frontmatter, permissions, placement in `agents/`), see [authoring.fr.md](authoring.fr.md).
