package views

import (
	"fmt"
	"strconv"
)

// FieldValidator defines validation rules for a configLine field.
// Used by editSelected() to reject invalid values before setting them,
// and by save() for a global validation pass.
type FieldValidator struct {
	// AllowedValues is a static enum constraint. If non-nil and non-empty,
	// the value must be one of these strings.
	AllowedValues []string
	// AllowedFunc is a dynamic enum constraint. Called at validation time
	// to retrieve the current allowed values (e.g., provider.AllProviders()).
	// Takes precedence over AllowedValues when non-nil.
	AllowedFunc func() []string
	// Numeric indicates the value must be a valid integer.
	Numeric bool
	// MinInt/MaxInt constrain numeric values (only when Numeric is true).
	MinInt *int
	MaxInt *int
	// Required indicates the value must not be empty.
	Required bool
	// AllowEmpty permits an empty string even when AllowedValues is set.
	// Useful for optional fields that have an enum when populated.
	AllowEmpty bool
}

// Validate checks the value against all configured rules.
// Returns nil if valid, or an error describing the first violation.
func (fv *FieldValidator) Validate(value string) error {
	if fv == nil {
		return nil
	}

	// Required check
	if fv.Required && value == "" {
		return fmt.Errorf("valeur requise")
	}

	// Allow empty if explicitly permitted or not required
	if value == "" && fv.AllowEmpty {
		return nil
	}

	// Enum check (dynamic first, then static)
	allowed := fv.AllowedValues
	if fv.AllowedFunc != nil {
		allowed = fv.AllowedFunc()
	}
	if len(allowed) > 0 && value != "" {
		found := false
		for _, a := range allowed {
			if a == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("valeur invalide %q (attendu: %v)", value, allowed)
		}
	}

	// Numeric check
	if fv.Numeric && value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("valeur numérique attendue, reçu %q", value)
		}
		if fv.MinInt != nil && n < *fv.MinInt {
			return fmt.Errorf("valeur minimum: %d", *fv.MinInt)
		}
		if fv.MaxInt != nil && n > *fv.MaxInt {
			return fmt.Errorf("valeur maximum: %d", *fv.MaxInt)
		}
	}

	return nil
}

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int {
	return &v
}
