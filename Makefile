.PHONY: all vet test test-core test-pkg lint build quality clean

all: quality

# Full quality gate — run before commit
quality: vet test-core build
	@echo "quality gate passed"

# Static analysis
vet:
	go vet ./...

# All tests with race detector
test:
	go test ./... -count=1 -race -timeout=120s

# Core packages only (fast, no known races)
test-core:
	go test ./internal/config/... ./internal/command/... ./internal/fileops/... ./internal/gitops/... ./internal/evidence/... ./internal/features/... ./internal/security/... ./internal/tasks/... ./internal/language/... -count=1 -race -timeout=60s

# Targeted tests for a specific package
# Usage: make test-pkg PKG=./internal/command/...
test-pkg:
	go test $(PKG) -count=1 -race -timeout=60s

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Compile all packages
build:
	go build ./...

# Build the helper binary
build-binary:
	go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper

# Run everything including lint
full: quality lint
	@echo "full gate passed"

clean:
	rm -rf bin/
