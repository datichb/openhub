package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

// ExpectedSecret describes a token_key that is referenced in the configuration
// (hub.toml or project config) but may or may not have a value in the keychain.
type ExpectedSecret struct {
	Key    string // e.g. "gitlab-token"
	Source string // e.g. "mcp.gitlab" or "projet T-SRU → mcp.gitlab"
	Scope  string // "global" or project ID
}

// SecretsViewConfig holds external dependencies for the secrets view.
type SecretsViewConfig struct {
	// GetStore returns the keychain store (with ListAll/SetScoped/DeleteScoped).
	// May return nil if no secret store is available.
	GetStore func() *keychain.Store
	// GetExpectedKeys returns all token_key values referenced in hub.toml + project config.
	// Used to display "absent" entries for keys that haven't been set yet.
	GetExpectedKeys func() []ExpectedSecret
	// GetActiveProjectID returns the active project ID (empty if none).
	GetActiveProjectID func() string
	// GetActiveProjectName returns the active project display name.
	GetActiveProjectName func() string
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// secretEntry is a merged entry combining index data and expected-keys for display.
type secretEntry struct {
	Key      string
	Scope    string // "global" or project ID
	Present  bool
	Masked   string // "****abcd" or ""
	Source   string // where referenced (e.g. "mcp.gitlab")
	Fallback string // "(fallback: global ✓)" or "(masque le global)" or ""
}

// SecretsView displays and manages secrets (tokens, API keys) with global and
// project scopes. Accessed via omnibar "secrets" / "tokens" / "credentials".
type SecretsView struct {
	app      *tview.Application
	content  *tview.Flex
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      SecretsViewConfig
	mountGen uint64

	entries []secretEntry
}

var _ View = (*SecretsView)(nil)
var _ CommandProvider = (*SecretsView)(nil)

func NewSecretsView(cfg SecretsViewConfig) *SecretsView {
	return &SecretsView{cfg: cfg}
}

func (v *SecretsView) SetShell(s ShellAccess) { v.shell = s }
func (v *SecretsView) ID() string             { return "secrets" }
func (v *SecretsView) Title() string          { return "Secrets & Tokens" }
func (v *SecretsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Enter %s · e %s · a %s · d %s · r %s · Esc %s",
		i18n.T("tui.hints.nav"), i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.edit"), i18n.T("tui.hints.add"),
		i18n.T("tui.hints.delete"), i18n.T("tui.hints.refresh"),
		i18n.T("tui.hints.back"))
}

func (v *SecretsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.mountGen++
	gen := v.mountGen

	// Show loading placeholder immediately
	loadingTV := tview.NewTextView().
		SetDynamicColors(true)
	loadingTV.SetBackgroundColor(theme.BgPanel)
	loadingTV.SetBorderPadding(1, 0, 2, 2)
	muted := theme.ColorTag(theme.TextMutedHex)
	loadingTV.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.secrets.loading"), theme.TagColor))
	content.AddItem(loadingTV, 0, 1, true)

	// Load secrets asynchronously (keychain access)
	go func() {
		entries := v.buildEntries()
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
				v.editByItem(item)
			})

			v.entries = entries
			v.renderList()
			content.RemoveItem(loadingTV)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

func (v *SecretsView) Unmount() {
	v.app = nil
	v.content = nil
	v.list = nil
}

func (v *SecretsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'e':
		v.editCurrent()
		return nil
	case 'a':
		v.addSecret()
		return nil
	case 'd':
		v.deleteCurrent()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.editCurrent()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) refresh() {
	gen := v.mountGen
	go func() {
		entries := v.buildEntries()
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			if v.list == nil || v.app == nil || v.mountGen != gen {
				return
			}
			v.entries = entries
			v.renderList()
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.T("tui.secrets.refreshed"), true)
			}
		})
	}()
}

func (v *SecretsView) buildEntries() []secretEntry {
	store := v.cfg.GetStore()
	ctx := context.Background()

	var indexEntries []keychain.SecretEntry
	if store != nil {
		indexEntries, _ = store.ListAll(ctx)
	}
	expected := v.cfg.GetExpectedKeys()

	existing := make(map[string]keychain.SecretEntry)
	for _, e := range indexEntries {
		existing[e.Scope+"/"+e.Key] = e
	}

	seen := make(map[string]bool)
	var result []secretEntry

	for _, e := range indexEntries {
		entry := secretEntry{
			Key:     e.Key,
			Scope:   e.Scope,
			Present: e.Present,
			Masked:  maskedValue(store, ctx, e.Key, e.Scope),
		}
		for _, exp := range expected {
			if exp.Key == e.Key && exp.Scope == e.Scope {
				entry.Source = exp.Source
				break
			}
		}
		entry.Fallback = v.computeFallback(e.Key, e.Scope, e.Present, existing)
		result = append(result, entry)
		seen[e.Scope+"/"+e.Key] = true
	}

	for _, exp := range expected {
		lookupKey := exp.Scope + "/" + exp.Key
		if seen[lookupKey] {
			continue
		}
		entry := secretEntry{
			Key:     exp.Key,
			Scope:   exp.Scope,
			Present: false,
			Source:  exp.Source,
		}
		entry.Fallback = v.computeFallback(exp.Key, exp.Scope, false, existing)
		result = append(result, entry)
		seen[lookupKey] = true
	}

	return result
}

func (v *SecretsView) computeFallback(key, scope string, present bool, existing map[string]keychain.SecretEntry) string {
	if scope == "global" || scope == "" {
		projectID := v.cfg.GetActiveProjectID()
		if projectID != "" {
			if pe, ok := existing[projectID+"/"+key]; ok && pe.Present {
				return i18n.T("tui.secrets.shadowed_by_project")
			}
		}
		return ""
	}
	if !present {
		if ge, ok := existing["global/"+key]; ok && ge.Present {
			return i18n.T("tui.secrets.fallback_global")
		}
	} else {
		if _, ok := existing["global/"+key]; ok {
			return i18n.T("tui.secrets.shadows_global")
		}
	}
	return ""
}

func maskedValue(store *keychain.Store, ctx context.Context, key, scope string) string {
	if store == nil {
		return ""
	}
	val, _ := store.GetScoped(ctx, key, scope)
	if val == "" {
		return ""
	}
	if len(val) <= 4 {
		return "****"
	}
	return "****" + val[len(val)-4:]
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering (SectionedList)
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) renderList() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	if len(v.entries) == 0 {
		items := []widgets.SectionItem{{
			MainText:      i18n.T("tui.secrets.empty"),
			SecondaryText: i18n.T("tui.secrets.empty_hint"),
		}}
		v.list.SetItems(items)
		return
	}

	// Group by scope
	var globals, projects []int
	for i, e := range v.entries {
		if e.Scope == "global" || e.Scope == "" {
			globals = append(globals, i)
		} else {
			projects = append(projects, i)
		}
	}

	items := make([]widgets.SectionItem, 0, len(v.entries)+4)

	if len(globals) > 0 {
		items = append(items, widgets.SectionItem{
			IsHeader: true,
			MainText: "Global",
		})
		for _, i := range globals {
			items = append(items, v.entryToItem(v.entries[i], i))
		}
	}

	if len(projects) > 0 {
		projectName := v.cfg.GetActiveProjectName()
		if projectName == "" {
			projectName = v.cfg.GetActiveProjectID()
		}
		items = append(items, widgets.SectionItem{
			IsHeader: true,
			MainText: fmt.Sprintf("%s: %s", i18n.T("tui.secrets.section_project"), projectName),
		})
		for _, i := range projects {
			items = append(items, v.entryToItem(v.entries[i], i))
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

func (v *SecretsView) entryToItem(e secretEntry, entryIdx int) widgets.SectionItem {
	// Status
	status := fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.secrets.absent"), theme.TagColor)
	if e.Present {
		status = fmt.Sprintf("%s✓%s %s", theme.ColorTag(theme.SuccessHex), theme.TagColor, e.Masked)
	}

	fallback := ""
	if e.Fallback != "" {
		fallback = fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), e.Fallback, theme.TagColor)
	}

	mainText := fmt.Sprintf("%-24s %s%s", e.Key, status, fallback)

	// Secondary: source (dependency indicator)
	secondary := ""
	if e.Source != "" {
		secondary = fmt.Sprintf("%s %s%s", i18n.T("tui.secrets.referenced_by"), e.Source, "")
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondary,
		Reference:     entryIdx,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) currentEntry() (secretEntry, int, bool) {
	if v.list == nil {
		return secretEntry{}, -1, false
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return secretEntry{}, -1, false
	}
	entryIdx, ok := item.Reference.(int)
	if !ok || entryIdx < 0 || entryIdx >= len(v.entries) {
		return secretEntry{}, -1, false
	}
	return v.entries[entryIdx], entryIdx, true
}

func (v *SecretsView) editCurrent() {
	entry, _, ok := v.currentEntry()
	if !ok || v.shell == nil {
		return
	}
	v.editEntry(entry)
}

func (v *SecretsView) editByItem(item widgets.SectionItem) {
	entryIdx, ok := item.Reference.(int)
	if !ok || entryIdx < 0 || entryIdx >= len(v.entries) || v.shell == nil {
		return
	}
	v.editEntry(v.entries[entryIdx])
}

func (v *SecretsView) editEntry(entry secretEntry) {
	store := v.cfg.GetStore()
	if store == nil {
		v.shell.ShowToastMsg(i18n.T("tui.secrets.store_unavailable"), false)
		return
	}

	v.shell.ShowSelectModal(
		fmt.Sprintf("%s %s (%s)", i18n.T("tui.hints.edit"), entry.Key, scopeLabel(entry.Scope)),
		[]SelectOption{
			{Label: i18n.T("tui.secrets.edit_value"), Value: "value"},
			{Label: i18n.T("tui.secrets.move_scope"), Value: "move"},
			{Label: i18n.T("tui.settings.cancel"), Value: ""},
		}, "", func(choice string) {
			switch choice {
			case "value":
				v.promptSecretValue(entry.Key, entry.Scope)
			case "move":
				v.moveSecret(entry)
			}
		})
}

func (v *SecretsView) promptSecretValue(key, scope string) {
	if v.shell == nil {
		return
	}
	v.shell.ShowPasswordModal(i18n.Tf("tui.secrets.value_for", key), func(value string) {
		if value == "" {
			return
		}
		store := v.cfg.GetStore()
		if store == nil {
			return
		}
		ctx := context.Background()
		if err := store.SetScoped(ctx, key, value, scope); err != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
			return
		}
		v.shell.ShowToastMsg(i18n.T("tui.secrets.updated"), true)
		v.refresh()
	})
}

func (v *SecretsView) moveSecret(entry secretEntry) {
	store := v.cfg.GetStore()
	if store == nil || v.shell == nil {
		return
	}

	ctx := context.Background()
	newScope := "global"
	newLabel := "global"
	if entry.Scope == "global" || entry.Scope == "" {
		projectID := v.cfg.GetActiveProjectID()
		if projectID == "" {
			v.shell.ShowToastMsg(i18n.T("tui.secrets.no_project_for_move"), false)
			return
		}
		newScope = projectID
		newLabel = i18n.T("tui.secrets.section_project") + " " + v.cfg.GetActiveProjectName()
	}

	val, err := store.GetScoped(ctx, entry.Key, entry.Scope)
	if err != nil || val == "" {
		v.shell.ShowToastMsg(i18n.T("tui.secrets.not_found"), false)
		return
	}
	if err := store.SetScoped(ctx, entry.Key, val, newScope); err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		return
	}
	_ = store.DeleteScoped(ctx, entry.Key, entry.Scope)

	v.shell.ShowToastMsg(i18n.Tf("tui.secrets.moved_to", newLabel), true)
	v.refresh()
}

func (v *SecretsView) addSecret() {
	if v.shell == nil {
		return
	}

	v.shell.ShowInputModal(i18n.T("tui.secrets.key_name"), "", func(key string) {
		if key == "" {
			return
		}
		key = strings.TrimSpace(key)

		projectID := v.cfg.GetActiveProjectID()
		projectName := v.cfg.GetActiveProjectName()

		opts := []SelectOption{{Label: "Global", Value: "global"}}
		if projectID != "" {
			opts = append(opts, SelectOption{Label: i18n.T("tui.secrets.section_project") + ": " + projectName, Value: projectID})
		}

		v.shell.ShowSelectModal(i18n.T("tui.secrets.scope"), opts, "", func(scope string) {
			if scope == "" {
				return
			}
			v.promptSecretValue(key, scope)
		})
	})
}

func (v *SecretsView) deleteCurrent() {
	entry, _, ok := v.currentEntry()
	if !ok || v.shell == nil {
		return
	}
	if !entry.Present {
		v.shell.ShowToastMsg(i18n.T("tui.secrets.nothing_to_delete"), false)
		return
	}

	store := v.cfg.GetStore()
	if store == nil {
		return
	}

	label := fmt.Sprintf("%s (%s)", entry.Key, scopeLabel(entry.Scope))
	v.shell.ShowSelectModal(
		i18n.Tf("tui.secrets.confirm_delete", label),
		[]SelectOption{
			{Label: i18n.T("tui.settings.cancel"), Value: ""},
			{Label: i18n.T("tui.secrets.yes_delete"), Value: "yes"},
		}, "", func(choice string) {
			if choice != "yes" {
				return
			}
			ctx := context.Background()
			if err := store.DeleteScoped(ctx, entry.Key, entry.Scope); err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
				return
			}
			v.shell.ShowToastMsg(i18n.T("tui.secrets.deleted"), true)
			v.refresh()
		})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func scopeLabel(scope string) string {
	if scope == "global" || scope == "" {
		return "global"
	}
	return i18n.T("tui.secrets.section_project")
}

// ContextCommands implements CommandProvider.
func (v *SecretsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "secrets.add", Label: i18n.T("tui.hints.add"), Aliases: []string{"add", "new", "ajouter"}, Description: i18n.T("tui.secrets.cmd_add"), Category: "Secrets", Action: func() { v.addSecret() }},
		{ID: "secrets.refresh", Label: i18n.T("tui.hints.refresh"), Aliases: []string{"refresh", "reload", "rafraîchir"}, Description: i18n.T("tui.secrets.cmd_refresh"), Category: "Secrets", Action: func() { v.refresh() }},
	}
}
