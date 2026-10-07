package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocaleParity verifies that fr.json and en.json have exactly the same keys.
// This prevents forgetting to add a translation when adding a new key.
func TestLocaleParity(t *testing.T) {
	frData, err := localeFS.ReadFile("locales/fr.json")
	require.NoError(t, err, "reading fr.json")

	enData, err := localeFS.ReadFile("locales/en.json")
	require.NoError(t, err, "reading en.json")

	var frKeys map[string]string
	var enKeys map[string]string

	require.NoError(t, json.Unmarshal(frData, &frKeys), "parsing fr.json")
	require.NoError(t, json.Unmarshal(enData, &enKeys), "parsing en.json")

	// Check keys in FR that are missing from EN
	for key := range frKeys {
		if _, ok := enKeys[key]; !ok {
			t.Errorf("key %q exists in fr.json but missing from en.json", key)
		}
	}

	// Check keys in EN that are missing from FR
	for key := range enKeys {
		if _, ok := frKeys[key]; !ok {
			t.Errorf("key %q exists in en.json but missing from fr.json", key)
		}
	}

	// Verify counts match
	assert.Equal(t, len(frKeys), len(enKeys),
		"fr.json and en.json should have the same number of keys")

	t.Logf("Locale parity OK: %d keys in each locale", len(frKeys))
}

// TestAllKeysHaveValues verifies no key has an empty value.
func TestAllKeysHaveValues(t *testing.T) {
	for _, locale := range []string{"fr", "en"} {
		data, err := localeFS.ReadFile("locales/" + locale + ".json")
		require.NoError(t, err)

		var keys map[string]string
		require.NoError(t, json.Unmarshal(data, &keys))

		for key, value := range keys {
			if value == "" {
				t.Errorf("[%s] key %q has empty value", locale, key)
			}
		}
	}
}

// formatVerbRe matches Go format verbs: %s, %d, %q, %v, %w, %f, %.Nf, %%, etc.
var formatVerbRe = regexp.MustCompile(`%[+\-#0 ]*(?:\d+)?(?:\.\d+)?[sdqvwfgetxXoUpTbn%]`)

// TestFormatVerbParity verifies that format verbs (%s, %d, %q, etc.) appear
// in the same quantity and order between fr.json and en.json for each key.
// This prevents runtime panics from mismatched format strings.
func TestFormatVerbParity(t *testing.T) {
	frData, err := localeFS.ReadFile("locales/fr.json")
	require.NoError(t, err)
	enData, err := localeFS.ReadFile("locales/en.json")
	require.NoError(t, err)

	var frKeys map[string]string
	var enKeys map[string]string
	require.NoError(t, json.Unmarshal(frData, &frKeys))
	require.NoError(t, json.Unmarshal(enData, &enKeys))

	for key, frVal := range frKeys {
		enVal, ok := enKeys[key]
		if !ok {
			continue // parity test catches this
		}

		frVerbs := formatVerbRe.FindAllString(frVal, -1)
		enVerbs := formatVerbRe.FindAllString(enVal, -1)

		// Filter out escaped %% (not a real verb)
		frVerbs = filterEscapedPercent(frVerbs)
		enVerbs = filterEscapedPercent(enVerbs)

		if len(frVerbs) != len(enVerbs) {
			t.Errorf("key %q: FR has %d format verbs %v, EN has %d format verbs %v",
				key, len(frVerbs), frVerbs, len(enVerbs), enVerbs)
			continue
		}

		for i := range frVerbs {
			if frVerbs[i] != enVerbs[i] {
				t.Errorf("key %q: format verb #%d differs — FR=%q EN=%q",
					key, i+1, frVerbs[i], enVerbs[i])
			}
		}
	}
}

func filterEscapedPercent(verbs []string) []string {
	var filtered []string
	for _, v := range verbs {
		if v != "%%" {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// TestNoDuplicateKeys decodes each locale token by token: json.Unmarshal keeps
// the last value of a repeated key silently, which hides the first one.
func TestNoDuplicateKeys(t *testing.T) {
	for _, locale := range []string{"fr", "en"} {
		data, err := localeFS.ReadFile("locales/" + locale + ".json")
		require.NoError(t, err)
		for _, key := range duplicateKeys(t, data) {
			t.Errorf("[%s] key %q is defined more than once", locale, key)
		}
	}
}

func duplicateKeys(t *testing.T, data []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok, "locale must be a JSON object")
	seen := map[string]bool{}
	var dups []string
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		key, ok := tok.(string)
		require.True(t, ok, "expected a key, got %v", tok)
		var value string
		require.NoError(t, dec.Decode(&value), "value of %q", key)
		if seen[key] {
			dups = append(dups, key)
		}
		seen[key] = true
	}
	return dups
}

func TestDuplicateKeysDetected(t *testing.T) {
	got := duplicateKeys(t, []byte(`{"a": "1", "b": "2", "a": "3"}`))
	assert.Equal(t, []string{"a"}, got)
}

// TestJSONValidity verifies both locale files are valid JSON.
func TestJSONValidity(t *testing.T) {
	for _, locale := range []string{"fr", "en"} {
		data, err := localeFS.ReadFile("locales/" + locale + ".json")
		require.NoError(t, err, fmt.Sprintf("reading %s.json", locale))

		var m map[string]string
		err = json.Unmarshal(data, &m)
		require.NoError(t, err, fmt.Sprintf("%s.json is not valid JSON", locale))
	}
}

// QB3: the V1 of the tool is no longer supported (D3): no message offers
// it (tui.start.free_desc did). The messages explaining that V1 is refused
// (cmd.v1.unsupported.*) are the exception.
func TestNoMessageOffersV1(t *testing.T) {
	v1 := regexp.MustCompile(`\bV1\b`) // not the oh/v1 schema
	for _, lang := range []string{"fr", "en"} {
		data, err := localeFS.ReadFile("locales/" + lang + ".json")
		require.NoError(t, err)
		var msgs map[string]string
		require.NoError(t, json.Unmarshal(data, &msgs))
		for k, v := range msgs {
			if v1.MatchString(v) && !regexp.MustCompile(`^cmd\.v1\.`).MatchString(k) {
				t.Errorf("%s %s mentions V1: %q", lang, k, v)
			}
		}
	}
}
