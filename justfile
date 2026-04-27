# KB - Knowledgebase CLI

set shell := ["bash", "-cu"]
set export := true

# Build variables
BIN := "bin/kb"
LDFLAGS := ""

# Build kb binary
build:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o {{BIN}} .

# Build with version
build-v VERSION="dev":
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -ldflags "-X main.version={{VERSION}}" -o {{BIN}} .

# Install to $GOPATH/bin
install:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go install -tags sqlite_fts5 .

# Run tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

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

# Default recipe
default: build

# Release build for macOS (arm64)
release-darwin-arm64:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" GOOS=darwin GOARCH=arm64 go build -tags sqlite_fts5 -ldflags "-s -w" -o dist/kb-darwin-arm64 .

# Release build for macOS (amd64)
release-darwin-amd64:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" GOOS=darwin GOARCH=amd64 go build -tags sqlite_fts5 -ldflags "-s -w" -o dist/kb-darwin-amd64 .

# Release build for Linux
release-linux:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" GOOS=linux GOARCH=amd64 go build -tags sqlite_fts5 -ldflags "-s -w" -o dist/kb-linux-amd64 .

# Build all release binaries
release: release-darwin-arm64 release-darwin-amd64 release-linux
