package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
	"github.com/rwese/kb/internal/id"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryCreate() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new entry",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Entry title"},
			&cli.StringFlag{Name: "content", Aliases: []string{"c"}, Usage: "Initial article content"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Read content from file"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.BoolFlag{Name: "stdin", Aliases: []string{"s"}, Usage: "Read content from stdin"},
			&cli.BoolFlag{Name: "no-article", Usage: "Create entry without initial article"},
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

			var title, content, tags string

			// Get title
			title = cmd.String("title")
			if title == "" {
				fmt.Print("Entry title: ")
				title = readLine()
				if title == "" {
					return fmt.Errorf("title required")
				}
			}

			// Get tags
			tags = cmd.String("tags")

			// Get content unless --no-article
			if !cmd.Bool("no-article") {
				if f := cmd.String("file"); f != "" {
					data, err := os.ReadFile(f)
					if err != nil {
						return err
					}
					content = string(data)
				} else if cmd.Bool("stdin") {
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					content = string(data)
				} else if c := cmd.String("content"); c != "" {
					content = c
				} else {
					fmt.Print("Article content (Ctrl+D to finish):\n")
					content = readMultiline()
				}
			}

			// Generate ID and create entry
			entryID := id.Entry()
			if err := database.AddEntry(entryID, title, tags); err != nil {
				return err
			}

			// Add initial article if content provided
			var articleID string
			if content != "" {
				articleID = id.Article(entryID)
				if err := database.AddArticle(articleID, entryID, "", content); err != nil {
					return err
				}

				// Compute and store embedding if embedder is available
				e := embed.NewEmbedder(cfg)
				if cfg.Embedder == "local" || cfg.Embedder == "ollama" {
					// For local embedder, check if assets are available
					if cfg.Embedder == "local" {
						le, ok := e.(*embed.LocalEmbedder)
						if ok && !le.IsAvailable() {
							fmt.Printf("Created entry %s with article %s (no embedding)\n", entryID, articleID)
							return nil
						}
					}

					// Compute embedding
					emb, err := e.Embed(ctx, content)
					if err != nil {
						fmt.Printf("Warning: failed to compute embedding: %v\n", err)
						fmt.Printf("Created entry %s with article %s\n", entryID, articleID)
						return nil
					}

					if emb != nil {
						if err := database.SaveVector(articleID, emb, cfg.Local.Model); err != nil {
							fmt.Printf("Warning: failed to store embedding: %v\n", err)
						}
					}
				}

				fmt.Printf("Created entry %s with article %s\n", entryID, articleID)
			} else {
				fmt.Printf("Created entry %s\n", entryID)
			}

			return nil
		},
	}
}

func readLine() string {
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func readMultiline() string {
	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}
