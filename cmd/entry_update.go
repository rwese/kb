package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryUpdate() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update an entry",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Entry title"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
		},
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

			// Get existing entry
			entry, err := database.GetEntry(id)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			// Update fields
			title := cmd.String("title")
			if title == "" {
				title = entry.Title
			}

			tags := cmd.String("tags")
			if tags == "" {
				tags = entry.Tags
			}

			if err := database.UpdateEntry(id, title, tags); err != nil {
				return err
			}

			fmt.Printf("Updated entry %s\n", id)
			return nil
		},
	}
}
