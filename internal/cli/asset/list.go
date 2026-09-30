package asset

import (
	"context"
	"fmt"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/cli/shared"
	"github.com/urfave/cli/v3"
)

func NewListCommand() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List assets attached to an article",
		ArgsUsage: "<entry-id> <article-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, database, err := shared.OpenDBFromConfig()
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			args := cmd.Args()
			if args.Len() < 2 {
				return fmt.Errorf("entry ID and article ID required")
			}

			entryID := args.Get(0)
			articleID := args.Get(1)
			if _, err := shared.RequireArticleOwnership(database, entryID, articleID); err != nil {
				return err
			}

			assetList, err := database.ListArticleAssets(articleID)
			if err != nil {
				return err
			}
			if cmd.Bool("json") {
				return shared.FormatJSON(assetList)
			}

			if len(assetList) == 0 {
				fmt.Printf("No assets found for article %s\n", articleID)
				return nil
			}

			fmt.Printf("Assets in article %s:\n\n", articleID)
			fmt.Println("| ID | Logical Path | Size | Created |")
			fmt.Println("|----|--------------|------|---------|")
			for _, asset := range assetList {
				fmt.Printf("| %s | %s | %s | %s |\n", asset.ID, asset.LogicalPath, assetstore.FormatSize(asset.SizeBytes), asset.CreatedAt)
			}
			return nil
		},
	}
}
