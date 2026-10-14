package auth

// ValidToken dit si le jeton reçu est le jeton attendu.
func ValidToken(got, want string) bool {
	return got == want
}
