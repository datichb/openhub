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

// A supported release of the tool is required (the V1 of the tool is no longer
// supported in oh v5). Every launch checks it first and refuses with a message
// that points to Doctor and the migration guide.

var (
	toolOnce    sync.Once
	toolAdapter adapters.ToolAdapter
	toolInfo    adapters.ToolInfo
	toolErr     error
	// detectSupportedTool finds the adapter of the installed tool (replaced by tests).
	detectSupportedTool = detectTool
)

// requireSupportedTool returns nil when a supported tool is installed, else an error
// to show as is (cmd.v1.unsupported.*).
func requireSupportedTool(ctx context.Context) error {
	toolOnce.Do(func() {
		toolAdapter, toolInfo, toolErr = detectSupportedTool(ctx)
		if toolErr == nil && toolAdapter != nil {
			detectedVersion.Store(toolInfo.Version)
		}
	})
	if toolErr == nil {
		return nil
	}
	slog.Debug("tool unavailable", "reason", toolErr)
	return unsupportedToolError(toolErr)
}

// toolAvailable reports whether a supported tool is installed.
func toolAvailable(ctx context.Context) bool { return requireSupportedTool(ctx) == nil }

// toolName is the displayed name of the tool (of the preferred adapter
// when none is installed).
func toolName() string {
	if toolInfo.DisplayName == "" {
		_ = requireSupportedTool(context.Background())
	}
	if toolInfo.DisplayName != "" {
		return toolInfo.DisplayName
	}
	return "tool"
}

// unsupportedToolError turns a detection error into the user message.
func unsupportedToolError(err error) error {
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
