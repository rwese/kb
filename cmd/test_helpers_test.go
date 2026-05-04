package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type testEnv struct {
	ConfigPath string
	DBPath     string
	AssetsPath string
}

func setupTempKBTestEnv(t *testing.T) testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "knowledgebase.db")
	assetsPath := filepath.Join(tmpDir, "data", "assets")
	configPath := filepath.Join(tmpDir, "config.yaml")

	configData := fmt.Sprintf("db_path: %s\nassets_path: %s\nembedder: none\ntop_k: 5\n", dbPath, assetsPath)
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	t.Setenv("KB_PATH", configPath)
	t.Setenv("HOME", tmpDir)

	return testEnv{
		ConfigPath: configPath,
		DBPath:     dbPath,
		AssetsPath: assetsPath,
	}
}
