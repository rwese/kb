package benchmark

import (
	"fmt"
	"math"
	"sort"
)

// QueryDiff is the per-query delta between two reports on the same dataset
// and configuration.
type QueryDiff struct {
	ID string `json:"id"`
	// Before/After per-query metrics; nil when a control.
	Before *Metrics `json:"before,omitempty"`
	After  *Metrics `json:"after,omitempty"`
	// Reordered is true when the top-k ID sequence changed between runs.
	Reordered bool `json:"reordered,omitempty"`
}

// MetricDelta is one metric's change between reports.
type MetricDelta struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
	Delta  float64 `json:"delta"` // after - before
}

// AggregateDiff is one unit's aggregate metric deltas.
type AggregateDiff struct {
	Queries   int         `json:"queries"`
	Precision MetricDelta `json:"precision"`
	Recall    MetricDelta `json:"recall"`
	MRR       MetricDelta `json:"mrr"`
	NDCG      MetricDelta `json:"ndcg"`
	Hit1      MetricDelta `json:"hit1"`
}

// DiffReport summarizes what changed between two runs of the same bound
// dataset and configuration.
type DiffReport struct {
	Dataset       string                   `json:"dataset"`
	SHA256        string                   `json:"sha256"`
	TopK          int                      `json:"top_k"`
	Queries       int                      `json:"queries"`
	Reordered     []string                 `json:"reordered,omitempty"`
	Aggregate     map[string]AggregateDiff `json:"aggregate"`
	QueriesDetail []QueryDiff              `json:"queries_detail,omitempty"`
}

// Diff compares two reports run against the same dataset bytes and
// configuration. Any binding mismatch (dataset SHA-256, top_k, or candidate
// policy) is an error: a diff across different setups would be noise.
func Diff(a, b Report) (DiffReport, error) {
	if a.Dataset.SHA256 != b.Dataset.SHA256 {
		return DiffReport{}, fmt.Errorf("dataset bytes differ (sha256 %s vs %s); diffs are only valid on identical dataset bytes",
			a.Dataset.SHA256, b.Dataset.SHA256)
	}
	if a.Config != b.Config {
		return DiffReport{}, fmt.Errorf("run configurations differ (%+v vs %+v)", a.Config, b.Config)
	}

	d := DiffReport{
		Dataset:   a.Dataset.ID,
		SHA256:    a.Dataset.SHA256,
		TopK:      a.Config.TopK,
		Queries:   len(a.Queries),
		Aggregate: map[string]AggregateDiff{},
	}

	for unit, aggA := range a.Aggregate {
		aggB, ok := b.Aggregate[unit]
		if !ok {
			return DiffReport{}, fmt.Errorf("report has no aggregate for unit %s", unit)
		}
		d.Aggregate[unit] = AggregateDiff{
			Queries:   aggA.Queries,
			Precision: newDelta(aggA.Precision, aggB.Precision),
			Recall:    newDelta(aggA.Recall, aggB.Recall),
			MRR:       newDelta(aggA.MRR, aggB.MRR),
			NDCG:      newDelta(aggA.NDCG, aggB.NDCG),
			Hit1:      newDelta(aggA.Hit1Rate, aggB.Hit1Rate),
		}
	}

	byID := map[string]QueryReport{}
	for _, qr := range b.Queries {
		byID[qr.ID] = qr
	}

	for _, qa := range a.Queries {
		qb, ok := byID[qa.ID]
		if !ok {
			return DiffReport{}, fmt.Errorf("query %s missing from second report", qa.ID)
		}
		qd := QueryDiff{ID: qa.ID, Before: qa.Metrics, After: qb.Metrics}

		if qa.Control {
			if !qb.Control || len(qb.Ranked) != 0 {
				return DiffReport{}, fmt.Errorf("query %s is a control in one report only, or retrieves hits now", qa.ID)
			}
			d.QueriesDetail = append(d.QueriesDetail, qd)
			continue
		}

		if !sameOrder(qa.Ranked, qb.Ranked) {
			qd.Reordered = true
			d.Reordered = append(d.Reordered, qa.ID)
		}
		d.QueriesDetail = append(d.QueriesDetail, qd)
	}

	sort.Strings(d.Reordered)
	return d, nil
}

// toInt rounds to 12 decimals to kill float noise in JSON output.
func newDelta(before, after float64) MetricDelta {
	return MetricDelta{Before: round(before), After: round(after), Delta: round(after - before)}
}

func round(v float64) float64 {
	return math.Round(v*1e12) / 1e12
}

func sameOrder(a, b []RankedHit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}
