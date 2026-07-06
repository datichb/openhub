.PHONY: build run test lint clean install

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
           -X github.com/datichb/openhub/cli/cmd.Version=$(VERSION) \
           -X github.com/datichb/openhub/cli/cmd.Commit=$(COMMIT) \
           -X github.com/datichb/openhub/cli/cmd.BuildDate=$(DATE)

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
