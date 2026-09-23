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
	ID           string            // optional step ID for RefreshLabels lookup
	LabelI18nKey string            // e.g. "cmd.init.wizard_step_mcp_figma"
	DisplayName  string            // e.g. "Figma", "GitLab", "Google Slides"
	TokenKey     string            // e.g. config.DefaultFigmaTokenKey
	HintI18nKey  string            // e.g. "cmd.init.mcp_hint_figma" (optional)
	TokenVar     *string           // pointer to the caller's token variable
	SkipIf       func() bool       // caller-specific skip logic
	Secrets      domain.SecretStore // may be nil; use SecretsFunc when not available at construction
	SecretsFunc  func() domain.SecretStore // lazy alternative to Secrets (for init.go where app is nil at build time)
	// AfterStore is called after the token is stored in keychain.
	// Use it to enable the service in config.
	AfterStore func() error

	// Optional checkbox (e.g. GitLab write mode toggle).
	// When CheckboxVar is non-nil, a checkbox is rendered after the hint.
	// CheckboxLabel should be SHORT (~12 chars max) to avoid polluting
	// tview's maxLabelWidth calculation, which would crush the password field.
	CheckboxLabel   string // short label for the checkbox (e.g. "Write access")
	CheckboxDescKey string // i18n key for description text shown above the checkbox
	CheckboxVar     *bool  // pointer to the bool that stores the checkbox state

	// ExtraInfoFields returns additional InfoField entries appended after the
	// standard "Configured"/"Skipped" field. Used by GitLab to show write mode status.
	ExtraInfoFields func() []views.InfoField
}

// resolveSecrets returns the secret store from opts, preferring SecretsFunc for lazy resolution.
func (o mcpTokenStepOpts) resolveSecrets() domain.SecretStore {
	if o.SecretsFunc != nil {
		return o.SecretsFunc()
	}
	return o.Secrets
}

// buildMCPTokenStep returns a WizardStep that collects a token for an MCP
// service, stores it in the keychain, and optionally runs an AfterStore hook.
// When a token already exists in the keychain, the field is left empty and a
// hint is shown ("leave empty to keep existing").
func buildMCPTokenStep(opts mcpTokenStepOpts) views.WizardStep {
	return views.WizardStep{
		ID:     opts.ID,
		Label:  i18n.T(opts.LabelI18nKey),
		SkipIf: opts.SkipIf,
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			// Check keychain for existing token (don't pre-fill — show hint instead)
			hasKeychainToken := false
			if secrets := opts.resolveSecrets(); *opts.TokenVar == "" && secrets != nil {
				if existing, err := secrets.Get(context.Background(), opts.TokenKey); err == nil && existing != "" {
					hasKeychainToken = true
				}
			}
			form := tview.NewForm()
			if hasKeychainToken {
				form.AddTextView("", i18n.T("cmd.init.wizard_keychain_hint"), 60, 2, true, false)
			}
			form.AddPasswordField(
				i18n.Tf("cmd.init.mcp_token_prompt", opts.DisplayName),
				*opts.TokenVar, 0, '*',
				func(t string) { *opts.TokenVar = t },
			)
			if opts.HintI18nKey != "" {
				form.AddTextView("", i18n.T(opts.HintI18nKey), 60, 2, true, false)
			}
			// Optional checkbox (e.g. GitLab write mode)
			if opts.CheckboxVar != nil {
				if opts.CheckboxDescKey != "" {
					form.AddTextView("", i18n.T(opts.CheckboxDescKey), 60, 2, true, false)
				}
				form.AddCheckbox(opts.CheckboxLabel, *opts.CheckboxVar, func(checked bool) {
					*opts.CheckboxVar = checked
				})
			}
			form.AddButton(i18n.T("wizard.hint.submit"), onDone)
			return form
		},
		OnDone: func() error {
			if *opts.TokenVar == "" {
				return nil
			}
			if secrets := opts.resolveSecrets(); secrets != nil {
				if err := secrets.Set(context.Background(), opts.TokenKey, *opts.TokenVar); err != nil {
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
			fields := []views.InfoField{{Label: opts.DisplayName, Value: v}}
			if opts.ExtraInfoFields != nil {
				fields = append(fields, opts.ExtraInfoFields()...)
			}
			return fields
		},
	}
}
