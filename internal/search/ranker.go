package search

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
)

// Ranking algorithms. Auto defers the bm25/hybrid choice to embedder
// availability; bm25 and hybrid are explicit.
const (
	AlgorithmAuto   = "auto"
	AlgorithmBM25   = "bm25"
	AlgorithmHybrid = "hybrid"
)

type Ranker struct {
	BM25Weight     float64
	SemanticWeight float64
}

func NewRanker(bm25Weight, semanticWeight float64) *Ranker {
	return &Ranker{
		BM25Weight:     bm25Weight,
		SemanticWeight: semanticWeight,
	}
}

func DefaultRanker() *Ranker {
	return NewRanker(config.DefaultBM25Weight, config.DefaultSemanticWeight)
}

// ValidAlgorithm reports whether algorithm names a known ranking mode.
func ValidAlgorithm(algorithm string) bool {
	switch algorithm {
	case AlgorithmAuto, AlgorithmBM25, AlgorithmHybrid:
		return true
	default:
		return false
	}
}

// ResolveAlgorithm maps Auto to a concrete algorithm based on embedder
// availability: hybrid when an embedder is configured, bm25 otherwise.
func ResolveAlgorithm(algorithm string, embedderAvailable bool) (string, error) {
	switch algorithm {
	case "", AlgorithmAuto:
		if embedderAvailable {
			return AlgorithmHybrid, nil
		}
		return AlgorithmBM25, nil
	case AlgorithmBM25, AlgorithmHybrid:
		return algorithm, nil
	default:
		return "", fmt.Errorf("unknown ranking algorithm %q (want auto, bm25, or hybrid)", algorithm)
	}
}

// HybridSearch combines BM25 scores with semantic similarity. Attachment
// metadata hits carry per-set normalized BM25 scores and are re-ranked only
// within their own set; semantic re-ranking applies to article hits only.
// Input scores are already direction-correct (higher = more relevant, best
// hit 1.0, worst 0.0) from the db merge; the re-normalization below is an
// identity over normalized input and never inverts it. Output stays
// direction-correct: higher = more relevant.
func (r *Ranker) HybridSearch(
	ctx context.Context,
	results []db.SearchResult,
	database *db.DB,
	queryEmbedding []float32,
) []db.SearchResult {
	if len(results) == 0 || queryEmbedding == nil {
		return results
	}

	// Find min/max BM25 scores for normalization over article hits only;
	// attachment hits were already normalized per set at query time.
	var minBM25, maxBM25 = math.MaxFloat64, -math.MaxFloat64
	hasArticle := false
	for _, res := range results {
		if res.Type != "article" {
			continue
		}
		hasArticle = true
		if res.Score < minBM25 {
			minBM25 = res.Score
		}
		if res.Score > maxBM25 {
			maxBM25 = res.Score
		}
	}
	if !hasArticle {
		return results
	}

	// Calculate hybrid scores
	for i := range results {
		if results[i].Type != "article" {
			continue
		}

		bm25Norm := NormalizeBM25(results[i].Score, minBM25, maxBM25)
		results[i].BM25Score = bm25Norm

		// Get article vector and compute similarity
		vec, err := database.GetVector(results[i].ID)
		if err != nil || vec == nil {
			// No vector available, use BM25 only
			results[i].SemanticScore = 0
			results[i].Score = r.BM25Weight*bm25Norm + r.SemanticWeight*0
			continue
		}

		// Compute cosine similarity
		cosineSim := embed.Cosine(queryEmbedding, vec)
		results[i].SemanticScore = cosineSim

		// Combined weighted score
		results[i].Score = r.BM25Weight*bm25Norm + r.SemanticWeight*cosineSim
	}

	// Sort by score descending (attachment hits keep their normalized scores)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// NormalizeBM25 rescales a direction-correct score (higher = more relevant,
// e.g. the per-set normalized BM25 that reaches this ranker) to 0-1 via
// linear min-max over the query's article hits: best hit 1.0, worst 0.0.
// A lone (or all-equal) hit maps to 0.5. It never flips ordering: if input
// scores are already in [0, 1] this is an identity.
func NormalizeBM25(score, min, max float64) float64 {
	if max-min == 0 {
		return 0.5
	}

	normalized := (score - min) / (max - min)
	return math.Max(0, math.Min(1, normalized))
}

// ReciprocalRankFusion combines ranked lists using RRF. It is currently
// unwired. Input lists must be ordered best-first; the output preserves
// direction (higher RRF score = more relevant).
func ReciprocalRankFusion(results [][]db.SearchResult, k int) []db.SearchResult {
	if k == 0 {
		k = 60
	}

	scores := make(map[string]float64)
	resultMap := make(map[string]db.SearchResult)

	for _, resultList := range results {
		for rank, r := range resultList {
			docID := r.ID
			scores[docID] += 1.0 / float64(k+rank+1)
			resultMap[docID] = r
		}
	}

	// Sort by combined score
	sorted := make([]db.SearchResult, 0, len(scores))
	for id, score := range scores {
		r := resultMap[id]
		r.Score = score
		sorted = append(sorted, r)
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	return sorted
}

// NormalizeScore maps any min/max range to 0-1. Currently unwired; it is
// direction-aware only when given an ordered range.
func NormalizeScore(score, min, max float64) float64 {
	if max == min {
		return 0.5
	}
	return math.Max(0, math.Min(1, (score-min)/(max-min)))
}
