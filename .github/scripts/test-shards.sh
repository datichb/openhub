#!/usr/bin/env bash
# Shards of the Go tests of the CI (job "Go CLI tests", see
# .github/workflows/ci.yml) and of `make test-shard`.
#
# ./cmd is the longest package by far (about 900 tests that run one after the
# other: they share the state of the package). Its tests are split in shards,
# each one a `go test ./cmd -run <names>` of its own, and all the other
# packages make a last shard, "rest". The cmd shards are listed in
# .github/test-shards-cmd.txt: one test per line, with its shard (1 to N) and
# its duration in seconds, as measured by `balance`.
#
#   test-shards.sh ids             prints the shards, one per line (cmd-1 … cmd-N, rest)
#   test-shards.sh args <shard>    prints the arguments of `go test` for the shard,
#                                  one per line: ./cmd -run ^(Test…|Test…)$ for a
#                                  cmd shard, the packages for "rest" (every package
#                                  but ./cmd; on Windows, the ones of
#                                  windows-test-packages.txt)
#   test-shards.sh check           prints an ::error:: and exits 1 when a test of
#                                  ./cmd is in no shard or in several, when the
#                                  file lists a test that does not exist (any more),
#                                  or when ci.yml does not run a shard
#   test-shards.sh sync            puts the new tests of ./cmd in the lightest
#                                  shard and drops the ones that are gone, the
#                                  others stay where they are: what to run when
#                                  `check` says a test is in no shard
#   test-shards.sh balance <json>  distributes the tests again on the shards from
#                                  the durations of a `go test ./cmd -json` run
#                                  (TEST_SHARDS_CMD: number of cmd shards, default
#                                  the current one)
#
# Run it from cli/ (go list, go test -list).
set -euo pipefail
export LC_ALL=C

github_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
shards_file="${TEST_SHARDS_FILE:-$github_dir/test-shards-cmd.txt}"
workflow_file="${TEST_SHARDS_WORKFLOW:-$github_dir/workflows/ci.yml}"
# What `sync` assumes of a test it has never seen, in seconds.
default_seconds="${TEST_SHARDS_DEFAULT_SECONDS:-1}"

# The tests of the file, "name shard seconds" per line: comments and blanks
# removed, CRLF tolerated.
entries() {
  tr -d '\r' <"$shards_file" | sed -e 's/#.*//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | { grep -v '^$' || true; }
}

# The tests of ./cmd, as Go lists them (benchmarks apart), sorted.
go_tests() {
  local out
  out="$(go test -list '^(Test|Example|Fuzz)' ./cmd)" || {
    echo "::error title=Tests of ./cmd not listed::go test -list ./cmd failed: $out" >&2
    return 1
  }
  printf '%s\n' "$out" | { grep -E '^(Test|Example|Fuzz)[A-Za-z0-9_]*$' || true; } | LC_ALL=C sort -u
}

# The number of cmd shards: the "# shards: N" line of the file (TEST_SHARDS_CMD
# overrides it, for `balance`).
shard_count() {
  if [ -n "${TEST_SHARDS_CMD:-}" ]; then
    echo "$TEST_SHARDS_CMD"
    return
  fi
  { grep -E '^# shards: [0-9]+' "$shards_file" || true; } | head -n 1 | sed -e 's/^# shards: \([0-9]*\).*/\1/' | { grep . || echo 0; }
}

# The tests of cmd shard $1, sorted.
shard_tests() {
  entries | awk -v s="$1" '$2 == s { print $1 }' | LC_ALL=C sort -u
}

# The -run pattern of the tests on stdin.
run_pattern() {
  local names
  names="$(paste -sd'|' -)"
  printf '^(%s)$' "$names"
}

# The packages of the rest: every package but ./cmd.
rest_packages() {
  local module all
  module="$(go list -m)"
  all="$(go list ./... | sed -e "s#^$module\$#.#" -e "s#^$module/#./#" | LC_ALL=C sort -u)"
  if [ "${RUNNER_OS:-}" = "Windows" ]; then
    all="$("$github_dir/scripts/windows-packages.sh" list)"
  fi
  printf '%s\n' "$all" | { grep -vx './cmd' || true; }
}

# Writes the file from lines "name shard seconds" on stdin.
write_file() {
  local n="$1"
  {
    echo "# Shards of the tests of ./cmd in the CI (job \"Go CLI tests\")."
    echo "#"
    echo "# Written by .github/scripts/test-shards.sh (balance, sync): do not edit by hand."
    echo "# A new test of ./cmd is in no shard until \`test-shards.sh sync\` (run from cli/)"
    echo "# puts it in the lightest one: the check of the CI fails until then."
    echo "# The shards were balanced on the durations of one run; \`balance <json>\` does it"
    echo "# again from a \`go test ./cmd -json\` run."
    echo "#"
    echo "# shards: $n"
    echo "# Format: test shard (1 to $n) seconds"
    LC_ALL=C sort -k2,2n -k3,3nr -k1,1
  } >"$shards_file"
}

# Distributes "name seconds" lines (stdin) over $1 shards, heaviest first onto
# the lightest shard (ties: the lowest shard). A test costs at least a tenth
# of a second (the many tests that last a few milliseconds are rounded to 0 by
# Go and would all fall in one shard). Prints "name shard seconds".
distribute() {
  LC_ALL=C sort -k2,2nr -k1,1 | awk -v n="$1" '
    BEGIN { for (i = 1; i <= n; i++) load[i] = 0 }
    {
      best = 1
      for (i = 2; i <= n; i++) if (load[i] < load[best]) best = i
      w = $2 + 0 < 0.1 ? 0.1 : $2 + 0
      load[best] += w
      printf "%s %d %.2f\n", $1, best, w
    }'
}

summary() {
  entries | awk '{ load[$2] += $3; count[$2]++ } END { for (s in load) printf "  cmd-%s: %d tests, %.0f s measured\n", s, count[s], load[s] }' | LC_ALL=C sort
}

case "${1:-}" in
  ids)
    n="$(shard_count)"
    for ((i = 1; i <= n; i++)); do echo "cmd-$i"; done
    echo rest
    ;;
  args)
    shard="${2:-}"
    case "$shard" in
      cmd-[0-9]*)
        number="${shard#cmd-}"
        if [ "$number" -lt 1 ] || [ "$number" -gt "$(shard_count)" ]; then
          echo "::error::no shard $shard (cmd-1 to cmd-$(shard_count))" >&2
          exit 1
        fi
        tests="$(shard_tests "$number")"
        if [ -z "$tests" ]; then
          echo "::error::the shard $shard has no test: $shards_file" >&2
          exit 1
        fi
        echo ./cmd
        echo -run
        printf '%s\n' "$tests" | run_pattern
        echo
        ;;
      rest)
        packages="$(rest_packages)"
        [ -n "$packages" ] || { echo "::error::no package in the shard rest" >&2; exit 1; }
        printf '%s\n' "$packages"
        ;;
      *)
        echo "usage: $0 args cmd-<n>|rest" >&2
        exit 2
        ;;
    esac
    ;;
  check)
    all="$(go_tests)"
    [ -n "$all" ] || { echo "::error::go test -list ./cmd found no test" >&2; exit 1; }
    n="$(shard_count)"
    listed="$(entries | awk '{ print $1 }' | LC_ALL=C sort)"
    failures=0
    for t in $(LC_ALL=C comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$listed" | uniq)); do
      echo "::error title=Test in no shard::$t (./cmd) is in no shard of .github/test-shards-cmd.txt. Run .github/scripts/test-shards.sh sync from cli/ and commit the file."
      failures=$((failures + 1))
    done
    for t in $(printf '%s\n' "$listed" | uniq -d); do
      echo "::error title=Test in several shards::$t (./cmd) is listed more than once in .github/test-shards-cmd.txt: keep one line."
      failures=$((failures + 1))
    done
    for t in $(LC_ALL=C comm -13 <(printf '%s\n' "$all") <(printf '%s\n' "$listed" | uniq)); do
      echo "::error title=Unknown test in a shard::$t is in .github/test-shards-cmd.txt but is not a test of ./cmd any more. Run .github/scripts/test-shards.sh sync from cli/."
      failures=$((failures + 1))
    done
    for bad in $(entries | awk -v n="$n" '$2 !~ /^[0-9]+$/ || $2 < 1 || $2 > n { print $1 }'); do
      echo "::error title=Test in a shard that does not exist::$bad is not in a shard from 1 to $n in .github/test-shards-cmd.txt."
      failures=$((failures + 1))
    done
    # What Go runs for each shard is what the file says, and a test runs once:
    # the patterns are the ones of `args`, listed by Go itself.
    counted=""
    for ((i = 1; i <= n; i++)); do
      expected="$(shard_tests "$i")"
      [ -n "$expected" ] || { echo "::error title=Empty shard::cmd-$i has no test in .github/test-shards-cmd.txt."; failures=$((failures + 1)); continue; }
      got="$(go test -list "$(printf '%s\n' "$expected" | run_pattern)" ./cmd | { grep -E '^(Test|Example|Fuzz)[A-Za-z0-9_]*$' || true; } | LC_ALL=C sort -u)"
      if [ "$got" != "$expected" ]; then
        echo "::error title=Shard pattern::the -run pattern of cmd-$i does not select the tests of the file: $(LC_ALL=C comm -3 <(printf '%s\n' "$expected") <(printf '%s\n' "$got") | tr -d '\t' | head -5 | paste -sd' ' -)"
        failures=$((failures + 1))
      fi
      counted="$counted"$'\n'"$got"
    done
    for t in $(printf '%s\n' "$counted" | { grep -v '^$' || true; } | LC_ALL=C sort | uniq -d); do
      echo "::error title=Test run by several shards::$t is selected by more than one shard."
      failures=$((failures + 1))
    done
    # The workflow runs every shard.
    if [ -f "$workflow_file" ]; then
      for id in $(bash "$0" ids); do
        if ! { grep 'shard' "$workflow_file" || true; } | grep -Eq "(^|[^A-Za-z0-9_-])$id($|[^A-Za-z0-9_-])"; then
          echo "::error title=Shard not run::the shard $id is not in the matrix of $(basename "$workflow_file"): add it, or the tests of the shard never run."
          failures=$((failures + 1))
        fi
      done
    fi
    if [ "$failures" -gt 0 ]; then
      echo "$failures problem(s) in the test shards (.github/test-shards-cmd.txt)" >&2
      exit 1
    fi
    echo "Test shards: every test of ./cmd is in exactly one of the $n shards ($(printf '%s\n' "$all" | grep -c .) tests)"
    summary
    ;;
  sync)
    all="$(go_tests)"
    [ -n "$all" ] || { echo "::error::go test -list ./cmd found no test" >&2; exit 1; }
    n="$(shard_count)"
    [ "$n" -ge 1 ] || { echo "::error::no shard in $shards_file: use balance" >&2; exit 1; }
    kept="$(entries | awk -v n="$n" '!seen[$1]++ && $2 >= 1 && $2 <= n' | LC_ALL=C sort -k1,1)"
    new="$(LC_ALL=C comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$kept" | awk 'NF { print $1 }' | LC_ALL=C sort))"
    out="$(LC_ALL=C join -j1 <(printf '%s\n' "$all") <(printf '%s\n' "$kept"))"
    for t in $new; do
      lightest="$(printf '%s\n' "$out" | awk -v n="$n" 'BEGIN { for (i = 1; i <= n; i++) load[i] = 0 } NF { load[$2] += $3 } END { best = 1; for (i = 2; i <= n; i++) if (load[i] < load[best]) best = i; print best }')"
      out="$out"$'\n'"$t $lightest $default_seconds"
      echo "added $t to cmd-$lightest"
    done
    printf '%s\n' "$out" | { grep -v '^$' || true; } | write_file "$n"
    echo "$shards_file written"
    summary
    ;;
  balance)
    json="${2:-}"
    [ -f "$json" ] || { echo "usage: $0 balance <go test ./cmd -json output>" >&2; exit 2; }
    all="$(go_tests)"
    n="$(shard_count)"
    [ "$n" -ge 1 ] || { echo "::error::set TEST_SHARDS_CMD to the number of cmd shards" >&2; exit 1; }
    # Elapsed of the top-level tests that ran (a skipped one costs nothing, but is a test).
    measured="$(grep -E '"Action":"(pass|fail|skip)"' "$json" | grep -E '"Test":"[^/"]*","Elapsed"' |
      sed -e 's/.*"Test":"\([^"]*\)","Elapsed":\([0-9.eE+-]*\).*/\1 \2/' | LC_ALL=C sort -k1,1 -u)"
    {
      LC_ALL=C join -j1 <(printf '%s\n' "$all") <(printf '%s\n' "$measured")
      for t in $(LC_ALL=C comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$measured" | awk '{ print $1 }' | LC_ALL=C sort)); do
        echo "$t $default_seconds"
      done
    } | distribute "$n" | write_file "$n"
    echo "$shards_file written"
    summary
    ;;
  *)
    echo "usage: $0 ids|args <shard>|check|sync|balance <json>" >&2
    exit 2
    ;;
esac
