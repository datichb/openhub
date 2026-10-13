package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// `oh wiki decide`: the decision of the user on proposals of the queue,
// outside the card of the commit checkpoint (same choices, same service).

// wikiEditText edits the content of a proposal in $EDITOR (a variable for
// the tests).
var wikiEditText = func(name, content string) (string, error) {
	out, _, err := editTextAt(name, []byte(content), 0)
	return string(out), err
}

func init() {
	decide := &cobra.Command{
		Use:   "decide <id>...",
		Short: i18n.T("cmd.wiki.decide.short"),
		Long:  i18n.T("cmd.wiki.decide.long"),
		Args:  cobra.MinimumNArgs(1),
		RunE:  runWikiDecide,
	}
	decide.Flags().Bool("accept", false, i18n.T("cmd.wiki.decide.flags.accept"))
	decide.Flags().Bool("refuse", false, i18n.T("cmd.wiki.decide.flags.refuse"))
	decide.Flags().Bool("reject", false, i18n.T("cmd.wiki.decide.flags.refuse"))
	_ = decide.Flags().MarkHidden("reject")
	decide.Flags().Bool("pending", false, i18n.T("cmd.wiki.decide.flags.pending"))
	decide.Flags().Bool("edit", false, i18n.T("cmd.wiki.decide.flags.edit"))
	decide.Flags().String("content", "", i18n.T("cmd.wiki.decide.flags.content"))
	decide.Flags().String("file", "", i18n.T("cmd.wiki.decide.flags.file"))
	decide.Flags().String("dir", ".", i18n.T("cmd.wiki.decide.flags.dir"))
	wikiCmd.AddCommand(withInitDB(decide))
}

// wikiDecision is what `oh wiki decide` was asked.
type wikiDecision struct {
	Choice string
	// Content is the new content of an edit ("" with Editor: from $EDITOR).
	Content string
	Editor  bool
}

// wikiDecisionFlags reads the choice of the flags: exactly one of
// --accept, --refuse (--reject), --pending, --edit; --content or --file
// edit without $EDITOR.
func wikiDecisionFlags(cmd *cobra.Command) (wikiDecision, error) {
	flag := func(name string) bool { v, _ := cmd.Flags().GetBool(name); return v }
	content, _ := cmd.Flags().GetString("content")
	file, _ := cmd.Flags().GetString("file")
	var d wikiDecision
	n := 0
	for _, c := range []struct {
		set    bool
		choice string
	}{
		{flag("accept"), wiki.ChoiceAccept},
		{flag("refuse") || flag("reject"), wiki.ChoiceRefuse},
		{flag("pending"), wiki.ChoicePend},
		{flag("edit") || content != "" || file != "", wiki.ChoiceEdit},
	} {
		if c.set {
			d.Choice = c.choice
			n++
		}
	}
	if n != 1 {
		return d, errors.New(i18n.T("cmd.wiki.decide.one_choice"))
	}
	if content != "" && file != "" {
		return d, errors.New(i18n.T("cmd.wiki.decide.content_or_file"))
	}
	switch {
	case file == "-":
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return d, err
		}
		d.Content = string(b)
	case file != "":
		b, err := os.ReadFile(file) //nolint:gosec // a file the user names
		if err != nil {
			return d, err
		}
		d.Content = string(b)
	case content != "":
		d.Content = content
	default:
		d.Editor = d.Choice == wiki.ChoiceEdit
	}
	if d.Choice == wiki.ChoiceEdit && !d.Editor && strings.TrimSpace(d.Content) == "" {
		return d, errors.New(i18n.T("cmd.wiki.decide.empty"))
	}
	return d, nil
}

func runWikiDecide(cmd *cobra.Command, args []string) error {
	d, err := wikiDecisionFlags(cmd)
	if err != nil {
		return err
	}
	if d.Choice == wiki.ChoiceEdit && len(args) > 1 {
		return errors.New(i18n.T("cmd.wiki.decide.edit_one"))
	}
	ctx := cmd.Context()
	dir, _ := cmd.Flags().GetString("dir")
	root, err := wikiRoot([]string{dir})
	if err != nil {
		return err
	}
	a := MustApp()
	projectID, err := wikiProject(ctx, root, func(ctx context.Context, path string) (string, error) {
		p, err := a.Projects.GetByPath(ctx, path)
		if err != nil {
			return "", err
		}
		return p.ID, nil
	})
	if err != nil {
		return err
	}
	return wikiDecide(ctx, newWikiService(), projectID, args, d, cmd.OutOrStdout())
}

// wikiDecide records the decision on each proposal; it goes on after an
// error and returns the first one.
func wikiDecide(ctx context.Context, svc *wiki.Service, projectID string, ids []string, d wikiDecision, out io.Writer) error {
	var first error
	fail := func(id string, err error) {
		fmt.Fprintln(out, i18n.Tf("cmd.wiki.decide.failed", id, wikiDecideError(err)))
		if first == nil {
			first = err
		}
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		content := d.Content
		if d.Editor {
			p, err := svc.Get(ctx, projectID, id)
			if err == nil && !p.Open() {
				err = wiki.ErrClosed
			}
			if err != nil {
				fail(id, err)
				continue
			}
			edited, err := wikiEditText(p.ID()+".md", p.Content)
			if err != nil {
				fail(id, err)
				continue
			}
			if t := strings.TrimSpace(edited); t == "" || t == strings.TrimSpace(p.Content) {
				// Emptied or untouched in the editor: nothing is decided.
				fmt.Fprintln(out, i18n.Tf("cmd.wiki.decide.unchanged", p.ID()))
				continue
			}
			content = edited
		}
		p, err := svc.Decide(ctx, projectID, id, d.Choice, content, string(domain.ResolvedByOh))
		if err != nil {
			fail(id, err)
			continue
		}
		fmt.Fprintln(out, i18n.Tf("cmd.wiki.decide.done", p.ID(), p.Page, wikiDecidedText(p, d.Choice)))
	}
	if first != nil {
		return errors.New(i18n.T("cmd.wiki.decide.some_failed"))
	}
	return nil
}

func wikiDecidedText(p *domain.WikiProposal, choice string) string {
	switch {
	case choice == wiki.ChoiceEdit && p.Original != "":
		return i18n.T("cmd.wiki.decide.edited")
	case p.Status == domain.WikiAccepted:
		return i18n.T("cmd.wiki.decide.accepted")
	case p.Status == domain.WikiRefused:
		return i18n.T("cmd.wiki.decide.refused")
	default:
		return i18n.T("cmd.wiki.decide.pending")
	}
}

func wikiDecideError(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return i18n.T("cmd.wiki.decide.not_found")
	case errors.Is(err, wiki.ErrBadID):
		return i18n.T("cmd.wiki.decide.bad_id")
	case errors.Is(err, wiki.ErrClosed):
		return i18n.T("cmd.wiki.decide.closed")
	}
	return err.Error()
}
