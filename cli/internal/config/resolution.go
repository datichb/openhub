package config

// ConfigSource indicates where a resolved configuration value came from.
type ConfigSource string

const (
	// SourceDefault indicates the value is the system default.
	SourceDefault ConfigSource = "default"
	// SourceTeamRecommended indicates the value comes from team-state config
	// as a recommendation (can be overridden by hub or project).
	SourceTeamRecommended ConfigSource = "team:recommended"
	// SourceTeamEnforced indicates the value is imposed by the team-state config
	// and cannot be overridden by hub or project.
	SourceTeamEnforced ConfigSource = "team:enforced"
	// SourceHub indicates the value comes from the user's hub.toml.
	SourceHub ConfigSource = "hub"
	// SourceProject indicates the value comes from a project-level override.
	SourceProject ConfigSource = "project"
	// SourceInherit indicates the value is inherited (nil/unset at this level).
	SourceInherit ConfigSource = "inherit"
)

// ResolvedValue holds a configuration value along with its resolution metadata.
// This is used by the TUI to display source annotations and lock indicators.
type ResolvedValue[T any] struct {
	// Value is the effective resolved value.
	Value T
	// Source indicates where this value came from.
	Source ConfigSource
	// TeamID is the ID of the team that provided this value (if Source is team-related).
	TeamID string
	// Locked indicates that this value is enforced and cannot be edited.
	Locked bool
}

// NewResolved creates a ResolvedValue with the given value and source.
func NewResolved[T any](value T, source ConfigSource) ResolvedValue[T] {
	return ResolvedValue[T]{
		Value:  value,
		Source: source,
		Locked: source == SourceTeamEnforced,
	}
}

// NewResolvedFromTeam creates a ResolvedValue sourced from a team.
func NewResolvedFromTeam[T any](value T, teamID string, enforced bool) ResolvedValue[T] {
	source := SourceTeamRecommended
	if enforced {
		source = SourceTeamEnforced
	}
	return ResolvedValue[T]{
		Value:  value,
		Source: source,
		TeamID: teamID,
		Locked: enforced,
	}
}

// ResolveBool implements the 4-step cascade resolution for a boolean setting.
//
// Resolution order:
//  1. Team ENFORCED? → use team value (locked)
//  2. Project has explicit override? → use project value
//  3. Hub has a value (non-nil)? → use hub value
//  4. Team RECOMMENDED? → use team recommendation
//  5. Fallback → system default
func ResolveBool(teamValue *bool, teamEnforced *bool, hubValue *bool, projectValue *bool, teamID string, defaultValue bool) ResolvedValue[bool] {
	// Step 1: Team enforced
	if teamEnforced != nil && *teamEnforced && teamValue != nil {
		return NewResolvedFromTeam(*teamValue, teamID, true)
	}

	// Step 2: Project override
	if projectValue != nil {
		return NewResolved(*projectValue, SourceProject)
	}

	// Step 3: Hub value
	if hubValue != nil {
		return NewResolved(*hubValue, SourceHub)
	}

	// Step 4: Team recommended
	if teamValue != nil {
		return NewResolvedFromTeam(*teamValue, teamID, false)
	}

	// Step 5: Default
	return NewResolved(defaultValue, SourceDefault)
}

// ResolveString implements the 4-step cascade resolution for a string setting.
func ResolveString(teamValue string, teamEnforced *bool, hubValue string, projectValue string, teamID string, defaultValue string) ResolvedValue[string] {
	// Step 1: Team enforced
	if teamEnforced != nil && *teamEnforced && teamValue != "" {
		return NewResolvedFromTeam(teamValue, teamID, true)
	}

	// Step 2: Project override
	if projectValue != "" {
		return NewResolved(projectValue, SourceProject)
	}

	// Step 3: Hub value
	if hubValue != "" {
		return NewResolved(hubValue, SourceHub)
	}

	// Step 4: Team recommended
	if teamValue != "" {
		return NewResolvedFromTeam(teamValue, teamID, false)
	}

	// Step 5: Default
	return NewResolved(defaultValue, SourceDefault)
}
