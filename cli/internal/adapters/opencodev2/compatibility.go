package opencodev2

import (
	_ "embed"
	"encoding/json"
	"github.com/datichb/openhub/cli/internal/adapters"

	"github.com/datichb/openhub/cli/internal/semver"
)

// Supported opencode releases per oh version (major.minor). oh v5 only
// supports opencode V2 (D3, revised on 06/10/2026): older releases are
// refused before any launch.

//go:embed compatibility.json
var compatibilityJSON []byte

// CompatRange is the supported opencode range of an oh version.
type CompatRange struct {
	OpencodeMin string `json:"opencode_min"`
	OpencodeMax string `json:"opencode_max"`
	// OpencodeRecommended is the release oh is validated against: the one
	// pinned (pinned_integrity.json) and installed by the CI.
	OpencodeRecommended string `json:"opencode_recommended,omitempty"`
	Notes               string `json:"notes,omitempty"`
}

type compatMatrix struct {
	Default    CompatRange            `json:"default"`
	OhVersions map[string]CompatRange `json:"oh_versions"`
}

// ErrNotInstalled is returned when no opencode binary is found.
var ErrNotInstalled = adapters.ErrToolNotInstalled

// UnsupportedError is returned for an opencode release outside the range
// supported by this oh version (opencode V1 in particular).
type UnsupportedError = adapters.UnsupportedVersionError

// DisplayName is the tool name shown to users; Command its binary.
const (
	DisplayName = "opencode"
	Command     = "opencode"
)

// SupportedRange returns the opencode range supported by ohVersion
// ("dev" or an unknown version: the default range).
func SupportedRange(ohVersion string) CompatRange {
	var m compatMatrix
	if err := json.Unmarshal(compatibilityJSON, &m); err != nil {
		return CompatRange{OpencodeMin: "2.0.0", OpencodeMax: "2.99.99"}
	}
	if r, ok := m.OhVersions[semver.MajorMinor(ohVersion)]; ok {
		return r
	}
	return m.Default
}

// RecommendedVersion returns the opencode release this oh version is validated
// against (the default range's: "dev" or an unknown version).
func RecommendedVersion() string {
	return SupportedRange("").OpencodeRecommended
}

// CheckVersion returns an *UnsupportedError when version is outside the
// range supported by ohVersion.
func CheckVersion(ohVersion, version string) error {
	r := SupportedRange(ohVersion)
	v := semver.Parse(version)
	if version == "" || v.LessThan(semver.Parse(r.OpencodeMin)) || semver.Parse(r.OpencodeMax).LessThan(v) {
		return &UnsupportedError{Tool: DisplayName, Found: version, Min: r.OpencodeMin, Max: r.OpencodeMax}
	}
	return nil
}
