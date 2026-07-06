package opencode

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/datichb/openhub/cli/internal/semver"
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
	ohMajorMinor := semver.MajorMinor(ohVersion)
	r, ok := matrix.OhVersions[ohMajorMinor]
	if !ok {
		// No entry for this oh version — assume compatible
		return CompatResult{Compatible: true, Warning: ""}
	}

	// Compare opencode version against range
	oc := semver.Parse(opencodeVersion)
	minVer := semver.Parse(r.OpencodeMin)
	maxVer := semver.Parse(r.OpencodeMax)

	if oc.LessThan(minVer) {
		return CompatResult{
			Compatible: false,
			Warning: fmt.Sprintf("opencode %s est trop ancien (minimum requis: %s)",
				opencodeVersion, r.OpencodeMin),
		}
	}
	if maxVer.LessThan(oc) {
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
