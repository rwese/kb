package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) setup() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Interactive setup and onboarding",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "non-interactive", Usage: "Run non-interactive (use defaults)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			fmt.Println("=== kb Knowledgebase Setup ===")
			fmt.Println()

			// Check/create database
			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			if err := database.Init(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}

			entryCount, _ := database.Count()
			fmt.Printf("✓ Database: %s\n", cfg.DBPath)
			fmt.Printf("✓ Entries: %d\n", entryCount)
			database.Close()

			fmt.Println()
			fmt.Println("Setup complete!")
			fmt.Println()
			fmt.Println("Quick start:")
			fmt.Println("  kb add -t 'First entry' -c 'My notes...'")
			fmt.Println("  kb search 'notes'")
			fmt.Println()
			fmt.Println("For more commands: kb --help")

			return nil
		},
	}
}
