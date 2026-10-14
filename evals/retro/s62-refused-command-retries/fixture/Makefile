.PHONY: test test-integration

test:
	go test ./...

# Base Postgres locale, puis les tests marqués integration.
test-integration:
	docker compose up -d db
	go test -tags integration ./...
