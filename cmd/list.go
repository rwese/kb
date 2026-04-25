package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) list() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List all entries and their articles",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "articles", Aliases: []string{"a"}, Usage: "Show article count per entry"},
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

			entries, err := database.ListEntries()
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No entries found")
				return nil
			}

			if cmd.Bool("json") {
				// Build full structure
				type EntryJSON struct {
					db.Entry
					Articles []db.Article `json:"articles"`
				}
				var result []EntryJSON
				for _, e := range entries {
					articles, _ := database.GetArticles(e.ID)
					result = append(result, EntryJSON{Entry: e, Articles: articles})
				}
				return formatJSON(result)
			}

			// Plain text
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tTitle\tTags\tArticles\tUpdated\n")
			for _, e := range entries {
				articles, _ := database.GetArticles(e.ID)
				fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\n",
					e.ID, e.Title, e.Tags, len(articles), e.UpdatedAt)
			}
			w.Flush()

			return nil
		},
	}
}
