// Package wildcard is the whole-value wildcard match shared by the limits,
// the credential proxy and the permission rules: the same pattern must give
// the same answer wherever oh evaluates it, and as the tool interprets it.
package wildcard

// Match reports whether s matches the whole pattern. '*' matches any run of
// characters (empty, and '/' included), '?' matches exactly one character.
// There is no escape: every other character stands for itself. Characters
// are runes, not bytes, so one '?' matches an accented letter or an emoji.
func Match(pattern, s string) bool {
	p, v := []rune(pattern), []rune(s)
	pi, si, star, mark := 0, 0, -1, 0
	for si < len(v) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == v[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}
