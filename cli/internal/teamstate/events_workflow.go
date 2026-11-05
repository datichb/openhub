package teamstate

// Workflow events, appended to the events of the scope's
// project, or of TeamEventsProject for team workflows.
const (
	EventWorkflowPublished = "workflow.published"
	EventWorkflowRestored  = "workflow.restored"
	EventWorkflowArchived  = "workflow.archived"
)

// TeamEventsProject holds the events of team-wide workflows
// (projects/_team/events/).
const TeamEventsProject = "_team"

// EventProject is the events project of scope.
func (s WorkflowScope) EventProject() string {
	if s.IsProject() {
		return s.Project
	}
	return TeamEventsProject
}

// EventConfigChanged is appended to the events of TeamEventsProject by every
// publication of the team config: Data["keys"] lists the settings changed
// (never their values: some are secrets), Actor is the member who did it.
const EventConfigChanged = "config_changed"
