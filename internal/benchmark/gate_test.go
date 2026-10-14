package benchmark

import (
	"bytes"
	"os"
	"testing"
)

// TestRetrievalBaseline is the committed quality gate: `go test ./...`
// always runs the committed dataset against the live ranking and fails when
// any baseline floor is violated. Floors are explicit reviewed values bound
// to the exact dataset bytes; edit them deliberately, never automatically.
func TestRetrievalBaseline(t *testing.T) {
	datasetRaw, err := os.ReadFile("testdata/lexical-v1.jsonl")
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}
	dataset, err := ParseDataset(bytes.NewReader(datasetRaw))
	if err != nil {
		t.Fatalf("parse dataset: %v", err)
	}

	baselineRaw, err := os.ReadFile("testdata/baseline-v1.json")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	baseline, err := LoadBaseline(bytes.NewReader(baselineRaw))
	if err != nil {
		t.Fatalf("parse baseline: %v", err)
	}

	c, err := baseline.Match(dataset.ID)
	if err != nil {
		t.Fatalf("match baseline case: %v", err)
	}

	report, err := Run(dataset, Config{TopK: c.TopK})
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}

	if err := c.Check(report); err != nil {
		t.Fatalf("retrieval baseline violated:\n%v", err)
	}
}
