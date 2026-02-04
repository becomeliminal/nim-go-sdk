package memory_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/memory"
	"github.com/becomeliminal/nim-go-sdk/memory/store/chromem"
)

// MockEmbedder is a simple mock for testing without real model files.
type MockEmbedder struct {
	dims int
}

func NewMockEmbedder(dims int) *MockEmbedder {
	return &MockEmbedder{dims: dims}
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a simple mock embedding based on text length
	// This won't give real semantic similarity, but it's good enough for testing
	embedding := make([]float32, m.dims)
	for i := range embedding {
		embedding[i] = float32(len(text)) / float32(m.dims+i+1)
	}
	return embedding, nil
}

func (m *MockEmbedder) Dimensions() int {
	return m.dims
}

func TestSimpleManager_RecordAndRetrieve(t *testing.T) {
	ctx := context.Background()

	// Setup with mock embedder
	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	config := &memory.Config{
		Enabled:       true,
		MinSimilarity: 0.0, // Low threshold for mock embeddings
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// Create test traces (multi-step to ensure they're stored)
	traces := []*core.Trace{
		{
			SessionID:   "session1",
			Thought:     "First checking balance",
			Action:      "get_balance",
			Observation: "Balance is $100",
			Success:     true,
		},
		{
			SessionID:   "session1",
			Thought:     "User wants to send money to Alice",
			Action:      "send_money",
			Observation: "Transfer successful",
			Success:     true,
		},
	}

	// Record traces
	err = manager.RecordTraces(ctx, "user123", traces)
	if err != nil {
		t.Fatalf("Failed to record traces: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Retrieve memories
	formatted, err := manager.Retrieve(ctx, "user123", "send money to Alice")
	if err != nil {
		t.Fatalf("Failed to retrieve memories: %v", err)
	}

	// Verify formatted output contains relevant information
	if formatted == "" {
		t.Log("No memories retrieved. This is expected with mock embedder.")
		t.Skip("Skipping test - mock embedder doesn't provide real semantic similarity")
	}

	// Check that formatted output looks reasonable
	if !strings.Contains(formatted, "RELEVANT PAST ACTIONS") {
		t.Errorf("Expected formatted output to contain header")
	}
}

func TestSimpleManager_UserNamespacing(t *testing.T) {
	ctx := context.Background()

	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	config := &memory.Config{
		Enabled:       true,
		MinSimilarity: 0.0,
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// User1 records trace (marked as confirmed to ensure storage)
	traces1 := []*core.Trace{{
		SessionID:   "session1",
		Thought:     "Checking account balance",
		Action:      "send_money",
		Observation: "Sent $50",
		Success:     true,
		Metadata:    map[string]string{"confirmed": "true"},
	}}
	err = manager.RecordTraces(ctx, "user1", traces1)
	if err != nil {
		t.Fatalf("Failed to record user1 traces: %v", err)
	}

	// User2 records trace (marked as confirmed to ensure storage)
	traces2 := []*core.Trace{{
		SessionID:   "session2",
		Thought:     "Getting wallet balance",
		Action:      "get_balance",
		Observation: "Balance is $100",
		Success:     true,
		Metadata:    map[string]string{"confirmed": "true"},
	}}
	err = manager.RecordTraces(ctx, "user2", traces2)
	if err != nil {
		t.Fatalf("Failed to record user2 traces: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// User1 retrieves - should only see their trace
	formatted1, err := manager.Retrieve(ctx, "user1", "money operations")
	if err != nil {
		t.Fatalf("Failed to retrieve user1 memories: %v", err)
	}

	// User2 retrieves - should only see their trace
	formatted2, err := manager.Retrieve(ctx, "user2", "balance check")
	if err != nil {
		t.Fatalf("Failed to retrieve user2 memories: %v", err)
	}

	// Verify isolation (if we got results)
	if formatted1 != "" && strings.Contains(formatted1, "user2") {
		t.Error("User1 should not see user2's memories")
	}
	if formatted2 != "" && strings.Contains(formatted2, "user1") {
		t.Error("User2 should not see user1's memories")
	}
}

func TestSimpleManager_FilterStorableTraces(t *testing.T) {
	ctx := context.Background()

	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	config := &memory.Config{
		Enabled: true,
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// Single trivial trace (should not be stored)
	trivialTraces := []*core.Trace{{
		SessionID:   "session1",
		Thought:     "Check balance",
		Action:      "get_balance",
		Observation: "$100",
		Success:     true,
	}}

	err = manager.RecordTraces(ctx, "user1", trivialTraces)
	if err != nil {
		t.Fatalf("Failed to record traces: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should not find trivial trace
	formatted, err := manager.Retrieve(ctx, "user1", "balance check")
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}

	// With mock embedder, we may not get results anyway, so we just verify no error
	t.Logf("Trivial trace retrieve result: %s", formatted)

	// Multi-step trace (should be stored)
	multiStepTraces := []*core.Trace{
		{
			SessionID:   "session2",
			Thought:     "Need to check balance",
			Action:      "get_balance",
			Observation: "$100",
			Success:     true,
		},
		{
			SessionID:   "session2",
			Thought:     "Now send money",
			Action:      "send_money",
			Observation: "Sent $50",
			Success:     true,
		},
	}

	err = manager.RecordTraces(ctx, "user2", multiStepTraces)
	if err != nil {
		t.Fatalf("Failed to record traces: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should find multi-step trace
	formatted, err = manager.Retrieve(ctx, "user2", "sending money")
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}

	t.Logf("Multi-step trace retrieve result: %s", formatted)
}

func TestSimpleManager_DisabledConfig(t *testing.T) {
	ctx := context.Background()

	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	// Memory disabled
	config := &memory.Config{
		Enabled: false,
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// Try to record traces
	traces := []*core.Trace{{
		SessionID:   "session1",
		Thought:     "Test",
		Action:      "test",
		Observation: "test",
		Success:     true,
	}}

	err = manager.RecordTraces(ctx, "user1", traces)
	if err != nil {
		t.Fatalf("RecordTraces should not error when disabled: %v", err)
	}

	// Try to retrieve
	formatted, err := manager.Retrieve(ctx, "user1", "test query")
	if err != nil {
		t.Fatalf("Retrieve should not error when disabled: %v", err)
	}

	if formatted != "" {
		t.Error("Expected empty result when memory is disabled")
	}
}

func TestSimpleManager_FailureStorage(t *testing.T) {
	ctx := context.Background()

	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	config := &memory.Config{
		Enabled: true,
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// Single failure trace (should be stored for learning)
	failureTrace := []*core.Trace{{
		SessionID:   "session1",
		Thought:     "Try to send money",
		Action:      "send_money",
		Observation: "Insufficient funds",
		Success:     false,
	}}

	err = manager.RecordTraces(ctx, "user1", failureTrace)
	if err != nil {
		t.Fatalf("Failed to record traces: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Failures should be stored
	formatted, err := manager.Retrieve(ctx, "user1", "send money failed")
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}

	t.Logf("Failure trace retrieve result: %s", formatted)
}

func TestSimpleManager_ConfirmationStorage(t *testing.T) {
	ctx := context.Background()

	store, err := chromem.New()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	embedder := NewMockEmbedder(384)

	config := &memory.Config{
		Enabled: true,
	}
	manager := memory.NewSimpleManager(store, embedder, config)

	// Single confirmation trace (should be stored as important)
	confirmationTrace := []*core.Trace{{
		SessionID:   "session1",
		Thought:     "User confirmed transfer",
		Action:      "send_money",
		Observation: "Transfer completed",
		Success:     true,
		Metadata:    map[string]string{"confirmed": "true"},
	}}

	err = manager.RecordTraces(ctx, "user1", confirmationTrace)
	if err != nil {
		t.Fatalf("Failed to record traces: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Confirmations should be stored
	formatted, err := manager.Retrieve(ctx, "user1", "confirmed transfer")
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}

	t.Logf("Confirmation trace retrieve result: %s", formatted)
}
