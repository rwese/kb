# KB Agent Configuration

Knowledgebase CLI in Go: entries (topics) with articles (notes) and assets (files), SQLite+FTS5 storage, weighted retrieval.

## Workflow

### 1. Build - FTS5 flags on every compile

The schema uses `fts5`, so a build or test without the FTS5 define compiles but dies at runtime. Always pass both:

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .
```

Done when `bin/kb status` exits 0.

### 2. Test - same flags, hermetic env

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
```

Done when every package reports `ok`.

Tests are hermetic: `setupTempKBTestEnv` (in `cmd/test_helpers_test.go`) points the CLI at a `t.TempDir()` via `KB_PATH` and `HOME` overrides. New tests that touch the database or assets follow that pattern - set up an isolated env like it, never hit a real knowledgebase.

### 3. Green gate

```bash
export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
just check    # fmt + lint + test (needs golangci-lint installed)
```

Done when gofmt reports no changes, golangci-lint passes, and `go test ./...` is green.

## Project map

| Path | Role |
|------|------|
| `cmd/` | CLI commands (urfave/cli/v3) + integration tests |
| `internal/db/` | SQLite: entries, articles, assets, FTS index, vectors |
| `internal/search/` | BM25 + hybrid ranking |
| `internal/embed/` | embedding provider: ollama |
| `internal/assets/` | asset copies, staging, cleanup |
| `internal/config/` | config discovery + defaults |
| `internal/id/` | id generation |
| `docs/prds/` | behavior specs - keep in step with code |

## Conventions

- **IDs**: 6 hex chars. A `-` in an id means article (`2f018d-273b00`); without is an entry.
- **Config order**: `$KB_PATH` → `~/.config/kb/config.yaml` → `.kb.yaml` → defaults.
- **Assets**: `asset add` copies files into KB-owned storage; symlinks are rejected, a logical-path collision needs `--overwrite`.
- **CLI reference**: read `kb <command> --help` for flags - commands change faster than docs. The command tree plus build/release details live in DEVELOPMENT.md; the user-facing command table in README.md.

## References

- `DEVELOPMENT.md` - build, release, gotchas, ranking internals
- `README.md` - user-facing docs
- `docs/prds/*.md` - feature specs; update with behavior changes
- `skills/knowledgebase/SKILL.md` - user-facing agent skill; update with behavior changes