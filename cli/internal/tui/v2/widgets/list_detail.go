package widgets

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ListDetailPage is the shared layout of the list pages (catalogues,
// configuration): the main panel on the left (3/5), framed in the colour of
// the active mode, and a detail pane on the right (2/5) with a muted frame.
// The left panel stacks optional header rows above the main primitive
// (usually a SectionedList).
type ListDetailPage struct {
	Root   *tview.Flex
	Left   *tview.Flex
	Detail *tview.TextView
}

// NewListDetailPage builds the layout; title is the left frame title,
// detailTitle the detail frame title.
func NewListDetailPage(title, detailTitle string) *ListDetailPage {
	p := &ListDetailPage{}
	p.Left = tview.NewFlex().SetDirection(tview.FlexRow)
	p.Left.SetBackgroundColor(theme.BgPanel)
	p.Left.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	p.Left.SetBorderColor(theme.ActiveMode.Primary)
	p.Left.SetBorderPadding(0, 0, 1, 1)
	p.SetTitle(title)

	p.Detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	p.Detail.SetBackgroundColor(theme.BgPanel)
	p.Detail.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	p.Detail.SetBorderColor(theme.FgMuted)
	p.Detail.SetBorderPadding(0, 0, 1, 1)
	p.SetDetailTitle(detailTitle)

	p.Root = tview.NewFlex().
		AddItem(p.Left, 0, 3, true).
		AddItem(p.Detail, 0, 2, false)
	p.Root.SetBackgroundColor(theme.BgPanel)
	return p
}

// SetTitle sets the title of the left frame.
func (p *ListDetailPage) SetTitle(title string) *ListDetailPage {
	if title != "" {
		title = " " + title + " "
	}
	p.Left.SetTitle(title)
	return p
}

// SetDetailTitle sets the title of the detail frame.
func (p *ListDetailPage) SetDetailTitle(title string) *ListDetailPage {
	if title != "" {
		title = " " + title + " "
	}
	p.Detail.SetTitle(title)
	return p
}

// AddHeader adds a fixed-height row above the main primitive.
func (p *ListDetailPage) AddHeader(prim tview.Primitive, rows int) *ListDetailPage {
	p.Left.AddItem(prim, rows, 0, false)
	return p
}

// SetMain sets the main primitive of the left panel (it keeps the headers).
func (p *ListDetailPage) SetMain(prim tview.Primitive) *ListDetailPage {
	p.Left.AddItem(prim, 0, 1, true)
	return p
}

// AddFooter adds a fixed-height row below the main primitive (call after
// SetMain).
func (p *ListDetailPage) AddFooter(prim tview.Primitive, rows int) *ListDetailPage {
	p.Left.AddItem(prim, rows, 0, false)
	return p
}

// SetRight replaces the right column by prim (which usually holds Detail,
// e.g. stacked with another pane).
func (p *ListDetailPage) SetRight(prim tview.Primitive) *ListDetailPage {
	p.Root.Clear()
	p.Root.AddItem(p.Left, 0, 3, true).AddItem(prim, 0, 2, false)
	return p
}

// SetDetail replaces the detail text.
func (p *ListDetailPage) SetDetail(text string) {
	p.Detail.SetText(text)
	p.Detail.ScrollToBeginning()
}

// Mount puts the layout in content (cleared first).
func (p *ListDetailPage) Mount(content *tview.Flex) {
	content.Clear()
	content.AddItem(p.Root, 0, 1, true)
}
