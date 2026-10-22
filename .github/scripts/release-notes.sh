#!/usr/bin/env bash
# Release notes of a tag, taken from CHANGELOG.md.
#
#   release-notes.sh <tag> [changelog]
#
# Prints the body of the `## [X.Y.Z]` section of the changelog (heading
# excluded, surrounding blank lines and trailing `---` separators trimmed) for the tag vX.Y.Z. The default
# changelog is CHANGELOG.md at the root of the repository.
#
# A pre-release tag (v5.0.0-rc.1) reads its own section `## [5.0.0-rc.1]`
# when there is one, else the section of the version it prepares
# (`## [5.0.0]`). Fails (exit 1, message on stderr) when no section is found
# or when the section is empty: a release is never published without notes.
set -euo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: release-notes.sh <tag> [changelog]" >&2
  exit 2
fi

tag="$1"
changelog="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/CHANGELOG.md}"
version="${tag#v}"

if [ ! -f "$changelog" ]; then
  echo "release-notes: $changelog not found" >&2
  exit 1
fi

# section <version>: body of `## [version]` (up to the next `## [`).
section() {
  awk -v want="$1" '
    /^## \[/ {
      if (found) exit
      name = substr($0, 5)
      name = substr(name, 1, index(name, "]") - 1)
      if (name == want) found = 1
      next
    }
    found { print }
  ' "$changelog" | sed -e 's/\r$//' | awk '
    NF { started = 1 }
    started { lines[++n] = $0 }
    END {
      while (n > 0 && (lines[n] !~ /[^[:space:]]/ || lines[n] ~ /^---+[[:space:]]*$/)) n--
      for (i = 1; i <= n; i++) print lines[i]
    }
  '
}

notes="$(section "$version")"
if [ -z "$notes" ] && [ "${version#*-}" != "$version" ]; then
  notes="$(section "${version%%-*}")"
fi

if [ -z "$notes" ]; then
  echo "release-notes: no non-empty section '## [$version]' in $changelog (tag $tag)" >&2
  exit 1
fi
printf '%s\n' "$notes"
