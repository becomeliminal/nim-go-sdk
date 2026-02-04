# Memory System Example

This example demonstrates the Nim Go SDK's memory system in action. The memory system enables agents to learn from past interactions and improve over time through semantic memory storage and retrieval.

## What This Example Shows

1. **Trace Memory Storage** - ReAct traces (thought-action-observation cycles) are automatically stored
2. **Vector Similarity Search** - Relevant past actions are retrieved based on semantic similarity
3. **Learning Across Conversations** - Agent remembers learnings even in new conversations
4. **Efficiency Improvements** - Agent skips redundant steps by recalling past actions

## Architecture

### Components

**SimpleManager** (`memory.SimpleManager`)
- Handles memory storage and retrieval
- Filters which traces are worth storing
- Formats memories for prompt injection
- Uses embeddings for semantic search

**ChromemStore** (`memory/store/chromem`)
- In-memory vector database using chromem-go
- Per-user memory isolation
- Fast similarity search

**ONNX Embedder** (`memory/embedder/onnx`)
- Converts text to 384-dimensional embeddings
- Uses all-MiniLM-L6-v2 model locally
- Requires ONNX Runtime

### Memory Flow

```
1. Agent executes action → Trace generated (thought, action, observation)
2. Trace stored as TraceMemory → Embedded and saved to vector store
3. New user message arrives → Embedded and used to query store
4. Similar past traces retrieved → Formatted and injected into prompt
5. Agent uses past learnings → More efficient execution
```

## How It Works

### Phase 0: Memory Retrieval

Before the agent processes a message, the memory system:
1. Embeds the user's message
2. Queries the vector store for similar past actions
3. Retrieves top N most relevant memories
4. Formats them as "RELEVANT PAST ACTIONS"
5. Injects them into the system prompt

### Phase 5: Memory Recording

After the agent completes execution, the memory system:
1. Filters traces worth storing (failures, confirmations, multi-step, etc.)
2. Creates TraceMemory objects for each trace
3. Embeds the traces using FormatForEmbedding()
4. Stores them in the vector database
5. Records are available for future retrievals

## The Example

This example demonstrates a payment assistant that learns username-to-user_id mappings:

**First Conversation:**
```
User: Send $50 to @alice
Agent:
  1. Calls search_user(@alice) → gets user_id
  2. Calls send_money(user_id, $50) → success
Result: Money sent (2 tool calls)
```

**Second Conversation (different conversation ID):**
```
User: Send $100 to @alice again
Agent:
  Memory retrieval finds: "Previously sent money to @alice (user_id: user_abc123)"
  1. Calls send_money(user_abc123, $100) → success
Result: Money sent (1 tool call - search skipped!)
```

The agent learned from the first conversation and applied that knowledge in the second, demonstrating cross-conversation memory.

## Prerequisites

1. **ONNX Runtime** - Required for local embeddings
   ```bash
   # See scripts/download-onnxruntime.sh for installation
   ```

2. **Embedding Model** - Download all-MiniLM-L6-v2
   ```bash
   # From SDK root
   ./scripts/download-model.sh
   ```

3. **Anthropic API Key**
   ```bash
   export ANTHROPIC_API_KEY="your-key-here"
   ```

## Running the Example

```bash
# From SDK root
cd examples/memory

# Build with ONNX tag (required for embedder)
go build -tags onnx -o memory-example

# Run
./memory-example
```

You should see output showing:
- Memory system setup
- First conversation with 2 tool calls
- Second conversation with 1 tool call (search skipped)
- Confirmation that memory is working

## Understanding the Output

```
Setting up memory system...
Setting up agent engine...

=== First Conversation ===
User: Send $50 to @alice
Agent: [Response showing it searched and then sent money]
Tools used: 2

=== Second Conversation (10 seconds later) ===
User: Send $100 to @alice again
Agent: [Response showing it directly sent money without searching]
Tools used: 1

✓ Memory system working!
```

The reduction from 2 to 1 tool call proves the agent remembered the username mapping.

## Configuration

Memory system is configured via `memory.Config`:

```go
memoryConfig := &memory.Config{
    Enabled:            true,  // Toggle memory on/off
    MinSimilarity:      0.5,   // Similarity threshold [0.0-1.0]
    MaxMemoriesPerUser: 1000,  // Limit per user
}
```

**Note:** `MinSimilarity` of 0.5 is reasonable for all-MiniLM-L6-v2. Production embedders (like Voyage) typically use 0.7-0.85.

## Extending This Example

### Add More Tools
Register additional tools to demonstrate memory with different actions:
```go
registry.Register(getBalanceTool)
registry.Register(analyzeSpendingTool)
```

### Custom Memory Types
Implement your own `memory.Memory` interface for different memory types:
- SemanticFact - User facts like "Jack lives in London"
- ShortcutMemory - Frequently used actions
- GraphRelation - Links between memories

### Production Store
Replace ChromemStore with a persistent store:
```go
store := pgvector.New(postgresConfig) // PostgreSQL with pgvector
```

### Production Embedder
Use a production embedder for better quality:
```go
embedder := voyage.New(voyageAPIKey) // Voyage AI embeddings
```

### Custom Manager
Implement `memory.Manager` interface for advanced features:
- Mem0-style fact extraction
- Contradiction resolution
- Graph relations
- Hierarchical memory tiers

## Memory Best Practices

1. **Filtering** - Only store valuable traces (failures, confirmations, multi-step reasoning)
2. **Importance Scoring** - Weight traces by value (failures = higher importance)
3. **User Isolation** - Use `OwnerID()` to keep memories per-user
4. **Conversation Grouping** - Use `ConversationID()` to track sessions
5. **Format Optimization** - Keep `Format()` concise to save tokens
6. **Embedding Quality** - Use production embedders for better retrieval
7. **Persistence** - Use a durable store (PostgreSQL + pgvector) in production

## Architecture Decisions

**Why Unopinionated Manager?**
- The Engine controls WHEN (retrieve in Phase 0, record in Phase 5)
- The Manager controls HOW (query logic, filtering, formatting)
- Users can implement custom memory strategies

**Why Self-Formatting Memories?**
- Each memory type knows how to present itself
- Flexible: TraceMemory formats as [Success/Failed] Action
- Extensible: Custom memory types define their own format

**Why Embedder Inside Manager?**
- Manager needs embeddings for both storage and retrieval
- Engine doesn't need to know about embeddings
- Clean separation of concerns

## Troubleshooting

**"ONNX Runtime not found"**
- Install ONNX Runtime: `scripts/download-onnxruntime.sh`
- Set library path: `export LD_LIBRARY_PATH=/path/to/onnxruntime/lib`

**"Model not found"**
- Download model: `./scripts/download-model.sh`
- Check path: `models/all-MiniLM-L6-v2/model.onnx`

**"Memory not retrieving"**
- Check `MinSimilarity` threshold (lower = more permissive)
- Verify traces are being stored (check logs)
- Ensure embeddings are generated correctly

**"Build failed: onnx package excluded"**
- Build with ONNX tag: `go build -tags onnx`
- Memory example requires ONNX embedder

## Learn More

- **Memory Architecture**: See `/memory/memory.go` for interfaces
- **Trace Memory**: See `/memory/trace.go` for implementation
- **SimpleManager**: See `/memory/manager.go` for logic
- **ChromemStore**: See `/memory/store/chromem/` for storage
- **ONNX Embedder**: See `/memory/embedder/onnx/` for embeddings
