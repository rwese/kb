package search

import (
	"math"

	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
)

type Ranker struct {
	PromptWeight  float64
	ContextWeight float64
	BM25Weight    float64
}

func DefaultRanker() *Ranker {
	return &Ranker{
		PromptWeight:  3.0,
		ContextWeight: -0.5,
		BM25Weight:    1.0,
	}
}

// WeightedResults re-ranks results using prompt/context embeddings
func (r *Ranker) WeightedResults(
	results []db.SearchResult,
	promptEmbedding []float32,
	contextEmbedding []float32,
) []db.SearchResult {
	if len(results) == 0 || promptEmbedding == nil {
		return results
	}

	// Calculate embedding for concatenated content
	for i := range results {
		content := results[i].Title + " " + results[i].Content

		promptSim := embed.Cosine(promptEmbedding, mustEmbed(content))
		var contextSim float64
		if contextEmbedding != nil {
			contextSim = embed.Cosine(contextEmbedding, mustEmbed(content))
		}

		// Normalize BM25 (typically negative, higher is better)
		bm25Score := -results[i].Score + 1

		// Combined weighted score
		results[i].Score = r.BM25Weight*bm25Score +
			r.PromptWeight*promptSim +
			r.ContextWeight*contextSim
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

func mustEmbed(text string) []float32 {
	// Placeholder - actual implementation would use the embedder
	// This is a zero vector placeholder for when embedder is None
	return make([]float32, 384)
}

// HybridSearch combines BM25 and semantic search
func HybridSearch(bm25Results, semanticResults []db.SearchResult, topK int) []db.SearchResult {
	// Reciprocal Rank Fusion
	scores := make(map[int64]float64)

	for i, r := range bm25Results {
		scores[r.ID] += 1.0 / float64(61+i) // rank starts at 1
	}
	for i, r := range semanticResults {
		scores[r.ID] += 1.0 / float64(61+i)
	}

	// Sort by combined score
	sorted := make([]db.SearchResult, 0, len(scores))
	for id, score := range scores {
		// Find the entry
		for _, r := range bm25Results {
			if r.ID == id {
				r.Score = score
				sorted = append(sorted, r)
				break
			}
		}
		for _, r := range semanticResults {
			if r.ID == id {
				found := false
				for _, s := range sorted {
					if s.ID == id {
						found = true
						break
					}
				}
				if !found {
					r.Score = score
					sorted = append(sorted, r)
				}
				break
			}
		}
	}

	// Sort by score
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Score > sorted[i].Score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if len(sorted) > topK {
		sorted = sorted[:topK]
	}

	return sorted
}

// Normalize score to 0-1 range
func NormalizeScore(score, min, max float64) float64 {
	if max == min {
		return 0.5
	}
	return math.Max(0, math.Min(1, (score-min)/(max-min)))
}
