package memory

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/becomeliminal/nim-go-sdk/core"
)

// SimpleManager is the SDK-provided Manager implementation.
// It provides basic memory operations suitable for local development.
//
// Features:
//   - Vector similarity search
//   - Automatic embedding
//   - Memory formatting
//   - Trace filtering
//
// For production, users can implement custom Manager with:
//   - Mem0-style fact extraction
//   - Contradiction resolution
//   - Graph relations
//   - Hierarchical memory tiers
type SimpleManager struct {
	store    Store
	embedder Embedder // Internal: Engine never sees this
	config   *Config
}

// NewSimpleManager creates a new SimpleManager.
func NewSimpleManager(store Store, embedder Embedder, config *Config) *SimpleManager {
	if config == nil {
		config = DefaultConfig
	}
	return &SimpleManager{
		store:    store,
		embedder: embedder,
		config:   config,
	}
}

// Retrieve finds relevant memories and returns formatted string.
func (m *SimpleManager) Retrieve(ctx context.Context, userID string, userMessage string) (string, error) {
	if !m.config.Enabled {
		return "", nil // Memory disabled
	}

	// Embed query
	embedding, err := m.embedder.Embed(ctx, userMessage)
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}

	// Query store for top 10 memories
	memories, err := m.store.Query(ctx, userID, embedding, 10)
	if err != nil {
		return "", fmt.Errorf("query store: %w", err)
	}

	// Log retrieval
	log.Printf("[MEMORY] Retrieved %d memories for query: %q", len(memories), truncateLog(userMessage, 50))
	if len(memories) == 0 {
		log.Printf("[MEMORY]   No memories found")
		return "", nil
	}

	// Format memories
	return m.formatMemories(memories, userID, userMessage), nil
}

// Record stores a complete interaction as memory.
// SimpleManager stores filtered traces only; conversation storage is a no-op.
// Custom implementations (e.g., Mem0Manager) can store conversations and extract facts.
func (m *SimpleManager) Record(ctx context.Context, userID string, interaction *Interaction) error {
	if !m.config.Enabled {
		return nil // Memory disabled
	}

	// Filter traces worth storing
	storableTraces := m.filterStorableTraces(interaction.Traces)
	if len(storableTraces) == 0 {
		log.Printf("[MEMORY] No traces worth storing (filtered out)")
		return nil
	}

	log.Printf("[MEMORY] Recording %d traces (filtered from %d)", len(storableTraces), len(interaction.Traces))

	// Convert traces to memories and embed them
	for i, trace := range storableTraces {
		// Create TraceMemory
		mem := NewTraceMemory(userID, trace.SessionID, trace)

		// Format memory for embedding
		text := mem.FormatForEmbedding()

		// Generate embedding
		embedding, err := m.embedder.Embed(ctx, text)
		if err != nil {
			log.Printf("[MEMORY] Failed to embed trace #%d: %v", i+1, err)
			continue
		}
		mem.SetEmbedding(embedding)

		// Store
		if err := m.store.Store(ctx, mem); err != nil {
			log.Printf("[MEMORY] Failed to store trace #%d: %v", i+1, err)
			continue
		}

		log.Printf("[MEMORY]   Stored trace #%d: action=%s", i+1, trace.Action)
	}

	return nil
}

// formatMemories formats retrieved memories into a structured string.
func (m *SimpleManager) formatMemories(memories []Memory, userID string, query string) string {
	if len(memories) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "=== RELEVANT PAST ACTIONS ===\n")

	// Calculate max length per memory
	maxLengthPerMemory := 2000 / len(memories)
	if maxLengthPerMemory < 100 {
		maxLengthPerMemory = 100 // Minimum reasonable length
	}

	// Format each memory
	for i, mem := range memories {
		formatted := mem.Format(FormatContext{
			UserID:    userID,
			Query:     query,
			MaxLength: maxLengthPerMemory,
		})
		parts = append(parts, fmt.Sprintf("%d. %s\n", i+1, formatted))
	}

	return strings.Join(parts, "\n")
}

// filterStorableTraces selects traces worth storing.
// SimpleManager's filtering logic - user implementations can define their own.
func (m *SimpleManager) filterStorableTraces(traces []*core.Trace) []*core.Trace {
	// Store multi-step traces (both successes and failures)
	if len(traces) > 1 {
		return traces
	}

	// For single trace, check if it's worth storing
	if len(traces) == 1 {
		trace := traces[0]

		// Store failures (for learning)
		if !trace.Success {
			return traces
		}

		// Store confirmations (important actions)
		if trace.Metadata != nil {
			if trace.Metadata["confirmed"] == "true" {
				return traces
			}
		}

		// Store contextually valuable actions
		contextualActions := []string{
			"search_users",     // User relationships
			"get_profile",      // User preferences/info
			"get_transactions", // Spending patterns
			"analyze_spending", // Financial insights
		}
		for _, action := range contextualActions {
			if trace.Action == action {
				return traces
			}
		}

		// Store traces with substantive thoughts (>30 chars indicates reasoning)
		if len(trace.Thought) > 30 {
			return traces
		}

		// Skip simple balance checks and other trivial reads
	}

	return nil
}

// truncateLog truncates text for logging.
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Config holds SimpleManager configuration.
type Config struct {
	// Enabled toggles memory system on/off.
	// Default: false (opt-in for local testing).
	Enabled bool

	// MinSimilarity is the minimum similarity for retrieval [0.0-1.0].
	// Default: 0.5
	// Note: Tiny models (all-MiniLM-L6-v2) produce lower scores (~0.35 for similar text)
	// Production models (Voyage) produce higher scores (0.7-0.85 range)
	MinSimilarity float64

	// MaxMemoriesPerUser caps total memories per user.
	// Default: 1000 (prevents unbounded growth).
	MaxMemoriesPerUser int

	// DecayEnabled toggles Ebbinghaus forgetting curve.
	// Default: false (not implemented in local version).
	DecayEnabled bool
}

// DefaultConfig returns sensible defaults for local SDK.
var DefaultConfig = &Config{
	Enabled:            false, // Opt-in
	MinSimilarity:      0.5,   // Reasonable for most embedders
	MaxMemoriesPerUser: 1000,
	DecayEnabled:       false, // Skip decay for local version
}
