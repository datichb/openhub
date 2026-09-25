package platform

// FileChange represents a file modification detected during a session.
// This type is shared across platform interfaces (ParallelRunner, SessionServer).
type FileChange struct {
	Path      string // Relative path within the project
	Operation string // "created", "modified", or "deleted"
}
