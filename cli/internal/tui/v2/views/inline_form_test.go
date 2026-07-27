package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormField_HintIsOptional verifies that FormField works correctly
// when Hint is empty (backward compatibility with existing callers).
func TestFormField_HintIsOptional(t *testing.T) {
	f := FormField{
		Key:   "name",
		Label: "Name",
		Type:  FieldText,
	}
	assert.Empty(t, f.Hint, "Hint should default to empty string")
}

// TestFormField_HintIsPreserved verifies that a non-empty Hint is stored correctly.
func TestFormField_HintIsPreserved(t *testing.T) {
	hint := "Enter your username"
	f := FormField{
		Key:   "username",
		Label: "Username",
		Type:  FieldText,
		Hint:  hint,
	}
	assert.Equal(t, hint, f.Hint)
}

// TestFormField_HintWithPasswordField verifies that Hint works for password fields.
func TestFormField_HintWithPasswordField(t *testing.T) {
	f := FormField{
		Key:   "token",
		Label: "Token d'accès",
		Type:  FieldPassword,
		Hint:  "Stocké dans votre keychain système, jamais dans oh",
	}
	assert.Equal(t, FieldPassword, f.Type)
	assert.NotEmpty(t, f.Hint)
}

// TestFormField_HintWithSelectField verifies that Hint works for select fields.
func TestFormField_HintWithSelectField(t *testing.T) {
	f := FormField{
		Key:  "role",
		Label: "Rôle",
		Type: FieldSelect,
		Options: []SelectOption{
			{Label: "Lead", Value: "lead"},
			{Label: "Développeur", Value: "dev"},
		},
		Hint: "Votre rôle dans l'équipe",
	}
	assert.Equal(t, "Votre rôle dans l'équipe", f.Hint)
	assert.Len(t, f.Options, 2)
}

// TestInlineFormConfig_FieldsWithHints verifies a complete form config
// with multiple fields having hints.
func TestInlineFormConfig_FieldsWithHints(t *testing.T) {
	cfg := InlineFormConfig{
		Title: "Step 2 — Credentials",
		Fields: []FormField{
			{
				Key:     "username",
				Label:   "Username",
				Type:    FieldText,
				Default: "oauth2",
				Hint:    "GitLab : oauth2 · GitHub : votre username",
			},
			{
				Key:   "token",
				Label: "Token d'accès",
				Type:  FieldPassword,
				Hint:  "Stocké dans votre keychain système, jamais dans oh",
			},
		},
		OnSubmit: func(_ map[string]string, _ map[string][]string) {},
	}

	assert.Equal(t, "Step 2 — Credentials", cfg.Title)
	assert.Len(t, cfg.Fields, 2)
	assert.Equal(t, "GitLab : oauth2 · GitHub : votre username", cfg.Fields[0].Hint)
	assert.Equal(t, FieldPassword, cfg.Fields[1].Type)
	assert.Equal(t, "Stocké dans votre keychain système, jamais dans oh", cfg.Fields[1].Hint)
}

// TestInlineFormConfig_BackwardCompatibility verifies that existing callers
// without Hint continue to work (zero-value Hint = no hint displayed).
func TestInlineFormConfig_BackwardCompatibility(t *testing.T) {
	cfg := InlineFormConfig{
		Title: "Config",
		Fields: []FormField{
			{Key: "name", Label: "Name", Type: FieldText, Default: "Alice"},
			{Key: "enabled", Label: "Enabled", Type: FieldBool, Default: "true"},
		},
	}

	for _, f := range cfg.Fields {
		assert.Empty(t, f.Hint, "existing fields without Hint should have empty hint")
	}
}
