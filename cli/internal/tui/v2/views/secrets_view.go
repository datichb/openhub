package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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
	app     *tview.Application
	content *tview.Flex
	list    *tview.List
	shell   ShellAccess
	cfg     SecretsViewConfig

	entries      []secretEntry
	listToEntry  []int // maps list item index → entries index (-1 for headers)
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
	return "j/k nav · e modifier · a ajouter · d supprimer · r refresh · Esc retour"
}

func (v *SecretsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	loadingTV := tview.NewTextView().
		SetDynamicColors(true)
	loadingTV.SetBackgroundColor(theme.BgPanel)
	loadingTV.SetBorderPadding(1, 0, 2, 2)
	muted := theme.ColorTag(theme.TextMutedHex)
	loadingTV.SetText(fmt.Sprintf("\n  %sChargement des secrets...%s", muted, theme.TagColor))
	content.AddItem(loadingTV, 0, 1, true)

	// Load secrets asynchronously (keychain access)
	go func() {
		entries := v.buildEntries()
		app.QueueUpdateDraw(func() {
			if v.list == nil {
				return
			}
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
		v.editSelected()
		return nil
	case 'a':
		v.addSecret()
		return nil
	case 'd':
		v.deleteSelected()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.editSelected()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) refresh() {
	v.entries = v.buildEntries()
	v.renderList()
}

func (v *SecretsView) buildEntries() []secretEntry {
	store := v.cfg.GetStore()
	ctx := context.Background()

	// Gather entries from the keychain index.
	var indexEntries []keychain.SecretEntry
	if store != nil {
		indexEntries, _ = store.ListAll(ctx)
	}

	// Gather expected keys from config.
	expected := v.cfg.GetExpectedKeys()

	// Build a lookup of existing secrets: scope/key → SecretEntry
	existing := make(map[string]keychain.SecretEntry)
	for _, e := range indexEntries {
		existing[e.Scope+"/"+e.Key] = e
	}

	// Merge expected + existing into the display list.
	seen := make(map[string]bool)
	var result []secretEntry

	// Add existing entries first.
	for _, e := range indexEntries {
		entry := secretEntry{
			Key:     e.Key,
			Scope:   e.Scope,
			Present: e.Present,
			Masked:  maskedValue(store, ctx, e.Key, e.Scope),
		}
		// Find source from expected list.
		for _, exp := range expected {
			if exp.Key == e.Key && exp.Scope == e.Scope {
				entry.Source = exp.Source
				break
			}
		}
		// Compute fallback/shadow indicators.
		entry.Fallback = v.computeFallback(e.Key, e.Scope, e.Present, existing)
		result = append(result, entry)
		seen[e.Scope+"/"+e.Key] = true
	}

	// Add expected keys not yet in the index (absent secrets).
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
		// Check if a project-scoped version shadows this global.
		projectID := v.cfg.GetActiveProjectID()
		if projectID != "" {
			if pe, ok := existing[projectID+"/"+key]; ok && pe.Present {
				return "(masqué par le projet)"
			}
		}
		return ""
	}

	// Project scope: check if global fallback is used.
	if !present {
		if ge, ok := existing["global/"+key]; ok && ge.Present {
			return "(fallback: global ✓)"
		}
	} else {
		// Project has its own value — it shadows the global.
		if _, ok := existing["global/"+key]; ok {
			return "(masque le global)"
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
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) renderList() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()
	v.list.Clear()
	v.listToEntry = nil

	if len(v.entries) == 0 {
		v.list.AddItem("  Aucun secret enregistré", "  Appuyez 'a' pour en ajouter", 0, nil)
		v.listToEntry = append(v.listToEntry, -1)
		return
	}

	// Group by scope for display.
	var globals, projects []int // indices into v.entries
	for i, e := range v.entries {
		if e.Scope == "global" || e.Scope == "" {
			globals = append(globals, i)
		} else {
			projects = append(projects, i)
		}
	}

	if len(globals) > 0 {
		v.list.AddItem(
			fmt.Sprintf("  %s─── Global ──────────────────────────────%s",
				theme.ColorTag(theme.AccentHex), theme.TagColor),
			"", 0, nil)
		v.listToEntry = append(v.listToEntry, -1) // header
		for _, i := range globals {
			v.addEntryItem(v.entries[i])
			v.listToEntry = append(v.listToEntry, i)
		}
	}

	if len(projects) > 0 {
		projectName := v.cfg.GetActiveProjectName()
		if projectName == "" {
			projectName = v.cfg.GetActiveProjectID()
		}
		v.list.AddItem(
			fmt.Sprintf("  %s─── Projet: %s ─────────────────────────%s",
				theme.ColorTag(theme.AccentHex), projectName, theme.TagColor),
			"", 0, nil)
		v.listToEntry = append(v.listToEntry, -1) // header
		for _, i := range projects {
			v.addEntryItem(v.entries[i])
			v.listToEntry = append(v.listToEntry, i)
		}
	}
	if savedIdx >= 0 && savedIdx < v.list.GetItemCount() {
		v.list.SetCurrentItem(savedIdx)
	}
}

func (v *SecretsView) addEntryItem(e secretEntry) {
	// Main line: key + status
	status := fmt.Sprintf("%s✗ absent%s", theme.ColorTag("#FF5252"), theme.TagColor)
	if e.Present {
		status = fmt.Sprintf("%s✓%s %s", theme.ColorTag("#4CAF50"), theme.TagColor, e.Masked)
	}

	fallback := ""
	if e.Fallback != "" {
		fallback = fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), e.Fallback, theme.TagColor)
	}

	main := fmt.Sprintf("  %-24s %s%s", e.Key, status, fallback)

	// Secondary line: source
	secondary := ""
	if e.Source != "" {
		secondary = fmt.Sprintf("    référencé par: %s", e.Source)
	}

	v.list.AddItem(main, secondary, 0, nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *SecretsView) selectedEntry() (secretEntry, bool) {
	if v.list == nil || v.list.GetItemCount() == 0 {
		return secretEntry{}, false
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.listToEntry) {
		return secretEntry{}, false
	}
	entryIdx := v.listToEntry[idx]
	if entryIdx < 0 || entryIdx >= len(v.entries) {
		return secretEntry{}, false // section header or out of bounds
	}
	return v.entries[entryIdx], true
}

func (v *SecretsView) editSelected() {
	entry, ok := v.selectedEntry()
	if !ok || v.shell == nil {
		return
	}

	store := v.cfg.GetStore()
	if store == nil {
		v.shell.ShowToastMsg("Secret store non disponible", false)
		return
	}

	// Context menu
	v.shell.ShowSelectModal("Modifier "+entry.Key+" ("+scopeLabel(entry.Scope)+")", []SelectOption{
		{Label: "Modifier la valeur (masqué)", Value: "value"},
		{Label: "Changer de portée (global ↔ projet)", Value: "move"},
		{Label: "Annuler", Value: ""},
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
	// Use a masked input field (ShowPasswordModal uses InputField with mask char).
	v.shell.ShowPasswordModal("Valeur pour "+key, func(value string) {
		if value == "" {
			return
		}
		store := v.cfg.GetStore()
		if store == nil {
			return
		}
		ctx := context.Background()
		if err := store.SetScoped(ctx, key, value, scope); err != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
			return
		}
		v.shell.ShowToastMsg("Secret mis à jour", true)
		v.refresh()
	})
}

func (v *SecretsView) moveSecret(entry secretEntry) {
	store := v.cfg.GetStore()
	if store == nil || v.shell == nil {
		return
	}

	ctx := context.Background()
	// Determine the target scope.
	newScope := "global"
	newLabel := "global"
	if entry.Scope == "global" || entry.Scope == "" {
		projectID := v.cfg.GetActiveProjectID()
		if projectID == "" {
			v.shell.ShowToastMsg("Aucun projet actif pour déplacer le secret", false)
			return
		}
		newScope = projectID
		newLabel = "projet " + v.cfg.GetActiveProjectName()
	}

	// Get current value, move to new scope.
	val, err := store.GetScoped(ctx, entry.Key, entry.Scope)
	if err != nil || val == "" {
		v.shell.ShowToastMsg("Secret introuvable à la portée actuelle", false)
		return
	}
	if err := store.SetScoped(ctx, entry.Key, val, newScope); err != nil {
		v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		return
	}
	_ = store.DeleteScoped(ctx, entry.Key, entry.Scope)

	v.shell.ShowToastMsg(fmt.Sprintf("Déplacé vers %s", newLabel), true)
	v.refresh()
}

func (v *SecretsView) addSecret() {
	if v.shell == nil {
		return
	}

	// Step 1: key name
	v.shell.ShowInputModal("Nom de la clé", "", func(key string) {
		if key == "" {
			return
		}
		key = strings.TrimSpace(key)

		// Step 2: scope
		projectID := v.cfg.GetActiveProjectID()
		projectName := v.cfg.GetActiveProjectName()

		opts := []SelectOption{{Label: "Global", Value: "global"}}
		if projectID != "" {
			opts = append(opts, SelectOption{Label: "Projet: " + projectName, Value: projectID})
		}

		v.shell.ShowSelectModal("Portée", opts, "", func(scope string) {
			if scope == "" {
				return
			}
			// Step 3: value (masked)
			v.promptSecretValue(key, scope)
		})
	})
}

func (v *SecretsView) deleteSelected() {
	entry, ok := v.selectedEntry()
	if !ok || v.shell == nil {
		return
	}
	if !entry.Present {
		v.shell.ShowToastMsg("Ce secret n'a pas de valeur à supprimer", false)
		return
	}

	store := v.cfg.GetStore()
	if store == nil {
		return
	}

	label := fmt.Sprintf("%s (%s)", entry.Key, scopeLabel(entry.Scope))
	v.shell.ShowSelectModal("Supprimer "+label+" ?", []SelectOption{
		{Label: "Confirmer la suppression", Value: "yes"},
		{Label: "Annuler", Value: ""},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		ctx := context.Background()
		if err := store.DeleteScoped(ctx, entry.Key, entry.Scope); err != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
			return
		}
		v.shell.ShowToastMsg("Secret supprimé", true)
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
	return "projet"
}

// ContextCommands implements CommandProvider.
func (v *SecretsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "secrets.add", Label: "Ajouter un secret", Aliases: []string{"add", "new"}, Description: "Ajouter un nouveau secret au keychain", Category: "Secrets", Action: func() { v.addSecret() }},
		{ID: "secrets.refresh", Label: "Rafraîchir", Aliases: []string{"refresh", "reload"}, Description: "Recharger les secrets", Category: "Secrets", Action: func() { v.refresh() }},
	}
}
