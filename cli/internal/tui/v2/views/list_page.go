package views

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// newListPage builds the shared layout of the list pages (same as the
// workflow catalogue): main on the left framed in the active mode colour,
// « Détail » pane on the right. Call it at Mount time (theme.ActiveMode is set).
func newListPage(title string, main tview.Primitive) *widgets.ListDetailPage {
	p := widgets.NewListDetailPage(title, i18n.T("tui.catalog.detail"))
	p.SetMain(main)
	return p
}

// setPageDetail sets the detail text of page (no-op when page is nil).
func setPageDetail(page *widgets.ListDetailPage, text string) {
	if page != nil {
		page.SetDetail(text)
	}
}

// detailText accumulates the content of a detail pane.
type detailText struct{ b strings.Builder }

// title writes the bold heading (already escaped / colour-tagged text).
func (d *detailText) title(s string) *detailText {
	fmt.Fprintf(&d.b, "[::b]%s[::-]\n\n", s)
	return d
}

// para writes a paragraph followed by a blank line (skipped when empty).
func (d *detailText) para(s string) *detailText {
	if strings.TrimSpace(s) != "" {
		fmt.Fprintf(&d.b, "%s\n\n", s)
	}
	return d
}

// muted writes a muted paragraph (skipped when empty).
func (d *detailText) muted(s string) *detailText {
	if strings.TrimSpace(s) != "" {
		fmt.Fprintf(&d.b, "%s%s%s\n\n", theme.ColorTag(theme.TextMutedHex), s, theme.TagReset)
	}
	return d
}

// warn writes a warning line.
func (d *detailText) warn(s string) *detailText {
	fmt.Fprintf(&d.b, "%s%s%s\n\n", theme.ColorTag(theme.WarningHex), s, theme.TagReset)
	return d
}

// field writes « label value » (skipped when value is empty).
func (d *detailText) field(label, value string) *detailText {
	if value != "" {
		fmt.Fprintf(&d.b, "%s%s%s %s\n", theme.ColorTag(theme.TextSecondaryHex), label, theme.TagReset, value)
	}
	return d
}

// gap writes a blank line.
func (d *detailText) gap() *detailText {
	d.b.WriteString("\n")
	return d
}

// keys writes the « Touches » block; pairs are key, action, key, action…
func (d *detailText) keys(pairs ...string) *detailText {
	if len(pairs) < 2 {
		return d
	}
	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	fmt.Fprintf(&d.b, "\n%s%s%s\n", theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.detail.keys"), theme.TagReset)
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&d.b, "  %s%-6s%s %s\n", accent, pairs[i], theme.TagReset, pairs[i+1])
	}
	return d
}

func (d *detailText) String() string { return d.b.String() }

// emptyDetail is the text of the detail pane when nothing is selected.
func emptyDetail() string {
	return theme.ColorTag(theme.TextMutedHex) + i18n.T("tui.detail.empty") + theme.TagReset
}

// fieldKeys returns the key/action pairs that apply to a config field.
func fieldKeys(f configField) []string {
	if f.Locked != nil && f.Locked() {
		return nil
	}
	switch f.Kind {
	case CfgFieldBool, CfgFieldTriBool:
		return []string{"Space", i18n.T("tui.hints.toggle"), "Enter", i18n.T("tui.hints.edit")}
	case CfgFieldLink:
		return []string{"Enter", i18n.T("tui.hints.open")}
	case CfgFieldString, CfgFieldSelect, CfgFieldInt, CfgFieldPassword:
		return []string{"Enter", i18n.T("tui.hints.edit")}
	}
	return nil
}

// configFieldDetail renders the detail pane of a config field: label, value,
// description, technical key, origin / lock, allowed values and keys. extra
// key/action pairs are appended to the keys of the field.
func configFieldDetail(f configField, extra ...string) string {
	var d detailText
	label := f.Label
	if label == "" {
		label = f.Key
	}
	d.title(label)
	d.para(f.Description)
	val := ""
	if f.Get != nil {
		val = f.Get()
	}
	switch f.Kind {
	case CfgFieldLink:
		d.field(i18n.T("tui.detail.opens"), f.LinkTarget)
	case CfgFieldReadonly:
		d.field(i18n.T("tui.detail.value"), displayOrPlaceholder(val, "-"))
		d.field(i18n.T("tui.detail.status"), i18n.T("tui.detail.readonly"))
	default:
		d.field(i18n.T("tui.detail.value"), formatFieldValue(f, val))
	}
	if f.Key != "" && f.Kind != CfgFieldLink {
		key := f.Key
		if f.Section != "" {
			key = f.Section + " · " + key
		}
		d.field(i18n.T("tui.detail.key"), tview.Escape(key))
	}
	if f.Source != nil {
		d.field(i18n.T("tui.detail.origin"), tview.Escape(f.Source()))
	}
	if f.Placeholder != "" {
		d.field(i18n.T("tui.detail.example"), tview.Escape(f.Placeholder))
	}
	if r := validatorRange(f.Validator); r != "" {
		d.field(i18n.T("tui.detail.range"), r)
	}
	if opts := fieldOptions(f); len(opts) > 0 {
		d.gap()
		d.field(i18n.T("tui.detail.options"), " ")
		for _, o := range opts {
			mark := "  "
			if o.Value == val {
				mark = theme.ColorTag(theme.ActiveMode.PrimaryHex) + "● " + theme.TagReset
			}
			fmt.Fprintf(&d.b, "  %s%s\n", mark, tview.Escape(o.Label))
		}
	}
	if f.Locked != nil && f.Locked() {
		d.gap()
		d.warn(i18n.T("tui.detail.locked"))
	}
	d.keys(append(fieldKeys(f), extra...)...)
	return d.String()
}

// fieldOptions returns the choices offered by a select / tri-state field.
func fieldOptions(f configField) []SelectOption {
	switch f.Kind {
	case CfgFieldSelect:
		if len(f.Options) > 0 {
			return f.Options
		}
		if f.OptionsFunc != nil {
			return f.OptionsFunc()
		}
	case CfgFieldTriBool:
		if len(f.Options) > 0 {
			return f.Options
		}
		return triStateOptions()
	}
	return nil
}

// validatorRange formats the numeric bounds of a validator ("" = none).
func validatorRange(v *FieldValidator) string {
	if v == nil || !v.Numeric || (v.MinInt == nil && v.MaxInt == nil) {
		return ""
	}
	lo, hi := "…", "…"
	if v.MinInt != nil {
		lo = fmt.Sprint(*v.MinInt)
	}
	if v.MaxInt != nil {
		hi = fmt.Sprint(*v.MaxInt)
	}
	return lo + " – " + hi
}

// fieldOfItem returns the config field referenced by a list item.
func fieldOfItem(fields []configField, it widgets.SectionItem) (configField, bool) {
	i, ok := it.Reference.(int)
	if !ok || i < 0 || i >= len(fields) {
		return configField{}, false
	}
	return fields[i], true
}

// showCurrentFieldDetail fills the detail pane with the field under the cursor.
func showCurrentFieldDetail(page *widgets.ListDetailPage, list *widgets.SectionedList, fields []configField, extra ...string) {
	if page == nil || list == nil {
		return
	}
	_, it, ok := list.CurrentItem()
	if !ok {
		page.SetDetail(emptyDetail())
		return
	}
	showFieldItemDetail(page, fields, it, extra...)
}

// showFieldItemDetail fills the detail pane with the field of it.
func showFieldItemDetail(page *widgets.ListDetailPage, fields []configField, it widgets.SectionItem, extra ...string) {
	if page == nil {
		return
	}
	f, ok := fieldOfItem(fields, it)
	if !ok {
		page.SetDetail(itemDetail(it))
		return
	}
	page.SetDetail(configFieldDetail(f, extra...))
}

// itemDetail is the fallback detail of a list item (main + secondary text).
func itemDetail(it widgets.SectionItem) string {
	if it.MainText == "" {
		return emptyDetail()
	}
	var d detailText
	d.title(strings.TrimSpace(it.MainText))
	d.para(strings.TrimSpace(it.SecondaryText))
	return d.String()
}

// modelOverrideDetail renders the detail pane of a model override row
// (section general | families | agents; key "" = empty placeholder row).
// value "" means not configured (inherited); level ("" = none) is the
// configuration level shown; extra key/action pairs follow.
func modelOverrideDetail(section, key, value, level string, extra ...string) string {
	var d detailText
	if key == "" {
		d.title(i18n.T("tui.detail.empty"))
		d.para(i18n.T("tui.detail.models.empty"))
		d.muted(i18n.T("tui.detail.models.cascade"))
		d.keys(extra...)
		return d.String()
	}
	switch section {
	case "families":
		d.title(tview.Escape(key))
		d.para(i18n.Tf("tui.detail.models.family", tview.Escape(key)))
	case "agents":
		d.title(tview.Escape(key))
		d.para(i18n.Tf("tui.detail.models.agent", tview.Escape(key)))
	default:
		d.title(i18n.T("tui.config.field.default_model.label"))
		d.para(i18n.T("tui.detail.models.default"))
	}
	d.field(i18n.T("tui.detail.level"), level)
	if value == "" {
		d.field(i18n.T("tui.detail.value"), theme.ColorTag(theme.TextMutedHex)+i18n.T("tui.config.not_configured")+theme.TagReset)
	} else {
		d.field(i18n.T("tui.detail.value"), tview.Escape(value))
	}
	d.gap()
	d.muted(i18n.T("tui.detail.inherit_hint") + " " + i18n.T("tui.detail.models.cascade"))
	keys := []string{"Enter", i18n.T("tui.hints.edit")}
	keys = append(keys, extra...)
	if section == "families" || section == "agents" {
		keys = append(keys, "d", i18n.T("tui.hints.delete"))
	}
	d.keys(keys...)
	return d.String()
}
