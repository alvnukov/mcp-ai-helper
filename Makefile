.PHONY: all vet test test-short test-pkg lint build build-binary quality full clean

all: quality

# Full quality gate — run before commit
quality: vet test-short build
	@echo "quality gate passed"

# Static analysis
vet:
	go vet ./...

# Everything, including the tests that drive a real Lean toolchain.
#
# No -race here, which is what CI runs too. These tests are dominated by lake
# subprocesses rather than by Go concurrency, so the detector finds nothing it
# does not already find in test-short — and instrumenting them makes the run
# long enough to be useless as a gate.
test:
	go test ./... -count=1 -timeout=1200s

# The fast loop: every package, minus the tests that build and run Lean.
# Defined by exclusion rather than by a list of packages, so a new package is
# covered the day it is added instead of the day someone remembers the Makefile.
test-short:
	go test ./... -count=1 -race -short -timeout=300s

# Targeted tests for a specific package
# Usage: make test-pkg PKG=./internal/command/...
test-pkg:
	go test $(PKG) -count=1 -race -timeout=120s

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Compile all packages
build:
	go build ./...

# Build the helper binary
build-binary:
	go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper

# Run everything, including the Lean tests and lint
full: vet test build lint
	@echo "full gate passed"

clean:
	rm -rf bin/
