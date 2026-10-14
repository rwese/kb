# Embedding solutions

Status: ollama implemented; onnx opt-in designed, not implemented; portable
stand-ins are tooling-only.

kb gets semantic ranking from one pluggable embedding provider, resolved as
flag > env (`KB_ALGORITHM`) > config `ranker.algorithm` (`auto` default)
over `config.embedder`. This doc records what each provider needs, how to
build/enable it, and what is intentionally not solved yet.

## Providers

| provider | status | needs |
|---|---|---|
| `none` (default) | implemented | nothing - bm25 only |
| `ollama` | implemented | running Ollama server + model (see below) |
| `onnx` | **not implemented** | `internal/embed/onnx` (build tag `onnx`) + vendored runtime docs Tbd |

### none

No config, no semantic tier, `auto` always resolves to `bm25`. All ranking
switches except the hybrid weights still apply.

### ollama

Wiring (`~/.config/kb/config.yaml` or scoped via `$KB_PATH`):

```yaml
embedder: ollama
ollama:
  model: nomic-embed-text      # kb's default when unset
  base_url: http://localhost:11434
```

With `embedder: ollama`, `auto` resolves to `hybrid`; explicit
`--algorithm hybrid` errors if Ollama is unreachable (no silent bm25
fallback outside `auto`).

Requirements (verified 2026-10, embeddings only - no LLM involved):

- Ollama server listening on the configured host/port; the client posts to
  `${base_url}/api/embeddings`, no extra flags.
- `ollama pull nomic-embed-text`: 274 MB download, ~1 GB resident RAM,
  768-dim vectors. Recommended Ollama >= 0.1.26 (current line: 0.34.x).
- Hosts: macOS 11+ (Metal automatic on Apple Silicon), Linux, Windows; 8 GB
  RAM minimum, 10 GB free disk. Sixteen GB-class hardware is far above what
  embeddings need.

Install (macOS):

```bash
brew install --cask ollama   # or the .dmg from ollama.com/download
ollama pull nomic-embed-text
```

Caveats (documented in DEVELOPMENT.md "Ranking"):

- Vectors are written only at ingest (`article add`, `entry create`);
  pre-existing articles stay vector-less and cap at
  `BM25Weight * BM25Score` inside hybrid scoring. There is no reindex
  command yet.
- Dimension mismatches between stored vectors and the configured model are
  benign (cosine 0), but swapping models silently semantically blinds old
  articles until they are re-added.

### onnx (opt-in, not implemented yet)

Design contract agreed and partially scaffolded:

- Build-time opt-in: the provider lives in `internal/embed/onnx` behind
  `go build -tags onnx`; default builds are identical to today's.
- Runtime opt-in: active only when the config names it (explicit
  `embedder: onnx`; eligible for `auto` once configured).
- Graceful states: not-compiled -> behaves as today; compiled but
  unconfigured -> ignored; configured but broken (missing dylib, missing
  model/tokenizer, missing sha-pinned files) -> hard error naming the
  missing piece and the install command, never silent degradation.
- `scripts/build.sh --variant onnx --os X --arch Y` builds the variant and
  bundles the onnxruntime shared library with the binary; **the guard
  currently refuses** (`internal/embed/onnx` absent) so release matrices
  cannot ship a half-built variant.
- Planned shape: hugot/onnxruntime-go over quantized all-MiniLM-L6-v2 ONNX
  (~90 MB, 384-dim, in-process, no server); model + runtime pinned by
  SHA-256 with reproducible install directions in this file once the code
  lands. Vendoring decisions (dylib in-repo vs. pinned download) follow the
  repo dependency rule: pinned to exact versions and hashes.

Release integration already anticipates it: `build.sh` variants and the
Forgejo/GitHub release matrices can add `variant: onnx` rows the day the
sources land.

## Portable stand-ins (benchmark tooling only)

For exercising the hybrid pipeline without any model server:

- `scripts/embed-stub.py` mimics `/api/embeddings` (the exact endpoint kb's
  ollama client uses). Default mode serves hand-built topic-dictionary
  vectors for the paraphrase fixture; `--corpus <kb-dataset.jsonl>` fits
  sublinear TF-IDF over the corpus vocabulary and serves L2-normalized
  TF-IDF vectors - a real, describable VSM model for algorithm
  comparisons.
- `KB_PATH=$(echo /tmp/stub-config.yaml) scripts/bench-matrix.sh ...` pairs
  the stub with the ranking matrix (config: `embedder: ollama`,
  `base_url: http://localhost:11434` under a running stub).
- Numbers from both modes are synthetic stand-ins, not neural-embedding
  quality: topic overlap or bag-of-words VSM, not meaning.

Longer-term portable option, designed but unbuilt: precomputed vector
sidecars (`float32` vectors committed next to a fixture, loaded by the bench
as a static embedder) would make neural-embedding benchmarks reproducible
with zero installs; the benchmark report schema already has fields to record
such a vector set.
