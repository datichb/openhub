package views

import (
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
	CfgFieldTriBool                        // tri-state: nil=not configured (with optional inheritance note based on Scope)
	CfgFieldInt                            // integer input with optional min/max
	CfgFieldPassword                       // masked input (Enter → PasswordModal) — unifies "tokenkey" and "password"
	CfgFieldReadonly                       // display only (cannot edit)
	CfgFieldLink                           // navigation link to another view (Enter → NavigateTo)
	CfgFieldSectionHeader                  // section divider (non-selectable, rendered as ── Name ──)
	CfgFieldSubHeader                      // sub-section divider
	CfgFieldPlaceholder                    // placeholder item (e.g. "no entries yet")
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
	// Options provides static choices for CfgFieldSelect / CfgFieldTriBool.
	// For CfgFieldTriBool, if non-empty these override the default triStateOptions().
	Options []SelectOption
	// OptionsFunc provides dynamic choices (called at edit time).
	OptionsFunc func() []SelectOption
	// Validator holds optional validation rules (reused from config_validator.go).
	Validator *FieldValidator
	// LinkTarget is the view ID to navigate to for CfgFieldLink.
	LinkTarget string
	// Scope identifies the configuration level (hub/team-shared/team-local/project).
	// Used by formatFieldValue to determine inheritance annotations on nil values.
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
	formattedVal := formatFieldValue(f, val)

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
// It receives the full configField so it can use Scope for inheritance annotations.
//
// Principle: nil / empty = "non configuré" — never "disabled", "(empty)" or "default".
// For CfgFieldTriBool at project/team-local scope, a parenthetical inheritance note
// is appended: "non configuré (hérite du hub)".
func formatFieldValue(f configField, val string) string {
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagReset

	switch f.Kind {
	case CfgFieldBool:
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.enabled"), reset)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.disabled"), reset)
		default:
			return fmt.Sprintf("%s%s%s", muted, i18n.T("tui.config.not_configured"), reset)
		}

	case CfgFieldTriBool:
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.enabled"), reset)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.disabled"), reset)
		default:
			label := i18n.T("tui.config.not_configured")
			switch f.Scope {
			case ScopeProject:
				label += " (" + i18n.T("tui.config.inherits_from_hub") + ")"
			case ScopeTeamLocal:
				label += " (" + i18n.T("tui.config.inherits_from_team") + ")"
			}
			return fmt.Sprintf("%s%s%s", muted, label, reset)
		}

	case CfgFieldPassword:
		if val != "" {
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.config.configured"), reset)
		}
		return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.config.not_configured"), reset)

	case CfgFieldSelect:
		if val == "" {
			return fmt.Sprintf("%s%s%s", muted, i18n.T("tui.config.not_configured"), reset)
		}
		return val

	case CfgFieldInt:
		if val == "" || val == "(hérité)" || val == "(inherit)" {
			return fmt.Sprintf("%s%s%s", muted, i18n.T("tui.config.not_configured"), reset)
		}
		return val

	case CfgFieldReadonly:
		return fmt.Sprintf("%s%s %s%s", muted, val, i18n.T("tui.config.readonly"), reset)

	case CfgFieldLink:
		return fmt.Sprintf("%s→%s", theme.ColorTag(theme.ActiveMode.PrimaryHex), reset)

	case CfgFieldPlaceholder:
		return fmt.Sprintf("%s%s%s", muted, val, reset)

	default: // CfgFieldString, CfgFieldInfo, CfgFieldProviderItem, CfgFieldAction
		if val == "" {
			return fmt.Sprintf("%s%s%s", muted, i18n.T("tui.config.not_configured"), reset)
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
		options := triStateOptions()
		if len(f.Options) > 0 {
			options = f.Options
		}
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
			f.Set("") // not configured
		default:
			f.Set("true")
		}
	}
}

// isToggleable returns true if the field kind supports Space-key toggling.
func isToggleable(kind FieldKind) bool {
	return kind == CfgFieldBool || kind == CfgFieldTriBool
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

// triStateOptions returns the standard tri-state options for fields where nil
// means "not configured". This is the default for CfgFieldTriBool unless
// the field overrides Options.
func triStateOptions() []SelectOption {
	return []SelectOption{
		{Label: "↩ " + i18n.T("tui.config.not_configured"), Value: ""},
		{Label: "✓ " + i18n.T("tui.config.enabled"), Value: "true"},
		{Label: "✗ " + i18n.T("tui.config.disabled"), Value: "false"},
	}
}

// ---------- Context helper ----------
