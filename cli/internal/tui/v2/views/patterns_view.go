package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// PatternsView displays and manages team decomposition patterns.
type PatternsView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	list        *tview.List
	shell       ShellAccess
	patterns    []teamstate.Pattern
}

var _ View = (*PatternsView)(nil)

// NewPatternsView creates a new patterns view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewPatternsView(resolveTeam ResolveTeamFunc) *PatternsView {
	return &PatternsView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for modal interactions.
func (v *PatternsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PatternsView) ID() string { return "team.patterns" }

// Title returns the display title.
func (v *PatternsView) Title() string { return i18n.T("tui.team.patterns") }

// StatusHints returns keybinding hints.
func (v *PatternsView) StatusHints() string {
	return "j/k nav · Enter voir · a ajouter · v valider · d supprimer · r refresh"
}

// Mount builds the patterns list.
func (v *PatternsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.refresh()
	content.AddItem(v.list, 0, 1, true)
}

// Unmount cleans up resources.
func (v *PatternsView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes patterns view key events.
func (v *PatternsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'a':
		v.addPattern()
		return nil
	case 'v':
		v.validatePattern()
		return nil
	case 'd':
		v.removePattern()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.showPattern()
		return nil
	}
	return event
}

func (v *PatternsView) getRepo() *teamstate.Repo {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	return repo
}

func (v *PatternsView) refresh() {
	v.list.Clear()
	v.patterns = nil

	repo := v.getRepo()
	if repo == nil {
		v.list.AddItem("  Team non configurée", "", 0, nil)
		return
	}

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderPatterns(repo)
	})
}

func (v *PatternsView) renderPatterns(repo *teamstate.Repo) {
	if v.list == nil {
		return
	}
	v.list.Clear()
	v.patterns = nil

	patterns, err := repo.ListPatterns(nil, 0)
	if err != nil {
		v.list.AddItem("  Erreur: "+err.Error(), "", 0, nil)
		return
	}
	v.patterns = patterns

	if len(patterns) == 0 {
		v.list.AddItem("  Aucun pattern", "  Appuyez 'a' pour en créer un", 0, nil)
		return
	}

	for _, p := range patterns {
		icon := "○"
		if p.Validated {
			icon = "✓"
		}
		tags := strings.Join(p.Tags, ", ")
		v.list.AddItem(
			fmt.Sprintf("  %s %s", icon, p.Name),
			fmt.Sprintf("    [%s] %s", tags, p.Complexity),
			0, nil,
		)
	}
}

func (v *PatternsView) showPattern() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.patterns) {
		return
	}
	p := v.patterns[idx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	content, err := repo.ReadPattern(p.Name)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur lecture: "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowScrollableModal("Pattern: "+p.Name, content, []ModalAction{
			{Label: "Fermer", Callback: func() {}},
		})
	}
}

func (v *PatternsView) addPattern() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg("Team non configurée", false)
		return
	}

	complexityOpts := []SelectOption{
		{Label: "Basse", Value: "low"},
		{Label: "Moyenne", Value: "medium"},
		{Label: "Haute", Value: "high"},
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Créer un pattern",
		Fields: []FormField{
			{Key: "name", Label: "Nom (slug)", Type: FieldText, Required: true},
			{Key: "tags", Label: "Tags (virgule)", Type: FieldText},
			{Key: "complexity", Label: "Complexité", Type: FieldSelect, Options: complexityOpts, Default: "medium"},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			name := values["name"]
			if name == "" {
				return
			}
			p := teamstate.Pattern{
				Name:       name,
				Tags:       splitTags(values["tags"]),
				Complexity: values["complexity"],
				Source:     "manual",
				Validated:  false,
				CreatedAt:  time.Now().Format("2006-01-02"),
			}
			content := fmt.Sprintf("# %s\n\n## Description\n\nTODO\n\n## Étapes\n\n1. ...\n", name)
			if err := repo.CreatePattern(context.Background(), p, content); err != nil {
				v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg("Pattern créé: "+name, true)
				v.refresh()
			}
		},
		OnCancel: nil,
	})
}

func (v *PatternsView) validatePattern() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.patterns) {
		return
	}
	p := v.patterns[idx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	if err := repo.ValidatePattern(p.Name); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg("Pattern validé: "+p.Name, true)
		}
		v.refresh()
	}
}

func (v *PatternsView) removePattern() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.patterns) {
		return
	}
	p := v.patterns[idx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	if err := repo.RemovePattern(p.Name); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg("Pattern supprimé: "+p.Name, true)
		}
		v.refresh()
	}
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
