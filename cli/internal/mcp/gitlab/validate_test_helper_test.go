package gitlab

// SetSkipURLValidationForTest désactive la validation d'URL GitLab pour les tests
// qui utilisent httptest.NewServer (HTTP, non HTTPS).
// Uniquement disponible dans les tests (fichier _test.go).
func SetSkipURLValidationForTest(skip bool) {
	skipURLValidation = skip
}
