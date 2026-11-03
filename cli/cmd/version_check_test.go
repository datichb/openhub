package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A1: `oh upgrade oh` never installs an older version unless it is asked for
// explicitly (then flagged as a downgrade).
func TestPlanUpgradeNeverDowngradesImplicitly(t *testing.T) {
	tests := []struct {
		current, latest, requested string
		want                       upgradePlan
	}{
		{"5.0.0-test", "4.2.0", "", upgradePlan{status: versionAhead}},
		{"v4.2.0-195-gfe9ed932", "4.2.0", "", upgradePlan{status: versionAhead}},
		{"4.2.0", "4.2.0", "", upgradePlan{status: versionSame}},
		{"4.1.0", "4.2.0", "", upgradePlan{status: versionAvailable, version: "4.2.0"}},
		{"5.0.0-test", "5.0.0", "", upgradePlan{status: versionAvailable, version: "5.0.0"}},
		{"dev", "4.2.0", "", upgradePlan{status: versionUnknown, version: "4.2.0"}},
		{"5.0.0", "5.0.0", "v4.2.0", upgradePlan{status: versionAhead, version: "4.2.0", downgrade: true}},
		{"4.2.0", "5.0.0", "4.2.0", upgradePlan{status: versionSame}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, planUpgrade(tt.current, tt.latest, tt.requested), "%+v", tt)
	}
}
