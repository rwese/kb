package shared

import (
	"path/filepath"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
)

// ArticleView pairs an article with its attached assets for display.
type ArticleView struct {
	db.Article
	Assets []db.ArticleAsset `json:"assets"`
}

// EntryWithArticleViews pairs an entry with article views for display.
type EntryWithArticleViews struct {
	db.Entry
	Articles []ArticleView `json:"articles"`
}

// EntryWithAttachments pairs an entry with its attachments for display.
type EntryWithAttachments struct {
	db.Entry
	Attachments []db.EntryAttachment `json:"attachments"`
}

// EntryWithArticlesAndAttachments pairs an entry with articles and
// attachments for display.
type EntryWithArticlesAndAttachments struct {
	db.Entry
	Articles    []ArticleView        `json:"articles"`
	Attachments []db.EntryAttachment `json:"attachments"`
}

// LoadArticleView loads an article with its attached assets.
func LoadArticleView(database *db.DB, article *db.Article) (ArticleView, error) {
	assetList, err := database.ListArticleAssets(article.ID)
	if err != nil {
		return ArticleView{}, err
	}
	return ArticleView{
		Article: *article,
		Assets:  assetList,
	}, nil
}

// LoadArticleViews loads the article views for a list of articles.
func LoadArticleViews(database *db.DB, articles []db.Article) ([]ArticleView, error) {
	views := make([]ArticleView, 0, len(articles))
	for _, article := range articles {
		view, err := LoadArticleView(database, &article)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// ManagedAssetPath returns the filesystem path where an article asset is stored.
func ManagedAssetPath(cfg *config.Config, asset db.ArticleAsset) string {
	return filepath.Join(cfg.AssetsPath, filepath.FromSlash(asset.StoreRelPath))
}

// ManagedAttachmentPath returns the filesystem path where an attachment is stored.
func ManagedAttachmentPath(cfg *config.Config, att db.EntryAttachment) string {
	return filepath.Join(cfg.AssetsPath, filepath.FromSlash(att.StoreRelPath))
}
