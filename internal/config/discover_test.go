package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDefaultsAssetsPathToDBSibling(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("db_path: ~/kb-test/knowledgebase.db\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	oldKBPath := os.Getenv("KB_PATH")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("KB_PATH", oldKBPath)
	})

	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("KB_PATH", configPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := Discover()
	if err != nil {
		t.Fatal(err)
	}

	wantDB := filepath.Join(tmpDir, "kb-test", "knowledgebase.db")
	wantAssets := filepath.Join(tmpDir, "kb-test", "assets")
	if cfg.DBPath != wantDB {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, wantDB)
	}
	if cfg.AssetsPath != wantAssets {
		t.Fatalf("AssetsPath = %q, want %q", cfg.AssetsPath, wantAssets)
	}
}
