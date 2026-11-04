package views

import (
	"context"
	"maps"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Names turns the IDs that the team-state and the registry keep into the names
// people gave: the name of a project, the display name of a member. It is a
// snapshot, read when the view reads its data (NamesFunc), so that a project
// added or renamed, a member who changed their name, shows at the next
// refresh. The zero Names answers every ID with itself.
//
// The ID stays what it was wherever it is an argument (-p, a proposed
// command): Names is for what is shown.
type Names struct {
	projects map[string]string
	members  map[string]string
}

// NamesFunc reads the names: the projects of the registry, and the members of
// the team-state the view reads (nil: none). It reads the registry and files:
// call it off the event loop, once per refresh.
type NamesFunc func(ctx context.Context, members teamstate.MemberReader) Names

// Read is f(ctx, members), the IDs as they are without a NamesFunc.
func (f NamesFunc) Read(ctx context.Context, members teamstate.MemberReader) Names {
	if f == nil {
		return Names{}
	}
	return f(ctx, members)
}

// NewNames builds a snapshot: the project names by ID (the registry of the hub)
// and the members of the team (their display name).
func NewNames(projects map[string]string, members []teamstate.Member) Names {
	return Names{projects: projects}.WithMembers(members)
}

// WithMembers is n with the display names of members, for a view that has read
// the members itself.
func (n Names) WithMembers(members []teamstate.Member) Names {
	if len(members) == 0 {
		return n
	}
	byID := make(map[string]string, len(n.members)+len(members))
	maps.Copy(byID, n.members)
	for _, m := range members {
		if m.DisplayName != "" {
			byID[m.ID] = m.DisplayName
		}
	}
	n.members = byID
	return n
}

// KnowsProject reports whether a project of the registry has this ID.
func (n Names) KnowsProject(id string) bool {
	_, ok := n.projects[id]
	return ok
}

// Project is the name of the project, or its ID when this machine does not
// know it. It is a ProjectNameResolver.
func (n Names) Project(id string) string {
	if name := n.projects[id]; name != "" {
		return name
	}
	return id
}

// ProjectLabel is the name of the project; the ID of a project this machine
// does not know is marked as such (« 88280c5b (projet inconnu) »), like the
// tabs of the team board. "" for no project. Without a registry read (a nil
// map) nothing is known to be unknown: the ID alone.
func (n Names) ProjectLabel(id string) string {
	if n.projects == nil {
		return id
	}
	return projectLabel(id, n.Project(id))
}

// Member is the display name of the member, or their ID when the team does not
// list them (a member who left).
func (n Names) Member(id string) string {
	if name := n.members[id]; name != "" {
		return name
	}
	return id
}

// projectLabel is the name of a project for display, given the name its
// resolver found: a name equal to the ID is the resolver's way to say that the
// project is unknown.
func projectLabel(id, name string) string {
	switch {
	case name != "" && name != id:
		return name
	case id == "":
		return ""
	}
	return i18n.Tf("tui.teamboard.project_unknown", id)
}
