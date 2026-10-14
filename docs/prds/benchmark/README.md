# Retrieval Benchmark Specification

Status: Implemented
Task: benchmark suite for retrieval fine-tuning experiments

## Goal

Make search quality measurable and regression-safe, so ranking changes and
fine-tuning experiments can be compared on the same data. Benchmark datasets
are JSONL files that are imported into an isolated KB and executed against
the live ranking; a committed baseline enforces a minimum quality level on
every `go test` run.

## Non-goals (first release)

- Semantic/hybrid benchmarking: the harness scores BM25-porter retrieval only
  (runs with no embedder configured). Hybrid experiments need deterministic
  vectors and a recorded embedding contract.
- Import into a real knowledgebase: benchmark KBs are throwaway.
- Configurable title/body BM25 weights: the current engine is benchmarked as
  it is; the harness records the ranking configuration so weight experiments
  become a schema change later.
- Fixing the known score-normalization inversion (see Limitations below).

## Dataset format (JSONL)

One file, records one JSON value per line (blank lines ignored). The first
record must be the dataset metadata:

```jsonl
{"type":"dataset","version":1,"id":"lexical-v1"}
{"type":"entry","id":"a1b2c3","title":"SQLite storage","tags":"database"}
{"type":"document","id":"a1b2c3-000001","entry_id":"a1b2c3","title":"SQLite busy timeout","content":"SQLite lock contention uses busy_timeout."}
{"type":"query","id":"q-timeout","text":"busy_timeout","unit":"article","relevant":[{"id":"a1b2c3-000001","grade":3}]}
```

Validation rules (parse errors carry line numbers):

- Entry IDs are 6 lowercase hex; document IDs follow the KB article shape
  `<entry-id>-<6 hex>` with the prefix matching `entry_id`.
- Entry/document/query IDs are unique; document `entry_id` references
  resolve; judgment IDs resolve to documents (article unit) or entries
  (entry unit) matching the query's unit; judgments are unique per query.
- Grades are 0..3. Positive relevance is grade > 0.
- A query with no judgments is an explicit **no-match control**: it must
  retrieve zero hits or the run fails. A non-empty judgment list needs at
  least one positive grade.
- Unknown fields, trailing JSON values, or lines above 8 MiB are rejected.
- The dataset SHA-256 of the exact input bytes is computed at parse time and
  recorded in every report and baseline binding.

## Extraction / evaluation

- Seeding: entries first, then articles, into `db.Open` + `Init` of a temp
  KB. Dataset order does not need to be dependency-sorted.
- Retrieval: `db.SearchWithDeleted(text, 10000, false)` — the `unbounded`
  candidate policy fetches all matching articles before ranking, so
  entry-unit deduplication cannot starve other matching entries.
- Deterministic ordering: results are re-sorted by (score desc, id asc)
  before truncation; SQL and the db merge have no tie-breaker.
- Article unit: rank by hit ID. Entry unit: map hits to `EntryID`,
  deduplicate keeping the first hit, then rank.
- Truncation to `top_k` (default 3) happens after deduplication.
- Scores are diagnostic (and, due to the normalization inversion below, may
  read 1.0/0.0 misleadingly); metrics use grades and ranks only.

## Metrics

All at cutoff `k = top_k`, macro-averaged per unit over non-control queries:

- **Precision@k** — positive hits / k (denominator is always k).
- **Recall@k** — positive hits / all positive judgments.
- **MRR@k** — 1/rank of the first positive hit, else 0.
- **nDCG@k** — gain `2^grade-1`, discount `log2(rank+1)`; ideal ordering is
  all judged grades descending, truncated to k.

No-match controls are excluded from averages and must return zero hits.

## Baseline gate

`internal/benchmark/testdata/baseline-v1.json` binds floors to the exact
dataset bytes, cutoff, and candidate policy:

```json
{
  "dataset": "lexical-v1",
  "dataset_sha256": "<sha-256 of lexical-v1.jsonl>",
  "top_k": 3,
  "candidate_policy": "unbounded",
  "minimum": {
    "article": {"precision": 0.4, "recall": 1.0, "mrr": 1.0, "ndcg": 1.0, "hit1": 1.0},
    "entry": {"precision": 0.45, "recall": 1.0, "mrr": 1.0, "ndcg": 1.0, "hit1": 1.0}
  },
  "maximum": {
    "db_bytes": 550000
  },
  "per_query_minimum": {
    "q-timeout": {"recall": 1.0},
    "q-sqlite": {"recall": 1.0},
    "q-lock": {"recall": 1.0, "ndcg": 0.85},
    "q-entry-database": {"recall": 1.0}
  },
  "reference": {
    "latency_mean_ms": 0.254,
    "latency_p95_ms": 0.277,
    "qps": 3938.3,
    "seed_alloc_bytes": 10840,
    "retrieval_alloc_bytes": 103808,
    "heap_alloc_peak_bytes": 684440,
    "gcs": 0
  }
}
```

Enforcement split, chosen so the gate never flakes on machine speed:

- **Enforced floors**: precision, recall, MRR, nDCG per unit (all four
  required), optional `hit1` rate, optional per-query floors, and
  `maximum.db_bytes` — the benchmark KB's on-disk footprint, which is
  deterministic for identical seed bytes and catches index-size regressions.
- **Recorded reference (never enforced)**: latency mean/p95, QPS, and the
  memory deltas under `reference`. Timing and memory vary across machines;
  violating a timing gate would fail unrelated machines, so these are
  documented measured values for human comparison only.

- Report binding mismatches (different dataset bytes, `top_k`, or candidate
  policy) are hard errors, never violations: re-measure and re-bind
  explicitly.
- Floors are compared with epsilon 1e-12; violations name scope, metric,
  actual value, and floor.
- `TestRetrievalBaseline` (internal/benchmark/gate_test.go) runs on every
  `go test ./...` and fails on any violation. It is never skipped under
  `-short` and has no environment bypass.
- Floors are measured engine values minus small per-metric margins. Editing
  a floor is a reviewed, explicit change; nothing regenerates them
  automatically. Changing dataset bytes invalidates the binding by design.

## CLI

Hidden command (does not touch config discovery or a real KB):

```bash
kb bench run --dataset <file.jsonl>                      # JSON report
kb bench run --dataset <file.jsonl> --top-k 5
kb bench run --dataset <file.jsonl> --baseline <base.json>
```

With `--baseline`, the report carries `baseline.pass` and
`baseline.violations`; the full report is printed even when the baseline
fails, and the command exits nonzero.

## Report

JSON with dataset identity (id, sha256), effective configuration
(top_k, candidate_policy, ranking name), informational latency (mean/p95/max,
p95 nearest-rank, queries/sec), coverage counters, process memory deltas,
per-unit aggregates, and per-query ranked lists (id, entry_id, diagnostic
score, judged grade, candidate count) with metrics. Two runs on the same
machine produce identical ranked lists, metrics, coverage, and storage
totals; latency and memory values vary.

- `coverage`: dataset entry/document totals, `documents_recovered` (distinct
  documents retrieved at least once in any article-unit top-k; entry-unit
  hits only reveal entries and are not counted), `no_hit_queries`,
  `zero_recall_queries`, `controls`.
- `mem` (informational, machine-dependent): `seed_alloc_bytes` and
  `retrieval_alloc_bytes` (runtime TotalAlloc deltas via ReadMemStats),
  `heap_alloc_peak_bytes` (largest HeapAlloc observed between queries),
  `gcs` (GC cycles during the run).
- `storage`: `db_bytes` — kb.db plus WAL/SHM sidecars at report time.

## Report diff

`kb bench diff a.json b.json` (or `benchmark.Diff(a, b)`) compares two
reports run on identical dataset bytes and configuration; any binding
mismatch is an error. Output: per-unit aggregate deltas (precision, recall,
MRR, nDCG, hit1_rate) with before/after/delta, per-query metric diffs, and
the list of queries whose top-k ranking reordered.

## Limitations / known engine issues (benchmark targets, not fixed here)

- **Score normalization inversion** (`db.normalizeScoreSet`): raw BM25 is
  lower-is-better, so the min-max normalization maps the best hit to 0.0 and
  sorts trained on it produce inverted diagnostics; single-hit sets map to
  1.0. The reports above show this artifact (best hits score 1.0 via
  single-hit sets, second-best score 0.0). A ranking fix must re-measure the
  baseline explicitly.
- Tiny fixtures detect regressions but overfit easily; use dedicated
  experiment datasets for weight comparisons before adopting changes.
- Seed dataset exemplifies lexical/stemming behavior (porter tokenizer);
  semantic-only paraphrase needs an embedding-enabled benchmark.

## Test hermeticity

All runner paths create their own KB under a temp directory (or
`t.TempDir()` via `runInDir`) and never read `$KB_PATH`, `~/.config/kb`, or
`.kb.yaml`.
