package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/embed"
	"github.com/urfave/cli/v3"
)

func (c *Commands) download() *cli.Command {
	return &cli.Command{
		Name:  "download",
		Usage: "Download local embedding assets (llama.cpp library and model)",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Force re-download even if assets exist"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Verbose output"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}

			// Check if local embedder is configured
			if cfg.Embedder != "local" {
				fmt.Println("Note: Local embedder is not configured.")
				fmt.Println("Set 'embedder: local' in ~/.config/kb/config.yaml to use local embeddings.")
				fmt.Println("")
			}

			cacheDir := cfg.Local.CacheDir
			if cacheDir == "" {
				cacheDir = os.ExpandEnv("$HOME/.cache/kb")
			}

			fmt.Printf("Cache directory: %s\n", cacheDir)
			fmt.Println("")

			// Check current status
			ok, msg := embed.CheckAssets(cacheDir)
			if ok && !cmd.Bool("force") {
				fmt.Println("Assets already downloaded:")
				assets, _ := embed.GetAssetInfo(cacheDir)
				if assets != nil {
					fmt.Printf("  Library: %s\n", assets.LibraryFile)
					fmt.Printf("  Model:   %s\n", assets.ModelFile)
				}
				fmt.Println("")
				fmt.Println("Use --force to re-download.")
				return nil
			}

			if !ok {
				fmt.Printf("Status: %s\n\n", msg)
			}

			// Download assets
			fmt.Println("Downloading local embedding assets...")
			fmt.Println("")

			progress := func(stage string, downloaded, total int64) {
				if cmd.Bool("verbose") {
					if total > 0 {
						pct := float64(downloaded) / float64(total) * 100
						fmt.Printf("\r  %s: %.1f%%", stage, pct)
					} else {
						fmt.Printf("\r  %s: connecting...", stage)
					}
				}
			}

			err = embed.DownloadAll(cacheDir, progress)
			if err != nil {
				fmt.Println("")
				return fmt.Errorf("download failed: %w", err)
			}

			if cmd.Bool("verbose") {
				fmt.Println("")
			}

			fmt.Println("")
			fmt.Println("✓ Local embedding assets downloaded successfully!")
			fmt.Println("")
			fmt.Println("You can now use local embeddings by setting 'embedder: local' in config.")

			return nil
		},
	}
}
