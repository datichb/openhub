# Skill Marketplace - Guide

> 🇫🇷 [Lire en français](skill-marketplace.fr.md)

## Overview

The skill marketplace lets the community extend `oh` with custom agent skills — without modifying the binary. Community skills are downloaded from the [oh-skills-index](https://github.com/datichb/oh-skills-index) or directly from a Git URL, then deployed automatically alongside built-in skills on `oh deploy`.

### Why community skills?

Built-in skills cover the core workflows (planner, pathfinder, onboarder, etc.). Community skills add:
- Language or framework-specific idioms (`golang-idioms`, `react-patterns`)
- Domain-specific protocols (`fintech-compliance`, `accessibility-audit`)
- Integration adapters beyond the built-in set
- Team-internal skills distributed via private Git repos

---

## Installing a Skill

### From the index (recommended)

```bash
oh skill add golang-idioms
```

This command:
1. Looks up `golang-idioms` in the skill index
2. Downloads the skill package to `~/.oh/skills/golang-idioms/`
3. Validates the manifest checksum
4. Marks the skill as active

The skill is deployed to your projects on the next `oh deploy`.

### From a Git URL

```bash
oh skill add https://github.com/myorg/my-oh-skill
```

Supports any public or private (SSH) Git repository. The repository must contain a valid `manifest.json` at its root.

### Pinning a version

```bash
oh skill add golang-idioms@1.2.0
oh skill add golang-idioms@latest  # default
```

---

## Listing Installed Skills

```bash
oh skill list
```

Output example:

```
SKILL                  VERSION  SOURCE   STATUS
golang-idioms          1.2.0    index    active
react-patterns         0.8.1    git      active
fintech-compliance     2.0.0    index    inactive
```

---

## Searching the Index

```bash
oh skill search go
oh skill search --tags backend,testing
oh skill search --author myorg
```

Returns a paginated list of matching skills with name, description, version, and author.

---

## Removing a Skill

```bash
oh skill remove golang-idioms
```

The skill is unlinked from deployments but its files remain in `~/.oh/skills/golang-idioms/` until purged:

```bash
oh skill remove golang-idioms --purge
```

---

## How It Works

1. **Download**: `oh skill add` fetches the skill package from the index or Git and stores it in `~/.oh/skills/<name>/`
2. **Validation**: the manifest checksum is verified before activation
3. **Deployment**: `oh deploy` reads all active skills and includes them in the generated `opencode.json` alongside built-in skills
4. **Agent access**: skills appear in the agent's skill list at session start, indistinguishable from built-in skills

Skills are stateless SKILL.md files — they do not modify the binary and can be safely removed at any time.

---

## Creating a Community Skill

### Package format

A skill package is a directory (or Git repo root) with at minimum:

```
my-skill/
├── manifest.json    # Required: package metadata
└── SKILL.md         # Required: skill instructions (loaded by agents)
```

Additional files (examples, tests, sub-skills) may be included; only `SKILL.md` is loaded by agents.

### SKILL.md structure

Follow the same conventions as built-in skills. The file must contain:
- A clear `# Title` describing the skill's purpose
- `## When to use` — conditions under which the agent should activate this skill
- `## Instructions` — step-by-step guidance
- Optional `## Examples` with concrete input/output pairs

See [authoring-skills.md](./authoring-skills.md) for the full authoring guide.

### Versioning

Use **semantic versioning** (`major.minor.patch`):
- `patch` — bug fixes and clarifications
- `minor` — new instructions or examples (backward-compatible)
- `major` — breaking changes to the skill's interface or behavior

---

## manifest.json Format

```json
{
  "name": "golang-idioms",
  "description": "Go idiomatic patterns and anti-patterns for the code-writer agent",
  "version": "1.2.0",
  "author": "myorg",
  "skill_file": "SKILL.md",
  "tags": ["go", "golang", "backend", "idioms"],
  "min_oh_version": "0.8.0",
  "homepage": "https://github.com/myorg/golang-idioms-skill",
  "license": "MIT"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique skill identifier (kebab-case) |
| `description` | Yes | One-line description shown in search results |
| `version` | Yes | Semantic version string |
| `author` | Yes | GitHub username or organization |
| `skill_file` | Yes | Path to the SKILL.md file (relative to package root) |
| `tags` | No | Array of searchable tags |
| `min_oh_version` | No | Minimum `oh` version required |
| `homepage` | No | URL to documentation or source |
| `license` | No | SPDX license identifier |

---

## Publishing to the Index

To make your skill available via `oh skill search` and `oh skill add <name>`:

1. Host your skill in a public GitHub repository
2. Ensure `manifest.json` and `SKILL.md` are at the repository root
3. Create a git tag matching the version in `manifest.json` (e.g. `v1.2.0`)
4. Submit a PR to [https://github.com/datichb/oh-skills-index](https://github.com/datichb/oh-skills-index) adding your skill entry to `index.json`

The PR template includes a checklist: valid manifest, SKILL.md present, version tagged, no malicious instructions.

---

## Authoring Best Practices

See the full authoring guide: [authoring-skills.md](./authoring-skills.md)

Key principles:
- **Be specific**: skills that activate in broad conditions dilute agent focus
- **Provide examples**: concrete input/output pairs reduce agent hallucination
- **Single responsibility**: one skill = one domain; compose via multiple skills rather than a monolithic file
- **Version carefully**: breaking changes require a major version bump to avoid breaking existing deployments
- **Test before publishing**: use `oh skill test <skill-name>` to run the skill against sample sessions

---

## Resources

- [oh-skills-index repository](https://github.com/datichb/oh-skills-index)
- [authoring-skills.md](./authoring-skills.md) — full skill authoring guide
- [MCP Protocol](https://modelcontextprotocol.io/)
