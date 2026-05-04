package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
)

type articleView struct {
	db.Article
	Assets []db.ArticleAsset `json:"assets"`
}

type entryWithArticleViews struct {
	db.Entry
	Articles []articleView `json:"articles"`
}

func loadArticleView(database *db.DB, article *db.Article) (articleView, error) {
	assetList, err := database.ListArticleAssets(article.ID)
	if err != nil {
		return articleView{}, err
	}
	return articleView{
		Article: *article,
		Assets:  assetList,
	}, nil
}

func loadArticleViews(database *db.DB, articles []db.Article) ([]articleView, error) {
	views := make([]articleView, 0, len(articles))
	for _, article := range articles {
		view, err := loadArticleView(database, &article)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func printAssetsSection(assetList []db.ArticleAsset) {
	if len(assetList) == 0 {
		return
	}

	fmt.Printf("## Assets\n\n")
	for _, asset := range assetList {
		fmt.Printf("- %s (%s, %s)\n", asset.LogicalPath, assets.FormatSize(asset.SizeBytes), asset.ID)
	}
	fmt.Println()
}

func managedAssetPath(cfg *config.Config, asset db.ArticleAsset) string {
	return filepath.Join(cfg.AssetsPath, filepath.FromSlash(asset.StoreRelPath))
}
