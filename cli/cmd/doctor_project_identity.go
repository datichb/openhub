package cmd

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitremote"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › identity of the projects in the team-state. The first claim a
// launch takes for a project records its remote in projects/<id>/project.toml
// (teamstate.ProjectIdentity). The check reports, for the active projects of
// this machine that sit in a team-state:
//
//   - an identity file that cannot be read;
//   - an identity whose remote is not the origin of the folder of the project
//     (the project was moved to another repository, or the ID is shared by two
//     repositories). Nothing is repaired: which one is right is for the person
//     to say.
//
// A project without identity is not reported: it is recorded by its next
// claim. Nothing is shown while all is well.

const doctorIDProjectIdentity = "project_identity"

func init() {
	registerDoctorCheck(projectIdentityDoctorChecks)
}

func projectIdentityDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil || a.Projects == nil {
		return nil
	}
	return projectIdentityResults(ctx, a, gitremote.Origin)
}

// projectIdentityResults runs the check over the active projects of a;
// originOf reads the origin remote of a folder.
func projectIdentityResults(ctx context.Context, a *app.App, originOf func(dir string) (string, bool)) []doctor.Result {
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil
	}
	name := i18n.T("v55ts.doctor.project_identity.name")
	var out []doctor.Result
	for i := range projects {
		p := &projects[i]
		repo, _, ok := teamClaimsRepo(a, p)
		if !ok {
			continue
		}
		store, ok := repo.(teamstate.ProjectIdentityStore)
		if !ok {
			continue
		}
		id, err := store.ReadProjectIdentity(ctx, p.ID)
		switch {
		case errors.Is(err, teamstate.ErrProjectIdentityNotFound):
			continue
		case err != nil:
			out = append(out, doctor.Result{ID: doctorIDProjectIdentity, Name: name, OK: true, Warn: true,
				Detail: i18n.Tf("v55ts.doctor.project_identity.unreadable", p.Name, err)})
			continue
		}
		origin, ok := originOf(p.Path)
		if !ok || origin == "" || id.Matches(origin) {
			continue // no remote to compare with, or the same repository
		}
		out = append(out, doctor.Result{ID: doctorIDProjectIdentity, Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("v55ts.doctor.project_identity.mismatch", p.Name, filepath.Base(repo.Path()), id.Remote, remoteLabel(origin))})
	}
	return out
}

// remoteLabel is a remote as "host/path", without the user nor a token.
func remoteLabel(remote string) string {
	if host, path, ok := gitremote.Split(remote); ok {
		return host + "/" + path
	}
	return filepath.Base(remote)
}
