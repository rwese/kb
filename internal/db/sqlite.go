package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      string    `json:"tags,omitempty"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type SearchResult struct {
	Entry
	Score float64 `json:"score"`
}

type DB struct {
	conn *sql.DB
}

func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			metadata TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
			title, content, tags,
			content='entries',
			content_rowid='id',
			tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
			INSERT INTO entries_fts(rowid, title, content, tags)
			VALUES (new.id, new.title, new.content, new.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, title, content, tags)
			VALUES ('delete', old.id, old.title, old.content, old.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, title, content, tags)
			VALUES ('delete', old.id, old.title, old.content, old.tags);
			INSERT INTO entries_fts(rowid, title, content, tags)
			VALUES (new.id, new.title, new.content, new.tags);
		END`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			entry_id INTEGER PRIMARY KEY,
			embedding BLOB,
			FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
		)`,
	}

	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) Add(title, content, tags string) (int64, error) {
	result, err := d.conn.Exec(
		"INSERT INTO entries (title, content, tags) VALUES (?, ?, ?)",
		title, content, tags,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (d *DB) AddBatch(entries []Entry) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO entries (title, content, tags) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.Title, e.Content, e.Tags); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) Search(query string, topK int) ([]SearchResult, error) {
	rows, err := d.conn.Query(`
		SELECT e.id, e.title, e.content, e.tags, e.created_at, e.updated_at,
			   bm25(entries_fts) as score
		FROM entries_fts
		JOIN entries e ON entries_fts.rowid = e.id
		WHERE entries_fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanResults(rows)
}

func (d *DB) SearchJSON(query string, topK int) ([]SearchResult, error) {
	results, err := d.Search(query, topK)
	if err != nil {
		return nil, err
	}

	// Output as JSON lines for piping
	for _, r := range results {
		data, _ := json.Marshal(r)
		println(string(data))
	}
	return results, nil
}

func (d *DB) Count() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count)
	return count, err
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func scanResults(rows *sql.Rows) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var tags sql.NullString
		var metadata sql.NullString
		if err := rows.Scan(&r.ID, &r.Title, &r.Content, &tags, &r.CreatedAt, &r.UpdatedAt, &r.Score); err != nil {
			return nil, err
		}
		if tags.Valid {
			r.Tags = tags.String
		}
		_ = metadata
		results = append(results, r)
	}
	return results, rows.Err()
}
