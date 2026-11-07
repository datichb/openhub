#!/usr/bin/env bash
# Shards of the Go tests of the CI (job "Go CLI tests", see
# .github/workflows/ci.yml) and of `make test-shard`.
#
# ./cmd is the longest package by far (about 900 tests that run one after the
# other: they share the state of the package). Its tests are split in shards
# cmd-1 … cmd-N, each one a `go test ./cmd -run <names>`; the other packages
# are split in shards rest-1 … rest-M, each one a `go test <packages>`. The
# shards are listed in two files, one line per test or package with its shard
# and its duration in seconds, as measured by `balance`:
#
#   .github/test-shards-cmd.txt    the tests of ./cmd
#   .github/test-shards-rest.txt   the packages of `go list ./...` but ./cmd
#
#   test-shards.sh ids             prints the shards, one per line
#   test-shards.sh args <shard>    prints the arguments of `go test` for the shard,
#                                  one per line: ./cmd -run ^(Test…|Test…)$ for a
#                                  cmd shard, the packages for a rest shard (on
#                                  Windows only the ones of windows-test-packages.txt;
#                                  nothing at all when there is none in the shard)
#   test-shards.sh check           prints an ::error:: and exits 1 when a test of
#                                  ./cmd or a package is in no shard or in several,
#                                  when a file lists one that does not exist (any
#                                  more), or when ci.yml does not run a shard
#   test-shards.sh sync            puts the new tests and packages in the lightest
#                                  shard and drops the ones that are gone, the others
#                                  stay where they are: what to run when `check`
#                                  says something is in no shard
#   test-shards.sh balance <json>...  distributes everything again on the shards from
#                                  the durations of `go test -json` runs (the tests of
#                                  ./cmd and the packages: one run of ./... or several)
#                                  (TEST_SHARDS_CMD, TEST_SHARDS_REST: the number of
#                                  shards of each file, by default the current ones)
#
# Run it from cli/ (go list, go test -list).
set -euo pipefail
export LC_ALL=C

github_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cmd_file="${TEST_SHARDS_FILE:-$github_dir/test-shards-cmd.txt}"
rest_file="${TEST_SHARDS_REST_FILE:-$github_dir/test-shards-rest.txt}"
workflow_file="${TEST_SHARDS_WORKFLOW:-$github_dir/workflows/ci.yml}"
# What `sync` assumes of a test or package it has never seen, in seconds.
default_seconds="${TEST_SHARDS_DEFAULT_SECONDS:-1}"

# The kind being handled, cmd or rest, and its file: set by use.
kind="" file="" env_count=""
use() {
  kind="$1"
  case "$kind" in
    cmd) file="$cmd_file" env_count="${TEST_SHARDS_CMD:-}" ;;
    rest) file="$rest_file" env_count="${TEST_SHARDS_REST:-}" ;;
  esac
}

# The lines of the file, "name shard seconds": comments and blanks removed,
# CRLF tolerated.
entries() {
  tr -d '\r' <"$file" | sed -e 's/#.*//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | { grep -v '^$' || true; }
}

# The tests of ./cmd, as Go lists them (benchmarks apart), sorted.
go_tests() {
  local out
  out="$(go test -list '^(Test|Example|Fuzz)' ./cmd)" || {
    echo "::error title=Tests of ./cmd not listed::go test -list ./cmd failed: $out" >&2
    return 1
  }
  printf '%s\n' "$out" | { grep -E '^(Test|Example|Fuzz)[A-Za-z0-9_]*$' || true; } | sort -u
}

# Every package but ./cmd, as `go list ./...` prints it relative to the module.
go_packages() {
  local module
  module="$(go list -m)"
  go list ./... | sed -e "s#^$module\$#.#" -e "s#^$module/#./#" | { grep -vx './cmd' || true; } | sort -u
}

# What can be in a shard of the kind.
units() {
  if [ "$kind" = cmd ]; then go_tests; else go_packages; fi
}

# The number of shards of the kind: the "# shards: N" line of its file (the
# environment overrides it, for `balance`).
shard_count() {
  if [ -n "$env_count" ]; then
    echo "$env_count"
    return
  fi
  [ -f "$file" ] || { echo 0; return; }
  { grep -E '^# shards: [0-9]+' "$file" || true; } | head -n 1 | sed -e 's/^# shards: \([0-9]*\).*/\1/' | { grep . || echo 0; }
}

# The units of shard $1, sorted.
shard_units() {
  entries | awk -v s="$1" '$2 == s { print $1 }' | sort -u
}

# The -run pattern of the tests on stdin.
run_pattern() {
  local names
  names="$(paste -sd'|' -)"
  printf '^(%s)$' "$names"
}

# Writes the file of the kind from lines "name shard seconds" on stdin.
write_file() {
  local n="$1" what
  if [ "$kind" = cmd ]; then what="tests of ./cmd"; else what="packages (but ./cmd)"; fi
  {
    echo "# Shards of the $what in the CI (job \"Go CLI tests\")."
    echo "#"
    echo "# Written by .github/scripts/test-shards.sh (balance, sync): do not edit by hand."
    echo "# A new test or package is in no shard until \`test-shards.sh sync\` (run from cli/)"
    echo "# puts it in the lightest one: the check of the CI fails until then."
    echo "# The shards were balanced on the durations of one run; \`balance <json>\` does it"
    echo "# again from a \`go test ./... -json\` run."
    echo "#"
    echo "# shards: $n"
    echo "# Format: name shard (1 to $n) seconds"
    sort -k2,2n -k3,3nr -k1,1
  } >"$file"
}

# Distributes "name seconds" lines (stdin) over $1 shards, heaviest first onto
# the lightest shard (ties: the lowest shard). A unit costs at least a tenth
# of a second (the many tests that last a few milliseconds are rounded to 0 by
# Go and would all fall in one shard). Prints "name shard seconds".
distribute() {
  sort -k2,2nr -k1,1 | awk -v n="$1" '
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
  entries | awk -v k="$kind" '{ load[$2] += $3; count[$2]++ } END { for (s in load) printf "  %s-%s: %d, %.0f s measured\n", k, s, count[s], load[s] }' | sort
}

# The arguments of `go test` for a shard id; prints nothing on a rest shard
# that has no package to test here.
shard_args() {
  local id="$1" number tests packages
  case "$id" in
    cmd-[0-9]*) use cmd ;;
    rest-[0-9]*) use rest ;;
    *)
      echo "usage: $0 args cmd-<n>|rest-<n>" >&2
      exit 2
      ;;
  esac
  number="${id#*-}"
  if [ "$number" -lt 1 ] || [ "$number" -gt "$(shard_count)" ]; then
    echo "::error::no shard $id ($kind-1 to $kind-$(shard_count))" >&2
    exit 1
  fi
  if [ "$kind" = cmd ]; then
    tests="$(shard_units "$number")"
    if [ -z "$tests" ]; then
      echo "::error::the shard $id has no test: $file" >&2
      exit 1
    fi
    echo ./cmd
    echo -run
    printf '%s\n' "$tests" | run_pattern
    echo
    return
  fi
  packages="$(shard_units "$number")"
  if [ "${RUNNER_OS:-}" = "Windows" ]; then
    # Only the packages whose tests pass there.
    packages="$(comm -12 <(printf '%s\n' "$packages") <("$github_dir/scripts/windows-packages.sh" list | sort -u))"
  elif [ -z "$packages" ]; then
    echo "::error::the shard $id has no package: $file" >&2
    exit 1
  fi
  [ -z "$packages" ] || printf '%s\n' "$packages"
}

# Checks the file of the kind; prints the errors and returns how many.
check_kind() {
  local all n listed failures=0 t bad i expected got counted="" label
  if [ "$kind" = cmd ]; then label="test of ./cmd"; else label="package"; fi
  all="$(units)"
  [ -n "$all" ] || { echo "::error::no $label found by go" >&2; return 1; }
  n="$(shard_count)"
  if [ ! -f "$file" ] || [ "$n" -lt 1 ]; then
    echo "::error title=No shards::$file is missing or has no '# shards: N' line. Run .github/scripts/test-shards.sh balance."
    return 1
  fi
  listed="$(entries | awk '{ print $1 }' | sort)"
  for t in $(comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$listed" | uniq)); do
    echo "::error title=In no shard::$t ($label) is in no shard of ${file#"$github_dir"/}. Run .github/scripts/test-shards.sh sync from cli/ and commit the file."
    failures=$((failures + 1))
  done
  for t in $(printf '%s\n' "$listed" | uniq -d); do
    echo "::error title=In several shards::$t ($label) is listed more than once in ${file#"$github_dir"/}: keep one line."
    failures=$((failures + 1))
  done
  for t in $(comm -13 <(printf '%s\n' "$all") <(printf '%s\n' "$listed" | uniq)); do
    echo "::error title=Unknown in a shard::$t is in ${file#"$github_dir"/} but is not a $label any more. Run .github/scripts/test-shards.sh sync from cli/."
    failures=$((failures + 1))
  done
  for bad in $(entries | awk -v n="$n" '$2 !~ /^[0-9]+$/ || $2 < 1 || $2 > n { print $1 }'); do
    echo "::error title=Shard that does not exist::$bad is not in a shard from 1 to $n in ${file#"$github_dir"/}."
    failures=$((failures + 1))
  done
  for ((i = 1; i <= n; i++)); do
    expected="$(shard_units "$i")"
    if [ -z "$expected" ]; then
      echo "::error title=Empty shard::$kind-$i has nothing in ${file#"$github_dir"/}."
      failures=$((failures + 1))
      continue
    fi
    # For ./cmd, what Go runs for each shard is what the file says, and a test
    # runs once: the pattern is the one of `args`, listed by Go itself.
    if [ "$kind" = cmd ]; then
      got="$(go test -list "$(printf '%s\n' "$expected" | run_pattern)" ./cmd | { grep -E '^(Test|Example|Fuzz)[A-Za-z0-9_]*$' || true; } | sort -u)"
      if [ "$got" != "$expected" ]; then
        echo "::error title=Shard pattern::the -run pattern of cmd-$i does not select the tests of the file: $(comm -3 <(printf '%s\n' "$expected") <(printf '%s\n' "$got") | tr -d '\t' | head -5 | paste -sd' ' -)"
        failures=$((failures + 1))
      fi
      counted="$counted"$'\n'"$got"
    fi
  done
  for t in $(printf '%s\n' "$counted" | { grep -v '^$' || true; } | sort | uniq -d); do
    echo "::error title=Run by several shards::$t is selected by more than one shard."
    failures=$((failures + 1))
  done
  # The workflow runs every shard.
  if [ -f "$workflow_file" ]; then
    for ((i = 1; i <= n; i++)); do
      if ! { grep 'shard' "$workflow_file" || true; } | grep -Eq "(^|[^A-Za-z0-9_-])$kind-$i($|[^A-Za-z0-9_-])"; then
        echo "::error title=Shard not run::the shard $kind-$i is not in the matrix of $(basename "$workflow_file"): add it, or what is in the shard never runs."
        failures=$((failures + 1))
      fi
    done
  fi
  [ "$failures" -gt 0 ] || echo "Test shards: every $label is in exactly one of the $n $kind shards ($(printf '%s\n' "$all" | grep -c .))"
  return "$failures"
}

# Adds the new units to the lightest shard of the kind and drops the gone ones.
sync_kind() {
  local all n kept new out t lightest
  all="$(units)"
  [ -n "$all" ] || { echo "::error::no $kind unit found by go" >&2; return 1; }
  n="$(shard_count)"
  [ "$n" -ge 1 ] || { echo "::error::no shard in $file: use balance" >&2; return 1; }
  kept="$(entries | awk -v n="$n" '!seen[$1]++ && $2 >= 1 && $2 <= n' | sort -k1,1)"
  new="$(comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$kept" | awk 'NF { print $1 }' | sort))"
  out="$(join -j1 <(printf '%s\n' "$all") <(printf '%s\n' "$kept"))"
  for t in $new; do
    lightest="$(printf '%s\n' "$out" | awk -v n="$n" 'BEGIN { for (i = 1; i <= n; i++) load[i] = 0 } NF { load[$2] += $3 } END { best = 1; for (i = 2; i <= n; i++) if (load[i] < load[best]) best = i; print best }')"
    out="$out"$'\n'"$t $lightest $default_seconds"
    echo "added $t to $kind-$lightest"
  done
  for t in $(comm -13 <(printf '%s\n' "$all") <(printf '%s\n' "$kept" | awk 'NF { print $1 }' | sort)); do
    echo "dropped $t"
  done
  printf '%s\n' "$out" | { grep -v '^$' || true; } | write_file "$n"
  echo "$file written"
  summary
}

# Distributes the units of the kind again from the measure in $1.
balance_kind() {
  local json="$1" all n measured
  all="$(units)"
  n="$(shard_count)"
  [ "$n" -ge 1 ] || { echo "::error::set TEST_SHARDS_$(echo "$kind" | tr a-z A-Z) to the number of $kind shards" >&2; return 1; }
  if [ "$kind" = cmd ]; then
    # Elapsed of the top-level tests of ./cmd that ran (a skipped one costs
    # nothing, but is a test).
    measured="$(grep -E '"Action":"(pass|fail|skip)"' "$json" | grep -E '"Package":"[^"]*/cmd","Test":"[^/"]*","Elapsed"' |
      sed -e 's/.*"Test":"\([^"]*\)","Elapsed":\([0-9.eE+-]*\).*/\1 \2/' | sort -k1,1 -u)"
  else
    # Elapsed of the packages (module path removed).
    measured="$(grep -E '"Action":"(pass|fail|skip)"' "$json" | grep -E '"Package":"[^"]*","Elapsed"' | grep -v '"Test"' |
      sed -e 's/.*"Package":"\([^"]*\)","Elapsed":\([0-9.eE+-]*\).*/\1 \2/' | awk -v m="$(go list -m)" '{ p = $1; if (p == m) p = "."; else sub("^" m "/", "./", p); print p, $2 }' | sort -k1,1 -u)"
  fi
  {
    join -j1 <(printf '%s\n' "$all") <(printf '%s\n' "$measured")
    for t in $(comm -23 <(printf '%s\n' "$all") <(printf '%s\n' "$measured" | awk '{ print $1 }' | sort)); do
      echo "$t $default_seconds"
    done
  } | distribute "$n" | write_file "$n"
  echo "$file written"
  summary
}

case "${1:-}" in
  ids)
    for k in cmd rest; do
      use "$k"
      n="$(shard_count)"
      for ((i = 1; i <= n; i++)); do echo "$k-$i"; done
    done
    ;;
  args)
    shard_args "${2:-}"
    ;;
  check)
    failures=0
    for k in cmd rest; do
      use "$k"
      check_kind || failures=$((failures + $?))
    done
    if [ "$failures" -gt 0 ]; then
      echo "$failures problem(s) in the test shards (.github/test-shards-*.txt)" >&2
      exit 1
    fi
    ;;
  sync)
    for k in cmd rest; do
      use "$k"
      sync_kind
    done
    ;;
  balance)
    [ "$#" -ge 2 ] || { echo "usage: $0 balance <go test -json output>..." >&2; exit 2; }
    shift
    for f in "$@"; do
      [ -f "$f" ] || { echo "$f: no such file" >&2; exit 2; }
    done
    json="$(mktemp)"
    trap 'rm -f "$json"' EXIT
    cat "$@" >"$json"
    for k in cmd rest; do
      use "$k"
      balance_kind "$json"
    done
    ;;
  *)
    echo "usage: $0 ids|args <shard>|check|sync|balance <json>" >&2
    exit 2
    ;;
esac
