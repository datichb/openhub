package widgets

import (
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
// When descriptions are set, each option takes 2 lines (option + description)
// plus a blank line between options for visual separation.
func (s *InlineSelect) GetFieldHeight() int {
	n := len(s.options)
	if s.hasDescriptions() {
		// 2 lines per option (label + desc) + (n-1) blank separators
		return n*2 + max(n-1, 0)
	}
	return n
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

	// Draw label on the first line
	if s.label != "" {
		labelStyle := tcell.StyleDefault.Background(s.bgColor).Foreground(s.labelColor)
		for i, ch := range s.label {
			if i >= width {
				break
			}
			screen.SetContent(x+i, y, ch, nil, labelStyle)
		}
		// Move to field area
		y++
	}

	hasDesc := s.hasDescriptions()
	row := 0 // current row offset from y

	// Draw options
	for i, opt := range s.options {
		// Cursor indicator
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

		// Draw the option row
		rowStyle := tcell.StyleDefault.Background(bg).Foreground(textColor)
		cursorStyle := tcell.StyleDefault.Background(bg).Foreground(cursorColor)

		col := x + 2 // indent
		if i == s.selected {
			screen.SetContent(col, y+row, '›', nil, cursorStyle)
		} else {
			screen.SetContent(col, y+row, ' ', nil, rowStyle)
		}
		col++
		screen.SetContent(col, y+row, ' ', nil, rowStyle)
		col++

		// Draw option text
		for _, ch := range opt {
			if col >= x+width {
				break
			}
			screen.SetContent(col, y+row, ch, nil, rowStyle)
			col++
		}

		// Fill rest of line with background
		for col < x+width {
			screen.SetContent(col, y+row, ' ', nil, rowStyle)
			col++
		}

		row++

		// Draw description below the option (if available)
		if hasDesc && i < len(s.descriptions) && s.descriptions[i] != "" {
			descStyle := tcell.StyleDefault.Background(bg).Foreground(theme.FgMuted)
			col = x + 6 // extra indent for description
			for _, ch := range s.descriptions[i] {
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
