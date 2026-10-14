package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
)

const (
	// BaselineVersion is the only supported baseline schema version.
	BaselineVersion = 1

	// baselineEpsilon tolerates float round-trips when comparing floors.
	baselineEpsilon = 1e-12
)

// MetricFloor is a set of optional metric floors. Pointer fields distinguish
// an absent floor from a zero floor. Hit1 is optional even at aggregate
// level; the other four are required there.
type MetricFloor struct {
	Precision *float64 `json:"precision,omitempty"`
	Recall    *float64 `json:"recall,omitempty"`
	MRR       *float64 `json:"mrr,omitempty"`
	NDCG      *float64 `json:"ndcg,omitempty"`
	Hit1      *float64 `json:"hit1,omitempty"`
}

// BaselineMinimum requires all four aggregate floors, per unit, matching the
// report aggregates. Hit1 is an additional optional floor there.
type BaselineMinimum struct {
	Article MetricFloor `json:"article"`
	Entry   MetricFloor `json:"entry"`
}

// Maximum holds optional resource ceilings, enforced like floors but from
// above. Only values that are stable across machines fit here (e.g. the
// deterministic benchmark database size); timing or memory must never be
// gated.
type Maximum struct {
	DBBytes *int64 `json:"db_bytes,omitempty"`
}

// Reference records one measured run's machine-dependent values (latency,
// throughput, memory) as documentation. It is never enforced: timing and
// memory vary across machines and runs, and the baseline must not flake.
type Reference struct {
	LatencyMeanMS *float64 `json:"latency_mean_ms,omitempty"`
	LatencyP95MS  *float64 `json:"latency_p95_ms,omitempty"`
	QPS           *float64 `json:"qps,omitempty"`

	SeedAllocBytes      *int64 `json:"seed_alloc_bytes,omitempty"`
	RetrievalAllocBytes *int64 `json:"retrieval_alloc_bytes,omitempty"`
	HeapAllocPeakBytes  *int64 `json:"heap_alloc_peak_bytes,omitempty"`
	GCs                 *int64 `json:"gcs,omitempty"`
}

// BaselineCase binds floors to one exact dataset: its SHA-256, the run
// configuration, and the candidate policy. A case whose dataset bytes change
// stops matching and must be re-measured explicitly.
type BaselineCase struct {
	Dataset         string                 `json:"dataset"`
	DatasetSHA256   string                 `json:"dataset_sha256"`
	TopK            int                    `json:"top_k"`
	CandidatePolicy string                 `json:"candidate_policy"`
	Minimum         BaselineMinimum        `json:"minimum"`
	PerQuery        map[string]MetricFloor `json:"per_query_minimum,omitempty"`
	Maximum         Maximum                `json:"maximum,omitempty"`
	Reference       *Reference             `json:"reference,omitempty"`
}

// Baseline is a committed set of cases loaded from JSON.
type Baseline struct {
	Version int            `json:"version"`
	Cases   []BaselineCase `json:"cases"`
}

// LoadBaseline parses and validates a baseline file. Baseline edits are
// explicit reviewed changes; nothing here auto-accepts floors.
func LoadBaseline(r io.Reader) (Baseline, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var b Baseline
	if err := dec.Decode(&b); err != nil {
		return Baseline{}, fmt.Errorf("parse baseline: %w", err)
	}
	if dec.More() {
		return Baseline{}, fmt.Errorf("parse baseline: trailing data after JSON value")
	}

	if b.Version != BaselineVersion {
		return Baseline{}, fmt.Errorf("unsupported baseline version %d (want %d)", b.Version, BaselineVersion)
	}
	if len(b.Cases) == 0 {
		return Baseline{}, fmt.Errorf("baseline has no cases")
	}

	seen := map[string]bool{}
	for i, c := range b.Cases {
		if c.Dataset == "" {
			return Baseline{}, fmt.Errorf("case %d: dataset required", i)
		}
		if seen[c.Dataset] {
			return Baseline{}, fmt.Errorf("case %d: duplicate dataset binding %s", i, c.Dataset)
		}
		seen[c.Dataset] = true
		if c.DatasetSHA256 == "" {
			return Baseline{}, fmt.Errorf("case %s: dataset_sha256 required", c.Dataset)
		}
		if c.TopK <= 0 {
			return Baseline{}, fmt.Errorf("case %s: top_k must be positive", c.Dataset)
		}
		if c.CandidatePolicy == "" {
			return Baseline{}, fmt.Errorf("case %s: candidate_policy required", c.Dataset)
		}
		if c.CandidatePolicy != CandidatePolicyUnbounded {
			return Baseline{}, fmt.Errorf("case %s: unknown candidate policy %q", c.Dataset, c.CandidatePolicy)
		}
		for _, f := range []MetricFloor{c.Minimum.Article, c.Minimum.Entry} {
			for _, v := range []*float64{f.Precision, f.Recall, f.MRR, f.NDCG} {
				if v == nil {
					return Baseline{}, fmt.Errorf("case %s: aggregate floors must cover all four metrics", c.Dataset)
				}
				if !validFloor(*v) {
					return Baseline{}, fmt.Errorf("case %s: floors must be finite and within [0,1]", c.Dataset)
				}
			}
			if f.Hit1 != nil && !validFloor(*f.Hit1) {
				return Baseline{}, fmt.Errorf("case %s: floors must be finite and within [0,1]", c.Dataset)
			}
		}
		for qid, f := range c.PerQuery {
			for _, v := range []*float64{f.Precision, f.Recall, f.MRR, f.NDCG, f.Hit1} {
				if v != nil && !validFloor(*v) {
					return Baseline{}, fmt.Errorf("case %s: per-query floor for %s must be finite and within [0,1]", c.Dataset, qid)
				}
			}
		}
		if v := c.Maximum.DBBytes; v != nil && *v <= 0 {
			return Baseline{}, fmt.Errorf("case %s: maximum db_bytes must be positive", c.Dataset)
		}
	}
	return b, nil
}

func validFloor(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1
}

// Match returns the case bound to a dataset ID.
func (b Baseline) Match(datasetID string) (BaselineCase, error) {
	for _, c := range b.Cases {
		if c.Dataset == datasetID {
			return c, nil
		}
	}
	return BaselineCase{}, fmt.Errorf("baseline has no case for dataset %s", datasetID)
}

// Check verifies a report against the case and returns every violation. A
// report from different dataset bytes, cutoff, or candidate policy is an
// error, not a violation: it must be re-bound explicitly.
func (c BaselineCase) Check(report Report) error {
	if report.Dataset.SHA256 != c.DatasetSHA256 {
		return fmt.Errorf("dataset %s changed since baseline was recorded (sha256 %s != %s); re-measure before editing floors",
			report.Dataset.ID, report.Dataset.SHA256, c.DatasetSHA256)
	}
	if report.Config.TopK != c.TopK {
		return fmt.Errorf("top_k %d does not match baseline top_k %d", report.Config.TopK, c.TopK)
	}
	if report.Config.CandidatePolicy != c.CandidatePolicy {
		return fmt.Errorf("candidate policy %q does not match baseline policy %q", report.Config.CandidatePolicy, c.CandidatePolicy)
	}

	var violations []string
	for unit, floors := range map[string]MetricFloor{
		UnitArticle: c.Minimum.Article,
		UnitEntry:   c.Minimum.Entry,
	} {
		agg, ok := report.Aggregate[unit]
		if !ok {
			return fmt.Errorf("report has no aggregate for unit %s", unit)
		}
		violations = append(violations, checkFloor(fmt.Sprintf("aggregate %s", unit), nameMap(floors), map[string]float64{
			"precision": agg.Precision,
			"recall":    agg.Recall,
			"mrr":       agg.MRR,
			"ndcg":      agg.NDCG,
			"hit1":      agg.Hit1Rate,
		})...)
	}

	for _, q := range sortedQueryIDs(c.PerQuery) {
		floors := c.PerQuery[q]
		var qr *QueryReport
		for i := range report.Queries {
			if report.Queries[i].ID == q {
				qr = &report.Queries[i]
				break
			}
		}
		if qr == nil {
			return fmt.Errorf("per-query floor references unknown query %s", q)
		}
		if qr.Control {
			return fmt.Errorf("per-query floor references control query %s", q)
		}
		if qr.Metrics == nil {
			return fmt.Errorf("per-query floor for %s has no metrics", q)
		}
		violations = append(violations, checkFloor(fmt.Sprintf("query %s", q), nameMap(floors), map[string]float64{
			"precision": qr.Metrics.Precision,
			"recall":    qr.Metrics.Recall,
			"mrr":       qr.Metrics.MRR,
			"ndcg":      qr.Metrics.NDCG,
		})...)
	}

	if c.Maximum.DBBytes != nil && report.Storage.DBBytes > *c.Maximum.DBBytes {
		violations = append(violations, fmt.Sprintf("storage: db_bytes %d > cap %d", report.Storage.DBBytes, *c.Maximum.DBBytes))
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		return fmt.Errorf("baseline violated:\n  %s", joinLines(violations))
	}
	return nil
}

// checkFloor compares each present floor against actual values and returns
// violation strings.
func checkFloor(scope string, floors, actual map[string]float64) []string {
	var violations []string
	for _, name := range []string{"precision", "recall", "mrr", "ndcg", "hit1"} {
		floor, ok := floors[name]
		if !ok {
			continue
		}
		value, ok := actual[name]
		if !ok {
			return []string{fmt.Sprintf("%s: %s not measurable here", scope, name)}
		}
		if value < floor-baselineEpsilon {
			violations = append(violations, fmt.Sprintf("%s: %s %.6f < floor %.6f", scope, name, value, floor))
		}
	}
	return violations
}

func sortedQueryIDs(m map[string]MetricFloor) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// nameMap flattens present pointer floors to name -> value.
func nameMap(f MetricFloor) map[string]float64 {
	out := map[string]float64{}
	if f.Precision != nil {
		out["precision"] = *f.Precision
	}
	if f.Recall != nil {
		out["recall"] = *f.Recall
	}
	if f.MRR != nil {
		out["mrr"] = *f.MRR
	}
	if f.NDCG != nil {
		out["ndcg"] = *f.NDCG
	}
	if f.Hit1 != nil {
		out["hit1"] = *f.Hit1
	}
	return out
}

func joinLines(lines []string) string {
	buf := new(bytes.Buffer)
	for i, l := range lines {
		if i > 0 {
			buf.WriteString("\n  ")
		}
		buf.WriteString(l)
	}
	return buf.String()
}

// EvaluateBaseline loads the baseline, matches the case for the dataset, and
// checks the report, returning a filled BaselineResult for the report.
func EvaluateBaseline(base Baseline, datasetID string, report Report) (*BaselineResult, error) {
	c, err := base.Match(datasetID)
	if err != nil {
		return nil, err
	}
	err = c.Check(report)
	res := &BaselineResult{Pass: err == nil}
	if err != nil {
		res.Violations = violationStrings(err)
	}
	return res, err
}

// violationStrings keeps the multi-line detail of a Check error.
func violationStrings(err error) []string {
	var lines []string
	for _, part := range bytes.Split([]byte(err.Error()), []byte("\n")) {
		s := string(bytes.TrimSpace(part))
		if s != "" && s != "baseline violated:" {
			lines = append(lines, s)
		}
	}
	return lines
}
