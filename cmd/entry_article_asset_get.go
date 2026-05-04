package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleAssetGet() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get asset metadata",
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

			if cmd.Bool("json") {
				return formatJSON(asset)
			}

			fmt.Printf("# Asset %s\n\n", asset.ID)
			fmt.Printf("- Article ID: %s\n", asset.ArticleID)
			fmt.Printf("- Logical Path: %s\n", asset.LogicalPath)
			fmt.Printf("- Original Path: %s\n", asset.OriginalPath)
			fmt.Printf("- Stored Path: %s\n", managedAssetPath(cfg, *asset))
			fmt.Printf("- SHA256: %s\n", asset.SHA256)
			fmt.Printf("- Size: %d bytes\n", asset.SizeBytes)
			fmt.Printf("- Created: %s\n", asset.CreatedAt)
			return nil
		},
	}
}
