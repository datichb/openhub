// Package i18n provides internationalization support using JSON locale files.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	messages map[string]map[string]string // locale -> key -> value
	current  string
	mu       sync.RWMutex
)

func init() {
	messages = make(map[string]map[string]string)
	current = "en"
}

// SetLocale sets the active locale.
func SetLocale(locale string) {
	mu.Lock()
	defer mu.Unlock()
	current = locale
}

// Locale returns the current active locale.
func Locale() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T translates a key using the current locale. Falls back to English, then to the key itself.
func T(key string) string {
	mu.RLock()
	locale := current
	mu.RUnlock()

	// Try current locale
	if msg := get(locale, key); msg != "" {
		return msg
	}
	// Fallback to English
	if locale != "en" {
		if msg := get("en", key); msg != "" {
			return msg
		}
	}
	// Return key as-is
	return key
}

// Tf translates a key with fmt.Sprintf formatting.
func Tf(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}

func get(locale, key string) string {
	mu.RLock()
	m, loaded := messages[locale]
	mu.RUnlock()

	if !loaded {
		m = loadLocale(locale)
	}
	if m == nil {
		return ""
	}
	return m[key]
}

func loadLocale(locale string) map[string]string {
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring write lock
	if m, ok := messages[locale]; ok {
		return m
	}

	data, err := localeFS.ReadFile(fmt.Sprintf("locales/%s.json", locale))
	if err != nil {
		messages[locale] = nil
		return nil
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		messages[locale] = nil
		return nil
	}

	messages[locale] = m
	return m
}

// KeyOf returns the key of a message of the en or fr locale ("" when s is
// not a message; the smallest key when several share it). Texts translated
// before the locale is known (cobra commands built at init) are translated
// again with it.
func KeyOf(s string) string {
	if s == "" {
		return ""
	}
	for _, locale := range []string{"en", "fr"} {
		best := ""
		for k, v := range loadLocale(locale) {
			if v == s && (best == "" || k < best) {
				best = k
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}
