package widgets

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// CardColumn — kanban column rendering tickets as individual bordered cards
// ─────────────────────────────────────────────────────────────────────────────

// Card represents a single kanban card within a CardColumn.
type Card struct {
	// MainText is the primary line (title). Supports tview dynamic color tags.
	MainText string
	// SecondaryText is the second line (ID, type). Supports tview dynamic color tags.
	SecondaryText string
	// MetaText is the third line (labels, assignee, refs). Supports tview dynamic color tags.
	// If empty, the card renders 2 content lines instead of 3.
	MetaText string
	// Reference is an opaque value for caller use (e.g. ticket ID).
	Reference any
}

// cardHeight returns how many screen rows this card occupies (border + content + spacing).
func (c *Card) contentLines() int {
	n := 2 // MainText + SecondaryText always present
	if c.MetaText != "" {
		n = 3
	}
	return n
}

// CardColumn is a tview.Primitive that renders a kanban column with card-styled
// ticket items. Each card has a rounded border, elevated background, and spacing.
//
// Visual layout (3-line card):
//
//	COLUMN NAME (3)
//	╭──────────────────────╮
//	│ P1 · Fix auth bug    │
//	│ BD-001 · bug         │
//	│ ← gitlab-42          │
//	╰──────────────────────╯
//
//	╭──────────────────────╮
//	│ P0 · Critical issue  │
//	│ BD-002 · bug         │
//	╰──────────────────────╯
type CardColumn struct {
	*tview.Box
	title          string
	titleColor     tcell.Color
	cards          []Card
	selected       int
	offset         int  // scroll offset (index of first visible card)
	focused        bool // whether this column currently has visual focus
	showLeftArrow  bool // show ◄ in header (columns to the left)
	showRightArrow bool // show ► in header (columns to the right)
}

// NewCardColumn creates a new kanban column with the given title and color.
func NewCardColumn(title string, titleColor tcell.Color) *CardColumn {
	c := &CardColumn{
		Box:        tview.NewBox(),
		title:      title,
		titleColor: titleColor,
		selected:   0,
		offset:     0,
	}
	c.SetBackgroundColor(theme.BgPanel)
	return c
}

// ─── Public API ──────────────────────────────────────────────────────────────

// AddCard appends a card to the column.
func (c *CardColumn) AddCard(card Card) *CardColumn {
	c.cards = append(c.cards, card)
	return c
}

// SetCards replaces all cards in the column.
func (c *CardColumn) SetCards(cards []Card) *CardColumn {
	c.cards = cards
	c.selected = 0
	c.offset = 0
	return c
}

// Clear removes all cards.
func (c *CardColumn) Clear() *CardColumn {
	c.cards = nil
	c.selected = 0
	c.offset = 0
	return c
}

// SetCurrentItem sets the selected card index, clamping to valid bounds.
func (c *CardColumn) SetCurrentItem(idx int) *CardColumn {
	if len(c.cards) == 0 {
		c.selected = 0
		return c
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(c.cards) {
		idx = len(c.cards) - 1
	}
	c.selected = idx
	return c
}

// GetCurrentItem returns the index of the selected card (-1 if empty).
func (c *CardColumn) GetCurrentItem() int {
	if len(c.cards) == 0 {
		return -1
	}
	return c.selected
}

// GetItemCount returns the number of cards.
func (c *CardColumn) GetItemCount() int {
	return len(c.cards)
}

// GetItemText returns the main and secondary text of the card at idx.
func (c *CardColumn) GetItemText(idx int) (mainText, secondaryText string) {
	if idx < 0 || idx >= len(c.cards) {
		return "", ""
	}
	return c.cards[idx].MainText, c.cards[idx].SecondaryText
}

// GetItemMeta returns the meta text of the card at idx.
func (c *CardColumn) GetItemMeta(idx int) string {
	if idx < 0 || idx >= len(c.cards) {
		return ""
	}
	return c.cards[idx].MetaText
}

// SetFocused sets the visual focus indicator (changes card border color).
func (c *CardColumn) SetFocused(f bool) *CardColumn {
	c.focused = f
	return c
}

// SetNavArrows controls the ◄/► directional arrows in the column header.
// Arrows are only rendered when the column has focus — they indicate that
// the user can navigate to adjacent columns with h/l keys.
func (c *CardColumn) SetNavArrows(left, right bool) *CardColumn {
	c.showLeftArrow = left
	c.showRightArrow = right
	return c
}

// MoveSelection moves the selection by delta (+1 = down, -1 = up).
func (c *CardColumn) MoveSelection(delta int) {
	if len(c.cards) == 0 {
		return
	}
	c.selected += delta
	if c.selected < 0 {
		c.selected = 0
	}
	if c.selected >= len(c.cards) {
		c.selected = len(c.cards) - 1
	}
}

// ─── Primitive interface ─────────────────────────────────────────────────────

// Draw renders the column: header, then a vertical stack of bordered cards.
func (c *CardColumn) Draw(screen tcell.Screen) {
	c.DrawForSubclass(screen, c)
	x, y, width, height := c.GetInnerRect()

	if width < 4 || height < 2 {
		return
	}

	bgPanel := theme.BgPanel

	// ── Header: " ◄ TITLE (N) ►" (arrows only when focused) ──
	headerStyle := tcell.StyleDefault.Background(bgPanel).Foreground(c.titleColor).Bold(true)
	arrowStyle := tcell.StyleDefault.Background(bgPanel).Foreground(theme.FgSecondary)

	// Build header text
	headerText := " "
	if c.focused && c.showLeftArrow {
		headerText += "◄ "
	}
	headerText += c.title
	if len(c.cards) > 0 {
		headerText += fmt.Sprintf(" (%d)", len(c.cards))
	}
	// Draw the base header text
	c.drawText(screen, x, y, width, headerText, headerStyle)

	// Overlay arrows in a different style (secondary color, not bold)
	if c.focused {
		dx := 0
		for _, r := range headerText {
			if dx < width && (r == '◄' || r == '►') {
				screen.SetContent(x+dx, y, r, nil, arrowStyle)
			}
			dx += runewidth.RuneWidth(r)
		}
		// Draw right arrow at the end of the line
		if c.showRightArrow {
			// Place ► after the header text with a space, or at rightmost position
			pos := runewidth.StringWidth(headerText) + 1
			if pos < width-1 {
				screen.SetContent(x+pos, y, '►', nil, arrowStyle)
			}
		}
	}
	y++
	height--

	// ── Thin separator line ──
	sepStyle := tcell.StyleDefault.Background(bgPanel).Foreground(theme.BorderCard)
	for dx := 0; dx < width; dx++ {
		screen.SetContent(x+dx, y, '─', nil, sepStyle)
	}
	y++
	height--

	// ── Blank line after header ──
	blankStyle := tcell.StyleDefault.Background(bgPanel)
	for dx := 0; dx < width; dx++ {
		screen.SetContent(x+dx, y, ' ', nil, blankStyle)
	}
	y++
	height--

	if len(c.cards) == 0 || height < 1 {
		// Fill remaining area with panel background
		c.fillRect(screen, x, y, width, height, bgPanel)
		return
	}

	// ── Ensure scroll offset keeps selected card visible ──
	c.adjustScroll(height)

	// ── Render visible cards ──
	cy := y
	for i := c.offset; i < len(c.cards) && cy < y+height; i++ {
		card := &c.cards[i]
		contentLines := card.contentLines()
		cardHeight := contentLines + 2 // +2 for top/bottom border
		spacingAfter := 1              // 1 blank line between cards

		if cy+cardHeight > y+height {
			// Not enough room for the full card — don't draw partial cards
			break
		}

		isSelected := c.focused && i == c.selected

		c.drawCard(screen, x, cy, width, card, contentLines, isSelected)
		cy += cardHeight

		// Spacing between cards
		if cy < y+height {
			for dx := 0; dx < width; dx++ {
				screen.SetContent(x+dx, cy, ' ', nil, blankStyle)
			}
			cy += spacingAfter
		}
	}

	// Fill remaining vertical space
	if cy < y+height {
		c.fillRect(screen, x, cy, width, y+height-cy, bgPanel)
	}
}

// drawCard renders a single card with rounded borders.
func (c *CardColumn) drawCard(screen tcell.Screen, x, y, maxWidth int, card *Card, contentLines int, selected bool) {
	bgCard := theme.BgCard
	borderColor := theme.BorderCard
	if selected {
		borderColor = theme.ActiveMode.Primary
	}

	borderStyle := tcell.StyleDefault.Background(theme.BgPanel).Foreground(borderColor)
	textPrimaryStyle := tcell.StyleDefault.Background(bgCard).Foreground(theme.FgPrimary)
	textMutedStyle := tcell.StyleDefault.Background(bgCard).Foreground(theme.FgMuted)
	bgPanelStyle := tcell.StyleDefault.Background(theme.BgPanel)

	// Card occupies columns [x+1 .. x+maxWidth-2] to leave margin on each side.
	// Minimum: cardX = x+1, cardWidth = maxWidth - 2.
	marginL := 1
	marginR := 1
	cardX := x + marginL
	cardW := maxWidth - marginL - marginR
	if cardW < 3 {
		cardW = maxWidth
		cardX = x
		marginL = 0
		marginR = 0
	}
	innerW := cardW - 2 // inside the border (left border + right border)

	// ── Left/right margin fills (panel bg) ──
	totalH := contentLines + 2
	for row := 0; row < totalH; row++ {
		for m := 0; m < marginL; m++ {
			screen.SetContent(x+m, y+row, ' ', nil, bgPanelStyle)
		}
		for m := 0; m < marginR; m++ {
			screen.SetContent(x+maxWidth-1-m, y+row, ' ', nil, bgPanelStyle)
		}
	}

	// ── Top border: ╭───╮ ──
	screen.SetContent(cardX, y, '╭', nil, borderStyle)
	for dx := 1; dx < cardW-1; dx++ {
		screen.SetContent(cardX+dx, y, '─', nil, borderStyle)
	}
	screen.SetContent(cardX+cardW-1, y, '╮', nil, borderStyle)

	// ── Content lines ──
	lines := []struct {
		text  string
		style tcell.Style
	}{
		{card.MainText, textPrimaryStyle},
		{card.SecondaryText, textMutedStyle},
	}
	if card.MetaText != "" {
		lines = append(lines, struct {
			text  string
			style tcell.Style
		}{card.MetaText, textMutedStyle})
	}

	for li, line := range lines {
		row := y + 1 + li
		screen.SetContent(cardX, row, '│', nil, borderStyle)
		// 1 char padding inside border
		screen.SetContent(cardX+1, row, ' ', nil, tcell.StyleDefault.Background(bgCard))
		c.drawColoredText(screen, cardX+2, row, innerW-2, line.text, line.style)
		screen.SetContent(cardX+cardW-1, row, '│', nil, borderStyle)
	}

	// ── Bottom border: ╰───╯ ──
	bottomY := y + 1 + contentLines
	screen.SetContent(cardX, bottomY, '╰', nil, borderStyle)
	for dx := 1; dx < cardW-1; dx++ {
		screen.SetContent(cardX+dx, bottomY, '─', nil, borderStyle)
	}
	screen.SetContent(cardX+cardW-1, bottomY, '╯', nil, borderStyle)
}

// drawText writes plain text at (x, y) with the given style, padded to width.
// Positions are counted in display cells (wide runes take two).
func (c *CardColumn) drawText(screen tcell.Screen, x, y, width int, text string, style tcell.Style) {
	dx := 0
	for _, r := range text {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			continue
		}
		if dx+w > width {
			break
		}
		screen.SetContent(x+dx, y, r, nil, style)
		dx += w
	}
	for ; dx < width; dx++ {
		screen.SetContent(x+dx, y, ' ', nil, style)
	}
}

// drawColoredText writes text supporting tview dynamic color tags at (x, y).
// Falls back to raw rune output for text without tags.
// Pads remaining space with bgCard background.
func (c *CardColumn) drawColoredText(screen tcell.Screen, x, y, maxW int, text string, defaultStyle tcell.Style) {
	bgCard := theme.BgCard

	// Parse tview color tags and render manually.
	// Tags have the form [color], [color:bg], [color:bg:attr], [-], [-:-:-].
	// Like tview, "[text[]" is an escaped literal "[text]"; any other bracket
	// content that is not a valid tag (e.g. "[!]", "[2/3]") is plain text.
	runes := []rune(text)
	dx := 0
	style := defaultStyle
	put := func(r rune) bool {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			return true
		}
		if dx+w > maxW {
			return false
		}
		screen.SetContent(x+dx, y, r, nil, style)
		dx += w
		return true
	}
	i := 0
	for i < len(runes) && dx < maxW {
		if runes[i] == '[' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == ']' {
					end = j
					break
				}
			}
			if end > i {
				tagContent := string(runes[i+1 : end])
				if strings.HasSuffix(tagContent, "[") {
					literal := append([]rune{'['}, runes[i+1:end-1]...)
					literal = append(literal, ']')
					stopped := false
					for _, r := range literal {
						if !put(r) {
							stopped = true
							break
						}
					}
					if stopped {
						break
					}
					i = end + 1
					continue
				}
				if newStyle, ok := c.parseColorTag(tagContent, defaultStyle); ok {
					style = newStyle.Background(bgCard)
					i = end + 1
					continue
				}
			}
		}
		if !put(runes[i]) {
			break
		}
		i++
	}
	// Pad remaining
	padStyle := tcell.StyleDefault.Background(bgCard)
	for dx < maxW {
		screen.SetContent(x+dx, y, ' ', nil, padStyle)
		dx++
	}
}

// parseColorTag interprets a tview color tag content (between [ and ]).
// Returns the new style and true, or zero-value and false if not a valid tag.
func (c *CardColumn) parseColorTag(tag string, defaultStyle tcell.Style) (tcell.Style, bool) {
	if tag == "-" || tag == "-:-:-" || tag == "-:-" {
		return defaultStyle, true
	}
	if !validColorTag(tag) {
		return tcell.StyleDefault, false
	}

	// Handle simple foreground: [#rrggbb] or [color]
	if len(tag) == 7 && tag[0] == '#' {
		color := tcell.GetColor(tag)
		return tcell.StyleDefault.Foreground(color), true
	}

	// Handle [fg:bg] format
	if tag != "" {
		// Split by ':'
		parts := splitTag(tag)
		style := defaultStyle
		if len(parts) >= 1 && parts[0] != "-" && parts[0] != "" {
			style = style.Foreground(tcell.GetColor(parts[0]))
		}
		if len(parts) >= 2 && parts[1] != "-" && parts[1] != "" {
			style = style.Background(tcell.GetColor(parts[1]))
		}
		if len(parts) >= 3 {
			for _, r := range parts[2] {
				switch r {
				case 'b':
					style = style.Bold(true)
				case 'i':
					style = style.Italic(true)
				case 'u':
					style = style.Underline(true)
				case '-':
					style = defaultStyle
				}
			}
		}
		return style, true
	}

	return tcell.StyleDefault, false
}

// validColorTag reports whether tag (the text between [ and ]) is a tview
// style tag: up to three ':'-separated parts, foreground and background being
// empty, "-", a color name or "#rrggbb", the attributes empty, "-" or made of
// the letters l b i d r u s.
func validColorTag(tag string) bool {
	parts := strings.Split(tag, ":")
	if len(parts) > 3 {
		return false
	}
	for i, p := range parts {
		switch {
		case p == "" || p == "-":
		case i == 2:
			if strings.Trim(p, "lbidrus") != "" {
				return false
			}
		case p[0] == '#':
			if len(p) != 7 || strings.Trim(p[1:], "0123456789abcdefABCDEF") != "" {
				return false
			}
		default:
			for _, r := range p {
				if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
					return false
				}
			}
		}
	}
	return true
}

// splitTag splits a color tag by ':' (max 3 parts).
func splitTag(tag string) []string {
	var parts []string
	start := 0
	for i, r := range tag {
		if r == ':' {
			parts = append(parts, tag[start:i])
			start = i + 1
			if len(parts) == 2 {
				// Rest is the attribute part
				parts = append(parts, tag[start:])
				return parts
			}
		}
	}
	parts = append(parts, tag[start:])
	return parts
}

// fillRect fills a rectangular area with spaces on the given background color.
func (c *CardColumn) fillRect(screen tcell.Screen, x, y, w, h int, bg tcell.Color) {
	style := tcell.StyleDefault.Background(bg)
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, style)
		}
	}
}

// adjustScroll ensures the selected card is within the visible viewport.
func (c *CardColumn) adjustScroll(availableHeight int) {
	if len(c.cards) == 0 {
		c.offset = 0
		return
	}

	// Count how many cards fit starting from offset.
	// Each card = contentLines + 2 (border) + 1 (spacing).
	// But last visible card doesn't need trailing spacing.

	// Scroll down if selected is below viewport
	for {
		visible := c.visibleCount(availableHeight)
		if visible == 0 {
			break
		}
		if c.selected < c.offset+visible {
			break
		}
		c.offset++
	}

	// Scroll up if selected is above viewport
	if c.selected < c.offset {
		c.offset = c.selected
	}
}

// visibleCount returns how many cards fit starting from c.offset within the given height.
func (c *CardColumn) visibleCount(availableHeight int) int {
	used := 0
	count := 0
	for i := c.offset; i < len(c.cards); i++ {
		cardH := c.cards[i].contentLines() + 2 // border top + bottom
		need := cardH
		if count > 0 {
			need++ // spacing before this card
		}
		if used+need > availableHeight {
			break
		}
		used += need
		count++
	}
	return count
}

// InputHandler returns the input handler for keyboard navigation.
func (c *CardColumn) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return c.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyDown:
			c.MoveSelection(1)
		case tcell.KeyUp:
			c.MoveSelection(-1)
		case tcell.KeyHome:
			c.selected = 0
		case tcell.KeyEnd:
			if len(c.cards) > 0 {
				c.selected = len(c.cards) - 1
			}
		case tcell.KeyPgDn:
			c.MoveSelection(5)
		case tcell.KeyPgUp:
			c.MoveSelection(-5)
		default:
			switch event.Rune() {
			case 'j':
				c.MoveSelection(1)
			case 'k':
				c.MoveSelection(-1)
			case 'g':
				c.selected = 0
			case 'G':
				if len(c.cards) > 0 {
					c.selected = len(c.cards) - 1
				}
			}
		}
	})
}

// Focus is called when the column receives focus.
func (c *CardColumn) Focus(delegate func(p tview.Primitive)) {
	c.focused = true
	c.Box.Focus(delegate)
}

// Blur is called when the column loses focus.
func (c *CardColumn) Blur() {
	c.focused = false
	c.Box.Blur()
}

// HasFocus returns true if this column currently has focus.
func (c *CardColumn) HasFocus() bool {
	return c.focused
}

// MouseHandler returns the mouse handler.
func (c *CardColumn) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return c.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		if !c.InRect(event.Position()) {
			return false, nil
		}

		switch action {
		case tview.MouseLeftClick:
			setFocus(c)
			// Determine which card was clicked
			_, y, _, _ := c.GetInnerRect()
			_, mouseY := event.Position()
			cardIdx := c.cardIndexAtY(y, mouseY)
			if cardIdx >= 0 && cardIdx < len(c.cards) {
				c.selected = cardIdx
			}
			return true, nil
		case tview.MouseScrollDown:
			c.MoveSelection(1)
			return true, nil
		case tview.MouseScrollUp:
			c.MoveSelection(-1)
			return true, nil
		}

		return false, nil
	})
}

// cardIndexAtY returns the card index at the given screen Y coordinate.
func (c *CardColumn) cardIndexAtY(startY, mouseY int) int {
	// Header takes 3 rows (title + separator + blank)
	cy := startY + 3
	for i := c.offset; i < len(c.cards); i++ {
		cardH := c.cards[i].contentLines() + 2 // border
		if mouseY >= cy && mouseY < cy+cardH {
			return i
		}
		cy += cardH + 1 // card + spacing
	}
	return -1
}
