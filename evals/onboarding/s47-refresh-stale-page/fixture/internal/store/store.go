// Package store garde les tâches dans un fichier texte, une tâche par ligne.
package store

import (
	"errors"
	"os"
	"strings"
)

// Store est le fichier des tâches.
type Store struct{ path string }

// Open renvoie le Store du fichier path (créé au premier ajout).
func Open(path string) *Store { return &Store{path: path} }

// Add ajoute une tâche à la fin du fichier.
func (s *Store) Add(task string) error {
	if strings.TrimSpace(task) == "" {
		return errors.New("tâche vide")
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(task + "\n")
	return err
}

// List renvoie les tâches dans l'ordre d'ajout (aucune si le fichier n'existe pas).
func (s *Store) List() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n"), nil
}
