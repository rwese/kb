package benchmark

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, input string) Dataset {
	t.Helper()
	ds, err := ParseDataset(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseDataset: %v", err)
	}
	return ds
}

func validDataset() string {
	return strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"Entry one"}`,
		`{"type":"document","id":"abc123-000001","entry_id":"abc123","title":"Alpha","content":"alpha content"}`,
		`{"type":"query","id":"q1","text":"alpha","unit":"article","relevant":[{"id":"abc123-000001","grade":3}]}`,
	}, "\n")
}

func TestParseDatasetValid(t *testing.T) {
	ds := mustParse(t, validDataset())

	if ds.ID != "t1" || ds.Version != 1 {
		t.Fatalf("unexpected identity: %s v%d", ds.ID, ds.Version)
	}
	if len(ds.Entries) != 1 || len(ds.Documents) != 1 || len(ds.Queries) != 1 {
		t.Fatalf("unexpected record counts: %d/%d/%d", len(ds.Entries), len(ds.Documents), len(ds.Queries))
	}
	if len(ds.SHA256) != 64 {
		t.Fatalf("sha256 missing: %q", ds.SHA256)
	}
	// Same bytes -> same hash.
	again := mustParse(t, validDataset())
	if again.SHA256 != ds.SHA256 {
		t.Fatalf("sha256 not stable over identical bytes")
	}
}

func TestParseDatasetControlQueries(t *testing.T) {
	ds := mustParse(t, strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"Entry one"}`,
		`{"type":"query","id":"q-none","text":"nothing","unit":"entry"}`,
		`{"type":"query","id":"q-empty","text":"nothing","unit":"entry","relevant":[]}`,
	}, "\n"))
	if len(ds.Queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(ds.Queries))
	}
	for _, q := range ds.Queries {
		if !q.IsControl() {
			t.Fatalf("query %s should be a control", q.ID)
		}
	}
}

func TestParseDatasetErrors(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		err    string
		noLine bool
	}{
		{
			name:  "missing metadata",
			input: `{"type":"entry","id":"abc123","title":"E"}`,
			err:   "before dataset metadata",
		},
		{
			name: "unsupported version",
			input: strings.Join([]string{
				`{"type":"dataset","version":2,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry"}`,
			}, "\n"),
			err: "unsupported dataset version",
		},
		{
			name: "duplicate metadata",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"dataset","version":1,"id":"t1"}`,
			}, "\n"),
			err: "duplicate dataset metadata",
		},
		{
			name: "missing dataset id",
			input: strings.Join([]string{
				`{"type":"dataset","version":1}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry"}`,
			}, "\n"),
			err: "dataset id required",
		},
		{
			name: "no queries",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
			}, "\n"),
			err:    "no queries",
			noLine: true,
		},
		{
			name: "bad entry id",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"ABC123","title":"E"}`,
			}, "\n"),
			err: "6 lowercase hex",
		},
		{
			name: "duplicate entry id",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"entry","id":"abc123","title":"E2"}`,
			}, "\n"),
			err: "duplicate entry id",
		},
		{
			name: "document prefix mismatch",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"document","id":"def456-000001","entry_id":"abc123","content":"c"}`,
			}, "\n"),
			err: "prefix does not match",
		},
		{
			name: "document unknown entry",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"document","id":"abc123-000001","entry_id":"abc123","content":"c"}`,
			}, "\n"),
			err: "references unknown entry",
		},
		{
			name: "document bad id shape",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"document","id":"abc123","entry_id":"abc123","content":"c"}`,
			}, "\n"),
			err: "<entry-id>-<6 hex>",
		},
		{
			name: "empty content and title",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"document","id":"abc123-000001","entry_id":"abc123"}`,
			}, "\n"),
			err: "needs title or content",
		},
		{
			name: "bad unit",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"attachment"}`,
			}, "\n"),
			err: `unit must be "article" or "entry"`,
		},
		{
			name: "blank query text",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"   ","unit":"entry"}`,
			}, "\n"),
			err: "text required",
		},
		{
			name: "grade out of range",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry","relevant":[{"id":"abc123","grade":4}]}`,
			}, "\n"),
			err: "grade must be 0..3",
		},
		{
			name: "duplicate judgment",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry","relevant":[{"id":"abc123","grade":1},{"id":"abc123","grade":2}]}`,
			}, "\n"),
			err: "duplicate judgment",
		},
		{
			name: "judgment unknown document",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"query","id":"q1","text":"x","unit":"article","relevant":[{"id":"abc123","grade":1}]}`,
			}, "\n"),
			err: "unknown document",
		},
		{
			name: "judgment unknown entry",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry","relevant":[{"id":"abc123","grade":1}]}`,
			}, "\n"),
			err: "unknown entry",
		},
		{
			name: "all zero grades without control semantics",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"entry","id":"abc123","title":"E"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry","relevant":[{"id":"abc123","grade":0}]}`,
			}, "\n"),
			err: "at least one positive",
		},
		{
			name: "unknown record type",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"asset","id":"x"}`,
			}, "\n"),
			err: `unknown record type "asset"`,
		},
		{
			name: "unknown field",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry","weight":2}`,
			}, "\n"),
			err: "unknown field",
		},
		{
			name: "trailing json value",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry"} {}`,
			}, "\n"),
			err: "after top-level value",
		},
		{
			name: "duplicate query id",
			input: strings.Join([]string{
				`{"type":"dataset","version":1,"id":"t1"}`,
				`{"type":"query","id":"q1","text":"x","unit":"entry"}`,
				`{"type":"query","id":"q1","text":"y","unit":"entry"}`,
			}, "\n"),
			err: "duplicate query id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDataset(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.err)
			}
			if !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.err)
			}
			if !tt.noLine && !strings.HasPrefix(err.Error(), "line ") {
				t.Fatalf("error must carry a line number: %q", err.Error())
			}
		})
	}
}

func TestParseDatasetLineNumbers(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"dataset","version":1,"id":"t1"}`,
		`{"type":"entry","id":"abc123","title":"E"}`,
		`{"type":"entry","id":"abc123","title":"E2"}`,
	}, "\n")
	_, err := ParseDataset(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("expected line 3 in error, got: %v", err)
	}
}
