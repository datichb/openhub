package views

// FieldType defines the type of an inline form field.
type FieldType int

const (
	// FieldText is a free-text input field.
	FieldText FieldType = iota
	// FieldPassword is a masked text input field.
	FieldPassword
	// FieldSelect is a single-choice selector (DropDown in the modal).
	FieldSelect
	// FieldMultiSelect is a multi-choice selector (individual checkboxes).
	FieldMultiSelect
	// FieldBool is a boolean toggle (Checkbox in the modal).
	FieldBool
)

// FormField defines a single field in an InlineFormConfig.
type FormField struct {
	// Key identifies the field in the results map.
	Key string
	// Label is displayed next to the field.
	Label string
	// Type determines the widget used in the modal form.
	Type FieldType
	// Options are the allowed values (for FieldSelect and FieldMultiSelect).
	Options []SelectOption
	// Default is the initial string value (for all types except FieldMultiSelect).
	Default string
	// DefaultMulti is the initial selection (for FieldMultiSelect only).
	DefaultMulti []string
	// Required means the form won't submit if this field is empty.
	Required bool
	// Conditional, if set, determines whether this field is shown.
	// It receives the current string values of all fields.
	// NOTE: Conditionals are currently not evaluated in the modal form —
	// all fields are always displayed. Reserved for future use.
	Conditional func(values map[string]string) bool
}

// InlineFormConfig holds the configuration for a modal form.
type InlineFormConfig struct {
	Title    string
	Fields   []FormField
	OnSubmit func(values map[string]string, multi map[string][]string)
	OnCancel func()
}
