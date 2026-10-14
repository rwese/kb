package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// stubEmbedder answers with fixed vectors keyed by exact input text; any
// unknown text is an error so tests fail loudly instead of silently falling
// back to BM25-only.
type stubEmbedder map[string][]float32

func (s stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := s[text]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no vector for %q", text)
}

func (s stubEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v, err := s.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func hybridDataset(t *testing.T) Dataset {
	t.Helper()
	return mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t-hybrid"}`,
		`{"type":"entry","id":"abc123","title":"Redis"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Persistence","content":"redis persistence"}`,
		`{"type":"document","id":"abc123-000002","entry_id":"abc123","title":"Caching","content":"redis caching"}`,
		`{"type":"query","id":"q-redis","text":"redis","unit":"article","relevant":[{"id":"abc123-000001","grade":3},{"id":"abc123-000002","grade":2}]}`,
	}, "\n"))
}

// The query's embedding matches the caching document only. With a dominant
// semantic weight the semantic signal must outrank whatever BM25 decided:
// the persistence document scores at most 0.1, the caching one at least 0.9.
func TestRunnerHybridReranksByWeights(t *testing.T) {
	emb := stubEmbedder{
		"redis persistence": {1, 0},
		"redis caching":     {0, 1},
		"redis":             {0, 1},
	}

	rep, err := runInDir(hybridDataset(t), Config{
		TopK:           2,
		Embedder:       emb,
		EmbeddingModel: "stub-v1",
		Ranking: RankingConfig{
			Algorithm:      "hybrid",
			Name:           "test-hybrid",
			BM25Weight:     0.1,
			SemanticWeight: 0.9,
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	qr := rep.Queries[0]
	if len(qr.Ranked) != 2 {
		t.Fatalf("expected both hits, got %+v", qr.Ranked)
	}
	if qr.Ranked[0].ID != "abc123-000002" {
		t.Fatalf("semantic match must rank first: %+v", qr.Ranked)
	}
	if qr.Metrics == nil || qr.Metrics.Recall != 1 {
		t.Fatalf("unexpected metrics: %+v", qr.Metrics)
	}
}

func TestRunnerHybridRecordsRankingConfig(t *testing.T) {
	emb := stubEmbedder{
		"redis persistence": {1, 0},
		"redis caching":     {0, 1},
		"redis":             {1, 1},
	}
	rep, err := runInDir(hybridDataset(t), Config{
		TopK:     2,
		Embedder: emb,
		Ranking: RankingConfig{
			Algorithm:      "hybrid",
			BM25Weight:     0.3,
			SemanticWeight: 0.7,
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	got := rep.Config.Ranking
	if got.Name != "hybrid-bm25-0.3-semantic-0.7" {
		t.Fatalf("ranking name = %q", got.Name)
	}
	if got.Algorithm != "hybrid" || got.BM25Weight != 0.3 || got.SemanticWeight != 0.7 {
		t.Fatalf("ranking config = %+v", got)
	}
}

func TestRunnerDefaultsToBM25Ranking(t *testing.T) {
	rep := runFixture(t, Config{TopK: 3})
	r := rep.Config.Ranking
	if r.Name != "bm25-porter-v1" || r.Algorithm != "bm25" || r.BM25Weight != 0 || r.SemanticWeight != 0 {
		t.Fatalf("default ranking = %+v", r)
	}
}

func TestRunnerHybridRequiresEmbedder(t *testing.T) {
	_, err := runInDir(hybridDataset(t), Config{
		Ranking: RankingConfig{Algorithm: "hybrid", BM25Weight: 0.3, SemanticWeight: 0.7},
	}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "embedder") {
		t.Fatalf("expected embedder error, got %v", err)
	}
}

func TestRunnerHybridEmptyEmbeddingFails(t *testing.T) {
	// The query embeds to nothing: a silent BM25 fallback would make the
	// run's provenance ambiguous, so it must fail.
	emb := stubEmbedder{
		"redis persistence": {1, 0},
		"redis caching":     {0, 1},
	}
	_, err := runInDir(hybridDataset(t), Config{
		TopK:     2,
		Embedder: emb,
		Ranking:  RankingConfig{Algorithm: "hybrid", BM25Weight: 0.3, SemanticWeight: 0.7},
	}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "q-redis") {
		t.Fatalf("expected query embedding error, got %v", err)
	}
}

func TestRunnerHybridValidation(t *testing.T) {
	ds := hybridDataset(t)
	emb := stubEmbedder{"redis persistence": {1, 0}, "redis caching": {0, 1}, "redis": {1, 1}}

	cases := []struct {
		name    string
		ranking RankingConfig
		wantErr string
	}{
		{"unknown algorithm", RankingConfig{Algorithm: "rrf"}, "unknown ranking algorithm"},
		{"negative weight", RankingConfig{Algorithm: "hybrid", BM25Weight: -1, SemanticWeight: 0.7}, "non-negative"},
		{"zero weights", RankingConfig{Algorithm: "hybrid"}, ""},
	}
	for _, tc := range cases {
		cfg := Config{TopK: 2, Ranking: tc.ranking}
		if tc.ranking.Algorithm == "hybrid" {
			cfg.Embedder = emb // the embedder check must not mask the one under test
		}
		_, err := runInDir(ds, cfg, t.TempDir())
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: expected error %q, got %v", tc.name, tc.wantErr, err)
		}
	}
}

func TestDiffRejectsDifferentRanking(t *testing.T) {
	ds := hybridDataset(t)
	emb := stubEmbedder{"redis persistence": {1, 0}, "redis caching": {0, 1}, "redis": {1, 1}}

	a, err := Run(ds, Config{TopK: 2, Embedder: emb, Ranking: RankingConfig{Algorithm: "hybrid", BM25Weight: 0.3, SemanticWeight: 0.7}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(ds, Config{TopK: 2, Embedder: emb, Ranking: RankingConfig{Algorithm: "hybrid", BM25Weight: 0.4, SemanticWeight: 0.6}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Diff(a, b); err == nil || !strings.Contains(err.Error(), "configurations differ") {
		t.Fatalf("expected config mismatch error, got %v", err)
	}
}

// idealEmbedder maps every document to a unique unit vector and every query
// to its grade-3 target's vector: the perfect semantic signal.
func idealEmbedder(ds Dataset) stubEmbedder {
	stub := stubEmbedder{}
	dim := make(map[string]int, len(ds.Documents))
	for i, d := range ds.Documents {
		dim[d.ID] = i
	}
	oneHot := func(i int) []float32 {
		v := make([]float32, len(ds.Documents))
		v[i] = 1
		return v
	}
	for _, d := range ds.Documents {
		stub[d.Content] = oneHot(dim[d.ID])
	}
	for _, q := range ds.Queries {
		if q.IsControl() {
			continue
		}
		// Entry-unit judgments name entries, not documents: embed the
		// entry's first document.
		id := q.Relevant[0].ID
		if _, isDoc := dim[id]; !isDoc {
			for _, d := range ds.Documents {
				if d.EntryID == id {
					id = d.ID
					break
				}
			}
		}
		stub[q.Text] = oneHot(dim[id])
	}
	return stub
}

// The committed paraphrase dataset is the algorithm-comparison fixture: its
// queries are single-token probes with a planted distractor, so BM25 can
// barely separate target from noise and the semantic signal must decide.
// This test keeps the fixture honest: the control retrieves nothing, every
// query has candidates, and a perfect embedder reaches hit1_rate 1.
func TestParaphraseFixtureComparison(t *testing.T) {
	raw, err := os.ReadFile("testdata/paraphrase-v1.jsonl")
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}
	ds, err := ParseDataset(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse dataset: %v", err)
	}

	bm25Report, err := Run(ds, Config{TopK: 3})
	if err != nil {
		t.Fatal(err)
	}
	if bm25Report.Coverage.Controls != 1 || bm25Report.Coverage.NoHitQueries != 0 {
		t.Fatalf("fixture coverage drifted: %+v", bm25Report.Coverage)
	}

	hybridReport, err := Run(ds, Config{
		TopK:           3,
		Embedder:       idealEmbedder(ds),
		EmbeddingModel: "ideal-stub",
		Ranking:        RankingConfig{Algorithm: "hybrid", BM25Weight: 0.1, SemanticWeight: 0.9},
	})
	if err != nil {
		t.Fatal(err)
	}

	bm25Agg := bm25Report.Aggregate[UnitArticle]
	hybridAgg := hybridReport.Aggregate[UnitArticle]
	if hybridAgg.Hit1Rate <= bm25Agg.Hit1Rate {
		t.Fatalf("hybrid hit1 %.3f did not beat bm25 %.3f", hybridAgg.Hit1Rate, bm25Agg.Hit1Rate)
	}
	if hybridAgg.Hit1Rate != 1 {
		t.Fatalf("perfect embedder must rank every target first, got hit1 %.3f", hybridAgg.Hit1Rate)
	}
}
