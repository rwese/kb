package benchmark

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCoverageAndCandidates(t *testing.T) {
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"SQLite storage"}`,
		`{"type":"entry","id":"def456","title":"Unrelated"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"sqlite alpha"}`,
		`{"type":"document","id":"abc123-000002","entry_id":"abc123","title":"Beta","content":"sqlite beta"}`,
		`{"type":"document","id":"def456-000001","entry_id":"def456","title":"Gamma","content":"nothing here"}`,
		`{"type":"query","id":"q-a","text":"sqlite","unit":"article","relevant":[{"id":"abc123-000001","grade":3},{"id":"abc123-000002","grade":2}]}`,
		`{"type":"query","id":"q-entry","text":"sqlite","unit":"entry","relevant":[{"id":"abc123","grade":3}]}`,
		`{"type":"query","id":"q-miss","text":"zzz","unit":"article","relevant":[{"id":"def456-000001","grade":3}]}`,
		`{"type":"query","id":"q-control","text":"_MISSING_","unit":"article"}`,
	}, "\n"))

	rep, err := Run(ds, Config{TopK: 2})
	if err != nil {
		t.Fatal(err)
	}

	cov := rep.Coverage
	if cov.Entries != 2 || cov.Documents != 3 || cov.Controls != 1 {
		t.Fatalf("unexpected coverage totals: %+v", cov)
	}
	// q-a recovers both sqlite docs; q-entry dedups to an entry (not
	// counted as a document); q-miss retrieves nothing.
	if cov.DocumentsRecovered != 2 {
		t.Fatalf("documents_recovered = %d, want 2", cov.DocumentsRecovered)
	}
	if cov.NoHitQueries != 1 || cov.ZeroRecallQueries != 1 {
		t.Fatalf("no-hit/zero-recall = %d/%d, want 1/1", cov.NoHitQueries, cov.ZeroRecallQueries)
	}

	byID := map[string]QueryReport{}
	for _, qr := range rep.Queries {
		byID[qr.ID] = qr
	}
	if c := byID["q-a"].Candidates; c != 2 {
		t.Fatalf("q-a candidates = %d, want 2", c)
	}
	if c := byID["q-miss"].Candidates; c != 0 {
		t.Fatalf("q-miss candidates = %d, want 0", c)
	}

	agg, ok := rep.Aggregate[UnitArticle]
	if !ok || agg.Queries != 2 {
		t.Fatalf("unexpected article aggregate: %+v", agg)
	}
	// q-a hits at rank 1, q-miss does not.
	if agg.Hit1Rate != 0.5 {
		t.Fatalf("hit1_rate = %v, want 0.5", agg.Hit1Rate)
	}

	// New informational fields are present.
	if rep.Latency.QPS <= 0 {
		t.Fatalf("qps = %v, want > 0", rep.Latency.QPS)
	}
	if rep.Mem.SeedAllocBytes <= 0 || rep.Mem.RetrievalAllocBytes <= 0 {
		t.Fatalf("mem deltas missing: %+v", rep.Mem)
	}
	if rep.Storage.DBBytes <= 0 {
		t.Fatalf("db bytes = %d, want > 0", rep.Storage.DBBytes)
	}
}

func TestDiffReports(t *testing.T) {
	base := Report{
		Version: ReportVersion,
		Dataset: DatasetInfo{ID: "t1", SHA256: "cafe"},
		Config: ConfigReport{
			TopK:            3,
			CandidatePolicy: CandidatePolicyUnbounded,
			Ranking:         RankingConfig{Name: "bm25-porter-v1"},
		},
		Aggregate: map[string]UnitAggregate{
			UnitArticle: {Queries: 2, Precision: 0.5, Recall: 1, MRR: 0.5, NDCG: 0.8, Hit1Rate: 0.5},
		},
		Queries: []QueryReport{
			{
				ID: "q1", Unit: UnitArticle,
				Metrics: &Metrics{Precision: 0.5, Recall: 1, MRR: 0.5, NDCG: 0.8},
				Ranked: []RankedHit{
					{ID: "x", Grade: 0},
					{ID: "a", Grade: 3},
				},
			},
			{ID: "q2", Unit: UnitArticle, Control: true},
		},
	}

	// Identical report: no changes.
	same := base
	same.Queries = cloneQueries(base.Queries)
	same.Aggregate = cloneAggregates(base.Aggregate)
	d, err := Diff(base, same)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Reordered) != 0 {
		t.Fatalf("identical reports must not reorder: %v", d.Reordered)
	}
	if delta := d.Aggregate[UnitArticle].NDCG.Delta; delta != 0 {
		t.Fatalf("ndcg delta = %v, want 0", delta)
	}

	// Improved ranking: q1 puts a(3) first; aggregate shifts.
	better := base
	better.Aggregate = map[string]UnitAggregate{
		UnitArticle: {Queries: 2, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1, Hit1Rate: 1},
	}
	better.Queries = cloneQueries(base.Queries)
	better.Queries[0].Metrics = &Metrics{Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1}
	better.Queries[0].Ranked = []RankedHit{
		{ID: "a", Grade: 3},
		{ID: "x", Grade: 0},
	}
	d, err = Diff(base, better)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Aggregate[UnitArticle].MRR.Delta; got != 0.5 {
		t.Fatalf("mrr delta = %v, want 0.5", got)
	}
	if !containsID(d.Reordered, "q1") {
		t.Fatalf("q1 must be reordered: %v", d.Reordered)
	}
}

func containsID(ids []string, id string) bool {
	for _, s := range ids {
		if s == id {
			return true
		}
	}
	return false
}

func cloneQueries(qs []QueryReport) []QueryReport {
	out := make([]QueryReport, len(qs))
	for i, q := range qs {
		out[i] = q
		if q.Metrics != nil {
			m := *q.Metrics
			out[i].Metrics = &m
		}
	}
	return out
}

func cloneAggregates(a map[string]UnitAggregate) map[string]UnitAggregate {
	out := map[string]UnitAggregate{}
	for k, v := range a {
		out[k] = v
	}
	return out
}

func TestDiffBindingErrors(t *testing.T) {
	base := Report{
		Version: ReportVersion,
		Dataset: DatasetInfo{ID: "t1", SHA256: "cafe"},
		Config:  ConfigReport{TopK: 3, CandidatePolicy: CandidatePolicyUnbounded, Ranking: RankingConfig{Name: "bm25-porter-v1"}},
		Aggregate: map[string]UnitAggregate{
			UnitArticle: {Queries: 1},
		},
	}
	if _, err := Diff(base, base); err != nil {
		t.Fatalf("empty-query diff should pass: %v", err)
	}

	// Different dataset bytes.
	other := base
	other.Dataset.SHA256 = "beef"
	if _, err := Diff(base, other); err == nil || !strings.Contains(err.Error(), "dataset bytes differ") {
		t.Fatalf("sha mismatch must error: %v", err)
	}

	// Different configuration.
	other = base
	other.Config.TopK = 5
	if _, err := Diff(base, other); err == nil || !strings.Contains(err.Error(), "configurations differ") {
		t.Fatalf("config mismatch must error: %v", err)
	}

	// Missing unit aggregate.
	other = base
	other.Aggregate = map[string]UnitAggregate{}
	if _, err := Diff(base, other); err == nil || !strings.Contains(err.Error(), "no aggregate") {
		t.Fatalf("missing unit must error: %v", err)
	}
}

func TestDiffJSONShape(t *testing.T) {
	// Ensures the diff marshals as documented.
	a := Report{
		Version: ReportVersion,
		Dataset: DatasetInfo{ID: "t1", SHA256: "cafe"},
		Config:  ConfigReport{TopK: 3, CandidatePolicy: CandidatePolicyUnbounded, Ranking: RankingConfig{Name: "bm25-porter-v1"}},
		Aggregate: map[string]UnitAggregate{
			UnitArticle: {Queries: 1, MRR: 0.5},
		},
		Queries: []QueryReport{{
			ID: "q1", Unit: UnitArticle,
			Metrics: &Metrics{MRR: 0.5},
			Ranked:  []RankedHit{{ID: "a", Grade: 3}},
		}},
	}
	b := a
	b.Aggregate = cloneAggregates(a.Aggregate)
	b.Queries = []QueryReport{{
		ID: "q1", Unit: UnitArticle,
		Metrics: &Metrics{MRR: 1},
		Ranked:  []RankedHit{{ID: "a", Grade: 3}},
	}}
	d, err := Diff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var back DiffReport
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("diff roundtrip: %v", err)
	}
	if back.QueriesDetail[0].After == nil || back.QueriesDetail[0].After.MRR != 1 || back.QueriesDetail[0].Before.MRR != 0.5 {
		t.Fatalf("per-query metrics lost in roundtrip: %+v", back)
	}
}
