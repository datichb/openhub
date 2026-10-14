# paiements

Encaisse les commandes auprès du prestataire de paiement.

- `payments/` : client du prestataire et relances.
- `ops/` : journaux d'exploitation exportés.
- Tests : `go test ./...`

## Exploitation

- Alerte : `payments_error_rate > 20 %` pendant 5 minutes.
