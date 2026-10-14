// Package benchmark provides a retrieval benchmark harness: JSONL datasets of
// entries, documents, and judged queries are imported into an isolated KB,
// executed against the live ranking, and scored with relevance metrics.
//
// The harness never touches a real knowledgebase: it seeds its own temporary
// database from the dataset, so benchmarks are reproducible and hermetic.
package benchmark

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	// DatasetVersion is the only supported dataset schema version.
	DatasetVersion = 1
	// MaxLineBytes caps a single JSONL line so malformed input fails fast
	// instead of exhausting memory.
	MaxLineBytes = 8 << 20

	// UnitArticle evaluates ranking of individual article IDs.
	UnitArticle = "article"
	// UnitEntry evaluates ranking of entry IDs; article hits are mapped to
	// their owning entry and deduplicated.
	UnitEntry = "entry"
)

var (
	entryIDPattern   = regexp.MustCompile(`^[0-9a-f]{6}$`)
	articleIDPattern = regexp.MustCompile(`^[0-9a-f]{6}-[0-9a-f]{6}$`)
)

// Entry is a benchmark entry record mirroring the KB entry model.
type Entry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Tags  string `json:"tags,omitempty"`
}

// Document is a benchmark article record; ID follows the KB article id
// convention <entry-id>-<6 hex>.
type Document struct {
	ID      string `json:"id"`
	EntryID string `json:"entry_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Judgment declares the expected relevance grade (0..3) of one result ID for
// a query. Grade 0 means explicitly judged non-relevant.
type Judgment struct {
	ID    string `json:"id"`
	Grade int    `json:"grade"`
}

// Query is one benchmark query. A query with no judgments is an explicit
// no-match control: it must retrieve zero hits and is excluded from
// relevance averages.
type Query struct {
	ID       string     `json:"id"`
	Text     string     `json:"text"`
	Unit     string     `json:"unit"`
	Relevant []Judgment `json:"relevant,omitempty"`
}

// IsControl reports whether the query is a no-match control.
func (q Query) IsControl() bool { return len(q.Relevant) == 0 }

// Dataset is a parsed and validated benchmark dataset.
type Dataset struct {
	Version   int
	ID        string
	SHA256    string
	Entries   []Entry
	Documents []Document
	Queries   []Query
}

// Dataset SHA input is the exact original bytes.

type envelope struct {
	Type string `json:"type"`
}

type datasetRecord struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	ID      string `json:"id"`
}

// Record shadows wrap the public types with the envelope's type tag.
type entryRecord struct {
	Type string `json:"type"`
	Entry
}

type documentRecord struct {
	Type string `json:"type"`
	Document
}

type queryRecord struct {
	Type string `json:"type"`
	Query
}

// ParseDataset reads a JSONL dataset: one metadata record
// {"type":"dataset",...} followed by entry, document, and query records.
// The whole file is parsed and validated before anything is returned; the
// returned Dataset carries the SHA-256 of the exact input bytes. Blank lines
// are ignored.
func ParseDataset(r io.Reader) (Dataset, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset: %w", err)
	}

	ds := Dataset{
		Version: DatasetVersion,
		SHA256:  hashBytes(raw),
	}

	scanner := newLineScanner(bytes.NewReader(raw))
	lineNo := 0
	seenMeta := false

	seenEntry := map[string]bool{}
	seenDoc := map[string]bool{}
	seenQuery := map[string]bool{}

	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var env envelope
		if err := json.Unmarshal(line, &env); err != nil {
			return Dataset{}, fmt.Errorf("line %d: unknown record shape: %w", lineNo, err)
		}
		switch env.Type {
		case "dataset":
			if seenMeta {
				return Dataset{}, fmt.Errorf("line %d: duplicate dataset metadata record", lineNo)
			}
			var rec datasetRecord
			if err := decodeStrict(line, &rec); err != nil {
				return Dataset{}, fmt.Errorf("line %d: dataset record: %w", lineNo, err)
			}
			if rec.Version != DatasetVersion {
				return Dataset{}, fmt.Errorf("line %d: unsupported dataset version %d (want %d)", lineNo, rec.Version, DatasetVersion)
			}
			if rec.ID == "" {
				return Dataset{}, fmt.Errorf("line %d: dataset id required", lineNo)
			}
			ds.Version = rec.Version
			ds.ID = rec.ID
			seenMeta = true

		case "entry":
			if !seenMeta {
				return Dataset{}, fmt.Errorf("line %d: entry record before dataset metadata", lineNo)
			}
			var rec entryRecord
			if err := decodeStrict(line, &rec); err != nil {
				return Dataset{}, fmt.Errorf("line %d: entry record: %w", lineNo, err)
			}
			e := rec.Entry
			if !entryIDPattern.MatchString(e.ID) {
				return Dataset{}, fmt.Errorf("line %d: entry id %q must be 6 lowercase hex chars", lineNo, e.ID)
			}
			if seenEntry[e.ID] {
				return Dataset{}, fmt.Errorf("line %d: duplicate entry id %s", lineNo, e.ID)
			}
			if strings.TrimSpace(e.Title) == "" {
				return Dataset{}, fmt.Errorf("line %d: entry %s title required", lineNo, e.ID)
			}
			seenEntry[e.ID] = true
			ds.Entries = append(ds.Entries, e)

		case "document":
			if !seenMeta {
				return Dataset{}, fmt.Errorf("line %d: document record before dataset metadata", lineNo)
			}
			var rec documentRecord
			if err := decodeStrict(line, &rec); err != nil {
				return Dataset{}, fmt.Errorf("line %d: document record: %w", lineNo, err)
			}
			d := rec.Document
			if !articleIDPattern.MatchString(d.ID) {
				return Dataset{}, fmt.Errorf("line %d: document id %q must match <entry-id>-<6 hex>", lineNo, d.ID)
			}
			if seenDoc[d.ID] {
				return Dataset{}, fmt.Errorf("line %d: duplicate document id %s", lineNo, d.ID)
			}
			if d.EntryID == "" || !entryIDPattern.MatchString(d.EntryID) {
				return Dataset{}, fmt.Errorf("line %d: document %s entry_id must be 6 lowercase hex chars", lineNo, d.ID)
			}
			if prefix := strings.SplitN(d.ID, "-", 2)[0]; prefix != d.EntryID {
				return Dataset{}, fmt.Errorf("line %d: document %s id prefix does not match entry_id %s", lineNo, d.ID, d.EntryID)
			}
			if !seenEntry[d.EntryID] {
				return Dataset{}, fmt.Errorf("line %d: document %s references unknown entry %s", lineNo, d.ID, d.EntryID)
			}
			if strings.TrimSpace(d.Content) == "" && strings.TrimSpace(d.Title) == "" {
				return Dataset{}, fmt.Errorf("line %d: document %s needs title or content", lineNo, d.ID)
			}
			seenDoc[d.ID] = true
			ds.Documents = append(ds.Documents, d)

		case "query":
			if !seenMeta {
				return Dataset{}, fmt.Errorf("line %d: query record before dataset metadata", lineNo)
			}
			var rec queryRecord
			if err := decodeStrict(line, &rec); err != nil {
				return Dataset{}, fmt.Errorf("line %d: query record: %w", lineNo, err)
			}
			q := rec.Query
			if q.ID == "" {
				return Dataset{}, fmt.Errorf("line %d: query id required", lineNo)
			}
			if seenQuery[q.ID] {
				return Dataset{}, fmt.Errorf("line %d: duplicate query id %s", lineNo, q.ID)
			}
			if strings.TrimSpace(q.Text) == "" {
				return Dataset{}, fmt.Errorf("line %d: query %s text required", lineNo, q.ID)
			}
			if q.Unit != UnitArticle && q.Unit != UnitEntry {
				return Dataset{}, fmt.Errorf("line %d: query %s unit must be %q or %q", lineNo, q.ID, UnitArticle, UnitEntry)
			}
			seenJudgment := map[string]bool{}
			positives := 0
			for _, j := range q.Relevant {
				if j.ID == "" {
					return Dataset{}, fmt.Errorf("line %d: query %s judgment id required", lineNo, q.ID)
				}
				if seenJudgment[j.ID] {
					return Dataset{}, fmt.Errorf("line %d: query %s duplicate judgment for %s", lineNo, q.ID, j.ID)
				}
				seenJudgment[j.ID] = true
				if j.Grade < 0 || j.Grade > 3 {
					return Dataset{}, fmt.Errorf("line %d: query %s judgment %s grade must be 0..3", lineNo, q.ID, j.ID)
				}
				switch q.Unit {
				case UnitArticle:
					if !seenDoc[j.ID] {
						return Dataset{}, fmt.Errorf("line %d: query %s judgment references unknown document %s", lineNo, q.ID, j.ID)
					}
				case UnitEntry:
					if !seenEntry[j.ID] {
						return Dataset{}, fmt.Errorf("line %d: query %s judgment references unknown entry %s", lineNo, q.ID, j.ID)
					}
				}
				if j.Grade > 0 {
					positives++
				}
			}
			if !q.IsControl() && positives == 0 {
				return Dataset{}, fmt.Errorf("line %d: query %s needs at least one positive (grade > 0) judgment, or no judgments at all to be a control", lineNo, q.ID)
			}
			seenQuery[q.ID] = true
			ds.Queries = append(ds.Queries, q)

		default:
			return Dataset{}, fmt.Errorf("line %d: unknown record type %q", lineNo, env.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return Dataset{}, err
	}

	if !seenMeta {
		return Dataset{}, fmt.Errorf("missing dataset metadata record")
	}
	if len(ds.Queries) == 0 {
		return Dataset{}, fmt.Errorf("dataset has no queries")
	}
	return ds, nil
}

// newLineScanner returns a scanner with the per-line cap applied.
func newLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)
	return sc
}

// decodeStrict decodes exactly one JSON value with unknown-field rejection
// and trailing-data detection.
func decodeStrict(line []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("trailing data after JSON value")
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing data after JSON value")
	}
	return nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
