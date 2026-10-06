.PHONY: build run test test-tui test-unit test-integration test-e2e test-container lint clean install embed-sync embed-bd

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
           -X github.com/datichb/openhub/cli/internal/buildinfo.Version=$(VERSION) \
           -X github.com/datichb/openhub/cli/internal/buildinfo.Commit=$(COMMIT) \
           -X github.com/datichb/openhub/cli/internal/buildinfo.BuildDate=$(DATE)

# Default target
all: build

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o bin/oh .

# Run without building a binary
run:
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

# Run all tests
test:
	go test ./... -v -race

# Run TUI tests only
test-tui:
	go test ./internal/tui/... -v -race

# Run unit tests (skip integration/long-running)
test-unit:
	go test ./... -v -race -short

# Contract tests against a real `opencode serve` (requires the opencode binary, no LLM call)
test-integration:
	go test ./... -race -count=1 -tags integration

# End-to-end tests with a real model (Bedrock EU Haiku; OH_E2E_BEDROCK_TOKEN or `opencode auth` key)
test-e2e:
	go test ./... -count=1 -tags "integration e2e" -run 'TestE2E'

# Container runtime tests against real engines (Colima and/or Podman running)
test-container:
	go test ./internal/runtime/... ./internal/runsvc/... -count=1 -tags container -v -run 'TestReal'

# Run tests with coverage
test-cover:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Vet
vet:
	go vet ./...

# Format
fmt:
	gofmt -s -w .

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

# Install to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" .

# Download dependencies
deps:
	go mod tidy
	go mod download

# Generate (if needed in the future)
generate:
	go generate ./...

# Sync hub content from repo root into the embed directory (for development).
# GoReleaser does this automatically at build time via its before-hook.
embed-sync:
	rm -rf internal/hubcontent/hub
	mkdir -p internal/hubcontent/hub
	cp -r ../agents internal/hubcontent/hub/agents
	cp -r ../skills internal/hubcontent/hub/skills
	cp -r ../permissions internal/hubcontent/hub/permissions
	cp -r ../workflows internal/hubcontent/hub/workflows
	$(MAKE) embed-bd

# Cross-compile the fake bd embedded in container images (linux/amd64, linux/arm64).
embed-bd:
	for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$$a go build -trimpath -ldflags "-s -w" -o internal/runtime/container/assets/bd-linux-$$a ./cmd/oh-bd-stub || exit 1; done
