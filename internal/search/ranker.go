package search

import (
	"context"
	"math"
	"sort"

	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
)

type Ranker struct {
	BM25Weight    float64
	SemanticWeight float64
}

func NewRanker(bm25Weight, semanticWeight float64) *Ranker {
	return &Ranker{
		BM25Weight:    bm25Weight,
		SemanticWeight: semanticWeight,
	}
}

func DefaultRanker() *Ranker {
	return &Ranker{
		BM25Weight:    0.3,
		SemanticWeight: 0.7,
	}
}

// HybridSearch combines BM25 scores with semantic similarity
func (r *Ranker) HybridSearch(
	ctx context.Context,
	results []db.SearchResult,
	database *db.DB,
	queryEmbedding []float32,
) []db.SearchResult {
	if len(results) == 0 || queryEmbedding == nil {
		return results
	}

	// Find min/max BM25 scores for normalization
	var minBM25, maxBM25 float64 = math.MaxFloat64, -math.MaxFloat64
	for _, res := range results {
		if res.Score < minBM25 {
			minBM25 = res.Score
		}
		if res.Score > maxBM25 {
			maxBM25 = res.Score
		}
	}

	// Calculate hybrid scores
	for i := range results {
		// Normalize BM25 score to 0-1 (BM25 is typically negative, higher is better)
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

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// NormalizeBM25 converts BM25 score to 0-1 range using sigmoid
func NormalizeBM25(score, min, max float64) float64 {
	// BM25 scores are typically negative, use sigmoid for normalization
	// Map to approximately 0-1 range
	if max-min == 0 {
		return 0.5
	}

	// Linear normalization adjusted for negative BM25
	normalized := (score - min) / (max - min)
	return math.Max(0, math.Min(1, normalized))
}

// ReciprocalRankFusion combines ranked lists using RRF
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

// Normalize score to 0-1 range
func NormalizeScore(score, min, max float64) float64 {
	if max == min {
		return 0.5
	}
	return math.Max(0, math.Min(1, (score-min)/(max-min)))
}
