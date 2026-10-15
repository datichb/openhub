package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of `oh gc` (modernisation tui-launch): the clean-up view plans
// with the options of [gc] (the dry run of oh gc), then removes what the
// user confirmed, like oh gc does after its confirmation.

// actionGC is the `gc` omnibar command (and Doctor › Nettoyage › Corriger).
func actionGC() {
	if tuiShell == nil {
		return
	}
	tuiShell.PushView(views.NewGCView(gcViewConfig(MustApp())))
}

func gcViewConfig(a *app.App) views.GCViewConfig {
	opts := gcOptions(a, nil, 0)
	return views.GCViewConfig{
		Bytes: formatBytes,
		Retention: i18n.Tf("tui.gc.retention", int(opts.SessionAge.Hours()/24), int(opts.HistoryAge.Hours()/24),
			int(opts.UsageAge.Hours()/24)),
		Plan: func(ctx context.Context) (gc.Report, error) {
			env, err := gcEnv(a)
			if err != nil {
				return gc.Report{}, err
			}
			return gc.Plan(ctx, env, opts)
		},
		Run: func(ctx context.Context, only gc.Report) (gc.Report, error) {
			env, err := gcEnv(a)
			if err != nil {
				return gc.Report{}, err
			}
			done, err := gc.Run(ctx, env, opts, &only)
			if errors.Is(err, gc.ErrBusy) {
				return gc.Report{}, errors.New(i18n.T("cmd.gc.busy"))
			}
			if err != nil {
				return gc.Report{}, err
			}
			writeGCLast(gcLast{At: time.Now(), Bytes: done.Bytes(), Items: len(done.Items), Rows: done.History.Total(), Armed: true})
			return done, nil
		},
	}
}
