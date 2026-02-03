package core

// BaseInput provides common fields for all tool inputs.
// Tools embed this struct to automatically include ReAct thought support.
type BaseInput struct {
	// Thought contains the agent's reasoning about why it's using this tool.
	// For write operations, this should explain the decision-making process.
	// Optional for read operations, required for write operations.
	Thought string `json:"thought,omitempty"`
}
