package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// opencode V2 is required (D3, revised on 06/10/2026: opencode V1 is no
// longer supported in oh v5). Every launch checks it first and refuses with
// a message that points to Doctor and the migration guide.

var (
	v5Once    sync.Once
	v5Adapter *opencodev2.Adapter
	v5Err     error
	// detectV2 finds the opencode V2 adapter (replaced by tests).
	detectV2 = detectV2Adapter
)

// requireV2 returns nil when a supported opencode V2 is installed, else an
// error to show as is (cmd.v1.unsupported.*).
func requireV2(ctx context.Context) error {
	v5Once.Do(func() {
		v5Adapter, v5Err = detectV2(ctx)
		if v5Err == nil && v5Adapter != nil {
			v5Ver.Store(v5Adapter.Ver)
		}
	})
	if v5Err == nil {
		return nil
	}
	slog.Debug("opencode V2 unavailable", "reason", v5Err)
	return v2Unsupported(v5Err)
}

// v5Available reports whether a supported opencode V2 is installed.
func v5Available(ctx context.Context) bool { return requireV2(ctx) == nil }

// v2Unsupported turns a detection error into the user message.
func v2Unsupported(err error) error {
	var ue *opencodev2.UnsupportedError
	switch {
	case errors.As(err, &ue):
		return fmt.Errorf("%s\n%s", i18n.Tf("cmd.v1.unsupported.version", ue.Found, ue.Min), i18n.T("cmd.v1.unsupported.hint"))
	case errors.Is(err, opencodev2.ErrNotInstalled):
		return fmt.Errorf("%s\n%s", i18n.T("cmd.v1.unsupported.missing"), i18n.T("cmd.v1.unsupported.hint"))
	}
	return fmt.Errorf("%s (%v)\n%s", i18n.T("cmd.v1.unsupported.unknown"), err, i18n.T("cmd.v1.unsupported.hint"))
}
