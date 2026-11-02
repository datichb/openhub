#!/usr/bin/env bash
# Packages the CI tests on Windows (see .github/windows-test-packages.txt).
#
#   windows-packages.sh list    prints the packages to test, one per line
#                               (./internal/..., usable as arguments of go test)
#   windows-packages.sh check   prints an ::error:: and exits 1 when a package
#                               of `go list ./...` is in neither list, when
#                               one is in both, or when an entry of a list is
#                               no package any more
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
    failures=0
    for p in $(LC_ALL=C comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$classified")); do
      echo "::error title=Package not classified for Windows::$p is in neither .github/windows-test-packages.txt nor .github/windows-test-packages-excluded.txt. Add it to the first (it is tested on Windows) or to the second with its cause."
      failures=$((failures + 1))
    done
    for p in $(LC_ALL=C comm -12 <(printf '%s\n' "$tested") <(printf '%s\n' "$excluded")); do
      echo "::error title=Package listed twice for Windows::$p is in both lists of .github: keep it in one."
      failures=$((failures + 1))
    done
    for p in $(LC_ALL=C comm -23 <(printf '%s\n' "$classified") <(printf '%s\n' "$all")); do
      echo "::error title=Unknown package in a Windows list::$p is in a list of .github but is not a package of go list ./... any more. Remove it."
      failures=$((failures + 1))
    done
    if [ "$failures" -gt 0 ]; then
      echo "$failures problem(s) in the Windows package lists (.github/windows-test-packages*.txt)" >&2
      exit 1
    fi
    echo "Windows package lists: every package is classified ($(printf '%s\n' "$tested" | grep -c .) tested, $(printf '%s\n' "$excluded" | grep -c .) excluded)"
    ;;
  *)
    echo "usage: $0 list|check" >&2
    exit 2
    ;;
esac
