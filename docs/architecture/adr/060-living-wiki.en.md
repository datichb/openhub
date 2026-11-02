> [Lire en français](060-living-wiki.fr.md)

# ADR-060 — Living documentation wiki: agents propose, the user validates, oh applies and checks

## Status

Accepted — decision applied since June 2026 (tree, tags, god nodes) and October 2026 (`wiki` MCP server, proposal queue, checks by oh); recorded here after the fact, as [the architecture document](../living-wiki.en.md) announced. Builds on [ADR-039](039-declarative-workflows-oh-v1.en.md) (workflows, `writes:`), [ADR-042](042-checkpoints-headless-decisions.en.md) (decision cards) and [ADR-047](047-session-interaction-daemon.en.md) (the daemon that serves the tools); the team wiki of the team-state stays the one of [ADR-024](024-team-state-repository.en.md)

## Date

2026-11-02

## Context

The context of a project (stack, conventions, business rules, decisions) lived in four flat files (`ONBOARDING.md`, `CONVENTIONS.md`, `docs/context/technical.md`, `docs/context/business/<domain>.md`). Every agent read them whole, without knowing what was certain and what was assumed. They aged without anyone noticing, and capitalising what a session discovered depended on the `documentarian` being in the workflow — absent from 8 workflows out of 14 — or on coordinators copying it, which lost information (modernisation report, § 5.3: [`rapport-modernisation-2026-10.md`](../../dev/rapport-modernisation-2026-10.md), in French).

A project knowledge base was needed that is read in a targeted way (few tokens), honest about its reliability, enriched by all agents without any of them being able to write anything anywhere, and checked without asking an agent to do it.

## Decision

The project carries a **living documentation wiki** in `docs/wiki/` (detail: [Living Documentation Wiki](../living-wiki.en.md)), with these rules:

1. **A single tree**: `index.md` (global map, read first), `glossary.md`, `technical/`, `business/`, `specs/`, `decisions/`, `postmortems/`. `ONBOARDING.md` is only a summary pointing to the wiki. `wiki_check` reports a page outside that tree.
2. **Targeted reading**: an agent reads `index.md`, then a single page or section (skill `wiki-navigation`, tools `wiki_index` and `wiki_read`). At the `index` instruction level of the project, the index and the god nodes enter the prompt.
3. **Confidence tags**: each enrichment carries `CONFIRMÉ` (observed, with `file:line`), `DÉDUIT` or `INCERTAIN`, the agent, the date and the source. The page frontmatter sums up the confidence.
4. **God nodes computed by oh**, not by an agent: a concept cited by at least two pages (maps aside). The `services/wiki` service recomputes them, with their criticality, and rewrites the index table.
5. **The agent proposes, the user validates, oh applies**: an agent does not modify the wiki. It calls `wiki_propose` (`wiki` MCP server, injected in the bundle of a project that has a `docs/wiki/`); oh sets the author, cleans and bounds the content, refuses any reference to oh session data, and queues the proposal (`wp-<n>`, per project, in the database). The commit checkpoint card shows the proposals of the session: the user accepts, edits or refuses each (a refusal keeps its reason, `wiki_pending` shows it to the agents so they do not propose the same thing again). **oh applies** the accepted proposals itself when the checkpoint passes, deterministically, with the tag line written by oh: they are committed with the code. Workflows without a commit leave the queue to the `doc-sync` workflow. Only the `documentarian` has `wiki_apply`.
6. **Deterministic check by oh** (`oh wiki check`, `wiki_check`): frontmatter, links, tags, freshness (a page older than the last commit of a file it cites), god nodes, tree. Restricted to a change, it detects **drift** (a `CONFIRMÉ` page the change contradicts, a god node touched without an update): the `reviewer` turns it into "Documentation" findings. `oh wiki check --fix` recomputes confidence and god nodes.
7. **Remote session**: same tools on the job's clone; **nothing is applied in the job**, the proposals come back in the `wiki.json` artifact and are validated on the machine.
8. **Limited direct writes**: three workflows write a folder of the wiki (`spec`, `incident`, `spike`), declared by `writes:` and committed alone; the `onboarder` creates the wiki, then only proposes (re-onboarding).

## Consequences

### Positive

- Agents read little and know what is reliable.
- Capitalisation no longer depends on the `documentarian` being present: every workflow, read-only ones included, proposes.
- No write escapes the user; application, numbering and tags are code, not prompt.
- The wiki is checked without a model (`oh wiki check` needs no model); the oh repository passes its own check (`TestOwnWiki`).

### Negative / trade-offs

- A wiki to maintain: without validating proposals or running `doc-sync`, the queue grows (`oh wiki stats` shows it).
- The commit checkpoint card carries one more decision load.
- Computing god nodes is heuristic (bold terms, code identifiers): a badly detected concept is fixed by hand in the index.
- A wiki that was never committed has nothing to synchronise: the synchronisation mark (`synced:`) lives in the repository, with the wiki.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Keep the flat files | Read whole at each session, with no confidence level and no check |
| Let agents write the wiki directly | No user control; uncleaned content; conflicts between parallel sessions |
| Go through the `documentarian` for every write | Absent from most workflows; one more agent turn (cost, non-determinism) in the middle of the workflow |
| Propose in the report and let the coordinator copy | Loses information (retranscription) — hence `workflow_report` ([ADR-059](059-return-envelope-and-workflow-report.en.md)) and the `wiki` server |
| God nodes counted by an agent | Not reproducible; oh computes them from the pages |
| One wiki for all projects (the team's) | The team-state wiki is cross-project; the project wiki must travel with the branch and the code |
