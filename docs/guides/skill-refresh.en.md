> [Lire en français](skill-refresh.fr.md)

# Stack skill freshness

The stack skills (`skills/developer/stacks/`, one per language, framework or tool: React, Go, Terraform…) tell the agents how to write code for a given version of a technology. When a major version ships, a skill that did not follow gives outdated advice: removed APIs, changed defaults, replaced practices. This guide explains how oh tracks the age of each skill and how to update it.

In short:

1. each stack skill declares **the versions it covers** and **the date of its last review**;
2. `oh skill check --stale` lists those whose review is older than 90 days;
3. the `skill-refresh` workflow researches the news, proposes the update, waits for your validation, then writes and commits;
4. a monthly task (CI or cron) tells you when skills are to review.

## The freshness metadata

Two frontmatter keys, read by oh and **never delivered to the agents** (they cost no token in the prompts):

```yaml
---
name: dev-standards-react
description: "Utiliser quand le projet utilise React (…) et que le ticket touche ce code."
versions:
  react: "19.3"
  react-compiler: "1.0"
reviewed: 2026-10-12
---
```

| Key | Format | Purpose |
|---|---|---|
| `versions` | lowercase technology (`react`, `spring-boot`, `external-secrets-api`) → quoted version | Versions the content describes. `"12+"` when the content only assumes a minimum; `"5.2 LTS / 6.1"` when it covers two release lines. |
| `reviewed` | `YYYY-MM-DD` date | Last review of the content against the published versions. |

Rules:

- every stack skill carries both keys: otherwise `oh skill check` reports `missing_freshness` (error); a malformed value (uppercase key, empty version, unreadable or future date) is an `invalid_freshness`;
- each version of `versions:` is also on the « > Versions de référence (…) : … » line of the skill, which the agents read (checked by the hub tests);
- `reviewed:` only moves when the skill was really compared with the published versions. A skill read again without a source keeps its date;
- another skill (outside `developer/stacks/`) may carry these keys: it is then tracked the same way.

## See the skills to review

```
oh skill check --stale --hub .
```

Run it at the root of a checkout of the hub (`--hub .`); without `--hub`, oh checks the installed hub (`~/.oh/hub`), which only changes with an update of oh.

```
Stack skills to review (41 checked, review every 90 days at most, today 2026-10-13):

  ! developer/stacks/dev-standards-cypress: reviewed on 2026-04-30 (166 days ago), covers cypress 12+
  ! developer/stacks/dev-standards-kubernetes: reviewed on 2026-04-30 (166 days ago), covers kubernetes 1.21+

  2 skill(s) to review: oh run skill-refresh (docs/guides/skill-refresh.en.md)
```

| Option | Effect |
|---|---|
| `--max-age 180d` | Another maximum age: days (`90`, `90d`) or weeks (`12w`) |
| `--strict` | Exit code 1 when a skill is to review (CI, cron) |
| `--json` | Report as JSON (`today`, `max_age_days`, `checked`, `skills`) |

**Why 90 days?** Most stacks of the hub publish a minor version every one to three months (React, Next.js, Vitest, Playwright…), Kubernetes every four months, Go and TypeScript every six months. One review per quarter catches every release cycle without turning the watch into permanent work. With a monthly check, about a third of the skills fall due each month, if the reviews are spread out. 180 days would miss a whole Kubernetes or Go release.

The command makes no network call: it compares dates, not the published versions. The research of the `skill-refresh` workflow is what compares with the published versions.

## Update: the `skill-refresh` workflow

In a checkout of the hub registered as an oh project ([`oh project add`](../reference/cli-projects.en.md#oh-project-add)):

```
oh run skill-refresh                                   # the skills whose review is older than 90 days (5 at most)
oh run skill-refresh -i skills=dev-standards-react     # a named skill, even a recent one
oh run skill-refresh -i max_age_days=60
```

For the web search, turn it on in oh (`oh config websearch enable`); without it, give the release notes yourself in the `notes` input (the `documentarian` never guesses a version and does not move `reviewed:` of a skill it could check against no source).

Steps:

1. **Selection**: the named skills, otherwise those whose `reviewed:` is older than `max_age_days` or that have no metadata, the oldest first, 5 at most per session. When there is none, the `documentarian` says so and the session stops, without a checkpoint.
2. **Research**: for each technology of `versions:`, the latest stable version and what changed since the version covered (recommended features, deprecated or removed APIs, defaults, migration guides), from your notes then from the official sources (release notes, changelog, documentation).
3. **Proposal**, without writing anything, presented at the `cp-refresh` checkpoint: for each skill, versions before → after, each change (section, before → after, reason, source) and the new `reviewed` date.
4. **Your decision**: validate; refuse with a correction (the `documentarian` reworks the proposal and goes through `cp-refresh` again); or refuse altogether (« nothing now »): nothing is written.
5. **After the validation**: it applies the proposal (content, « Versions de référence » line, `versions:`, `reviewed:`), then commits the skills alone (`docs(skills): refresh … stack skill(s) (skill-refresh)`). The push is yours.

The added content follows the rule of the hub skills: **in French, inspired by the sources and never copied** (code examples rewritten, no paragraph taken as is), in the structure and tone of the skill, replacing what is outdated rather than adding. A prerelease version (beta, release candidate) is never the reference.

After the commit, check the hub before pushing:

```
oh skill check --hub .
cd cli && make sync-content && go test ./internal/bundle -run 'Skill|Freshness|Budget'
```

The budget test (`budget.golden`) shows whether a stack skill grew; an increase is justified in the commit message or the MR.

**Why the `documentarian`?** It already writes the documentation, has the web search (`websearch`, `webfetch`), writes files and commits, and loads `skill-authoring-protocol` (writing a hub skill) and `research` (web research: sources, cross-checking, security, reporting). No new agent is needed.

## Monthly task

The update stays a supervised session: the `cp-refresh` checkpoint waits for a person in every mode. The scheduled task therefore only **warns**.

### In CI (hub repository)

The GitHub Actions workflow [`.github/workflows/skill-freshness.yml`](../../.github/workflows/skill-freshness.yml) runs on the 1st of each month (and on demand, `workflow_dispatch`): it builds oh, writes the list of the skills to review in the job summary and fails when there is one (`--strict`). GitHub notifies the failure of a scheduled job. On another CI system, the same principle takes two commands:

```
cd cli && make embed-sync && go build -o bin/oh .
OH_HOME=$(mktemp -d) ./bin/oh skill check --stale --hub .. --strict
```

### Locally (cron)

```
# the 1st of the month at 9:00: list of the skills to review in ~/.oh/skill-stale.txt
0 9 1 * * cd ~/workspace/openhub && oh skill check --stale --hub . > ~/.oh/skill-stale.txt 2>&1
```

Then, when the list is not empty: `oh run skill-refresh` in the checkout of the hub.

## Limits

- `oh skill check --stale` does not compare with the latest published versions (no network call): it measures the age of the reviews. The comparison is done by the research of `skill-refresh`.
- `reviewed:` says when a skill was reviewed, not that it is right: the validation at the checkpoint remains the guarantee.

See also: [`oh skill check`](../reference/cli-tools.en.md#oh-skill-check) · [`skill-refresh` workflow](../reference/workflows.en.md#skill-refresh) · [Stack skills](../architecture/skills.en.md#stack-skills--developerstacks) · [Authoring skills](authoring-skills.en.md)
