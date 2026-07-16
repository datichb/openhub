package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
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

// GetFieldHeight returns the number of options (all visible).
func (s *InlineSelect) GetFieldHeight() int {
	return len(s.options)
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
	s.Box.DrawForSubclass(screen, s)
	x, y, width, _ := s.GetInnerRect()

	// Draw label on the first line
	if s.label != "" {
		labelStyle := tcell.StyleDefault.Background(s.bgColor).Foreground(s.labelColor)
		lw := s.labelWidth
		if lw == 0 {
			lw = len(s.label) + 1
		}
		for i, ch := range s.label {
			if i >= width {
				break
			}
			screen.SetContent(x+i, y, ch, nil, labelStyle)
		}
		// Move to field area
		y++
	}

	// Draw options
	for i, opt := range s.options {
		if y+i >= y+len(s.options) {
			break
		}

		// Cursor indicator
		cursor := "  "
		cursorColor := theme.FgMuted
		textColor := s.fieldText
		bg := s.fieldBg

		if i == s.selected {
			if s.hasFocus {
				cursor = fmt.Sprintf("%s› ", colorTag(theme.Accent))
				cursorColor = theme.Accent
				textColor = theme.FgPrimary
				bg = theme.BgElement
			} else {
				cursor = "› "
				cursorColor = theme.FgSecondary
				textColor = theme.FgPrimary
			}
		}

		// Draw the row
		rowStyle := tcell.StyleDefault.Background(bg).Foreground(textColor)
		cursorStyle := tcell.StyleDefault.Background(bg).Foreground(cursorColor)

		col := x + 2 // indent
		// Draw cursor
		for _, ch := range cursor {
			if ch == '[' || ch == ']' || ch == '#' || ch == '-' {
				// Skip tview color tag chars in raw rendering
				continue
			}
			screen.SetContent(col, y+i, ch, nil, cursorStyle)
			col++
		}

		// Actually use a simpler approach: just draw "› " or "  "
		col = x + 2
		if i == s.selected {
			screen.SetContent(col, y+i, '›', nil, cursorStyle)
		} else {
			screen.SetContent(col, y+i, ' ', nil, rowStyle)
		}
		col++
		screen.SetContent(col, y+i, ' ', nil, rowStyle)
		col++

		// Draw option text
		for _, ch := range opt {
			if col >= x+width {
				break
			}
			screen.SetContent(col, y+i, ch, nil, rowStyle)
			col++
		}

		// Fill rest of line with background
		for col < x+width {
			screen.SetContent(col, y+i, ' ', nil, rowStyle)
			col++
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
			if optionY >= 0 && optionY < len(s.options) && mx >= x {
				s.selected = optionY
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
