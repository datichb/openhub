package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// limitsDoctorChecks reports the hub session restrictions (I6) and the
// memory the session servers use, with an estimate per server for sizing
// the memory cap.
func limitsDoctorChecks(ctx context.Context) []views.DoctorCheck {
	var set []string
	hub := TryApp()
	var l limits.Limits
	if hub != nil {
		l = hub.Config.Limits
	}
	for _, f := range limits.Fields {
		if v := l.Value(f); v != "" {
			set = append(set, i18n.T("cmd.budget.field."+f)+" "+v)
		}
	}
	detail := i18n.T("cmd.budget.doctor.off")
	if len(set) > 0 {
		detail = strings.Join(set, " · ")
	}
	out := []views.DoctorCheck{{Name: i18n.T("cmd.budget.doctor.name"), OK: true, Detail: detail}}

	h, err := daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).Health(ctx)
	if err != nil || h.Servers == 0 {
		return out
	}
	mem := i18n.Tf("cmd.budget.doctor.memory", h.MemoryMB, h.Servers, h.MemoryMB/h.Servers)
	ok := true
	if l.MemoryMB > 0 {
		mem += " · " + i18n.Tf("cmd.budget.doctor.memory_cap", l.MemoryMB)
		if h.MemoryMB > l.MemoryMB {
			ok = false
			mem += " · " + i18n.T("cmd.budget.doctor.memory_over")
		}
	}
	return append(out, views.DoctorCheck{Name: i18n.T("cmd.budget.doctor.memory_name"), OK: ok, Detail: mem})
}
