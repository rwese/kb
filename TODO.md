# kb-local-embeddings Implementation

## Phase 1: Core Infrastructure
- [x] Create `internal/embed/local.go` with llama.cpp interface
- [x] Create `internal/embed/download.go` with GitHub releases integration
- [x] Update `internal/config/discover.go` with local config
- [x] Add SQLite vectors table migration
- [x] Create `scripts/build-llama.sh` for cross-platform compilation

## Phase 2: Integration
- [ ] Update `kb add` to compute + store embeddings
- [ ] Update `kb append` to compute + store embeddings
- [ ] Update `kb delete` to remove vector entries
- [ ] Implement hybrid search in `kb search`

## Phase 3: Polish
- [x] Add `kb download` command
- [x] Enhance `kb check` with embedder status
- [ ] Progress indicators for downloads
- [ ] Graceful degradation on cache dir issues

## Phase 4: Release
- [ ] CI/CD pipeline for multi-platform builds
- [ ] GitHub Releases with asset upload
- [ ] Documentation update
- [ ] Version bump

## Success Criteria
- [ ] `kb add -t "test" -c "content"` stores vector in DB
- [ ] `kb search "content"` returns semantic matches
- [ ] `kb` works on fresh install
- [ ] `kb check` reports green for local embedder
- [ ] No Ollama installation required for embeddings
