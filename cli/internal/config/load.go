package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Strict reading of hub.toml (v5.0). The file is decoded with go-toml into a
// Config that already holds the defaults, so a key left out keeps its
// default. What the former reader (viper and mapstructure, weakly typed)
// accepted and this one does not is listed in docs/reference/config.*.md
// ("Strict reading of hub.toml") and tested in load_test.go:
//   - a value of the wrong type is an error that names the file, the line,
//     the column and the key ("true" in quotes for a boolean, a number in
//     quotes, a number for a string, a comma-separated string for a list…);
//   - the names of the keys are still matched without regard to case, and
//     the keys of models.families and models.agents are still lowercased;
//   - a key that no setting uses is a warning (Notice), not an error: a
//     hub.toml written by a newer oh still loads.

// ConfigError is a hub.toml that cannot be read: a syntax error, or a value
// that does not have the type of its setting.
type ConfigError struct {
	// Path is the file.
	Path string
	// Line and Column locate the error (1-based; 0 when unknown).
	Line, Column int
	// Key is the dotted key whose value is wrong ("" for a syntax error).
	Key string
	// Expected and Found describe a type mismatch with the Kind* values
	// ("" otherwise).
	Expected, Found string
	// InList: the mismatch is on an element of a list, Expected being the
	// kind of the elements.
	InList bool
	// Msg is the message of the TOML library (English).
	Msg string
}

// Kinds of value, as Expected and Found of a ConfigError.
const (
	KindBoolean = "boolean"
	KindInteger = "integer"
	KindNumber  = "number"
	KindString  = "string"
	KindList    = "list"
	KindTable   = "table"
)

// Error is the English text: file, line, column, key, what is wrong.
func (e *ConfigError) Error() string {
	var b strings.Builder
	b.WriteString(e.Path)
	if e.Line > 0 {
		fmt.Fprintf(&b, ":%d:%d", e.Line, e.Column)
	}
	b.WriteString(": ")
	if e.Key != "" {
		fmt.Fprintf(&b, "key %q: ", e.Key)
	}
	if e.Expected != "" && e.Found != "" {
		if e.InList {
			fmt.Fprintf(&b, "expected %s in the list, found %s", article(e.Expected), article(e.Found))
		} else {
			fmt.Fprintf(&b, "expected %s, found %s", article(e.Expected), article(e.Found))
		}
		switch e.Hint() {
		case HintUnquote:
			b.WriteString(" (write it without quotes)")
		case HintList:
			b.WriteString(` (write a list: ["a", "b"])`)
		}
		return b.String()
	}
	b.WriteString(e.Msg)
	return b.String()
}

func article(kind string) string {
	if strings.ContainsAny(kind[:1], "aeiou") {
		return "an " + kind
	}
	return "a " + kind
}

// Hints of a ConfigError.
const (
	// HintUnquote: a string where a boolean or a number is expected.
	HintUnquote = "unquote"
	// HintList: a string where a list is expected.
	HintList = "list"
)

// Hint tells how to fix the value when it is a known case ("" otherwise).
func (e *ConfigError) Hint() string {
	switch {
	case e.InList:
		return ""
	case e.Found == KindString && (e.Expected == KindBoolean || e.Expected == KindInteger || e.Expected == KindNumber):
		return HintUnquote
	case e.Found == KindString && e.Expected == KindList:
		return HintList
	}
	return ""
}

// defaultConfig is the configuration of a hub without hub.toml.
func defaultConfig() *Config {
	c := &Config{Name: "OpenHub"}
	c.CLI.Language = "en"
	c.Worktree.AutoCleanup = true
	c.GC.Enabled = true
	return c
}

// parseHub decodes the content of hub.toml (read from path): the
// configuration, and the notices (former keys, unknown keys) to raise.
func parseHub(path string, data []byte) (*Config, []Notice, error) {
	c := defaultConfig()
	var unknown []Notice
	if err := decode(data, c, true); err != nil {
		var missing *toml.StrictMissingError
		if !errors.As(err, &missing) {
			return nil, nil, newConfigError(path, data, err)
		}
		unknown = unknownKeys(path, missing)
	}

	var l legacyFile
	if err := decode(data, &l, false); err != nil {
		return nil, nil, newConfigError(path, data, err)
	}
	var notes []Notice
	for _, key := range applyLegacyKeys(l, c) {
		n := Notice{Kind: NoticeLegacyKey, Key: key, Replacement: legacyReplacement[key], File: path}
		n.Line, n.Column = findKey(data, key)
		notes = append(notes, n)
	}
	for _, u := range unknown {
		if _, legacy := legacyReplacement[strings.ToLower(u.Key)]; legacy || isHistoricalKey(u.Key) {
			continue
		}
		notes = append(notes, u)
	}

	// The keys of these maps name agents and families: viper lowercased
	// them, so a key written with capitals still matched.
	c.Models.Families = lowerKeys(c.Models.Families)
	c.Models.Agents = lowerKeys(c.Models.Agents)
	return c, notes, nil
}

// decode reads a TOML document into v; unknownIsError makes the keys v has
// no field for a *toml.StrictMissingError (the value is still filled). The
// library panics on a date or time where it expects a string: that is
// reported as an error too.
func decode(data []byte, v any, unknownIsError bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("a date or time value where another type is expected")
		}
	}()
	dec := toml.NewDecoder(bytes.NewReader(data))
	if unknownIsError {
		dec.DisallowUnknownFields()
	}
	return dec.Decode(v)
}

func lowerKeys(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

// unknownKeys lists the keys no setting uses, without the keys under an
// unknown table already listed.
func unknownKeys(path string, missing *toml.StrictMissingError) []Notice {
	var out []Notice
	var listed []string
	for _, e := range missing.Errors {
		key := strings.Join(e.Key(), ".")
		under := false
		for _, l := range listed {
			if strings.HasPrefix(key, l+".") {
				under = true
				break
			}
		}
		if under {
			continue
		}
		listed = append(listed, key)
		line, col := e.Position()
		out = append(out, Notice{Kind: NoticeUnknownKey, Key: key, File: path, Line: line, Column: col})
	}
	return out
}

// Validate reports whether data would load as the hub.toml at path (the
// errors of Load, none for unknown keys): oh doctor --fix only restores a
// backup that loads.
func Validate(path string, data []byte) error {
	_, _, err := parseHub(path, data)
	return err
}

// Inspect reads hub.toml again, without touching the cache or the notices
// handler, and returns what Load would raise: the former keys still read and
// the keys ignored (oh doctor). A missing file has none; a file that does
// not load returns the same error as Load.
func Inspect() ([]Notice, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading hub.toml: %w", err)
	}
	_, notes, err := parseHub(path, data)
	return notes, err
}

// mismatch matches the messages of the TOML library for a value that does
// not fit its setting.
var mismatch = regexp.MustCompile(`cannot (?:decode|assign|store)|cannot be assigned`)

// decodeMismatch reads "cannot decode TOML string into struct field
// config.CLIConfig.SetupDone of type bool".
var decodeMismatch = regexp.MustCompile(`cannot decode TOML (.+?) into (?:struct field (\S+)|a (\S+)) of type (\S+)`)

// newConfigError turns an error of the TOML library into a ConfigError.
func newConfigError(path string, data []byte, err error) *ConfigError {
	ce := &ConfigError{Path: path, Msg: strings.TrimPrefix(err.Error(), "toml: ")}
	var de *toml.DecodeError
	if errors.As(err, &de) {
		ce.Line, ce.Column = de.Position()
		ce.Msg = strings.TrimPrefix(de.Error(), "toml: ")
	}
	isMismatch := mismatch.MatchString(ce.Msg)

	// The setting named by the message, when the library gives no position.
	var field string
	m := decodeMismatch.FindStringSubmatch(ce.Msg)
	if m != nil {
		field = m[2]
	}
	if ce.Line == 0 {
		if leaf := tomlNameOf(field); leaf != "" {
			if line, col := findKey(data, leaf); line > 0 {
				ce.Line, ce.Column = line, col
			} else {
				ce.Key = leaf
			}
		}
	}
	if ce.Line > 0 {
		ce.Key = keyAt(data, ce.Line)
	}
	if !isMismatch {
		return ce
	}

	// Expected: the type of the setting; Found: the value as written.
	if t, ok := typeOfKey(ce.Key); ok {
		ce.Expected = kindOf(t)
		ce.Found = valueKindAt(data, ce.Line, ce.Column)
		if ce.Expected == KindList && ce.Found != "" && ce.Found != KindList && valueStartsList(data, ce.Line) {
			ce.Expected, ce.InList = kindOf(t.Elem()), true
		}
	}
	if ce.Expected == "" && m != nil {
		ce.Expected = expectedKind(m[4])
	}
	if ce.Found == "" && m != nil {
		ce.Found = foundKind(m[1])
	}
	if ce.Found == "" || ce.Expected == "" {
		ce.Expected, ce.Found = "", ""
	}
	return ce
}

func foundKind(tomlType string) string {
	switch tomlType {
	case "string":
		return KindString
	case "integer":
		return KindInteger
	case "float":
		return KindNumber
	case "boolean":
		return KindBoolean
	case "array":
		return KindList
	case "table", "inline table":
		return KindTable
	}
	return tomlType
}

// kindOf is the kind of value a Go type holds.
func kindOf(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return KindBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return KindInteger
	case reflect.Float32, reflect.Float64:
		return KindNumber
	case reflect.String:
		return KindString
	case reflect.Slice, reflect.Array:
		return KindList
	case reflect.Struct, reflect.Map:
		return KindTable
	}
	return ""
}

// typeOfKey walks the settings along a dotted key (names as in the file, any
// case) and returns the Go type of the value: a segment under a map is a map
// key, under a list of tables it goes on in the elements.
func typeOfKey(key string) (reflect.Type, bool) {
	if key == "" {
		return nil, false
	}
	for _, root := range []reflect.Type{reflect.TypeOf(Config{}), reflect.TypeOf(legacyFile{})} {
		t := root
		ok := true
		for _, seg := range strings.Split(key, ".") {
			seg = strings.Trim(seg, `"'`)
			for t.Kind() == reflect.Pointer || (t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Struct) {
				t = t.Elem()
			}
			switch t.Kind() {
			case reflect.Struct:
				var next reflect.Type
				for i := 0; i < t.NumField(); i++ {
					name, _, _ := strings.Cut(t.Field(i).Tag.Get("toml"), ",")
					if strings.EqualFold(name, seg) {
						next = t.Field(i).Type
						break
					}
				}
				if next == nil {
					ok = false
				} else {
					t = next
				}
			case reflect.Map:
				t = t.Elem()
			default:
				ok = false
			}
			if !ok {
				break
			}
		}
		if ok {
			return t, true
		}
	}
	return nil, false
}

var (
	numberText = regexp.MustCompile(`^[+-]?(?:inf|nan|[0-9_]+(?:\.[0-9_]+)?(?:[eE][+-]?[0-9_]+)?)`)
	dateText   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}|^\d{2}:\d{2}`)
)

// valueKindAt names the kind of the value written at a position of a TOML
// document ("" when it cannot tell).
func valueKindAt(data []byte, line, col int) string {
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) || col < 1 || col > len(lines[line-1]) {
		return ""
	}
	v := lines[line-1][col-1:]
	switch {
	case strings.HasPrefix(v, `"`), strings.HasPrefix(v, `'`):
		return KindString
	case strings.HasPrefix(v, "true"), strings.HasPrefix(v, "false"):
		return KindBoolean
	case strings.HasPrefix(v, "["):
		return KindList
	case strings.HasPrefix(v, "{"):
		return KindTable
	case dateText.MatchString(v):
		return "date"
	}
	if m := numberText.FindString(v); m != "" {
		if strings.ContainsAny(m, ".eEn") && !strings.HasPrefix(strings.ToLower(m), "0x") {
			return KindNumber
		}
		return KindInteger
	}
	return ""
}

// valueStartsList reports a key line whose value is a list: "key = [".
func valueStartsList(data []byte, line int) bool {
	lines := strings.Split(string(data), "\n")
	for i := line - 1; i >= 0 && i >= line-1-64; i-- {
		if keyLine.MatchString(lines[i]) {
			_, v, _ := strings.Cut(lines[i], "=")
			return strings.HasPrefix(strings.TrimSpace(v), "[")
		}
	}
	return false
}

func expectedKind(goType string) string {
	t := strings.TrimLeft(goType, "*")
	switch {
	case t == "bool":
		return KindBoolean
	case strings.HasPrefix(t, "int"), strings.HasPrefix(t, "uint"):
		return KindInteger
	case strings.HasPrefix(t, "float"):
		return KindNumber
	case t == "string":
		return KindString
	case strings.HasPrefix(t, "[]"):
		return KindList
	case strings.HasPrefix(t, "map["), strings.HasPrefix(t, "config."), strings.HasPrefix(t, "limits."):
		return KindTable
	}
	return goType
}

// tomlNameOf resolves "config.CLIConfig.SetupDone" (a field named by the
// TOML library) to the TOML name of that field ("setup_done").
func tomlNameOf(goField string) string {
	i := strings.LastIndex(goField, ".")
	if i < 0 {
		return ""
	}
	owner, name := goField[:i], goField[i+1:]
	if j := strings.LastIndex(owner, "."); j >= 0 {
		owner = owner[j+1:]
	}
	var found string
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if t.Name() == owner && f.Name == name && found == "" {
				found, _, _ = strings.Cut(f.Tag.Get("toml"), ",")
			}
			walk(f.Type)
		}
	}
	walk(reflect.TypeOf(Config{}))
	return found
}

var (
	tableHeader = regexp.MustCompile(`^\s*\[{1,2}\s*([^\[\]]+?)\s*\]{1,2}\s*(?:#.*)?$`)
	keyLine     = regexp.MustCompile(`^\s*((?:[A-Za-z0-9_-]+|"[^"]*"|'[^']*')(?:\s*\.\s*(?:[A-Za-z0-9_-]+|"[^"]*"|'[^']*'))*)\s*=`)
	dotSpaces   = regexp.MustCompile(`\s*\.\s*`)
)

// keyAt returns the dotted key of the value at a line of a TOML document: the
// key of the nearest line "key = …" up from there, under the nearest table
// header above it.
func keyAt(data []byte, line int) string {
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	keyIdx := -1
	for i := line - 1; i >= 0 && i >= line-1-64; i-- {
		if keyLine.MatchString(lines[i]) {
			keyIdx = i
			break
		}
	}
	if keyIdx < 0 {
		return ""
	}
	key := dotSpaces.ReplaceAllString(keyLine.FindStringSubmatch(lines[keyIdx])[1], ".")
	for i := keyIdx - 1; i >= 0; i-- {
		if m := tableHeader.FindStringSubmatch(lines[i]); m != nil {
			return dotSpaces.ReplaceAllString(m[1], ".") + "." + key
		}
	}
	return key
}

// findKey locates a key in a document: the line and column of "b = …" under
// the header [a] for "a.b", or of "a.b = …" at the top level; a bare name
// matches its last part under any header, when only one line has it. 0, 0
// when it is not found.
func findKey(data []byte, dotted string) (line, col int) {
	lines := strings.Split(string(data), "\n")
	want := strings.ToLower(dotted)
	for i, l := range lines {
		m := keyLine.FindStringSubmatchIndex(l)
		if m == nil {
			continue
		}
		if strings.ToLower(keyAt(data, i+1)) == want {
			return i + 1, m[2] + 1
		}
		if !strings.Contains(want, ".") && strings.EqualFold(strings.TrimSpace(l[m[2]:m[3]]), want) {
			if line != 0 {
				return 0, 0
			}
			line, col = i+1, m[2]+1
		}
	}
	return line, col
}
