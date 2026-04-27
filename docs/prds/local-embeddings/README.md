# PRD: Local Embeddings for KB

## Project Name: kb-local-embeddings

**Version:** 1.0.0-draft
**Date:** 2026-04-27
**Status:** Planning
**Target:** Standalone knowledgebase CLI with zero external dependencies

---

## 1. Executive Summary

**Problem Statement**

KB currently supports embeddings via Ollama only. Users must:
- Install Ollama separately
- Run Ollama daemon in background
- Download/serve embedding models via HTTP

This creates friction and external dependency overhead.

**Related Projects**

| Project | Purpose | Tech | Status |
|---------|---------|------|--------|
| KB | Knowledgebase CLI | Go 1.22, SQLite, urfave/cli | Active |
| kelindar/search | Go vector search via llama.cpp | Go, purego, llama.cpp | Reference |
| viant/sqlite-vec | Pure Go vector storage | Go, modernc.org/sqlite | Reference |

**Proposed Solution**

Bundle llama.cpp compiled library with KB binary. At first run:
1. Check for local `libllama_go` library
2. If missing, download from GitHub releases (~20 MB)
3. Download bundled GGUF model (~21 MB)
4. Use purego to load library and compute embeddings

---

## 2. Goals & Non-Goals

**Goals**

1. **Zero External Dependencies** — Single binary that works offline
2. **Cross-Platform** — Support macOS (Intel/Apple Silicon), Linux (x64/arm64), Windows
3. **Automatic Setup** — Seamless download of required assets on first use
4. **Hybrid Search** — Combine BM25 + semantic similarity with configurable weights
5. **Performance** — Sub-second embedding computation on modern CPUs

**Non-Goals**

1. ~~GPU acceleration support~~ (future consideration)
2. ~~Multiple embedding models~~ (single model: all-MiniLM-L6-v2)
3. ~~Ollama fallback~~ (standalone only)
4. ~~Streaming/incremental indexing~~ (batch at add/append time)

---

## 3. Technical Architecture

### 3.1 Technology Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| CLI Framework | urfave/cli/v3 | Current stack |
| Database | mattn/go-sqlite3 | Current stack, CGO required |
| Vector Storage | viant/sqlite-vec | Pure Go, no CGO, HNSW indexing |
| Embedding Runtime | llama.cpp via purego | CGO-free, cross-platform |
| Embedding Model | all-MiniLM-L6-v2 GGUF Q4_K_M | 21 MB, good quality/size |
| HTTP Client | net/http | Standard library |

### 3.2 Project Structure

```
kb/
├── cmd/
│   └── kb/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── discover.go          # Config loading
│   ├── db/
│   │   └── sqlite.go           # DB operations
│   ├── embed/
│   │   ├── embed.go            # Embedder interface
│   │   ├── local.go            # NEW: llama.cpp embedder
│   │   ├── download.go         # NEW: Runtime asset download
│   │   └── ollama.go           # Keep for advanced users
│   ├── id/
│   │   └── id.go              # ID generation
│   └── search/
│       └── ranker.go          # BM25 + hybrid search
├── dist/                       # Built release assets (gitignore)
│   ├── darwin-amd64/
│   │   ├── libllama_go.dylib
│   │   └── all-MiniLM-L6-v2-Q4_K_M.gguf
│   ├── darwin-arm64/
│   │   ├── libllama_go.dylib
│   │   └── all-MiniLM-L6-v2-Q4_K_M.gguf
│   ├── linux-amd64/
│   │   ├── libllama_go.so
│   │   └── all-MiniLM-L6-v2-Q4_K_M.gguf
│   ├── linux-arm64/
│   │   ├── libllama_go.so
│   │   └── all-MiniLM-L6-v2-Q4_K_M.gguf
│   └── windows-amd64/
│       ├── llamago.dll
│       └── all-MiniLM-L6-v2-Q4_K_M.gguf
├── scripts/
│   ├── build-llama.sh         # Compile llama.cpp per platform
│   └── download-model.sh       # Download/quantize GGUF
├── docs/
│   └── prds/
│       └── local-embeddings/   # This document
├── go.mod
├── go.sum
└── justfile
```

### 3.3 Configuration

**`~/.config/kb/config.yaml`**

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
embedder: local  # "none", "ollama", or "local"
top_k: 5

# Local embedding settings
local:
  model: all-MiniLM-L6-v2-Q4_K_M  # Model name (future: configurable)
  cache_dir: ~/.cache/kb          # Where lib/model are cached
  bm25_weight: 0.3                # BM25 contribution (0.0-1.0)
  semantic_weight: 0.7            # Vector similarity contribution

# Ollama (kept for power users)
ollama:
  model: nomic-embed-text
  base_url: http://localhost:11434
```

### 3.4 Data Model

**New: `vectors` table**

```sql
CREATE TABLE IF NOT EXISTS vectors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id TEXT NOT NULL,
    article_id TEXT NOT NULL,
    embedding BLOB NOT NULL,          -- float32[], 384 dimensions
    model TEXT NOT NULL DEFAULT 'all-MiniLM-L6-v2-Q4_K_M',
    created_at INTEGER NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id),
    FOREIGN KEY (article_id) REFERENCES articles(id)
);

CREATE INDEX idx_vectors_entry ON vectors(entry_id);
CREATE INDEX idx_vectors_article ON vectors(article_id);
```

**Existing tables unchanged** — entries, articles, fts_articles

### 3.5 Download Flow

```
┌─────────────────────────────────────────────────────────┐
│                    KB Start                             │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
         ┌────────────────────────┐
         │ Check: libllama_go      │
         │ Check: GGUF model       │
         └────────────┬───────────┘
                      │
          ┌───────────┴───────────┐
          │                       │
          ▼                       ▼
    ┌──────────┐           ┌──────────────┐
    │ Found    │           │ Not Found     │
    └────┬─────┘           └───────┬──────┘
         │                         │
         ▼                         ▼
   ┌──────────┐           ┌──────────────────────┐
   │ Load     │           │ Detect OS/Arch       │
   │ Library  │           └──────────┬───────────┘
   └────┬─────┘                      │
        │                            ▼
        │              ┌──────────────────────────────┐
        │              │ Download from GitHub        │
        │              │ Releases:                   │
        │              │ - libllama_go (~20 MB)      │
        │              │ - model.gguf (~21 MB)       │
        │              └──────────┬───────────────────┘
        │                         │
        │                         ▼
        │              ┌──────────────────────────────┐
        │              │ Extract to cache_dir          │
        │              │ ~/.cache/kb/                  │
        │              └──────────┬───────────────────┘
        │                         │
        └─────────┬───────────────┘
                  │
                  ▼
         ┌─────────────────┐
         │ Initialize      │
         │ Model + Library │
         └────────┬────────┘
                  │
                  ▼
         ┌─────────────────┐
         │ Ready to Embed  │
         └─────────────────┘
```

### 3.6 Hybrid Search Algorithm

```go
func HybridScore(bm25Score, cosineSim float64, bm25Weight float64) float64 {
    semanticWeight := 1.0 - bm25Weight
    return bm25Weight*bm25Score + semanticWeight*cosineSim
}

// BM25 score normalized to [0,1] via sigmoid
func NormalizedBM25(score float64) float64 {
    return 1.0 / (1.0 + math.Exp(-score/10.0))
}

// Cosine similarity already in [0,1]
```

---

## 4. Interface Specification

### 4.1 CLI Commands

```bash
kb [OPTIONS] <COMMAND> [ARGS]

Commands:
  init       Initialize database
  add        Create entry with article
  append     Add article to entry
  list       List entries
  get        Get entry or article
  search     Search articles
  delete     Delete entry or article
  stats      Show database statistics
  config     Show config
  check      Validate installation
  download   Manually download/refresh local assets  # NEW

Options:
  --all       Include deleted entries
  --articles  Include articles when getting entry
  --json      JSON output
  --verbose   Verbose output (for download progress)
```

### 4.2 New: `kb download` Command

```bash
# Download/update local embedding assets
kb download

# Output:
Downloading libllama_go.dylib (20.4 MB)...
  [████████████████████] 100%
Downloading all-MiniLM-L6-v2-Q4_K_M.gguf (21.0 MB)...
  [████████████████████] 100%
Local embeddings ready.
```

### 4.3 `kb check` Enhancement

```bash
$ kb check

Database:     ~/.local/share/kb/knowledgebase.db ✓
Embedder:     local (all-MiniLM-L6-v2-Q4_K_M) ✓
Library:      ~/.cache/kb/libllama_go.dylib ✓
Model:        ~/.cache/kb/all-MiniLM-L6-v2-Q4_K_M.gguf ✓
Vectors:      1,247 indexed ✓
```

### 4.4 Error Messages

| Error | Message | Action |
|-------|---------|--------|
| E1 | `Local embedder not available: download failed` | Run `kb download` |
| E2 | `Model not found in cache` | Run `kb download` |
| E3 | `Unsupported platform: darwin/arm64` | Install from source |
| E4 | `Failed to load library: incompatible version` | Re-download assets |

---

## 5. Feature Roadmap

### Phase 1: Core Infrastructure

- [ ] Create `internal/embed/local.go` with llama.cpp interface
- [ ] Create `internal/embed/download.go` with GitHub releases integration
- [ ] Update `internal/config/discover.go` with local config
- [ ] Create `scripts/build-llama.sh` for cross-platform compilation
- [ ] Add SQLite vector table migration

### Phase 2: Integration

- [ ] Update `kb add` to compute + store embeddings
- [ ] Update `kb append` to compute + store embeddings
- [ ] Update `kb delete` to remove vector entries
- [ ] Implement hybrid search in `kb search`

### Phase 3: Polish

- [ ] Add `kb download` command
- [ ] Enhance `kb check` with embedder status
- [ ] Progress indicators for downloads
- [ ] Graceful degradation on cache dir issues

### Phase 4: Release

- [ ] CI/CD pipeline for multi-platform builds
- [ ] GitHub Releases with asset upload
- [ ] Documentation update
- [ ] Version bump

**MVP Milestone:** Users can `kb add`, `kb search`, and get semantic results without Ollama.

---

## 6. Integration Notes

### 6.1 Comparison: Current vs. New

| Aspect | Current (Ollama) | New (Local) |
|--------|------------------|-------------|
| Setup | Install Ollama + model | Zero config |
| Runtime | Ollama daemon running | Self-contained |
| Dependencies | Ollama, network | None |
| Latency | ~100ms (HTTP) | ~50ms (in-process) |
| Offline | Requires cache | Works offline |
| Memory | Ollama overhead | ~100MB additional |

### 6.2 Migration Path

Existing users:
1. No automatic migration needed
2. Vectors computed lazily on search
3. Old Ollama config preserved for power users

---

## 7. Open Questions

1. **Release hosting**: Where to host `libllama_go` binaries?
   - Option A: KB's own GitHub Releases
   - Option B: Separate `kb-assets` repo

2. **Model updates**: How to handle model version bumps?
   - Option A: Freeze model, never update
   - Option B: Version in config, re-embed on change

3. **Cache invalidation**: When to refresh downloaded assets?
   - Option A: Never auto-update
   - Option B: Check version on startup, prompt if new

4. **Build reproducibility**: Pin llama.cpp commit hash?
   - Ensures consistent builds
   - Prevents silent regressions

---

## 8. Success Criteria

- [ ] `kb add -t "test" -c "content"` stores vector in DB
- [ ] `kb search "content"` returns semantic matches
- [ ] `kb` works on fresh macOS/Linux/Windows install
- [ ] `kb check` reports green for local embedder
- [ ] No Ollama installation required for embeddings
- [ ] First-run download completes < 60s on broadband
- [ ] Embedding computation < 100ms per article
- [ ] Search returns relevant results within 500ms

---

## 9. Repository Location

```
/Users/wese/Repos/github.com/rwese/kb/docs/prds/local-embeddings/
```

---

## Appendix A: Build Scripts

### `scripts/build-llama.sh`

```bash
#!/bin/bash
set -e

# Build llama.cpp library for current platform using kelindar/search approach
# Output: dist/<os>-<arch>/libllama_go.<ext>

OS=$(go env GOOS)
ARCH=$(go env GOARCH)

case "$OS" in
    darwin) EXT="dylib";;
    linux)  EXT="so";;
    windows) EXT="dll";;
esac

echo "Building llama.cpp for $OS/$ARCH..."

# Clone and build llama.cpp with CMake
git clone --depth 1 https://github.com/ggerganov/llama.cpp.git /tmp/llama.cpp
cd /tmp/llama.cpp
mkdir build && cd build
cmake -DLLAMA_BERT=ON -DBUILD_SHARED_LIBS=ON ..
cmake --build . --config Release

# Copy output
mkdir -p "../../dist/${OS}-${ARCH}"
cp libllama.* "../../dist/${OS}-${ARCH}/"

echo "Built: dist/${OS}-${ARCH}/libllama_go.${EXT}"
```

### `scripts/download-model.sh`

```bash
#!/bin/bash
set -e

MODEL="all-MiniLM-L6-v2-Q4_K_M"
REPO="second-state/All-MiniLM-L6-v2-Embedding-GGUF"
OUT_DIR="${1:-$HOME/.cache/kb}"

mkdir -p "$OUT_DIR"

echo "Downloading $MODEL..."
curl -L \
  "https://huggingface.co/$REPO/resolve/main/${MODEL}.gguf" \
  -o "$OUT_DIR/${MODEL}.gguf"

echo "Model saved to: $OUT_DIR/${MODEL}.gguf"
```

---

## Appendix B: API Reference

### `embed.Embedder` Interface

```go
package embed

type Embedder interface {
    // Embed computes vector for single text
    Embed(ctx context.Context, text string) ([]float32, error)

    // EmbedBatch computes vectors for multiple texts
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

    // Dimension returns embedding vector size
    Dimension() int

    // Close releases resources
    Close() error
}
```

### `embed.LocalEmbedder` Implementation

```go
package embed

type LocalEmbedder struct {
    model   *llama.Model
    context *llama.Context
    library string
    modelPath string
}

func NewLocalEmbedder(libPath, modelPath string) (*LocalEmbedder, error)
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error)
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
func (e *LocalEmbedder) Dimension() int { return 384 }
func (e *LocalEmbedder) Close() error
```

---

## Appendix C: Example Session

```bash
# Fresh install
$ kb init
Database initialized at ~/.local/share/kb/knowledgebase.db

# Add entry (triggers embedding download on first run)
$ kb add -t "Postgres indexing" -c "Use B-tree indexes for equality, GIN for arrays"
Downloading libllama_go.dylib...
  [████████████████████] 100% (20.4 MB)
Downloading all-MiniLM-L6-v2-Q4_K_M.gguf...
  [████████████████████] 100% (21.0 MB)
Created entry 3f8a2c

# Search with semantic understanding
$ kb search "database performance"
Entry 3f8a2c: Postgres indexing
  Article: "Use B-tree indexes for equality, GIN for arrays"
  Score: 0.847 (BM25: 0.12 + Semantic: 0.73)

# Check status
$ kb check
Database:     ~/.local/share/kb/knowledgebase.db ✓
Embedder:     local (all-MiniLM-L6-v2-Q4_K_M) ✓
Library:      ~/.cache/kb/libllama_go.dylib ✓
Model:        ~/.cache/kb/all-MiniLM-L6-v2-Q4_K_M.gguf ✓
Vectors:      1 indexed ✓
```
