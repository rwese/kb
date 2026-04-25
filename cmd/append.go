package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) append() *cli.Command {
	return &cli.Command{
		Name:  "append",
		Usage: "Append article to existing entry",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Article title (optional)"},
			&cli.StringFlag{Name: "content", Aliases: []string{"c"}, Usage: "Article content"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Read content from file"},
			&cli.BoolFlag{Name: "stdin", Aliases: []string{"s"}, Usage: "Read content from stdin"},
			&cli.Int64Flag{Name: "entry", Aliases: []string{"e"}, Usage: "Entry ID to append to"},
			&cli.StringFlag{Name: "entry-title", Aliases: []string{"T"}, Usage: "Entry title to append to (creates entry if not exists)"},
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

			var entryID int64
			var articleTitle, content string

			// Get entry ID
			if e := cmd.Int64("entry"); e > 0 {
				entryID = e
			} else if t := cmd.String("entry-title"); t != "" {
				// Try to find existing entry or create new one
				entry, err := database.GetEntryByTitle(t)
				if err == db.ErrNotFound {
					id, err := database.AddEntry(t, "")
					if err != nil {
						return fmt.Errorf("create entry: %w", err)
					}
					entryID = id
					fmt.Printf("Created new entry #%d\n", entryID)
				} else if err != nil {
					return fmt.Errorf("find entry: %w", err)
				} else {
					entryID = entry.ID
				}
			} else {
				return fmt.Errorf("either --entry or --entry-title required")
			}

			// Verify entry exists
			_, err = database.GetEntry(entryID)
			if err != nil {
				return fmt.Errorf("entry not found: %d", entryID)
			}

			// Get article title
			articleTitle = cmd.String("title")

			// Get content
			if f := cmd.String("file"); f != "" {
				data, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				content = string(data)
				if articleTitle == "" {
					articleTitle = f
				}
			} else if cmd.Bool("stdin") {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				content = string(data)
			} else if c := cmd.String("content"); c != "" {
				content = c
			} else {
				return fmt.Errorf("content required (--content, --file, or --stdin)")
			}

			id, err := database.AddArticle(entryID, articleTitle, content)
			if err != nil {
				return err
			}

			fmt.Printf("Added article #%d to entry #%d\n", id, entryID)
			return nil
		},
	}
}
