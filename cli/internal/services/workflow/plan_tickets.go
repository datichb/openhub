package workflow

import (
	"errors"
	"fmt"
)

// Launch plan: tickets of a launch (v5 corrections, A24). An epic given to a
// workflow that runs one session per ticket stands for its open children.

// TicketSource reads the tickets of a project (Beads).
type TicketSource interface {
	// IsEpic reports whether the ticket is an epic.
	IsEpic(id string) (bool, error)
	// OpenChildren returns the children of an epic still to work on (open
	// or in progress), in the tracker order.
	OpenChildren(epic string) ([]string, error)
}

// ErrEmptyEpic is returned for an epic without open child (EmptyEpicError).
var ErrEmptyEpic = errors.New("the epic has no open child ticket")

// EmptyEpicError names the epic without open child.
type EmptyEpicError struct{ Epic string }

func (e *EmptyEpicError) Error() string { return ErrEmptyEpic.Error() + ": " + e.Epic }

// Is matches ErrEmptyEpic.
func (e *EmptyEpicError) Is(target error) bool { return target == ErrEmptyEpic }

// EpicExpansion is an epic replaced by its open children.
type EpicExpansion struct {
	Epic     string
	Children []string
}

// ExpandEpics replaces every epic of tickets by its open children (each
// ticket once, order kept). A ticket that cannot be read is kept as given
// (the workflow reports it). An epic without open child is an error.
func ExpandEpics(src TicketSource, tickets []string) ([]string, []EpicExpansion, error) {
	if src == nil {
		return tickets, nil, nil
	}
	var (
		out  []string
		exps []EpicExpansion
	)
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, t := range tickets {
		epic, err := src.IsEpic(t)
		if err != nil || !epic {
			add(t)
			continue
		}
		children, err := src.OpenChildren(t)
		if err != nil {
			return nil, nil, fmt.Errorf("epic %s: %w", t, err)
		}
		if len(children) == 0 {
			return nil, nil, &EmptyEpicError{Epic: t}
		}
		exps = append(exps, EpicExpansion{Epic: t, Children: children})
		for _, c := range children {
			add(c)
		}
	}
	return out, exps, nil
}
