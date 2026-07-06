// Package semver provides simple semantic version parsing and comparison.
package semver

import (
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

// IsAtLeast checks if version >= minimum (convenience function).
func IsAtLeast(version, minimum string) bool {
	return Parse(version).AtLeast(Parse(minimum))
}
