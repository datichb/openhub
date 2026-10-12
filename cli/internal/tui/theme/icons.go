package theme

// ─────────────────────────────────────────────────────────────────────────────
// Canonical icon constants (deduplicated, single source of truth)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// IconActive marks the currently active view/item.
	IconActive = "◆"
	// IconDone marks a completed step.
	IconDone = "●"
	// IconPending marks a future/pending step.
	IconPending = "○"
	// IconSkipped marks a skipped step.
	IconSkipped = "◌"
	// IconSuccess is a checkmark for success feedback.
	IconSuccess = "✓"
	// IconError is a cross for error feedback.
	IconError = "✗"
	// IconWarning is an exclamation for warnings.
	IconWarning = "!"
	// IconInfo is an info indicator.
	IconInfo = "ℹ"
	// IconArrow is a pointer/expanded indicator.
	IconArrow = "▸"
	// IconCollapsed is a collapsed category indicator.
	IconCollapsed = "▾"
	// IconDot is a bullet for menu items.
	IconDot = "·"
	// IconConnector is a step bar connector.
	IconConnector = "───"
	// IconGutter is the active item left gutter.
	IconGutter = "▎"
	// IconStepActive marks the active step in a wizard.
	IconStepActive = "◆"
	// IconSepBold is a bold separator.
	IconSepBold = "━"
	// IconSepNormal is a normal separator.
	IconSepNormal = "─"
	// IconSepConn is a connector separator.
	IconSepConn = "┃"
)

// Workflow catalogue pictograms (catalogue list, legend, sessions, board).
const (
	// IconDraft marks a draft (leading) or a published workflow that has a
	// draft (trailing).
	IconDraft = "✎"
	// IconPinned marks a pinned workflow.
	IconPinned = "★"
	// IconRuntimeLocal, IconRuntimeContainer and IconRuntimeRemote are the
	// runtimes (local, container, remote).
	IconRuntimeLocal     = "⌂"
	IconRuntimeContainer = "▣"
	IconRuntimeRemote    = "☁"
	// IconValid marks a valid workflow.
	IconValid = "✔"
	// IconInvalid marks a workflow with errors.
	IconInvalid = "✗"
	// IconWarn marks validation or integrity warnings.
	IconWarn = "⚠"
	// IconQueued marks an operation waiting for the network.
	IconQueued = "⏳"
	// IconNewBrick marks a draft that introduces a team brick.
	IconNewBrick = "+"
	// IconExtends introduces the `extends` chain of a workflow.
	IconExtends = "↳"
)

// RuntimeIcon returns the pictogram of a runtime ("" = local); an unknown
// runtime is returned as is.
func RuntimeIcon(rt string) string {
	switch rt {
	case "local", "":
		return IconRuntimeLocal
	case "container":
		return IconRuntimeContainer
	case "remote":
		return IconRuntimeRemote
	}
	return rt
}
