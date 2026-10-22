// Package policy edits the team policies (policies.toml of the team-state).
//
// Every change is one team-state transaction: pull, load the policies,
// change one, write the whole file with the TOML encoder (teamstate.
// SavePolicies), commit and push. The file is therefore always valid TOML,
// whatever the characters of the names and values. Rewriting it drops the
// comments and the layout of a hand-written file.
//
// The calls pull and push: they are blocking and network-bound. Callers
// that sit on a UI loop run them in a goroutine.
package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Errors of the service.
var (
	ErrNoName      = errors.New("policy: the name is empty")
	ErrExists      = errors.New("policy: a policy with this name already exists")
	ErrNotFound    = errors.New("policy: no policy with this name")
	ErrBadType     = errors.New("policy: the type is regex, boolean, limit or forbidden_pattern")
	ErrBadEnforced = errors.New("policy: the enforcement is disabled, warn or refuse")
)

// Store is what the service needs from the team-state repository.
// *teamstate.Repo implements it.
type Store interface {
	Transact(ctx context.Context, build func(ctx context.Context, tx *teamstate.Tx) (teamstate.TxResult, error)) error
	LoadPolicies(project string) ([]teamstate.Policy, error)
	SavePolicies(ctx context.Context, policies map[string]teamstate.Policy) error
}

var _ Store = (*teamstate.Repo)(nil)

// Service edits the policies of a team-state.
type Service struct {
	Store Store
}

// New returns a service on store.
func New(store Store) *Service { return &Service{Store: store} }

// Add creates the policy name. ErrExists when the name is taken.
func (s *Service) Add(ctx context.Context, name string, p teamstate.Policy) error {
	if err := validate(name, p); err != nil {
		return err
	}
	return s.edit(ctx, "policies: add "+name, func(m map[string]teamstate.Policy) error {
		if _, ok := m[name]; ok {
			return fmt.Errorf("%w: %s", ErrExists, name)
		}
		m[name] = p
		return nil
	})
}

// Update replaces the policy name by p. When p.Name is set and differs from
// name, the policy is renamed (ErrExists if the new name is taken).
// ErrNotFound when name does not exist.
func (s *Service) Update(ctx context.Context, name string, p teamstate.Policy) error {
	newName := name
	if p.Name != "" {
		newName = p.Name
	}
	if err := validate(newName, p); err != nil {
		return err
	}
	return s.edit(ctx, "policies: update "+newName, func(m map[string]teamstate.Policy) error {
		if _, ok := m[name]; !ok {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		if newName != name {
			if _, ok := m[newName]; ok {
				return fmt.Errorf("%w: %s", ErrExists, newName)
			}
			delete(m, name)
		}
		m[newName] = p
		return nil
	})
}

// SetEnforcement changes the enforcement of the policy name and nothing else.
func (s *Service) SetEnforcement(ctx context.Context, name string, e teamstate.PolicyEnforcement) error {
	if !validEnforcement(e) {
		return ErrBadEnforced
	}
	return s.edit(ctx, "policies: update "+name, func(m map[string]teamstate.Policy) error {
		p, ok := m[name]
		if !ok {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		p.Enforcement = e
		m[name] = p
		return nil
	})
}

// Remove deletes the policy name. ErrNotFound when it does not exist.
func (s *Service) Remove(ctx context.Context, name string) error {
	return s.edit(ctx, "policies: remove "+name, func(m map[string]teamstate.Policy) error {
		if _, ok := m[name]; !ok {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		delete(m, name)
		return nil
	})
}

// edit runs change on the policies as read inside the transaction (a
// rejected push runs it again on the new remote state).
func (s *Service) edit(ctx context.Context, msg string, change func(map[string]teamstate.Policy) error) error {
	return s.Store.Transact(ctx, func(ctx context.Context, _ *teamstate.Tx) (teamstate.TxResult, error) {
		list, err := s.Store.LoadPolicies("")
		if err != nil {
			return teamstate.TxResult{}, err
		}
		m := make(map[string]teamstate.Policy, len(list)+1)
		for _, p := range list {
			name := p.Name
			p.Name = ""
			m[name] = p
		}
		if err := change(m); err != nil {
			return teamstate.TxResult{}, err
		}
		if err := s.Store.SavePolicies(ctx, m); err != nil {
			return teamstate.TxResult{}, err
		}
		return teamstate.TxResult{Message: msg}, nil
	})
}

func validate(name string, p teamstate.Policy) error {
	if strings.TrimSpace(name) == "" {
		return ErrNoName
	}
	switch p.Type {
	case teamstate.PolicyTypeRegex, teamstate.PolicyTypeBoolean,
		teamstate.PolicyTypeLimit, teamstate.PolicyTypeForbiddenPattern:
	default:
		return fmt.Errorf("%w: %q", ErrBadType, p.Type)
	}
	if !validEnforcement(p.Enforcement) {
		return fmt.Errorf("%w: %q", ErrBadEnforced, p.Enforcement)
	}
	return nil
}

func validEnforcement(e teamstate.PolicyEnforcement) bool {
	switch e {
	case teamstate.EnforcementDisabled, teamstate.EnforcementWarn, teamstate.EnforcementRefuse:
		return true
	}
	return false
}
