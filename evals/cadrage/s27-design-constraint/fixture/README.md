# shop

Service de commandes en Go, sans dépendance externe.

- `internal/order` : les commandes et leurs règles.
- `internal/mail` : l'envoi d'e-mails (SMTP).
- `cmd/shop` : le programme, qui assemble les paquets.

Tests : `go test ./...` ; lint : `golangci-lint run`
