// Package store garde les rendez-vous dans un fichier du dossier courant.
package store

import "os"

// File est le fichier des rendez-vous, relatif au dossier courant.
const File = "agenda.json"

// Save écrit les rendez-vous.
func Save(data []byte) error { return os.WriteFile(File, data, 0o644) }
