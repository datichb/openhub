# Living Documentation Wiki

This directory contains the **living documentation wiki** for the openhub project itself.

The living wiki is an incrementally enriched knowledge base that agents maintain
as they work on the project. Each page follows a structured format with confidence
tags (`CONFIRMED`, `INFERRED`, `SPECULATIVE`), source references, and timestamps.

## Pages

| Page | Topic |
|------|-------|
| [Config Cascade](technical/config-cascade.en.md) | Configuration resolution across Team / Hub / Project levels |
| [Conventions](conventions.en.md) | Code style, commit format, branch naming, review process |
| [Architecture](architecture.en.md) | System architecture overview — Go CLI, MCP servers, TUI, agents |
| [Stack](stack.en.md) | Technology stack and dependencies |

## How it works

- Pages are created by the `onboarder` agent during `oh run onboarding`
- Pages are enriched by `developer`, `reviewer`, and `auditor` agents as they discover patterns
- Proposals are submitted via the `team_wiki_write` MCP tool and reviewed before merging
- See [ADR-024](../architecture/adr/024-team-state-repository.en.md) and [Living Wiki Architecture](../architecture/living-wiki.en.md) for design details
