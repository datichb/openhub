> 🇫🇷 [Lire en français](external-agents.fr.md)

# Guide — External Agents per Project

This guide explains how to integrate existing agents from a target project with the opencode-hub, without overwriting them or forcing migration.

---

## Prerequisites

- Hub initialized (`oc install`)
- Project registered (`oc init PROJECT_ID`)
- `.md` agent files already present in `<project>/.opencode/agents/` (not hub-generated agents)

---

## How It Works

During `oc deploy PROJECT_ID`, the hub automatically detects agents in `.opencode/agents/` that it doesn't own. It then asks you how to integrate them:

```
── Existing agents detected in project

  ● Agent found: planner  (.opencode/agents/my-planner.md)
    → Similar to hub agent: planner

    What do you want to do?
      [s] Substitute our 'planner' agent with this one
      [c] Add as complement (both coexist)
      [i] Ignore (do not integrate)

    Your choice [s/c/i]:
```

Your choice is **persisted** in `projects.md` under the `External agents` field. Subsequent deploys no longer prompt for already-configured agents.

---

## The Two Integration Modes

| Mode | What happens | When to use |
|------|-------------|------------|
| **Substitute** | The project agent **replaces** the corresponding hub agent in `.opencode/agents/` | The project agent is better suited to the domain than the hub's |
| **Complement** | The project agent **is added** alongside hub agents | The project agent covers a need not met by the hub |

---

## Triggering

### Automatically at deploy

```bash
oc deploy MY-PROJECT
```

If non-hub agents are found in `.opencode/agents/` and not yet configured → interactive prompt.

> **In CI/CD** (`OC_NON_INTERACTIVE=1`): discovery is silently skipped. Deploy continues normally.

### On demand

```bash
oc agent discover MY-PROJECT
```

Runs only discovery without triggering a deploy. Useful to configure before the first deploy.

---

## Step-by-step Example

### Initial situation

Your project `MY-APP` already has these files in its repo:

```
my-app/
└── .opencode/
    └── agents/
        ├── planner.md          ← manual agent, domain-specific
        └── feature-reviewer.md ← custom agent with no hub equivalent
```

### Step 1 — First deploy

```bash
oc deploy MY-APP
```

The hub detects `planner.md` (similar to hub agent `planner`) and `feature-reviewer.md` (no equivalent):

```
  ● Agent found: planner  (.opencode/agents/planner.md)
    → Similar to hub agent: planner

    [s] Substitute / [c] Complement / [i] Ignore: s
    → Substitution: 'planner' will replace 'planner' for this project

  ● Agent found: feature-reviewer  (.opencode/agents/feature-reviewer.md)
    → No equivalent hub agent found

    [c] Complement / [i] Ignore: c
    → Complement: 'feature-reviewer' will be added alongside hub agents
```

### Step 2 — Automatic persistence

`projects.md` is updated:

```markdown
## MY-APP
- Nom : My Application
- Stack : TypeScript · Vue 3
- Agents : all
- External agents : .opencode/agents/planner.md:substitute:planner|.opencode/agents/feature-reviewer.md:complement
```

### Step 3 — Result in the project

After deploy, `.opencode/agents/` contains:

```
my-app/.opencode/agents/
├── planner.md          ← your agent (copied from your source)
├── feature-reviewer.md ← your agent (copied from your source)
├── orchestrator.md     ← hub
├── developer-frontend.md ← hub
├── reviewer.md         ← hub
└── ...                 ← other hub agents
```

> The hub's `planner` agent is **replaced** by yours. All other hub agents are deployed normally.

### Step 4 — Subsequent deploys

```bash
oc deploy MY-APP   # no more prompt, choices are remembered
```

---

## `External agents` Field Format

```markdown
- External agents : <entry1>|<entry2>|...
```

Each entry follows this format:

| Format | Meaning |
|--------|---------|
| `path:substitute:hub-id` | The project agent replaces hub agent `hub-id` |
| `path:complement` | The project agent is added alongside hub agents |

Paths can be:
- **Relative** to `project_path` (e.g. `.opencode/agents/planner.md`)
- **Absolute** (e.g. `/home/user/agents/planner.md`)

---

## Manual Editing

You can directly edit `projects.md` without going through the interactive prompt:

```markdown
## MY-APP
- External agents : .opencode/agents/planner.md:substitute:planner
```

To **remove** an integration, delete the corresponding entry or empty the field:

```markdown
## MY-APP
- External agents :
```

> An empty field is equivalent to the absence of the field.

---

## Similarity Resolution

The hub automatically recognizes common names thanks to `config/agent-aliases.json`:

| Name in project | Recognized hub agent |
|-----------------|---------------------|
| `planner`, `plan`, `planning`, `project-planner` | `planner` |
| `orchestrator`, `coordinator`, `router` | `orchestrator` |
| `frontend`, `front`, `ui-dev` | `developer-frontend` |
| `backend`, `back`, `server` | `developer-backend` |
| `qa`, `tester`, `test` | `qa-engineer` |
| `reviewer`, `review`, `code-reviewer` | `reviewer` |
| `docs`, `documentation`, `writer` | `documentarian` |
| `devops`, `ops`, `ci-cd` | `developer-devops` |
| `debug`, `debugger`, `bug-hunter` | `debugger` |

To see the full list: `cat config/agent-aliases.json`

---

## Behavior of the Substituted Agent

A substitution agent is deployed **as-is** from its source file, going through the normal build pipeline. This means:

- If the frontmatter declares `skills:` → they are injected (Bucket A)
- If the frontmatter declares `native_skills:` → they are deployed (Bucket B)
- If the frontmatter has no skills → the agent is deployed without hub skill injection
- The frontmatter `mode:` is respected (or the project override if configured)

> **Stack skills** (ADR-008) do not apply to substitution agents — they only benefit from them if declared in their own frontmatter.

---

## Troubleshooting

| Symptom | Likely cause | Solution |
|---------|-------------|---------|
| Agent not detected at deploy | It has the `<!-- generated by opencode-hub` marker | It's a hub agent — it will be regenerated normally |
| Prompt doesn't appear | `OC_NON_INTERACTIVE=1` or no TTY | Run `oc agent discover MY-PROJECT` manually |
| "Substitute file not found" at deploy | Relative path is incorrect | Check that the path is relative to `project_path` |
| "Complement agent 'X' is a duplicate" | A hub agent has the same ID | Change your agent's ID or use substitution |
| Choice is not remembered | `projects.md` is read-only or Perl error | Check permissions on `projects.md` |

---

## See Also

- [ADR-011 — External Agents per Project](../architecture/adr/011-external-agents-per-project.en.md)
- [CLI Reference — `oc agent discover`](../reference/cli.en.md#oc-agent)
- [Authoring Guide](./authoring.en.md)
