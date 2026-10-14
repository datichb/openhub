#!/bin/sh
# Release <version> après <tag précédent> : section datée du jour, section
# [Unreleased] vide au-dessus, liens de comparaison à jour, tag annoté sur le
# commit de release (HEAD), CHANGELOG commité, aucun autre tag, arbre propre ;
# les motifs suivants (expressions de grep -E) sont dans la section de la
# version.
v="$1"; prev="$2"; shift 2; tag="v$v"; f=CHANGELOG.md; today=$(date +%Y-%m-%d)
fail() { sed -n '1,30p' "$f"; git log --oneline -5; git tag -l; echo "FAIL: $1"; exit 1; }
[ "$(git cat-file -t "$tag" 2>/dev/null)" = tag ] || fail "$tag absent ou non annoté"
[ "$(git rev-parse "$tag^{commit}")" = "$(git rev-parse HEAD)" ] || fail "$tag ne marque pas HEAD"
git diff --name-only HEAD~1 HEAD | grep -qx "$f" || fail "le commit de release ne contient pas $f"
[ "$(git tag -l | sort | tr '\n' ' ')" = "$(printf '%s\n%s\n' "$prev" "$tag" | sort | tr '\n' ' ')" ] || fail "tags inattendus : $(git tag -l | tr '\n' ' ')"
[ -z "$(git status --porcelain -- . ':(exclude)check_release.sh' ':(exclude).beads')" ] || fail "arbre de travail sale : $(git status --porcelain)"
esc=$(echo "$v" | sed 's/\./\\./g')
grep -Eq "^## \[$esc\] (-|–|—) $today\$" "$f" || fail "pas de titre « ## [$v] - $today »"
un=$(grep -n '^## \[Unreleased\]' "$f" | cut -d: -f1); rel=$(grep -En "^## \[$esc\]" "$f" | cut -d: -f1)
[ -n "$un" ] && [ "$un" -lt "$rel" ] || fail "[Unreleased] absent ou sous [$v]"
sed -n "$((un + 1)),$((rel - 1))p" "$f" | grep -q '^- ' && fail "[Unreleased] n'est pas vide"
grep -Eq "^\[Unreleased\]: .*/compare/$tag\.\.\.HEAD\$" "$f" || fail "lien [Unreleased] non mis à jour"
grep -Eq "^\[$esc\]: .*/compare/$prev\.\.\.$tag\$" "$f" || fail "lien [$v] absent"
section=$(awk -v h="## [$v]" 'index($0, h) == 1 { on = 1; next } on && /^## \[/ { exit } on' "$f")
for p in "$@"; do
  echo "$section" | grep -Eqi -- "$p" || fail "la section [$v] ne mentionne pas « $p »"
done
echo ok
