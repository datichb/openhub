package views

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// « Démarrer » section of the landings (P1-T20, 10 §1–§3): pinned workflows,
// recent ones, then (project and team modes) the categories, collapsed. The
// entries come from the cache kept by the wiring layer (no I/O on the event
// loop). `*` on an entry pins or unpins it.

// StartScope selects the context of a landing (both empty = hub).
type StartScope struct {
	ProjectID string
	TeamID    string
}

// StartEntry is a workflow of the section.
type StartEntry struct {
	ID     string
	Label  string // short description
	Origin string // « hub », « équipe v3 »…
	Pinned bool
	// PinScope is the preference scope the entry is pinned in ("" = the
	// scope of the landing).
	PinScope string
	Ago      string // recents: « il y a 2 h »
}

// StartCategory groups the workflows of a category.
type StartCategory struct {
	ID      string
	Label   string
	Entries []StartEntry
}

// StartEntries is the content of the section for a scope.
type StartEntries struct {
	Pinned      []StartEntry
	Recents     []StartEntry
	Suggestions []StartEntry
	Categories  []StartCategory
	// Total counts the workflows of the catalogue.
	Total int
	// Loaded is false until the first load finished (shows « chargement »).
	Loaded bool
}

// StartSectionConfig is shared by the home, project and team landings.
type StartSectionConfig struct {
	// Entries returns the cached entries of a scope. Nil hides the section.
	Entries func(scope StartScope) StartEntries
	// Launch opens the launch form of a workflow.
	Launch func(scope StartScope, workflowID string)
	// TogglePin pins or unpins a workflow (`*`).
	TogglePin func(scope StartScope, entry StartEntry)
	// OpenCatalog shows the workflow catalogue.
	OpenCatalog func(scope StartScope)
	// FreeSession launches a session without workflow (opencode V1, or no
	// workflow in the catalogue). Nil hides the entry.
	FreeSession func(scope StartScope)
}

// startItem is an item of the section; Entry is set on workflow entries
// (target of `*`).
type startItem struct {
	Icon, Label, Desc string
	Entry             *StartEntry
	Action            func()
}

// startSection returns the header and the items of the section. collapsed
// adds the categories (project and team modes); pick shows a choice (the
// workflows of a category).
func startSection(cfg StartSectionConfig, scope StartScope, collapsed bool, pick func(title string, opts []SelectOption, onSelect func(string))) (header string, items []startItem, ok bool) {
	if cfg.Entries == nil {
		return "", nil, false
	}
	e := cfg.Entries(scope)
	launch := func(id string) func() {
		return func() {
			if cfg.Launch != nil {
				cfg.Launch(scope, id)
			}
		}
	}
	entry := func(icon string, en StartEntry, desc string) startItem {
		return startItem{Icon: icon, Label: en.ID, Desc: desc, Entry: &en, Action: launch(en.ID)}
	}
	describe := func(en StartEntry) string {
		d := en.Label
		if en.Origin != "" {
			if d != "" {
				d += " · "
			}
			d += en.Origin
		}
		return d
	}
	header = i18n.T("tui.start.section")
	for _, en := range e.Pinned {
		items = append(items, entry("★", en, describe(en)))
	}
	for _, en := range e.Suggestions {
		items = append(items, entry("▸", en, describe(en)))
	}
	if len(e.Recents) > 0 {
		items = append(items, startItem{Icon: "·", Label: i18n.T("tui.start.recents")})
		for _, en := range e.Recents {
			d := describe(en)
			if en.Ago != "" {
				d = en.Ago + " · " + d
			}
			items = append(items, entry("↺", en, d))
		}
	}
	if collapsed {
		for _, c := range e.Categories {
			c := c
			items = append(items, startItem{Icon: "▸", Label: fmt.Sprintf("%s (%d)", c.Label, len(c.Entries)),
				Desc: i18n.T("tui.start.category_desc"), Action: func() {
					if pick == nil {
						return
					}
					opts := make([]SelectOption, len(c.Entries))
					for i, en := range c.Entries {
						opts[i] = SelectOption{Label: en.ID + " — " + describe(en), Value: en.ID}
					}
					pick(c.Label, opts, func(id string) { launch(id)() })
				}})
		}
	}
	switch {
	case !e.Loaded:
		items = append(items, startItem{Icon: "…", Label: i18n.T("tui.start.loading")})
	case e.Total > 0:
		items = append(items, startItem{Icon: "…", Label: i18n.Tf("tui.start.all", e.Total), Desc: i18n.T("tui.start.all_desc"),
			Action: func() {
				if cfg.OpenCatalog != nil {
					cfg.OpenCatalog(scope)
				}
			}})
	}
	if cfg.FreeSession != nil && e.Loaded && e.Total == 0 {
		items = append(items, startItem{Icon: "💻", Label: i18n.T("tui.start.free"), Desc: i18n.T("tui.start.free_desc"),
			Action: func() { cfg.FreeSession(scope) }})
	}
	return header, items, true
}

// togglePinOf pins or unpins the workflow of an item (`*`); it reports
// whether the item was a workflow entry.
func togglePinOf(cfg StartSectionConfig, scope StartScope, it *startItem) bool {
	if it == nil || it.Entry == nil || cfg.TogglePin == nil {
		return false
	}
	cfg.TogglePin(scope, *it.Entry)
	return true
}
