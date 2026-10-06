> [Lire en francais](review-feedback.fr.md)

# Code Review and Feedback — Guide

## Overview

openhub provides an AI-powered code review pipeline with three stages: automated review, merge request publication, and feedback processing. This guide covers the complete workflow from review to correction.

---

## Review Modes

Launch an AI code review with the `review` workflow:

```bash
oh run review                                   # The reviewer offers the mode choice at start
oh run review -i review_mode=standard           # Standard review
oh run review -i review_mode=adversarial        # Adversarial review (edge cases, security)
oh run review -i review_mode=edge-case          # Edge-case focused review
oh run review -i review_mode=standard+adversarial  # Combined modes
oh run review -i review_mode=all                # All review modes
```

`oh review [--mode …] [--branch …]` remains a deprecated alias of `oh run review`.

**Automatic branch detection:** When on a feature branch, the reviewer agent automatically detects the base branch and injects `[BRANCH:feature/xyz] [BASE:main]` context into the review prompt.

### Inputs and flags

| Input / flag | Description |
|------|-------------|
| `-i review_mode=…` | Review mode: `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` (empty: choice at start) |
| `-i branch=…` | Branch to review (empty: recent changes) |
| `-i base=…` | Base branch (default: `main`) |
| `-p`, `--project` | Project ID |

See [Shipped workflows › review](../reference/workflows.en.md#review).

---

## Publishing a Merge Request

After the review completes, publish the results as a GitLab merge request. Publishing is not a workflow: it remains an oh command.

```bash
oh review --publish                          # Create MR for current branch
oh review --publish --reviewer alice         # Create MR and assign reviewer
```

This command:
1. Creates a merge request on GitLab for the current feature branch
2. Optionally assigns a reviewer (resolves team member ID to GitLab user ID)
3. Transitions the claim status to `review` in the team state
4. Appends a `review.ready` event and dispatches a notification
5. Applies the `agent-reviewed` label to the MR

> **Note:** The merge itself remains a manual developer action. `--publish` only creates the MR.

### Flags

| Flag | Description |
|------|-------------|
| `--publish` | Create a GitLab merge request |
| `--reviewer` | Team member ID to assign as reviewer |

---

## Processing Feedback

When a human reviewer leaves comments on the MR, the `review-feedback` workflow addresses them. `oh review feedback <ticket-or-branch>` reads the discussions on GitLab then launches this workflow (deprecated alias of `oh run review-feedback`: oh shows a warning):

```bash
oh review feedback BD-42             # By ticket reference
oh review feedback feat/auth-flow    # By branch name
```

The ticket or branch is required. With `oh run review-feedback`, you provide the `mr`, `branch` and `feedback` (discussions) inputs yourself (`-i key=value`): see [Shipped workflows › review-feedback](../reference/workflows.en.md#review-feedback).

### How it works

1. **Fetches unresolved MR discussions** from GitLab
2. **Displays a preview**: MR info, unresolved discussion count, authors, affected files
3. **Asks for confirmation** before launching
4. **Launches the `review-feedback` workflow** with a structured prompt containing all discussions
5. The agent reads each comment, applies corrections, runs tests, makes a grouped commit
6. Optionally replies on each resolved thread via `gitlab_reply_to_mr_discussion`

### Limits

- Maximum **30 discussions** per feedback session
- Maximum **2000 characters** per note body (truncated with `[truncated]`)

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID |
| `--yes` | | Skip confirmation prompt |

---

## End-to-End Workflow

```
1. oh run ticket                          # Implement the feature
2. oh run review -i review_mode=standard  # AI reviews the code
3. oh review --publish                    # Create MR on GitLab
4. [Human reviews on GitLab]              # Reviewer leaves comments
5. oh review feedback BD-42               # AI addresses feedback (review-feedback workflow)
6. [Human approves MR]                    # Final approval
7. [Developer merges]                     # Manual merge
```

---

## Prerequisites

- **GitLab token** with `api` scope configured via `oh mcp setup gitlab` or `oh secrets set GITLAB_TOKEN`
- **MCP GitLab server** enabled in `hub.toml`
- **Team state** initialized (`oh team init`) for claim status transitions and notifications

---

## Troubleshooting

### MR not found

```
Error: no merge request found for branch "feat/xyz"
```

Verify the branch has been pushed to the remote. `oh review feedback` looks for an open MR matching the branch name.

### Insufficient token permissions

```
Error: 403 Forbidden
```

The GitLab token needs the `api` scope. Tokens with `read_api` only cannot create MRs or read discussions.

### Feedback prompt too large

If the MR has many discussions, the prompt may be capped. The agent processes the first 30 discussions with truncated note bodies (2000 chars max). For very large reviews, address feedback in multiple rounds.

---

## Resources

- [GitLab Integration Guide](gitlab-integration.en.md)
- [MCP Team Server Reference](../reference/mcp-team.en.md)
- [CLI Reference](../reference/cli.en.md)
