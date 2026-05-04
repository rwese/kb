package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleAssetAdd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Attach files or directories to an article",
		ArgsUsage: "<entry-id> <article-id> <path>...",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "overwrite", Usage: "Replace an existing asset when logical paths collide"},
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
				return fmt.Errorf("entry ID, article ID, and at least one path are required")
			}

			entryID := args.Get(0)
			articleID := args.Get(1)
			if _, err := requireArticleOwnership(database, entryID, articleID); err != nil {
				return err
			}

			files, err := assetstore.ExpandPaths(args.Slice()[2:])
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("no files found to import")
			}

			existingAssets, err := database.ListArticleAssets(articleID)
			if err != nil {
				return err
			}
			existingByPath := make(map[string]db.ArticleAsset, len(existingAssets))
			for _, asset := range existingAssets {
				existingByPath[asset.LogicalPath] = asset
			}

			var overwriteIDs []string
			var replaced []db.ArticleAsset
			for _, file := range files {
				if assetstore.HasPathTraversal(file.LogicalPath) {
					return fmt.Errorf("invalid logical path %q", file.LogicalPath)
				}
				if existing, ok := existingByPath[file.LogicalPath]; ok {
					if !cmd.Bool("overwrite") {
						return fmt.Errorf("asset already exists at logical path %q", file.LogicalPath)
					}
					overwriteIDs = append(overwriteIDs, existing.ID)
					replaced = append(replaced, existing)
				}
			}

			staged, err := assetstore.StageImports(cfg.AssetsPath, articleID, files)
			if err != nil {
				return err
			}
			defer func() {
				if staged != nil {
					assetstore.CleanupStaged(cfg.AssetsPath, staged)
				}
			}()

			if err := database.SaveArticleAssets(entryID, staged, overwriteIDs); err != nil {
				return err
			}

			for _, asset := range replaced {
				if err := assetstore.RemoveAssetTree(cfg.AssetsPath, asset); err != nil {
					return fmt.Errorf("asset metadata updated but failed to remove old stored file tree %s: %w", filepath.Join(cfg.AssetsPath, asset.ArticleID, asset.ID), err)
				}
			}

			imported := staged
			staged = nil
			if cmd.Bool("json") {
				return formatJSON(imported)
			}

			for _, asset := range imported {
				fmt.Printf("Added asset %s to article %s at %s\n", asset.ID, articleID, asset.LogicalPath)
			}
			return nil
		},
	}
}
