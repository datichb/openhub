package opencode

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

//go:embed compatibility.json
var compatibilityJSON []byte

// CompatMatrix holds the full compatibility matrix.
type CompatMatrix struct {
	OhVersions map[string]CompatRange `json:"oh_versions"`
}

// CompatRange defines the supported version range for a given oh version.
type CompatRange struct {
	OpencodeMin string `json:"opencode_min"`
	OpencodeMax string `json:"opencode_max"`
	RTKMin      string `json:"rtk_min"`
	Notes       string `json:"notes"`
}

// CompatResult holds the outcome of a compatibility check.
type CompatResult struct {
	Compatible bool
	Warning    string
}

// CheckCompatibility verifies that the installed opencode version is compatible
// with the current oh version. Returns a CompatResult with details.
func CheckCompatibility(ohVersion, opencodeVersion string) CompatResult {
	if opencodeVersion == "" {
		return CompatResult{Compatible: false, Warning: "version opencode inconnue"}
	}

	matrix, err := loadMatrix()
	if err != nil {
		return CompatResult{Compatible: true, Warning: ""}
	}

	// Find the matching oh version range (match on major.minor)
	ohMajorMinor := majorMinor(ohVersion)
	r, ok := matrix.OhVersions[ohMajorMinor]
	if !ok {
		// No entry for this oh version — assume compatible
		return CompatResult{Compatible: true, Warning: ""}
	}

	// Compare opencode version against range
	oc := parseVersion(opencodeVersion)
	minVer := parseVersion(r.OpencodeMin)
	maxVer := parseVersion(r.OpencodeMax)

	if oc.lessThan(minVer) {
		return CompatResult{
			Compatible: false,
			Warning: fmt.Sprintf("opencode %s est trop ancien (minimum requis: %s)",
				opencodeVersion, r.OpencodeMin),
		}
	}
	if maxVer.lessThan(oc) {
		return CompatResult{
			Compatible: false,
			Warning: fmt.Sprintf("opencode %s n'a pas été testé avec oh %s (max testé: %s)",
				opencodeVersion, ohVersion, r.OpencodeMax),
		}
	}

	return CompatResult{Compatible: true, Warning: ""}
}

func loadMatrix() (*CompatMatrix, error) {
	var matrix CompatMatrix
	if err := json.Unmarshal(compatibilityJSON, &matrix); err != nil {
		return nil, fmt.Errorf("parsing compatibility matrix: %w", err)
	}
	return &matrix, nil
}

// majorMinor extracts "2.0" from "2.0.1" or "2.0.0-SNAPSHOT-abc123".
func majorMinor(version string) string {
	version = strings.TrimPrefix(version, "v")
	// Remove any pre-release suffix
	if idx := strings.IndexByte(version, '-'); idx > 0 {
		version = version[:idx]
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

// semver is a simple semantic version for comparison.
type semver struct {
	major, minor, patch int
}

func parseVersion(s string) semver {
	s = strings.TrimPrefix(s, "v")
	// Remove pre-release suffix
	if idx := strings.IndexByte(s, '-'); idx > 0 {
		s = s[:idx]
	}
	parts := strings.SplitN(s, ".", 4)
	v := semver{}
	if len(parts) >= 1 {
		v.major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		v.minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		v.patch, _ = strconv.Atoi(parts[2])
	}
	return v
}

func (v semver) lessThan(other semver) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}
