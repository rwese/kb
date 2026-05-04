package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) stats() *cli.Command {
	return &cli.Command{
		Name:  "stats",
		Usage: "Show database statistics",
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
			if err := database.Init(); err != nil {
				return err
			}

			stats, err := database.Stats()
			if err != nil {
				return err
			}

			// Markdown format as default
			fmt.Printf("# Database Statistics\n\n")
			fmt.Printf("**Path:** `%s`\n\n", cfg.DBPath)

			fmt.Println("## Entries")
			fmt.Printf("- Total: %d\n", stats.TotalEntries)
			fmt.Printf("- Active: %d\n", stats.ActiveEntries)
			fmt.Printf("- Deleted: %d\n\n", stats.DeletedEntries)

			fmt.Println("## Articles")
			fmt.Printf("- Total: %d\n", stats.TotalArticles)
			fmt.Printf("- Active: %d\n", stats.ActiveArticles)
			fmt.Printf("- Deleted: %d\n\n", stats.DeletedArticles)

			fmt.Println("## Assets")
			fmt.Printf("- Total: %d\n\n", stats.TotalAssets)

			fmt.Println("## History")
			fmt.Printf("- Total: %d\n", stats.TotalHistory)

			return nil
		},
	}
}
