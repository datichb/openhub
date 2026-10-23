package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › ambient instructions (Q35): the instruction files the tool adds
// to the prompts of an oh session by itself, outside the session bundle
// (adapters.AmbientInstructions; what is and is not loaded is documented by
// the adapter).

func init() { registerDoctorCheck(ambientInstructionsDoctorChecks) }

func ambientInstructionsDoctorChecks(ctx context.Context) []doctor.Result {
	ai, ok := preferredAdapter().(adapters.AmbientInstructions)
	if !ok {
		return nil
	}
	name := i18n.T("cmd.doctor.agents_md")
	var found []string
	if a := TryApp(); a != nil && a.Projects != nil {
		if list, err := a.Projects.List(ctx, domain.ProjectStatusActive); err == nil {
			for _, p := range list {
				if files := ai.ProjectAmbientInstructions(ctx, p.Path); len(files) > 0 {
					found = append(found, p.Name+" ("+strings.Join(files, ", ")+")")
				}
			}
		}
	}
	var out []doctor.Result
	if len(found) == 0 {
		out = append(out, doctor.Result{Name: name, OK: true, Detail: i18n.T("cmd.doctor.agents_md.none")})
	} else {
		out = append(out, doctor.Result{Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("cmd.doctor.agents_md.nested", strings.Join(found, " · "), toolName())})
	}
	for _, p := range ai.UserAmbientInstructions() {
		out = append(out, doctor.Result{Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("cmd.doctor.agents_md.global", p)})
	}
	return out
}
