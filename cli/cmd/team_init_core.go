package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
)

// teamInitParams holds the parameters for the core team initialization logic
// (see teamInitCore).
type teamInitParams struct {
	StateRepo          string
	StatePath          string // empty = use default
	MemberID           string
	DisplayName        string
	GitLabUsername     string
	MattermostUsername string
	TrackerUsername    string // name of the member on the tracker (empty = keep)
	Role               string
	Claim              bool   // « C'est moi »: attach the card that has this ID (see teamsvc.InitParams)
	GitLabURL          string // GitLab of the repository when its remote is an SSH alias
}

// teamInitCore performs the git+member+config operations of the team step of
// the `oh init` wizard (teamInitParams come from its state). It does NOT
// display any UI — the caller handles presentation. The work is done by the
// team service (Service.Init): clone/pull repo → init structure → add/update
// member → write hub.toml (the team is added, or replaces the entry of the
// same team; the other teams, solo spaces included, are left as they are).
// warning is what the caller shows without stopping: the commit of the
// structure of the team-state failed (empty otherwise).
func teamInitCore(ctx context.Context, a *app.App, p teamInitParams) (warning string, err error) {
	res, err := teamService(a).Init(ctx, teamsvc.InitParams(p))
	if err == nil {
		return structureWarningText(res.StructureWarning), nil
	}
	return "", teamInitError(err)
}

// teamInitError words the failure of Service.Init by the step that failed.
func teamInitError(err error) error {
	var initErr *teamsvc.InitError
	if !errors.As(err, &initErr) {
		return err
	}
	switch initErr.Step {
	case teamsvc.StepJoinMember:
		return joinRefusalError(initErr.Err, "v532m.join.id_taken_hint")
	case teamsvc.StepClone:
		return fmt.Errorf(i18n.T("cmd.team_init_core.cloning_team_state"), initErr.Err)
	case teamsvc.StepStructure:
		return fmt.Errorf(i18n.T("cmd.team_init_core.init_structure"), initErr.Err)
	case teamsvc.StepUpdateMember:
		return fmt.Errorf(i18n.T("cmd.team_init_core.update_member"), initErr.Err)
	case teamsvc.StepAddMember:
		return fmt.Errorf(i18n.T("cmd.team_init_core.add_member"), initErr.Err)
	default:
		return fmt.Errorf(i18n.T("cmd.team_init_core.writing_hub_toml"), initErr.Err)
	}
}
