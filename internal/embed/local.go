package embed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/rwese/kb/internal/config"
)

// LlamaGo library bindings - using purego for CGO-free FFI
// This requires libllama_go to be compiled with C bindings

/*
#include <stdint.h>
#include <stdbool.h>

// Forward declarations for llama.cpp C API
typedef struct {
    float* data;
    int n;
} embedding_t;

typedef struct {
    void* model;
    void* context;
} llama_ctx_t;

// These would be implemented in the libllama_go shared library
// For now, we define the interface that would be called via purego
*/
import "C"

const (
	// Dimension for all-MiniLM-L6-v2 model
	EmbeddingDim = 384
)

// LocalEmbedder uses llama.cpp compiled library for embeddings
type LocalEmbedder struct {
	modelPath  string
	libraryPath string
	initialized bool
	errorMsg   string
}

// NewLocalEmbedder creates a new local embedder
func NewLocalEmbedder(cfg *config.LocalConfig) (*LocalEmbedder, error) {
	le := &LocalEmbedder{
		modelPath:  filepath.Join(cfg.CacheDir, ModelFileName),
		libraryPath: filepath.Join(cfg.CacheDir, getLibraryFileName()),
	}

	// Check if assets exist
	ok, msg := CheckAssets(cfg.CacheDir)
	if !ok {
		le.errorMsg = fmt.Sprintf("local embedder not available: %s. Run 'kb download' to fetch assets.", msg)
		return le, nil // Return with error message set
	}

	le.initialized = true
	return le, nil
}

func getLibraryFileName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libllama_go.dylib"
	case "linux":
		return "libllama_go.so"
	case "windows":
		return "llamago.dll"
	default:
		return "libllama_go.so"
	}
}

// Embed computes embedding for a single text
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if !e.initialized {
		return nil, fmt.Errorf(e.errorMsg)
	}

	// This would call into libllama_go via purego
	// For now, return an error indicating the library needs implementation
	return nil, fmt.Errorf("local embedding requires libllama_go library with C bindings - not yet implemented")
}

// EmbedBatch computes embeddings for multiple texts
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if !e.initialized {
		return nil, fmt.Errorf(e.errorMsg)
	}

	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("batch embed %d: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}

// Dimension returns the embedding vector size
func (e *LocalEmbedder) Dimension() int {
	return EmbeddingDim
}

// Close releases resources
func (e *LocalEmbedder) Close() error {
	// Cleanup llama context if needed
	return nil
}

// IsAvailable returns true if local embeddings are ready to use
func (e *LocalEmbedder) IsAvailable() bool {
	return e.initialized && e.errorMsg == ""
}

// ErrorMessage returns the error message if not available
func (e *LocalEmbedder) ErrorMessage() string {
	return e.errorMsg
}

// ModelPath returns the path to the model file
func (e *LocalEmbedder) ModelPath() string {
	return e.modelPath
}

// LibraryPath returns the path to the library file
func (e *LocalEmbedder) LibraryPath() string {
	return e.libraryPath
}

// EnsureAssets downloads required assets if missing
func EnsureAssets(cacheDir string, progress func(stage string, downloaded, total int64)) error {
	return DownloadAll(cacheDir, progress)
}

// LoadLibrary loads the llama.cpp shared library (placeholder for purego)
// In production, this would use purego.Dlsym to load functions
func LoadLibrary(path string) (unsafe.Pointer, error) {
	// This is a placeholder - actual implementation would use purego
	// purego.DlOpen(path) or similar

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("library not found: %s", path)
	}

	// For now, return nil - actual purego binding would load the library
	return nil, fmt.Errorf("purego bindings not yet implemented")
}

// nativeEmbedding computes embedding using native llama.cpp bindings
// This would be the actual implementation using purego
func nativeEmbedding(library unsafe.Pointer, modelPath, text string) ([]float32, error) {
	// Placeholder for actual llama.cpp embedding computation
	// Would involve:
	// 1. llama_load_model_from_file
	// 2. llama_new_context_with_model
	// 3. llama_model_quantize (if needed)
	// 4. llama_tokenize
	// 5. llama_forward
	// 6. llama_get_embeddings
	return nil, fmt.Errorf("native embedding not yet implemented")
}
