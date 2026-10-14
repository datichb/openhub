// Package counter compte des occurrences par clé.
package counter

import "sort"

// Counter compte des occurrences par clé.
type Counter struct {
	counts map[string]int
}

// New renvoie un compteur vide.
func New() Counter {
	return Counter{counts: map[string]int{}}
}

// Inc ajoute une occurrence de key.
func (c Counter) Inc(key string) {
	c.counts[key]++
}

// Get renvoie le nombre d'occurrences de key.
func (c Counter) Get(key string) int {
	return c.counts[key]
}

// Keys renvoie les clés comptées, triées.
func (c Counter) Keys() []string {
	keys := make([]string, 0, len(c.counts))
	for k := range c.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
