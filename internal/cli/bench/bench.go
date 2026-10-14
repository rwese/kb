// Package bench wires the hidden `kb bench` command: it runs JSONL
// retrieval benchmarks against an isolated KB seeded from the dataset, so it
// never touches a real knowledgebase. Config discovery is consulted only for
// ranking defaults (algorithm, weights, embedder); flags and KB_* env
// variables override them.
package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/benchmark"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/embed"
	"github.com/rwese/kb/internal/search"
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
			newDiffCommand(),
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
			&cli.StringFlag{
				Name:    "algorithm",
				Usage:   "Ranking algorithm: bm25 or hybrid (default bm25)",
				Sources: cli.EnvVars("KB_ALGORITHM"),
			},
			&cli.FloatFlag{
				Name:    "bm25-weight",
				Usage:   "Hybrid weight of the normalized BM25 score (default 0.3)",
				Sources: cli.EnvVars("KB_BM25_WEIGHT"),
			},
			&cli.FloatFlag{
				Name:    "semantic-weight",
				Usage:   "Hybrid weight of the query cosine similarity (default 0.7)",
				Sources: cli.EnvVars("KB_SEMANTIC_WEIGHT"),
			},
			&cli.StringFlag{Name: "baseline", Aliases: []string{"b"}, Usage: "Optional baseline JSON; fails with nonzero exit when floors are violated"},
		},
		Action: runAction,
	}
}

// resolveRanking builds the ranking config from flags/env (via Sources)
// over config discovery over package defaults, and returns the embedder the
// hybrid algorithm needs.
func resolveRanking(cmd *cli.Command) (benchmark.RankingConfig, embed.Embedder, string, error) {
	cfg, err := config.Discover()
	if err != nil {
		return benchmark.RankingConfig{}, nil, "", err
	}

	algorithm := cmd.String("algorithm")
	if algorithm == "" {
		algorithm = cfg.Ranker.Algorithm
	}
	// Auto is a search-side convenience; a benchmark needs a concrete
	// algorithm so reports stay reproducible.
	if algorithm == "" || algorithm == search.AlgorithmAuto {
		algorithm = search.AlgorithmBM25
	}

	bm25Weight := cmd.Float("bm25-weight")
	if bm25Weight == 0 {
		bm25Weight = cfg.Ranker.BM25Weight
	}
	semanticWeight := cmd.Float("semantic-weight")
	if semanticWeight == 0 {
		semanticWeight = cfg.Ranker.SemanticWeight
	}

	// Weights are inert for pure BM25 retrieval; recording them anyway
	// would make those reports incomparable with weight-less gate runs.
	rankCfg := benchmark.RankingConfig{Algorithm: algorithm}
	if algorithm == search.AlgorithmHybrid {
		rankCfg.BM25Weight = bm25Weight
		rankCfg.SemanticWeight = semanticWeight
	}

	var emb embed.Embedder
	if algorithm == search.AlgorithmHybrid {
		if cfg.Embedder != "ollama" {
			return benchmark.RankingConfig{}, nil, "", fmt.Errorf("algorithm %q requires an embedder (config embedder: %s)", algorithm, cfg.Embedder)
		}
		emb = embed.NewEmbedder(cfg)
	}
	return rankCfg, emb, cfg.Ollama.Model, nil
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

	rankCfg, embedder, embeddingModel, err := resolveRanking(cmd)
	if err != nil {
		return err
	}

	report, err := benchmark.Run(dataset, benchmark.Config{
		TopK:           cmd.Int("top-k"),
		Ranking:        rankCfg,
		Embedder:       embedder,
		EmbeddingModel: embeddingModel,
	})
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

func newDiffCommand() *cli.Command {
	return &cli.Command{
		Name:      "diff",
		Usage:     "Compare two benchmark reports run on identical dataset bytes",
		ArgsUsage: "<reportA.json> <reportB.json>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args()
			if args.Len() != 2 {
				return fmt.Errorf("two report paths required")
			}

			var reports [2]benchmark.Report
			for i, path := range [2]string{args.Get(0), args.Get(1)} {
				raw, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read report %s: %w", path, err)
				}
				dec := json.NewDecoder(bytes.NewReader(raw))
				if err := dec.Decode(&reports[i]); err != nil {
					return fmt.Errorf("parse report %s: %w", path, err)
				}
			}

			diff, err := benchmark.Diff(reports[0], reports[1])
			if err != nil {
				return err
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(diff)
		},
	}
}
