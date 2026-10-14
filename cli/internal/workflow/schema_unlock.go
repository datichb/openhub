package workflow

// A second commit checkpoint (p5-fixes, `unlock_again`). An operation is
// unlocked by one checkpoint (unlock_duplicate), except the commit: a
// later checkpoint that declares `unlock_again: true` unlocks it again
// (incident: the commit of the mitigation at cp-mitigated, then the one of
// the postmortem at cp-postmortem). The window of the earlier checkpoint
// is then bounded to one commit: oh does not open it when there is nothing
// to commit, closes it after the first commit and, at the latest, when the
// next checkpoint that unlocks the commit is asked (services/checkpoint).
// The repeated commit adds no irreversible pause to the budget (CP-R2).

// CommitAgain returns the checkpoint after id that unlocks the commit again
// (`unlock_again`), when id unlocks the commit: "" otherwise. Disabled
// checkpoints are left out.
func (s *Spec) CommitAgain(id string) string {
	keys := s.Checkpoints.Keys()
	for i, k := range keys {
		if k != id {
			continue
		}
		if cp, _ := s.Checkpoints.Get(k); cp.Disabled || !containsOp(cp.Unlocks, UnlockCommit) {
			return ""
		}
		for _, next := range keys[i+1:] {
			cp, _ := s.Checkpoints.Get(next)
			if cp.Disabled || !containsOp(cp.Unlocks, UnlockCommit) {
				continue
			}
			if cp.UnlockAgain {
				return next
			}
			return "" // unlock_duplicate
		}
	}
	return ""
}

// repeatsCommit reports whether the commit cp unlocks repeats an earlier
// one (it adds no irreversible pause).
func (cp CheckpointSpec) repeatsCommit() bool {
	return cp.UnlockAgain && containsOp(cp.Unlocks, UnlockCommit)
}
