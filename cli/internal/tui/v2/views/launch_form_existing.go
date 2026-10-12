package views

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Launch on a ticket already worked on (runsvc.RunPlan.Existing): the recap
// compares each open session of the tickets with the launch and, for the
// session the choices apply to, offers them as buttons and keys: [r]
// reprendre, [t] reprendre ailleurs (transfer), [k] remplacer en gardant le
// travail, [d] supprimer et relancer (confirmed), [p] en parallèle. A choice
// launches at once.

// ErrTransferFailed marks a launch error caused by a failed transfer of the
// existing session (left unchanged): the form offers to replace it while
// keeping its work.
var ErrTransferFailed = errors.New("transfer of the existing session failed")

type transferFailedError struct{ msg string }

func (e transferFailedError) Error() string        { return e.msg }
func (e transferFailedError) Is(target error) bool { return target == ErrTransferFailed }

// TransferFailedError is an error with message msg that matches
// ErrTransferFailed.
func TransferFailedError(msg string) error { return transferFailedError{msg: msg} }

// existingChoice is a choice offered for the existing session.
type existingChoice struct {
	key   rune
	value string
}

var existingChoices = []existingChoice{
	{'r', LaunchExistingResume},
	{'t', LaunchExistingTransfer},
	{'k', LaunchExistingKeep},
	{'d', LaunchExistingDelete},
	{'p', LaunchExistingParallel},
}

// runtimeName is the label of a runtime, with its icon.
func runtimeName(rt string) string {
	switch rt {
	case "local", "container", "remote":
		return i18n.T("tui.launch.runtime_" + rt)
	case "":
		return ""
	}
	return theme.RuntimeIcon(rt) + " " + rt
}

// existingChoiceLabel is the button label of a choice.
func existingChoiceLabel(e LaunchExisting, choice string) string {
	if choice == LaunchExistingTransfer {
		if e.Differs(LaunchDiffRuntime) && (e.PlannedRuntime == "local" || e.PlannedRuntime == "container") {
			return i18n.T("tui.launch.existing.choice.transfer_" + e.PlannedRuntime)
		}
		return i18n.T("tui.launch.existing.choice.transfer")
	}
	return i18n.T("tui.launch.existing.choice." + choice)
}

// existingButtons are the choices shown as buttons for the target session:
// its allowed choices, « en parallèle » only when « Lancer » is hidden.
func (v *LaunchFormView) existingButtons() (LaunchExisting, []existingChoice) {
	e, ok := v.recap.existingTarget()
	if !ok || v.launching {
		return e, nil
	}
	var out []existingChoice
	for _, c := range existingChoices {
		if !e.Allows(c.value) || (c.value == LaunchExistingParallel && !v.recap.NeedsChoice) {
			continue
		}
		out = append(out, c)
	}
	return e, out
}

// existingText renders the comparison blocks of the recap.
func (v *LaunchFormView) existingText() string {
	if v.recap == nil || len(v.recap.Existing) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range v.recap.Existing {
		b.WriteString("\n" + existingBlock(e))
	}
	if e, ok := v.recap.existingTarget(); ok {
		var keys []string
		for _, c := range existingChoices {
			if e.Allows(c.value) {
				keys = append(keys, fmt.Sprintf("[%c] %s", c.key, existingChoiceLabel(e, c.value)))
			}
		}
		if len(keys) > 0 {
			b.WriteString("[" + theme.TextMutedHex + "]" + tview.Escape(strings.Join(keys, " · ")) + "[-]\n")
		}
	}
	return b.String()
}

const (
	existingLabelWidth = 18
	existingColWidth   = 34
)

// existingBlock is the table « Session existante | Nouveau lancement » of a
// session, the differing rows marked ⚠.
func existingBlock(e LaunchExisting) string {
	var b strings.Builder
	shared := strings.Join(e.Shared, ", ")
	if e.SameWorkflow {
		b.WriteString("[" + theme.WarningHex + "]⚠ " + tview.Escape(i18n.Tf("tui.launch.existing.same", e.ID, shared)) + "[-]\n")
	} else {
		b.WriteString("[" + theme.InfoHex + "]" + theme.IconInfo + " " + tview.Escape(i18n.Tf("tui.launch.existing.other", e.ID, e.Workflow, shared)) + "[-]\n")
	}
	muted := "[" + theme.TextMutedHex + "]"
	fmt.Fprintf(&b, "  %s%s %s %s[-]\n", muted, pad("", existingLabelWidth), pad(i18n.T("tui.launch.existing.col.existing"), existingColWidth), i18n.T("tui.launch.existing.col.new"))
	row := func(label, existing, planned string, differs bool) {
		mark := " "
		if differs {
			mark = "[" + theme.WarningHex + "]⚠[-]"
		}
		fmt.Fprintf(&b, "%s %s%s[-] %s %s\n", mark, muted, pad(label, existingLabelWidth),
			tview.Escape(pad(clipText(dashIfEmpty(existing), existingColWidth), existingColWidth)), tview.Escape(dashIfEmpty(planned)))
	}
	state := e.StateLabel
	if state == "" {
		state = e.State
	}
	if e.OpenDecisions > 0 {
		state += " · " + i18n.Tf("tui.sessions.waiting_decisions", e.OpenDecisions)
	}
	row(i18n.T("tui.launch.existing.row.state"), state, i18n.T("tui.launch.existing.new_session"), false)
	row(i18n.T("tui.launch.existing.row.workflow"), withVersion(e.Workflow, e.Version), withVersion(e.PlannedWorkflow, e.PlannedVersion),
		e.Differs(LaunchDiffWorkflow, LaunchDiffVersion))
	row(i18n.T("tui.launch.existing.row.runtime"), runtimeName(e.Runtime), runtimeName(e.PlannedRuntime), e.Differs(LaunchDiffRuntime))
	row(i18n.T("tui.launch.existing.row.mode"), e.Mode, e.PlannedMode, e.Differs(LaunchDiffMode))
	row(i18n.T("tui.launch.existing.row.model"), e.Model, e.PlannedModel, e.Differs(LaunchDiffModel, "provider"))
	row(i18n.T("tui.launch.existing.row.location"), withBranch(e.Path, e.Branch), withBranch(e.PlannedPath, e.PlannedBranch),
		e.Differs(LaunchDiffLocation, LaunchDiffBranch))
	work := i18n.Tf("tui.launch.existing.work_nogit", e.Cost)
	if e.WorkKnown {
		work = i18n.Tf("tui.launch.existing.work", e.CommitsUnpushed, e.ChangedFiles, e.Cost)
	}
	fmt.Fprintf(&b, "  %s%s[-] %s\n", muted, pad(i18n.T("tui.launch.existing.row.work"), existingLabelWidth), tview.Escape(work))
	return b.String()
}

func withVersion(id string, v int) string {
	if v == 0 || id == "" {
		return id
	}
	return fmt.Sprintf("%s v%d", id, v)
}

func withBranch(path, branch string) string {
	if branch == "" {
		return clipLeft(path, existingColWidth)
	}
	return clipLeft(path, existingColWidth-len([]rune(branch))-3) + " (" + branch + ")"
}

// clipLeft keeps the end of s (paths).
func clipLeft(s string, n int) string {
	r := []rune(s)
	if n < 8 {
		n = 8
	}
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n+1:])
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// pad right-pads s to width w (display width).
func pad(s string, w int) string {
	if n := tview.TaggedStringWidth(tview.Escape(s)); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// existingKey applies a choice key on the recap; it reports whether the
// key was used.
func (v *LaunchFormView) existingKey(r rune) bool {
	if v.step != launchStepRecap || v.launching || v.recap == nil {
		return false
	}
	e, ok := v.recap.existingTarget()
	if !ok {
		return false
	}
	for _, c := range existingChoices {
		if c.key == r {
			if e.Allows(c.value) {
				v.chooseExisting(c.value)
			}
			return true
		}
	}
	return false
}

// chooseExisting launches with a choice for the existing session; a
// deletion is confirmed first.
func (v *LaunchFormView) chooseExisting(choice string) {
	if v.launching || v.step != launchStepRecap || v.recap == nil {
		return
	}
	e, ok := v.recap.existingTarget()
	if !ok || !e.Allows(choice) {
		return
	}
	if choice != LaunchExistingDelete {
		v.launchExisting(choice, false)
		return
	}
	if v.shell == nil {
		return
	}
	title := i18n.Tf("tui.launch.existing.delete_title", e.ID)
	if v.cfg.PlanDelete == nil {
		v.shell.Confirm(title, i18n.T("tui.delete.irreversible"), true, func() { v.launchExisting(LaunchExistingDelete, false) })
		return
	}
	app, ctx, id := v.app, v.ctx, e.ID
	go func() {
		s, err := v.cfg.PlanDelete(ctx, id)
		if app == nil || ctx.Err() != nil {
			return
		}
		app.QueueUpdateDraw(func() {
			if err != nil {
				v.shell.ShowToastMsg(err.Error(), false)
				return
			}
			confirmSessionDelete(v.shell, title, s, func(discard bool) { v.launchExisting(LaunchExistingDelete, discard) })
		})
	}()
}

func (v *LaunchFormView) launchExisting(choice string, discard bool) {
	if v.launching || v.step != launchStepRecap {
		return
	}
	v.m.onExisting, v.m.discardBase = choice, discard
	v.launchWith(v.cfg.Launch)
	if !v.launching {
		v.m.onExisting, v.m.discardBase = "", false // not launched
	}
}

// offerKeep follows a failed transfer: the session is unchanged; replacing
// it while keeping its work is offered.
func (v *LaunchFormView) offerKeep(err error) {
	if v.shell == nil || v.recap == nil {
		return
	}
	e, ok := v.recap.existingTarget()
	if !ok || !e.Allows(LaunchExistingKeep) {
		return
	}
	v.shell.ShowToastMsg(err.Error(), false)
	v.shell.Confirm(i18n.T("tui.launch.existing.transfer_failed_title"),
		err.Error()+"\n\n"+i18n.T("tui.launch.existing.transfer_fallback"), false,
		func() { v.launchExisting(LaunchExistingKeep, false) })
}

// confirmSessionDelete asks to confirm the deletion of a session (danger),
// listing what it loses, then, when the session worked in the project
// directory with uncommitted changes, whether to discard them too. onYes
// gets that choice.
func confirmSessionDelete(sh ShellAccess, title string, s DeleteSummary, onYes func(discardBase bool)) {
	detail := strings.Join(s.Items, "\n")
	if s.Losses {
		detail += "\n\n" + "⚠ " + i18n.T("tui.delete.losses")
	}
	if strings.TrimSpace(detail) == "" {
		detail = i18n.T("tui.delete.irreversible")
	} else {
		detail += "\n\n" + i18n.T("tui.delete.irreversible")
	}
	sh.Confirm(title, detail, true, func() {
		if s.BaseChanges == 0 {
			onYes(false)
			return
		}
		sh.ShowScrollableModal(i18n.T("tui.delete.base_title"), tview.Escape(i18n.Tf("tui.delete.base_detail", s.BaseChanges)), []ModalAction{
			{Label: i18n.T("tui.delete.base_keep"), Callback: func() { onYes(false) }},
			{Label: i18n.T("tui.delete.base_discard"), Danger: true, Callback: func() { onYes(true) }},
		})
	})
}
