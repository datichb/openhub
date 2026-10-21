#!/usr/bin/env bash
# Packages the CI tests on Windows (see .github/windows-test-packages.txt).
#
#   windows-packages.sh list    prints the packages to test, one per line
#                               (./internal/..., usable as arguments of go test)
#   windows-packages.sh check   prints a ::warning:: for every package of
#                               `go list ./...` that is in neither list, and
#                               for every entry of a list that is no package
#                               (never fails because of them)
#
# Run it from cli/ (go list ./...).
set -euo pipefail

github_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tested_file="${WINDOWS_TESTED_FILE:-$github_dir/windows-test-packages.txt}"
excluded_file="${WINDOWS_EXCLUDED_FILE:-$github_dir/windows-test-packages-excluded.txt}"

# Names of a list: comments and blanks removed, CRLF tolerated, sorted.
names() {
  tr -d '\r' <"$1" | sed -e 's/#.*//' -e 's/^[[:space:]]*//' -e 's/[[:space:]].*$//' | { grep -v '^$' || true; } | LC_ALL=C sort -u
}

module="$(go list -m)"
all="$(go list ./... | sed -e "s#^$module\$#.#" -e "s#^$module/#./#" | LC_ALL=C sort -u)"
tested="$(names "$tested_file")"
excluded="$(names "$excluded_file")"

case "${1:-}" in
  list)
    selected="$(LC_ALL=C comm -12 <(printf '%s\n' "$tested") <(printf '%s\n' "$all"))"
    if [ -z "$selected" ]; then
      echo "::error::no package to test on Windows: check $tested_file" >&2
      exit 1
    fi
    printf '%s\n' "$selected"
    ;;
  check)
    classified="$(printf '%s\n%s\n' "$tested" "$excluded" | { grep -v '^$' || true; } | LC_ALL=C sort -u)"
    for p in $(LC_ALL=C comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$classified")); do
      echo "::warning title=Package not classified for Windows::$p is in neither .github/windows-test-packages.txt nor .github/windows-test-packages-excluded.txt: it is not tested on Windows. Add it to one of them."
    done
    for p in $(LC_ALL=C comm -12 <(printf '%s\n' "$tested") <(printf '%s\n' "$excluded")); do
      echo "::warning title=Package listed twice for Windows::$p is in both lists of .github: it is tested on Windows."
    done
    for p in $(LC_ALL=C comm -23 <(printf '%s\n' "$classified") <(printf '%s\n' "$all")); do
      echo "::warning title=Unknown package in a Windows list::$p is in a list of .github but is not a package of go list ./... any more. Remove it."
    done
    ;;
  *)
    echo "usage: $0 list|check" >&2
    exit 2
    ;;
esac
