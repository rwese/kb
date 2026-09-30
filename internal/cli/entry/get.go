package entry

import (
	"context"
	"fmt"
	"strings"

	"github.com/rwese/kb/internal/cli/shared"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func NewGetCommand() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get entry by ID",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted entries"},
			&cli.BoolFlag{Name: "articles", Aliases: []string{"a"}, Usage: "Include articles"},
			&cli.BoolFlag{Name: "attachments", Usage: "Include attachments"},
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
			defer func() { _ = database.Close() }()
			if err := database.Init(); err != nil {
				return err
			}

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
				view, err := shared.LoadArticleView(database, article)
				if err != nil {
					return err
				}
				return shared.PrintArticle(view, asJSON)
			}

			// Entry ID
			entry, err := database.GetEntryWithDeleted(id, includeDeleted)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			showArticles := cmd.Bool("articles")
			showAttachments := cmd.Bool("attachments")
			if !showArticles && !showAttachments {
				return printEntry(entry, asJSON)
			}

			var articles []shared.ArticleView
			if showArticles {
				articleList, err := database.GetArticlesWithDeleted(id, includeDeleted)
				if err != nil {
					return err
				}
				articles, err = shared.LoadArticleViews(database, articleList)
				if err != nil {
					return err
				}
			}

			var attachments []db.EntryAttachment
			if showAttachments {
				attachments, err = database.ListEntryAttachments(id)
				if err != nil {
					return err
				}
			}

			if asJSON {
				switch {
				case showArticles && showAttachments:
					return shared.FormatJSON(shared.EntryWithArticlesAndAttachments{Entry: *entry, Articles: articles, Attachments: attachments})
				case showArticles:
					return shared.FormatJSON(shared.EntryWithArticleViews{Entry: *entry, Articles: articles})
				default:
					return shared.FormatJSON(shared.EntryWithAttachments{Entry: *entry, Attachments: attachments})
				}
			}

			if err := printEntryWithArticles(entry, articles, asJSON); err != nil {
				return err
			}
			shared.PrintAttachmentsSection(attachments)
			return nil
		},
	}
}

func printEntryWithArticles(entry *db.Entry, articles []shared.ArticleView, asJSON bool) error {
	if asJSON {
		return shared.FormatJSON(shared.EntryWithArticleViews{Entry: *entry, Articles: articles})
	}

	// Markdown document format
	fmt.Printf("# %s\n\n", entry.Title)
	if entry.Tags != "" {
		fmt.Printf("Tags: %s\n\n", entry.Tags)
	}

	fmt.Printf("## Metadata\n\n")
	fmt.Printf("- ID: %s\n", entry.ID)
	fmt.Printf("- Created: %s\n", entry.CreatedAt)
	fmt.Printf("- Updated: %s\n\n", entry.UpdatedAt)

	for i, a := range articles {
		if a.Title != "" {
			fmt.Printf("## %s\n\n", a.Title)
		} else {
			fmt.Printf("## Article %d\n\n", i+1)
		}
		fmt.Printf("## Metadata\n\n")
		fmt.Printf("- Added: %s\n\n", a.CreatedAt)
		fmt.Printf("---\n\n%s\n\n", a.Content)
		shared.PrintAssetsSection(a.Assets)
	}

	return nil
}

func printEntry(entry *db.Entry, asJSON bool) error {
	if asJSON {
		return shared.FormatJSON(entry)
	}

	// Markdown document format - entry only
	fmt.Printf("# %s\n\n", entry.Title)
	if entry.Tags != "" {
		fmt.Printf("Tags: %s\n\n", entry.Tags)
	}

	fmt.Printf("## Metadata\n\n")
	fmt.Printf("- ID: %s\n", entry.ID)
	fmt.Printf("- Created: %s\n", entry.CreatedAt)
	fmt.Printf("- Updated: %s\n", entry.UpdatedAt)

	return nil
}
