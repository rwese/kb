package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

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

type ArticleAsset struct {
	ID           string `json:"id"`
	ArticleID    string `json:"article_id"`
	LogicalPath  string `json:"logical_path"`
	OriginalPath string `json:"original_path"`
	StoreRelPath string `json:"store_rel_path"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"size_bytes"`
	CreatedAt    string `json:"created_at"`
}

type ArticleWithAssets struct {
	Article
	Assets []ArticleAsset `json:"assets"`
}

type EntryWithArticles struct {
	Entry
	Articles []Article `json:"articles"`
}

type SearchResult struct {
	Article
	EntryID       string  `json:"entry_id"`
	EntryTitle    string  `json:"entry_title"`
	Score         float64 `json:"score"`
	BM25Score     float64 `json:"bm25_score,omitempty"`
	SemanticScore float64 `json:"semantic_score,omitempty"`
}

// Vector stores embedding vectors for articles
type Vector struct {
	ID        int64
	ArticleID string
	Embedding []byte // Stored as blob
	Model     string
	CreatedAt string
}

type DB struct {
	conn *sql.DB
}

func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
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

		// Vector storage table (Phase 1: SQLite blob storage)
		// Later can migrate to sqlite-vec for HNSW indexing
		`CREATE TABLE IF NOT EXISTS vectors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id TEXT NOT NULL UNIQUE,
			embedding BLOB NOT NULL,
			model TEXT NOT NULL DEFAULT 'all-MiniLM-L6-v2-Q4_K_M',
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vectors_article ON vectors(article_id)`,
		`CREATE TABLE IF NOT EXISTS article_assets (
			id TEXT PRIMARY KEY,
			article_id TEXT NOT NULL,
			logical_path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			store_rel_path TEXT NOT NULL UNIQUE,
			sha256 TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
			UNIQUE(article_id, logical_path)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_article_assets_article ON article_assets(article_id)`,
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

func (d *DB) UpdateEntry(id, title, tags string) error {
	_, err := d.conn.Exec(
		"UPDATE entries SET title = ?, tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		title, tags, id,
	)
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

func (d *DB) UpdateArticle(id, title, content string) error {
	_, err := d.conn.Exec(
		"UPDATE articles SET title = ?, content = ? WHERE id = ?",
		title, content, id,
	)
	if err != nil {
		return err
	}

	// Update entry timestamp
	var entryID string
	d.conn.QueryRow("SELECT entry_id FROM articles WHERE id = ?", id).Scan(&entryID)
	return d.UpdateEntryTime(entryID)
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

// Article asset operations

func (d *DB) AddArticleAsset(asset ArticleAsset) error {
	_, err := d.conn.Exec(`
		INSERT INTO article_assets (
			id, article_id, logical_path, original_path, store_rel_path, sha256, size_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, asset.ID, asset.ArticleID, asset.LogicalPath, asset.OriginalPath, asset.StoreRelPath, asset.SHA256, asset.SizeBytes)
	return err
}

func (d *DB) SaveArticleAssets(entryID string, assets []ArticleAsset, overwriteIDs []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, assetID := range overwriteIDs {
		if _, err := tx.Exec("DELETE FROM article_assets WHERE id = ?", assetID); err != nil {
			return err
		}
	}

	for _, asset := range assets {
		if _, err := tx.Exec(`
			INSERT INTO article_assets (
				id, article_id, logical_path, original_path, store_rel_path, sha256, size_bytes
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, asset.ID, asset.ArticleID, asset.LogicalPath, asset.OriginalPath, asset.StoreRelPath, asset.SHA256, asset.SizeBytes); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("UPDATE entries SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", entryID); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) GetArticleAsset(articleID, assetID string) (*ArticleAsset, error) {
	var asset ArticleAsset
	err := d.conn.QueryRow(`
		SELECT id, article_id, logical_path, original_path, store_rel_path, sha256, size_bytes, created_at
		FROM article_assets
		WHERE article_id = ? AND id = ?
	`, articleID, assetID).Scan(
		&asset.ID,
		&asset.ArticleID,
		&asset.LogicalPath,
		&asset.OriginalPath,
		&asset.StoreRelPath,
		&asset.SHA256,
		&asset.SizeBytes,
		&asset.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (d *DB) GetArticleAssetByLogicalPath(articleID, logicalPath string) (*ArticleAsset, error) {
	var asset ArticleAsset
	err := d.conn.QueryRow(`
		SELECT id, article_id, logical_path, original_path, store_rel_path, sha256, size_bytes, created_at
		FROM article_assets
		WHERE article_id = ? AND logical_path = ?
	`, articleID, logicalPath).Scan(
		&asset.ID,
		&asset.ArticleID,
		&asset.LogicalPath,
		&asset.OriginalPath,
		&asset.StoreRelPath,
		&asset.SHA256,
		&asset.SizeBytes,
		&asset.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (d *DB) ListArticleAssets(articleID string) ([]ArticleAsset, error) {
	rows, err := d.conn.Query(`
		SELECT id, article_id, logical_path, original_path, store_rel_path, sha256, size_bytes, created_at
		FROM article_assets
		WHERE article_id = ?
		ORDER BY logical_path
	`, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []ArticleAsset
	for rows.Next() {
		var asset ArticleAsset
		if err := rows.Scan(
			&asset.ID,
			&asset.ArticleID,
			&asset.LogicalPath,
			&asset.OriginalPath,
			&asset.StoreRelPath,
			&asset.SHA256,
			&asset.SizeBytes,
			&asset.CreatedAt,
		); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (d *DB) DeleteArticleAsset(articleID, assetID string) error {
	_, err := d.conn.Exec("DELETE FROM article_assets WHERE article_id = ? AND id = ?", articleID, assetID)
	return err
}

func (d *DB) DeleteArticleAssetsByArticle(articleID string) error {
	_, err := d.conn.Exec("DELETE FROM article_assets WHERE article_id = ?", articleID)
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

func (d *DB) AssetCount() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM article_assets").Scan(&count)
	return count, err
}

// Vector operations

// SaveVector stores an embedding vector for an article
func (d *DB) SaveVector(articleID string, embedding []float32, model string) error {
	// Convert float32 slice to bytes
	bytes := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := float32Bits(v)
		bytes[i*4] = byte(bits >> 24)
		bytes[i*4+1] = byte(bits >> 16)
		bytes[i*4+2] = byte(bits >> 8)
		bytes[i*4+3] = byte(bits)
	}

	_, err := d.conn.Exec(`
		INSERT OR REPLACE INTO vectors (article_id, embedding, model, created_at)
		VALUES (?, ?, ?, strftime('%s', 'now'))
	`, articleID, bytes, model)
	return err
}

// GetVector retrieves an embedding vector for an article
func (d *DB) GetVector(articleID string) ([]float32, error) {
	var bytes []byte
	err := d.conn.QueryRow("SELECT embedding FROM vectors WHERE article_id = ?", articleID).Scan(&bytes)
	if err != nil {
		return nil, err
	}

	// Convert bytes back to float32 slice
	dim := len(bytes) / 4
	embedding := make([]float32, dim)
	for i := 0; i < dim; i++ {
		bits := (uint32(bytes[i*4]) << 24) |
			(uint32(bytes[i*4+1]) << 16) |
			(uint32(bytes[i*4+2]) << 8) |
			uint32(bytes[i*4+3])
		embedding[i] = float32FromBits(bits)
	}
	return embedding, nil
}

// DeleteVector removes a vector entry
func (d *DB) DeleteVector(articleID string) error {
	_, err := d.conn.Exec("DELETE FROM vectors WHERE article_id = ?", articleID)
	return err
}

// GetAllVectors retrieves all vectors for batch processing
func (d *DB) GetAllVectors() ([]Vector, error) {
	rows, err := d.conn.Query(`
		SELECT id, article_id, embedding, model, created_at
		FROM vectors
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vectors []Vector
	for rows.Next() {
		var v Vector
		if err := rows.Scan(&v.ID, &v.ArticleID, &v.Embedding, &v.Model, &v.CreatedAt); err != nil {
			return nil, err
		}
		vectors = append(vectors, v)
	}
	return vectors, rows.Err()
}

// GetArticleVectors retrieves vectors for all articles of an entry
func (d *DB) GetArticleVectors(entryID string) (map[string][]float32, error) {
	rows, err := d.conn.Query(`
		SELECT v.article_id, v.embedding
		FROM vectors v
		JOIN articles a ON v.article_id = a.id
		WHERE a.entry_id = ?
	`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vectors := make(map[string][]float32)
	for rows.Next() {
		var articleID string
		var bytes []byte
		if err := rows.Scan(&articleID, &bytes); err != nil {
			return nil, err
		}

		dim := len(bytes) / 4
		embedding := make([]float32, dim)
		for i := 0; i < dim; i++ {
			bits := (uint32(bytes[i*4]) << 24) |
				(uint32(bytes[i*4+1]) << 16) |
				(uint32(bytes[i*4+2]) << 8) |
				uint32(bytes[i*4+3])
			embedding[i] = float32FromBits(bits)
		}
		vectors[articleID] = embedding
	}
	return vectors, rows.Err()
}

// VectorCount returns the number of stored vectors
func (d *DB) VectorCount() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&count)
	return count, err
}

// Helper functions for float32 bit conversion
func float32Bits(f float32) uint32 {
	return *(*uint32)(unsafe.Pointer(&f))
}

func float32FromBits(bits uint32) float32 {
	return *(*float32)(unsafe.Pointer(&bits))
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
	TotalAssets     int
	TotalHistory    int
	VectorCount     int
}

func (d *DB) Stats() (*Stats, error) {
	s := &Stats{}

	if err := d.conn.QueryRow("SELECT COUNT(*) FROM entries").Scan(&s.TotalEntries); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM articles").Scan(&s.TotalArticles); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM article_assets").Scan(&s.TotalAssets); err != nil {
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

	// Vector count
	s.VectorCount, _ = d.VectorCount()

	return s, nil
}
