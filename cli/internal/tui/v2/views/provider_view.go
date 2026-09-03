package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProviderViewConfig holds external dependencies for the provider config view.
type ProviderViewConfig struct {
	// GetConfig returns the live hub config pointer.
	GetConfig func() *config.Config
	// SaveConfig persists the modified config to hub.toml (unified mutation path).
	SaveConfig func(c *config.Config) error
	// CheckSecret tests whether a keychain key has a value stored.
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a new secret value in the keychain.
	SetSecret func(ctx context.Context, key, value string) error
	// DeleteSecret removes a secret from the keychain.
	DeleteSecret func(ctx context.Context, key string) error
}

// providerConfigLine represents a single editable line in the provider config view.
type providerConfigLine struct {
	provider string // "bedrock", "anthropic", etc.
	key      string // "auth_mode", "aws_region", etc.
	kind     string // "select", "string", "password", "info", "section-header", "action"
	options  []SelectOption
	visible  func() bool
	get      func(c *config.Config) string
	set      func(c *config.Config, val string)
}

// ProviderView displays provider configuration and allows credential setup.
type ProviderView struct {
	app    *tview.Application
	appCtx *app.App
	vcfg   ProviderViewConfig
	shell  ShellAccess

	list     *widgets.SectionedList
	commands []ContextCommand
	mountGen uint64

	lines            []providerConfigLine
	selectedProvider string // currently selected provider for detail section
	dirty            bool
}

var _ View = (*ProviderView)(nil)
var _ CommandProvider = (*ProviderView)(nil)

// NewProviderView creates a new provider configuration view.
func NewProviderView(a *app.App, vcfg ProviderViewConfig) *ProviderView {
	return &ProviderView{appCtx: a, vcfg: vcfg, selectedProvider: "bedrock"}
}

// SetShell provides the shell reference for modal interactions.
func (v *ProviderView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ProviderView) ID() string { return "provider" }

// Title returns the display title.
func (v *ProviderView) Title() string { return "Provider" }

// StatusHints returns keybinding hints.
func (v *ProviderView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · Enter %s · s %s · w %s · r %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.setup"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.refresh"),
	)
}

// Mount builds the provider configuration interface.
func (v *ProviderView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirty = false

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.provider.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build asynchronously (keychain checks may do I/O)
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			v.buildLines()
			v.renderList()

			v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
				v.handleSelect(index, item)
			})
			v.list.SetItemChangedFunc(func(index int, item widgets.SectionItem) {
				v.handleChanged(index, item)
			})

			v.buildCommands()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

// Unmount cleans up resources.
func (v *ProviderView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.provider.unsaved"), false)
	}
	v.app = nil
	v.list = nil
}

// HandleKey processes provider view key events.
func (v *ProviderView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case ' ':
		v.setAsDefault()
		return nil
	case 'e':
		v.editCurrent()
		return nil
	case 's':
		v.setupProvider()
		return nil
	case 'd':
		v.resetCurrentProvider()
		return nil
	case 'w':
		v.save()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	switch event.Key() {
	case tcell.KeyEnter:
		v.editCurrent()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Line definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProviderView) buildLines() {
	cfg := v.vcfg.GetConfig()
	defaultProv := cfg.Opencode.DefaultProvider

	v.lines = []providerConfigLine{
		{kind: "section-header", key: i18n.T("tui.provider.section_providers")},
	}

	// One line per provider
	for _, name := range provider.AllProviders() {
		n := string(name)
		v.lines = append(v.lines, providerConfigLine{
			provider: n,
			key:      n,
			kind:     "provider-item",
			get: func(c *config.Config) string {
				return n
			},
		})
	}

	// Detail section for selected provider
	v.lines = append(v.lines,
		providerConfigLine{kind: "section-header", key: fmt.Sprintf(i18n.T("tui.provider.section_detail"), v.selectedProvider)},
	)

	// Bedrock-specific fields
	v.lines = append(v.lines,
		providerConfigLine{
			provider: "bedrock", key: "auth_mode", kind: "select",
			options: bedrockAuthModeOptions,
			visible: func() bool { return v.selectedProvider == "bedrock" },
			get:     func(c *config.Config) string { return c.Provider.Bedrock.AuthMode },
			set:     func(c *config.Config, val string) { c.Provider.Bedrock.AuthMode = val },
		},
		providerConfigLine{
			provider: "bedrock", key: "aws_region", kind: "string",
			visible: func() bool { return v.selectedProvider == "bedrock" },
			get:     func(c *config.Config) string { return c.Provider.Bedrock.AWSRegion },
			set:     func(c *config.Config, val string) { c.Provider.Bedrock.AWSRegion = val },
		},
		providerConfigLine{
			provider: "bedrock", key: "aws_profile", kind: "string",
			visible: func() bool {
				return v.selectedProvider == "bedrock" && cfg.Provider.Bedrock.AuthMode == "profile"
			},
			get: func(c *config.Config) string { return c.Provider.Bedrock.AWSProfile },
			set: func(c *config.Config, val string) { c.Provider.Bedrock.AWSProfile = val },
		},
		providerConfigLine{
			provider: "bedrock", key: "token", kind: "password",
			visible: func() bool {
				return v.selectedProvider == "bedrock" && cfg.Provider.Bedrock.AuthMode == "bearer"
			},
			get: func(_ *config.Config) string {
				ctx := context.Background()
				present, masked := v.vcfg.CheckSecret(ctx, "bedrock-token-default")
				if present {
					return "✓ " + masked
				}
				return "✗ " + i18n.T("tui.provider.not_configured")
			},
		},
	)

	// Anthropic token
	v.lines = append(v.lines,
		providerConfigLine{
			provider: "anthropic", key: "token", kind: "password",
			visible: func() bool { return v.selectedProvider == "anthropic" },
			get: func(_ *config.Config) string {
				ctx := context.Background()
				key := provider.KeychainKey(provider.Anthropic, "")
				present, masked := v.vcfg.CheckSecret(ctx, key)
				if present {
					return "✓ " + masked
				}
				return "✗ " + i18n.T("tui.provider.not_configured")
			},
		},
	)

	// OpenRouter token
	v.lines = append(v.lines,
		providerConfigLine{
			provider: "openrouter", key: "token", kind: "password",
			visible: func() bool { return v.selectedProvider == "openrouter" },
			get: func(_ *config.Config) string {
				ctx := context.Background()
				key := provider.KeychainKey(provider.OpenRouter, "")
				present, masked := v.vcfg.CheckSecret(ctx, key)
				if present {
					return "✓ " + masked
				}
				return "✗ " + i18n.T("tui.provider.not_configured")
			},
		},
	)

	// GitHub Copilot info
	v.lines = append(v.lines,
		providerConfigLine{
			provider: "github-copilot", key: "info", kind: "info",
			visible: func() bool { return v.selectedProvider == "github-copilot" },
			get: func(_ *config.Config) string {
				return i18n.T("tui.provider.copilot_info")
			},
		},
	)

	// Actions section
	v.lines = append(v.lines,
		providerConfigLine{kind: "section-header", key: i18n.T("tui.provider.section_actions")},
		providerConfigLine{
			key: i18n.T("tui.provider.action_detect"), kind: "action",
			get: func(_ *config.Config) string { return "(r)" },
		},
		providerConfigLine{
			key: i18n.T("tui.provider.action_reset"), kind: "action",
			get: func(_ *config.Config) string { return "(d)" },
		},
		providerConfigLine{
			key: i18n.T("tui.provider.action_wizard"), kind: "action",
			get: func(_ *config.Config) string { return "(s)" },
		},
	)

	_ = defaultProv
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProviderView) renderList() {
	if v.list == nil {
		return
	}
	cfg := v.vcfg.GetConfig()
	defaultProv := cfg.Opencode.DefaultProvider

	savedIdx := v.list.GetCurrentItem()
	items := make([]widgets.SectionItem, 0, len(v.lines))

	for i, line := range v.lines {
		// Skip invisible fields
		if line.visible != nil && !line.visible() {
			continue
		}

		switch line.kind {
		case "section-header":
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: line.key,
			})

		case "provider-item":
			name := line.provider
			result := provider.Detect(provider.Name(name))
			var statusStr string
			if result.Available {
				statusStr = fmt.Sprintf("%s✓%s %s", theme.ColorTag(theme.SuccessHex), theme.TagColor, result.Source)
			} else {
				statusStr = fmt.Sprintf("%s✗%s %s", theme.ColorTag(theme.ErrorHex), theme.TagColor, i18n.T("tui.provider.not_detected"))
			}
			defaultMarker := ""
			if name == defaultProv {
				defaultMarker = fmt.Sprintf("  %s★ %s%s", theme.ColorTag(theme.AccentHex), i18n.T("tui.provider.default"), theme.TagColor)
			}
			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%-18s %s%s", name, statusStr, defaultMarker),
				SecondaryText: provider.Description(provider.Name(name)),
				Reference:     i,
			})

		case "select", "string":
			val := ""
			if line.get != nil {
				val = line.get(cfg)
			}
			if val == "" {
				val = fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.provider.empty"), theme.TagColor)
			}
			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%-18s %s", line.key, val),
				SecondaryText: "",
				Reference:     i,
			})

		case "password":
			val := ""
			if line.get != nil {
				val = line.get(cfg)
			}
			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%-18s %s", line.key, val),
				SecondaryText: "",
				Reference:     i,
			})

		case "info":
			val := ""
			if line.get != nil {
				val = line.get(cfg)
			}
			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor),
				SecondaryText: "",
				Reference:     i,
			})

		case "action":
			val := ""
			if line.get != nil {
				val = line.get(cfg)
			}
			items = append(items, widgets.SectionItem{
				MainText:      fmt.Sprintf("→ %s  %s%s%s", line.key, theme.ColorTag(theme.TextMutedHex), val, theme.TagColor),
				SecondaryText: "",
				Reference:     i,
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 && savedIdx < len(items) {
		v.list.SelectIndex(savedIdx)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Interaction handlers
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProviderView) handleChanged(index int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.kind == "provider-item" && line.provider != v.selectedProvider {
		v.selectedProvider = line.provider
		v.buildLines()
		v.renderList()
		// Re-select the provider item
		v.list.SelectIndex(index)
	}
}

func (v *ProviderView) handleSelect(index int, item widgets.SectionItem) {
	v.editCurrent()
}

func (v *ProviderView) editCurrent() {
	if v.list == nil || v.shell == nil {
		return
	}
	idx, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	cfg := v.vcfg.GetConfig()

	switch line.kind {
	case "provider-item":
		// Switch to this provider's detail
		if line.provider != v.selectedProvider {
			v.selectedProvider = line.provider
			v.buildLines()
			v.renderList()
			v.list.SelectIndex(idx)
		}

	case "select":
		cur := ""
		if line.get != nil {
			cur = line.get(cfg)
		}
		v.shell.ShowSelectModal(line.key, line.options, cur, func(newVal string) {
			if line.set != nil {
				line.set(cfg, newVal)
				v.dirty = true
				v.buildLines() // Rebuild because visibility may change (e.g., auth_mode)
				v.renderList()
			}
		})

	case "string":
		cur := ""
		if line.get != nil {
			cur = line.get(cfg)
		}
		v.shell.ShowInputModal(line.key, cur, func(newVal string) {
			if line.set != nil {
				line.set(cfg, newVal)
				v.dirty = true
				v.renderList()
			}
		})

	case "password":
		v.shell.ShowPasswordModal(line.key, func(val string) {
			if val == "" {
				return
			}
			name := provider.Name(line.provider)
			keychainKey := provider.KeychainKey(name, "")
			if line.provider == "bedrock" {
				keychainKey = "bedrock-token-default"
			}
			if keychainKey != "" {
				_ = v.vcfg.SetSecret(context.Background(), keychainKey, val)
			}
			v.renderList()
			v.shell.ShowToastMsg(i18n.Tf("tui.provider.token_saved", line.provider), true)
		})

	case "action":
		// Actions are executed via their keybinding, not Enter.
		// But for convenience, map them.
		switch {
		case line.key == i18n.T("tui.provider.action_detect"):
			v.refresh()
		case line.key == i18n.T("tui.provider.action_reset"):
			v.resetCurrentProvider()
		case line.key == i18n.T("tui.provider.action_wizard"):
			v.setupProvider()
		}
	}
}

func (v *ProviderView) setAsDefault() {
	if v.list == nil || v.shell == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.kind != "provider-item" {
		return
	}

	cfg := v.vcfg.GetConfig()
	cfg.Opencode.DefaultProvider = line.provider
	v.dirty = true
	v.renderList()
	v.shell.ShowToastMsg(i18n.Tf("tui.provider.set_default", line.provider), true)
}

func (v *ProviderView) resetCurrentProvider() {
	if v.shell == nil {
		return
	}
	name := v.selectedProvider
	v.shell.ShowSelectModal(
		i18n.Tf("tui.provider.confirm_reset", name),
		[]SelectOption{
			{Label: i18n.T("tui.provider.yes_reset"), Value: "yes"},
			{Label: i18n.T("tui.provider.cancel"), Value: "no"},
		},
		"",
		func(choice string) {
			if choice != "yes" {
				return
			}
			cfg := v.vcfg.GetConfig()
			// Clear provider config fields
			switch name {
			case "bedrock":
				cfg.Provider.Bedrock = config.ProviderConfig{}
				if v.vcfg.DeleteSecret != nil {
					_ = v.vcfg.DeleteSecret(context.Background(), "bedrock-token-default")
				}
			case "anthropic":
				cfg.Provider.Anthropic = config.ProviderConfig{}
				if v.vcfg.DeleteSecret != nil {
					key := provider.KeychainKey(provider.Anthropic, "")
					_ = v.vcfg.DeleteSecret(context.Background(), key)
				}
			case "openrouter":
				cfg.Provider.OpenRouter = config.ProviderConfig{}
				if v.vcfg.DeleteSecret != nil {
					key := provider.KeychainKey(provider.OpenRouter, "")
					_ = v.vcfg.DeleteSecret(context.Background(), key)
				}
			}
			// Clear default if this was the default
			if cfg.Opencode.DefaultProvider == name {
				cfg.Opencode.DefaultProvider = ""
			}
			v.dirty = true
			v.buildLines()
			v.renderList()
			v.shell.ShowToastMsg(i18n.Tf("tui.provider.reset_done", name), true)
		})
}

func (v *ProviderView) save() {
	if v.shell == nil {
		return
	}
	cfg := v.vcfg.GetConfig()
	if err := v.vcfg.SaveConfig(cfg); err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.provider.save_error")+": "+err.Error(), false)
		return
	}
	v.dirty = false
	v.shell.ShowToastMsg(i18n.T("tui.provider.saved"), true)
}

func (v *ProviderView) refresh() {
	v.buildLines()
	v.renderList()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.provider.refreshed"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Setup wizards (preserved from old implementation)
// ─────────────────────────────────────────────────────────────────────────────

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
	v.shell.ShowSelectModal(i18n.T("tui.provider.choose"), providerSetupOptions, "", func(prov string) {
		switch prov {
		case "bedrock":
			v.setupBedrock()
		case "anthropic":
			v.setupAPIKey(provider.Anthropic, i18n.T("tui.provider.key_anthropic"))
		case "openrouter":
			v.setupAPIKey(provider.OpenRouter, i18n.T("tui.provider.key_openrouter"))
		case "github-copilot":
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.T("tui.provider.copilot_info"), true)
			}
		}
	})
}

func (v *ProviderView) setupBedrock() {
	v.shell.ShowSelectModal(i18n.T("tui.provider.bedrock_auth_mode"), bedrockAuthModeOptions, "", func(authMode string) {
		cfg := v.vcfg.GetConfig()
		cfg.Provider.Bedrock.AuthMode = authMode

		switch authMode {
		case "bearer":
			v.shell.ShowInlineForm(InlineFormConfig{
				Title: i18n.T("tui.provider.bedrock_bearer_title"),
				Fields: []FormField{
					{Label: "Bearer token", Key: "token", Type: FieldPassword, Required: true},
					{Label: "AWS Region", Key: "region", Type: FieldText, Default: "us-east-1"},
				},
				OnSubmit: func(values map[string]string, _ map[string][]string) {
					token := values["token"]
					if token == "" {
						return
					}
					if v.vcfg.SetSecret != nil {
						_ = v.vcfg.SetSecret(context.Background(), "bedrock-token-default", token)
					}
					if region := values["region"]; region != "" {
						cfg.Provider.Bedrock.AWSRegion = region
					}
					cfg.Opencode.DefaultProvider = "bedrock"
					_ = v.vcfg.SaveConfig(cfg)
					v.dirty = false
					v.buildLines()
					v.renderList()
					v.shell.ShowToastMsg(i18n.T("tui.provider.bedrock_configured_bearer"), true)
				},
			})
		case "profile":
			v.shell.ShowInlineForm(InlineFormConfig{
				Title: i18n.T("tui.provider.bedrock_profile_title"),
				Fields: []FormField{
					{Label: "AWS Profile", Key: "profile", Type: FieldText, Default: "default"},
					{Label: "AWS Region", Key: "region", Type: FieldText, Default: "us-east-1"},
				},
				OnSubmit: func(values map[string]string, _ map[string][]string) {
					if profile := values["profile"]; profile != "" {
						cfg.Provider.Bedrock.AWSProfile = profile
					}
					if region := values["region"]; region != "" {
						cfg.Provider.Bedrock.AWSRegion = region
					}
					cfg.Opencode.DefaultProvider = "bedrock"
					_ = v.vcfg.SaveConfig(cfg)
					v.dirty = false
					v.buildLines()
					v.renderList()
					v.shell.ShowToastMsg(i18n.T("tui.provider.bedrock_configured_profile"), true)
				},
			})
		case "env":
			cfg.Opencode.DefaultProvider = "bedrock"
			_ = v.vcfg.SaveConfig(cfg)
			v.dirty = false
			v.buildLines()
			v.renderList()
			v.shell.ShowToastMsg(i18n.T("tui.provider.bedrock_configured_env"), true)
		}
	})
}

func (v *ProviderView) setupAPIKey(name provider.Name, title string) {
	v.shell.ShowPasswordModal(title, func(key string) {
		if key == "" {
			return
		}
		keychainKey := provider.KeychainKey(name, "")
		if v.vcfg.SetSecret != nil && keychainKey != "" {
			_ = v.vcfg.SetSecret(context.Background(), keychainKey, key)
		}
		cfg := v.vcfg.GetConfig()
		cfg.Opencode.DefaultProvider = string(name)
		_ = v.vcfg.SaveConfig(cfg)
		v.dirty = false
		v.buildLines()
		v.renderList()
		v.shell.ShowToastMsg(i18n.Tf("tui.provider.configured", string(name)), true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Contextual commands
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProviderView) ContextCommands() []ContextCommand {
	return v.commands
}

func (v *ProviderView) buildCommands() {
	v.commands = []ContextCommand{
		{
			ID: "provider.setup", Label: i18n.T("tui.hints.setup"),
			Aliases: []string{"configurer", "configure", "setup"},
			Description: i18n.T("tui.provider.cmd_setup"), Category: "Provider",
			Action: v.setupProvider,
		},
		{
			ID: "provider.refresh", Label: i18n.T("tui.hints.refresh"),
			Aliases: []string{"rafraîchir", "reload", "refresh"},
			Description: i18n.T("tui.provider.cmd_refresh"), Category: "Provider",
			Action: v.refresh,
		},
		{
			ID: "provider.save", Label: i18n.T("tui.hints.save"),
			Aliases: []string{"save", "write", "sauvegarder"},
			Description: i18n.T("tui.provider.cmd_save"), Category: "Provider",
			Action: v.save,
		},
	}
	for _, name := range provider.AllProviders() {
		n := string(name)
		v.commands = append(v.commands, ContextCommand{
			ID: "provider.setup." + n, Label: "setup " + n,
			Aliases:     []string{n},
			Description: i18n.Tf("tui.provider.cmd_setup_specific", n), Category: "Provider",
			Action: func() {
				v.selectedProvider = n
				v.buildLines()
				v.renderList()
			},
		})
	}
}
