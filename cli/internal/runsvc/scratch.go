package runsvc

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionctx"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// setScratchContext creates the session scratch folders of the workflow
// (SCR, ~/.oh/sessions/<session>/scratch/<agent>, 0700) and gives their
// path to the agents (session context entry oh.scratch; a synthetic message
// without the capability). Who may read or write them is decided by the
// guards (bundle.ScratchAgentGuard, bundle.ScratchSessionRules).
func (s *Service) setScratchContext(ctx context.Context, h adapters.ServerHandle, sessionID string, wf *sessionspec.WorkflowRuntime) {
	if wf == nil || len(wf.Scratch) == 0 || s.SessionsDir == "" {
		return
	}
	dirs := map[string]string{}
	var lines []string
	for _, sc := range wf.Scratch {
		dir := bundle.ScratchDir(s.SessionsDir, sessionID, sc.Owner)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			slog.Warn("runsvc: scratch folder not created", "session", sessionID, "agent", sc.Owner, "error", err)
			continue
		}
		dirs[sc.Owner] = dir
		readers := "—"
		if len(sc.Readers) > 0 {
			readers = strings.Join(sc.Readers, ", ")
		}
		lines = append(lines, i18n.Tf("cmd.session.scratch.folder", sc.Owner, dir, readers))
	}
	if len(dirs) == 0 {
		return
	}
	text := i18n.T("cmd.session.scratch.context") + "\n" + strings.Join(lines, "\n")
	w := &sessionctx.Writer{Adapter: s.Adapter, Dir: s.SessionsDir}
	if _, err := w.Set(ctx, h, sessionID, sessionctx.Entry{Key: sessionctx.KeyScratch,
		Value: map[string]any{"text": text, "dirs": dirs}, Fallback: text}); err != nil {
		slog.Warn("runsvc: scratch folders not given", "session", sessionID, "error", err)
	}
}
