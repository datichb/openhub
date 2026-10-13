> [Lire en français](README.fr.md)

# Evaluations (`oh eval`)

Each `evals/<workflow>/<case>/` directory is an evaluation case, run by [`oh eval`](../docs/reference/cli-tools.en.md#oh-eval) with a real model:

```
evals/<workflow>/<case>/
  case.yaml       the session and its assertions
  fixture/        the starting project (copied into a throwaway git clone, committed on main)
  hidden_tests/   tests copied over the clone after the session (never seen by the agents)
```

## `case.yaml` format

```yaml
id: S2                      # unique id (S1, S2…)
title: "Ticket: …"
workflow: ticket
active: true                # false: case skipped; inactive_reason says why
mode: semi-auto             # default: the workflow one
inputs: { review_mode: standard }   # workflow inputs
text: "…"                   # first free text input
tickets: [ev-1]             # tickets given to the workflow
beads:                      # Beads database created in the clone
  prefix: ev
  tickets:
    - { id: ev-1, title: "…", type: bug, labels: [ai-delegated], description: "…", acceptance: "…" }
setup:                      # shell commands run in the clone, each committed
  - git checkout -q -b feature/x
timeout: 20m                # default: 20m
budget_usd: 1.0             # default: the pass one (--budget)
answers:
  checkpoints: { cp-2: approve, "*": approve }   # approve | fix | reject | other, optional ": message"
  permission: once          # once | reject
  question: first           # first option, or a free answer
  wiki: accept              # wiki proposals on the card of a commit checkpoint: accept | refuse | pending (default: left queued)
assertions:
  - type: ran
    command: "git commit*"
    absent: true
    before: { checkpoint_passed: cp-2 }
hidden_tests: "go test ./..."   # default: by stack (go.mod, package.json, pyproject.toml, else make test)
reports: off                # the daemon keeps no report: workflow_report answers "not recorded"
```

The `before:` anchors name the first matching event of the trace: `checkpoint` (reached), `checkpoint_passed`, `subagent`, `tool` (with `arg`, a pattern of the path or command), `text` (regular expression), `command` (shell pattern), restricted to an agent with `agent`. Shell patterns accept `*` and also check each part of a chained command (`cd x && go test ./...`).

The assertions are described in the [`oh eval` reference](../docs/reference/cli-tools.en.md#oh-eval). After an assertion is tuned, `oh eval --rescore <dir>` checks the traces already obtained again without paying the passes again.

## Shipped cases

| Case | Workflow | What is checked |
|------|----------|-----------------|
| S1 | review | The verification runs first; « corriger » verdict on a seeded bug |
| S2 | ticket | Developer and reviewer never call `question`; no commit before `cp-2` |
| S3 | feature | The pathfinder returns its block without asking; a blocking question is relayed |
| S4 | audit | Each phase skill is loaded before its phase |
| S5 | cadrage | The planner follows the subagent variant |
| S6 | debug | Phase skill loaded before each phase |
| S7 | quick | A skill out of scope is reported missing, without loop |
| S8 | ticket | Reports not recorded (`reports: off`): developer and reviewer include their whole report in the block |
| S9 | sweep | The developer and reviewer returns are addressed to `conductor` |
| S10 | quick | A data decision is asked in « Questions bloquantes », not with `question` |
| S11 | ticket | Project with a wiki: a `wiki_propose` proposal is accepted on the card of `cp-2`, applied by oh and committed with the code |

The reference measure (Haiku, three repetitions) is recorded in `cli/internal/evalrun/testdata/evals/baseline.json`; `make test-eval` compares the current run with it.

## Running faster, within a budget

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5
```

- `OH_EVAL_PARALLEL` (`--parallel`): number of passes run at the same time. Each pass stays isolated (its `OH_HOME`, `HOME`, `TMPDIR`, clone, daemon and proxy) and the report is the same as in a sequential run. Measured on S7 and S10 ×2 with Haiku: 1 min 14 s four at a time against about 3 min 6 s sequentially; each pass takes a little longer (57 s on average against 44 to 49 s).
- `OH_EVAL_BUDGET_TOTAL` (`--budget-total`): cap of the whole evaluation; once reached, no new pass starts. The cap per pass (`OH_EVAL_BUDGET_USD`) still applies: $1 by default, $2.50 with a Sonnet model.
- A pass stopped by a Bedrock rate limit (throttling) runs again up to three times after a growing wait; the « Relances » section of the report says so. When retries pile up, lower `OH_EVAL_PARALLEL`.
