package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
	"github.com/rwese/kb/internal/search"
)

const (
	// CandidatePolicyUnbounded fetches all matching articles for every query
	// before ranking and truncation, so entry-unit deduplication never
	// starves other matching entries. Suitable for fixture-sized datasets.
	CandidatePolicyUnbounded = "unbounded"

	// ReportVersion is the report schema version.
	ReportVersion = 1

	// DefaultTopK is used when Config.TopK is unset.
	DefaultTopK = 3

	// maxCandidates is the candidate fetch limit for the unbounded policy.
	maxCandidates = 10000
)

// Config controls one benchmark run.
type Config struct {
	// TopK is the ranking cutoff used for retrieval and evaluation.
	// 0 selects DefaultTopK.
	TopK int
	// CandidatePolicy must be CandidatePolicyUnbounded (or empty for it).
	CandidatePolicy string
	// Ranking selects the ranking under test. The zero value selects the
	// default BM25 ranking.
	Ranking RankingConfig
	// Embedder supplies document and query embeddings; required exactly
	// when Ranking.Algorithm is hybrid.
	Embedder embed.Embedder
	// EmbeddingModel is recorded with the stored vectors as metadata.
	EmbeddingModel string
}

// RankingConfig describes the deterministic configuration of the ranking
// being benchmarked; recorded in reports and compared by diff. Name is
// derived from the other fields when empty.
type RankingConfig struct {
	Name           string  `json:"name"`
	Algorithm      string  `json:"algorithm"`
	BM25Weight     float64 `json:"bm25_weight"`
	SemanticWeight float64 `json:"semantic_weight"`
}

// ConfigReport records the effective run configuration.
type ConfigReport struct {
	TopK            int           `json:"top_k"`
	CandidatePolicy string        `json:"candidate_policy"`
	Ranking         RankingConfig `json:"ranking"`
}

// DatasetInfo identifies the benchmarked dataset.
type DatasetInfo struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

// RankedHit is one ranked result with its diagnostic score and judged grade.
// Scores are query-relative diagnostics, never thresholds.
type RankedHit struct {
	ID      string  `json:"id"`
	EntryID string  `json:"entry_id,omitempty"`
	Score   float64 `json:"score"`
	Grade   int     `json:"grade"`
}

// QueryReport is the outcome of one query: the deterministic ranked list,
// its latency, and its metrics (nil for controls). Candidates is the number
// of raw matches returned by the DB before unit mapping, deduplication, and
// truncation.
type QueryReport struct {
	ID         string      `json:"id"`
	Unit       string      `json:"unit"`
	Control    bool        `json:"control"`
	LatencyMS  float64     `json:"latency_ms"`
	Candidates int         `json:"candidates"`
	Ranked     []RankedHit `json:"ranked"`
	Metrics    *Metrics    `json:"metrics,omitempty"`
}

// UnitAggregate is the macro average over a unit's non-control queries.
type UnitAggregate struct {
	Queries   int     `json:"queries"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	MRR       float64 `json:"mrr"`
	NDCG      float64 `json:"ndcg"`
	// Hit1Rate is the fraction of queries whose rank-1 hit is positively
	// judged.
	Hit1Rate float64 `json:"hit1_rate"`
}

// LatencyReport summarizes per-query retrieval latency. Latency is
// informational only: no timing gate is enforced.
type LatencyReport struct {
	MeanMS  float64 `json:"mean_ms"`
	P95MS   float64 `json:"p95_ms"`
	MaxMS   float64 `json:"max_ms"`
	TotalMS float64 `json:"total_ms"`
	QPS     float64 `json:"qps"`
}

// CoverageReport counts what retrieval found across the whole dataset.
type CoverageReport struct {
	// Entries and Documents are the dataset totals.
	Entries   int `json:"entries"`
	Documents int `json:"documents"`
	// DocumentsRecovered is the number of distinct documents retrieved at
	// least once by any query (all units, counted after each query's
	// truncation to top_k).
	DocumentsRecovered int `json:"documents_recovered"`
	// NoHitQueries counts non-control queries that retrieved no hits.
	NoHitQueries int `json:"no_hit_queries"`
	// ZeroRecallQueries counts non-control queries whose recall is 0.
	ZeroRecallQueries int `json:"zero_recall_queries"`
	// Controls is the number of no-match control queries.
	Controls int `json:"controls"`
}

// MemReport summarizes process memory cost, deltas between phases. Heap
// sampling uses runtime.ReadMemStats after each phase; values are
// informational and vary across machines and runs.
type MemReport struct {
	// SeedAllocBytes is TotalAlloc during dataset import.
	SeedAllocBytes int64 `json:"seed_alloc_bytes"`
	// RetrievalAllocBytes is TotalAlloc across all queries.
	RetrievalAllocBytes int64 `json:"retrieval_alloc_bytes"`
	// HeapAllocPeakBytes is the largest HeapAlloc observed between queries
	// during retrieval.
	HeapAllocPeakBytes int64 `json:"heap_alloc_peak_bytes"`
	// GCs is the number of GC cycles during seeding + retrieval.
	GCs int64 `json:"gcs"`
}

// StorageReport is the disk footprint of the benchmark KB. DBBytes covers
// the main database file plus WAL/SHM sidecars at report time.
type StorageReport struct {
	DBBytes int64 `json:"db_bytes"`
}

// BaselineResult records the outcome of the baseline check attached to a
// report by the CLI or gate test.
type BaselineResult struct {
	Pass       bool     `json:"pass"`
	Violations []string `json:"violations,omitempty"`
}

// Report is the full outcome of one benchmark run.
type Report struct {
	Version   int                      `json:"version"`
	Dataset   DatasetInfo              `json:"dataset"`
	Config    ConfigReport             `json:"config"`
	Latency   LatencyReport            `json:"latency"`
	Coverage  CoverageReport           `json:"coverage"`
	Mem       MemReport                `json:"mem"`
	Aggregate map[string]UnitAggregate `json:"aggregate"`
	Queries   []QueryReport            `json:"queries"`
	Storage   StorageReport            `json:"storage"`
	Baseline  *BaselineResult          `json:"baseline,omitempty"`
}

// Run imports the dataset into an isolated temporary KB and evaluates every
// query against the live ranking. It never touches config discovery or a
// real knowledgebase. Reports are deterministic apart from latency.
func Run(dataset Dataset, cfg Config) (Report, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return Report{}, err
	}

	dir, err := os.MkdirTemp("", "kb-bench-")
	if err != nil {
		return Report{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	return runInDir(dataset, cfg, dir)
}

// runInDir runs against a caller-provided directory; tests pass t.TempDir().
func runInDir(dataset Dataset, cfg Config, dir string) (Report, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return Report{}, err
	}

	database, err := db.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		return Report{}, fmt.Errorf("open bench db: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Init(); err != nil {
		return Report{}, fmt.Errorf("init bench db: %w", err)
	}

	memBefore := memSnapshot()
	if err := seed(database, dataset); err != nil {
		return Report{}, fmt.Errorf("seed bench db: %w", err)
	}
	if cfg.Ranking.Algorithm == search.AlgorithmHybrid {
		if err := embedArticles(context.Background(), database, dataset, cfg); err != nil {
			return Report{}, err
		}
	}
	memAfterSeed := memSnapshot()

	report := Report{
		Version: ReportVersion,
		Dataset: DatasetInfo{ID: dataset.ID, SHA256: dataset.SHA256},
		Config: ConfigReport{
			TopK:            cfg.TopK,
			CandidatePolicy: cfg.CandidatePolicy,
			Ranking:         cfg.Ranking,
		},
		Aggregate: map[string]UnitAggregate{},
		Coverage: CoverageReport{
			Entries:   len(dataset.Entries),
			Documents: len(dataset.Documents),
		},
	}
	heapPeak := int64(0)
	gcAtStart := memAfterSeed.NumGC

	var memOut MemReport
	memOut.SeedAllocBytes = int64(memAfterSeed.TotalAlloc) - int64(memBefore.TotalAlloc)

	recovered := map[string]bool{}
	byUnit := map[string][]float64{} // per-unit metric sums: unit -> [p, r, mrr, ndcg, hit1]

	for _, q := range dataset.Queries {
		qr, err := runQuery(context.Background(), database, q, cfg)
		if err != nil {
			return Report{}, fmt.Errorf("query %s: %w", q.ID, err)
		}
		report.Queries = append(report.Queries, qr)

		ms := memSnapshot()
		if h := int64(ms.HeapAlloc); h > heapPeak {
			heapPeak = h
		}
		memOut.RetrievalAllocBytes += int64(ms.TotalAlloc) - int64(memAfterSeed.TotalAlloc)
		memAfterSeed = ms

		for _, r := range qr.Ranked {
			if id := hitDocumentID(q, r); id != "" {
				recovered[id] = true
			}
		}

		if qr.Control {
			// Controls must retrieve nothing at all; unjudged hits would
			// silently grade 0 and hide dataset contamination.
			if len(qr.Ranked) != 0 {
				return Report{}, fmt.Errorf("query %s: no-match control retrieved %d hits", q.ID, len(qr.Ranked))
			}
			report.Coverage.Controls++
			continue
		}

		if len(qr.Ranked) == 0 {
			report.Coverage.NoHitQueries++
		}
		if qr.Metrics != nil && qr.Metrics.Recall == 0 {
			report.Coverage.ZeroRecallQueries++
		}

		m := qr.Metrics
		sums := byUnit[q.Unit]
		hit1 := 0.0
		if len(qr.Ranked) > 0 && qr.Ranked[0].Grade > 0 {
			hit1 = 1
		}
		sums = append(sums, m.Precision, m.Recall, m.MRR, m.NDCG, hit1)
		byUnit[q.Unit] = sums
	}

	for unit, sums := range byUnit {
		n := len(sums) / 5
		report.Aggregate[unit] = UnitAggregate{
			Queries:   n,
			Precision: avg(sums, 5, 0, n),
			Recall:    avg(sums, 5, 1, n),
			MRR:       avg(sums, 5, 2, n),
			NDCG:      avg(sums, 5, 3, n),
			Hit1Rate:  avg(sums, 5, 4, n),
		}
	}

	memOut.HeapAllocPeakBytes = heapPeak
	memOut.GCs = int64(memAfterSeed.NumGC) - int64(gcAtStart)
	report.Mem = memOut
	report.Storage = storageReport(dir)
	report.Coverage.DocumentsRecovered = len(recovered)
	report.Latency = latencyReport(report.Queries)
	return report, nil
}

// hitDocumentID maps one ranked hit back to a document id where possible:
// article-unit hits ARE document ids; entry-unit hits are entries.
func hitDocumentID(q Query, r RankedHit) string {
	if q.Unit == UnitArticle {
		return r.ID
	}
	return ""
}

// runQuery retrieves, ranks deterministically, deduplicates for the query
// unit, truncates to TopK, and evaluates.
func runQuery(ctx context.Context, database *db.DB, q Query, cfg Config) (QueryReport, error) {
	start := time.Now()
	results, err := database.SearchWithDeleted(q.Text, maxCandidates, false)
	if err != nil {
		return QueryReport{}, fmt.Errorf("search failed: %w", err)
	}
	latency := time.Since(start)

	// Hybrid re-ranking: embed the query and rescore with the configured
	// weight balance. Runs are expected to be reproducible, so an embedding
	// failure or an empty embedding is a hard error, never a silent BM25
	// fallback. Controls skip re-ranking: they must retrieve nothing, and
	// re-ranking cannot remove hits anyway.
	if cfg.Ranking.Algorithm == search.AlgorithmHybrid && !q.IsControl() {
		queryEmb, err := cfg.Embedder.Embed(ctx, q.Text)
		if err != nil {
			return QueryReport{}, fmt.Errorf("embed query: %w", err)
		}
		if len(queryEmb) == 0 {
			return QueryReport{}, fmt.Errorf("embed query: empty embedding")
		}
		ranker := search.NewRanker(cfg.Ranking.BM25Weight, cfg.Ranking.SemanticWeight)
		results = ranker.HybridSearch(ctx, results, database, queryEmb)
	}

	// Deterministic order: score descending, then ID ascending. SQL and the
	// db merge have no explicit tie-breaker, so the benchmark fixes ties
	// itself before truncation.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})

	// Map to evaluation units, deduplicating and keeping the first hit.
	ranked := make([]RankedHit, 0, cfg.TopK)
	seen := map[string]bool{}
	for _, r := range results {
		id := r.ID
		if q.Unit == UnitEntry {
			id = r.EntryID
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		grade := 0
		for _, j := range q.Relevant {
			if j.ID == id {
				grade = j.Grade
				break
			}
		}
		ranked = append(ranked, RankedHit{ID: id, EntryID: r.EntryID, Score: r.Score, Grade: grade})
		if len(ranked) == cfg.TopK {
			break
		}
	}

	qr := QueryReport{
		ID:         q.ID,
		Unit:       q.Unit,
		Control:    q.IsControl(),
		LatencyMS:  float64(latency.Microseconds()) / 1000.0,
		Candidates: len(results),
		Ranked:     ranked,
	}

	if !q.IsControl() {
		ids := make([]string, len(ranked))
		for i, h := range ranked {
			ids[i] = h.ID
		}
		m, err := Evaluate(ids, q.Relevant, cfg.TopK)
		if err != nil {
			return QueryReport{}, err
		}
		qr.Metrics = &m
	}
	return qr, nil
}

// seed imports entries first, then documents; dataset order need not be
// dependency-sorted because validation already resolved references.
func seed(database *db.DB, dataset Dataset) error {
	for _, e := range dataset.Entries {
		if err := database.AddEntry(e.ID, e.Title, e.Tags); err != nil {
			return fmt.Errorf("entry %s: %w", e.ID, err)
		}
	}
	for _, d := range dataset.Documents {
		if err := database.AddArticle(d.ID, d.EntryID, d.Title, d.Content); err != nil {
			return fmt.Errorf("article %s: %w", d.ID, err)
		}
	}
	return nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.TopK == 0 {
		cfg.TopK = DefaultTopK
	}
	if cfg.TopK < 0 {
		return Config{}, fmt.Errorf("top_k must be positive, got %d", cfg.TopK)
	}
	if cfg.CandidatePolicy == "" {
		cfg.CandidatePolicy = CandidatePolicyUnbounded
	}
	if cfg.CandidatePolicy != CandidatePolicyUnbounded {
		return Config{}, fmt.Errorf("unknown candidate policy %q", cfg.CandidatePolicy)
	}

	// Default weights for hybrid runs that leave both unset; callers with a
	// configured balance (e.g. via config discovery) set both explicitly.
	if cfg.Ranking.Algorithm == "" {
		cfg.Ranking.Algorithm = search.AlgorithmBM25
	}
	switch cfg.Ranking.Algorithm {
	case search.AlgorithmBM25:
		// Weights are inert for pure BM25 retrieval.
	case search.AlgorithmHybrid:
		if cfg.Embedder == nil {
			return Config{}, fmt.Errorf("ranking algorithm %q requires an embedder", cfg.Ranking.Algorithm)
		}
		if cfg.Ranking.BM25Weight < 0 || cfg.Ranking.SemanticWeight < 0 {
			return Config{}, fmt.Errorf("ranking weights must be non-negative")
		}
		if cfg.Ranking.BM25Weight == 0 && cfg.Ranking.SemanticWeight == 0 {
			def := search.DefaultRanker()
			cfg.Ranking.BM25Weight = def.BM25Weight
			cfg.Ranking.SemanticWeight = def.SemanticWeight
		}
		if cfg.Ranking.BM25Weight+cfg.Ranking.SemanticWeight == 0 {
			return Config{}, fmt.Errorf("ranking weights must sum to a positive value")
		}
	default:
		return Config{}, fmt.Errorf("unknown ranking algorithm %q", cfg.Ranking.Algorithm)
	}
	if cfg.Ranking.Name == "" {
		cfg.Ranking.Name = rankingName(cfg.Ranking)
	}
	return cfg, nil
}

// rankingName derives the stable ranking name recorded in reports and
// compared by diff: the BM25 engine keeps its historic name; hybrid names
// encode the weight balance.
func rankingName(r RankingConfig) string {
	if r.Algorithm == search.AlgorithmHybrid {
		return fmt.Sprintf("hybrid-bm25-%.2g-semantic-%.2g", r.BM25Weight, r.SemanticWeight)
	}
	return "bm25-porter-v1"
}

// embedArticles embeds every dataset document and stores the vector under
// the article id, mirroring the production ingestion path (content-only
// embeddings). Missing vectors make HybridSearch fall back to BM25-only for
// that hit.
func embedArticles(ctx context.Context, database *db.DB, dataset Dataset, cfg Config) error {
	texts := make([]string, len(dataset.Documents))
	for i, d := range dataset.Documents {
		texts[i] = d.Content
	}
	vectors, err := cfg.Embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed documents: %w", err)
	}
	for i, d := range dataset.Documents {
		if len(vectors[i]) == 0 {
			continue
		}
		if err := database.SaveVector(d.ID, vectors[i], cfg.EmbeddingModel); err != nil {
			return fmt.Errorf("save vector %s: %w", d.ID, err)
		}
	}
	return nil
}

// avg averages every stride-th element of sums starting at offset.
func avg(sums []float64, stride, offset, n int) float64 {
	total := 0.0
	for i := offset; i < len(sums); i += stride {
		total += sums[i]
	}
	if n == 0 {
		return 0
	}
	return total / float64(n)
}

// latencyReport computes informational latency stats over all queries,
// p95 by nearest-rank.
func latencyReport(queries []QueryReport) LatencyReport {
	if len(queries) == 0 {
		return LatencyReport{}
	}
	latencies := make([]float64, len(queries))
	var total float64
	for i, q := range queries {
		latencies[i] = q.LatencyMS
		total += q.LatencyMS
	}
	sort.Float64s(latencies)
	idx := int(0.95*float64(len(latencies))+0.5) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	qps := 0.0
	if total > 0 {
		qps = float64(len(latencies)) * 1000.0 / total
	}
	return LatencyReport{
		MeanMS:  total / float64(len(latencies)),
		P95MS:   latencies[idx],
		MaxMS:   latencies[len(latencies)-1],
		TotalMS: total,
		QPS:     qps,
	}
}

// memSnapshot captures the runtime memory counters we report on.
func memSnapshot() runtime.MemStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms
}

// storageReport sums the benchmark KB's file footprint (main db plus WAL and
// shared-memory sidecars); missing files count as zero bytes.
func storageReport(dir string) StorageReport {
	var total int64
	for _, name := range []string{"kb.db", "kb.db-wal", "kb.db-shm"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil {
			total += info.Size()
		}
	}
	return StorageReport{DBBytes: total}
}
