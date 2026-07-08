# Team Conventions Guide

## Principle

Team conventions are shared rules that apply to code, branches, and commits. They are automatically verified by AI agents and by the `oh conventions check` command.

## Where to document conventions

### Project-specific conventions

Location: `docs/wiki/technical/conventions.md` in the project repo.

This file is read directly by agents during their session (filesystem access).

### Cross-project conventions (team)

Location: `conventions` page in the team-state wiki.

Accessible to agents via `team_wiki_read("conventions")`.

## Expected format

Conventions must include machine-readable patterns for automatic verification.

### Branch pattern

```markdown
## Branches

branch_pattern = `^(feat|fix|chore|refactor|docs)/[A-Z]+-\d+-.+`

Convention: `<type>/<TICKET-ID>-<description-slug>`

Examples:
- `feat/SRU-142-user-authentication`
- `fix/SRU-99-token-refresh`
- `chore/SRU-200-cleanup-deps`
```

### Commit format

```markdown
## Commits

We use Conventional Commits.

commit_pattern = `^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\(.+\))?!?:\s.+`

Examples:
- `feat(auth): implement JWT middleware`
- `fix(api): handle null response body`
- `docs: update API reference`
```

If the file simply mentions "Conventional Commits" without an explicit pattern, the checker automatically applies the standard pattern.

## Verification

### Standalone command

```bash
oh conventions check
```

Displays:
- Current branch status vs the pattern
- Recent commits status vs the format
- Claim status (if team enabled)

### Agent verification

The orchestrator-dev automatically checks conventions:
- **Before creating a branch**: applies the naming pattern
- **Before each commit**: respects the documented format
- **Non-blocking warnings**: the agent informs but does not block

## Enforcement

Enforcement level is **medium**:
- Warnings in the terminal and in the agent session
- No hard blocking (dev can ignore a warning)
- No imposed pre-commit hook (optional)
