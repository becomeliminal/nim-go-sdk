package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/google/uuid"
)

// TraceMemory stores a ReAct trace (thought-action-observation cycle).
// This is the SDK-provided implementation of the Memory interface.
//
// TraceMemory is used to store agent reasoning patterns so the agent
// can learn from past actions and improve over time.
type TraceMemory struct {
	id             string
	ownerID        string
	conversationID string
	createdAt      time.Time
	embedding      []float32
	importance     float64
	metadata       map[string]interface{}

	// Trace-specific fields
	Thought     string
	Action      string
	Observation string
	Success     bool
}

// NewTraceMemory creates a TraceMemory from a core.Trace.
func NewTraceMemory(ownerID string, conversationID string, trace *core.Trace) *TraceMemory {
	// Assess importance
	importance := assessTraceImportance(trace)

	// Build metadata
	metadata := map[string]interface{}{
		"action":  trace.Action,
		"success": trace.Success,
	}
	for k, v := range trace.Metadata {
		metadata[k] = v
	}

	return &TraceMemory{
		id:             uuid.New().String(),
		ownerID:        ownerID,
		conversationID: conversationID,
		createdAt:      time.Now(),
		importance:     importance,
		metadata:       metadata,
		Thought:        trace.Thought,
		Action:         trace.Action,
		Observation:    trace.Observation,
		Success:        trace.Success,
	}
}

// NewTraceMemoryFromStorage creates a TraceMemory from stored data.
// This is used by Store implementations when deserializing.
func NewTraceMemoryFromStorage(
	id string,
	ownerID string,
	conversationID string,
	createdAt time.Time,
	embedding []float32,
	thought string,
	action string,
	observation string,
	success bool,
	metadata map[string]interface{},
) *TraceMemory {
	return &TraceMemory{
		id:             id,
		ownerID:        ownerID,
		conversationID: conversationID,
		createdAt:      createdAt,
		embedding:      embedding,
		importance:     0.5, // Default, can be overridden
		metadata:       metadata,
		Thought:        thought,
		Action:         action,
		Observation:    observation,
		Success:        success,
	}
}

// Memory interface implementation

func (t *TraceMemory) ID() string {
	return t.id
}

func (t *TraceMemory) OwnerID() string {
	return t.ownerID
}

func (t *TraceMemory) ConversationID() string {
	return t.conversationID
}

func (t *TraceMemory) Type() string {
	return "trace"
}

func (t *TraceMemory) Content() interface{} {
	return map[string]interface{}{
		"thought":     t.Thought,
		"action":      t.Action,
		"observation": t.Observation,
		"success":     t.Success,
	}
}

func (t *TraceMemory) Metadata() map[string]interface{} {
	return t.metadata
}

func (t *TraceMemory) CreatedAt() time.Time {
	return t.createdAt
}

func (t *TraceMemory) Embedding() []float32 {
	return t.embedding
}

func (t *TraceMemory) SetEmbedding(emb []float32) {
	t.embedding = emb
}

// Format formats this trace for prompt injection.
// Produces a readable summary of the trace with thought, action, and observation.
func (t *TraceMemory) Format(ctx FormatContext) string {
	var parts []string

	// Status indicator
	status := "Success"
	if !t.Success {
		status = "Failed"
	}

	// Action line
	parts = append(parts, fmt.Sprintf("[%s] %s", status, t.Action))

	// Thought (if meaningful)
	if len(t.Thought) > 0 {
		thought := truncate(t.Thought, ctx.MaxLength/4) // Use up to 25% of space for thought
		parts = append(parts, fmt.Sprintf("  Thought: %q", thought))
	}

	// Observation (truncate to fit)
	if len(t.Observation) > 0 {
		observation := truncate(t.Observation, ctx.MaxLength/2) // Use up to 50% for observation
		parts = append(parts, fmt.Sprintf("  Observation: %q", observation))
	}

	// Add prevention strategy for failures
	if !t.Success {
		if prevention, ok := t.metadata["prevention"]; ok {
			parts = append(parts, fmt.Sprintf("  Prevention: %s", prevention))
		}
	}

	return strings.Join(parts, "\n")
}

// FormatForEmbedding returns text representation for embedding.
// This is used by Manager when embedding the trace.
func (t *TraceMemory) FormatForEmbedding() string {
	return fmt.Sprintf("Thought: %s\nAction: %s\nObservation: %s",
		t.Thought, t.Action, t.Observation)
}

// Importance returns the importance score for this trace.
func (t *TraceMemory) Importance() float64 {
	return t.importance
}

// Helper functions

// assessTraceImportance scores trace importance [0.0-1.0].
// More important traces are prioritized for retrieval.
func assessTraceImportance(trace *core.Trace) float64 {
	importance := 0.5 // Base

	// Failures are important for learning
	if !trace.Success {
		importance += 0.3
	}

	// Confirmations are high-value actions
	if trace.Metadata != nil {
		if trace.Metadata["confirmed"] == "true" {
			importance += 0.2
		}
	}

	// Multi-word thoughts indicate complex reasoning
	if len(trace.Thought) > 50 {
		importance += 0.1
	}

	// Cap at 1.0
	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}

// truncate truncates a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

