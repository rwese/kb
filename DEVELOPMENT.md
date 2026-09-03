# kb — Development

How to build, test, and extend the kb CLI.

## Prerequisites

- Go 1.22+
- A C compiler (CGO) — the SQLite driver compiles C code
- SQLite with FTS5 — enabled at compile time via `SQLITE_ENABLE_FTS5` (pre-installed on macOS; on Linux install `libsqlite3-dev`)

FTS5 is required at **runtime**: the schema creates an `articles_fts` virtual table. Builds or tests without the FTS5 define compile but fail when the database schema is created.

## Build & test

```bash
# Build
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .

# Test — the same CGO flags are required
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
```

`./build.sh` is a thin wrapper: `./build.sh` builds `bin/kb`, `./build.sh install` installs to `$GOPATH/bin`.

### just recipes

| Recipe | Runs |
|--------|------|
| `just build` | build with FTS5 flags (sets `CGO_CFLAGS` itself) |
| `just build-v VERSION=` | build with version ldflag |
| `just install` | `go install` to `$GOPATH/bin` |
| `just test` | `go test ./...` |
| `just test-verbose` | `go test -v ./...` |
| `just fmt` | `go fmt ./...` |
| `just lint` | `golangci-lint run` |
| `just tidy` | `go mod tidy` |
| `just check` | fmt + lint + test |
| `just clean` | remove `bin/` |
| `just release` | darwin-arm64, darwin-amd64, linux-amd64 into `dist/` |

### Gotchas

- `just test` does **not** set `CGO_CFLAGS`. Export it first (`export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"`) so tooling runs consistently.
- `golangci-lint` must be installed separately for `just lint` / `just check`.
- The `sqlite_fts5` tag is required, not decorative: it activates `sqlite3_opt_fts5.go` in mattn/go-sqlite3, which adds the FTS5 define **and `-lm`** (the link flag Linux needs for FTS5's `log()`). Without the tag on Linux, linking fails with `undefined reference to 'log'`; macOS does not need `-lm` so it masks the error. Always pass both the tag and the CGO define.

## Project layout

| Path | Role |
|------|------|
| `main.go` | entrypoint; wires `cmd.Commands` |
| `cmd/` | urfave/cli/v3 command definitions and orchestration |
| `cmd/*_test.go` | CLI-level integration tests (hermetic, see below) |
| `internal/db/` | SQLite storage: entries, articles, assets, FTS index, vectors |
| `internal/search/` | BM25 and hybrid ranking (`ranker.go`) |
| `internal/embed/` | embedding providers: `ollama.go` (server) and `local.go` (bundled model) |
| `internal/assets/` | asset import/staging/cleanup (KB-owned copies) |
| `internal/id/` | id generation (6 hex chars) |
| `internal/config/` | config discovery, defaults, `~` expansion |
| `docs/prds/` | feature specifications (export, local embeddings) |
| `skills/knowledgebase/` | the user-facing agent skill shipped with the repo |

## CLI surface

```
kb
├── entry
│   ├── list | create | get | update | delete     # entries
│   └── article
│       ├── list | add | get | update | delete    # articles
│       └── asset
│           └── add | list | get | delete         # attached files
├── search        # weighted retrieval (see Ranking)
├── export        # Obsidian markdown
├── download      # fetch local embedding model
├── init | status | stats | config                # database & install
└── delete entry  # alias for `entry delete`
```

Use `kb <command> --help` for current flags; commands change faster than docs.

## Conventions

### IDs

- Entries: 6 hex chars, e.g. `2f018d`
- Articles: `entryID-articleID`, e.g. `2f018d-273b00`
- Auto-detected by presence of `-`: with → article, without → entry

### Config discovery

Order: `$KB_PATH` env override → `~/.config/kb/config.yaml` → `.kb.yaml` (cwd) → built-in defaults. `~` in paths is expanded.

### Hermetic tests

Tests never touch a real knowledgebase. `setupTempKBTestEnv` (in `cmd/test_helpers_test.go`) creates a temp dir via `t.TempDir()` and points the CLI at it with `KB_PATH` and `HOME` env overrides. New tests that touch the database or assets must follow this pattern.

### Asset model

`asset add` copies files (directories are walked) into KB-owned storage under `<assets_path>/<articleID>/<assetID>/<logical path>`. Symlinks are rejected, path traversal is blocked, and a colliding logical path requires `--overwrite` (which replaces the stored file tree). Imports are staged in a temp area first and cleaned up on failure.

### Ranking (current implementation)

1. BM25 search over the FTS index (`top_k * 2` candidates).
2. If an embedder is configured and `--bm25-only` is not set, embed the query and re-rank: BM25 scores are normalized to 0–1 and blended with the query-vector similarity using `bm25_weight` (default 0.3) and `semantic_weight` (default 0.7) from the `local:` config block; weights are normalized to sum to 1.
3. `--prompt` text is used as the query (falls back to the positional arg). The `--context`/`--context-file` flags exist but are currently unused. 

If no query embedding is available (e.g. the local model is not downloaded), search falls back to BM25 results only.

## Writing tests

Existing coverage is CLI-level (`cmd/*_test.go`) and unit-level (`internal/*/*_test.go`), table-driven where practical. Run the whole suite with the CGO flags from [Build & test](#build--test).

## PRDs

Feature specs live under `docs/prds/` (e.g. `export/`, `local-embeddings/`) as the source of truth for behavior. Update the matching PRD when a feature's behavior changes.