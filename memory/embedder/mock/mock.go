package mock

import (
	"context"
	"hash/fnv"
	"math"
)

// MockEmbedder is a simple mock embedder for testing.
// It generates deterministic embeddings based on text hash.
type MockEmbedder struct {
	dimensions int
}

// New creates a new mock embedder.
func New() *MockEmbedder {
	return &MockEmbedder{
		dimensions: 384, // Match all-MiniLM-L6-v2 dimensions
	}
}

// Embed creates a deterministic embedding from text.
// Uses hash-based generation for consistent results.
func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Hash the text
	h := fnv.New64a()
	h.Write([]byte(text))
	hash := h.Sum64()

	// Generate deterministic embedding
	embedding := make([]float32, m.dimensions)

	// Use hash as seed for pseudo-random generation
	seed := hash
	for i := 0; i < m.dimensions; i++ {
		// Simple LCG (Linear Congruential Generator)
		seed = seed*6364136223846793005 + 1442695040888963407
		// Convert to [-1, 1] range
		val := float32(int64(seed)) / float32(math.MaxInt64)
		embedding[i] = val
	}

	// Normalize to unit vector
	embedding = normalize(embedding)

	return embedding, nil
}

// Dimensions returns the embedding size.
func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

// normalize converts embedding to unit vector.
func normalize(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}

	if norm == 0 {
		return vec
	}

	norm = float32(math.Sqrt(float64(norm)))
	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}

	return normalized
}
