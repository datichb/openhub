package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProviderView displays provider configuration and allows credential setup.
type ProviderView struct {
	app    *tview.Application
	appCtx *app.App
	text   *tview.TextView
	shell  ShellAccess
}

var _ View = (*ProviderView)(nil)

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
	return "s setup provider · r refresh · Esc retour"
}

// Mount builds the provider status display.
func (v *ProviderView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.text = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.text.SetBackgroundColor(theme.BgPanel)
	v.text.SetBorderPadding(1, 0, 2, 2)

	v.refresh()
	content.AddItem(v.text, 0, 1, true)
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

	v.text.SetText(text)
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
	// Step 2: Auth mode
	v.shell.ShowSelectModal("Mode d'authentification Bedrock", bedrockAuthModeOptions, "", func(authMode string) {
		vip := providerConfigViper()
		vip.Set("provider.bedrock.auth_mode", authMode)

		switch authMode {
		case "bearer":
			// Step 3: Token
			v.shell.ShowPasswordModal("Bearer token Bedrock", func(token string) {
				if token == "" {
					return
				}
				// Store in keychain
				if v.appCtx.Secrets != nil {
					_ = v.appCtx.Secrets.Set(context.Background(), "bedrock-token-default", token)
				}
				// Step 4: Region
				v.shell.ShowInputModal("AWS Region", "us-east-1", func(region string) {
					if region != "" {
						vip.Set("provider.bedrock.aws_region", region)
					}
					vip.Set("opencode.default_provider", "bedrock")
					_ = vip.WriteConfigAs(config.ConfigPath())
					v.refresh()
					v.shell.ShowToastMsg("Bedrock configuré (bearer)", true)
				})
			})
		case "profile":
			// Step 3: Profile name
			v.shell.ShowInputModal("AWS Profile", "default", func(profile string) {
				if profile != "" {
					vip.Set("provider.bedrock.aws_profile", profile)
				}
				// Step 4: Region
				v.shell.ShowInputModal("AWS Region", "us-east-1", func(region string) {
					if region != "" {
						vip.Set("provider.bedrock.aws_region", region)
					}
					vip.Set("opencode.default_provider", "bedrock")
					_ = vip.WriteConfigAs(config.ConfigPath())
					v.refresh()
					v.shell.ShowToastMsg("Bedrock configuré (profile)", true)
				})
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
