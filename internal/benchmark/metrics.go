package benchmark

import (
	"fmt"
	"math"
)

// Metrics holds the relevance scores for one ranked list at one cutoff k.
// All values are in [0, 1].
type Metrics struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	MRR       float64 `json:"mrr"`
	NDCG      float64 `json:"ndcg"`
}

// Evaluate scores one ranked ID list against judgments at cutoff k.
//
//   - Positive relevance is grade > 0; grade 0 items are judged non-relevant.
//   - Precision@k: positive hits in the top k divided by k (denominator is
//     always k, not the result count).
//   - Recall@k: positive hits in the top k divided by the total number of
//     positive judgments.
//   - MRR@k: reciprocal rank of the first positive hit within k, else 0.
//   - nDCG@k: gain 2^grade-1, discount log2(rank+1); ideal ordering is all
//     judged items by descending grade truncated to k.
//
// Result IDs are deduplicated keeping the first occurrence. Unjudged results
// count as grade 0. No-match control queries (no judgments) must be excluded
// by the caller; calling with empty judgments yields zero metrics.
func Evaluate(ids []string, judgments []Judgment, k int) (Metrics, error) {
	if k <= 0 {
		return Metrics{}, fmt.Errorf("k must be positive, got %d", k)
	}

	grade := make(map[string]int, len(judgments))
	positives := 0
	for _, j := range judgments {
		if j.Grade > 0 {
			positives++
		}
		grade[j.ID] = j.Grade
	}

	// Deduplicate ranked ids, keep first occurrence, truncate to k.
	deduped := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		deduped = append(deduped, id)
		if len(deduped) == k {
			break
		}
	}

	var m Metrics
	hits := 0
	for i, id := range deduped {
		g := grade[id]
		if g <= 0 {
			continue
		}
		hits++
		if m.MRR == 0 {
			m.MRR = 1.0 / float64(i+1)
		}
		m.NDCG += dcgGain(g, i+1)
	}

	if len(deduped) == 0 {
		return Metrics{}, nil
	}

	m.Precision = float64(hits) / float64(k)
	if positives > 0 {
		m.Recall = float64(hits) / float64(positives)
	}
	m.NDCG /= idealDCG(judgments, k)
	return m, nil
}

// dcgGain returns the discounted gain of grade g at 1-based rank i.
func dcgGain(g, rank int) float64 {
	if g <= 0 {
		return 0
	}
	return float64(int(1)<<uint(g)-1) / math.Log2(float64(rank+1))
}

// idealDCG computes the DCG of the ideal ordering: all judged grades sorted
// descending, truncated to k.
func idealDCG(judgments []Judgment, k int) float64 {
	grades := make([]int, 0, len(judgments))
	for _, j := range judgments {
		if j.Grade > 0 {
			grades = append(grades, j.Grade)
		}
	}
	// Grades are 0..3, counting sort keeps this simple.
	buckets := make([]int, 4)
	for _, g := range grades {
		buckets[g]++
	}
	sorted := make([]int, 0, len(grades))
	for g := 3; g >= 1; g-- {
		for i := 0; i < buckets[g]; i++ {
			sorted = append(sorted, g)
		}
	}

	ideal := 0.0
	for i, g := range sorted {
		if i == k {
			break
		}
		ideal += dcgGain(g, i+1)
	}
	if ideal == 0 {
		return 1 // avoid division by zero; metrics stay zero
	}
	return ideal
}
