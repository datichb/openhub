# Living Documentation Wiki

This directory contains the **living documentation wiki** for the openhub project itself. Pages are bilingual (`x.fr.md` / `x.en.md`): each page starts with its frontmatter, followed by the language link.

The living wiki is an incrementally enriched knowledge base that agents maintain
as they work on the project. Each page follows a structured format with confidence
tags (`CONFIRMED`, `INFERRED`, `SPECULATIVE`), source references, and timestamps.

## Pages

| Page | Topic |
|------|-------|
| [Architecture](architecture.en.md) ([fr](architecture.fr.md)) | v5 system architecture — Go CLI, workflows, session bundle, adapters, `ohd` daemon, gateways, runtimes, MCP servers, TUI |
| [Stack](stack.en.md) ([fr](stack.fr.md)) | Technology stack and dependencies |
| [Conventions](conventions.en.md) ([fr](conventions.fr.md)) | Code style, commit format, branch naming, review process, file naming |
| [Config Cascade](technical/config-cascade.en.md) ([fr](technical/config-cascade.fr.md)) | Configuration resolution across Team / Hub / Project levels, workflows, runtime, session limits |

## How it works

- Pages are created by the `onboarder` agent during `oh run onboarding` (it only writes to `docs/wiki/`)
- Agents propose enrichments as they discover patterns (skill `shared/living-docs-enrichment`); the `documentarian` writes them when it is a member of the session workflow (`feature`, `ticket`), otherwise they are only proposed
- oh itself never writes to the wiki; team wiki proposals go through the `team_wiki_write` MCP tool and are reviewed before merging
- See [ADR-024](../architecture/adr/024-team-state-repository.en.md) and [Living Wiki Architecture](../architecture/living-wiki.en.md) for design details
