package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound is returned when an entry or article is not found
var ErrNotFound = errors.New("not found")

type Entry struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Tags      string `json:"tags,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Article struct {
	ID        string `json:"id"`
	EntryID   string `json:"entry_id"`
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
	EntryID    string  `json:"entry_id"`
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
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			tags TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			entry_id TEXT NOT NULL,
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
			VALUES (new.rowid, new.title, new.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, content)
			VALUES ('delete', old.rowid, old.title, old.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, content)
			VALUES ('delete', old.rowid, old.title, old.content);
			INSERT INTO articles_fts(rowid, title, content)
			VALUES (new.rowid, new.title, new.content);
		END`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			article_id TEXT PRIMARY KEY,
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

func (d *DB) AddEntry(id, title, tags string) error {
	_, err := d.conn.Exec(
		"INSERT INTO entries (id, title, tags) VALUES (?, ?, ?)",
		id, title, tags,
	)
	return err
}

func (d *DB) GetEntry(id string) (*Entry, error) {
	return d.GetEntryWithDeleted(id, false)
}

func (d *DB) GetEntryWithDeleted(id string, includeDeleted bool) (*Entry, error) {
	var e Entry
	hasDeleted := d.hasDeletedColumn("entries")
	var query string

	if !hasDeleted || includeDeleted {
		query = "SELECT id, title, tags, created_at, updated_at FROM entries WHERE id = ?"
	} else {
		query = "SELECT id, title, tags, created_at, updated_at FROM entries WHERE id = ? AND deleted_at IS NULL"
	}

	err := d.conn.QueryRow(query, id).Scan(&e.ID, &e.Title, &e.Tags, &e.CreatedAt, &e.UpdatedAt)
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
	return d.ListEntriesWithDeleted(false)
}

// hasDeletedColumn checks if the deleted_at column exists in a table
func (d *DB) hasDeletedColumn(table string) bool {
	rows, err := d.conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "deleted_at" {
			return true
		}
	}
	return false
}

func (d *DB) ListEntriesWithDeleted(includeDeleted bool) ([]Entry, error) {
	hasDeleted := d.hasDeletedColumn("entries")

	var query string
	if !hasDeleted || includeDeleted {
		query = "SELECT id, title, tags, created_at, updated_at FROM entries ORDER BY updated_at DESC"
	} else {
		query = "SELECT id, title, tags, created_at, updated_at FROM entries WHERE deleted_at IS NULL ORDER BY updated_at DESC"
	}

	rows, err := d.conn.Query(query)
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

func (d *DB) UpdateEntryTime(id string) error {
	_, err := d.conn.Exec("UPDATE entries SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// Article operations

func (d *DB) AddArticle(id, entryID, title, content string) error {
	_, err := d.conn.Exec(
		"INSERT INTO articles (id, entry_id, title, content) VALUES (?, ?, ?, ?)",
		id, entryID, title, content,
	)
	if err != nil {
		return err
	}

	// Update entry timestamp
	return d.UpdateEntryTime(entryID)
}

func (d *DB) GetArticles(entryID string) ([]Article, error) {
	return d.GetArticlesWithDeleted(entryID, false)
}

func (d *DB) GetArticlesWithDeleted(entryID string, includeDeleted bool) ([]Article, error) {
	hasDeleted := d.hasDeletedColumn("articles")
	var query string
	if !hasDeleted || includeDeleted {
		query = "SELECT id, entry_id, title, content, created_at FROM articles WHERE entry_id = ? ORDER BY created_at"
	} else {
		query = "SELECT id, entry_id, title, content, created_at FROM articles WHERE entry_id = ? AND deleted_at IS NULL ORDER BY created_at"
	}

	rows, err := d.conn.Query(query, entryID)
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

func (d *DB) GetArticle(id string) (*Article, error) {
	return d.GetArticleWithDeleted(id, false)
}

func (d *DB) GetArticleWithDeleted(id string, includeDeleted bool) (*Article, error) {
	var a Article
	hasDeleted := d.hasDeletedColumn("articles")
	var query string

	if !hasDeleted || includeDeleted {
		query = "SELECT id, entry_id, title, content, created_at FROM articles WHERE id = ?"
	} else {
		query = "SELECT id, entry_id, title, content, created_at FROM articles WHERE id = ? AND deleted_at IS NULL"
	}

	err := d.conn.QueryRow(query, id).Scan(&a.ID, &a.EntryID, &a.Title, &a.Content, &a.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (d *DB) DeleteArticle(id string) error {
	// Get entry_id before delete
	var entryID string
	d.conn.QueryRow("SELECT entry_id FROM articles WHERE id = ?", id).Scan(&entryID)

	_, err := d.conn.Exec("DELETE FROM articles WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Update entry timestamp
	return d.UpdateEntryTime(entryID)
}

func (d *DB) DeleteEntry(id string) error {
	_, err := d.conn.Exec("DELETE FROM entries WHERE id = ?", id)
	return err
}

// Search operations

func (d *DB) Search(query string, topK int) ([]SearchResult, error) {
	return d.SearchWithDeleted(query, topK, false)
}

func (d *DB) SearchWithDeleted(query string, topK int, includeDeleted bool) ([]SearchResult, error) {
	hasDeleted := d.hasDeletedColumn("entries") && d.hasDeletedColumn("articles")

	var whereClause string
	if !hasDeleted || includeDeleted {
		whereClause = "articles_fts MATCH ?"
	} else {
		whereClause = "articles_fts MATCH ? AND a.deleted_at IS NULL AND e.deleted_at IS NULL"
	}

	rows, err := d.conn.Query(`
		SELECT a.id, a.entry_id, a.title, a.content, a.created_at,
			   e.title as entry_title,
			   bm25(articles_fts) as score
		FROM articles_fts
		JOIN articles a ON articles_fts.rowid = a.rowid
		JOIN entries e ON a.entry_id = e.id
		WHERE `+whereClause+`
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

type Stats struct {
	TotalEntries    int
	ActiveEntries   int
	DeletedEntries  int
	TotalArticles   int
	ActiveArticles  int
	DeletedArticles int
	TotalHistory    int
}

func (d *DB) Stats() (*Stats, error) {
	s := &Stats{}

	if err := d.conn.QueryRow("SELECT COUNT(*) FROM entries").Scan(&s.TotalEntries); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM articles").Scan(&s.TotalArticles); err != nil {
		return nil, err
	}

	// Check if deleted_at column exists
	hasDeletedAt := d.hasDeletedColumn("entries")

	if hasDeletedAt {
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM entries WHERE deleted_at IS NULL").Scan(&s.ActiveEntries); err != nil {
			return nil, err
		}
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM entries WHERE deleted_at IS NOT NULL").Scan(&s.DeletedEntries); err != nil {
			return nil, err
		}
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM articles WHERE deleted_at IS NULL").Scan(&s.ActiveArticles); err != nil {
			return nil, err
		}
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM articles WHERE deleted_at IS NOT NULL").Scan(&s.DeletedArticles); err != nil {
			return nil, err
		}
	} else {
		s.ActiveEntries = s.TotalEntries
		s.ActiveArticles = s.TotalArticles
	}

	// History table may not exist yet
	var historyCount int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM history").Scan(&historyCount)
	if err == nil {
		s.TotalHistory = historyCount
	}

	return s, nil
}
