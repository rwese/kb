# kb CLI - Implementation Tasks

## Core
- [x] Project structure & go.mod
- [x] Config discovery
- [x] SQLite+FTS5 database
- [x] `kb init` command
- [x] `kb add` command
- [x] `kb search` command
- [x] `kb config` command
- [x] Ollama embedder implementation
- [ ] Weighted retrieval scoring (re-ranking with embeddings)

## Testing
- [x] Verify `go build` succeeds
- [x] Test `kb init`
- [x] Test `kb add` with inline content
- [x] Test `kb search`

## Polish
- [x] Add `--json` output for search
- [ ] Add `kb list` command
- [ ] Add `kb delete` command

## Documentation
- [x] README.md
- [ ] man page
