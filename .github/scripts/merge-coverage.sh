#!/usr/bin/env bash
# Merges Go coverage profiles into one.
#
#   merge-coverage.sh <out> <profile>...
#
# The profiles are written by `go test -coverprofile` in the same mode (the CI
# uses -covermode=atomic): the first line of each is « mode: <mode> », then one
# line per block of statements. The result has the header once and every
# block of every profile. A block that is in several profiles (a package whose
# tests ran in several shards) stays on several lines: `go tool cover` adds up
# the counts of the same block. A profile in another mode, an empty one or a
# missing one is an error: the total would not be the one of the tests.
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <out> <profile>..." >&2
  exit 2
fi
out="$1"
shift

mode=""
for f in "$@"; do
  [ -s "$f" ] || { echo "::error::coverage profile $f is missing or empty" >&2; exit 1; }
  head="$(head -n 1 "$f" | tr -d '\r')"
  case "$head" in
    mode:\ *) ;;
    *) echo "::error::$f is not a coverage profile (first line: $head)" >&2; exit 1 ;;
  esac
  if [ -z "$mode" ]; then
    mode="$head"
  elif [ "$head" != "$mode" ]; then
    echo "::error::$f is in '$head' and the others in '$mode': the profiles cannot be merged" >&2
    exit 1
  fi
done

{
  echo "$mode"
  for f in "$@"; do
    tr -d '\r' <"$f" | tail -n +2
  done
} >"$out"
echo "Merged $# profile(s) into $out ($(($(wc -l <"$out") - 1)) blocks)"
