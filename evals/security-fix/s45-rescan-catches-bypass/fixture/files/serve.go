// Package files sert les fichiers déposés par les clients.
package files

import (
	"os"
	"path/filepath"
)

// Open ouvre le fichier name du dossier des dépôts root.
func Open(root, name string) (*os.File, error) {
	return os.Open(filepath.Join(root, name))
}
