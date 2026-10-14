// Package bench wires the hidden `kb bench` command: it runs JSONL
// retrieval benchmarks against an isolated KB seeded from the dataset, so it
// never touches a real knowledgebase or config discovery.
package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/benchmark"
	"github.com/urfave/cli/v3"
)

// NewBenchCommand returns the hidden benchmark command group.
func NewBenchCommand() *cli.Command {
	return &cli.Command{
		Name:   "bench",
		Usage:  "Run retrieval benchmarks (hidden)",
		Hidden: true,
		Commands: []*cli.Command{
			newRunCommand(),
		},
	}
}

func newRunCommand() *cli.Command {
	return &cli.Command{
		Name:      "run",
		Usage:     "Run one benchmark dataset and print a JSON report",
		ArgsUsage: "",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dataset", Aliases: []string{"d"}, Usage: "Path to a JSONL benchmark dataset (required)"},
			&cli.IntFlag{Name: "top-k", Aliases: []string{"k"}, Usage: "Ranking cutoff (default 3)"},
			&cli.StringFlag{Name: "baseline", Aliases: []string{"b"}, Usage: "Optional baseline JSON; fails with nonzero exit when floors are violated"},
		},
		Action: runAction,
	}
}

func runAction(ctx context.Context, cmd *cli.Command) error {
	datasetPath := cmd.String("dataset")
	if datasetPath == "" {
		return fmt.Errorf("--dataset required")
	}

	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		return fmt.Errorf("read dataset: %w", err)
	}
	dataset, err := benchmark.ParseDataset(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	report, err := benchmark.Run(dataset, benchmark.Config{TopK: cmd.Int("top-k")})
	if err != nil {
		return err
	}

	var checkErr error
	if baselinePath := cmd.String("baseline"); baselinePath != "" {
		baselineRaw, err := os.ReadFile(baselinePath)
		if err != nil {
			return fmt.Errorf("read baseline: %w", err)
		}
		baseline, err := benchmark.LoadBaseline(bytes.NewReader(baselineRaw))
		if err != nil {
			return err
		}
		result, err := benchmark.EvaluateBaseline(baseline, dataset.ID, report)
		if err != nil {
			checkErr = err
		}
		report.Baseline = result
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}

	// Emit the full report before failing, so violations are always visible.
	return checkErr
}
