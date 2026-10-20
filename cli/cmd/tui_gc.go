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

// TUI wiring of the clean-up view (modernisation tui-launch, merged by
// tui-docs): « Données d'oh » plans with the options of [gc] (the dry run
// of oh gc), then removes what the user confirmed, like oh gc does after
// its confirmation; « Anciens déploiements » is oh migrate deploy-cleanup
// (tui_cleanup.go).

// actionCleanup is the `cleanup` omnibar command (and Doctor › Nettoyage ›
// Corriger): the clean-up view on « Données d'oh ».
func actionCleanup() { openCleanup(views.CleanupData) }

// actionCleanupDeploy is Doctor › Anciens déploiements › Corriger.
func actionCleanupDeploy() { openCleanup(views.CleanupDeploy) }

func openCleanup(section views.CleanupSection) {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	cfg := gcViewConfig(a)
	cfg.Deploy = deployCleanupConfig(a)
	cfg.Section = section
	tuiShell.PushView(views.NewGCView(cfg))
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
