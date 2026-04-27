package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBPath   string        `yaml:"db_path"`
	Embedder string        `yaml:"embedder"`
	Ollama   OllamaConfig  `yaml:"ollama"`
	Local    LocalConfig   `yaml:"local"`
	TopK     int           `yaml:"top_k"`
}

type OllamaConfig struct {
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
}

type LocalConfig struct {
	Model           string  `yaml:"model"`
	CacheDir        string  `yaml:"cache_dir"`
	BM25Weight      float64 `yaml:"bm25_weight"`
	SemanticWeight  float64 `yaml:"semantic_weight"`
}

func Discover() (*Config, error) {
	searchPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".config", "kb", "config.yaml"),
		".kb.yaml",
	}

	if envPath := os.Getenv("KB_PATH"); envPath != "" {
		searchPaths = append([]string{envPath}, searchPaths...)
	}

	for _, path := range searchPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				if cfg.DBPath == "" {
					cfg.DBPath = filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db")
				}
				if cfg.TopK == 0 {
					cfg.TopK = 5
				}
				// Expand ~ in paths
				cfg.DBPath = expandTilde(cfg.DBPath)
				cfg.Local.CacheDir = expandTilde(cfg.Local.CacheDir)
				return &cfg, nil
			}
		}
	}

	// Default config
	return &Config{
		DBPath:   filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db"),
		Embedder: "none",
		TopK:     5,
		Local: LocalConfig{
			Model:           "all-MiniLM-L6-v2-Q4_K_M",
			CacheDir:        filepath.Join(os.Getenv("HOME"), ".cache", "kb"),
			BM25Weight:      0.3,
			SemanticWeight:  0.7,
		},
	}, nil
}

// expandTilde expands ~ to $HOME
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
