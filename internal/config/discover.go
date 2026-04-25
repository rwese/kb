package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBPath   string `yaml:"db_path"`
	Embedder string `yaml:"embedder"`
	Ollama   OllamaConfig `yaml:"ollama"`
	TopK     int    `yaml:"top_k"`
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
				if cfg.TopK == 0 {
					cfg.TopK = 5
				}
				return &cfg, nil
			}
		}
	}

	// Default config
	return &Config{
		DBPath:   filepath.Join(os.Getenv("HOME"), ".local", "share", "kb", "knowledgebase.db"),
		Embedder: "none",
		TopK:     5,
	}, nil
}
