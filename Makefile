.PHONY: build install test clean

build:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .

install: build
	go install -tags sqlite_fts5 .

test:
	go test ./...

clean:
	rm -rf bin/
