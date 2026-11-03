package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// A supported release of the tool is required (D3, revised on 06/10/2026:
// the V1 of the tool is no longer supported in oh v5). Every launch checks it
// first and refuses with a message that points to Doctor and the migration
// guide.

var (
	v5Once    sync.Once
	v5Adapter adapters.ToolAdapter
	v5Tool    adapters.ToolInfo
	v5Err     error
	// detectV2 finds the adapter of the installed tool (replaced by tests).
	detectV2 = detectTool
)

// requireV2 returns nil when a supported tool is installed, else an error
// to show as is (cmd.v1.unsupported.*).
func requireV2(ctx context.Context) error {
	v5Once.Do(func() {
		v5Adapter, v5Tool, v5Err = detectV2(ctx)
		if v5Err == nil && v5Adapter != nil {
			v5Ver.Store(v5Tool.Version)
		}
	})
	if v5Err == nil {
		return nil
	}
	slog.Debug("tool unavailable", "reason", v5Err)
	return v2Unsupported(v5Err)
}

// v5Available reports whether a supported tool is installed.
func v5Available(ctx context.Context) bool { return requireV2(ctx) == nil }

// toolName is the displayed name of the tool (of the preferred adapter
// when none is installed).
func toolName() string {
	if v5Tool.DisplayName == "" {
		_ = requireV2(context.Background())
	}
	if v5Tool.DisplayName != "" {
		return v5Tool.DisplayName
	}
	return "tool"
}

// v2Unsupported turns a detection error into the user message.
func v2Unsupported(err error) error {
	var ue *adapters.UnsupportedVersionError
	name := toolName()
	switch {
	case errors.As(err, &ue):
		return fmt.Errorf("%s\n%s", i18n.Tf("cmd.v1.unsupported.version", name, ue.Found, ue.Min), i18n.Tf("cmd.v1.unsupported.hint", name))
	case errors.Is(err, adapters.ErrToolNotInstalled):
		return fmt.Errorf("%s\n%s", i18n.Tf("cmd.v1.unsupported.missing", name), i18n.Tf("cmd.v1.unsupported.hint", name))
	}
	return fmt.Errorf("%s (%v)\n%s", i18n.Tf("cmd.v1.unsupported.unknown", name), err, i18n.Tf("cmd.v1.unsupported.hint", name))
}
