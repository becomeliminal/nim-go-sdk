package memory

import (
	"context"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
)

// Memory is the core interface for all memory types.
// Implementations can be SDK-provided (TraceMemory) or user-defined
// (SemanticFact, ShortcutMemory, GraphRelation, etc.).
//
// Each memory type controls its own:
//   - Content structure (fields, data)
//   - Formatting for prompt injection (Format method)
//   - Metadata schema
//
// Example implementations:
//   - TraceMemory: ReAct traces (SDK-provided)
//   - SemanticFact: User facts like "Jack lives in London"
//   - ShortcutMemory: Frequently used actions
//   - GraphRelation: Links between memories
type Memory interface {
	// Identity & Ownership
	ID() string
	OwnerID() string        // User ID (empty = global memory, available to all users)
	ConversationID() string // Conversation ID (empty = not conversation-specific)
	Type() string           // Memory type identifier (e.g., "trace", "semantic", "shortcut")

	// Content & Metadata
	Content() interface{}             // Memory-specific data structure
	Metadata() map[string]interface{} // Flexible metadata for custom fields

	// Temporal
	CreatedAt() time.Time

	// Operations
	Format(ctx FormatContext) string // Formats this memory for prompt injection
	Embedding() []float32            // Vector for similarity search
	SetEmbedding([]float32)          // Set embedding vector
}

// FormatContext provides context for smart memory formatting.
// Memory.Format() implementations can use this to:
//   - Truncate based on available space (MaxLength)
//   - Customize output based on user context (UserID)
//   - Emphasize query-relevant parts (Query)
type FormatContext struct {
	UserID    string // Current user
	Query     string // Current query being answered
	MaxLength int    // Max characters for this memory's output
}

// Manager orchestrates memory operations.
// This is the main interface that Engine uses.
//
// The Engine is opinionated about WHEN to use memory (PHASE 0 retrieve, PHASE 5 record).
// The Manager is unopinionated about HOW - implementations decide:
//   - Which memories to retrieve
//   - How to format them
//   - Which traces to store
//   - How to process them
//
// Implementations:
//   - SimpleManager: Basic manager for local SDK
//   - Mem0Manager: Advanced manager with fact extraction, contradiction resolution (user-implemented)
type Manager interface {
	// Retrieve finds relevant memories for the user's message and returns formatted string.
	// The Manager decides:
	//   - How to query (vector search, filters, limits)
	//   - Which memories to include
	//   - How to format them
	//
	// Returns a formatted string ready for prompt injection.
	Retrieve(ctx context.Context, userID string, userMessage string) (string, error)

	// RecordTraces stores ReAct traces as memories.
	// The Manager decides:
	//   - Which traces are worth storing (filtering)
	//   - How to process them (importance scoring, fact extraction, etc.)
	//   - What format to store them in
	//
	// This is called asynchronously after the ReAct loop completes.
	RecordTraces(ctx context.Context, userID string, traces []*core.Trace) error

	// RecordConversation stores a conversational exchange as a memory.
	// Called after every successful agent response with the user's message
	// and the assistant's text response. Captures context from exchanges
	// that may not involve tool calls (e.g., "Faiz is my friend").
	//
	// The Manager decides:
	//   - Whether the exchange is worth storing (filtering trivial messages)
	//   - How to process it (fact extraction, importance scoring)
	//   - What format to store it in
	RecordConversation(ctx context.Context, userID string, userMessage string, assistantResponse string) error
}

// Store is the vector storage backend interface.
// Implementations: ChromemStore (local SDK), PgVectorStore (production).
type Store interface {
	// Store saves a memory with its embedding.
	// Memory must have embedding set before calling Store.
	Store(ctx context.Context, mem Memory) error

	// Query retrieves memories by vector similarity.
	// Returns memories sorted by similarity (highest first).
	Query(ctx context.Context, userID string, embedding []float32, limit int) ([]Memory, error)

	// Get retrieves a specific memory by ID and owner.
	Get(ctx context.Context, ownerID string, memoryID string) (Memory, error)

	// Delete removes a memory permanently.
	Delete(ctx context.Context, ownerID string, memoryID string) error

	// Close releases resources.
	Close() error
}

// Embedder converts text to vector embeddings.
// Implementations: MockEmbedder (testing), ONNXEmbedder (local SDK), VoyageEmbedder (production).
//
// Note: Embedder is an implementation detail of Manager.
// The Engine does not interact with Embedder directly.
type Embedder interface {
	// Embed converts a single text to embedding vector.
	Embed(ctx context.Context, text string) ([]float32, error)

	// Dimensions returns embedding vector size.
	Dimensions() int
}
