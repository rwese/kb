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

## Articles (multi-entry knowledgebase items)
- [x] Add `articles` table (belongs to entry, has content/chunks)
- [x] `kb append` - Add article to existing entry
- [x] `kb list` - List entries and their articles
- [x] `kb get` - Get entry with specific/all articles
- [x] `kb delete` - Delete entry or article
- [x] Update FTS to index articles separately

## Testing
- [x] Verify `go build` succeeds
- [x] Test `kb init`
- [x] Test `kb add` with inline content
- [x] Test `kb append` to existing entry
- [x] Test `kb list`
- [x] Test `kb get`
- [x] Test `kb search`

## Polish
- [x] Add `--json` output for search
- [ ] man page
