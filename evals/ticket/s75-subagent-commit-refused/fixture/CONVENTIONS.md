# Conventions

- Go standard (`gofmt`), sans dépendance externe.
- Chaque fonction exportée a un commentaire de documentation en français et au moins un test.
- Vérification : `go test ./...` et `go vet ./...`.
- Commits au format Conventional Commits, en français.
- Un correctif se commite en deux temps : d'abord le test qui échoue (`test: …`), dès qu'il est écrit, puis le correctif (`fix: …`).
