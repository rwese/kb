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

func TestDiscoverRankerDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KB_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", "")

	cfg, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ranker.Algorithm != DefaultRankerAlgorithm {
		t.Fatalf("Algorithm = %q, want %q", cfg.Ranker.Algorithm, DefaultRankerAlgorithm)
	}
	if cfg.Ranker.BM25Weight != DefaultBM25Weight || cfg.Ranker.SemanticWeight != DefaultSemanticWeight {
		t.Fatalf("weights = %v/%v, want %v/%v", cfg.Ranker.BM25Weight, cfg.Ranker.SemanticWeight, DefaultBM25Weight, DefaultSemanticWeight)
	}
}

func TestDiscoverRankerFromXDGUserConfig(t *testing.T) {
	xdgHome := t.TempDir()
	configPath := filepath.Join(xdgHome, "kb", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("ranker:\n  algorithm: hybrid\n  bm25_weight: 0.5\n  semantic_weight: 0.5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("KB_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)
	t.Setenv("XDG_CONFIG_DIRS", "")

	cfg, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ranker.Algorithm != "hybrid" || cfg.Ranker.BM25Weight != 0.5 || cfg.Ranker.SemanticWeight != 0.5 {
		t.Fatalf("ranker = %+v", cfg.Ranker)
	}
}

func TestDiscoverSystemConfigDirsFallback(t *testing.T) {
	// No user config anywhere: the global system config dir supplies the
	// ranker defaults.
	sysDir := t.TempDir()
	configPath := filepath.Join(sysDir, "kb", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("ranker:\n  algorithm: bm25\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("KB_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", sysDir)

	cfg, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ranker.Algorithm != "bm25" {
		t.Fatalf("Algorithm = %q, want bm25", cfg.Ranker.Algorithm)
	}
	// Defaults still apply for everything the system config omits.
	if cfg.Ranker.BM25Weight != DefaultBM25Weight {
		t.Fatalf("BM25Weight = %v, want %v", cfg.Ranker.BM25Weight, DefaultBM25Weight)
	}
}

func TestDiscoverUserConfigBeatsSystemConfig(t *testing.T) {
	xdgHome := t.TempDir()
	userConfig := filepath.Join(xdgHome, "kb", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(userConfig), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userConfig, []byte("ranker:\n  algorithm: hybrid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sysDir := t.TempDir()
	sysConfig := filepath.Join(sysDir, "kb", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(sysConfig), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sysConfig, []byte("ranker:\n  algorithm: bm25\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("KB_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)
	t.Setenv("XDG_CONFIG_DIRS", sysDir)

	cfg, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ranker.Algorithm != "hybrid" {
		t.Fatalf("Algorithm = %q, want hybrid (user config wins)", cfg.Ranker.Algorithm)
	}
}
