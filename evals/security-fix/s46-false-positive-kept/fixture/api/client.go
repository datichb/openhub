// Package api appelle l'API du partenaire.
package api

import (
	"log"
	"net/http"
)

// Client appelle l'API du partenaire avec son jeton.
type Client struct {
	BaseURL string
	Token   string
}

// NewRequest prépare un appel authentifié.
func (c Client) NewRequest(path string) (*http.Request, error) {
	log.Printf("calling %s%s with token %s", c.BaseURL, path, c.Token)
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return req, nil
}
