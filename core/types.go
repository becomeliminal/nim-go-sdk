// Package core provides the fundamental types for the Nim agent SDK.
package core

import (
	"encoding/json"
	"fmt"
	"time"
)

// Role represents the role of a message sender.
type Role string

const (
	// RoleUser represents a message from the user.
	RoleUser Role = "user"

	// RoleAssistant represents a message from the assistant.
	RoleAssistant Role = "assistant"
)

// Message represents a single message in a conversation.
type Message struct {
	// Role is who sent the message (user or assistant).
	Role Role `json:"role"`

	// Content is the text content for simple messages.
	Content string `json:"content,omitempty"`

	// ContentBlocks contains structured content for complex messages.
	ContentBlocks []ContentBlock `json:"content_blocks,omitempty"`
}

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	// Type is the kind of content block.
	Type ContentBlockType `json:"type"`

	// Text is the text content (for TextBlock type).
	Text string `json:"text,omitempty"`

	// ToolUse contains tool invocation details (for ToolUseBlock type).
	ToolUse *ToolUseContent `json:"tool_use,omitempty"`

	// ToolResult contains tool execution result (for ToolResultBlock type).
	ToolResult *ToolResultContent `json:"tool_result,omitempty"`
}

// ContentBlockType indicates the type of content block.
type ContentBlockType string

const (
	// TextBlockType contains plain text.
	TextBlockType ContentBlockType = "text"

	// ToolUseBlockType contains a tool invocation.
	ToolUseBlockType ContentBlockType = "tool_use"

	// ToolResultBlockType contains the result of a tool execution.
	ToolResultBlockType ContentBlockType = "tool_result"
)

// ToolUseContent contains details about a tool invocation.
type ToolUseContent struct {
	// ID is Claude's unique identifier for this tool use.
	ID string `json:"id"`

	// Name is the tool name.
	Name string `json:"name"`

	// Input is the tool parameters as JSON.
	Input json.RawMessage `json:"input"`
}

// ToolResultContent contains the result of a tool execution.
type ToolResultContent struct {
	// ToolUseID references the corresponding tool_use block.
	ToolUseID string `json:"tool_use_id"`

	// Content is the result data.
	Content string `json:"content"`

	// IsError indicates if the tool execution failed.
	IsError bool `json:"is_error,omitempty"`
}

// Trace represents a single ReAct reasoning-action-observation cycle
type Trace struct {
	ID          string                 `json:"id"`            // Unique trace identifier
	SessionID   string                 `json:"session_id"`    // Links to session
	TurnNumber  int                    `json:"turn_number"`   // Sequence within session
	Thought     string                 `json:"thought"`       // Agent's reasoning
	Action      string                 `json:"action"`        // Tool name
	ActionInput json.RawMessage        `json:"action_input"`  // Tool parameters
	Observation string                 `json:"observation"`   // Formatted result
	Success     bool                   `json:"success"`       // Execution outcome
	Timestamp   int64                  `json:"timestamp"`     // Unix timestamp
	Metadata    map[string]string      `json:"metadata,omitempty"` // Error context, prevention
}

// String formats the trace for logging and debugging
func (t *Trace) String() string {
	status := "✓"
	if !t.Success {
		status = "✗"
	}
	return fmt.Sprintf("[%s] %s | Thought: %q | Observation: %q",
		status, t.Action, truncate(t.Thought, 60), truncate(t.Observation, 100))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// NewUserMessage creates a user text message.
func NewUserMessage(text string) Message {
	return Message{Role: RoleUser, Content: text}
}

// NewAssistantMessage creates an assistant text message.
func NewAssistantMessage(text string) Message {
	return Message{Role: RoleAssistant, Content: text}
}

// NewAssistantMessageWithBlocks creates an assistant message with content blocks.
func NewAssistantMessageWithBlocks(blocks []ContentBlock) Message {
	return Message{Role: RoleAssistant, ContentBlocks: blocks}
}

// NewToolResultMessage creates a user message containing tool results.
func NewToolResultMessage(results []ToolResultContent) Message {
	blocks := make([]ContentBlock, len(results))
	for i, result := range results {
		blocks[i] = ContentBlock{
			Type:       ToolResultBlockType,
			ToolResult: &result,
		}
	}
	return Message{Role: RoleUser, ContentBlocks: blocks}
}

// NewTextBlock creates a text content block.
func NewTextBlock(text string) ContentBlock {
	return ContentBlock{Type: TextBlockType, Text: text}
}

// NewToolUseBlock creates a tool_use content block.
func NewToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{
		Type: ToolUseBlockType,
		ToolUse: &ToolUseContent{
			ID:    id,
			Name:  name,
			Input: input,
		},
	}
}

// NewToolResultBlock creates a tool_result content block.
func NewToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{
		Type: ToolResultBlockType,
		ToolResult: &ToolResultContent{
			ToolUseID: toolUseID,
			Content:   content,
			IsError:   isError,
		},
	}
}

// GetText returns all text content concatenated.
func (m *Message) GetText() string {
	if m.Content != "" {
		return m.Content
	}
	var text string
	for _, block := range m.ContentBlocks {
		if block.Type == TextBlockType {
			text += block.Text
		}
	}
	return text
}

// Context contains all contextual information for agent execution.
type Context struct {
	// UserID is the authenticated user's unique identifier.
	UserID string

	// SessionID identifies the current agent session.
	SessionID string

	// ConversationID links to the persistent conversation.
	ConversationID string

	// RequestID is a unique identifier for this request (for tracing).
	RequestID string

	// AuditParentID links sub-agent audit entries to their parent.
	AuditParentID *string

	// Preferences contains user's configuration and defaults.
	Preferences *UserPreferences

	// UserLimits contains user-specific financial limits.
	UserLimits *UserLimits

	// Limits contains execution constraints.
	Limits *ExecutionLimits

	// StartTime is when this execution started.
	StartTime time.Time
}

// NewContext creates a new Context with default values.
func NewContext(userID, sessionID, conversationID, requestID string) *Context {
	return &Context{
		UserID:         userID,
		SessionID:      sessionID,
		ConversationID: conversationID,
		RequestID:      requestID,
		Preferences:    DefaultPreferences(),
		Limits:         DefaultLimits(),
		StartTime:      time.Now(),
	}
}

// UserPreferences contains user-specific configuration.
type UserPreferences struct {
	// DefaultChain is the user's preferred blockchain (e.g., "arbitrum").
	DefaultChain string `json:"default_chain"`

	// DefaultToken is the user's preferred token (e.g., "usdc").
	DefaultToken string `json:"default_token"`

	// DefaultVault is the user's preferred savings vault.
	DefaultVault string `json:"default_vault"`

	// Locale is the user's language preference (e.g., "en-US").
	Locale string `json:"locale"`

	// Timezone is the user's timezone (e.g., "America/New_York").
	Timezone string `json:"timezone"`

	// Shortcuts maps user-defined nicknames to user IDs.
	// For example: {"mom": "user_abc123", "landlord": "user_xyz789"}
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
}

// DefaultPreferences returns the default user preferences.
func DefaultPreferences() *UserPreferences {
	return &UserPreferences{
		DefaultChain: "arbitrum",
		DefaultToken: "usdc",
		DefaultVault: "morpho",
		Locale:       "en-US",
		Timezone:     "UTC",
	}
}

// UserLimits contains user-specific financial limits.
type UserLimits struct {
	// DailyTransferLimit is the maximum amount the user can transfer per day.
	DailyTransferLimit string `json:"daily_transfer_limit"`

	// DailyTransferUsed is the amount already transferred today.
	DailyTransferUsed string `json:"daily_transfer_used"`

	// SingleTransferMax is the maximum amount for a single transfer.
	SingleTransferMax string `json:"single_transfer_max"`
}

// DefaultUserLimits returns sensible default user limits.
func DefaultUserLimits() *UserLimits {
	return &UserLimits{
		DailyTransferLimit: "10000.00",
		DailyTransferUsed:  "0.00",
		SingleTransferMax:  "5000.00",
	}
}

// ExecutionLimits constrains agent execution.
type ExecutionLimits struct {
	// MaxTurns is the maximum number of agent turns (API round-trips).
	MaxTurns int

	// MaxTokens is the maximum response tokens per turn.
	MaxTokens int64

	// Timeout is the maximum execution time.
	Timeout time.Duration

	// MaxToolCalls is the maximum total tool calls per execution.
	MaxToolCalls int

	// CanConfirm indicates whether this execution can request user confirmation.
	CanConfirm bool
}

// DefaultLimits returns the default execution limits.
func DefaultLimits() *ExecutionLimits {
	return &ExecutionLimits{
		MaxTurns:     20,
		MaxTokens:    4096,
		Timeout:      5 * time.Minute,
		MaxToolCalls: 50,
		CanConfirm:   true,
	}
}

// SubAgentLimits returns restricted limits for sub-agent execution.
// Sub-agents have tighter constraints and cannot request confirmation.
func SubAgentLimits() *ExecutionLimits {
	return &ExecutionLimits{
		MaxTurns:     10,
		MaxTokens:    2048,
		Timeout:      60 * time.Second,
		MaxToolCalls: 20,
		CanConfirm:   false, // Sub-agents cannot request confirmation
	}
}

// ForSubAgent creates a new context for sub-agent execution.
// The new context inherits user identity but has restricted limits
// and sets up audit parent chain.
func (c *Context) ForSubAgent(requestID string) *Context {
	parentID := c.RequestID
	return &Context{
		UserID:         c.UserID,
		SessionID:      c.SessionID,
		ConversationID: c.ConversationID,
		RequestID:      requestID,
		AuditParentID:  &parentID,
		Preferences:    c.Preferences,
		UserLimits:     c.UserLimits,
		Limits:         SubAgentLimits(),
		StartTime:      time.Now(),
	}
}

// Elapsed returns the time elapsed since StartTime.
func (c *Context) Elapsed() time.Duration {
	return time.Since(c.StartTime)
}

// IsTimedOut returns true if the context has exceeded its timeout.
func (c *Context) IsTimedOut() bool {
	if c.Limits == nil || c.Limits.Timeout == 0 {
		return false
	}
	return c.Elapsed() >= c.Limits.Timeout
}

// TokenUsage tracks Claude API token consumption.
type TokenUsage struct {
	// InputTokens is the number of tokens in the input.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of tokens in Claude's response.
	OutputTokens int `json:"output_tokens"`

	// CacheCreationInputTokens is tokens written to prompt cache.
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`

	// CacheReadInputTokens is tokens read from prompt cache.
	CacheReadInputTokens int `json:"cache_read_input_tokens,omitempty"`
}

// TotalTokens returns the sum of input and output tokens.
func (t TokenUsage) TotalTokens() int {
	return t.InputTokens + t.OutputTokens
}

// PendingAction represents an action awaiting user confirmation.
type PendingAction struct {
	// ID is the unique identifier for this pending action.
	ID string `json:"id"`

	// IdempotencyKey is a hash for deduplicating similar confirmations.
	// Generated from userID, tool, input, and time bucket.
	IdempotencyKey string `json:"idempotency_key"`

	// SessionID identifies which session created this confirmation.
	SessionID string `json:"session_id"`

	// UserID is the user who initiated the action.
	UserID string `json:"user_id"`

	// Tool is the name of the tool to execute.
	Tool string `json:"tool"`

	// Input is the tool parameters as JSON.
	Input json.RawMessage `json:"input"`

	// Thought is the agent's reasoning for this action (stored for ReAct trace).
	Thought string `json:"thought,omitempty"`

	// Summary is a human-readable description of the action.
	Summary string `json:"summary"`

	// BlockID is Claude's tool_use block ID for session reconstruction.
	BlockID string `json:"block_id"`

	// CreatedAt is when the action was created (unix timestamp).
	CreatedAt int64 `json:"created_at"`

	// ExpiresAt is when this confirmation expires (unix timestamp).
	ExpiresAt int64 `json:"expires_at"`
}

// ToolExecution records a single tool invocation.
type ToolExecution struct {
	// Tool is the name of the tool.
	Tool string `json:"tool"`

	// Input is the tool parameters.
	Input interface{} `json:"input"`

	// Result is the tool output.
	Result interface{} `json:"result,omitempty"`

	// Error is any error message.
	Error string `json:"error,omitempty"`

	// DurationMs is execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}
