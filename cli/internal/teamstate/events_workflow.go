package teamstate

// Workflow events (v5 phase 2), appended to the events of the scope's
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
