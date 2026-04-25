package db

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound is returned when an entry or article is not found
var ErrNotFound = errors.New("not found")

type Entry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Tags      string `json:"tags,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Article struct {
	ID        int64  `json:"id"`
	EntryID   int64  `json:"entry_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type EntryWithArticles struct {
	Entry
	Articles []Article `json:"articles"`
}

type SearchResult struct {
	Article
	EntryID    int64   `json:"entry_id"`
	EntryTitle string  `json:"entry_title"`
	Score      float64 `json:"score"`
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
			tags TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			title TEXT,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
			title, content,
			content='articles',
			content_rowid='id',
			tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
			INSERT INTO articles_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, content)
			VALUES ('delete', old.id, old.title, old.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, content)
			VALUES ('delete', old.id, old.title, old.content);
			INSERT INTO articles_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			article_id INTEGER PRIMARY KEY,
			embedding BLOB,
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_entry ON articles(entry_id)`,
	}

	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

// Entry operations

func (d *DB) AddEntry(title, tags string) (int64, error) {
	result, err := d.conn.Exec(
		"INSERT INTO entries (title, tags) VALUES (?, ?)",
		title, tags,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (d *DB) GetEntry(id int64) (*Entry, error) {
	var e Entry
	err := d.conn.QueryRow(
		"SELECT id, title, tags, created_at, updated_at FROM entries WHERE id = ?",
		id,
	).Scan(&e.ID, &e.Title, &e.Tags, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (d *DB) GetEntryByTitle(title string) (*Entry, error) {
	var e Entry
	err := d.conn.QueryRow(
		"SELECT id, title, tags, created_at, updated_at FROM entries WHERE title = ?",
		title,
	).Scan(&e.ID, &e.Title, &e.Tags, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (d *DB) ListEntries() ([]Entry, error) {
	rows, err := d.conn.Query(
		"SELECT id, title, tags, created_at, updated_at FROM entries ORDER BY updated_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Title, &e.Tags, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) UpdateEntryTime(id int64) error {
	_, err := d.conn.Exec("UPDATE entries SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// Article operations

func (d *DB) AddArticle(entryID int64, title, content string) (int64, error) {
	result, err := d.conn.Exec(
		"INSERT INTO articles (entry_id, title, content) VALUES (?, ?, ?)",
		entryID, title, content,
	)
	if err != nil {
		return 0, err
	}

	// Update entry timestamp
	d.UpdateEntryTime(entryID)

	return result.LastInsertId()
}

func (d *DB) GetArticles(entryID int64) ([]Article, error) {
	rows, err := d.conn.Query(
		"SELECT id, entry_id, title, content, created_at FROM articles WHERE entry_id = ? ORDER BY created_at",
		entryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.EntryID, &a.Title, &a.Content, &a.CreatedAt); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

func (d *DB) GetArticle(id int64) (*Article, error) {
	var a Article
	err := d.conn.QueryRow(
		"SELECT id, entry_id, title, content, created_at FROM articles WHERE id = ?",
		id,
	).Scan(&a.ID, &a.EntryID, &a.Title, &a.Content, &a.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (d *DB) DeleteArticle(id int64) error {
	// Get entry_id before delete
	var entryID int64
	d.conn.QueryRow("SELECT entry_id FROM articles WHERE id = ?", id).Scan(&entryID)

	_, err := d.conn.Exec("DELETE FROM articles WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Update entry timestamp
	return d.UpdateEntryTime(entryID)
}

func (d *DB) DeleteEntry(id int64) error {
	_, err := d.conn.Exec("DELETE FROM entries WHERE id = ?", id)
	return err
}

// Search operations

func (d *DB) Search(query string, topK int) ([]SearchResult, error) {
	rows, err := d.conn.Query(`
		SELECT a.id, a.entry_id, a.title, a.content, a.created_at,
			   e.title as entry_title,
			   bm25(articles_fts) as score
		FROM articles_fts
		JOIN articles a ON articles_fts.rowid = a.id
		JOIN entries e ON a.entry_id = e.id
		WHERE articles_fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var title sql.NullString
		if err := rows.Scan(&r.ID, &r.EntryID, &title, &r.Content, &r.CreatedAt, &r.EntryTitle, &r.Score); err != nil {
			return nil, err
		}
		if title.Valid {
			r.Title = title.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (d *DB) Count() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count)
	return count, err
}

func (d *DB) ArticleCount() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
	return count, err
}

func (d *DB) Close() error {
	return d.conn.Close()
}
