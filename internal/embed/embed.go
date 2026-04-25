package embed

import (
	"context"

	"github.com/rwese/kb/internal/config"
)

// Embedder interface for semantic similarity
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// New creates an embedder based on config
func New(cfg *config.Config) Embedder {
	switch cfg.Embedder {
	case "ollama":
		return NewOllamaEmbedder(cfg.Ollama.Model, cfg.Ollama.BaseURL)
	default:
		return &NoneEmbedder{}
	}
}

// NoneEmbedder returns zero vectors (BM25-only mode)
type NoneEmbedder struct{}

func (e *NoneEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func (e *NoneEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

// Cosine similarity between two vectors
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (normA * normB)
}
