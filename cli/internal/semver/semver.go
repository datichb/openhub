// Package semver provides simple semantic version parsing and comparison.
package semver

import (
	"regexp"
	"strconv"
	"strings"
)

// Version represents a parsed semantic version (major.minor.patch).
type Version struct {
	Major, Minor, Patch int
}

// Parse parses a semantic version string (e.g., "1.17.2", "v2.0.1-beta").
// It strips the "v" prefix and any pre-release suffix before parsing.
func Parse(s string) Version {
	s = strings.TrimPrefix(s, "v")
	// Remove pre-release suffix (e.g., "-beta", "-SNAPSHOT-abc123")
	if idx := strings.IndexByte(s, '-'); idx > 0 {
		s = s[:idx]
	}
	var v Version
	parts := strings.SplitN(s, ".", 4)
	if len(parts) >= 1 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		v.Patch, _ = strconv.Atoi(parts[2])
	}
	return v
}

// LessThan returns true if v is strictly less than other.
func (v Version) LessThan(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// AtLeast returns true if v >= minimum.
func (v Version) AtLeast(minimum Version) bool {
	return !v.LessThan(minimum)
}

// MajorMinor returns the "major.minor" string (e.g., "2.0" from "2.0.1").
func MajorMinor(version string) string {
	version = strings.TrimPrefix(version, "v")
	if idx := strings.IndexByte(version, '-'); idx > 0 {
		version = version[:idx]
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

// Release is a fully parsed version: semver core, pre-release identifiers
// and, for a `git describe` build (`v4.2.0-195-gfe9ed932[-dirty]`), the
// number of commits after the tag and the dirty flag.
type Release struct {
	Core  Version
	Pre   []string
	Ahead int
	Dirty bool
}

// Dev reports a development build made after its tag (commits after the
// tag or uncommitted changes).
func (r Release) Dev() bool { return r.Ahead > 0 || r.Dirty }

var describeSuffix = regexp.MustCompile(`^(.+)-(\d+)-g[0-9a-f]+$`)

// ParseRelease parses "v5.0.0", "5.0.0-test", "5.0.0-rc.1+build" or a
// `git describe` version. ok is false when s is not such a version ("dev",
// a bare commit hash…).
func ParseRelease(s string) (Release, bool) {
	var r Release
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	if rest, ok := strings.CutSuffix(s, "-dirty"); ok {
		r.Dirty, s = true, rest
	}
	if m := describeSuffix.FindStringSubmatch(s); m != nil {
		r.Ahead, _ = strconv.Atoi(m[2])
		s = m[1]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Release{}, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" {
			return Release{}, false
		}
		nums[i] = n
	}
	r.Core = Version{nums[0], nums[1], nums[2]}
	if hasPre {
		if pre == "" {
			return Release{}, false
		}
		r.Pre = strings.Split(pre, ".")
	}
	return r, true
}

// Compare orders two versions following semver 2.0 (a pre-release is older
// than its release: 4.2.0 < 5.0.0-test < 5.0.0), a `git describe` build
// being newer than its tag. ok is false when either version cannot be
// parsed (see ParseRelease).
func Compare(a, b string) (cmp int, ok bool) {
	ra, okA := ParseRelease(a)
	rb, okB := ParseRelease(b)
	if !okA || !okB {
		return 0, false
	}
	return compareRelease(ra, rb), true
}

func compareRelease(a, b Release) int {
	switch {
	case a.Core.LessThan(b.Core):
		return -1
	case b.Core.LessThan(a.Core):
		return 1
	}
	if c := comparePre(a.Pre, b.Pre); c != 0 {
		return c
	}
	switch {
	case a.Dev() && !b.Dev():
		return 1
	case b.Dev() && !a.Dev():
		return -1
	case a.Ahead != b.Ahead:
		if a.Ahead < b.Ahead {
			return -1
		}
		return 1
	}
	return 0
}

// comparePre orders pre-release identifiers (semver 2.0 §11): none is
// newer than any; numeric identifiers sort numerically and before
// alphanumeric ones; a shorter list is older when all shared fields match.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		na, errA := strconv.Atoi(a[i])
		nb, errB := strconv.Atoi(b[i])
		switch {
		case errA == nil && errB == nil:
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// IsAtLeast checks if version >= minimum (convenience function).
func IsAtLeast(version, minimum string) bool {
	return Parse(version).AtLeast(Parse(minimum))
}

// FromOutput extracts the version number from a `--version` output
// ("1.17.13", "tool v2.0.20"): the first field starting with a digit
// once a "v" prefix is removed, else the trimmed output.
func FromOutput(out string) string {
	for _, f := range strings.Fields(out) {
		f = strings.TrimLeft(f, "vV")
		if f != "" && f[0] >= '0' && f[0] <= '9' {
			return f
		}
	}
	return strings.TrimSpace(out)
}
