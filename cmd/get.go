package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) get() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get entry or article",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted entries"},
			&cli.BoolFlag{Name: "articles", Aliases: []string{"a"}, Usage: "Include articles (for entries)"},
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

			includeDeleted := cmd.Bool("all")
			asJSON := cmd.Bool("json")

			// Detect if this is an article ID (contains '-')
			if strings.Contains(id, "-") {
				article, err := database.GetArticleWithDeleted(id, includeDeleted)
				if err != nil {
					return fmt.Errorf("article not found: %w", err)
				}
				return printArticle(article, asJSON)
			}

			// Entry ID
			entry, err := database.GetEntryWithDeleted(id, includeDeleted)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			// Show articles only with --articles flag
			if cmd.Bool("articles") {
				articles, err := database.GetArticlesWithDeleted(id, includeDeleted)
				if err != nil {
					return err
				}
				return printEntryWithArticles(entry, articles, asJSON)
			}

			return printEntry(entry, asJSON)
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

	// Markdown document format
	fmt.Printf("# %s\n\n", entry.Title)
	if entry.Tags != "" {
		fmt.Printf("Tags: %s\n\n", entry.Tags)
	}
	fmt.Printf("*Entry %s | Created: %s | Updated: %s*\n\n", entry.ID, entry.CreatedAt, entry.UpdatedAt)

	for i, a := range articles {
		if a.Title != "" {
			fmt.Printf("## %s\n\n", a.Title)
		} else {
			fmt.Printf("## Article %d\n\n", i+1)
		}
		fmt.Printf("*Added: %s*\n\n", a.CreatedAt)
		fmt.Printf("---\n\n%s\n\n", a.Content)
	}

	return nil
}

func printEntry(entry *db.Entry, asJSON bool) error {
	if asJSON {
		return formatJSON(entry)
	}

	// Markdown document format - entry only
	fmt.Printf("# %s\n\n", entry.Title)
	if entry.Tags != "" {
		fmt.Printf("Tags: %s\n\n", entry.Tags)
	}
	fmt.Printf("*Entry %s | Created: %s | Updated: %s*\n", entry.ID, entry.CreatedAt, entry.UpdatedAt)

	return nil
}

func printArticle(article *db.Article, asJSON bool) error {
	if asJSON {
		return formatJSON(article)
	}

	// Markdown document format
	fmt.Printf("# Article %s\n\n", article.ID)
	if article.Title != "" {
		fmt.Printf("**%s**\n\n", article.Title)
	}
	fmt.Printf("*Entry %s | Added: %s*\n\n", article.EntryID, article.CreatedAt)
	fmt.Printf("---\n\n%s\n", article.Content)

	return nil
}
