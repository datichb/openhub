.PHONY: build build-linux run test test-tui test-unit test-integration test-e2e test-container lint clean install embed-sync embed-bd sync-content

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

# Linux oh binary for remote sessions (development builds):
# oh remote setup --oh-binary bin/oh-linux-<arch>
ARCH ?= amd64
build-linux: embed-bd
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -trimpath -ldflags "$(LDFLAGS)" -o bin/oh-linux-$(ARCH) .

# Run without building a binary
run:
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

# Run all tests (the full -race suite takes longer than the default 10 minutes)
TEST_TIMEOUT ?= 40m
test:
	go test ./... -v -race -timeout $(TEST_TIMEOUT)

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
# Every finding, on the three systems and with the test build tags.
lint:
	golangci-lint run --build-tags integration,e2e,container ./...
	GOOS=linux golangci-lint run --build-tags integration,e2e,container ./...
	GOOS=windows golangci-lint run --build-tags integration,e2e,container ./...

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

# Copy the hub content (agents, skills, permissions, workflows) from the repo
# root into the embed directory, without rebuilding the embedded bd.
# TestEmbeddedContentInSync fails while the copy differs from the sources.
sync-content:
	rm -rf internal/hubcontent/hub
	mkdir -p internal/hubcontent/hub
	cp -r ../agents ../skills ../permissions ../workflows internal/hubcontent/hub/

# Sync hub content and rebuild the embedded bd (for development).
# GoReleaser does this automatically at build time via its before-hook.
embed-sync: sync-content
	$(MAKE) embed-bd

# Cross-compile the fake bd embedded in container images (linux/amd64, linux/arm64).
embed-bd:
	for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$$a go build -trimpath -ldflags "-s -w" -o internal/runtime/container/assets/bd-linux-$$a ./cmd/oh-bd || exit 1; done

# ── Behavioural evaluations (P0.5) ────────────────────────────────────────
# Real model calls (Bedrock EU): Haiku by default, OH_EVAL_MODELS for others
# (e.g. claude-sonnet-4-6), OH_EVAL_REPS repetitions, OH_EVAL_BUDGET_USD cap
# per pass (default 1, 2.50 with a Sonnet model), OH_EVAL_CASES to select
# (S1,S2), OH_EVAL_PARALLEL passes at the same time, OH_EVAL_BUDGET_TOTAL cap
# of the whole run. OH_EVAL_RECORD=1 records the reference measure
# (OH_EVAL_NOTE: its note); OH_EVAL_OUT keeps the traces and reports there.
# OH_EVAL_VARIANTS compares versions of the hub, separated by spaces, the
# first one being the reference (e.g. "base sans-23=-skill:debugger-phase-2-3").
# OH_EVAL_TIERED=1 runs a reference by tiers: a case at 100 % on its last two
# measures of the baseline runs once, the others OH_EVAL_REPS times (3 by
# default), a case run once that fails runs again up to that (escalation;
# OH_EVAL_ESCALATE=0 turns it off). OH_EVAL_REPS_CASE forces the repetitions
# of some cases (S5=3,S15=3). OH_EVAL_PLAN=1 prints the passes per case and
# the estimated cost without running anything (OH_EVAL_COSTS: other
# report.json, glob patterns separated by spaces, for the costs of the cases
# not in the baseline). OH_EVAL_HISTORY: other report.json whose measures
# count in the tiered plan after those of the baseline, by date (the passes
# of the lots since the last reference), and in the cost estimate;
# OH_EVAL_STABLE_MEASURES the number of last measures at 100 % that let a
# case run once (2 by default).
EVAL_BASELINE := internal/evalrun/testdata/evals/baseline.json

.PHONY: test-eval
test-eval:
	go build -ldflags "$(LDFLAGS)" -o bin/oh .
	./bin/oh eval --cases ../evals --baseline $(EVAL_BASELINE) \
		$(if $(OH_EVAL_REPS),--reps $(OH_EVAL_REPS)) \
		$(if $(filter-out 0 false no,$(OH_EVAL_TIERED)),--tiered) \
		$(if $(OH_EVAL_ESCALATE),--escalate=$(if $(filter 0 false no,$(OH_EVAL_ESCALATE)),false,true)) \
		$(if $(OH_EVAL_REPS_CASE),--reps-case $(OH_EVAL_REPS_CASE)) \
		$(if $(filter-out 0 false no,$(OH_EVAL_PLAN)),--plan) \
		$(foreach c,$(OH_EVAL_COSTS),--costs '$(c)') \
		$(foreach c,$(OH_EVAL_HISTORY),--history '$(c)') \
		$(if $(OH_EVAL_STABLE_MEASURES),--stable-measures $(OH_EVAL_STABLE_MEASURES)) \
		$(if $(OH_EVAL_CASES),--case $(OH_EVAL_CASES)) \
		$(if $(OH_EVAL_PARALLEL),--parallel $(OH_EVAL_PARALLEL)) \
		$(if $(OH_EVAL_BUDGET_TOTAL),--budget-total $(OH_EVAL_BUDGET_TOTAL)) \
		$(if $(OH_EVAL_OUT),--out $(OH_EVAL_OUT)) \
		$(if $(OH_EVAL_NOTE),--note "$(OH_EVAL_NOTE)") \
		$(foreach v,$(OH_EVAL_VARIANTS),--variant '$(v)') \
		$(if $(OH_EVAL_RECORD),--record-baseline $(EVAL_BASELINE))
