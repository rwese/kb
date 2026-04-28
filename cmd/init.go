package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) init() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize knowledgebase database",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force init even if database already exists",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			force := cmd.Bool("force")

			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			// Check if database already exists
			if _, err := os.Stat(cfg.DBPath); err == nil && !force {
				return fmt.Errorf("database already exists at %s (use --force to override)", cfg.DBPath)
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()

			if err := database.Init(); err != nil {
				return err
			}

			count, _ := database.Count()
			fmt.Printf("Initialized: %s\nEntries: %d\n", cfg.DBPath, count)
			return nil
		},
	}
}
