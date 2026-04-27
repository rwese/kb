package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) delete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Aliases:   []string{"rm", "del"},
		Usage:     "Delete entry or article",
		ArgsUsage: "<id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()

			id := cmd.Args().First()
			if id == "" {
				return fmt.Errorf("ID required")
			}

			// Detect if this is an article ID (contains '-')
			if strings.Contains(id, "-") {
				// Delete vector first
				database.DeleteVector(id)
				return database.DeleteArticle(id)
			}

			// Entry ID - delete all associated vectors first
			vectors, err := database.GetArticleVectors(id)
			if err == nil {
				for articleID := range vectors {
					database.DeleteVector(articleID)
				}
			}
			return database.DeleteEntry(id)
		},
	}
}
