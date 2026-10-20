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
//  1. read the integrity of the four packages, from the registry:
//     for t in linux-x64 linux-x64-musl linux-arm64 linux-arm64-musl; do
//     echo $t $(npm view @opencode/cli-$t@<version> dist.integrity)
//     done
//  2. check each value against the tarball itself:
//     curl -sSL https://registry.npmjs.org/@opencode/cli-<t>/-/cli-<t>-<version>.tgz |
//     openssl dgst -sha512 -binary | base64
//     (the value of step 1 is "sha512-" followed by this output);
//  3. add the version, with its four targets, to pinned_integrity.json
//     (TestPinnedIntegrityTable checks the format, the targets and that the
//     version is within the supported range);
//  4. remove the versions that are no longer supported.
//
// Do not change the file in the same commit as a bump of the range without
// having done step 2.

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
