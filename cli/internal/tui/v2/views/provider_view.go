package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProviderView displays provider configuration and allows credential setup.
type ProviderView struct {
	app      *tview.Application
	appCtx   *app.App
	text     *tview.TextView
	shell    ShellAccess
	commands []ContextCommand
}

var _ View = (*ProviderView)(nil)
var _ CommandProvider = (*ProviderView)(nil)

// NewProviderView creates a new provider configuration view.
func NewProviderView(a *app.App) *ProviderView {
	return &ProviderView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *ProviderView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ProviderView) ID() string { return "provider" }

// Title returns the display title.
func (v *ProviderView) Title() string { return "Provider" }

// StatusHints returns keybinding hints.
func (v *ProviderView) StatusHints() string {
	return fmt.Sprintf("s %s · r %s · Ctrl+P %s",
		i18n.T("tui.hints.setup"),
		i18n.T("tui.hints.refresh"),
		i18n.T("tui.hints.commands"),
	)
}

// Mount builds the provider status display.
func (v *ProviderView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.text = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.text.SetBackgroundColor(theme.BgPanel)
	v.text.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.text.SetText(fmt.Sprintf("\n  %sDétection des providers...%s", muted, theme.TagColor))
	v.buildCommands()
	content.AddItem(v.text, 0, 1, true)

	// Load provider data asynchronously
	go func() {
		text := v.buildRefreshText()
		app.QueueUpdateDraw(func() {
			if v.text == nil {
				return
			}
			v.text.SetText(text)
		})
	}()
}

// Unmount cleans up resources.
func (v *ProviderView) Unmount() {
	v.app = nil
	v.text = nil
}

// HandleKey processes provider view key events.
func (v *ProviderView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 's':
		v.setupProvider()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	return event
}

func (v *ProviderView) refresh() {
	if v.text == nil {
		return
	}
	v.text.SetText(v.buildRefreshText())
}

// buildRefreshText builds the provider status text. Safe to call from any goroutine
// because it only reads config and calls stateless provider detection functions.
func (v *ProviderView) buildRefreshText() string {
	var text string
	text += fmt.Sprintf("  %s%s%s\n\n",
		theme.ColorTag(theme.AccentHex), "Configuration Provider", theme.TagColor)

	// Current default
	defaultProv := v.appCtx.Config.Opencode.DefaultProvider
	if defaultProv == "" {
		defaultProv = "(non défini)"
	}
	text += fmt.Sprintf("  %sDéfaut:%s  %s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, defaultProv)

	// Detection status per provider
	text += fmt.Sprintf("  %s%s%s\n",
		theme.ColorTag(theme.TextSecondaryHex), "Détection des credentials:", theme.TagColor)

	for _, name := range provider.AllProviders() {
		result := provider.Detect(name)
		var icon, detail string
		if result.Available {
			icon = "[green]✓[-]"
			detail = result.Source
			if result.Details != "" {
				detail += " (" + result.Details + ")"
			}
		} else {
			icon = "[red]✗[-]"
			detail = "non détecté"
		}

		// Also check keychain
		keychainOk := false
		if v.appCtx.Secrets != nil {
			key := provider.KeychainKey(name, "")
			if key != "" {
				val, err := v.appCtx.Secrets.Get(context.Background(), key)
				if err == nil && val != "" {
					keychainOk = true
				}
			}
		}
		keychainIcon := ""
		if keychainOk {
			keychainIcon = " [green]🔑[-]"
		}

		text += fmt.Sprintf("    %s %-16s %s%s\n", icon, string(name), detail, keychainIcon)
	}

	text += fmt.Sprintf("\n  %sAppuyez 's' pour configurer un provider%s\n",
		theme.ColorTag(theme.TextMutedHex), theme.TagColor)

	return text
}

// Provider options for setup
var providerSetupOptions = []SelectOption{
	{Label: "Amazon Bedrock", Value: "bedrock"},
	{Label: "Anthropic (API directe)", Value: "anthropic"},
	{Label: "OpenRouter", Value: "openrouter"},
	{Label: "GitHub Copilot", Value: "github-copilot"},
}

var bedrockAuthModeOptions = []SelectOption{
	{Label: "Bearer Token (SSO/STS)", Value: "bearer"},
	{Label: "AWS Profile", Value: "profile"},
	{Label: "Variables d'env", Value: "env"},
}

func (v *ProviderView) setupProvider() {
	if v.shell == nil {
		return
	}

	// Step 1: Choose provider
	v.shell.ShowSelectModal("Provider à configurer", providerSetupOptions, "", func(prov string) {
		switch prov {
		case "bedrock":
			v.setupBedrock()
		case "anthropic":
			v.setupAPIKey(provider.Anthropic, "Clé API Anthropic")
		case "openrouter":
			v.setupAPIKey(provider.OpenRouter, "Clé API OpenRouter")
		case "github-copilot":
			if v.shell != nil {
				v.shell.ShowToastMsg("GitHub Copilot utilise 'gh auth' — pas de config nécessaire", true)
			}
		}
	})
}

func (v *ProviderView) setupBedrock() {
	// Step 1: Auth mode selection (kept as a select modal — it determines which fields to show)
	v.shell.ShowSelectModal("Mode d'authentification Bedrock", bedrockAuthModeOptions, "", func(authMode string) {
		vip := providerConfigViper()
		vip.Set("provider.bedrock.auth_mode", authMode)

		switch authMode {
		case "bearer":
			// Step 2: Inline form with token + region
			v.shell.ShowInlineForm(InlineFormConfig{
				Title: "Configuration Bedrock (Bearer)",
				Fields: []FormField{
					{Label: "Bearer token", Key: "token", Type: FieldPassword, Required: true},
					{Label: "AWS Region", Key: "region", Type: FieldText, Default: "us-east-1"},
				},
				OnSubmit: func(values map[string]string, _ map[string][]string) {
					token := values["token"]
					if token == "" {
						return
					}
					// Store in keychain
					if v.appCtx.Secrets != nil {
						_ = v.appCtx.Secrets.Set(context.Background(), "bedrock-token-default", token)
					}
					if region := values["region"]; region != "" {
						vip.Set("provider.bedrock.aws_region", region)
					}
					vip.Set("opencode.default_provider", "bedrock")
					_ = vip.WriteConfigAs(config.ConfigPath())
					v.refresh()
					v.shell.ShowToastMsg("Bedrock configuré (bearer)", true)
				},
			})
		case "profile":
			// Step 2: Inline form with profile + region
			v.shell.ShowInlineForm(InlineFormConfig{
				Title: "Configuration Bedrock (Profile)",
				Fields: []FormField{
					{Label: "AWS Profile", Key: "profile", Type: FieldText, Default: "default"},
					{Label: "AWS Region", Key: "region", Type: FieldText, Default: "us-east-1"},
				},
				OnSubmit: func(values map[string]string, _ map[string][]string) {
					if profile := values["profile"]; profile != "" {
						vip.Set("provider.bedrock.aws_profile", profile)
					}
					if region := values["region"]; region != "" {
						vip.Set("provider.bedrock.aws_region", region)
					}
					vip.Set("opencode.default_provider", "bedrock")
					_ = vip.WriteConfigAs(config.ConfigPath())
					v.refresh()
					v.shell.ShowToastMsg("Bedrock configuré (profile)", true)
				},
			})
		case "env":
			vip.Set("opencode.default_provider", "bedrock")
			_ = vip.WriteConfigAs(config.ConfigPath())
			v.refresh()
			v.shell.ShowToastMsg("Bedrock configuré (env vars)", true)
		}
	})
}

func (v *ProviderView) setupAPIKey(name provider.Name, title string) {
	v.shell.ShowPasswordModal(title, func(key string) {
		if key == "" {
			return
		}
		// Store in keychain
		keychainKey := provider.KeychainKey(name, "")
		if v.appCtx.Secrets != nil && keychainKey != "" {
			_ = v.appCtx.Secrets.Set(context.Background(), keychainKey, key)
		}
		// Set as default provider
		vip := providerConfigViper()
		vip.Set("opencode.default_provider", string(name))
		_ = vip.WriteConfigAs(config.ConfigPath())
		v.refresh()
		v.shell.ShowToastMsg(string(name)+" configuré", true)
	})
}

func providerConfigViper() *viper.Viper {
	return hubViper()
}

// ContextCommands returns contextual commands for the omnibar.
func (v *ProviderView) ContextCommands() []ContextCommand {
	return v.commands
}

func (v *ProviderView) buildCommands() {
	v.commands = []ContextCommand{
		{
			ID:          "provider.setup",
			Label:       "setup",
			Aliases:     []string{"configurer", "configure"},
			Description: "Configurer un provider",
			Category:    "Provider",
			Action:      v.setupProvider,
		},
		{
			ID:          "provider.refresh",
			Label:       "refresh",
			Aliases:     []string{"rafraîchir", "reload"},
			Description: "Rafraîchir la détection",
			Category:    "Provider",
			Action:      v.refresh,
		},
		{
			ID:          "provider.setup.bedrock",
			Label:       "setup bedrock",
			Aliases:     []string{"bedrock", "aws"},
			Description: "Configurer Amazon Bedrock",
			Category:    "Provider",
			Action: func() {
				v.setupSpecificProvider("bedrock")
			},
		},
		{
			ID:          "provider.setup.anthropic",
			Label:       "setup anthropic",
			Aliases:     []string{"anthropic", "claude"},
			Description: "Configurer Anthropic",
			Category:    "Provider",
			Action: func() {
				v.setupSpecificProvider("anthropic")
			},
		},
		{
			ID:          "provider.setup.openrouter",
			Label:       "setup openrouter",
			Aliases:     []string{"openrouter", "or"},
			Description: "Configurer OpenRouter",
			Category:    "Provider",
			Action: func() {
				v.setupSpecificProvider("openrouter")
			},
		},
	}
}

func (v *ProviderView) setupSpecificProvider(name string) {
	if v.shell == nil {
		return
	}
	v.shell.ShowPasswordModal("API Key "+name, func(key string) {
		if key == "" {
			return
		}
		// Store in keychain
		if v.appCtx.Secrets != nil {
			keychainKey := provider.KeychainKey(provider.Name(name), "")
			if keychainKey != "" {
				_ = v.appCtx.Secrets.Set(context.Background(), keychainKey, key)
			}
		}
		// Set as default
		vip := providerConfigViper()
		vip.Set("opencode.default_provider", name)
		_ = vip.WriteConfigAs(config.ConfigPath())
		v.refresh()
		v.shell.ShowToastMsg(name+" configuré", true)
	})
}
