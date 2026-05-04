package cmd

import (
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
)

func openDBFromConfig() (*config.Config, *db.DB, error) {
	cfg, err := config.Discover()
	if err != nil {
		return nil, nil, err
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}
	if err := database.Init(); err != nil {
		database.Close()
		return nil, nil, err
	}
	return cfg, database, nil
}

func requireArticleOwnership(database *db.DB, entryID, articleID string) (*db.Article, error) {
	if _, err := database.GetEntry(entryID); err != nil {
		return nil, fmt.Errorf("entry not found: %w", err)
	}

	article, err := database.GetArticle(articleID)
	if err != nil {
		return nil, fmt.Errorf("article not found: %w", err)
	}
	if article.EntryID != entryID {
		return nil, fmt.Errorf("article %s does not belong to entry %s", articleID, entryID)
	}
	return article, nil
}
