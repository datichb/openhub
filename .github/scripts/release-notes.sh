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
#
# The notes are shown on the release page, where a relative link
# (`MIGRATION.md`, `docs/...`, anchors included) leads nowhere. The Markdown
# links and images with a relative target are rewritten to absolute URLs at
# the tag: https://github.com/<repo>/blob/<tag>/<path> (`raw` for images);
# a bare anchor `#x` points to CHANGELOG.md#x at the tag. Absolute links
# (`https:`, `mailto:`...), code spans and code blocks are left as they are.
# The repository is $GITHUB_REPOSITORY (set by GitHub Actions), else
# datichb/openhub.
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

# absolute_links <tag> <repo>: stdin to stdout, relative links made absolute.
absolute_links() {
  awk -v tag="$1" -v repo="$2" '
    function is_absolute(t) {
      return t == "" || t ~ /^<?[A-Za-z][A-Za-z0-9+.-]*:/ || t ~ /^\/\// || t ~ /^</
    }
    function absolute(t, img,   path, kind) {
      if (is_absolute(t)) return t
      kind = img ? "raw" : "blob"
      if (t ~ /^#/) return "https://github.com/" repo "/blob/" tag "/CHANGELOG.md" t
      path = t
      sub(/^\.\//, "", path)
      sub(/^\/+/, "", path)
      return "https://github.com/" repo "/" kind "/" tag "/" path
    }
    # is the link whose "](" ends pre an image: "![...](" ?
    function is_image(pre,   k, c, depth) {
      depth = 1
      for (k = length(pre) - 2; k >= 1; k--) {
        c = substr(pre, k, 1)
        if (c == "]") depth++
        else if (c == "[" && --depth == 0) return k > 1 && substr(pre, k - 1, 1) == "!"
      }
      return 0
    }
    function inline(s,   out, i, pre, tgt) {
      out = ""
      while ((i = index(s, "](")) > 0) {
        pre = out substr(s, 1, i + 1)
        s = substr(s, i + 2)
        match(s, /^[^) \t]*/)
        tgt = substr(s, 1, RLENGTH)
        s = substr(s, RLENGTH + 1)
        out = pre absolute(tgt, is_image(pre))
      }
      return out s
    }
    # code spans (odd-numbered backtick-separated parts are the code).
    function spans(line,   n, parts, k, out) {
      n = split(line, parts, "`")
      out = ""
      for (k = 1; k <= n; k++) {
        out = out (k > 1 ? "`" : "") (k % 2 == 1 ? inline(parts[k]) : parts[k])
      }
      return out
    }
    /^ ? ? ?(```|~~~)/ { fence = !fence; print; next }
    fence { print; next }
    {
      if (match($0, /^ ? ? ?\[[^]]+\]:[ \t]+/)) {
        head = substr($0, 1, RLENGTH)
        rest = substr($0, RLENGTH + 1)
        match(rest, /^[^ \t]*/)
        print head absolute(substr(rest, 1, RLENGTH), 0) substr(rest, RLENGTH + 1)
        next
      }
      print spans($0)
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
printf '%s\n' "$notes" | absolute_links "$tag" "${GITHUB_REPOSITORY:-datichb/openhub}"
