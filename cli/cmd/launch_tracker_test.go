package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/launch"
)

// Bug 20: #623 is read as a GitLab issue where the tickets are on GitLab. With
// no tracker declared, the forge of the origin remote tells.
func TestLaunchTrackerFollowsTheOriginRemote(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	t.Setenv("OH_GITLAB_URL", "")
	for _, c := range []struct {
		name, origin string
		want         launch.TrackerInfo
	}{
		{"gitlab.com https", "https://gitlab.com/acme/web.git", launch.TrackerInfo{Type: "gitlab", Project: "acme/web"}},
		{"gitlab.com ssh", "git@gitlab.com:acme/sub/web.git", launch.TrackerInfo{Type: "gitlab", Project: "acme/sub/web"}},
		{"self-hosted gitlab", "git@gitlab.corp.example:acme/web.git", launch.TrackerInfo{Type: "gitlab", Project: "acme/web"}},
		{"github", "https://github.com/acme/web.git", launch.TrackerInfo{}},
		{"unknown forge", "git@git.corp.example:acme/web.git", launch.TrackerInfo{}},
		{"no origin", "", launch.TrackerInfo{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := &app.App{Config: &config.Config{}}
			p := &domain.Project{ID: "p1", Path: gitRepoWithOrigin(t, c.origin)}
			assert.Equal(t, c.want, launchTracker(t.Context(), a, p))
		})
	}
}

func TestLaunchTrackerKnowsTheGitLabOfTheHub(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://git.corp.example")
	a := &app.App{Config: &config.Config{}}
	p := &domain.Project{ID: "p1", Path: gitRepoWithOrigin(t, "git@git.corp.example:acme/web.git")}
	assert.Equal(t, launch.TrackerInfo{Type: "gitlab", Project: "acme/web"}, launchTracker(t.Context(), a, p))
}

func TestLaunchTrackerKeepsTheDeclaredTracker(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	a := &app.App{Config: &config.Config{}}
	p := &domain.Project{ID: "p1", Path: gitRepoWithOrigin(t, "https://gitlab.com/acme/web.git"),
		TrackerConfig: &domain.ProjectTrackerConfig{TrackerProject: "tickets/board"}}
	// No team: the declared project only counts with a tracker type of the
	// team; the remote tells the type, the declared project is kept.
	got := launchTracker(t.Context(), a, p)
	assert.Equal(t, "gitlab", got.Type)
	assert.Equal(t, "tickets/board", got.Project)
}
