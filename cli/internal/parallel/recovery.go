package parallel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// attemptRecovery checks for failed sessions eligible for retry and recovers them.
// Called from the monitor loop. Sessions with RetryCount >= MaxRetries are skipped.
func (c *Coordinator) attemptRecovery(ctx context.Context) {
	if c.opts.Config.MaxRetries <= 0 {
		return // recovery disabled
	}

	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusFailed {
			continue
		}
		if sess.RetryCount >= c.opts.Config.MaxRetries {
			continue // exhausted retries
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		c.recoverSession(ctx, sess)
	}
}

// recoverSession restarts a failed session by aborting the old task and launching a new one.
func (c *Coordinator) recoverSession(ctx context.Context, sess SessionInfo) {
	ticketID := sess.TicketID
	slog.Info("parallel: attempting recovery", "task", ticketID, "retry", sess.RetryCount+1, "maxRetries", c.opts.Config.MaxRetries)

	// 1. Transition to Retrying, preserve error history
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.Status = StatusRetrying
		s.RetryCount++
		s.LastRetryAt = time.Now().UTC()
		if s.Error != "" {
			s.RetryErrors = append(s.RetryErrors, s.Error)
		}
		s.Error = ""
		s.SessionID = ""
		s.CompletedAt = time.Time{}
	})

	// 2. Abort old task in the runner (best-effort)
	_ = c.runner.AbortTask(ctx, ticketID)

	// 3. Brief delay to let resources release
	if c.opts.Config.RetryDelaySeconds > 0 {
		select {
		case <-ctx.Done():
			c.state.UpdateSession(ticketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("retry %d: cancelled during delay", s.RetryCount)
			})
			return
		case <-time.After(time.Duration(c.opts.Config.RetryDelaySeconds) * time.Second):
		}
	}

	// 4. Build recovery prompt
	prompt := c.promptForTask(ticketID)
	retryNum := sess.RetryCount + 1
	if len(sess.FilesModified) > 0 {
		prompt += fmt.Sprintf("\n\n[RECOVERY] Cette session est une reprise après échec (tentative %d). "+
			"Des fichiers ont déjà été modifiés dans une tentative précédente : %v. "+
			"Vérifier leur état et compléter l'implémentation.",
			retryNum, sess.FilesModified)
	}

	// 5. Launch new task via the runner
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.Status = StatusStarting
	})

	handle, err := c.runner.LaunchTask(ctx, platform.TaskOpts{
		TaskID:       ticketID,
		Title:        fmt.Sprintf("parallel: %s (retry %d)", ticketID, retryNum),
		WorktreePath: sess.WorktreePath,
		Prompt:       prompt,
		Agent:        c.opts.Agent,
	})
	if err != nil {
		c.state.UpdateSession(ticketID, func(s *SessionInfo) {
			s.Status = StatusFailed
			s.Error = fmt.Sprintf("retry %d: %v", retryNum, err)
		})
		return
	}

	// 6. Success — back to running
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.SessionID = handle.SessionID
		s.Status = StatusRunning
		s.StartedAt = time.Now().UTC()
	})
}
