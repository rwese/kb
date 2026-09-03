package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBPath     string       `yaml:"db_path"`
	AssetsPath string       `yaml:"assets_path"`
	Embedder   string       `yaml:"embedder"`
	Ollama     OllamaConfig `yaml:"ollama"`
	TopK       int          `yaml:"top_k"`
}

type OllamaConfig struct {
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
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
				cfg.DBPath = expandTilde(cfg.DBPath)
				if cfg.AssetsPath == "" {
					cfg.AssetsPath = filepath.Join(filepath.Dir(cfg.DBPath), "assets")
				}
				if cfg.TopK == 0 {
					cfg.TopK = 5
				}
				// Expand ~ in paths
				cfg.AssetsPath = expandTilde(cfg.AssetsPath)
				return &cfg, nil
			}
		}
	}

	// Default config
	return &Config{
		DBPath:     filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db"),
		AssetsPath: filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "assets"),
		Embedder:   "none",
		TopK:       5,
	}, nil
}

// expandTilde expands ~ to $HOME
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
