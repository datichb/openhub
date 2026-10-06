package runsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Quitting oh (I7, P3-T21): sessions that are idle or waiting go to sleep
// now; for each session still working the user chooses: finish the current
// step then sleep (default), keep running in the background, or stop now.
// The sleep policy is held by the daemon per server group; stopping is per
// session.

// QuitChoice is what happens to a working session when oh quits.
type QuitChoice string

const (
	QuitFinish     QuitChoice = "finish"     // finish the current step, then sleep
	QuitBackground QuitChoice = "background" // keep running (normal idle timer)
	QuitStop       QuitChoice = "stop"       // stop the session now
)

// QuitPlan is the state of the open sessions when oh quits.
type QuitPlan struct {
	Working []domain.Session // agent loop running: the user chooses
	Resting []domain.Session // idle or waiting: put to sleep now
	Groups  []string         // server groups of Working and Resting
}

// policySetter is the daemon call used to apply a quit policy.
type policySetter interface {
	SetPolicy(ctx context.Context, group string, p daemon.QuitPolicy) error
}

// QuitPlan lists the open sessions whose servers are awake.
func (s *Service) QuitPlan(ctx context.Context) (QuitPlan, error) {
	var plan QuitPlan
	if s.Sessions == nil {
		return plan, nil
	}
	all, err := s.Sessions.List(ctx, "")
	if err != nil {
		return plan, err
	}
	groups := map[string]bool{}
	for _, sess := range all {
		if sess.GroupKey == "" || terminal(sess.State) || sess.State == domain.RunSleeping {
			continue
		}
		if !groups[sess.GroupKey] {
			groups[sess.GroupKey] = true
			plan.Groups = append(plan.Groups, sess.GroupKey)
		}
		if sess.State == domain.RunActive || sess.State == domain.RunPreparing {
			plan.Working = append(plan.Working, sess)
		} else {
			plan.Resting = append(plan.Resting, sess)
		}
	}
	return plan, nil
}

// ApplyQuit applies the choices (session ID → choice; missing = finish).
// A group keeps running when one of its working sessions was sent to the
// background; otherwise it sleeps once its turns are over.
func (s *Service) ApplyQuit(ctx context.Context, plan QuitPlan, choices map[string]QuitChoice) error {
	if len(plan.Groups) == 0 {
		return nil
	}
	dc, err := s.Daemon(ctx)
	if err != nil {
		return err
	}
	ps, ok := dc.(policySetter)
	if !ok {
		return errors.New("runsvc: the daemon client cannot set quit policies")
	}
	background := map[string]bool{}
	var errs []error
	for _, sess := range plan.Working {
		switch choices[sess.ID] {
		case QuitBackground:
			background[sess.GroupKey] = true
		case QuitStop:
			if err := s.StopSession(ctx, sess.ID); err != nil {
				errs = append(errs, fmt.Errorf("stopping %s: %w", sess.ID, err))
			}
		}
	}
	for _, g := range plan.Groups {
		p := daemon.PolicySleepWhenIdle
		if background[g] {
			p = daemon.PolicyBackground
		}
		if err := ps.SetPolicy(ctx, g, p); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WaitFinished waits until the server groups of the working sessions whose
// choice is "finish the step" are asleep or stopped (the daemon sleeps them
// after their step), until ctx is done. Used when the daemon lives in the
// oh process (Windows): it must not stop before those steps end.
func (s *Service) WaitFinished(ctx context.Context, plan QuitPlan, choices map[string]QuitChoice) error {
	groups := map[string]bool{}
	for _, sess := range plan.Working {
		c := choices[sess.ID]
		if (c == "" || c == QuitFinish) && sess.GroupKey != "" {
			groups[sess.GroupKey] = true
		}
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for len(groups) > 0 {
		for g := range groups {
			srv, err := s.Servers.Get(ctx, g)
			if err != nil || srv.Status != domain.ServerReady {
				delete(groups, g)
			}
		}
		if len(groups) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}
