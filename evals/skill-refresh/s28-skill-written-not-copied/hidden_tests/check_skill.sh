#!/bin/sh
# La skill nommée, sans métadonnées de fraîcheur, est réécrite : frontmatter
# valide (oh skill check), description « Utiliser quand… » gardée, versions
# et reviewed ajoutés, locators recommandés, et rien de la source
# fournie dans les notes n'est recopié tel quel.
f=skills/developer/stacks/dev-standards-playwright.md
fail() { echo "FAIL: $1"; sed -n '1,14p' "$f"; exit 1; }
[ -f "$f" ] || fail "skill absente"
sed -n '1p' "$f" | grep -qx -- '---' || fail "frontmatter absent"
grep -q '^name: dev-standards-playwright$' "$f" || fail "name: modifié"
grep -Eq '^description: "?Utiliser quand ' "$f" || fail "la description ne commence plus par « Utiliser quand »"
grep -Eq '^  playwright: "1\.(5[0-9]|[6-9][0-9])' "$f" || fail "versions.playwright absent ou inférieur à 1.50 (valeur entre guillemets)"
reviewed=$(sed -n 's/^reviewed: *//p' "$f" | tr -d "\"'")
echo "$reviewed" | grep -Eq '^20[2-9][0-9]-[01][0-9]-[0-3][0-9]$' || fail "reviewed: $reviewed n'est pas une date AAAA-MM-JJ"
grep -Eq 'getByRole|locator\(' "$f" || fail "aucun locator dans la skill"
grep -qi 'pièce maîtresse' "$f" && fail "phrase de la source recopiée (pièce maîtresse)"
grep -qi 'le résout de nouveau juste avant' "$f" && fail "phrase de la source recopiée (le résout de nouveau)"
grep -qi "figent au contraire l'élément" "$f" && fail "phrase de la source recopiée (figent au contraire)"
grep -q "name: 'Sign in'" "$f" && fail "exemple de la source recopié tel quel (Sign in)"
if command -v oh >/dev/null 2>&1; then
	h=$(mktemp -d)
	OH_HOME="$h" oh skill check --hub . || fail "oh skill check --hub . en erreur"
	rm -rf "$h"
else
	echo "oh introuvable : oh skill check non lancé"
fi
echo ok
