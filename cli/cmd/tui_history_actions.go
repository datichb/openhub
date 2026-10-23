package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
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
			n, err := sessionsvc.ExportHistory(context.Background(), a.Sessions, outputPath, sessionsvc.ExportOptions{MemberID: activeMemberID(a)})
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(historyToastPrefix(err)+err.Error(), shell.ToastError)
				})
				return
			}

			msg := fmt.Sprintf("%s %s",
				theme.IconSuccess, i18n.Tf("tui.history.sessions_exported", n, outputPath))
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		})
	})
}

// historyToastPrefix is the beginning of the error toast of an export or an
// import, by the step that failed.
func historyToastPrefix(err error) string {
	var he *sessionsvc.HistoryError
	if !errors.As(err, &he) {
		return i18n.T("tui.history.error_prefix")
	}
	switch he.Stage {
	case sessionsvc.HistoryEncode:
		return i18n.T("tui.history.encoding_error")
	case sessionsvc.HistoryWrite:
		return i18n.T("tui.history.write_error")
	case sessionsvc.HistoryRead:
		return i18n.T("tui.history.read_error")
	case sessionsvc.HistoryParse:
		return i18n.T("tui.history.invalid_file")
	}
	return i18n.T("tui.history.error_prefix")
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
			res, err := sessionsvc.ImportHistoryFile(context.Background(), a.Sessions, filePath, sessionsvc.ImportOptions{DefaultMemberID: activeMemberID(a)})
			if err != nil {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(historyToastPrefix(err)+err.Error(), shell.ToastError)
				})
				return
			}

			msg := fmt.Sprintf("%s %s",
				theme.IconSuccess, i18n.Tf("tui.history.sessions_imported", res.Imported, res.Skipped))
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(msg, shell.ToastSuccess)
			})
		})
	})
}
