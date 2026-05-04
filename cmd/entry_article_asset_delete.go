package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleAssetDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an article asset",
		ArgsUsage: "<entry-id> <article-id> <asset-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, database, err := openDBFromConfig()
			if err != nil {
				return err
			}
			defer database.Close()

			args := cmd.Args()
			if args.Len() < 3 {
				return fmt.Errorf("entry ID, article ID, and asset ID required")
			}

			entryID := args.Get(0)
			articleID := args.Get(1)
			assetID := args.Get(2)
			if _, err := requireArticleOwnership(database, entryID, articleID); err != nil {
				return err
			}

			asset, err := database.GetArticleAsset(articleID, assetID)
			if err != nil {
				return fmt.Errorf("asset not found: %w", err)
			}

			if err := database.DeleteArticleAsset(articleID, assetID); err != nil {
				return err
			}
			if err := database.UpdateEntryTime(entryID); err != nil {
				return err
			}

			if err := assetstore.RemoveAssetTree(cfg.AssetsPath, *asset); err != nil {
				return fmt.Errorf("asset deleted from database but failed to remove stored file tree %s: %w", filepath.Join(cfg.AssetsPath, asset.ArticleID, asset.ID), err)
			}

			if cmd.Bool("json") {
				return formatJSON(asset)
			}

			fmt.Printf("Deleted asset %s from article %s\n", assetID, articleID)
			return nil
		},
	}
}
