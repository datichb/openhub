package cmd

// actionFreeSession opens the launch form of the `libre` workflow (free
// session with the orchestrator; another entry agent: `oh run libre
// --agent <id>`). Used by the "coder" omnibar command and the « Démarrer »
// fallback.
func actionFreeSession() {
	if tuiShell == nil {
		return
	}
	openLaunchForm(MustApp(), tuiLaunchRequest{WorkflowID: "libre"})
}
