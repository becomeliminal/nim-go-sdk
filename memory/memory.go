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

// Interaction represents a complete user-agent interaction: the user's message,
// the agent's response, and any ReAct traces from tool use. Passed to Manager.Record
// so implementations have full context in a single call.
type Interaction struct {
	UserMessage       string
	AssistantResponse string
	Traces            []*core.Trace
}

// Manager orchestrates memory operations.
// This is the main interface that Engine uses.
//
// The Engine is opinionated about WHEN to use memory (PHASE 0 retrieve, PHASE 5 record).
// The Manager is unopinionated about HOW - implementations decide:
//   - Which memories to retrieve and how to format them
//   - Which traces/conversations to store and how to process them
//   - Whether to extract facts, build graphs, or just store raw data
//
// Implementations:
//   - SimpleManager: Basic manager for local SDK
//   - Mem0Manager: Advanced manager with fact extraction, knowledge graph (user-implemented)
type Manager interface {
	// Retrieve finds relevant memories for the user's message and returns formatted string.
	// The Manager decides:
	//   - How to query (vector search, filters, limits)
	//   - Which memories to include
	//   - How to format them
	//
	// Returns a formatted string ready for prompt injection.
	Retrieve(ctx context.Context, userID string, userMessage string) (string, error)

	// Record stores a complete interaction as memory.
	// Called once after each agent response with full context: the user's message,
	// the agent's response, and all ReAct traces from tool use.
	//
	// The Manager decides:
	//   - Which traces are worth storing (filtering)
	//   - Whether to store the conversation separately
	//   - How to process the interaction (fact extraction, importance scoring, etc.)
	//   - What format to store everything in
	//
	// Having traces and conversation in one call lets implementations do entity
	// resolution across both sources (e.g., matching "faiz" in user text to
	// "Faiz Abbas" from a search_users tool observation).
	Record(ctx context.Context, userID string, interaction *Interaction) error
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
