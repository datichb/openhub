# ADR-002 — Splitting the Developer into 9 Specialized Agents

## Status

~~Accepted~~ **Superseded by [ADR-013](./013-developer-agent-consolidation.en.md)**

The 9 specialized agents have been consolidated back into a single generic `developer` agent.
Specialization is now handled at invocation time via the prompt (domain + native_skills list),
leveraging the Bucket B architecture introduced in ADR-010.

---

## Context

The hub started with a single all-purpose `developer.md` agent. As skills were added (`dev-standards-backend`, `dev-standards-frontend`, etc.), this agent accumulated all skills from all domains, which caused several problems:

- The context injected into the AI tool became very long (all standards at once)
- The agent couldn't have a focused identity: it was simultaneously frontend, backend, data, devops, mobile, and API
- The orchestrator's routing matrix couldn't delegate with precision

## Decision

The `developer.md` agent is removed and replaced by 9 specialized agents (7 in the original design, later extended):

| Agent | Domain |
|-------|--------|
| `developer-frontend` | UI, components, Vue.js, CSS, accessibility |
| `developer-backend` | Services, repositories, migrations, business logic |
| `developer-fullstack` | Features spanning both layers |
| `developer-data` | Pipelines, ETL, ML, dbt, Airflow |
| `developer-devops` | Docker, CI/CD, shell scripts, infra |
| `developer-mobile` | React Native, Flutter, iOS, Android |
| `developer-api` | REST, GraphQL, webhooks, third-party integrations |
| `developer-platform` | Terraform, K8s, Helm, GitOps, infrastructure as code |
| `developer-security` | Application hardening post-security audit |

Each specialized agent only injects skills relevant to its domain.
All share `dev-standards-universal`, `dev-standards-git`, `beads-plan`, and `beads-dev`.

## Consequences

### Positive

- Injected context is reduced and relevant for each domain
- The orchestrator can route precisely via the routing matrix
- Each agent has a clear identity and rules adapted to its domain
- Makes adding a new domain easy without modifying existing agents

### Negative / trade-offs

- Multiple agent files to maintain instead of one
- `developer-fullstack` is inherently ambiguous — it covers the "I don't know" case
- The boundary between `developer-api` and `developer-backend` can be blurry

## Rejected Alternatives

**Single agent with internal routing**: one agent that detects the domain and adapts its behavior. Rejected because this reintroduces complexity into the agent and doesn't reduce the injected context.

**Framework-based agents** (developer-vuejs, developer-laravel, etc.): too granular, explodes the number of agents, doesn't match how tickets are typically worded.
