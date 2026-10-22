package opencodev2

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Pinned integrity of the opencode Linux packages (audit v5, S-38).
//
// The integrity npm gives for a package comes from the same registry response
// as the tarball: a compromised registry entry supplies both. So oh carries,
// for the opencode versions it was checked against, the sha512 integrity of
// each native package (pinned_integrity.json, embedded). A version that is in
// the table is downloaded only when npm announces exactly that integrity; a
// version that is not is downloaded as before, with a warning, and refused
// when execution.strict_isolation is set.
//
// To pin a new opencode version (a maintainer does it when the supported range
// moves or a release is validated; never copy the value from a machine that
// installed the package through the same channel you want to check):
//
//  1. run the read-only script, from the root of the repository:
//     .github/scripts/opencode-pin.sh [--darwin] <version>
//     For each of the four Linux packages it reads dist.integrity and
//     dist.tarball from the registry, downloads the tarball, recomputes its
//     sha512 and fails if it differs from the announced integrity; then it
//     prints the fragment to paste. --darwin also prints the SHA-256 of the
//     macOS packages (OPENCODE_SHA256_DARWIN_* of .github/workflows/ci.yml);
//  2. run it a second time, ideally from another network, and compare the two
//     fragments: they must be identical. Compare them as well with the
//     integrity npm gives (npm view @opencode/cli-<t>@<version> dist.integrity);
//  3. add the fragment to pinned_integrity.json (TestPinnedIntegrityTable
//     checks the format, the targets and that the version is within the
//     supported range). Never write the file from a test;
//  4. if the version becomes the recommended one, set opencode_recommended in
//     compatibility.json (default and the oh version) and OPENCODE_VERSION and
//     the two darwin SHA-256 in .github/workflows/ci.yml: TestRecommendedVersionIsPinned
//     and TestCIVersionIsRecommended check the three agree;
//  5. remove the versions that are no longer supported.
//
// Without the script, the same check by hand, for each target <t>
// (linux-x64, linux-x64-musl, linux-arm64, linux-arm64-musl):
//
//	npm view @opencode/cli-<t>@<version> dist.integrity
//	curl -sSL https://registry.npmjs.org/@opencode/cli-<t>/-/cli-<t>-<version>.tgz |
//	  openssl dgst -sha512 -binary | base64
//
// (the first value is "sha512-" followed by the output of the second).
//
// Do not change the file in the same commit as a bump of the range without
// having done the check.

//go:embed pinned_integrity.json
var pinnedIntegrityJSON []byte

// IntegrityPins maps an opencode version to the sha512 integrity of its
// native Linux package, by npm target (linux-x64, linux-arm64-musl…).
type IntegrityPins map[string]map[string]string

type pinFile struct {
	Versions IntegrityPins `json:"versions"`
}

var (
	embeddedPinsOnce sync.Once
	embeddedPins     IntegrityPins
	embeddedPinsErr  error
)

// PinnedIntegrity returns the table embedded in oh.
func PinnedIntegrity() (IntegrityPins, error) {
	embeddedPinsOnce.Do(func() {
		embeddedPins, embeddedPinsErr = parsePins(pinnedIntegrityJSON)
	})
	return embeddedPins, embeddedPinsErr
}

func parsePins(data []byte) (IntegrityPins, error) {
	var f pinFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("pinned opencode integrity table: %w", err)
	}
	for ver, targets := range f.Versions {
		for target, integrity := range targets {
			if err := checkIntegrity(integrity); err != nil {
				return nil, fmt.Errorf("pinned opencode integrity table: %s %s: %w", ver, target, err)
			}
		}
	}
	if f.Versions == nil {
		f.Versions = IntegrityPins{}
	}
	return f.Versions, nil
}

// checkIntegrity accepts a sha512 subresource integrity: "sha512-" and the
// base64 of 64 bytes.
func checkIntegrity(s string) error {
	b64, ok := strings.CutPrefix(s, "sha512-")
	if !ok {
		return fmt.Errorf("%q is not a sha512 integrity", s)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) != 64 {
		return fmt.Errorf("%q is not the base64 of a sha512 digest", s)
	}
	return nil
}

// lookup returns the pinned integrity of a version and target.
func (p IntegrityPins) lookup(version, target string) (string, bool) {
	s, ok := p[version][target]
	return s, ok
}
