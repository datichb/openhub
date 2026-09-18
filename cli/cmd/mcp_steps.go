package cmd

import (
	"context"
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Shared MCP token step builders
// ─────────────────────────────────────────────────────────────────────────────

// mcpTokenStepOpts configures a reusable MCP token wizard step.
type mcpTokenStepOpts struct {
	LabelI18nKey string            // e.g. "cmd.init.wizard_step_mcp_figma"
	DisplayName  string            // e.g. "Figma", "GitLab", "Google Slides"
	TokenKey     string            // e.g. config.DefaultFigmaTokenKey
	HintI18nKey  string            // e.g. "cmd.init.mcp_hint_figma" (optional)
	TokenVar     *string           // pointer to the caller's token variable
	SkipIf       func() bool       // caller-specific skip logic
	Secrets      domain.SecretStore // may be nil
	// AfterStore is called after the token is stored in keychain.
	// Use it to enable the service in config.
	AfterStore func() error
}

// buildMCPTokenStep returns a WizardStep that collects a token for an MCP
// service, stores it in the keychain, and optionally runs an AfterStore hook.
// The step pre-fills the token from keychain if already stored.
func buildMCPTokenStep(opts mcpTokenStepOpts) views.WizardStep {
	return views.WizardStep{
		Label:  i18n.T(opts.LabelI18nKey),
		SkipIf: opts.SkipIf,
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			// Pre-fill from keychain if available
			if *opts.TokenVar == "" && opts.Secrets != nil {
				if existing, err := opts.Secrets.Get(context.Background(), opts.TokenKey); err == nil && existing != "" {
					*opts.TokenVar = existing
				}
			}
			form := tview.NewForm()
			form.AddPasswordField(
				i18n.Tf("cmd.init.mcp_token_prompt", opts.DisplayName),
				*opts.TokenVar, 50, '*',
				func(t string) { *opts.TokenVar = t },
			)
			if opts.HintI18nKey != "" {
				form.AddTextView("", i18n.T(opts.HintI18nKey), 60, 2, true, false)
			}
			form.AddButton(i18n.T("wizard.hint.submit"), onDone)
			return form
		},
		OnDone: func() error {
			if *opts.TokenVar == "" {
				return nil
			}
			if opts.Secrets != nil {
				if err := opts.Secrets.Set(context.Background(), opts.TokenKey, *opts.TokenVar); err != nil {
					return fmt.Errorf("keychain: %w", err)
				}
			}
			if opts.AfterStore != nil {
				return opts.AfterStore()
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			v := i18n.T("cmd.init.wizard_mcp_configured")
			if *opts.TokenVar == "" {
				v = i18n.T("cmd.init.wizard_mcp_skipped")
			}
			return []views.InfoField{{Label: opts.DisplayName, Value: v}}
		},
	}
}
