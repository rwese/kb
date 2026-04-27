package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/id"
	"github.com/urfave/cli/v3"
)

func (c *Commands) append() *cli.Command {
	return &cli.Command{
		Name:      "append",
		Usage:     "Append article to entry",
		ArgsUsage: "<entry> [content]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Article title"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Read content from file"},
			&cli.BoolFlag{Name: "stdin", Aliases: []string{"s"}, Usage: "Read content from stdin"},
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

			entryID := cmd.Args().First()
			if entryID == "" {
				return fmt.Errorf("entry ID required")
			}

			// Verify entry exists
			_, err = database.GetEntry(entryID)
			if err != nil {
				return fmt.Errorf("entry not found: %s", entryID)
			}

			// Get content from args, file, stdin, or flag
			var content, title string

			if cmd.Args().Len() > 1 {
				content = cmd.Args().Slice()[1]
			} else if f := cmd.String("file"); f != "" {
				data, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				content = string(data)
				title = cmd.String("title")
				if title == "" {
					title = f
				}
			} else if cmd.Bool("stdin") {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				content = string(data)
			} else {
				content = cmd.String("content")
			}

			title = cmd.String("title")

			if content == "" {
				return fmt.Errorf("content required")
			}

			articleID := id.Article(entryID)
			if err := database.AddArticle(articleID, entryID, title, content); err != nil {
				return err
			}

			fmt.Printf("Added article %s to entry %s\n", articleID, entryID)
			return nil
		},
	}
}
