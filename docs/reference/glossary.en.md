> [Lire en francais](glossary.fr.md)

# Glossary

Quick reference for terms used throughout the OpenHub documentation.

---

### Adapter

The oh component that translates a [SessionSpec](#sessionspec) into the format of a tool (today opencode V2: configuration, agents, permissions, `opencode serve` server) and checks the [closed world](#closed-world) at each start (`Attest`). See [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.en.md).

### Agent

A specialized AI role definition (Markdown file with YAML frontmatter) that determines the persona, permissions, skills, and delegation rules for an AI session. Agents are organized into 7 families: planning, developer, auditor, quality, design, documentation, utility. See [Agent architecture](../architecture/agents.en.md).

### Brick / Brick catalogue

A brick is an agent or a skill that workflows may use. The brick catalogue gathers those of the hub and those of the team (`catalog/agents`, `catalog/skills` of the team-state). TUI: `bricks` command. See [Team workflows](../guides/team-workflows.en.md#team-brick-catalogue).

### Beads / MCP gateway

Services of the [ohd daemon](#ohd-daemon). The **Beads gateway** runs the `bd` commands of a session, local or in a container, on the machine (fake `bd` first on the session `PATH` or in the container, `beads.allow` allow-list); remotely, a snapshot leaves with the session and the journal is replayed on return (`oh session resolve`). The **MCP gateway** serves the oh MCP servers of the bundle over HTTP, tokens staying on the machine. See [ADR-046](../architecture/adr/046-beads-gateways.en.md).

### Beads

The integrated lightweight ticket tracking system used by OpenHub. Beads tickets (`bd-1`, `bd-2`, ...) are created by the planner agent and consumed by the orchestrator-dev during implementation workflows. Requires the `bd` CLI tool. See [Beads model reference](beads-model.en.md).

### Bucket A (Inline Skills)

Skills listed in an agent's `skills:` frontmatter array. These are injected directly into the agent's system prompt when the session bundle is built -- always present in the agent's context. Contrast with [Bucket B](#bucket-b-native-skills).

### Bucket B (Native Skills)

Skills listed in an agent's `native_skills:` frontmatter array. These are delivered as separate files in the session bundle and loaded on-demand via the `skill` tool during a session. They extend an agent's capabilities without consuming baseline context. Contrast with [Bucket A](#bucket-a-inline-skills).

### Checkpoint (CP)

A gate of a [workflow](#workflow) (`checkpoints:`, e.g. `cp-0`, `cp-2`) where the user approves, fixes or gives other instructions before going on. Its behavior depends on the [workflow mode](#workflow-mode) (`pause`, `auto`, `skip`, `conditional`); a `mandatory` checkpoint cannot be relaxed. It is held at **3 levels**: the generated prompt (skill `workflow/workflow-map`), the MCP tool `workflow_checkpoint` (`ask` permission approved by oh) and the oh plugin; the state machine lives in oh. A pause becomes a ⏸ [decision](#decision). See [`oh/v1` schema](workflow-schema.en.md#checkpoints) and [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.en.md).

### Closed world

v5 rule: a session only sees the agents and skills of its [bundle](#session-bundle); opencode native agents and the personal configuration are hidden. Checked at each start (`Attest`); on failure, the session does not start. See [ADR-041](../architecture/adr/041-closed-world-isolation.en.md).

### Conductor

Generic entry agent of a workflow when `entry.agent` is absent: without write or shell, it follows the generated workflow map and delegates to the other agents according to `after` and the checkpoints (workflows `cadrage`, `sweep`). See [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.en.md).

### Credential proxy

Service of the [ohd daemon](#ohd-daemon) through which the LLM calls of sessions go: each [server group](#server-group) gets an `ohs_…` token, the real keys stay on the machine; the proxy applies the path and model allow-lists and signs Bedrock requests (SigV4). See [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

### Claim

In team mode, a claim is a ticket assignment: a team member "claims" a ticket to signal they are working on it. Claims prevent duplicate work across team members. Managed via `oh team claim` / `oh team release`. See [Team setup](../guides/team-setup.en.md).

### Decision

What a session waits for from the user: ⏸ checkpoint, ? question, ! permission, $ budget, ✗ error or circuit breaker. The first answer wins, wherever it comes from (TUI, opencode, browser, `oh session approve|answer`). Shown in the "To handle" section of the Sessions view and in `oh session inbox`. See [v5 sessions](../guides/sessions-v5.en.md#checkpoints).

### Deploy (removed in v5)

Formerly, the process of copying agents, skills, permissions, and configuration from the central hub (`~/.oh/`) into a target project's `.opencode/` directory (`oh deploy` / `oh sync`, removed in v5). Replaced by the session bundle: each session starts from a bundle built at launch outside the project (`~/.oh/bundles/<hash>/`) from its workflow; inspect it with `oh bundle show <workflow>`. Leftovers of former deployments are removed by `oh migrate deploy-cleanup`. See [Getting started](../guides/getting-started.en.md#session-bundle).

### Draft / Publication

A draft is the unpublished version of a team or project workflow, owned by one member (`workflows/drafts/<member>/`), validated when saved and launchable locally (`oh run <id> --draft`). Publication (`oh workflow publish`, any member) revalidates it, creates a new version and updates `workflows.lock`; offline, it is queued. See [Team workflows](../guides/team-workflows.en.md#drafts-publication-history).

### Execution environment (runtime)

Where the server of a session runs: `local` (the machine), `container` (Colima, Podman or Docker, one image per project, bundle mounted read-only) or `remote` (GitLab CI, `oh-runner` project). Allowed by the workflow `runtime.allowed`; choice order: option > project > Settings > workflow. See [ADR-045](../architecture/adr/045-execution-environments.en.md), [Container](../guides/container.en.md), [Remote runners](../guides/remote-runners.en.md).

### Hub

The central installation directory (`~/.oh/`) containing the configuration file (`hub.toml`), project database (`oh.db`), embedded agents and skills (`hub/`), and encryption keys. The hub is created by `oh init` and serves as the single source of truth for all projects.

### Hub Content

The collection of agent definitions and skill protocols embedded in the `oh` binary at compile time via `go:embed`. Extracted to `~/.oh/hub/` on first run. Updated when you upgrade `oh`.

### Living Wiki

A structured documentation system generated by the [onboarder](#onboarder) agent when discovering a project. It consists of interconnected Markdown pages with confidence tags, god nodes (high-connectivity concepts), and incremental enrichment. See [Living wiki architecture](../architecture/living-wiki.en.md).

### MCP Server (Model Context Protocol)

A service that exposes external tool capabilities to AI agents via the JSON-RPC protocol over stdio. OpenHub includes 7 built-in MCP servers: Figma, GitHub, GitLab, Google Slides, Jira, Linear, and Team. See [Services reference](services.en.md).

### ohd daemon

The oh background process (`oh daemon status|stop`): [credential proxy](#credential-proxy), session supervision, feeds, [decisions](#decision), system notifications and [gateways](#beads--mcp-gateway). On Windows, it runs inside the oh process. See [ADR-047](../architecture/adr/047-session-interaction-daemon.en.md).

### Mode (Agent)

An agent can operate in two modes:
- **Primary**: launched directly by the user via `oh run <workflow>` or the TUI (entry agent of the workflow). Has its own session.
- **Subagent**: invoked by another agent via the `task` tool. Runs within the parent agent's session.

### Hub Mode

Default navigation mode in the TUI, displaying all projects and teams. Auto-selected when multiple projects or teams are configured. Allows selecting a project or team to switch to a focused mode. See [TUI usage](../guides/tui-usage.en.md).

### Project Mode

TUI navigation mode focused on an active project. The omnibar shows only project commands (sessions, board, project config) and global commands. Auto-selected when a single project is configured, or activated by selecting the project from the Hub Home (`Esc` goes back to the Hub, `Ctrl+T` enters the team mode). See [TUI usage](../guides/tui-usage.en.md).

### Team Mode

TUI navigation mode focused on an active team. The omnibar shows only team commands (team board, status, policies) and global commands. Auto-selected when a single team is configured, or activated manually via `Ctrl+T` / selection from the Hub Home. See [TUI usage](../guides/tui-usage.en.md).

### Onboarder

A primary agent in the planning family that explores an existing codebase, detects the tech stack, identifies risks, and produces a [living wiki](#living-wiki). Invoked with `oh run onboarding`.

### OpenCode

The underlying AI coding agent runtime (separate binary) that OpenHub orchestrates. OpenCode manages the actual LLM conversation, tool execution, and session persistence. oh requires opencode V2 (>= 2.0.0), installed with its own tool; V1 is no longer supported (`oh doctor` checks it). See the [v5 migration guide](../guides/migration-v5.en.md).

### Orchestrator

The main coordinator agent. Receives user requests, delegates to specialized agents (planner, designer, developer, auditor, reviewer, debugger, documentarian), and manages the workflow through [checkpoints](#checkpoint-cp). Never performs direct analysis or coding.

### Orchestrator-dev

A specialized coordinator agent for implementation workflows. Manages the Beads ticket lifecycle: picks tickets, routes to the appropriate [developer](../architecture/agents.en.md) domain, triggers review, and handles merge. See [Workflows](../guides/workflows.en.md).

### Permission Profile

A YAML file (in `permissions/`) that defines what tools an agent can use (`bash`, `read`, `edit`, `write`, `glob`, `grep`, `webfetch`, `websearch`, `skill`, `task`). Three built-in profiles: `coordinator` (read-only), `developer-rw` (full access), `readonly-code` (read-only code).

### Provider

The LLM backend that serves AI model responses. Supported providers: Amazon Bedrock, Anthropic (direct API), OpenRouter, GitHub Copilot. Configured via `oh init` or `oh provider setup`. See [Providers guide](../guides/providers.en.md).

### Server group

The sessions served by the same `opencode serve` server: same [session bundle](#session-bundle) version, same project, same [execution environment](#execution-environment-runtime). Each group gets its own [credential proxy](#credential-proxy) token. See [v5 sessions](../guides/sessions-v5.en.md#how-it-works).

### Session bundle

The immutable, hashed directory (`~/.oh/bundles/<hash>/`) built at launch from the [workflow](#workflow), outside the project: agents, skills, permissions, MCP, plugins, model, workflow map. A resume starts from the same bundle. Inspect: `oh bundle show <workflow>`. Replaces the [deploy](#deploy-removed-in-v5). See [ADR-043](../architecture/adr/043-session-bundle-deploy-removal.en.md).

### Session restrictions (I6)

Optional session limits, off by default: maximum active sessions, budget per session and per day (USD), memory cap, model list. Cascade hub (`[limits]`) → team (recommended / enforced) → project → workflow (`limits:`). Managed with `oh budget show|set|unset|raise` and Settings › Restrictions. See [v5 sessions](../guides/sessions-v5.en.md#restrictions) and [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

### SessionSpec

The tool-independent description of a session (agents, skills, permissions, MCP, model, workflow) whose recorded form is the [session bundle](#session-bundle); the [adapter](#adapter) translates it for opencode. See [ADR-038](../architecture/adr/038-sessionspec-tool-adapters.en.md).

### Solo space

A local team-state, without remote repository (`~/.oh/teams/<id>/`), that lets a project without a team have its own workflows. Created by `oh team init --solo`, shared later with `oh team promote --remote <url>`. See [Team workflows](../guides/team-workflows.en.md#project-without-a-team-solo-space).

### Skill

A Markdown protocol document that provides domain-specific knowledge, workflows, or behavioral instructions to an agent. Skills are categorized as [Bucket A](#bucket-a-inline-skills) (always loaded) or [Bucket B](#bucket-b-native-skills) (on-demand). See [Skills architecture](../architecture/skills.en.md).

### Stack Skills

Framework-specific skill protocols (e.g., `dev-standards-react`, `dev-standards-golang`) that are dynamically added to the session bundle based on the detected tech stack of the target project (languages Go, TypeScript, Python, Rust, Java, Ruby; frameworks Next.js, Nuxt, React, Vue, Express, Django, FastAPI, Rails; Vitest, Jest, Docker, CI). Located in `skills/developer/stacks/`.

### Target Project

A codebase registered with OpenHub via `oh project add`. Sessions are launched on it with `oh run <workflow>`; nothing is deployed into the project (session bundles live in `~/.oh/bundles/`). Multiple projects can be registered simultaneously.

### Team-state Repository

A Git repository shared by team members to synchronize collaboration state: claims, member identities, wiki, events, policies, patterns, and credentials. Created with `oh team init`. See [Team setup](../guides/team-setup.en.md).

### TUI (Terminal User Interface)

The interactive terminal dashboard launched by running `oh` without arguments. Provides visual navigation for projects, sessions, team board, configuration, and more. Built with tview (huh is still used for a few inline prompts, outside the TUI). See [TUI usage](../guides/tui-usage.en.md).

### Workflow Mode

Set at launch (`--mode`, launch form), it determines the behavior of each [checkpoint](#checkpoint-cp), declared by the workflow for each mode:
- **Manual** (`manuel`): most checkpoints pause for approval
- **Semi-auto** (default of the shipped workflows): usual checkpoints pass on their own; key decision points (e.g. `cp-0`, `cp-2`) pause
- **Auto**: everything passes on its own except the checkpoints that stay on `pause` (e.g. `cp-0` plan approval, `cp-2` commit)

A workflow may restrict the modes (`modes.allowed`). See [`oh/v1` schema](workflow-schema.en.md#modes).

### Workflow

The declarative description of a use case (YAML `apiVersion: oh/v1`): entry agent, agents, checkpoints, inputs, resources, environments and limits. 12 workflows shipped by the hub (`feature`, `ticket`, `quick`, `cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `sweep`, `brief-enrich`, `libre`), extensible by the team and the project (`extends`, security may only be hardened). Launched with `oh run <workflow>`. See [Shipped workflows](workflows.en.md) and [`oh/v1` schema](workflow-schema.en.md).

### Worktree

A Git worktree used to isolate parallel AI sessions. Each worktree gets its own branch and working directory, allowing multiple agents to work simultaneously without conflicts. Managed via `oh worktree` or `oh run <workflow> --location new`. See [Worktree guide](../worktree.md).
