package workflow

import "slices"

// Breakdown of a session read by oh (sweep-fanout). A checkpoint with
// `tasks:` is passed with its breakdown in the call (`tasks` of
// workflow_checkpoint): a title, a file scope, a domain and, outside a
// fan-out, whether the task may run at the same time as others. oh checks
// it before any card is shown (the scope of each task names files of the
// repository, or a new file in an existing folder; two tasks that may run
// together never share a file; at most `max` tasks; a known domain), shows
// it on the card as oh read it, and records it once the checkpoint passes:
// output `record: tasks`, and the children of a fan-out `source: tasks`.
//
//	checkpoints:
//	  cp-plan:
//	    mandatory: true
//	    tasks: { max: 12, domains: [backend, frontend, refactor, tests] }
//	outputs:
//	  - { id: tasks, type: text, record: tasks }

// TasksCheck is the check of the breakdown given at a checkpoint.
type TasksCheck struct {
	// Max is the number of tasks allowed (default DefaultMaxTasks, at most
	// MaxTasks).
	Max int `yaml:"max,omitempty"`
	// Domains are the domains a task may name (empty: any, and none
	// required).
	Domains []string `yaml:"domains,omitempty"`
}

// Bounds of a breakdown.
const (
	DefaultMaxTasks = 12
	MaxTasks        = 20
)

// EffectiveMax is Max bounded to [1, MaxTasks] (DefaultMaxTasks when unset).
func (t *TasksCheck) EffectiveMax() int {
	if t == nil || t.Max <= 0 {
		return DefaultMaxTasks
	}
	return min(t.Max, MaxTasks)
}

func (t *TasksCheck) clone() *TasksCheck {
	if t == nil {
		return nil
	}
	c := *t
	c.Domains = slices.Clone(t.Domains)
	return &c
}

// TasksCheckpoint returns the checkpoint that reads the breakdown of the
// session ("" = none; the first one when several, refused by validation).
func (s *Spec) TasksCheckpoint() string {
	if s == nil {
		return ""
	}
	for _, k := range s.Checkpoints.Keys() {
		if cp, _ := s.Checkpoints.Get(k); cp.Tasks != nil && !cp.Disabled {
			return k
		}
	}
	return ""
}
