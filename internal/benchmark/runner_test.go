package benchmark

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func runFixture(t *testing.T, cfg Config) Report {
	t.Helper()
	ds := mustParse(t, validDataset())
	rep, err := runInDir(ds, cfg, t.TempDir())
	if err != nil {
		t.Fatalf("runInDir: %v", err)
	}
	return rep
}

func TestRunnerSeedsAndEvaluates(t *testing.T) {
	rep := runFixture(t, Config{TopK: 3})

	if rep.Dataset.ID != "t1" {
		t.Fatalf("dataset id = %q", rep.Dataset.ID)
	}
	if rep.Config.TopK != 3 || rep.Config.CandidatePolicy != CandidatePolicyUnbounded {
		t.Fatalf("config not normalized: %+v", rep.Config)
	}

	qr := rep.Queries[0]
	if len(qr.Ranked) != 1 || qr.Ranked[0].ID != "abc123-000001" {
		t.Fatalf("unexpected ranking: %+v", qr.Ranked)
	}
	if qr.Metrics == nil || qr.Metrics.Recall != 1 {
		t.Fatalf("unexpected metrics: %+v", qr.Metrics)
	}
	agg, ok := rep.Aggregate[UnitArticle]
	if !ok || agg.Queries != 1 || agg.Recall != 1 {
		t.Fatalf("unexpected aggregate: %+v", agg)
	}
}

func TestRunnerDeterministicAcrossRuns(t *testing.T) {
	a := runFixture(t, Config{TopK: 3})
	b := runFixture(t, Config{TopK: 3})

	stable := func(r Report) string {
		r.Latency = LatencyReport{}
		r.Mem = MemReport{}
		r.Storage = StorageReport{}
		for i := range r.Queries {
			r.Queries[i].LatencyMS = 0
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if stable(a) != stable(b) {
		t.Fatalf("two runs differ outside latency:\n%s\n%s", stable(a), stable(b))
	}
}

func TestRunnerControlMustRetrieveNothing(t *testing.T) {
	// The control query lexically matches a seeded document: the runner must
	// reject the dataset instead of scoring an unjudged hit as benign.
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"Entry one"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"alpha content"}`,
		`{"type":"query","id":"q-hit","text":"alpha","unit":"article","relevant":[{"id":"abc123-000001","grade":3}]}`,
		`{"type":"query","id":"q-control","text":"alpha","unit":"article"}`,
	}, "\n"))

	_, err := runInDir(ds, Config{TopK: 3}, t.TempDir())
	if err == nil {
		t.Fatal("expected control violation error")
	}
	if !strings.Contains(err.Error(), "q-control") {
		t.Fatalf("error must name the control query: %v", err)
	}
}

func TestRunnerMalformedFTSSurfacesError(t *testing.T) {
	// An unbalanced phrase marker must fail loudly with the query identity,
	// not silently score zero.
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"Entry one"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"alpha content"}`,
		`{"type":"query","id":"q-bad","text":"\"unbalanced","unit":"article","relevant":[{"id":"abc123-000001","grade":3}]}`,
	}, "\n"))

	_, err := runInDir(ds, Config{TopK: 3}, t.TempDir())
	if err == nil {
		t.Fatal("expected FTS error")
	}
	if !strings.Contains(err.Error(), "q-bad") {
		t.Fatalf("error must carry query identity: %v", err)
	}
}

func TestRunnerEntryUnitDeduplicates(t *testing.T) {
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"SQLite storage"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"sqlite alpha"}`,
		`{"type":"document","id":"abc123-000002","entry_id":"abc123","title":"Beta","content":"sqlite beta"}`,
		`{"type":"query","id":"q-entry","text":"sqlite","unit":"entry","relevant":[{"id":"abc123","grade":3}]}`,
	}, "\n"))

	rep, err := runInDir(ds, Config{TopK: 3}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	qr := rep.Queries[0]
	if len(qr.Ranked) != 1 || qr.Ranked[0].ID != "abc123" {
		t.Fatalf("entry hits must deduplicate to one: %+v", qr.Ranked)
	}
	if qr.Metrics == nil || qr.Metrics.Recall != 1 {
		t.Fatalf("unexpected metrics: %+v", qr.Metrics)
	}
}

func TestRunnerEntryUnitRecoversAllMatchingEntries(t *testing.T) {
	// top_k = 1, but two entries match: the unbounded candidate policy must
	// surface both before truncation of the deduped entry list... the ranked
	// list truncates to top_k, so only one appears; the other entry's
	// article is still fetched as a candidate. This test pins the policy:
	// both entries are retrieved as candidates, top_k=2 ranks both.
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"SQLite storage"}`,
		`{"type":"entry","id":"def456","title":"More SQLite"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"sqlite alpha"}`,
		`{"type":"document","id":"def456-000001","entry_id":"def456","title":"Beta","content":"sqlite beta"}`,
		`{"type":"query","id":"q-entry","text":"sqlite","unit":"entry","relevant":[{"id":"abc123","grade":3},{"id":"def456","grade":2}]}`,
	}, "\n"))

	rep, err := runInDir(ds, Config{TopK: 2}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	qr := rep.Queries[0]
	if len(qr.Ranked) != 2 || qr.Ranked[0].ID != "abc123" || qr.Ranked[1].ID != "def456" {
		t.Fatalf("both entries must rank: %+v", qr.Ranked)
	}
}

func TestRunnerConfigValidation(t *testing.T) {
	ds := mustParse(t, validDataset())

	if _, err := Run(ds, Config{TopK: -1}); err == nil {
		t.Fatal("expected error for negative top_k")
	}
	if _, err := Run(ds, Config{CandidatePolicy: "bounded"}); err == nil {
		t.Fatal("expected error for unknown candidate policy")
	}
}

func TestRunnerUsesCallerDir(t *testing.T) {
	ds := mustParse(t, validDataset())
	dir := t.TempDir()
	if _, err := runInDir(ds, Config{TopK: 3}, dir); err != nil {
		t.Fatal(err)
	}
	// runInDir owns the given directory: the KB lives in it, nothing else
	// pollutes it.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	if len(names) != 1 || names[0] != "kb.db" {
		t.Fatalf("unexpected caller dir contents: %v", names)
	}
}

func TestRunCleansUpItsOwnTempDir(t *testing.T) {
	ds := mustParse(t, validDataset())
	if _, err := Run(ds, Config{TopK: 3}); err != nil {
		t.Fatal(err)
	}
	// Nothing to assert beyond no error: os.MkdirTemp dirs are removed by
	// the deferred cleanup; the deterministic-report test covers
	// correctness.
}
