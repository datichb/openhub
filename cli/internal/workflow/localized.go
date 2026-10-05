package workflow

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// LocalizedText is a user-facing string that is either a plain scalar
// (any language) or a mapping language → text:
//
//	description: Implémenter un ticket
//	description: { fr: Implémenter un ticket, en: Implement a ticket }
type LocalizedText struct {
	// Default is the plain scalar form (empty when the mapping form is used).
	Default string
	// ByLang holds the mapping form, keyed by language code ("fr", "en").
	ByLang map[string]string
}

// Text returns the text for lang, falling back to the plain form, then "en",
// then "fr", then any language (stable order).
func (t LocalizedText) Text(lang string) string {
	if s := t.ByLang[lang]; s != "" {
		return s
	}
	if t.Default != "" {
		return t.Default
	}
	for _, l := range []string{"en", "fr"} {
		if s := t.ByLang[l]; s != "" {
			return s
		}
	}
	langs := make([]string, 0, len(t.ByLang))
	for l := range t.ByLang {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, l := range langs {
		if s := t.ByLang[l]; s != "" {
			return s
		}
	}
	return ""
}

// IsZero reports whether no text is set (used by `omitempty`).
func (t LocalizedText) IsZero() bool {
	return t.Default == "" && len(t.ByLang) == 0
}

// UnmarshalYAML accepts a scalar or a mapping of language codes.
func (t *LocalizedText) UnmarshalYAML(unmarshal func(any) error) error {
	node, err := captureNode(unmarshal)
	if err != nil {
		return err
	}
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			*t = LocalizedText{}
			return nil
		}
		*t = LocalizedText{Default: node.Value}
		return nil
	case yaml.MappingNode:
		var m map[string]string
		if err := unmarshal(&m); err != nil {
			return err
		}
		*t = LocalizedText{ByLang: m}
		return nil
	default:
		return nodeError(node, "expected_text")
	}
}

// MarshalYAML writes the plain form when set, the mapping form otherwise.
func (t LocalizedText) MarshalYAML() (any, error) {
	if len(t.ByLang) == 0 {
		return t.Default, nil
	}
	return t.ByLang, nil
}
