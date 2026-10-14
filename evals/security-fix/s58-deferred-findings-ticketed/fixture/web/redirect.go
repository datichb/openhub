// Package web gère la redirection après connexion.
package web

// SafeNext renvoie la page où rediriger après la connexion (paramètre next).
func SafeNext(next string) string {
	if next == "" {
		return "/"
	}
	return next
}
