package cmd

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/semver"
)

// versionStatus is the position of the running oh against a published one.
type versionStatus int

const (
	versionUnknown   versionStatus = iota // "dev" or an unparsable version: no comparison
	versionSame                           // same release
	versionAhead                          // newer (pre-release of a later version, dev build after the tag)
	versionAvailable                      // a newer release is published
)

// compareOhVersion places current against latest (semver 2.0, `git describe`
// builds newer than their tag): oh never offers an older version as an
// update (A1).
func compareOhVersion(current, latest string) versionStatus {
	if current == "" || current == "dev" {
		return versionUnknown
	}
	c, ok := semver.Compare(current, latest)
	switch {
	case !ok:
		if strings.TrimPrefix(current, "v") == strings.TrimPrefix(latest, "v") {
			return versionSame
		}
		return versionUnknown
	case c == 0:
		return versionSame
	case c > 0:
		return versionAhead
	default:
		return versionAvailable
	}
}

// upgradePlan is what `oh upgrade oh` does for a current, latest and
// requested (empty = latest) version.
type upgradePlan struct {
	status    versionStatus // current against the resolved version
	version   string        // version to install ("" = nothing to do)
	downgrade bool          // an explicit older version was requested
}

func planUpgrade(current, latest, requested string) upgradePlan {
	requested = strings.TrimPrefix(requested, "v")
	if requested == "" {
		st := compareOhVersion(current, latest)
		switch st {
		case versionSame, versionAhead:
			return upgradePlan{status: st}
		}
		if st == versionUnknown && current == latest {
			return upgradePlan{status: versionSame}
		}
		return upgradePlan{status: st, version: latest}
	}
	st := compareOhVersion(current, requested)
	if st == versionSame {
		return upgradePlan{status: st}
	}
	return upgradePlan{status: st, version: requested, downgrade: st == versionAhead}
}
