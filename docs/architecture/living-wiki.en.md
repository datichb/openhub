# Living Documentation Wiki

The **living documentation wiki** is the contextual documentation system for projects
managed by the hub. It replaces the old flat files (`ONBOARDING.md`, `CONVENTIONS.md`,
`docs/context/`) with a structured, navigable arborescence enriched incrementally.

---

## Concept

Inspired by Graphify's "knowledge graph" concept, the living documentation wiki is built
on two fundamental ideas:

**1. God nodes** — some concepts are more connected than others in a codebase.
They appear in multiple domains (technical AND business) and represent the zones
where critical decisions are concentrated. The wiki explicitly identifies them in `index.md`
to guide agents toward the essentials first.

**2. Confidence tags** — each enrichment carries an explicit confidence level
(`CONFIRMED`, `INFERRED`, `UNCERTAIN`) with the source (file + line when possible).
Future agents immediately know whether they can use information directly or need to verify it.

---

## Structure

```
docs/wiki/
├── index.md                    ← global map — always read first
├── technical/
│   ├── architecture.md         ← dominant patterns, layering, structural decisions
│   ├── stack.md                ← full stack, versions, key libraries
│   ├── tests.md                ← strategy, conventions, thresholds, frameworks
│   └── conventions.md          ← naming, git, linting, config, team patterns
└── business/
    ├── index.md                ← business domain map
    └── <domain>.md             ← business rules, flows, entities, risks
```

At the project root:
```
ONBOARDING.md                   ← minimal summary (15-25 lines), redirects to the wiki
```

---

## Confidence tag format

```markdown
- Description of the observation
  — `CONFIRMED` · <agent> · <YYYY-MM-DD> · <file:line>

- Description of an inferred observation
  — `INFERRED` · <agent> · <YYYY-MM-DD> · <file>

- Uncertain description
  — `UNCERTAIN` · <agent> · <YYYY-MM-DD>
```

| Tag | Meaning |
|-----|---------|
| `` `CONFIRMED` `` | Direct observation in code, file + line cited |
| `` `INFERRED` `` | Contextual reasoning from multiple files |
| `` `UNCERTAIN` `` | Hypothesis or undocumented convention, to be validated |

> **Note:** The hub uses French tags (`CONFIRMÉ`, `DÉDUIT`, `INCERTAIN`) in the wiki
> files since the hub operates primarily in French.

---

## Navigation protocol (skill `wiki-navigation`)

The `shared/wiki-navigation` skill is **Bucket A** — always active in all agents
that consult a project's context.

**Fundamental rule:** read `docs/wiki/index.md` first, then load only the page
relevant to the current task. Never read the full wiki by default.

```
Current task
     │
     ▼
docs/wiki/index.md (always)
     │
     ├── Implementation / naming  → technical/conventions.md
     ├── Architecture / layering  → technical/architecture.md
     ├── Stack / dependencies     → technical/stack.md
     ├── Tests / coverage         → technical/tests.md
     ├── Specific business domain → business/<domain>.md
     └── General context          → index.md is enough
```

---

## God node algorithm

A concept becomes a **god node** when it appears in ≥ 2 distinct wiki pages.
The `documentarian` reevaluates the table after each enrichment:

1. Identify concepts mentioned in the modified page
2. Count how many distinct pages each concept appears in
3. If ≥ 2 pages → god node candidate → add to `index.md`
4. Criticality: `Critical` (≥ 4 pages or in "Active critical points"), `High` (3 pages), `Normal` (2 pages)

---

## Generation and enrichment

### Initial generation (onboarder)

The `onboarder` generates the wiki in Phase 5, after validation of the context report.
All pages are created with the canonical format defined in the `doc-wiki-protocol` skill.

### Incremental enrichment (all agents)

After each report (audit, diagnosis, implementation, review, QA), agents identify
discoveries to capitalize via the `shared/living-docs-enrichment` skill:

1. The agent consolidates enrichments and proposes them with their confidence tags
2. The user confirms
3. The agent delegates to the `documentarian` via `task`
4. The `documentarian` enriches the targeted pages and reevaluates god nodes

### Re-onboarding

If `docs/wiki/index.md` already exists, the `onboarder` proposes:
- **Incremental enrichment** (recommended) — via `living-docs-enrichment`
- **Full rewrite** — with a warning about loss of accumulated enrichments
- **Keep as is**

---

## Before / after comparison

| Before | After |
|--------|-------|
| 4 flat files (`ONBOARDING.md`, `CONVENTIONS.md`, `docs/context/technical.md`, `docs/context/business/<domain>.md`) | Structured wiki arborescence (`docs/wiki/`) |
| Agents potentially read all context each session | Agents read `index.md` (40-80 lines) then a single page |
| No confidence level on information | 3 levels: `CONFIRMED` / `INFERRED` / `UNCERTAIN` |
| Important concepts not identified | Explicit god nodes in `index.md` |
| `CONVENTIONS.md` read in full by each agent | `conventions.md` loaded only when relevant |
| No navigation protocol | `wiki-navigation` Bucket A skill in all agents |

---

## References

- Skill `shared/wiki-navigation` — navigation protocol + god node algorithm
- Skill `documentarian/doc-wiki-protocol` — canonical formats + enrichment rules
- Skill `shared/living-docs-enrichment` — enrichment workflow delegated to documentarian
