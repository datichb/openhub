package cmd

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitremote"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The identity of a project in the team-state (projects/<id>/project.toml,
// teamstate.ProjectIdentity) is recorded in the commit of the first claim the
// launch takes for the project: the launch hook builds it from the origin of
// the project and carries it in the context to the grouped write.

type projectIdentityKey struct{}

// localProjectIdentity is the identity of project from the origin remote of
// its folder (read in .git/config, no git process). ok is false for a project
// without folder, without origin, or whose origin is a local path.
func localProjectIdentity(project *domain.Project, member string, now time.Time) (teamstate.ProjectIdentity, bool) {
	if project == nil || project.ID == "" || project.Path == "" {
		return teamstate.ProjectIdentity{}, false
	}
	origin, ok := gitremote.Origin(project.Path)
	if !ok || origin == "" {
		return teamstate.ProjectIdentity{}, false
	}
	id, err := teamstate.NewProjectIdentity(project.ID, project.Name, origin, member, now)
	return id, err == nil
}

// withProjectIdentity carries the identity of project to startTicketClaim.
func withProjectIdentity(ctx context.Context, project *domain.Project, member string) context.Context {
	id, ok := localProjectIdentity(project, member, time.Now())
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, projectIdentityKey{}, &id)
}

// projectIdentityFrom is the identity withProjectIdentity carried (nil: none).
func projectIdentityFrom(ctx context.Context) *teamstate.ProjectIdentity {
	id, _ := ctx.Value(projectIdentityKey{}).(*teamstate.ProjectIdentity)
	return id
}
