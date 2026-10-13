// Package tasks suit les tâches d'une équipe.
package tasks

import "time"

// Task est une tâche de l'équipe.
type Task struct {
	ID      int
	Title   string
	Author  string
	Done    bool
	Private bool       // visible de son autrice seulement
	Deleted *time.Time // date de mise à la corbeille, nil sinon
	Updated time.Time
}

// Store garde les tâches en mémoire.
type Store struct {
	tasks []Task
}

// Add ajoute une tâche et renvoie son identifiant.
func (s *Store) Add(t Task) int {
	t.ID = len(s.tasks) + 1
	s.tasks = append(s.tasks, t)
	return t.ID
}

// Visible renvoie les tâches que viewer peut voir : les tâches privées des
// autres membres et celles de la corbeille sont exclues.
func (s *Store) Visible(viewer string) []Task {
	var out []Task
	for _, t := range s.tasks {
		if t.Deleted != nil || (t.Private && t.Author != viewer) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Delete met une tâche à la corbeille.
func (s *Store) Delete(id int, now time.Time) {
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks[i].Deleted = &now
		}
	}
}

// Purge supprime définitivement les tâches restées 30 jours à la corbeille.
func (s *Store) Purge(now time.Time) {
	var kept []Task
	for _, t := range s.tasks {
		if t.Deleted != nil && now.Sub(*t.Deleted) >= 30*24*time.Hour {
			continue
		}
		kept = append(kept, t)
	}
	s.tasks = kept
}
