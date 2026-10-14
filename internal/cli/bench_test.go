package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// benchTestdata returns the absolute path to the committed benchmark dataset
// or baseline file.
func benchTestdata(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	abs, err := filepath.Abs(thisFile)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(abs), "..", "benchmark", "testdata", name)
}

// benchReportShape is the subset of the report JSON the tests assert on.
type benchReportShape struct {
	Dataset struct {
		ID     string `json:"id"`
		SHA256 string `json:"sha256"`
	} `json:"dataset"`
	Config struct {
		TopK            int    `json:"top_k"`
		CandidatePolicy string `json:"candidate_policy"`
	} `json:"config"`
	Aggregate map[string]struct {
		Queries int     `json:"queries"`
		Recall  float64 `json:"recall"`
		NDCG    float64 `json:"ndcg"`
	} `json:"aggregate"`
	Baseline *struct {
		Pass       bool     `json:"pass"`
		Violations []string `json:"violations"`
	} `json:"baseline"`
}

func TestBenchRunReport(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env

	c := &Commands{}
	out := runCmdCapture(t, c, "kb", "bench", "run", "--dataset", benchTestdata(t, "lexical-v1.jsonl"))

	var rep benchReportShape
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, out)
	}
	if rep.Dataset.ID != "lexical-v1" {
		t.Fatalf("dataset id = %q", rep.Dataset.ID)
	}
	if rep.Config.TopK != 3 {
		t.Fatalf("top_k = %d, want default 3", rep.Config.TopK)
	}
	if rep.Config.CandidatePolicy != "unbounded" {
		t.Fatalf("candidate policy = %q", rep.Config.CandidatePolicy)
	}
	if rep.Aggregate["article"].Queries != 8 || rep.Aggregate["entry"].Queries != 2 {
		t.Fatalf("unexpected aggregate counts: %+v", rep.Aggregate)
	}
}

func TestBenchRunBaselinePass(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env

	c := &Commands{}
	out := runCmdCapture(t, c, "kb", "bench", "run",
		"--dataset", benchTestdata(t, "lexical-v1.jsonl"),
		"--baseline", benchTestdata(t, "baseline-v1.json"))

	var rep benchReportShape
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	if rep.Baseline == nil || !rep.Baseline.Pass {
		t.Fatalf("baseline should pass: %+v", rep.Baseline)
	}
}

// runCmdCaptureAllowError is runCmdCapture for commands expected to fail;
// it returns stdout and the run error.
func runCmdCaptureAllowError(t *testing.T, c *Commands, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := c.Run(context.Background(), args)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	return buf.String(), runErr
}

func TestBenchRunBaselineFailureExitsNonzero(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env

	// A baseline whose floor no run can meet: q-timeout precision@3 is 1/3.
	broken := strings.ReplaceAll(mustReadBaseline(t),
		`"q-timeout": { "recall": 1.0 }`,
		`"q-timeout": { "precision": 1.0 }`)
	brokenPath := filepath.Join(t.TempDir(), "baseline-broken.json")
	if err := os.WriteFile(brokenPath, []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}

	c := &Commands{}
	out, runErr := runCmdCaptureAllowError(t, c, "kb", "bench", "run",
		"--dataset", benchTestdata(t, "lexical-v1.jsonl"),
		"--baseline", brokenPath)

	if runErr == nil {
		t.Fatal("expected nonzero exit on baseline violation")
	}
	if !strings.Contains(runErr.Error(), "q-timeout") || !strings.Contains(runErr.Error(), "precision") {
		t.Fatalf("error must name query and violated metric: %v", runErr)
	}

	// The report is still emitted, with the failure attached.
	var rep benchReportShape
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, out)
	}
	if rep.Baseline == nil || rep.Baseline.Pass || len(rep.Baseline.Violations) == 0 {
		t.Fatalf("baseline failure must be attached to the report: %+v", rep.Baseline)
	}
}

func TestBenchRunMissingDatasetErrors(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env

	c := &Commands{}
	if _, runErr := runCmdCaptureAllowError(t, c, "kb", "bench", "run"); runErr == nil || !strings.Contains(runErr.Error(), "--dataset required") {
		t.Fatalf("missing --dataset must error: %v", runErr)
	}
	if _, runErr := runCmdCaptureAllowError(t, c, "kb", "bench", "run", "--dataset", "/nonexistent/dataset.jsonl"); runErr == nil {
		t.Fatal("missing dataset file must error")
	}
}

func mustReadBaseline(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(benchTestdata(t, "baseline-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
