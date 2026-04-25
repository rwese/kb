package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) get() *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get entry with articles",
		Flags: []cli.Flag{
			&cli.Int64Flag{Name: "entry", Aliases: []string{"e"}, Usage: "Entry ID"},
			&cli.Int64Flag{Name: "article", Aliases: []string{"a"}, Usage: "Specific article ID"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
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

			// Get specific article
			if articleID := cmd.Int64("article"); articleID > 0 {
				article, err := database.GetArticle(articleID)
				if err != nil {
					return fmt.Errorf("article not found: %w", err)
				}
				return printArticle(article, cmd.Bool("json"))
			}

			// Get entry
			entryID := cmd.Int64("entry")
			if entryID == 0 {
				return fmt.Errorf("--entry required")
			}

			entry, err := database.GetEntry(entryID)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			articles, err := database.GetArticles(entryID)
			if err != nil {
				return err
			}

			return printEntryWithArticles(entry, articles, cmd.Bool("json"))
		},
	}
}

func printEntryWithArticles(entry *db.Entry, articles []db.Article, asJSON bool) error {
	if asJSON {
		type EntryJSON struct {
			db.Entry
			Articles []db.Article `json:"articles"`
		}
		return formatJSON(EntryJSON{Entry: *entry, Articles: articles})
	}

	fmt.Printf("=== Entry #%d: %s ===\n", entry.ID, entry.Title)
	if entry.Tags != "" {
		fmt.Printf("Tags: %s\n", entry.Tags)
	}
	fmt.Printf("Created: %s | Updated: %s\n\n", entry.CreatedAt, entry.UpdatedAt)

	for i, a := range articles {
		fmt.Printf("--- Article #%d ---\n", i+1)
		if a.Title != "" {
			fmt.Printf("Title: %s\n", a.Title)
		}
		fmt.Printf("Added: %s\n\n%s\n\n", a.CreatedAt, a.Content)
	}

	return nil
}

func printArticle(article *db.Article, asJSON bool) error {
	if asJSON {
		return formatJSON(article)
	}

	fmt.Printf("=== Article #%d ===\n", article.ID)
	fmt.Printf("Entry: %d\n", article.EntryID)
	if article.Title != "" {
		fmt.Printf("Title: %s\n", article.Title)
	}
	fmt.Printf("Added: %s\n\n%s\n", article.CreatedAt, article.Content)
	return nil
}
