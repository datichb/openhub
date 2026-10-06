package views

import (
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
)

// budgetFields are the hub session restrictions (I6, hub.toml [limits]),
// shown after the Sessions settings. Empty = off. Team ceilings and project
// values are shown by `oh budget show`.
func (v *SettingsView) budgetFields() []configField {
	field := func(key string, kind FieldKind, validator *FieldValidator, placeholder string) configField {
		return configField{
			Key: "limits_" + key, Kind: kind, Validator: validator, Placeholder: placeholder,
			Label:       i18n.T("tui.budget.field." + key + ".label"),
			Description: i18n.T("tui.budget.field." + key + ".desc"),
			Get:         func() string { return v.live.Limits.Value(key) },
			Set: func(val string) {
				l := v.live.Limits
				if l.Set(key, val) == nil { // invalid amounts are ignored
					v.live.Limits = l
				}
			},
		}
	}
	positive := func(max int) *FieldValidator {
		return &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(max), AllowEmpty: true}
	}
	return []configField{
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.budget.section")},
		field(limits.FieldMaxActive, CfgFieldInt, positive(100), ""),
		field(limits.FieldSessionBudget, CfgFieldString, nil, "5"),
		field(limits.FieldDailyBudget, CfgFieldString, nil, "50"),
		field(limits.FieldMemory, CfgFieldInt, positive(1<<20), ""),
		field(limits.FieldModels, CfgFieldString, nil, "eu.anthropic.*"),
	}
}

// insertFieldsAfter inserts extra after the field with key (at the end
// when absent).
func insertFieldsAfter(fields []configField, key string, extra []configField) []configField {
	for i, f := range fields {
		if f.Key == key {
			out := make([]configField, 0, len(fields)+len(extra))
			out = append(out, fields[:i+1]...)
			out = append(out, extra...)
			return append(out, fields[i+1:]...)
		}
	}
	return append(fields, extra...)
}
