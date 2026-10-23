package archtest

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/glyph"
)

// UI style guard (v5.1, A2): the colors and the symbols of the TUI come from
// internal/tui/theme, nowhere else. The production code of internal/tui/** and
// of cmd/tui_*.go is parsed, and four kinds of shortcut are counted per file:
//
//	tag-color      a tview color tag written by hand ("[#64a0ff]", "[red]",
//	               "[%s]" given a color, "[" + hex + "]"): use theme.Paint,
//	               theme.ColorTag or theme.ColorTagOf
//	tag-style      a tview style or background tag written by hand ("[::b]",
//	               "[:#313244]"): use theme.StyleTag or theme.BgTag
//	color-literal  a color named in the code (tcell.ColorRed, tcell.NewRGBColor,
//	               lipgloss.Color, "#rrggbb"): use a token of the theme
//	widgets-tag    a call to widgets.ColorTag, which is theme.ColorTagOf
//	symbol         a symbol of the table of meanings (glyph.Symbols, and ✓, the
//	               variant of ✔) written in a literal: use theme.Sym*
//
// The shortcuts that exist today are listed in testdata/ui_style_exceptions.txt,
// per file and per kind, with their count. The list only shrinks: a new
// shortcut fails the test, and so does a count that went down or a file that
// has none left until the line is updated or removed. The theme itself is out
// of the guard (it is where the colors are defined), and theme/icons.go is
// where the symbols are.

const uiStyleExceptionsFile = "testdata/ui_style_exceptions.txt"

const (
	catTagColor     = "tag-color"
	catTagStyle     = "tag-style"
	catColorLiteral = "color-literal"
	catWidgetsTag   = "widgets-tag"
	catSymbol       = "symbol"
)

var uiStyleAdvice = map[string]string{
	catTagColor:     "build the tag with theme.Paint(color, text), theme.ColorTag(hex) or theme.ColorTagOf(color), which follow NO_COLOR",
	catTagStyle:     "build the tag with theme.StyleTag(theme.AttrBold…) or theme.BgTag(color), which follow NO_COLOR",
	catColorLiteral: "take the color from internal/tui/theme (a token such as theme.Success, theme.AccentHex), do not name one here",
	catWidgetsTag:   "call theme.ColorTagOf (or theme.Paint) instead of widgets.ColorTag, which goes away",
	catSymbol:       "take the symbol from the table of meanings of the theme (theme.SymSuccess, SymWarning…), do not write it in a literal",
}

// uiFinding is one shortcut found in a file.
type uiFinding struct {
	cat  string
	line int
	text string
}

var (
	tviewTag      = regexp.MustCompile(`\[([^\[\]]*)\]`)
	tviewHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)
	tviewStyleSet = regexp.MustCompile(`^[a-zA-Z]+$`)
	hexLiteral    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fmtColorVerb  = regexp.MustCompile(`^%(\[[0-9]+\])?[sv]$`)
	fmtVerb       = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z]`)
)

// namedColors are the color names that a tview tag (and a literal) takes.
var namedColors = map[string]bool{
	"black": true, "red": true, "green": true, "yellow": true, "blue": true, "magenta": true, "cyan": true,
	"white": true, "gray": true, "grey": true, "orange": true, "purple": true, "pink": true, "silver": true,
	"maroon": true, "navy": true, "teal": true, "lime": true, "olive": true, "fuchsia": true, "aqua": true,
	"brown": true, "gold": true, "violet": true, "darkred": true, "darkgreen": true, "darkblue": true,
	"darkgray": true, "darkgrey": true, "lightgray": true, "lightgrey": true,
}

// tcellNeutral are the tcell.Color* that are no color at all.
var tcellNeutral = map[string]bool{"ColorDefault": true, "ColorReset": true}

// uiSymbols are the symbols that a literal must not hold: the table of
// meanings, and ✓ which is a variant of ✔.
func uiSymbols() map[string]string {
	out := map[string]string{"✓": "success"}
	for _, s := range glyph.Symbols {
		out[s.Glyph] = s.Name
	}
	return out
}

// colorPart tells whether one part of a tview tag ("fg", "bg") is a color
// written by hand: a hex, a name, or a format directive that will hold one.
func colorPart(p string) bool {
	return tviewHexColor.MatchString(p) || namedColors[strings.ToLower(p)] || fmtColorVerb.MatchString(p)
}

// tagFindings looks at the tview tags of a literal. Only a tag whose every
// part is a color (or "-", or nothing) and whose third part is a set of
// attribute letters counts: "[error]" or "[%s: enforced]" are text.
func tagFindings(s string) (color, style bool) {
	for _, m := range tviewTag.FindAllStringSubmatch(s, -1) {
		parts := strings.Split(m[1], ":")
		if len(parts) > 3 {
			continue
		}
		valid, c, st := true, false, false
		for i, p := range parts {
			switch {
			case p == "" || p == "-":
			case i < 2 && colorPart(p):
				if i == 0 {
					c = true
				} else {
					st = true // a background
				}
			case i == 2 && tviewStyleSet.MatchString(p):
				st = true
			default:
				valid = false
			}
		}
		if valid {
			color = color || c
			style = style || st
		}
	}
	return color, style
}

// symbolsIn lists the symbols that a literal holds as a word of their own:
// at the start, after a space, an opening parenthesis or the end of a tag, and before a space,
// the end or a closing parenthesis, once the tview tags and the fmt verbs
// are taken out (" %s✓%s " reads " ✓ "). The en dash is only a symbol alone:
// it is also the punctuation of a sentence.
func symbolsIn(lit string, table map[string]string) []string {
	s := tviewTag.ReplaceAllString(lit, "")
	s = fmtVerb.ReplaceAllString(s, " ")
	var out []string
	for g := range table {
		if !strings.Contains(s, g) {
			continue
		}
		if g == "–" {
			if strings.TrimSpace(s) == g {
				out = append(out, g)
			}
			continue
		}
		for i := 0; ; {
			j := strings.Index(s[i:], g)
			if j < 0 {
				break
			}
			start, end := i+j, i+j+len(g)
			before := start == 0 || strings.ContainsRune(" \t\n(]", rune(s[start-1]))
			after := end == len(s) || strings.ContainsRune(" \t\n),:", rune(s[end]))
			if before && after {
				out = append(out, g)
				break
			}
			i = end
		}
	}
	sort.Strings(out)
	return out
}

// flatten gives the operands of a chain of + in order.
func flatten(e ast.Expr) []ast.Expr {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
		return append(flatten(b.X), flatten(b.Y)...)
	}
	return []ast.Expr{e}
}

func stringOf(e ast.Expr) (string, bool) {
	l, ok := e.(*ast.BasicLit)
	if !ok || l.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(l.Value)
	return v, err == nil
}

// uiFindings parses a Go file and returns its shortcuts. inWidgets tells that
// the file is in package widgets (where ColorTag is called without a prefix);
// isIcons is theme/icons.go, which defines the symbols.
func uiFindings(fset *token.FileSet, path string, src any, inTheme, isIcons bool) ([]uiFinding, error) {
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	table := uiSymbols()
	var out []uiFinding
	add := func(cat string, pos token.Pos, text string) {
		out = append(out, uiFinding{cat, fset.Position(pos).Line, text})
	}
	inWidgets := file.Name.Name == "widgets"
	chains := map[*ast.BinaryExpr]bool{} // the inner parts of a chain of +, seen with the whole

	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.BinaryExpr:
			// "[" + hex + "]…": the tag is cut by an expression.
			if chains[n] {
				return true
			}
			for x := n.X; ; {
				b, ok := x.(*ast.BinaryExpr)
				if !ok || b.Op != token.ADD {
					break
				}
				chains[b] = true
				x = b.X
			}
			ops := flatten(n)
			if len(ops) < 3 {
				return true
			}
			for i := 1; i+1 < len(ops); i++ {
				l, ok1 := stringOf(ops[i-1])
				r, ok2 := stringOf(ops[i+1])
				_, isLit := stringOf(ops[i])
				if ok1 && ok2 && !isLit && strings.HasSuffix(l, "[") && (strings.HasPrefix(r, "]") || strings.HasPrefix(r, ":")) {
					add(catTagColor, n.Pos(), l+"…"+r)
				}
			}
		case *ast.BasicLit:
			switch n.Kind {
			case token.STRING:
				v, err := strconv.Unquote(n.Value)
				if err != nil {
					return true
				}
				if !inTheme {
					color, style := tagFindings(v)
					if color {
						add(catTagColor, n.Pos(), v)
					}
					if style {
						add(catTagStyle, n.Pos(), v)
					}
					if hexLiteral.MatchString(v) {
						add(catColorLiteral, n.Pos(), v)
					}
				}
				if !isIcons {
					for _, g := range symbolsIn(v, table) {
						add(catSymbol, n.Pos(), g+" in "+v)
					}
				}
			case token.CHAR:
				v, err := strconv.Unquote(n.Value)
				if err == nil && !isIcons {
					if _, ok := table[v]; ok {
						add(catSymbol, n.Pos(), v)
					}
				}
			}
		case *ast.SelectorExpr:
			pkg, ok := n.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case pkg.Name == "tcell" && strings.HasPrefix(n.Sel.Name, "Color") && n.Sel.Name != "Color" && !tcellNeutral[n.Sel.Name] && !inTheme:
				add(catColorLiteral, n.Pos(), "tcell."+n.Sel.Name)
			case pkg.Name == "tcell" && (n.Sel.Name == "NewRGBColor" || n.Sel.Name == "NewHexColor") && !inTheme:
				add(catColorLiteral, n.Pos(), "tcell."+n.Sel.Name)
			case pkg.Name == "lipgloss" && (n.Sel.Name == "Color" || n.Sel.Name == "AdaptiveColor") && !inTheme:
				add(catColorLiteral, n.Pos(), "lipgloss."+n.Sel.Name)
			case pkg.Name == "widgets" && n.Sel.Name == "ColorTag":
				add(catWidgetsTag, n.Pos(), "widgets.ColorTag")
			}
		case *ast.CallExpr:
			// tcell.GetColor(theme.XHex) converts a token; GetColor("red") names a color.
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && !inTheme && len(n.Args) == 1 {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "tcell" && sel.Sel.Name == "GetColor" {
					if v, isLit := stringOf(n.Args[0]); isLit && !hexLiteral.MatchString(v) {
						add(catColorLiteral, n.Pos(), "tcell.GetColor("+v+")")
					}
				}
			}
			if id, ok := n.Fun.(*ast.Ident); ok && inWidgets && (id.Name == "ColorTag") {
				add(catWidgetsTag, n.Pos(), "ColorTag")
			}
		}
		return true
	})
	return out, nil
}

// uiGuarded tells whether a production file is under the guard, and how.
func uiGuarded(rel string) (guarded, inTheme, isIcons bool) {
	if strings.HasSuffix(rel, "_test.go") {
		return false, false, false
	}
	switch {
	case strings.HasPrefix(rel, "internal/tui/"):
		return true, strings.HasPrefix(rel, "internal/tui/theme/"), rel == "internal/tui/theme/icons.go"
	case strings.HasPrefix(rel, "cmd/tui_") && strings.Count(rel, "/") == 1:
		return true, false, false
	}
	return false, false, false
}

// uiCounts is the number of shortcuts per "<kind> <file>".
func uiCounts(t *testing.T) (map[string]int, map[string][]uiFinding) {
	t.Helper()
	fset := token.NewFileSet()
	counts := map[string]int{}
	found := map[string][]uiFinding{}
	for _, f := range goFiles(t) {
		guarded, inTheme, isIcons := uiGuarded(f.rel)
		if !guarded {
			continue
		}
		list, err := uiFindings(fset, f.path, nil, inTheme, isIcons)
		if err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		for _, fd := range list {
			k := fd.cat + " " + f.rel
			counts[k]++
			found[k] = append(found[k], fd)
		}
	}
	return counts, found
}

// readUIExceptions reads the frozen list: "<kind> <file> <count>", with # for
// comments.
func readUIExceptions(t *testing.T) map[string]int {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleRoot(t), "internal", "archtest", filepath.FromSlash(uiStyleExceptionsFile)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]int{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("%s:%d: expected \"<kind> <file> <count>\", got %q", uiStyleExceptionsFile, n, line)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil || count < 1 {
			t.Fatalf("%s:%d: bad count %q", uiStyleExceptionsFile, n, fields[2])
		}
		if _, ok := uiStyleAdvice[fields[0]]; !ok {
			t.Fatalf("%s:%d: unknown kind %q", uiStyleExceptionsFile, n, fields[0])
		}
		key := fields[0] + " " + fields[1]
		if _, dup := out[key]; dup {
			t.Fatalf("%s:%d: %q is listed twice", uiStyleExceptionsFile, n, key)
		}
		out[key] = count
	}
	return out
}

// TestUIStyleGuard: colors and symbols come from the theme; the shortcuts
// that exist are frozen and can only go down.
func TestUIStyleGuard(t *testing.T) {
	counts, found := uiCounts(t)
	frozen := readUIExceptions(t)
	if len(counts) == 0 && len(frozen) == 0 {
		t.Fatal("the guard found nothing under internal/tui and cmd/tui_*: it checks nothing any more")
	}

	keys := map[string]bool{}
	for k := range counts {
		keys[k] = true
	}
	for k := range frozen {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		cat, file, _ := strings.Cut(k, " ")
		got, want := counts[k], frozen[k]
		switch {
		case got > want:
			var lines []string
			for _, fd := range found[k] {
				lines = append(lines, fmt.Sprintf("%s:%d %s", file, fd.line, strings.TrimSpace(fd.text)))
			}
			t.Errorf("%s: %d %s shortcut(s), %d accepted — %s (the list must not grow)\n\t%s",
				file, got, cat, want, uiStyleAdvice[cat], strings.Join(lines, "\n\t"))
		case got < want && got == 0:
			t.Errorf("%s: no %s shortcut left — remove the line \"%s %s %d\" from %s", file, cat, cat, file, want, uiStyleExceptionsFile)
		case got < want:
			t.Errorf("%s: %d %s shortcut(s) left, %d listed — write \"%s %s %d\" in %s", file, got, cat, want, cat, file, got, uiStyleExceptionsFile)
		}
	}
}

// The list has no line for a file that no longer exists or is not guarded.
func TestUIStyleExceptionsPointToGuardedFiles(t *testing.T) {
	for k := range readUIExceptions(t) {
		_, file, _ := strings.Cut(k, " ")
		if guarded, _, _ := uiGuarded(file); !guarded {
			t.Errorf("%s is listed in %s but is not under the guard", file, uiStyleExceptionsFile)
		}
		if _, err := os.Stat(filepath.Join(moduleRoot(t), filepath.FromSlash(file))); err != nil {
			t.Errorf("%s is listed in %s but does not exist", file, uiStyleExceptionsFile)
		}
	}
}

// The detection sees each shortcut, and only them (comments are no code).
func TestUIStyleDetection(t *testing.T) {
	src := `package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// "[#64a0ff]" and ✔ in a comment are not code
func f(hex string, c tcell.Color) {
	_ = "[#64a0ff]text"
	_ = "[red]text[-]"
	_ = "[red:blue]"
	_ = fmt.Sprintf("[%s]text[-]", hex)
	_ = "[" + hex + "]text"
	_ = "[::b]bold[::-]"
	_ = "[:#313244]"
	_ = tcell.ColorRed
	_ = tcell.ColorDefault
	_ = tcell.NewRGBColor(1, 2, 3)
	_ = tcell.GetColor("red")
	_ = tcell.GetColor(hex)
	_ = tcell.GetColor("#ff0000")
	_ = "#ff0000"
	_ = widgets.ColorTag(c)
	_ = ColorTag(c)
	_ = "✔ done"
	_ = "[" + hex + "]✓[-] done"
	_ = fmt.Sprintf("%s✓%s %s", "", "", "")
	_ = "⚠"
	_ = '◉'
	_ = "–"

	// not shortcuts
	_ = "[-]"
	_ = "[-:-:-]"
	_ = "[error] failed"
	_ = "[note]"
	_ = "a – b"
	_ = "✔done"
	_ = "[::-]"
	_ = fmt.Sprintf("[%d]", 3)
	_ = "x[0]"
}
`
	got, err := uiFindings(token.NewFileSet(), "p.go", src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, f := range got {
		count[f.cat]++
	}
	want := map[string]int{
		catTagColor:     6, // [#64a0ff], [red], [red:blue], [%s], "["+hex+"]", "["+hex+"]✓"
		catTagStyle:     3, // [red:blue] background, [::b], [:#313244]
		catColorLiteral: 5, // tcell.ColorRed, tcell.NewRGBColor, tcell.GetColor("red"), "#ff0000" twice
		catWidgetsTag:   2,
		catSymbol:       6, // ✔ done, ✓ (concatenated), ✓ (fmt), ⚠, ◉, – alone
	}
	for cat, n := range want {
		if count[cat] != n {
			t.Errorf("%s: %d found, %d expected", cat, count[cat], n)
			for _, f := range got {
				if f.cat == cat {
					t.Logf("  line %d: %s", f.line, f.text)
				}
			}
		}
	}
}
