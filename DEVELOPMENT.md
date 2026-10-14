# kb - Development

How to build, test, and extend the kb CLI.

## Prerequisites

- Go 1.22+
- A C compiler (CGO) - the SQLite driver compiles C code
- SQLite with FTS5 - enabled at compile time via `SQLITE_ENABLE_FTS5` (pre-installed on macOS; on Linux install `libsqlite3-dev`)

FTS5 is required at **runtime**: the schema creates an `articles_fts` virtual table. Builds or tests without the FTS5 define compile but fail when the database schema is created.

## Build & test

```bash
# Build
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .

# Test - the same CGO flags are required
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
- Rebuild `golangci-lint` with the active Go toolchain (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`). A binary built with an older Go panics at load on newer toolchains (`file requires newer Go version go1.27 (application built with go1.26)`).
- The `sqlite_fts5` tag is required, not decorative: it activates `sqlite3_opt_fts5.go` in mattn/go-sqlite3, which adds the FTS5 define **and `-lm`** (the link flag Linux needs for FTS5's `log()`). Without the tag on Linux, linking fails with `undefined reference to 'log'`; macOS does not need `-lm` so it masks the error. Always pass both the tag and the CGO define.

## Project layout

| Path | Role |
|------|------|
| `cmd/kb/main.go` | entrypoint; wires `cli.Commands` |
| `internal/cli/` | urfave/cli/v3 root assembly (`cli.go`) + CLI-level integration tests |
| `internal/cli/{entry,article,asset,attachment,search,export,system}/` | subcommand definitions, one package per domain |
| `internal/cli/shared/` | cross-cutting CLI helpers (db access, view types, printers) |
| `internal/cli/bench/` | hidden `kb bench run` command (JSONL benchmark reports) |
| `internal/db/` | SQLite storage: entries, articles, assets, attachments, FTS index, vectors |
| `internal/search/` | BM25, hybrid ranking, and per-set score normalization (`ranker.go`) |
| `internal/benchmark/` | JSONL retrieval benchmarks: dataset parser, metrics, isolated runner, baseline gate (`docs/prds/benchmark/`) |
| `internal/embed/` | embedding providers: `ollama.go` (server) |
| `internal/assets/` | asset + attachment import/staging/cleanup (KB-owned copies) |
| `internal/id/` | id generation (6 hex chars) |
| `internal/config/` | config discovery, defaults, `~` expansion |
| `docs/prds/` | feature specifications (export) |
| `skills/knowledgebase/` | the user-facing agent skill shipped with the repo |

## CLI surface

```
kb
├── entry
│   ├── list | create | get | update | delete     # entries
│   ├── article
│   │   ├── list | add | get | update | delete    # articles
│   │   └── asset
│   │       └── add | list | get | delete         # attached files
│   └── attachment
│       └── add | list | get | update | delete    # titled entry files
├── search        # weighted retrieval (see Ranking)
├── export        # Obsidian markdown
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

Tests never touch a real knowledgebase. `setupTempKBTestEnv` (in `internal/cli/testenv_test.go`) creates a temp dir via `t.TempDir()` and points the CLI at it with `KB_PATH` and `HOME` env overrides. New tests that touch the database or assets must follow this pattern.

### Asset model

`asset add` copies files (directories are walked) into KB-owned storage under `<assets_path>/<articleID>/<assetID>/<logical path>`. Symlinks are rejected, path traversal is blocked, and a colliding logical path requires `--overwrite` (which replaces the stored file tree). Imports are staged in a temp area first and cleaned up on failure.

### Attachment model

`attachment add` copies **exactly one regular file** into KB-owned storage under `<assets_path>/entries/<entryID>/attachments/<attachmentID>/<file name>` - a namespace separate from article assets. Directories, symlinks, sockets, devices, and named pipes are rejected; the title is trimmed and must be non-empty; SHA-256, byte size, and regular permission bits (including the executable bit; special bits stripped) are recorded. The file is treated as opaque untrusted bytes: never inspected, sourced, or executed. Replacement and deletion stage new bytes before the metadata commit and return an explicit partial-failure error when the old stored bytes cannot be removed.

### Ranking (ground truth)

This section is the authoritative description of how retrieval works, documented from the code (`internal/db/sqlite.go`, `internal/search/ranker.go`, `internal/cli/search/search.go`). It supersedes the earlier local-embeddings PRD (removed in `91e83ab`); the 0.3/0.7 split and the design caveats below are inherited from that PRD's defaults (`bf413f5`) and have not been revisited since.

**Pipeline**

1. **BM25 retrieval** - `SearchWithDeleted(query, topK*2, all)` runs two FTS5 queries (external-content, porter tokenizer): `articles_fts` (title + content) and `entry_attachments_fts` (title + file_name only; binary contents are never indexed). Both are kept in sync purely by triggers (`articles_ai/au/ad`, `entry_attachments_ai/au/ad`) - there is no background reindex.
2. **Per-set normalization** - each candidate set is linearly normalized to 0-1 at the database layer (`normalizeScoreSet`) before merging; raw BM25 values from separate FTS tables are not comparable. A single-hit (or all-equal) set maps to 1.0.
3. **Hybrid re-rank (`Ranker.HybridSearch`, article hits only)** - if `cfg.Embedder == "ollama"` and `--bm25-only` is not set, embed the query and re-rank article hits only; attachment hits keep their per-set normalized BM25 score:
   - `BM25Score` = min-max normalized over the article hits of *this query* (best hit 1.0, worst 0.0, a lone hit 0.5) - plain linear rescaling, `NormalizeBM25` in `ranker.go`.
   - `SemanticScore` = raw cosine similarity between query and article vector (`db.GetVector` + `embed.Cosine`), *not* min-max normalized. Nomic-embed-text cosines cluster roughly 0.6-0.95 for real matches.
   - `Score = BM25Weight * BM25Score + SemanticWeight * SemanticScore`, then a global descending sort mixes article and attachment hits.

**What the weights mean**

- `BM25Weight = 0.3` (`search.DefaultRanker`, hardcoded, `NewRanker` constructor is not wired to config): lexical signal - exact keyword match strength as the precision/tiebreak component.
- `SemanticWeight = 0.7`: meaning signal - paraphrase/intent match via embeddings as the dominant component.

**Fallbacks**

- Missing or erroring article vector: semantic score 0, result caps at `0.3 * BM25Score`.
- Query embedding failure: silently degrades to the merged BM25 results.
- `embedder: none` (the config default): no semantic tier at all, BM25-only.

**Known limitations** (candidate optimization targets)

- The fused `Score` is per-query relative: BM25 is rescaled to whatever hits came back, so scores are not comparable across queries.
- Scale asymmetry: weighted 0.3 of a spread-out 0-1 keyword score vs 0.7 of a tight ~0.6-0.95 cosine band - semantics decides more than the nominal 0.7 suggests.
- Vectors are written only at ingest (`article add`, `entry create`); updated article text keeps a stale embedding, and there is no reindex command.
- `ReciprocalRankFusion` and `NormalizeScore` in `ranker.go` are implemented but unwired; `--context`/`--context-file` flags are registered but unused in the search action.

`--prompt` text is used as the query (falls back to the positional arg).

### Benchmarking

The retrieval benchmark suite lives in `internal/benchmark/` with its spec in
`docs/prds/benchmark/README.md`. Datasets are JSONL (entries, documents,
judged queries, no-match controls); runs seed an isolated temp KB and are
scored with precision@k / recall@k / MRR@k / nDCG@k per unit (article, entry).

- `TestRetrievalBaseline` (internal/benchmark/gate_test.go) enforces the
  committed baseline (`testdata/baseline-v1.json`, bound to the exact dataset
  SHA-256) on every `go test` run — the gate must never be bypassed or
  auto-relaxed.
- Manual experiments:

  ```bash
  bin/kb bench run --dataset internal/benchmark/testdata/lexical-v1.jsonl
  bin/kb bench run --dataset <experiment.jsonl> --baseline <floor.json>
  ```

  Violating floors print the full report first and exit nonzero. Floors are
  measured values; edit them only with an explicit reviewed change after
  re-measuring.

## Writing tests

Existing coverage is CLI-level (`internal/cli/*_test.go`) and unit-level (`internal/*/*_test.go`), table-driven where practical. Run the whole suite with the CGO flags from [Build & test](#build--test).

## PRDs

Feature specs live under `docs/prds/` (e.g. `export/`) as the source of truth for behavior. Update the matching PRD when a feature's behavior changes.