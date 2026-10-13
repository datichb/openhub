#!/bin/sh
# La skill périmée est revue : reviewed avance (date valide, après la
# fixture), versions passe à Vitest 4 ou plus (selon la recherche web) et le contenu retiré par Vitest 4
# (coverage.all, vitest.workspace) ne reste pas comme consigne.
f=skills/developer/stacks/dev-standards-vitest.md
fail() { echo "FAIL: $1"; sed -n '1,12p' "$f"; exit 1; }
reviewed=$(sed -n 's/^reviewed: *//p' "$f" | tr -d "\"'")
echo "$reviewed" | grep -Eq '^20[2-9][0-9]-[01][0-9]-[0-3][0-9]$' || fail "reviewed: $reviewed n'est pas une date AAAA-MM-JJ"
latest=$(printf '%s\n%s\n' 2025-03-03 "$reviewed" | sort | tail -n 1)
[ "$reviewed" != 2025-03-03 ] && [ "$latest" = "$reviewed" ] || fail "reviewed: $reviewed n'a pas avancé"
grep -Eq '^  vitest: "?([4-9]|[1-9][0-9])' "$f" || fail "versions.vitest n'est pas 4 ou plus"
grep -q 'all: true' "$f" && fail "coverage.all (retiré par Vitest 4) reste dans l'exemple"
grep -q 'name: dev-standards-vitest' "$f" || fail "name: modifié"
echo ok
