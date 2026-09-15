package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

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

	tuiShell.ShowInputModal("Fichier de sortie", defaultPath, func(outputPath string) {
		if outputPath == "" {
			return
		}

		go func() {
			ctx := context.Background()

			sessions, err := a.Sessions.List(ctx, "")
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Erreur : "+err.Error(), shell.ToastError)
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
					tuiShell.ShowToast("Erreur d'encodage : "+err.Error(), shell.ToastError)
				})
				return
			}

			if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Erreur d'écriture : "+err.Error(), shell.ToastError)
				})
				return
			}

			msg := fmt.Sprintf("%s %d sessions exportées → %s",
				theme.IconSuccess, len(sessions), outputPath)
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		}()
	})
}

// actionHistoryImport imports session history from a JSON file via the TUI.
func actionHistoryImport() {
	if tuiShell == nil {
		return
	}
	a := MustApp()

	tuiShell.ShowInputModal("Fichier à importer", "", func(filePath string) {
		if filePath == "" {
			return
		}

		go func() {
			ctx := context.Background()

			raw, err := os.ReadFile(filePath)
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Erreur de lecture : "+err.Error(), shell.ToastError)
				})
				return
			}

			var data historyExportData
			if err := json.Unmarshal(raw, &data); err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Fichier invalide : "+err.Error(), shell.ToastError)
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

			msg := fmt.Sprintf("%s %d sessions importées (%d ignorées)",
				theme.IconSuccess, imported, len(data.Sessions)-imported)
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		}()
	})
}
