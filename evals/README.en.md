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
  sequence:                 # answers to the first asks of a checkpoint, in order; then checkpoints
    cp-spec: ["reject: add a rule…"]
assertions:
  - type: ran
    command: "git commit*"
    absent: true
    before: { checkpoint_passed: cp-2 }
hidden_tests: "go test ./..."   # default: by stack (go.mod, package.json, pyproject.toml, else make test)
reports: off                # the daemon keeps no report: workflow_report answers "not recorded"
previous:                   # finished sessions of the project, recorded before the launch (session.output sources)
  - { workflow: spec, outputs: { tickets: [ev-1], spec: docs/wiki/specs/x.md } }
fanout: { parallel: 2 }     # fan-out (R6): the pass enables the fan-out; fanout assertion (fan-out events, checkpoints of the children)
```

The `before:` anchors name the first matching event of the trace: `checkpoint` (reached), `checkpoint_passed`, `subagent`, `tool` (with `arg`, a pattern of the path or command), `text` (regular expression), `command` (shell pattern), restricted to an agent with `agent`. Shell patterns accept `*` and also check each part of a chained command (`cd x && go test ./...`).

The assertions are described in the [`oh eval` reference](../docs/reference/cli-tools.en.md#oh-eval). After an assertion is tuned, `oh eval --rescore <dir>` checks the traces already obtained again without paying the passes again.

## Shipped cases

| Case | Workflow | What is checked |
|------|----------|-----------------|
| S1 | review | The verification runs first; « corriger » verdict on a seeded bug |
| S2 | ticket | Developer and reviewer never call `question`; no commit before `cp-2` |
| S3 | feature (alias of `spec`) | The planner's blocking question is relayed by the conductor; the `tdd` method is recorded for `implement-spec` |
| S4 | audit | Each phase skill is loaded before its phase |
| S5 | cadrage | The planner follows the subagent variant |
| S6 | debug | Phase skill loaded before each phase |
| S7 | quick | A skill out of scope is reported missing, without loop |
| S8 | ticket | Reports not recorded (`reports: off`): developer and reviewer include their whole report in the block |
| S9 | sweep | The developer and reviewer returns are addressed to `conductor` |
| S10 | quick | A data decision is asked in « Questions bloquantes », not with `question` |
| S11 | ticket | Project with a wiki: a `wiki_propose` proposal is accepted on the card of `cp-2`, applied by oh and committed with the code |
| S12 | doc-sync | A drifting `CONFIRMÉ` page is fixed; oh records the synchronization; only the wiki is committed, after `cp-sync` |
| S13 | onboarding | Phase 1 and 5 skills loaded before the exploration and before writing the wiki; writes limited to `docs/wiki/` and `ONBOARDING.md` |
| S14 | libre | The onboarder launched as a subagent asks no question and returns its envelope to `orchestrator` |
| S15 | cadrage | A UI request has the designer invoked: `ui-protocol` loaded, no question, envelope to its caller |
| S16 | skill-refresh | Reduced hub: only the stale skill is selected, proposed at `cp-refresh` with no write before, then updated (`versions`, `reviewed`) and committed; the release notes input keeps the case deterministic without web search |
| S17 | skill-refresh | Proposal refused at `cp-refresh`: no write nor commit, before or after |
| S18 | skill-refresh | Every stack skill is recent: no checkpoint, no write, no commit |
| S19 | cadrage | "All the tasks" clashes with two rules of the code (private tasks, trash): each one becomes a question asked to the user |
| S20 | spec | "Réservation" means a loan in the code and a waiting list in a type: the meaning is asked to the user, the spec written under `docs/wiki/specs/` carries a glossary that defines it (a `cadrage` case until R4) |
| S21 | spec | Each acceptance criterion of the written spec carries, on its own line, the test that checks it (a `cadrage` case until R4) |
| S22 | ticket | `tdd` method: `cp-seams`, then `cp-red` passed (tests failing for the right reason) before the commit, `cp-2` green, hidden tests |
| S23 | quick | Mutex asked for without changing the API: `go vet ./...` (a CI step, outside the package changed) and hidden tests pass; a naive implementation passes `go test` |
| S24 | debug | Misleading lead: the stack points to `report/format.go`, the fix ticket names the real cause (`splitRecord`, `ledger/parse.go`) |
| S25 | quick | Merge committed with its conflict markers: both sides survive (including a semantic conflict without markers), no marker left |
| S26 | review | Two-axis review: the forgotten ticket criterion (`ErrDivByZero`) is raised under « conformity to the request », the debug print under « code quality »; reports off (`reports: off`) |
| S27 | cadrage | Two designs, only one respects the `depguard` rule of `.golangci.yml` (the domain imports no other package): that option is recommended, the constraint cited, despite the user's preference |
| S28 | skill-refresh | Stack skill without metadata rewritten: `oh skill check --hub .` clean (when `oh` is on the `PATH` of the hidden tests), « Utiliser quand… » description, locators, and nothing of the excerpt given in the notes copied verbatim |
| S29 | spec | The spec is only written under `docs/wiki/specs/`, committed alone by the planner after `cp-spec`; the tickets, created after `cp-tickets`, cite it |
| S30 | spec | A rule added by the user at `cp-spec` becomes a criterion with its test point, covered by the breakdown before the tickets are created |
| S31 | spec | A slice asked outside the spec at `cp-tickets` (the CI) is refused by oh (convergence pass) until a criterion of the spec covers it |
| S33 | release | Since `v1.2.0`, a feature, a fix and an internal commit: `1.3.0`, `[Unreleased]` dated today with an empty section above, comparison links updated, version written in a script without extension (`bin/agenda`) updated, annotated tag on the release commit, nothing before `cp-release`, no push |
| S34 | release | A breaking change announced only in a commit footer (`BREAKING CHANGE:`), missing from the CHANGELOG: `2.0.0`, the change and its migration in the section |
| S35 | release | Release refused at `cp-release`: no write, no commit, no tag |
| S36 | release | Release MR asked on `release/1.3`, without a remote: no MR opened nor push, complete description (title, breaking changes, checks, rollback) and the command to open it after the push in the report (`reports: off`) |
| S37 | implement-spec | Two existing tickets processed one after the other: `cp-plan` before any delegation, one `cp-2` per ticket, review of the whole at `cp-final`; oh records both closed tickets (output `tickets`); hidden tests |
| S38 | implement-spec | Plan refused at `cp-plan`: nothing is delegated, changed nor committed |
| S39 | spec | oh records the spec and the created tickets (`record:` outputs, none declared by an agent) and proposes `implement-spec` with these tickets and this spec (`next:`) |
| S40 | implement-spec | No ticket given: tickets and spec taken from the last `spec` session of the project (`session.output`, `previous:`), implemented, closed ticket recorded by oh; hidden tests |
| S41 | incident | Invoices one cent off since the deployment: the triggering commit (misleading title) is undone alone in the work tree (hidden tests: rounding back, next feature kept), the blameless postmortem (the person blamed is not named) is only committed after `cp-postmortem`, the action tickets after `cp-actions` |
| S42 | incident | Mitigation outside the repository (the `PAYMENTS_MAX_RETRIES=0` switch of the README): no code changed; the postmortem names the cause (`payments/client.go`), gives the impact (3412 payments) and the detection delay (about 17 minutes, read in the log) in figures, without naming anyone |
| S43 | incident | Postmortem refused at `cp-postmortem` with an instruction ("Ce qui a aidé"): revised before it is committed; actions refused at `cp-actions`: no ticket, not asked again |
| S44 | security-fix | Three findings, two kept at `cp-triage`: logged password and open redirect fixed with their test (hidden tests: `//host`, `/\host`, `http:host`, `javascript:` variants), the finding left out is not touched, commit after `cp-2` |
| S45 | security-fix | Path traversal: the fix holds against a neighbouring folder (`uploads-private`) and a symbolic link dropped by a client (hidden tests); the auditor's re-analysis comes before `cp-2` |
| S46 | security-fix | Everything kept at triage: the logged token is fixed with its test, the false positive (comparison already in constant time) is reported without touching `auth/` |
| S47 | onboarding | Re-onboarding (`refresh`) of a wiki whose architecture page cites a package renamed since: the drift is raised, the fix proposed with `wiki_propose`, no direct write in `docs/wiki/` before `cp-wiki`, the page fixed by oh once accepted on the card |
| S48 | onboarding | Re-onboarding of an up-to-date wiki: no proposal, no write, `cp-wiki` passed |
| S49 | spike | encoding/csv without `LazyQuotes` cannot read the real statement (bare quote): kill decision written under `docs/wiki/decisions/` with its reason, throwaway prototype deleted (checked by oh), only the decision committed after `cp-decision` |
| S50 | spike | Go decision corrected to clarify at `cp-decision`: the file is rewritten with the reason, keeps its sourced findings (`proxy_read_timeout 60s`, the caller's 45 s deadline) and is only committed after the second approval |
| S51 | spike | `t.Parallel` everywhere? `os.Setenv` and `os.Chdir` in two tests: kill, both files cited, no test changed |
| S52 | implement-spec | Fan-out (`fanout=true`): two independent tickets, one child session each at the same time; the `cp-2` of each child passes by delegation (CP-R3), oh merges both branches, `cp-merge` validated, oh merges the integration branch and closes both tickets; the parent session commits nor closes anything; hidden tests |
| S53 | implement-spec | Fan-out: two tickets that change the same line, launched at the same time; the merge of the second conflicts, the `merger` launched by oh keeps both changes, oh checks the result; no marker, hidden tests |

`cadrage` is an alias of `spec` since R5: cases S5, S15, S19 and S27 (folder `cadrage/`) launch `spec`, and S27 goes through the exploration by the pathfinder that precedes the interview when the request compares designs.


The reference measure (Haiku, three repetitions) is recorded in `cli/internal/evalrun/testdata/evals/baseline.json`; `make test-eval` compares the current run with it.

## Running faster, within a budget

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5
```

- `OH_EVAL_PARALLEL` (`--parallel`): number of passes run at the same time. Each pass stays isolated (its `OH_HOME`, `HOME`, `TMPDIR`, clone, daemon and proxy) and the report is the same as in a sequential run. Measured on S7 and S10 ×2 with Haiku: 1 min 14 s four at a time against about 3 min 6 s sequentially; each pass takes a little longer (57 s on average against 44 to 49 s).
- `OH_EVAL_BUDGET_TOTAL` (`--budget-total`): cap of the whole evaluation; once reached, no new pass starts. The cap per pass (`OH_EVAL_BUDGET_USD`) still applies: $1 by default, $2.50 with a Sonnet model.
- A pass stopped by a Bedrock rate limit (throttling) runs again up to three times after a growing wait; the « Relances » section of the report says so. When retries pile up, lower `OH_EVAL_PARALLEL`.

## Comparing with and without a skill

A new skill is kept only when an evaluation with and without it shows a gain. `--variant` (or `OH_EVAL_VARIANTS` with `make test-eval`, variants separated by spaces) runs each case with several versions of the hub:

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5 OH_EVAL_CASES=S6 \
  OH_EVAL_VARIANTS="base without-23=-skill:debugger-phase-2-3"
```

| Variant | Hub of the passes |
|---|---|
| `base` | the hub embedded in oh, as is |
| `without-x=-skill:<id>` | the embedded hub without this skill (name or category/name; several `-skill:` separated by commas) |
| `other=<directory>` | another hub (`agents/`, `skills/`, `workflows/`, `permissions/`), for example `..` for the repository root without `make sync-content`; `other=<directory>,-skill:<id>` combines it with a removal |

- **The first variant is the reference**: the one that holds what is evaluated (the hub with the skill). The others are compared with it.
- **Removing a skill**: the file of the skill, and its mentions in `skills` and `native_skills` of the agents' frontmatter. The body of the agents and skills is not rewritten: an instruction « charge `x` » stays, and the model tries to load a missing skill (`no_denied_loop` sees it if it insists). If the hub no longer composes (another skill needs it in `requires:`, a workflow names it in `skills:`), the evaluation stops before any pass and lists the errors.
- **Check**: before any pass, the hub of each variant goes through the skill check (errors of `oh skill check`), the validation of the workflows of the cases run and the composition of their bundle.
- **Passes**: each case × model × variant × repetition is a pass; the variants of a repetition run side by side, `--parallel` and `--budget-total` cover the whole run. The traces go to `passes/<case>/<variant>/<model>/<n>/`; `--rescore` computes the comparison again.
- **Reference measure**: `--record-baseline` refuses variants; the « Référence » column is only filled for a variant that is the embedded hub as is.

### Gain rule

For each case and each model, `report.md` (section « Comparaison avec la référence ») and `report.json` (`comparisons`, `verdicts`) give the pass rate, mean cost and mean duration of each variant, the gap with the reference and a verdict:

| Verdict | When |
|---|---|
| `gain` | the reference passes at least 1/3 more (for example 3/3 against 1/3), or passes as much and costs at least 10 % less on average |
| `ambiguous` | the gap is a single pass (2/3 against 1/3): run ×3 more (`OH_EVAL_REPS=6`, a gap of 1/3 is then two passes) |
| `no_gain` | smaller gap, drop, or same pass rate without a large enough cost cut |
| `incomplete` | no pass on one side (overall cap reached) |

The verdict of a variant over every case run is a gain when there is a gain on at least one case without a drop elsewhere; a drop of a single pass next to a gain makes it ambiguous.

Removing a skill fails by construction the `skill_loaded_before` assertions that target it (a failed load does not count). The « Résultat hors chargement des skills » section therefore makes the comparison and the verdict again without those assertions: the business result (diagnosis, flaw found, hidden tests) tells whether the skill brings something. The « Assertions par variante » table details each assertion (◆: different pass rate).

First measure (13 October 2026, Haiku ×3, S4 and S6 with `base`, `sans-23=-skill:debugger-phase-2-3` and `sans-a3=-skill:auditor-phase-3`, $1.82): without its skill, each case fails on the loading assertion (S6 0/3, S4 0/3), but the business result does not change (S6: cause found and ticket created 3/3; S4: injection found 3/3) and the passes cost 17 to 19 % less. Without the loading of the skills, the verdict is therefore « no gain » for both skills on these simple cases; the control case of the other variant (S4 without `debugger-phase-2-3`) shows a drop of one pass, due to noise in `base` (`auditor-phase-4` loaded one event too late), which makes the verdict of `sans-23` ambiguous.
