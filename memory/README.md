# Memory System

The memory system enables agents to learn from past interactions by storing and retrieving ReAct traces (Thought-Action-Observation cycles) using semantic vector search.

## Design Philosophy

**"Unopinionated implementation, opinionated flow"**

- **The Engine is opinionated about WHEN**: Memory retrieval happens in PHASE 0 (before execution), memory recording happens in PHASE 5 (after execution)
- **The Manager is unopinionated about HOW**: Implementations decide which memories to retrieve, how to format them, which traces to store, and how to process them

This design enables maximum flexibility while ensuring consistent integration.

## Architecture

### Interface-Based Design

The memory system uses interface-based architecture for maximum extensibility:

```go
// Memory is the core interface - any memory type must implement this
type Memory interface {
    ID() string
    OwnerID() string        // User ID (empty = global memory)
    ConversationID() string // Conversation ID (empty = not specific)
    Type() string           // "trace", "semantic", "shortcut", etc.

    Content() interface{}
    Metadata() map[string]interface{}
    CreatedAt() time.Time

    Format(ctx FormatContext) string  // Self-formatting!
    Embedding() []float32
    SetEmbedding([]float32)
}

// Manager orchestrates memory operations
type Manager interface {
    // Manager decides HOW to retrieve
    Retrieve(ctx context.Context, userID string, userMessage string) (string, error)

    // Manager decides WHAT to store
    RecordTraces(ctx context.Context, userID string, traces []*core.Trace) error
}
```

### Components

**Memory** (Interface)
- Core abstraction for all memory types
- Self-formatting via `Format(FormatContext)` method
- SDK provides: TraceMemory (ReAct traces)
- Users can implement: SemanticFact, ShortcutMemory, GraphRelation, etc.

**Manager** (Interface)
- Simple API: `Retrieve()` and `RecordTraces()`
- SDK provides: SimpleManager (basic filtering and formatting)
- Users can implement: Mem0Manager (fact extraction, contradiction resolution)

**Store** (Interface)
- Vector storage backend for memories
- Works with Memory interface
- SDK provides: ChromemStore (chromem-go, in-memory)
- Users can implement: PgVectorStore (PostgreSQL + pgvector)

**Embedder** (Internal to Manager)
- Text-to-vector conversion for semantic search
- SDK provides: ONNXEmbedder (all-MiniLM-L6-v2, 384 dims)
- Users can implement: VoyageEmbedder (Voyage AI, 1024 dims)

## Quick Start

### 1. Download Model Files (ONNX only)

```bash
./scripts/download-model.sh
```

This downloads the all-MiniLM-L6-v2 model (~80MB) for local embeddings.

**Note:** ONNX embedder is optional. Build with `-tags onnx` to use it.

### 2. Setup Memory System

```go
import (
    "github.com/becomeliminal/nim-go-sdk/memory"
    "github.com/becomeliminal/nim-go-sdk/memory/store/chromem"
    "github.com/becomeliminal/nim-go-sdk/memory/embedder/onnx"
)

// Create store (works with Memory interface)
store, err := chromem.New()
if err != nil {
    log.Fatal(err)
}

// Create embedder
embedder, err := onnx.New(onnx.Config{
    ModelPath:     "models/all-MiniLM-L6-v2/model.onnx",
    TokenizerPath: "models/all-MiniLM-L6-v2/tokenizer.json",
    Dimensions:    384,
})
if err != nil {
    log.Fatal(err)
}
defer embedder.Close()

// Create manager (SimpleManager is SDK-provided)
config := &memory.Config{
    Enabled:       true,
    MinSimilarity: 0.5, // Standard threshold for embeddings
}
memoryMgr := memory.NewSimpleManager(store, embedder, config)
```

### 3. Integrate with Engine

```go
import "github.com/becomeliminal/nim-go-sdk/engine"

eng := engine.NewEngine(&client, registry,
    engine.WithMemory(memoryMgr),
)
```

### 4. Run Agent

The engine automatically handles memory through the Manager interface:

**PHASE 0: RETRIEVE** (before execution)
```go
output, err := eng.Run(ctx, &engine.Input{
    UserMessage: "Send $100 to @alice",
    Context: &core.Context{
        UserID: "user123",
    },
})

// Engine calls: memoryMgr.Retrieve(ctx, "user123", "Send $100 to @alice")
// Manager decides HOW:
//   - Embeds the message
//   - Queries store for similar memories
//   - Filters and formats memories
//   - Returns formatted string for prompt injection
```

**PHASE 5: RECORD** (after execution)
```go
// Engine calls: memoryMgr.RecordTraces(ctx, "user123", session.Traces)
// Manager decides WHAT to store:
//   - Filters traces (multi-step, failures, confirmations)
//   - Converts to TraceMemory objects
//   - Embeds them
//   - Stores in vector database
// This happens async (non-blocking)
```

## Example

See `examples/memory/main.go` for a complete example showing:
1. First conversation: Agent searches for @alice, finds user_abc123
2. Second conversation: Agent remembers @alice = user_abc123, skips search

## Configuration

```go
config := &memory.Config{
    // Enable memory system (default: false)
    Enabled: true,

    // Minimum similarity for retrieval (default: 0.5)
    // Range: 0.0-1.0, higher = stricter matching
    MinSimilarity: 0.5,

    // Max memories per user (default: 1000)
    MaxMemoriesPerUser: 1000,

    // Enable decay (default: false, not implemented in local version)
    DecayEnabled: false,
}
```

## User Isolation

**Critical:** All memories are namespaced by `OwnerID()` for multi-user support.

```go
// User A's memories
memories, _ := memoryMgr.Retrieve(ctx, "userA", "send money to alice")
memoryMgr.RecordTraces(ctx, "userA", traces)

// User B's memories (completely isolated)
memories, _ := memoryMgr.Retrieve(ctx, "userB", "send money to alice")
memoryMgr.RecordTraces(ctx, "userB", traces)
```

**Store-Level Isolation:**
- ChromemStore creates per-user collections
- Each user's memories are stored separately
- Queries are filtered by `owner_id` metadata
- Users can never access each other's memories

**Global Memories:**
- Set `OwnerID()` to empty string for global memories
- Available to all users (e.g., system knowledge, FAQs)

## Memory Filtering

SimpleManager filters traces to avoid clutter:

**Stored:**
- ✅ Multi-step traces (complex reasoning)
- ✅ Failures (for learning)
- ✅ Confirmations (important actions)
- ✅ Contextually valuable actions (user searches, transactions)
- ✅ Traces with substantive thoughts (>30 chars)

**Skipped:**
- ❌ Single trivial reads (e.g., lone balance check)

**Custom Filtering:**
Implement your own Manager to define custom filtering logic:

```go
func (m *MyManager) RecordTraces(ctx context.Context, userID string, traces []*core.Trace) error {
    // Your custom filtering logic here
    // Only store traces that match your criteria
}
```

## Extending the System

### Custom Memory Types

Implement the `Memory` interface for custom memory types:

```go
// SemanticFact stores user facts like "Jack lives in London"
type SemanticFact struct {
    id             string
    ownerID        string
    createdAt      time.Time
    embedding      []float32
    Fact           string  // "Jack lives in London"
    Confidence     float64 // 0.0-1.0
    Source         string  // Where this came from
}

func (f *SemanticFact) Format(ctx FormatContext) string {
    return fmt.Sprintf("Fact: %s (confidence: %.0f%%)", f.Fact, f.Confidence*100)
}

// Implement other Memory interface methods...
```

### Custom Manager

Implement the `Manager` interface for advanced features:

```go
type Mem0Manager struct {
    store    Store
    embedder Embedder
    llm      *anthropic.Client  // For fact extraction
}

func (m *Mem0Manager) Retrieve(ctx context.Context, userID string, userMessage string) (string, error) {
    // Custom retrieval logic:
    // 1. Query for traces AND semantic facts
    // 2. Apply custom ranking algorithm
    // 3. Format with custom template
    // 4. Return formatted string
}

func (m *Mem0Manager) RecordTraces(ctx context.Context, userID string, traces []*core.Trace) error {
    // Custom recording logic:
    // 1. Store traces as TraceMemory
    // 2. Extract facts using LLM
    // 3. Detect contradictions
    // 4. Build graph relations
    // 5. Store everything
}
```

### Custom Store

Implement the `Store` interface for custom backends:

```go
type RedisStore struct {
    client *redis.Client
}

func (s *RedisStore) Store(ctx context.Context, mem Memory) error {
    // Serialize Memory and store in Redis
}

func (s *RedisStore) Query(ctx context.Context, userID string, embedding []float32, limit int) ([]Memory, error) {
    // Use RediSearch vector similarity
    // Deserialize and return memories
}
```

## Production Migration

For production deployment, swap implementations:

### Store: chromem → pgvector

```go
// Local
store, err := chromem.New()

// Production
store, err := pgvector.New(&pgvector.Config{
    ConnectionString: "postgres://...",
})
```

### Embedder: ONNX → Voyage

```go
// Local
embedder, err := onnx.New(onnx.Config{
    ModelPath:     "models/all-MiniLM-L6-v2/model.onnx",
    TokenizerPath: "models/all-MiniLM-L6-v2/tokenizer.json",
    Dimensions:    384,
})

// Production
embedder, err := voyage.New(&voyage.Config{
    APIKey:     os.Getenv("VOYAGE_API_KEY"),
    Model:      "voyage-finance-2",
    Dimensions: 1024,
})
```

### Differences

| Feature | Local (chromem + ONNX) | Production (pgvector + Voyage) |
|---------|------------------------|-------------------------------|
| **Storage** | In-memory | PostgreSQL (persistent) |
| **Embedding Dims** | 384 | 1024 |
| **Latency** | 10-50ms | 100-500ms |
| **Quality** | Good | Excellent |
| **Cost** | Free | ~$0.10 per 1M tokens |
| **Offline** | ✅ Yes | ❌ No (API required) |

## Testing

Run tests:

```bash
go test ./memory/...
```

Tests use mock embedders (no model files required).

## Performance

**RETRIEVE Phase:**
- One-time cost before execution
- ~10-50ms for embedding + vector search
- Top 5 memories retrieved

**RECORD Phase:**
- Async (non-blocking)
- Runs in goroutine after response returned
- ~10-50ms per trace

## Troubleshooting

### Model files not found

```
Error: read vocab: open models/all-MiniLM-L6-v2/tokenizer.json: no such file
```

**Solution:** Run `./scripts/download-model.sh`

### Out of memory

If storing too many memories:

```go
config := &memory.Config{
    MaxMemoriesPerUser: 500, // Reduce cap
}
```

### Low similarity scores

If retrieval returns irrelevant memories:

```go
config := &memory.Config{
    MinSimilarity: 0.7, // Increase threshold
}
```

## Architecture Benefits

**Why Interface-Based?**
1. **Maximum Extensibility** - Users can implement custom memory types without modifying SDK
2. **Self-Formatting** - Each memory type knows how to present itself
3. **Simple Manager API** - Just 2 methods: Retrieve() and RecordTraces()
4. **Clean Separation** - Engine controls WHEN, Manager controls HOW

**Why Self-Formatting Memories?**
- TraceMemory formats as: `[Success/Failed] Action`
- SemanticFact formats as: `Fact: ... (confidence: %)`
- Custom types define their own format
- Flexible and extensible

**Why Embedder Inside Manager?**
- Manager needs embeddings for both storage and retrieval
- Engine doesn't need to know about embeddings
- Clean separation of concerns
- Implementation detail

## Future Enhancements

For production implementations:

**Memory Types:**
- [ ] SemanticFact - Extracted user facts
- [ ] ShortcutMemory - Frequently used actions
- [ ] GraphRelation - Links between memories
- [ ] SpendingPattern - Financial behavior patterns

**Manager Features:**
- [ ] Mem0-style fact extraction
- [ ] Contradiction detection and resolution
- [ ] Memory decay (Ebbinghaus forgetting curve)
- [ ] Memory promotion (episodic → semantic)
- [ ] Pattern extraction
- [ ] Importance-based ranking

**Compliance:**
- [ ] GDPR compliance (right to be forgotten)
- [ ] Encryption for PII
- [ ] Audit logging
- [ ] Data retention policies

## Build Tags

The ONNX embedder is optional to reduce dependencies:

```bash
# Build without ONNX (default)
go build ./...

# Build with ONNX
go build -tags onnx ./...
```

Core SDK dependencies: 13 modules (without ONNX)

## License

Same as nim-go-sdk.
