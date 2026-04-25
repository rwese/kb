package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/urfave/cli/v3"
)

func (c *Commands) config() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Show current config",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			fmt.Printf("DB Path:   %s\n", cfg.DBPath)
			fmt.Printf("Embedder:  %s\n", cfg.Embedder)
			fmt.Printf("Top K:     %d\n", cfg.TopK)

			// Check if db exists
			if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
				fmt.Println("\nDatabase: NOT FOUND (run 'kb init')")
			} else {
				fmt.Println("\nDatabase: EXISTS")
			}

			return nil
		},
	}
}
