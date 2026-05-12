> 🇫🇷 [Lire en français](009-inter-agent-handoff-contracts.fr.md)

# ADR-009 — Formalization of inter-agent communication contracts as dedicated skills

## Status

Accepted

## Context

The hub's multi-agent architecture relies on an orchestration chain where agents invoke sub-agents via the `Task` tool and exploit their results to drive decision checkpoints. This chain covered two levels:

**Level 1 — orchestrator-dev → orchestrator:** already formalized in v1.3.0 via `orchestrator/orchestrator-handoff-format`. This skill defined the `## Return to orchestrator` and `## Question for the orchestrator` blocks, shared between the producer and consumer.

**Level 2 — all other sub-agents → their respective consumers:** not formalized. Sub-agents (developer-*, reviewer, qa-engineer, planner, onboarder, debugger, ux-designer, ui-designer, auditor-*) returned free-form text results. The consuming agents (orchestrator-dev, orchestrator) had to manually extract information from this unstructured output, leading to:

- Incomplete summaries: the orchestrator-dev's global recap was poorly populated because it lacked structured data from its sub-agents
- Inconsistent routing: the routing decision to `developer-security` after a security review required manual analysis of the report text, rather than reading a `### Recommended routing` field
- Verbatim corrections lost: Beads comments contained manual summaries of reviewer corrections, not the exact wording required for the developer to act on them
- Review without context: the reviewer received no structured information about which zones the developer considered fragile or technically risky
- Incomplete checkpoints: CP-spec and CP-audit at the orchestrator level were built from free-form text returned by design and audit agents, with no guarantee of completeness

The problem manifested concretely in orchestrator-dev's global recap: the `### Attention points` section was systematically empty or superficial, because the information existed in the sub-agents' outputs but wasn't structured enough to be reliably aggregated.

## Decision

Formalize **all** inter-agent communication contracts as dedicated skills, following the same pattern established by `orchestrator/orchestrator-handoff-format`:

1. **One skill per producer-consumer pair** (or per agent family), injected in both the producing agent and the consuming agent — guaranteeing a shared contract without the risk of desynchronization.

2. **Standardized `## Return to <consumer>` block** for each sub-agent, produced at the end of the session when invoked from its parent. Contains: the complete output (never summarized), actionable metadata fields (status, routing, verdict), and structured information ready to be passed to the next step in the chain.

3. **The consumer agent is responsible for detecting the presence of the block** and explicitly requesting it from the producer if absent — it never builds a checkpoint from incomplete or unstructured input.

4. **Skills created:**

| Skill | Producer | Consumer | Key fields |
|-------|---------|---------|------------|
| `developer/developer-handoff-format` | developer-* | orchestrator-dev | Files modified, acceptance criteria checked, **points of attention for the reviewer**, status |
| `reviewer/reviewer-handoff-format` | reviewer | orchestrator-dev | **Actionable verdict** (commit/fix/fix-security), corrections verbatim, **recommended routing** |
| `qa/qa-handoff-format` | qa-engineer | orchestrator-dev | Tests written, criteria checked, non-testable zones |
| `auditor/audit-handoff-format` | auditor-* | orchestrator | Vulnerability table, prioritized recommendations, residual risk |
| `design/design-handoff-format` | ux-designer, ui-designer | orchestrator | Complete spec, **implementation constraints**, open points |
| `planning/planner-handoff-format` | planner | orchestrator | Complete tickets table with planned agents and dependencies |
| `planning/onboarder-handoff-format` | onboarder | orchestrator | Stack, conventions, technical debt, uncertainty zones |
| `quality/debugger-handoff-format` | debugger | orchestrator | Root cause with certainty level, impact, emergency actions |

5. **Cascading exploitation:** the fields of each block are explicitly used in the consumer's protocol:
   - Developer's `### Points of attention` are transmitted verbatim to the reviewer
   - Reviewer's `### Required corrections` are copied verbatim into the Beads comment (no manual summary)
   - Reviewer's `### Recommended routing` determines whether to go to `developer-security` or the initial agent
   - QA's uncovered criteria are transmitted to the reviewer
   - `orchestrator-dev`'s global recap aggregates attention points from the entire chain

## Consequences

### Positive

- **Complete summaries:** orchestrator-dev's global recap is now fed by structured data from all sub-agents — the `### Attention points` section is populated from reviewer, QA, and developer data.
- **Deterministic routing:** the decision to route to `developer-security` is based on the `### Recommended routing` field, not on manual analysis of the review text.
- **Reliable corrections:** Beads comments contain the reviewer's exact wording, ready for the developer to apply — no information loss through manual summarization.
- **Informed review:** the reviewer receives the developer's attention points for each ticket, allowing them to focus on the sensitive zones.
- **Reliable checkpoints:** CP-spec, CP-audit, CP-onboard are built from structured blocks with mandatory fields — an absent or incomplete block triggers an explicit request before continuing.
- **Zero desynchronization:** by injecting the same skill into producer and consumer, any format change automatically propagates to both sides.

### Negative / trade-offs

- **More skills injected:** agents now receive more skills, increasing the size of assembled agent files at deploy time. This is acceptable given that the skills are injected once at deploy and not at inference time.
- **Strict contract:** sub-agents that do not produce the expected block trigger a retry request from the consumer. This is an intentional behavior — an incomplete result must be explicitly signaled rather than silently ignored.
- **Dual injection obligation:** adding a new handoff skill requires updating two frontmatters (producer + consumer). This rule is documented in the contribution guide's checklist.

## Rejected Alternatives

**Structured parsing of free-form text at the consumer level:** instruct orchestrator-dev and orchestrator to parse the sub-agents' text to extract the required information. Rejected because it creates a dependency on the sub-agents' exact wording, is fragile over time, and doesn't guarantee completeness — the parser may miss a mention or misinterpret a formulation.

**Shared state file between agents:** store the results in a JSON file in `.beads/` or a similar location, which each agent reads and writes. Rejected because it creates a tight coupling to the filesystem, makes the workflow non-reproducible between sessions, and contradicts the hub's stateless architecture principle.

**Extending the existing `orchestrator-handoff-format`:** add all new formats to the single existing skill. Rejected because the skill would become a sprawling monolith covering 10 different agent pairs — difficult to maintain and understand. The domain-per-skill approach is more consistent with the hub's organization.
