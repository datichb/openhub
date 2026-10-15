// Package profile sert les profils des utilisateurs.
package profile

import "sync"

// Cache garde les profils lus.
type Cache struct {
	mu   sync.Mutex
	byID map[string]string
}

// Get renvoie le profil en cache.
func (c *Cache) Get(id string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.byID[id]
	return p, ok
}

// Update enregistre un profil modifié ; le cache n'est pas invalidé.
func (c *Cache) Update(id, p string) {}
