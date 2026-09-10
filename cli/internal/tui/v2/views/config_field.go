package views

import (
	"context"
	"fmt"
	"strconv"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// FieldKind identifies the type of a configuration field, replacing the
// ad-hoc string constants ("bool", "tri-bool", "tri-state", "tokenkey", …)
// spread across the 6 former configLine struct variants.
type FieldKind int

const (
	CfgFieldBool          FieldKind = iota // true/false toggle (Space key)
	CfgFieldString                         // free-text input (Enter → InputModal)
	CfgFieldSelect                         // choice from a list (Enter → SelectModal)
	CfgFieldTriBool                        // project/team: nil=inherit from parent level
	CfgFieldTriDefault                     // hub-level: nil=system default (NOT "inherited")
	CfgFieldInt                            // integer input with optional min/max
	CfgFieldPassword                       // masked input (Enter → PasswordModal) — unifies "tokenkey" and "password"
	CfgFieldReadonly                        // display only (cannot edit)
	CfgFieldLink                           // navigation link to another view (Enter → NavigateTo)
	CfgFieldSectionHeader                  // section divider (non-selectable, rendered as ── Name ──)
	CfgFieldSubHeader                      // sub-section divider
	CfgFieldPlaceholder                    // placeholder item (e.g. "no entries yet")
	CfgFieldAgents                         // multi-agent selection (Enter → MultiSelectModal)
	CfgFieldProviderItem                   // provider entry in ProviderView
	CfgFieldAction                         // clickable action in ProviderView
	CfgFieldInfo                           // information display in ProviderView
)

// ConfigScope identifies at which configuration level a field lives.
type ConfigScope int

const (
	ScopeHub        ConfigScope = iota // hub.toml personal settings
	ScopeTeamShared                    // team-state Git repo (shared)
	ScopeTeamLocal                     // hub.toml local overrides for team settings
	ScopeProject                       // project (SQLite)
)

// configField is the unified descriptor for a single configuration field.
// It replaces the 6 former struct variants (configLine, projectConfigLine,
// teamConfigLine, mcpFieldDef, mcpLine, providerConfigLine).
//
// All getters/setters use zero-arg closures so the field definition can
// capture its data source (hub Config, Project, TeamConfig, …) at build time.
type configField struct {
	// Section groups fields visually. Used by renderConfigItem for section headers.
	Section string
	// Label is the human-readable, translated name (e.g. "Synchronisation automatique").
	Label string
	// Description is an optional explanatory text shown as SecondaryText in the list.
	Description string
	// Key is the technical config key (e.g. "auto_sync"), used for debugging.
	Key string
	// Kind determines editing behavior and display formatting.
	Kind FieldKind
	// Placeholder is shown inside InputModal when the current value is empty.
	Placeholder string
	// Options provides static choices for CfgFieldSelect / CfgFieldTriBool / CfgFieldTriDefault.
	Options []SelectOption
	// OptionsFunc provides dynamic choices (called at edit time).
	OptionsFunc func() []SelectOption
	// Validator holds optional validation rules (reused from config_validator.go).
	Validator *FieldValidator
	// LinkTarget is the view ID to navigate to for CfgFieldLink.
	LinkTarget string
	// Scope identifies the configuration level (hub/team-shared/team-local/project).
	Scope ConfigScope
	// Source returns a resolution annotation (e.g. "[hub]", "[equipe: enforced]").
	// nil means no annotation.
	Source func() string
	// Locked returns true if the field is enforced and cannot be edited.
	// nil means always editable.
	Locked func() bool
	// Dynamic means the field can be added/deleted (CRUD maps).
	Dynamic bool
	// Visible controls conditional display. nil means always visible.
	Visible func() bool
	// Get returns the current value as a display string.
	Get func() string
	// Set applies a new value.
	Set func(string)
}

// ---------- Rendering ----------

// renderConfigItem converts a configField into a SectionItem for the SectionedList widget.
// columnWidth controls the label column padding (typically 28-32).
func renderConfigItem(f configField, columnWidth int) widgets.SectionItem {
	if f.Kind == CfgFieldSectionHeader {
		return widgets.SectionItem{
			MainText: f.Label,
			IsHeader: true,
		}
	}
	if f.Kind == CfgFieldSubHeader {
		return widgets.SectionItem{
			MainText: fmt.Sprintf("    %s%s%s", theme.ColorTag(theme.TextMutedHex), f.Label, theme.TagReset),
			IsHeader: true,
		}
	}

	locked := f.Locked != nil && f.Locked()

	// Build the value display
	val := ""
	if f.Get != nil {
		val = f.Get()
	}
	formattedVal := formatFieldValue(f.Kind, val, f.Scope)

	// Build source annotation
	sourceAnnotation := ""
	if f.Source != nil {
		if src := f.Source(); src != "" {
			sourceAnnotation = fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), src, theme.TagReset)
		}
	}

	// Assemble main text: "  Label          value    [source]"
	label := f.Label
	if label == "" {
		label = f.Key // fallback to technical key
	}
	mainText := fmt.Sprintf("%-*s %s%s", columnWidth, label, formattedVal, sourceAnnotation)

	// Description as secondary text (muted color)
	secondaryText := ""
	if f.Description != "" {
		secondaryText = fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), f.Description, theme.TagReset)
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondaryText,
		Locked:        locked,
	}
}

// formatFieldValue produces a colored display string for a config value.
func formatFieldValue(kind FieldKind, val string, scope ConfigScope) string {
	switch kind {
	case CfgFieldBool:
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.enabled"), theme.TagReset)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.disabled"), theme.TagReset)
		default:
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.empty"), theme.TagReset)
		}

	case CfgFieldTriBool:
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.enabled"), theme.TagReset)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.disabled"), theme.TagReset)
		default:
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.inherited"), theme.TagReset)
		}

	case CfgFieldTriDefault:
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.enabled"), theme.TagReset)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.disabled"), theme.TagReset)
		default:
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.system_default"), theme.TagReset)
		}

	case CfgFieldPassword:
		if val != "" {
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.configured"), theme.TagReset)
		}
		return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.not_configured"), theme.TagReset)

	case CfgFieldSelect:
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.empty"), theme.TagReset)
		}
		return val

	case CfgFieldInt:
		if val == "" || val == "(hérité)" || val == "(inherit)" {
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.system_default"), theme.TagReset)
		}
		return val

	case CfgFieldReadonly:
		return fmt.Sprintf("%s%s %s%s", theme.ColorTag(theme.TextMutedHex), val, i18n.T("tui.config.readonly"), theme.TagReset)

	case CfgFieldLink:
		return fmt.Sprintf("%s→%s", theme.ColorTag(theme.AccentHex), theme.TagReset)

	case CfgFieldPlaceholder:
		return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagReset)

	default: // CfgFieldString, FieldAgentsCfgFieldInfo, CfgFieldProviderItem, CfgFieldAction
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.config.empty"), theme.TagReset)
		}
		return val
	}
}

// ---------- Editing ----------

// editConfigField dispatches to the appropriate modal for the given field kind.
// The onDone callback is called after a successful edit (for dirty tracking, auto-save, etc.).
func editConfigField(shell ShellAccess, f *configField, onDone func()) {
	if f.Locked != nil && f.Locked() {
		shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		return
	}

	switch f.Kind {
	case CfgFieldBool:
		toggleConfigField(f)
		if onDone != nil {
			onDone()
		}

	case CfgFieldTriBool:
		options := triInheritOptions()
		current := f.Get()
		shell.ShowSelectModal(f.Label, options, current, func(val string) {
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldTriDefault:
		options := triDefaultOptions()
		current := f.Get()
		shell.ShowSelectModal(f.Label, options, current, func(val string) {
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldSelect:
		options := f.Options
		if f.OptionsFunc != nil {
			options = f.OptionsFunc()
		}
		current := f.Get()
		shell.ShowSelectModal(f.Label, options, current, func(val string) {
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldInt:
		current := f.Get()
		shell.ShowInputModal(f.Label, current, func(val string) {
			if val == "" {
				f.Set("")
				if onDone != nil {
					onDone()
				}
				return
			}
			if _, err := strconv.Atoi(val); err != nil {
				shell.ShowToastMsg(i18n.T("tui.config.invalid_number"), false)
				return
			}
			if f.Validator != nil {
				if err := f.Validator.Validate(val); err != nil {
					shell.ShowToastMsg(err.Error(), false)
					return
				}
			}
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldPassword:
		shell.ShowPasswordModal(f.Label, func(val string) {
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldString:
		current := f.Get()
		shell.ShowInputModal(f.Label, current, func(val string) {
			if f.Validator != nil {
				if err := f.Validator.Validate(val); err != nil {
					shell.ShowToastMsg(err.Error(), false)
					return
				}
			}
			f.Set(val)
			if onDone != nil {
				onDone()
			}
		})

	case CfgFieldLink:
		if f.LinkTarget != "" {
			shell.NavigateTo(f.LinkTarget)
		}

	case CfgFieldReadonly, CfgFieldSectionHeader, CfgFieldSubHeader, CfgFieldPlaceholder, CfgFieldInfo:
		// Non-editable — no action
		return

	case CfgFieldAgents:
		// Agent editing is view-specific; handled by the view directly.
		return

	case CfgFieldAction, CfgFieldProviderItem:
		// Provider-specific; handled by ProviderView directly.
		return
	}
}

// toggleConfigField cycles a boolean or tri-state field via Space key.
func toggleConfigField(f *configField) {
	switch f.Kind {
	case CfgFieldBool:
		current := f.Get()
		if current == "true" {
			f.Set("false")
		} else {
			f.Set("true")
		}

	case CfgFieldTriBool:
		current := f.Get()
		switch current {
		case "true":
			f.Set("false")
		case "false":
			f.Set("") // inherit
		default:
			f.Set("true")
		}

	case CfgFieldTriDefault:
		current := f.Get()
		switch current {
		case "true":
			f.Set("false")
		case "false":
			f.Set("") // system default
		default:
			f.Set("true")
		}
	}
}

// isToggleable returns true if the field kind supports Space-key toggling.
func isToggleable(kind FieldKind) bool {
	return kind == CfgFieldBool || kind == CfgFieldTriBool || kind == CfgFieldTriDefault
}

// isEditable returns true if the field kind supports Enter-key editing.
func isEditable(kind FieldKind) bool {
	switch kind {
	case CfgFieldSectionHeader, CfgFieldSubHeader, CfgFieldPlaceholder, CfgFieldReadonly:
		return false
	default:
		return true
	}
}

// isSelectable returns true if the field kind can receive cursor focus.
func isSelectable(kind FieldKind) bool {
	return kind != CfgFieldSectionHeader && kind != CfgFieldSubHeader
}

// ---------- Validation ----------

// validateAllFields runs validation on all editable fields and returns a list of error messages.
func validateAllFields(fields []configField) []string {
	var errs []string
	for _, f := range fields {
		if f.Validator == nil || f.Kind == CfgFieldSectionHeader || f.Kind == CfgFieldSubHeader {
			continue
		}
		val := ""
		if f.Get != nil {
			val = f.Get()
		}
		if err := f.Validator.Validate(val); err != nil {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			errs = append(errs, fmt.Sprintf("%s: %s", label, err.Error()))
		}
	}
	return errs
}

// ---------- Option helpers ----------

// triInheritOptions returns the standard tri-bool options for project/team fields
// where nil means "inherit from parent level".
func triInheritOptions() []SelectOption {
	return []SelectOption{
		{Label: "↩ " + i18n.T("tui.config.inherited"), Value: ""},
		{Label: "✓ " + i18n.T("tui.config.enabled"), Value: "true"},
		{Label: "✗ " + i18n.T("tui.config.disabled"), Value: "false"},
	}
}

// triDefaultOptions returns the standard tri-bool options for hub-level fields
// where nil means "use system default" (NOT "inherited" — hub is top level).
func triDefaultOptions() []SelectOption {
	return []SelectOption{
		{Label: "↩ " + i18n.T("tui.config.system_default"), Value: ""},
		{Label: "✓ " + i18n.T("tui.config.enabled"), Value: "true"},
		{Label: "✗ " + i18n.T("tui.config.disabled"), Value: "false"},
	}
}

// ---------- Context helper ----------

// fieldContext returns a background context. Views that need cancellation
// should pass their shell.Context() instead.
func fieldContext() context.Context {
	return context.Background()
}
