# ADR-005 — Language Adaptation of Agents for the Target Project

## Status

Accepted

## Context

The opencode-hub is written entirely in French: agents, skills, documentation.
However, projects on which agents are deployed may have different working languages (English, Spanish, etc.).

Currently, an agent deployed on an English-speaking project will produce its reports, summaries, and messages in French, which creates friction for non-French-speaking teams.

The question is: how to allow agents to adapt to the target project's language without duplicating all agent/skill files for each language?

## Decision

**Option A selected**: injection of a language instruction at the top of each deployed agent, conditional on the presence of a `Langue` (Language) field in `projects.md`.

The instruction is injected by the deployment script — source files (agents, skills) remain in French. Default behavior (field absent): no change, agents express themselves in French.

## Implementation

### 1. Optional field in `projects.md`

```markdown
## MY-APP
- Nom : My Application
- Stack : Vue 3 + Laravel
- Board Beads : MY-APP
- Tracker : jira
- Labels : feature, fix
- Langue : english        # optional — if absent: French by default
```

### 2. Reading the field via `common.sh`

New function `get_project_language <PROJECT_ID>`:
- Reads the `- Langue :` field in `projects.md` for the given project
- Returns the normalized lowercase value, or an empty string if absent

### 3. Injection in `prompt-builder.sh`

The `build_agent_content` function accepts a 3rd parameter `$3` = language (optional).
If non-empty, an instruction is inserted after the generated header comment:

```markdown
> **Working language: english.** Write all your responses, reports and comments
> in english, regardless of the language of the instructions below.
```

### 4. Passing the language in adapters

The 2 adapters (`opencode.adapter.sh`)
read the project language via `get_project_language "$PROJECT_ID"` and pass it
as the 3rd argument to `build_agent_content`.

If `PROJECT_ID` is empty (hub-level deployment without a target project), no instruction is injected.

## Rejected Options

### Option B — Skill translation by the adapter

The deployment (`oc deploy`) automatically translates skill files via an LLM API before injection. Deployed agents are in the project's language.

**Rejected**: API cost on every deployment, increased deployment time, translation maintenance hard to version and audit.

### Option C — Multi-language skills

Skills exist in multiple language versions:
`skills/developer/dev-standards-universal.fr.md`, `.en.md`, etc.
The adapter selects the version based on the project language.

**Rejected**: file multiplication (×N languages), heavy maintenance, risk of divergence between language versions.

## Consequences

- The `projects.md` structure gains an optional `Langue` field
- Default behavior is unchanged for existing projects (backward-compatible)
- Adapters and `prompt-builder.sh` handle an additional parameter
- The reliability of the instruction depends on the AI model used — acceptable for the intended use
