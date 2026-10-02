> [Lire en francais](037-documentation-policy.fr.md)

# ADR-037 — Documentation Policy

## Status

Accepted

## Date

2026-10-02

## Context

Between July and October 2026, the openhub codebase grew significantly: 1,171 commits over 6 months, 37 Go packages, 19 agents, 197 skills, and 7 MCP servers. However, the documentation-to-code commit ratio dropped from 58% (July) to 6% (October), resulting in:

- 285 commits undocumented in the CHANGELOG
- 6 major subsystems shipped without any user-facing documentation (parallel mode, sweep mode, review feedback, notifications, Google Slides MCP, plugin RTK updates)
- Factual errors in existing documentation (features documented but not implemented)
- No CI checks preventing documentation drift

A retrospective audit (Sprint 1-4) was required to close the gap. This ADR codifies the rules to prevent recurrence.

## Decision

### 1. Documentation companion rule

Every commit with a `feat:` prefix that introduces a user-facing change (new CLI command, new flag, new MCP tool, new workflow, new agent capability) **must** have a corresponding `docs:` commit in the same PR or within 5 calendar days. "User-facing" means any change that modifies behavior observable by `oh --help`, the TUI, or agent prompts.

Internal refactors (`refactor:`), test-only changes (`test:`), and CI changes (`ci:`) are exempt.

### 2. CHANGELOG discipline

- `CHANGELOG.md` **must** be updated before any release tag (`v*`)
- The `[Unreleased]` section must not fall more than **50 commits** behind HEAD (enforced by CI)
- The CHANGELOG is maintained in **French** (project convention); this may be revisited if the project internationalizes

### 3. Bilingual requirement

- **Mandatory bilingual** (FR + EN): `docs/guides/`, `docs/reference/`, `docs/architecture/adr/`
- **Exempt from bilingual**: `docs/dev/` (internal developer notes, monolingual by convention), `docs/design/` (visual specs)
- Files use the naming convention `<slug>.{en,fr}.md` with a language switcher link on line 1
- CI enforces parity: every `.fr.md` must have a `.en.md` counterpart and vice versa (enforced in bilingual-required directories)

### 4. ADR requirement

An Architecture Decision Record is required when:
- A design choice involves 2 or more alternatives considered
- The change affects cross-cutting concerns (security model, data model, agent permissions, CI pipeline)
- The change is irreversible or expensive to reverse

ADRs follow the existing format: Status, Date, Context, Decision, Consequences, Alternatives Considered. Numbered sequentially (`NNN-kebab-case.{en,fr}.md`).

### 5. CI enforcement

Two new CI checks are added to `.github/workflows/ci.yml`:
- **Changelog freshness**: fails if CHANGELOG.md has not been modified within the last 50 commits on the PR branch
- **Bilingual parity**: fails if any `.fr.md` in `docs/guides/`, `docs/reference/`, or `docs/architecture/adr/` lacks a corresponding `.en.md` (and vice versa)

### 6. PR template

A pull request template (`.github/pull_request_template.md`) includes a documentation checklist: tests pass, lint passes, documentation updated (if user-facing), CHANGELOG updated (if user-facing), bilingual docs provided.

## Consequences

### Positive

- Documentation debt is structurally prevented, not just retroactively fixed
- CI enforcement removes reliance on developer discipline for changelog and bilingual parity
- PR template makes documentation a visible, checkable step in every contribution
- Clear exemptions (`docs/dev/`, `docs/design/`) avoid unnecessary burden for internal notes

### Negative / Trade-offs

- Slight overhead per PR for documentation companion commits (~15 min per feature)
- CI checks may occasionally block legitimate PRs that intentionally defer documentation (use `[skip-docs]` in PR description to bypass, logged for follow-up)
- French-only CHANGELOG is inconsistent with the bilingual guides, but avoids maintaining a 1600-line translation

## Alternatives Considered

| Alternative | Rejected because |
|-------------|-----------------|
| Automated changelog generation from commit messages | Loses the human-curated grouping by theme that makes the changelog readable |
| English-only documentation | Breaks the existing FR-first convention and alienates the current contributor base |
| No CI enforcement (convention only) | The 6-month drift from 58% to 6% proves convention alone is insufficient |
| Mandatory bilingual for all docs including `docs/dev/` | Excessive burden for internal notes that are rarely consulted outside the core team |
