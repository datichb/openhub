#!/usr/bin/env bash
# Tests of test-shards.sh and merge-coverage.sh, on a small module of their own
# (a ./cmd package of five tests, a package ./other): no dependency on the tests
# of oh. Run it from anywhere.
set -euo pipefail

scripts="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

failures=0
fail() { echo "FAIL: $*" >&2; failures=$((failures + 1)); }
# expect <description> <exit code wanted> <command>...
expect() {
  local what="$1" want="$2"
  shift 2
  local out code=0
  out="$("$@" 2>&1)" || code=$?
  if [ "$code" -ne "$want" ]; then
    fail "$what: exit $code, wanted $want: $out"
  fi
  LAST="$out"
}
# contains <description> <text>: LAST has text.
contains() { case "$LAST" in *"$2"*) ;; *) fail "$1: no '$2' in: $LAST" ;; esac; }

mkdir -p "$work/mod/cmd" "$work/mod/other"
cd "$work/mod"
cat >go.mod <<'EOF'
module example.com/shards

go 1.22
EOF
cat >cmd/a_test.go <<'EOF'
package cmd

import "testing"

func TestAlpha(t *testing.T)    {}
func TestAlphaMore(t *testing.T) {}
func TestBeta(t *testing.T)     {}
func TestGamma(t *testing.T)    {}
func TestDelta(t *testing.T)    {}
EOF
cat >cmd/doc.go <<'EOF'
package cmd
EOF
cat >other/other_test.go <<'EOF'
package other

import "testing"

func TestOther(t *testing.T) {}
EOF
cat >other/doc.go <<'EOF'
package other
EOF

export TEST_SHARDS_FILE="$work/shards.txt" TEST_SHARDS_WORKFLOW="$work/ci.yml"
cat >"$TEST_SHARDS_WORKFLOW" <<'EOF'
        shard: [cmd-1, cmd-2, rest]
EOF
shards="$scripts/test-shards.sh"

# balance: from a go test -json output, TestAlpha is the long one.
cat >"$work/run.json" <<'EOF'
{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/shards/cmd","Test":"TestAlpha","Elapsed":10}
{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/shards/cmd","Test":"TestAlpha/sub","Elapsed":9}
{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/shards/cmd","Test":"TestAlphaMore","Elapsed":4}
{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/shards/cmd","Test":"TestBeta","Elapsed":3}
{"Time":"2026-01-01T00:00:00Z","Action":"skip","Package":"example.com/shards/cmd","Test":"TestGamma","Elapsed":0}
{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.com/shards/cmd","Elapsed":17}
EOF
: >"$TEST_SHARDS_FILE"
TEST_SHARDS_CMD=2 expect "balance" 0 "$shards" balance "$work/run.json"
expect "check after balance" 0 "$shards" check
contains "check says every test is placed" "exactly one of the 2 shards (5 tests)"
expect "ids" 0 "$shards" ids
[ "$LAST" = $'cmd-1\ncmd-2\nrest' ] || fail "ids: $LAST"
# TestAlpha (10 s) alone on a shard, the others (4 + 3 + 0.1 + 0.1) on the other.
expect "args cmd-1" 0 "$shards" args cmd-1
[ "$LAST" = $'./cmd\n-run\n^(TestAlpha)$' ] || fail "args cmd-1: $LAST"
expect "args cmd-2" 0 "$shards" args cmd-2
[ "$LAST" = $'./cmd\n-run\n^(TestAlphaMore|TestBeta|TestDelta|TestGamma)$' ] || fail "args cmd-2: $LAST"
expect "args rest" 0 "$shards" args rest
[ "$LAST" = "./other" ] || fail "args rest: $LAST"
expect "args of a shard that does not exist" 1 "$shards" args cmd-3
expect "args of nothing" 2 "$shards" args bogus

# The patterns do not overlap: TestAlpha is not TestAlphaMore (anchored).
TEST_SHARDS_CMD=2 expect "check, prefix names" 0 "$shards" check

# A test that is in no shard fails the check, naming it.
grep -v '^TestDelta ' "$TEST_SHARDS_FILE" >"$work/without" && cp "$work/without" "$TEST_SHARDS_FILE"
expect "check, test in no shard" 1 "$shards" check
contains "names the test" "TestDelta (./cmd) is in no shard"
# sync puts it back, in the lightest shard.
expect "sync" 0 "$shards" sync
contains "sync reports it" "added TestDelta to cmd-"
expect "check after sync" 0 "$shards" check

# A test listed twice (in two shards) fails.
cp "$TEST_SHARDS_FILE" "$work/good"
other=1; grep -q '^TestBeta 1 ' "$TEST_SHARDS_FILE" && other=2
echo "TestBeta $other 1" >>"$TEST_SHARDS_FILE"
expect "check, test in two shards" 1 "$shards" check
contains "names the test listed twice" "TestBeta (./cmd) is listed more than once"
cp "$work/good" "$TEST_SHARDS_FILE"

# A test that is gone fails, sync drops it.
echo "TestGone 1 1" >>"$TEST_SHARDS_FILE"
expect "check, unknown test" 1 "$shards" check
contains "names the unknown test" "TestGone is in .github/test-shards-cmd.txt"
expect "sync drops it" 0 "$shards" sync
expect "check after the drop" 0 "$shards" check

# A shard number outside 1..N.
cp "$work/good" "$TEST_SHARDS_FILE"
sed -i.bak 's/^TestGamma [0-9]/TestGamma 7/' "$TEST_SHARDS_FILE" && rm -f "$TEST_SHARDS_FILE.bak"
expect "check, shard 7" 1 "$shards" check
contains "names the test of the unknown shard" "TestGamma is not in a shard from 1 to"
cp "$work/good" "$TEST_SHARDS_FILE"

# A shard the workflow does not run.
echo "        shard: [cmd-1, rest]" >"$TEST_SHARDS_WORKFLOW"
expect "check, shard not in the workflow" 1 "$shards" check
contains "names the shard" "the shard cmd-2 is not in the matrix"
echo "        shard: [cmd-1, cmd-2, rest]" >"$TEST_SHARDS_WORKFLOW"
expect "check, back to green" 0 "$shards" check

# The merge of coverage profiles.
merge="$scripts/merge-coverage.sh"
printf 'mode: atomic\nexample.com/shards/cmd/a.go:3.10,5.2 1 4\nexample.com/shards/cmd/a.go:6.10,7.2 1 0\n' >"$work/p1.out"
printf 'mode: atomic\nexample.com/shards/other/o.go:3.10,5.2 2 1\n' >"$work/p2.out"
expect "merge" 0 "$merge" "$work/merged.out" "$work/p1.out" "$work/p2.out"
[ "$(head -n 1 "$work/merged.out")" = "mode: atomic" ] || fail "merge: header"
[ "$(grep -c '^mode:' "$work/merged.out")" = 1 ] || fail "merge: the header must be there once"
[ "$(wc -l <"$work/merged.out" | tr -d ' ')" = 4 ] || fail "merge: wanted the header and three blocks: $(cat "$work/merged.out")"
printf 'mode: set\nexample.com/shards/other/o.go:3.10,5.2 2 1\n' >"$work/p3.out"
expect "merge, another mode" 1 "$merge" "$work/bad.out" "$work/p1.out" "$work/p3.out"
contains "merge names the mode" "cannot be merged"
: >"$work/empty.out"
expect "merge, empty profile" 1 "$merge" "$work/bad.out" "$work/p1.out" "$work/empty.out"
expect "merge, missing profile" 1 "$merge" "$work/bad.out" "$work/p1.out" "$work/nothing.out"

if [ "$failures" -gt 0 ]; then
  echo "$failures failure(s) in the tests of the scripts" >&2
  exit 1
fi
echo "Tests of test-shards.sh and merge-coverage.sh: all passed"
