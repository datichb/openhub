> [Lire en français](release.fr.md)

# Preparing a release

The `release` workflow prepares a release of the project: it chooses the version, dates the CHANGELOG, writes the release commit and the annotated tag and, when you ask, the description of the release merge request. You approve everything at a single checkpoint, **before** any commit or tag. oh never pushes: publishing stays your move.

```bash
oh run release                          # version deduced from the commits and the CHANGELOG
oh run release -i version=5.0.0         # forced version
oh run release -i mr=true -i target=main  # with the description of the release MR
```

## Before the launch: the preconditions

oh checks the repository before starting the session and refuses the launch when:

- the work tree has uncommitted changes (modified, staged or untracked files; `.beads/` excluded) — the message lists the first ones;
- the current branch is neither `main`, `master` nor `release/*` (a detached HEAD is refused too).

Without a `CHANGELOG.md` at the root, oh only warns: the release-manager will create it. These tests (`git_clean`, `branch`, `tools`) are available to any workflow: see the [schema](../reference/workflow-schema.en.md#preconditions).

## During the session

1. The `release-manager` reads the last tag (`git describe --tags`), the commits since that tag (bodies included, for the `BREAKING CHANGE:` footers), the `[Unreleased]` section and the files where the version is written.
2. It presents at **`cp-release`**, without writing anything: the version and its reason (a breaking change gives a major, a feature a minor, fixes alone a patch; in `0.y.z`, a breaking change gives a minor), the CHANGELOG section as it will be written, the tag message, the release notes and, with `mr=true`, the title and description of the MR.
3. You approve, correct ("fix: release 5.0.0") or refuse. A refusal: nothing is written.
4. After the approval: CHANGELOG (and version files, each one asked), release commit, annotated tag on that commit, then the MR when the branch already exists on the remote.
5. The report gives the push commands and how to undo.

## What oh locks

| Operation | Before `cp-release` | After `cp-release` |
|---|---|---|
| `git commit` | refused | allowed until the release commit |
| `git tag` (creation, listing included; `git push --tags`, `update-ref refs/tags/…`, `gh release create`, `glab release create`) | refused | allowed **once the release is committed**: the tag always marks the release commit; a single tag, refused again after the first |
| opening an MR (`glab mr create`, `gh pr create`, their APIs, MCP tool `create_mr`) | refused | allowed |
| `git push` | refused | refused (the release-manager may not push) |
| `git tag -f`, `git tag -d` | refused | refused |

The tag and the MR are refused in any other workflow, even to an agent allowed to run `git tag`: only a `risk: publish` workflow can unlock them. A `publish` workflow never runs remotely; its publishing checkpoint pauses in every mode.

## Why oh does not push

Pushing a tag publishes the release: the CI (goreleaser for oh) builds and distributes the binaries, and a pushed tag cannot be cleanly withdrawn. It is the only truly irreversible step: it stays in your hands, after reviewing the commit and the tag locally.

```bash
git show --stat HEAD && git cat-file -p v5.0.0   # review
git push origin main                              # the branch first
git push origin v5.0.0                            # then this tag alone (never --tags)
```

Undo before the push: `git tag -d v5.0.0`, then `git reset --soft HEAD~1`.

## The release merge request

With `mr=true`, the description follows the `pr-description` skill: why, what changes, breaking changes and migration, checks, risks and rollback, links. When the branch is not on the remote yet, the release-manager creates nothing: it gives you the description and the command to run after your push (`glab mr create …` or `gh pr create …`).

## Recipe: test on a copy, without risk

To try the workflow on a real project without touching the repository:

```bash
git clone /path/to/project /tmp/release-test
cd /tmp/release-test
git remote remove origin                 # no remote any more: nothing can leave
oh run release                           # or -i version=X.Y.Z
git tag -l && git show --stat HEAD       # local tag and release commit
```

To check: the launch is refused after `echo x >> README.md` (dirty tree) and on a `feat/x` branch; no `git tag` nor `git commit` passes before `cp-release` is approved; the tag is annotated (`git cat-file -t <tag>` answers `tag`) and points to `HEAD`; nothing is pushed.
