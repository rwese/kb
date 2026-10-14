package benchmark

import (
	"strings"
	"testing"
)

func validBaselineJSON() string {
	return `{
		"version": 1,
		"cases": [{
			"dataset": "t1",
			"dataset_sha256": "` + hashBytes([]byte("x")) + `",
			"top_k": 3,
			"candidate_policy": "unbounded",
			"minimum": {
				"article": {"precision": 0.4, "recall": 1.0, "mrr": 1.0, "ndcg": 1.0},
				"entry": {"precision": 0.4, "recall": 1.0, "mrr": 1.0, "ndcg": 1.0}
			},
			"per_query_minimum": {
				"q1": {"recall": 1.0}
			}
		}]
	}`
}

func mustLoadBaseline(t *testing.T, input string) Baseline {
	t.Helper()
	b, err := LoadBaseline(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	return b
}

func TestLoadBaselineValid(t *testing.T) {
	b := mustLoadBaseline(t, validBaselineJSON())
	if len(b.Cases) != 1 {
		t.Fatalf("want 1 case, got %d", len(b.Cases))
	}
	c, err := b.Match("t1")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if c.TopK != 3 || len(c.PerQuery) != 1 {
		t.Fatalf("unexpected case: %+v", c)
	}
}

func TestLoadBaselineErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "bad json",
			input: `{`,
			err:   "parse baseline",
		},
		{
			name:  "unsupported version",
			input: `{"version":2,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "unsupported baseline version",
		},
		{
			name:  "no cases",
			input: `{"version":1,"cases":[]}`,
			err:   "no cases",
		},
		{
			name:  "missing sha",
			input: `{"version":1,"cases":[{"dataset":"t1","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "dataset_sha256 required",
		},
		{
			name:  "bad top_k",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":0,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "top_k must be positive",
		},
		{
			name:  "missing policy",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "candidate_policy required",
		},
		{
			name:  "unknown policy",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"bounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "unknown candidate policy",
		},
		{
			name:  "incomplete aggregate floors",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "all four metrics",
		},
		{
			name:  "floor above 1",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":1.5,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "within [0,1]",
		},
		{
			name:  "per-query floor above 1",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}},"per_query_minimum":{"q1":{"recall":2}}}]}`,
			err:   "within [0,1]",
		},
		{
			name:  "duplicate dataset binding",
			input: `{"version":1,"cases":[{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}},{"dataset":"t1","dataset_sha256":"x","top_k":3,"candidate_policy":"unbounded","minimum":{"article":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1},"entry":{"precision":0.4,"recall":1,"mrr":1,"ndcg":1}}}]}`,
			err:   "duplicate dataset binding",
		},
		{
			name:  "unknown field",
			input: `{"version":1,"cases":[],"auto":true}`,
			err:   "unknown field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadBaseline(strings.NewReader(tt.input))
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("want error containing %q, got %v", tt.err, err)
			}
		})
	}
}

func TestMatchMissing(t *testing.T) {
	b := mustLoadBaseline(t, validBaselineJSON())
	if _, err := b.Match("other"); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func fixtureReport(sha string, topK int, agg map[string]UnitAggregate, queries []QueryReport) Report {
	return Report{
		Version: ReportVersion,
		Dataset: DatasetInfo{ID: "t1", SHA256: sha},
		Config: ConfigReport{
			TopK:            topK,
			CandidatePolicy: CandidatePolicyUnbounded,
			Ranking:         RankingConfig{Name: "bm25-porter-v1"},
		},
		Aggregate: agg,
		Queries:   queries,
	}
}

func TestCheckBaselinePass(t *testing.T) {
	c := mustLoadBaseline(t, validBaselineJSON()).Cases[0]

	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}
	queries := []QueryReport{{
		ID:      "q1",
		Unit:    UnitArticle,
		Metrics: &Metrics{Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}}

	rep := fixtureReport(hashBytes([]byte("x")), 3, agg, queries)
	if err := c.Check(rep); err != nil {
		t.Fatalf("Check should pass: %v", err)
	}
}

func TestCheckBaselineBindingErrors(t *testing.T) {
	c := mustLoadBaseline(t, validBaselineJSON()).Cases[0]
	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}

	// Different dataset bytes.
	if err := c.Check(fixtureReport("deadbeef", 3, agg, nil)); err == nil || !strings.Contains(err.Error(), "re-measure") {
		t.Fatalf("sha mismatch must demand re-measure: %v", err)
	}
	// Different cutoff.
	if err := c.Check(fixtureReport(hashBytes([]byte("x")), 5, agg, nil)); err == nil || !strings.Contains(err.Error(), "top_k") {
		t.Fatalf("top_k mismatch must error: %v", err)
	}
	// Different candidate policy.
	rep := fixtureReport(hashBytes([]byte("x")), 3, agg, nil)
	rep.Config.CandidatePolicy = "bounded"
	if err := c.Check(rep); err == nil || !strings.Contains(err.Error(), "candidate policy") {
		t.Fatalf("policy mismatch must error: %v", err)
	}
}

func TestCheckBaselineViolations(t *testing.T) {
	c := mustLoadBaseline(t, validBaselineJSON()).Cases[0]

	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.1, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.5, Recall: 0.5, MRR: 1, NDCG: 1},
	}
	queries := []QueryReport{{
		ID:      "q1",
		Unit:    UnitArticle,
		Metrics: &Metrics{Precision: 0.5, Recall: 0.5, MRR: 1, NDCG: 1},
	}}

	err := c.Check(fixtureReport(hashBytes([]byte("x")), 3, agg, queries))
	if err == nil {
		t.Fatal("expected violations")
	}
	for _, want := range []string{"aggregate article: precision", "aggregate entry: recall", "query q1: recall"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing violation %q in:\n%v", want, err)
		}
	}
}

func TestCheckBaselinePerQueryReferences(t *testing.T) {
	c := mustLoadBaseline(t, validBaselineJSON()).Cases[0]
	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}

	// Unknown query.
	if err := c.Check(fixtureReport(hashBytes([]byte("x")), 3, agg, nil)); err == nil || !strings.Contains(err.Error(), "unknown query q1") {
		t.Fatalf("unknown per-query reference must error: %v", err)
	}
	// Control query.
	control := []QueryReport{{ID: "q1", Unit: UnitArticle, Control: true}}
	if err := c.Check(fixtureReport(hashBytes([]byte("x")), 3, agg, control)); err == nil || !strings.Contains(err.Error(), "control query q1") {
		t.Fatalf("control per-query floor must error: %v", err)
	}
}

func TestCheckBaselineEpsilon(t *testing.T) {
	c := mustLoadBaseline(t, validBaselineJSON()).Cases[0]
	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.4 - 5e-13, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.4, Recall: 1, MRR: 1, NDCG: 1},
	}
	queries := []QueryReport{{
		ID:      "q1",
		Unit:    UnitArticle,
		Metrics: &Metrics{Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}}
	if err := c.Check(fixtureReport(hashBytes([]byte("x")), 3, agg, queries)); err != nil {
		t.Fatalf("round-trip noise within epsilon must pass: %v", err)
	}
}

func TestEvaluateBaselineAttachesResult(t *testing.T) {
	b := mustLoadBaseline(t, validBaselineJSON())
	agg := map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
		UnitEntry:   {Queries: 1, Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1},
	}
	queries := []QueryReport{{ID: "q1", Unit: UnitArticle, Metrics: &Metrics{Precision: 0.5, Recall: 1, MRR: 1, NDCG: 1}}}
	rep := fixtureReport(hashBytes([]byte("x")), 3, agg, queries)

	res, err := EvaluateBaseline(b, "t1", rep)
	if err != nil {
		t.Fatalf("EvaluateBaseline: %v", err)
	}
	if res == nil || !res.Pass || len(res.Violations) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	// Failing case attaches violations and returns the error.
	bad := fixtureReport(hashBytes([]byte("x")), 3, map[string]UnitAggregate{
		UnitArticle: {Queries: 1, Precision: 0.0, Recall: 0, MRR: 0, NDCG: 0},
		UnitEntry:   {Queries: 1, Precision: 0.0, Recall: 0, MRR: 0, NDCG: 0},
	}, queries)
	res, err = EvaluateBaseline(b, "t1", bad)
	if err == nil {
		t.Fatal("expected baseline error")
	}
	if res == nil || res.Pass || len(res.Violations) == 0 {
		t.Fatalf("violations must be attached: %+v", res)
	}
}
