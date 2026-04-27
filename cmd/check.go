package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
	"github.com/urfave/cli/v3"
)

func (c *Commands) check() *cli.Command {
	return &cli.Command{
		Name:    "check",
		Aliases: []string{"doctor", "status"},
		Usage:   "Validate kb installation and database",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				fmt.Println("✗ Config discovery failed:", err)
				return err
			}

			errors := 0

			// Check config
			fmt.Printf("Config path:   %s\n", cfg.DBPath)
			fmt.Printf("Embedder:      %s\n", cfg.Embedder)
			fmt.Printf("Top K:         %d\n", cfg.TopK)

			// Check database file
			fmt.Println()
			if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
				fmt.Println("✗ Database file not found (run 'kb init' or 'kb setup')")
				errors++
			} else {
				fmt.Printf("✓ Database file exists\n")

				// Check database
				database, err := db.Open(cfg.DBPath)
				if err != nil {
					fmt.Println("✗ Failed to open database:", err)
					errors++
				} else {
					defer database.Close()

					// Check tables
					entryCount, err := database.Count()
					if err != nil {
						fmt.Println("✗ Failed to query entries:", err)
						errors++
					} else {
						fmt.Printf("✓ Entries table: %d entries\n", entryCount)
					}

					articleCount, err := database.ArticleCount()
					if err != nil {
						fmt.Println("✗ Failed to query articles:", err)
						errors++
					} else {
						fmt.Printf("✓ Articles table: %d articles\n", articleCount)
					}

					// Test search
					results, err := database.Search("test", 1)
					if err != nil {
						fmt.Println("✗ FTS search failed:", err)
						errors++
					} else {
						fmt.Printf("✓ FTS search: OK (test returned %d results)\n", len(results))
					}

					// Check vectors
					vectorCount, err := database.VectorCount()
					if err != nil {
						fmt.Println("✗ Failed to query vectors:", err)
						errors++
					} else {
						fmt.Printf("✓ Vectors: %d indexed\n", vectorCount)
					}
				}
			}

			// Check embedder status
			fmt.Println()
			switch cfg.Embedder {
			case "local":
				fmt.Printf("Embedder: local (%s)\n", cfg.Local.Model)
				ok, msg := embed.CheckAssets(cfg.Local.CacheDir)
				if ok {
					fmt.Printf("✓ Library: %s\n", embed.LibraryFileFromCache(cfg.Local.CacheDir))
					fmt.Printf("✓ Model:   %s\n", embed.ModelFileFromCache(cfg.Local.CacheDir))
				} else {
					fmt.Printf("✗ Assets: %s\n", msg)
					fmt.Println("  Run 'kb download' to fetch assets")
					errors++
				}
			case "ollama":
				fmt.Printf("Embedder: ollama (%s)\n", cfg.Ollama.Model)
				fmt.Printf("Base URL: %s\n", cfg.Ollama.BaseURL)
			default:
				fmt.Println("Embedder: none (BM25-only mode)")
			}

			// Summary
			fmt.Println()
			if errors == 0 {
				fmt.Println("✓ All checks passed")
				return nil
			}
			return fmt.Errorf("%d check(s) failed", errors)
		},
	}
}
