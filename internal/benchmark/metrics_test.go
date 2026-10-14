package benchmark

import (
	"reflect"
	"testing"
)

func judged(ids ...int) []Judgment {
	out := make([]Judgment, 0, len(ids))
	for i, g := range ids {
		out = append(out, Judgment{ID: string(rune('a' + i)), Grade: g})
	}
	return out
}

func TestEvaluatePerfect(t *testing.T) {
	// a(3), b(1): perfect ranking.
	m, err := Evaluate([]string{"a", "b", "zz"}, judged(3, 1), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := Metrics{Precision: 2.0 / 3.0, Recall: 1, MRR: 1, NDCG: 1}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got %+v, want %+v", m, want)
	}
}

func TestEvaluateMRRSecondPosition(t *testing.T) {
	// Only b relevant, ranked second.
	m, err := Evaluate([]string{"x", "b"}, judged(0, 3), 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.MRR != 0.5 {
		t.Fatalf("mrr = %v, want 0.5", m.MRR)
	}
	if m.Recall != 1 {
		t.Fatalf("recall = %v, want 1", m.Recall)
	}
	if m.Precision != 1.0/3.0 {
		t.Fatalf("precision = %v, want 1/3", m.Precision)
	}
}

func TestEvaluateMiss(t *testing.T) {
	// Relevant doc not retrieved at all.
	m, err := Evaluate([]string{"x", "y", "z"}, judged(3), 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.Recall != 0 || m.MRR != 0 || m.NDCG != 0 || m.Precision != 0 {
		t.Fatalf("all metrics should be zero, got %+v", m)
	}
}

func TestEvaluateNDCGDiscrimination(t *testing.T) {
	// a(3), b(1) ranked worst-case: b first, a second.
	m, err := Evaluate([]string{"b", "a"}, judged(3, 1), 2)
	if err != nil {
		t.Fatal(err)
	}
	// DCG = (2^1-1)/log2(2) + (2^3-1)/log2(3) = 1 + 7/1.58496
	dcg := 1 + 7/1.5849625007211562
	idcg := 7/1.0 + 1/1.5849625007211562
	if diff := m.NDCG - dcg/idcg; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("ndcg = %v, want %v", m.NDCG, dcg/idcg)
	}
}

func TestEvaluateIdealTruncatedAtK(t *testing.T) {
	// Judgments beyond k do not inflate the ideal DCG: k=1, only a(3) at rank 1.
	m, err := Evaluate([]string{"a"}, judged(3, 2, 1), 1)
	if err != nil {
		t.Fatal(err)
	}
	if m.NDCG != 1 {
		t.Fatalf("ndcg = %v, want 1", m.NDCG)
	}
	if m.Recall != 1.0/3.0 {
		t.Fatalf("recall = %v, want 1/3", m.Recall)
	}
}

func TestEvaluateDeduplicatesResults(t *testing.T) {
	m, err := Evaluate([]string{"a", "a", "b"}, judged(1, 1), 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.Recall != 1 {
		t.Fatalf("recall = %v, want 1", m.Recall)
	}
	// 2 deduped positive hits of k=3.
	if m.Precision != 2.0/3.0 {
		t.Fatalf("precision = %v, want 2/3", m.Precision)
	}
}

func TestEvaluateShortResultList(t *testing.T) {
	// Empty results yield zeros, no error.
	m, err := Evaluate(nil, judged(3), 3)
	if err != nil {
		t.Fatal(err)
	}
	if m != (Metrics{}) {
		t.Fatalf("want zero metrics, got %+v", m)
	}
}

func TestEvaluateNoJudgments(t *testing.T) {
	// Controls are excluded upstream, but Evaluate must not divide by zero.
	m, err := Evaluate([]string{"a"}, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if m != (Metrics{}) {
		t.Fatalf("want zero metrics, got %+v", m)
	}
}

func TestEvaluateInvalidK(t *testing.T) {
	if _, err := Evaluate([]string{"a"}, judged(1), 0); err == nil {
		t.Fatal("expected error for k=0")
	}
	if _, err := Evaluate([]string{"a"}, judged(1), -1); err == nil {
		t.Fatal("expected error for k=-1")
	}
}
