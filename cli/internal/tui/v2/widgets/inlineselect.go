package widgets

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// InlineSelect — a FormItem that displays all options inline with ↑↓ navigation
// ─────────────────────────────────────────────────────────────────────────────

// InlineSelect is a custom form item that shows all options as a vertical list.
// The user navigates with ↑↓ and confirms with Enter or Tab.
// It implements tview.FormItem and can be added to a Form via AddFormItem().
type InlineSelect struct {
	*tview.Box

	label        string
	options      []string
	descriptions []string // optional: one description per option, shown below each option in muted color
	selected     int
	onChange     func(value string, idx int)
	finishedFunc func(key tcell.Key)
	hasFocus     bool
	disabled     bool
	centered     bool // when true, the entire block (label + options + descriptions) is horizontally centered

	// Style
	labelWidth int
	labelColor tcell.Color
	bgColor    tcell.Color
	fieldText  tcell.Color
	fieldBg    tcell.Color
}

// NewInlineSelect creates a new inline select form item.
// options are the display labels. defaultIdx is the initially selected index.
// onChange is called whenever the selection changes.
func NewInlineSelect(label string, options []string, defaultIdx int, onChange func(value string, idx int)) *InlineSelect {
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	s := &InlineSelect{
		Box:        tview.NewBox(),
		label:      label,
		options:    options,
		selected:   defaultIdx,
		onChange:   onChange,
		labelColor: theme.FgPrimary,
		bgColor:    theme.BgPanel,
		fieldText:  theme.FgPrimary,
		fieldBg:    theme.BgPanel,
	}
	s.SetBackgroundColor(theme.BgPanel)
	return s
}

// SetDescriptions adds descriptions displayed below each option in muted color.
// The slice length must match the options length; extra entries are ignored.
func (s *InlineSelect) SetDescriptions(descriptions []string) *InlineSelect {
	s.descriptions = descriptions
	return s
}

// SetCentered enables horizontal centering of the entire block (label, options,
// descriptions) within the available width. When false (default), content is
// drawn from the left edge with a small indent.
func (s *InlineSelect) SetCentered(centered bool) *InlineSelect {
	s.centered = centered
	return s
}

// hasDescriptions returns true when descriptions are configured.
func (s *InlineSelect) hasDescriptions() bool {
	return len(s.descriptions) > 0 && len(s.descriptions) >= len(s.options)
}

// GetLabel returns the item's label.
func (s *InlineSelect) GetLabel() string {
	return s.label
}

// SetFormAttributes sets form-level attributes.
func (s *InlineSelect) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	s.labelWidth = labelWidth
	s.labelColor = labelColor
	s.bgColor = bgColor
	s.fieldText = fieldTextColor
	s.fieldBg = fieldBgColor
	s.SetBackgroundColor(bgColor)
	return s
}

// GetFieldWidth returns 0 (flexible width).
func (s *InlineSelect) GetFieldWidth() int {
	return 0
}

// GetFieldHeight returns the number of lines needed for all options.
// When descriptions are set, each option takes 1 + descLines lines
// (option label + description lines) plus a blank separator between options.
// Descriptions may contain newlines for multi-line text.
func (s *InlineSelect) GetFieldHeight() int {
	n := len(s.options)
	if !s.hasDescriptions() {
		return n
	}
	total := 0
	for i := range s.options {
		total++ // option label line
		if i < len(s.descriptions) && s.descriptions[i] != "" {
			total += strings.Count(s.descriptions[i], "\n") + 1 // description lines
		}
		if i < n-1 {
			total++ // blank separator between options
		}
	}
	return total
}

// SetFinishedFunc sets the handler called when the user confirms selection.
func (s *InlineSelect) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	s.finishedFunc = handler
	return s
}

// SetDisabled sets the disabled state.
func (s *InlineSelect) SetDisabled(disabled bool) tview.FormItem {
	s.disabled = disabled
	return s
}

// GetSelectedIndex returns the currently selected index.
func (s *InlineSelect) GetSelectedIndex() int {
	return s.selected
}

// GetSelectedValue returns the currently selected option string.
func (s *InlineSelect) GetSelectedValue() string {
	if s.selected >= 0 && s.selected < len(s.options) {
		return s.options[s.selected]
	}
	return ""
}

// Focus is called when this primitive receives focus.
func (s *InlineSelect) Focus(delegate func(p tview.Primitive)) {
	s.hasFocus = true
	s.Box.Focus(delegate)
}

// Blur is called when this primitive loses focus.
func (s *InlineSelect) Blur() {
	s.hasFocus = false
	s.Box.Blur()
}

// HasFocus returns whether or not this primitive has focus.
func (s *InlineSelect) HasFocus() bool {
	return s.hasFocus
}

// Draw renders the inline select.
func (s *InlineSelect) Draw(screen tcell.Screen) {
	s.DrawForSubclass(screen, s)
	x, y, width, _ := s.GetInnerRect()

	// ── Compute centering offset ──
	padLeft := 0
	if s.centered {
		maxW := utf8.RuneCountInString(s.label)
		for _, opt := range s.options {
			w := 4 + utf8.RuneCountInString(opt) // indent(2) + cursor(1) + space(1) + text
			if w > maxW {
				maxW = w
			}
		}
		if s.hasDescriptions() {
			for _, desc := range s.descriptions {
				for _, line := range strings.Split(desc, "\n") {
					w := 6 + utf8.RuneCountInString(line) // desc indent
					if w > maxW {
						maxW = w
					}
				}
			}
		}
		if maxW < width {
			padLeft = (width - maxW) / 2
		}
	}

	// ── Draw label on the first line ──
	if s.label != "" {
		labelStyle := tcell.StyleDefault.Background(s.bgColor).Foreground(s.labelColor)
		col := x + padLeft
		for _, ch := range s.label {
			if col >= x+width {
				break
			}
			screen.SetContent(col, y, ch, nil, labelStyle)
			col++
		}
		y++
	}

	hasDesc := s.hasDescriptions()
	row := 0

	// ── Draw options ──
	for i, opt := range s.options {
		cursorColor := theme.FgMuted
		textColor := s.fieldText
		bg := s.fieldBg

		if i == s.selected {
			if s.hasFocus {
				cursorColor = theme.ActiveMode.Primary
				textColor = theme.FgPrimary
				bg = theme.BgElement
			} else {
				cursorColor = theme.FgSecondary
				textColor = theme.FgPrimary
			}
		}

		rowStyle := tcell.StyleDefault.Background(bg).Foreground(textColor)
		cursorStyle := tcell.StyleDefault.Background(bg).Foreground(cursorColor)

		// Option label line
		col := x + padLeft + 2 // indent
		if i == s.selected {
			screen.SetContent(col, y+row, '›', nil, cursorStyle)
		} else {
			screen.SetContent(col, y+row, ' ', nil, rowStyle)
		}
		col++
		screen.SetContent(col, y+row, ' ', nil, rowStyle)
		col++

		for _, ch := range opt {
			if col >= x+width {
				break
			}
			screen.SetContent(col, y+row, ch, nil, rowStyle)
			col++
		}
		// Fill rest of line
		for col < x+width {
			screen.SetContent(col, y+row, ' ', nil, rowStyle)
			col++
		}
		row++

		// Description lines (multi-line support via \n splitting)
		if hasDesc && i < len(s.descriptions) && s.descriptions[i] != "" {
			descStyle := tcell.StyleDefault.Background(bg).Foreground(theme.FgMuted)
			for _, descLine := range strings.Split(s.descriptions[i], "\n") {
				col = x + padLeft + 6 // extra indent for description
				for _, ch := range descLine {
					if col >= x+width {
						break
					}
					screen.SetContent(col, y+row, ch, nil, descStyle)
					col++
				}
				for col < x+width {
					screen.SetContent(col, y+row, ' ', nil, descStyle)
					col++
				}
				row++
			}

			// Blank separator between options (except after last)
			if i < len(s.options)-1 {
				bgStyle := tcell.StyleDefault.Background(s.fieldBg)
				for col := x; col < x+width; col++ {
					screen.SetContent(col, y+row, ' ', nil, bgStyle)
				}
				row++
			}
		}
	}
}

// InputHandler returns the handler for this primitive.
func (s *InlineSelect) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if s.disabled {
			return
		}

		switch event.Key() {
		case tcell.KeyUp:
			if s.selected > 0 {
				s.selected--
				if s.onChange != nil {
					s.onChange(s.options[s.selected], s.selected)
				}
			}
		case tcell.KeyDown:
			if s.selected < len(s.options)-1 {
				s.selected++
				if s.onChange != nil {
					s.onChange(s.options[s.selected], s.selected)
				}
			}
		case tcell.KeyEnter, tcell.KeyTab:
			if s.finishedFunc != nil {
				s.finishedFunc(tcell.KeyTab)
			}
		case tcell.KeyBacktab:
			if s.finishedFunc != nil {
				s.finishedFunc(tcell.KeyBacktab)
			}
		case tcell.KeyEscape:
			if s.finishedFunc != nil {
				s.finishedFunc(tcell.KeyEscape)
			}
		default:
			// Handle rune keys for vim-style navigation
			switch event.Rune() {
			case 'j':
				if s.selected < len(s.options)-1 {
					s.selected++
					if s.onChange != nil {
						s.onChange(s.options[s.selected], s.selected)
					}
				}
			case 'k':
				if s.selected > 0 {
					s.selected--
					if s.onChange != nil {
						s.onChange(s.options[s.selected], s.selected)
					}
				}
			}
		}
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (s *InlineSelect) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		if s.disabled {
			return false, nil
		}

		x, y, _, _ := s.GetInnerRect()
		mx, my := event.Position()

		if action == tview.MouseLeftClick {
			// Check if click is within options area
			optionY := my - y
			if s.label != "" {
				optionY-- // account for label row
			}

			// Map pixel row to option index.
			optIdx := -1
			if s.hasDescriptions() {
				// With descriptions: each option occupies 3 rows (opt+desc+blank),
				// except the last which occupies 2 (opt+desc).
				linesPerOpt := 3
				optIdx = optionY / linesPerOpt
				if optIdx >= len(s.options) {
					optIdx = len(s.options) - 1
				}
			} else {
				optIdx = optionY
			}

			if optIdx >= 0 && optIdx < len(s.options) && mx >= x {
				s.selected = optIdx
				if s.onChange != nil {
					s.onChange(s.options[s.selected], s.selected)
				}
				setFocus(s)
				return true, nil
			}
		}
		return false, nil
	})
}
