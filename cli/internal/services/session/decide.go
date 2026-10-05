package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Decisions (P3-T08): oh answers permissions and questions through the tool
// API, from any client. The first answer wins: oh claims the decision in the
// store before replying, and a request already settled in the tool (TUI,
// web, mobile) is reported as resolved there.

// Reply answers a decision.
type Reply struct {
	DecisionID string
	// Decision: permission once | always | reject; error/budget dismiss
	// (default); other kinds: the choice understood by their resolver.
	Decision string
	Message  string         // note forwarded to the agent (permission, checkpoint)
	Answer   map[string]any // question answers by field key (see ParseAnswers)
}

// Resolver answers a decision kind owned by another service (checkpoint:
// CheckpointService). It runs after oh claimed the decision; an error
// reopens it.
type Resolver func(ctx context.Context, d domain.Decision, r Reply) error

// Errors returned by Decide.
var (
	ErrAlreadyResolved  = errors.New("decision already resolved")
	ErrAlwaysForbidden  = errors.New(`"always" is not allowed by the strict isolation of this session`)
	ErrUnsupportedKind  = errors.New("this decision cannot be answered from oh")
	ErrServerNotRunning = errors.New("the tool server of this session is not running")
	ErrNotSupported     = errors.New("the session tool does not support this operation")
)

// ResolvedError tells who resolved a decision first.
type ResolvedError struct {
	By domain.DecisionResolver
}

func (e *ResolvedError) Error() string {
	if e.By == "" {
		return ErrAlreadyResolved.Error()
	}
	return fmt.Sprintf("%s (%s)", ErrAlreadyResolved, e.By)
}

// Is makes errors.Is(err, ErrAlreadyResolved) true.
func (e *ResolvedError) Is(target error) bool { return target == ErrAlreadyResolved }

// Decide answers an open decision.
func (s *Service) Decide(ctx context.Context, r Reply) error {
	if s.Decisions == nil {
		return errors.New("session: no decision store")
	}
	d, err := s.Decisions.Get(ctx, r.DecisionID)
	if err != nil {
		return fmt.Errorf("decision %s: %w", r.DecisionID, err)
	}
	if !d.Open() {
		return &ResolvedError{By: d.ResolvedBy}
	}
	resolution := &domain.DecisionResolution{Decision: r.Decision, Message: r.Message, Answer: r.Answer}

	var deliver func(ctx context.Context) error
	switch d.Kind {
	case domain.DecisionPermission:
		if err := s.checkPermission(ctx, d, r.Decision); err != nil {
			return err
		}
		deliver = func(ctx context.Context) error {
			return s.replyTool(ctx, d, adapters.DecisionReply{SessionID: d.SessionID, ID: d.ToolRef, Kind: adapters.DecisionPermission, Decision: r.Decision, Message: r.Message})
		}
	case domain.DecisionQuestion:
		if err := ValidateAnswers(d.Payload.Fields, r.Answer); err != nil {
			return err
		}
		deliver = func(ctx context.Context) error {
			return s.replyTool(ctx, d, adapters.DecisionReply{SessionID: d.SessionID, ID: d.ToolRef, Kind: adapters.DecisionQuestion, Answer: r.Answer})
		}
	case domain.DecisionError, domain.DecisionBudget:
		if r.Decision != "" && r.Decision != "dismiss" {
			return fmt.Errorf("invalid choice %q (dismiss)", r.Decision)
		}
		resolution.Decision = "dismiss"
		deliver = func(context.Context) error { return nil }
	default:
		res, ok := s.Resolvers[d.Kind]
		if !ok {
			return fmt.Errorf("%w (%s)", ErrUnsupportedKind, d.Kind)
		}
		deliver = func(ctx context.Context) error { return res(ctx, *d, r) }
	}

	// Claim first: a concurrent answer from another oh client loses here.
	claimed, err := s.Decisions.Resolve(ctx, d.ID, domain.ResolvedByOh, resolution, s.now())
	if err != nil {
		return err
	}
	if !claimed {
		if cur, err := s.Decisions.Get(ctx, d.ID); err == nil {
			return &ResolvedError{By: cur.ResolvedBy}
		}
		return ErrAlreadyResolved
	}
	if err := deliver(ctx); err != nil {
		_ = s.Decisions.Reopen(ctx, d.ID)
		if errors.Is(err, adapters.ErrRequestGone) {
			// Answered in the tool (or cancelled) before us.
			_, _ = s.Decisions.Resolve(ctx, d.ID, domain.ResolvedByTool, nil, s.now())
			return &ResolvedError{By: domain.ResolvedByTool}
		}
		return err
	}
	return nil
}

func (s *Service) checkPermission(ctx context.Context, d *domain.Decision, decision string) error {
	switch decision {
	case "once", "reject":
		return nil
	case "always":
		if s.strict(ctx, d.SessionID) {
			return ErrAlwaysForbidden
		}
		return nil
	}
	return fmt.Errorf("invalid permission decision %q (once | always | reject)", decision)
}

// strict reports whether the session bundle requires strict isolation.
// Unknown (no bundle) counts as strict: "always" is never granted blindly.
func (s *Service) strict(ctx context.Context, sessionID string) bool {
	if s.Sessions == nil {
		return true
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil || sess.BundleHash == "" || s.BundlesDir == "" {
		return true
	}
	b, err := bundle.Load(s.BundlesDir, sess.BundleHash)
	if err != nil {
		return true
	}
	return b.Spec.StrictIsolation
}

func (s *Service) replyTool(ctx context.Context, d *domain.Decision, reply adapters.DecisionReply) error {
	if s.Adapter == nil {
		return ErrNotSupported
	}
	if !s.Adapter.Capabilities().HeadlessDecisions {
		return ErrNotSupported
	}
	h, err := s.handle(ctx, d.SessionID)
	if err != nil {
		return err
	}
	return s.Adapter.Reply(ctx, h, reply)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ── Questions ─────────────────────────────────────────────────────────────

// ParseAnswers converts textual answers (CLI --field k=v, TUI inputs) to the
// types of the question fields: numbers, booleans, comma-separated lists.
func ParseAnswers(fields []domain.DecisionField, raw map[string]string) (map[string]any, error) {
	byKey := map[string]domain.DecisionField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	out := map[string]any{}
	for k, v := range raw {
		f, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("unknown field %q", k)
		}
		switch f.Type {
		case "number":
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("field %q: a number is expected", k)
			}
			out[k] = n
		case "integer":
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("field %q: an integer is expected", k)
			}
			out[k] = n
		case "boolean":
			b, err := parseBool(v)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			out[k] = b
		case "multiselect":
			var list []string
			for _, p := range strings.Split(v, ",") {
				if p = strings.TrimSpace(p); p != "" {
					list = append(list, p)
				}
			}
			out[k] = list
		default:
			out[k] = v
		}
	}
	return out, ValidateAnswers(fields, out)
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "y", "oui", "o", "1":
		return true, nil
	case "false", "no", "n", "non", "0":
		return false, nil
	}
	return false, errors.New("true or false is expected")
}

// ValidateAnswers checks typed answers against the question fields: known
// keys, required fields, value types, and options unless free answers are
// allowed.
func ValidateAnswers(fields []domain.DecisionField, answer map[string]any) error {
	byKey := map[string]domain.DecisionField{}
	for _, f := range fields {
		byKey[f.Key] = f
		if _, ok := answer[f.Key]; !ok && f.Required && f.Type != "external" {
			return fmt.Errorf("field %q is required", f.Key)
		}
	}
	for k, v := range answer {
		f, ok := byKey[k]
		if !ok {
			return fmt.Errorf("unknown field %q", k)
		}
		switch f.Type {
		case "number", "integer":
			switch v.(type) {
			case float64, int64, int:
			default:
				return fmt.Errorf("field %q: a number is expected", k)
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("field %q: true or false is expected", k)
			}
		case "multiselect":
			list, ok := v.([]string)
			if !ok {
				return fmt.Errorf("field %q: a list is expected", k)
			}
			for _, item := range list {
				if err := checkOption(f, item); err != nil {
					return err
				}
			}
		default:
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("field %q: text is expected", k)
			}
			if err := checkOption(f, str); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkOption(f domain.DecisionField, v string) error {
	if len(f.Options) == 0 || f.Custom {
		return nil
	}
	for _, o := range f.Options {
		if o.Value == v {
			return nil
		}
	}
	values := make([]string, 0, len(f.Options))
	for _, o := range f.Options {
		values = append(values, o.Value)
	}
	return fmt.Errorf("field %q: %q is not one of %s", f.Key, v, strings.Join(values, ", "))
}
