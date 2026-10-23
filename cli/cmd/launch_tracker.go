package cmd

import (
	"context"
	"net/url"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/launch"
)

// launchTracker says where the tickets of a project live, for the references
// of a launch (#623 is a GitLab issue only where the tickets are on GitLab):
// the tracker declared for the project or its team, else the forge of its
// origin remote when that is a GitLab (the project path is then the path of
// the remote).
func launchTracker(ctx context.Context, a *app.App, project *domain.Project) launch.TrackerInfo {
	svc := teamService(a)
	kind := svc.TrackerOf(project)
	info := launch.TrackerInfo{Type: kind.Type, Project: kind.Project}
	if info.Type != "" && (info.Type != "gitlab" || info.Project != "") {
		return info
	}
	known := ""
	if u, err := url.Parse(gitlabURLWith(svc, project)); err == nil {
		known = u.Hostname()
	}
	if path, ok := gitlabRemotePath(ctx, project.Path, known); ok {
		info.Type = "gitlab"
		if info.Project == "" {
			info.Project = path
		}
	}
	return info
}

// gitlabRemotePath returns the project path of the origin remote of the
// repository at dir when its host is a GitLab: the host of the GitLab the hub
// is set for (known), or a host named gitlab (gitlab.com, gitlab.example.com).
func gitlabRemotePath(ctx context.Context, dir, known string) (string, bool) {
	if dir == "" {
		return "", false
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", false
	}
	gr, err := config.ParseGitRemote(string(out))
	if err != nil {
		return "", false
	}
	host := gr.Host
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	if host == "" || strings.HasPrefix(host, "github.") {
		return "", false
	}
	if host == strings.ToLower(known) || strings.Contains(host, "gitlab") {
		return gr.Path, true
	}
	return "", false
}
