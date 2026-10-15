# Conventions

- Go standard (`gofmt`), sans dépendance externe.
- Vérification : `go test ./...`.
- Vocabulaire : un prêt (`Loan`) sort un document ; une réservation (`Reservation`) attend son tour dans une file. `Reserve` est un nom historique qui crée un prêt.
