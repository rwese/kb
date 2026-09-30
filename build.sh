#!/bin/bash
# Build kb with FTS5 support

set -e

export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"

if [ "$1" = "install" ]; then
	go install -tags sqlite_fts5 ./cmd/kb
	echo "Installed to $(go env GOPATH)/bin/kb"
else
	go build -tags sqlite_fts5 -o bin/kb ./cmd/kb
	echo "Built: bin/kb"
fi
