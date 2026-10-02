package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// actionHistoryExport exports session history to a JSON file via the TUI.
func actionHistoryExport() {
	if tuiShell == nil {
		return
	}
	a := MustApp()

	defaultPath := fmt.Sprintf("oh-history-%s.json", time.Now().Format("20060102-150405"))

	tuiShell.ShowInputModal(i18n.T("tui.history.export_file"), defaultPath, func(outputPath string) {
		if outputPath == "" {
			return
		}

		safego.Go(func() {
			ctx := context.Background()

			sessions, err := a.Sessions.List(ctx, "")
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.T("tui.history.error_prefix")+err.Error(), shell.ToastError)
				})
				return
			}

			memberID := ""
			if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled {
				memberID = activeTeam.MemberID
			}

			data := historyExportData{
				ExportedAt: time.Now().UTC(),
				MemberID:   memberID,
				Sessions:   sessions,
			}

			encoded, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.T("tui.history.encoding_error")+err.Error(), shell.ToastError)
				})
				return
			}

			if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.T("tui.history.write_error")+err.Error(), shell.ToastError)
				})
				return
			}

			msg := fmt.Sprintf("%s %s",
				theme.IconSuccess, i18n.Tf("tui.history.sessions_exported", len(sessions), outputPath))
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		})
	})
}

// actionHistoryImport imports session history from a JSON file via the TUI.
func actionHistoryImport() {
	if tuiShell == nil {
		return
	}
	a := MustApp()

	tuiShell.ShowInputModal(i18n.T("tui.history.import_file"), "", func(filePath string) {
		if filePath == "" {
			return
		}

		safego.Go(func() {
			ctx := context.Background()

			raw, err := os.ReadFile(filePath)
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.T("tui.history.read_error")+err.Error(), shell.ToastError)
				})
				return
			}

			var data historyExportData
			if err := json.Unmarshal(raw, &data); err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(i18n.T("tui.history.invalid_file")+err.Error(), shell.ToastError)
				})
				return
			}

			// Resolve member_id: export data → current config
			memberID := data.MemberID
			if memberID == "" {
				if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled {
					memberID = activeTeam.MemberID
				}
			}

			imported := 0
			for _, s := range data.Sessions {
				if memberID != "" {
					s.MemberID = &memberID
				}
				if _, err := a.Sessions.Get(ctx, s.ID); err == nil {
					continue // already exists
				}
				if err := a.Sessions.Create(ctx, &s); err != nil {
					continue
				}
				imported++
			}

			msg := fmt.Sprintf("%s %s",
				theme.IconSuccess, i18n.Tf("tui.history.sessions_imported", imported, len(data.Sessions)-imported))
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		})
	})
}
