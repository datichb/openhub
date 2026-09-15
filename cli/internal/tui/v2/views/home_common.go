package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Common helpers shared by the 3 home views (HomeView, ProjectModeView,
// TeamModeView). Extracted to reduce duplication (audit Phase 3.2).
// ─────────────────────────────────────────────────────────────────────────────

// homeHandleKey processes common key events for home-like views.
// It handles dual-column navigation (h/l/Tab) and Enter selection.
// Returns nil if the event was consumed, or the original event if not.
// The executeRef callback is invoked with the selected item's Reference.(int).
func homeHandleKey(
	event *tcell.EventKey,
	list *widgets.SectionedList,
	dual *homeDualLayout,
	executeRef func(int),
) *tcell.EventKey {
	if list == nil {
		return event
	}

	// Dual-column navigation (h/l/Tab)
	if dual != nil {
		if consumed := dual.HandleKey(event); consumed == nil {
			return nil
		}
	}

	if event.Key() == tcell.KeyEnter {
		active := list
		if dual != nil {
			active = dual.activeList()
		}
		if _, item, ok := active.CurrentItem(); ok {
			if ref, refOk := item.Reference.(int); refOk {
				executeRef(ref)
			}
		}
		return nil
	}

	return event
}

// homeSectionItem describes a generic home-page menu item with an icon,
// label, description, and a structural section ID for split logic.
type homeSectionItem struct {
	Icon      string // "─" for section headers
	Label     string
	Desc      string
	SectionID string // structural ID (headers only, used for split)
}

// toSectionItem converts a homeSectionItem to a widgets.SectionItem.
// idx is the position in the original items slice, stored as Reference.
func toSectionItem(it homeSectionItem, idx int) widgets.SectionItem {
	if it.Icon == "─" {
		return widgets.SectionItem{
			MainText: it.Label,
			IsHeader: true,
		}
	}
	return widgets.SectionItem{
		MainText:      fmt.Sprintf("%s  %s", it.Icon, it.Label),
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}

// splitBySectionID distributes items into left and right columns.
// All items from the first section matching splitAfterID (inclusive) go right.
func splitBySectionID(
	items []homeSectionItem,
	splitAfterID string,
	convert func(homeSectionItem, int) widgets.SectionItem,
) (left, right []widgets.SectionItem) {
	configIdx := -1
	for i, it := range items {
		if it.SectionID == splitAfterID {
			configIdx = i
			break
		}
	}

	for i, it := range items {
		si := convert(it, i)
		if configIdx >= 0 && i >= configIdx {
			right = append(right, si)
		} else {
			left = append(left, si)
		}
	}
	return
}
