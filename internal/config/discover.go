package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ranking defaults shared by the search command, the benchmark CLI, and the
// hybrid ranker.
const (
	// DefaultRankerAlgorithm defers the bm25/hybrid choice to embedder
	// availability at call time.
	DefaultRankerAlgorithm = "auto"
	// DefaultBM25Weight is the hybrid weight of the normalized BM25 score.
	DefaultBM25Weight = 0.3
	// DefaultSemanticWeight is the hybrid weight of the cosine similarity.
	DefaultSemanticWeight = 0.7
)

// RankerConfig holds the ranking switches shared by `kb search` and
// `kb bench run`. Weights only apply to the hybrid algorithm.
type RankerConfig struct {
	// Algorithm is "auto" (default), "bm25", or "hybrid". "auto" uses
	// hybrid when the configured embedder can produce embeddings.
	Algorithm string `yaml:"algorithm"`
	// BM25Weight weights the per-query min-max normalized BM25 score.
	BM25Weight float64 `yaml:"bm25_weight"`
	// SemanticWeight weights the query/document cosine similarity.
	SemanticWeight float64 `yaml:"semantic_weight"`
}

type Config struct {
	DBPath     string       `yaml:"db_path"`
	AssetsPath string       `yaml:"assets_path"`
	Embedder   string       `yaml:"embedder"`
	Ollama     OllamaConfig `yaml:"ollama"`
	TopK       int          `yaml:"top_k"`
	Ranker     RankerConfig `yaml:"ranker"`
}

type OllamaConfig struct {
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
}

func Discover() (*Config, error) {
	for _, path := range configSearchPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		applyDefaults(&cfg)
		return &cfg, nil
	}

	cfg := &Config{
		DBPath:     filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db"),
		AssetsPath: filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "assets"),
	}
	applyDefaults(cfg)
	return cfg, nil
}

// configSearchPaths lists the candidate config files in precedence order:
// $KB_PATH, then the XDG user config dir ($XDG_CONFIG_HOME, default
// ~/.config), then the XDG system config dirs ($XDG_CONFIG_DIRS, default
// /etc/xdg), then a repo-local .kb.yaml.
func configSearchPaths() []string {
	var paths []string
	if envPath := os.Getenv("KB_PATH"); envPath != "" {
		paths = append(paths, envPath)
	}
	paths = append(paths, filepath.Join(userConfigHome(), "kb", "config.yaml"))
	for _, dir := range filepath.SplitList(systemConfigDirs()) {
		if dir == "" {
			continue
		}
		paths = append(paths, filepath.Join(dir, "kb", "config.yaml"))
	}
	return append(paths, ".kb.yaml")
}

// userConfigHome returns $XDG_CONFIG_HOME or the default ~/.config.
func userConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return expandTilde(xdg)
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// systemConfigDirs returns $XDG_CONFIG_DIRS or the default /etc/xdg.
func systemConfigDirs() string {
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		return dirs
	}
	return "/etc/xdg"
}

// applyDefaults fills unset values with the package defaults. Weights of
// exactly zero are indistinguishable from unset, so a hybrid run that wants
// a zero weight must give the other weight the full balance instead.
func applyDefaults(cfg *Config) {
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db")
	}
	cfg.DBPath = expandTilde(cfg.DBPath)
	if cfg.AssetsPath == "" {
		cfg.AssetsPath = filepath.Join(filepath.Dir(cfg.DBPath), "assets")
	}
	cfg.AssetsPath = expandTilde(cfg.AssetsPath)
	if cfg.Embedder == "" {
		cfg.Embedder = "none"
	}
	if cfg.TopK == 0 {
		cfg.TopK = 5
	}
	if cfg.Ranker.Algorithm == "" {
		cfg.Ranker.Algorithm = DefaultRankerAlgorithm
	}
	if cfg.Ranker.BM25Weight == 0 {
		cfg.Ranker.BM25Weight = DefaultBM25Weight
	}
	if cfg.Ranker.SemanticWeight == 0 {
		cfg.Ranker.SemanticWeight = DefaultSemanticWeight
	}
}

// expandTilde expands ~ to $HOME
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
