# ADR-035 — Dynamic Board Columns and Tracker Discovery

## Status

accepted

## Date

2026-09-15

## Context

The team board had 6 hardcoded columns (TODO, IN PROGRESS, REVIEW, VALIDATION, DONE,
BLOCKED). This rigidity caused several problems:

- **Non-adaptable workflows** — teams with specific stages (TESTING instead of VALIDATION,
  PREPROD, STAGING) could not customize the board. Every team had to conform to the 6
  imposed statuses, even if their actual workflow was different.

- **Manual tracker configuration** — label→status mappings and pool label configuration
  (unassigned tickets) required manual TOML editing. No guidance existed for this critical
  setup step.

- **No tracker onboarding** — new team members had no guided wizard for configuring the
  tracker connection (GitLab/Jira) and its mappings.

- **Partial ticket coverage** — the sync tracker only fetched unassigned tickets, missing
  tickets assigned to non-members or tickets matching the team's workflow labels.

## Decision

We decided to introduce a dynamic column system and a 5-step tracker discovery wizard.

### 1. `BoardConfig` with Customizable Columns

Each column carries a semantic role (`initial`, `active`, `terminal`, `blocked`) that
allows the board to retain its behavior (starting column, terminal columns, blocked
column) while accepting arbitrary columns.

### 2. 5-Step Discovery Wizard (TUI + CLI)

1. **Tracker connection** — connect to the configured tracker (GitLab/Jira)
2. **Label/status discovery** — query the tracker API to list available labels and statuses
3. **Column suggestion** — propose board columns based on the workflow detected in the
   tracker
4. **Label→column mapping** — propose mappings via a bilingual heuristic engine (FR/EN)
   that matches labels to columns
5. **Pool configuration** — configure pool labels for unassigned ticket import

### 3. Column ID = Claim Status

The column identifier becomes the claim status directly. The bridge layer between columns
and statuses is no longer needed for custom columns.

### 4. Backward Compatibility

The 6 existing statuses remain as defaults. `IsValidBoardStatus` extends validation to
accept any custom column ID in addition to the constants.

### 5. Relaxed Transitions

Custom statuses are not constrained by `ValidTransitions` — any transition is allowed
(any→any) to avoid imposing a rigid workflow on teams that define their own columns.

### 6. Full Tracker Sync on 'r'

The `r` key on the board triggers a full tracker API sync before git pull, fetching all
tickets matching workflow labels (not just unassigned tickets).

## Consequences

### Positive

- Teams can fully customize their workflow without code changes
- The discovery wizard reduces setup from manual TOML editing to a guided 2-minute flow
- All tickets matching workflow labels appear on the board (not just unassigned)
- The `InlineWizardView` component (ADR-034) is reused for the TUI wizard

### Negative / Trade-offs

- `ValidTransitions` is bypassed for custom statuses — less guardrails on manual status changes
- Increased API calls on the `r` key (5-15s vs 1-2s for git pull only)
- Claim status values are no longer limited to 6 constants — any column ID is valid
- The serve API still uses `DefaultBoardConfig` — custom columns are not exposed via HTTP

## Alternatives Considered

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Visibility/order only — allow hiding/reordering the 6 default columns but not creating new ones | Too limited — does not support workflows with custom stages (PREPROD, STAGING, QA) |
| Fully dynamic statuses in `claims.go` — replace all hardcoded status constants with a dynamic registry | Too invasive, high regression risk. The chosen approach keeps the constants as fallback and extends with `IsValidBoardStatus` |
| GitLab Boards API for discovery — use `GET /api/v4/projects/:id/boards` to infer columns from GitLab board lists | Not all teams use GitLab boards, and the heuristic engine covers more patterns (labels, Jira statuses, bilingual matching) |
