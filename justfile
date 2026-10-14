# KB - Knowledgebase CLI

set shell := ["bash", "-cu"]
set export := true
set default-list

# Build variables
BIN := "bin/kb"
LDFLAGS := ""

# Build kb binary
build:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o {{BIN}} ./cmd/kb

# Build with version
build-v VERSION="dev":
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -ldflags "-X main.version={{VERSION}}" -o {{BIN}} ./cmd/kb

# Install to $GOPATH/bin
install:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go install -tags sqlite_fts5 ./cmd/kb

# Run tests
# NOTE: -tags sqlite_fts5 is required - enables FTS5 + -lm on Linux (link error without)
test:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -tags sqlite_fts5 ./...

# Run tests with verbose output
test-verbose:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -tags sqlite_fts5 -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Full quality check
check: fmt lint test

# Run the ranking matrix (bm25 + hybrid weight balances) on a dataset and
# print a comparison table; hybrid runs need Ollama + embedder: ollama
bench-matrix DATASET TOP_K="3":
    ./scripts/bench-matrix.sh {{DATASET}} --top-k {{TOP_K}}

# Default recipe
default: build

# Release build for macOS (arm64) - native on this machine
release-darwin-arm64 VERSION="":
	scripts/build.sh --os darwin --arch arm64 --version "{{VERSION}}"

# Release build for macOS (amd64) - native on this machine
release-darwin-amd64 VERSION="":
	scripts/build.sh --os darwin --arch amd64 --version "{{VERSION}}"

# Linux release builds run in docker: CGO cannot be crossed from macOS, and
# a cgo-less linux build compiles but dies at runtime (see AGENTS.md).
release-linux VERSION="dockerbuild":
	docker run --rm -v "$(pwd)":/src -w /src --entrypoint bash golang:1.25-bookworm \
		scripts/build.sh --os linux --arch amd64 --version "{{VERSION}}"

# Build all release binaries
release: release-darwin-arm64 release-darwin-amd64 release-linux
