package chromem

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"

	"github.com/becomeliminal/nim-go-sdk/memory"
)

// ChromemStore wraps chromem-go for vector storage.
// chromem-go is a pure Go, embedded vector database.
type ChromemStore struct {
	db          *chromem.DB
	collections map[string]*chromem.Collection // Per-user collections
	mu          sync.RWMutex
}

// New creates a new chromem-based store.
func New() (*ChromemStore, error) {
	db := chromem.NewDB()

	return &ChromemStore{
		db:          db,
		collections: make(map[string]*chromem.Collection),
	}, nil
}

// getOrCreateCollection returns the collection for a user.
// Each user gets their own collection for namespace isolation.
func (s *ChromemStore) getOrCreateCollection(userID string) (*chromem.Collection, error) {
	s.mu.RLock()
	col, exists := s.collections[userID]
	s.mu.RUnlock()

	if exists {
		return col, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if col, exists := s.collections[userID]; exists {
		return col, nil
	}

	// Create new collection for this user
	collectionName := fmt.Sprintf("user_%s", userID)
	if userID == "" {
		collectionName = "global" // Global memories
	}

	col, err := s.db.CreateCollection(
		collectionName,
		nil, // No custom embedding func (we provide embeddings)
		nil, // No custom distance func (use default cosine)
	)
	if err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	s.collections[userID] = col
	return col, nil
}

// Store saves a memory with its embedding.
func (s *ChromemStore) Store(ctx context.Context, mem memory.Memory) error {
	col, err := s.getOrCreateCollection(mem.OwnerID())
	if err != nil {
		return err
	}

	log.Printf("[CHROMEM] Storing memory: id=%s, owner=%s, type=%s",
		mem.ID(), mem.OwnerID(), mem.Type())

	// Serialize memory for storage
	stored, err := serializeMemory(mem)
	if err != nil {
		return fmt.Errorf("serialize memory: %w", err)
	}

	// Create chromem document
	doc := chromem.Document{
		ID:        mem.ID(),
		Content:   stored.ContentJSON,
		Embedding: mem.Embedding(),
		Metadata:  stored.Metadata,
	}

	err = col.AddDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("add document: %w", err)
	}

	return nil
}

// Query retrieves memories by vector similarity.
func (s *ChromemStore) Query(ctx context.Context, userID string, embedding []float32, limit int) ([]memory.Memory, error) {
	col, err := s.getOrCreateCollection(userID)
	if err != nil {
		return nil, err
	}

	log.Printf("[CHROMEM] Querying collection for owner=%s, limit=%d", userID, limit)

	// Build where clause for filtering
	where := map[string]string{
		"owner_id": userID,
	}

	// Query chromem with embedding
	// chromem-go requires nResults <= collection size
	// Retry with smaller limits if necessary
	var results []chromem.Result
	for currentLimit := limit; currentLimit >= 1; currentLimit-- {
		var err error
		results, err = col.QueryEmbedding(ctx, embedding, currentLimit, where, nil)
		if err == nil {
			break
		}

		// Check if error is due to insufficient documents
		if isInsufficientDocsError(err) {
			if currentLimit == 1 {
				// Collection is empty
				log.Printf("[CHROMEM] Collection is empty")
				return nil, nil
			}
			continue
		}

		// Some other error
		return nil, fmt.Errorf("chromem query: %w", err)
	}

	log.Printf("[CHROMEM] Retrieved %d raw results", len(results))

	// Convert and filter results
	var memories []memory.Memory
	for i, result := range results {
		// Deserialize memory
		mem, err := deserializeMemory(result)
		if err != nil {
			log.Printf("[CHROMEM] Skipping result #%d: %v", i+1, err)
			continue
		}

		memories = append(memories, mem)
	}

	log.Printf("[CHROMEM] Returning %d memories", len(memories))
	return memories, nil
}

// Get retrieves a specific memory by ID and owner.
func (s *ChromemStore) Get(ctx context.Context, ownerID string, memoryID string) (memory.Memory, error) {
	// chromem-go doesn't have a direct Get by ID
	// We'd need to query with a dummy embedding and filter by ID
	// For now, return not supported
	return nil, fmt.Errorf("Get not supported in chromem store (use Query instead)")
}

// Delete removes a memory.
func (s *ChromemStore) Delete(ctx context.Context, ownerID string, memoryID string) error {
	// Note: chromem-go doesn't expose direct delete by ID in current API
	// For local version, this is acceptable (memories decay naturally)
	log.Printf("[CHROMEM] Delete not supported (chromem-go limitation)")
	return nil
}

// Close releases resources.
func (s *ChromemStore) Close() error {
	// chromem-go keeps everything in memory, nothing to close
	return nil
}

// StoredMemory represents a serialized memory for storage.
type StoredMemory struct {
	Type        string
	ContentJSON string
	Metadata    map[string]string
}

// serializeMemory converts a Memory interface to storage format.
func serializeMemory(mem memory.Memory) (*StoredMemory, error) {
	// Serialize content
	contentBytes, err := json.Marshal(mem.Content())
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}

	// Prepare metadata
	metadata := map[string]string{
		"type":            mem.Type(),
		"owner_id":        mem.OwnerID(),
		"conversation_id": mem.ConversationID(),
		"created_at":      mem.CreatedAt().Format(time.RFC3339),
	}

	// Add custom metadata
	for k, v := range mem.Metadata() {
		if str, ok := v.(string); ok {
			metadata[k] = str
		} else {
			// Convert to JSON for non-string values
			if bytes, err := json.Marshal(v); err == nil {
				metadata[k] = string(bytes)
			}
		}
	}

	return &StoredMemory{
		Type:        mem.Type(),
		ContentJSON: string(contentBytes),
		Metadata:    metadata,
	}, nil
}

// deserializeMemory converts stored format back to Memory interface.
func deserializeMemory(result chromem.Result) (memory.Memory, error) {
	memType := result.Metadata["type"]

	// Deserialize based on type
	switch memType {
	case "trace":
		return deserializeTraceMemory(result)
	default:
		// Unknown type - return a generic memory wrapper
		return nil, fmt.Errorf("unknown memory type: %s", memType)
	}
}

// deserializeTraceMemory deserializes a TraceMemory from chromem result.
func deserializeTraceMemory(result chromem.Result) (*memory.TraceMemory, error) {
	// Parse content
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content), &content); err != nil {
		return nil, fmt.Errorf("unmarshal content: %w", err)
	}

	// Extract fields
	thought, _ := content["thought"].(string)
	action, _ := content["action"].(string)
	observation, _ := content["observation"].(string)
	success, _ := content["success"].(bool)

	// Parse timestamps
	createdAt, _ := time.Parse(time.RFC3339, result.Metadata["created_at"])

	// Parse metadata
	metadata := make(map[string]interface{})
	for k, v := range result.Metadata {
		if k != "type" && k != "owner_id" && k != "conversation_id" && k != "created_at" {
			metadata[k] = v
		}
	}

	// Create TraceMemory using storage constructor
	return memory.NewTraceMemoryFromStorage(
		result.ID,
		result.Metadata["owner_id"],
		result.Metadata["conversation_id"],
		createdAt,
		result.Embedding,
		thought,
		action,
		observation,
		success,
		metadata,
	), nil
}

// isInsufficientDocsError checks if error is due to insufficient documents.
func isInsufficientDocsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "nResults must be") || contains(errStr, "number of documents")
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
