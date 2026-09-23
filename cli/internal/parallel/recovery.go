package parallel

import (
	"context"
	"fmt"
	"time"
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

		// Check context before attempting recovery
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.recoverSession(ctx, sess)
	}
}

// recoverSession restarts a failed session on the same worktree with a new server and port.
func (c *Coordinator) recoverSession(ctx context.Context, sess SessionInfo) {
	ticketID := sess.TicketID

	// 1. Transition to Retrying, preserve error history
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.Status = StatusRetrying
		s.RetryCount++
		s.LastRetryAt = time.Now().UTC()
		if s.Error != "" {
			s.RetryErrors = append(s.RetryErrors, s.Error)
		}
		s.Error = ""
		s.SessionID = ""          // will get a new session
		s.CompletedAt = time.Time{} // reset
	})

	// 2. Kill old server (best-effort)
	oldSrv := c.findServer(ticketID)
	if oldSrv != nil {
		_ = oldSrv.Dispose()
		oldSrv.Kill()
	}

	// 3. Brief delay to let port/resources release
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

	// 4. Compute new port (offset avoids the old port)
	newPort := sess.Port + 10 + sess.RetryCount

	// 5. Create new server on the same worktree
	newSrv := NewServer(newPort, sess.WorktreePath, ticketID)
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.Status = StatusStarting
		s.Port = newPort
	})

	if err := newSrv.Start(ctx, c.opencodeBin); err != nil {
		c.state.UpdateSession(ticketID, func(s *SessionInfo) {
			s.Status = StatusFailed
			s.Error = fmt.Sprintf("retry %d: failed to start server: %v", s.RetryCount, err)
		})
		return
	}

	// 6. Replace server in slice
	c.replaceServer(ticketID, newSrv)

	// 7. Wait ready (shorter timeout for retry -- worktree already exists)
	if err := newSrv.WaitReady(ctx, 15*time.Second); err != nil {
		c.state.UpdateSession(ticketID, func(s *SessionInfo) {
			s.Status = StatusFailed
			s.Error = fmt.Sprintf("retry %d: server did not become ready: %v", s.RetryCount, err)
		})
		newSrv.Kill()
		return
	}

	// 8. Create session
	retryNum := sess.RetryCount + 1
	title := fmt.Sprintf("parallel: %s (retry %d)", ticketID, retryNum)
	sessionID, err := newSrv.CreateSession(title)
	if err != nil {
		c.state.UpdateSession(ticketID, func(s *SessionInfo) {
			s.Status = StatusFailed
			s.Error = fmt.Sprintf("retry %d: failed to create session: %v", s.RetryCount, err)
		})
		return
	}

	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.SessionID = sessionID
	})

	// 9. Build recovery prompt -- include context about partial work
	prompt := c.promptForTask(ticketID)
	if len(sess.FilesModified) > 0 {
		prompt += fmt.Sprintf("\n\n[RECOVERY] Cette session est une reprise après échec (tentative %d). "+
			"Des fichiers ont déjà été modifiés dans une tentative précédente : %v. "+
			"Vérifier leur état et compléter l'implémentation.",
			retryNum, sess.FilesModified)
	}

	if err := newSrv.SendPromptAsync(sessionID, prompt, c.opts.Agent); err != nil {
		c.state.UpdateSession(ticketID, func(s *SessionInfo) {
			s.Status = StatusFailed
			s.Error = fmt.Sprintf("retry %d: failed to send prompt: %v", s.RetryCount, err)
		})
		return
	}

	// 10. Success -- back to running
	c.state.UpdateSession(ticketID, func(s *SessionInfo) {
		s.Status = StatusRunning
		s.StartedAt = time.Now().UTC()
	})
}
