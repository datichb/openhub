package selfupdate

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestCheckVersionChange(t *testing.T) {
	rel := func(tag string) *Release { return &Release{TagName: tag} }
	tests := []struct {
		name      string
		current   string
		release   string
		requested string
		opts      updateOptions
		want      error // nil = allowed; ErrDowngrade; or a substring in wantMsg
		wantMsg   string
	}{
		{name: "newer", current: "4.2.0", release: "v5.0.0"},
		{name: "newer than a pre-release", current: "5.0.0-test", release: "v5.0.0"},
		{name: "newer than a describe build of the previous tag", current: "v4.2.0-195-gfe9ed932", release: "v5.0.0"},
		{name: "same", current: "5.0.0", release: "v5.0.0", want: ErrDowngrade},
		{name: "same with a v prefix", current: "v5.0.0", release: "v5.0.0", want: ErrDowngrade},
		{name: "older", current: "5.0.0", release: "v4.2.0", want: ErrDowngrade},
		{name: "older, patch", current: "5.0.2", release: "v5.0.1", want: ErrDowngrade},
		{name: "pre-release of the running version", current: "5.0.0", release: "v5.0.0-rc.1", want: ErrDowngrade},
		{name: "dev build after the tag", current: "v5.0.0-12-gabcdef0", release: "v5.0.0", want: ErrDowngrade},
		{name: "older, explicit request", current: "5.0.0", release: "v4.2.0", requested: "4.2.0", want: ErrDowngrade},
		{name: "older, allowed", current: "5.0.0", release: "v4.2.0", requested: "4.2.0", opts: updateOptions{allowDowngrade: true}},
		{name: "same, allowed", current: "5.0.0", release: "v5.0.0", opts: updateOptions{allowDowngrade: true}},
		{name: "dev build compares with nothing", current: "dev", release: "v4.2.0"},
		{name: "no version compares with nothing", current: "", release: "v4.2.0"},
		{name: "unparsable version compares with nothing", current: "abc1234", release: "v4.2.0"},
		{name: "requested version, right release", current: "4.0.0", release: "v5.0.0", requested: "v5.0.0"},
		{name: "requested version, other release", current: "4.0.0", release: "v9.9.9", requested: "5.0.0", wantMsg: "not the requested version"},
		{name: "requested version, other release, downgrade allowed", current: "4.0.0", release: "v3.0.0", requested: "5.0.0",
			opts: updateOptions{allowDowngrade: true}, wantMsg: "not the requested version"},
		{name: "latest keyword", current: "4.0.0", release: "v5.0.0", requested: "latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkVersionChange(tt.current, rel(tt.release), tt.requested, tt.opts)
			switch {
			case tt.want == nil && tt.wantMsg == "":
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
			case tt.want != nil:
				if !errors.Is(err, tt.want) {
					t.Fatalf("got %v, want %v", err, tt.want)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("got %v, want %q", err, tt.wantMsg)
				}
			}
		})
	}
}

// S-39: Update itself refuses a release that is not newer, before any
// download, and only an explicit option lets an older one through.
func TestUpdateRefusesDowngrade(t *testing.T) {
	const releaseJSON = `{"tag_name":"v1.0.0","assets":[]}`
	f := startFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseJSON))
	})
	oldCurrent, oldLatest, oldTag := currentVersion, ohReleasesAPI, ohReleaseTagAPI
	t.Cleanup(func() { currentVersion, ohReleasesAPI, ohReleaseTagAPI = oldCurrent, oldLatest, oldTag })
	ohReleasesAPI = "https://api.github.com/repos/datichb/openhub/releases/latest"
	ohReleaseTagAPI = "https://api.github.com/repos/datichb/openhub/releases/tags/v%s"

	for _, current := range []string{"2.0.0", "1.0.0"} {
		currentVersion = func() string { return current }
		for _, version := range []string{"", "latest", "1.0.0", "v1.0.0"} {
			_, err := Update(version, nil)
			if !errors.Is(err, ErrDowngrade) {
				t.Errorf("current %s, version %q: got %v, want ErrDowngrade", current, version, err)
			}
		}
	}
	if n := f.hits("github.com"); n != 0 {
		t.Errorf("an asset was requested (%d request(s) to github.com)", n)
	}

	// Explicit option: the version check passes, and the update goes on to
	// look for its asset (absent from this release: nothing is installed).
	currentVersion = func() string { return "2.0.0" }
	_, err := Update("1.0.0", nil, AllowDowngrade())
	if err == nil || errors.Is(err, ErrDowngrade) || !strings.Contains(err.Error(), "not found in release") {
		t.Fatalf("with AllowDowngrade: got %v", err)
	}
}
