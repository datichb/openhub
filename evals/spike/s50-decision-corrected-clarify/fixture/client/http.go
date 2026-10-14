// Package client appelle le service de tarifs.
package client

import (
	"net/http"
	"time"
)

// Timeout est le délai maximal d'un appel au service de tarifs.
const Timeout = 5 * time.Second

// New renvoie le client HTTP du service de tarifs.
func New() *http.Client {
	return &http.Client{Timeout: Timeout}
}
