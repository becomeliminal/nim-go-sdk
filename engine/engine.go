package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/memory"
	"github.com/google/uuid"
)

// Engine is the agent runner that executes tools and manages Claude API interactions.
type Engine struct {
	client     *anthropic.Client
	registry   *ToolRegistry
	guardrails Guardrails      // Optional: rate limiting and circuit breaker
	audit      AuditLogger     // Optional: audit logging
	memory     memory.Manager  // Optional: memory system for trace retrieval/storage
}

// Option configures the engine.
type Option func(*Engine)

// WithGuardrails sets the guardrails implementation for rate limiting.
func WithGuardrails(g Guardrails) Option {
	return func(e *Engine) {
		e.guardrails = g
	}
}

// WithAudit sets the audit logger implementation.
func WithAudit(a AuditLogger) Option {
	return func(e *Engine) {
		e.audit = a
	}
}

// WithMemory configures the engine with a memory manager.
func WithMemory(m memory.Manager) Option {
	return func(e *Engine) {
		e.memory = m
	}
}

// NewEngine creates a new engine with the given Anthropic client and registry.
func NewEngine(client *anthropic.Client, registry *ToolRegistry, opts ...Option) *Engine {
	e := &Engine{
		client:   client,
		registry: registry,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Registry returns the engine's tool registry.
func (e *Engine) Registry() *ToolRegistry {
	return e.registry
}

// Input represents the input to an agent run.
type Input struct {
	// UserMessage is the user's message to process.
	UserMessage string

	// Context contains user identity, preferences, and execution limits.
	Context *core.Context

	// History contains previous messages in the conversation.
	History []core.Message

	// SystemPrompt is the system prompt to use.
	SystemPrompt string

	// Model is the Claude model to use.
	Model string

	// MaxTokens is the maximum response tokens.
	MaxTokens int64

	// AgentName identifies the agent for audit logging.
	// Defaults to "default" if not specified.
	AgentName string

	// AvailableTools filters which tools from the registry are available.
	// If empty, all registered tools are available.
	AvailableTools []string

	// StreamCallback is an optional callback for streaming responses.
	StreamCallback func(chunk string, done bool)
}

// Output represents the output from an agent run.
type Output struct {
	// Type indicates the kind of output.
	Type OutputType

	// Text is the agent's text response.
	Text string

	// PendingAction is set when Type is OutputConfirmationNeeded.
	PendingAction *core.PendingAction

	// ToolsUsed records all tools invoked during this run.
	ToolsUsed []core.ToolExecution

	// ResponseBlocks contains the full response for persistence.
	ResponseBlocks []core.ContentBlock

	// TokensUsed tracks Claude API token consumption for this run.
	TokensUsed core.TokenUsage

	// Error is set when Type is OutputError.
	Error error
}

// OutputType indicates the kind of output from an agent run.
type OutputType int

const (
	// OutputComplete indicates the agent finished successfully.
	OutputComplete OutputType = iota

	// OutputConfirmationNeeded indicates a write operation needs user confirmation.
	OutputConfirmationNeeded

	// OutputError indicates an error occurred.
	OutputError
)

// Run executes the agent loop until completion or confirmation is needed.
func (e *Engine) Run(ctx context.Context, input *Input) (*Output, error) {
	// Check guardrails if configured
	if e.guardrails != nil && input.Context != nil {
		result, err := e.guardrails.Check(ctx, input.Context.UserID)
		if err != nil {
			return &Output{
				Type:  OutputError,
				Error: fmt.Errorf("guardrails check failed: %w", err),
			}, nil
		}
		if !result.Allowed {
			return &Output{
				Type:  OutputError,
				Error: fmt.Errorf("request blocked by guardrails: %s", result.Warning),
			}, nil
		}
	}

	// === PHASE 0: RETRIEVE MEMORIES ===
	var enrichment string
	if e.memory != nil && input.UserMessage != "" && input.Context != nil {
		log.Printf("[MEMORY] Retrieving memories for query: %s", input.UserMessage)

		// Manager decides how to retrieve and format
		var err error
		enrichment, err = e.memory.Retrieve(ctx, input.Context.UserID, input.UserMessage)
		if err != nil {
			log.Printf("[MEMORY] Retrieval failed: %v", err)
			enrichment = "" // Non-fatal, continue without memories
		} else if enrichment != "" {
			log.Printf("[MEMORY] Retrieved memories successfully")
		}
	}

	// Apply defaults
	model := input.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	maxTokens := input.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	systemPrompt := input.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	// === PHASE 1: ENRICH SYSTEM PROMPT ===
	if enrichment != "" {
		systemPrompt += "\n\n" + enrichment
	}

	// Get limits from context
	maxTurns := 20
	canConfirm := true
	if input.Context != nil && input.Context.Limits != nil {
		maxTurns = input.Context.Limits.MaxTurns
		canConfirm = input.Context.Limits.CanConfirm
		if input.Context.Limits.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, input.Context.Limits.Timeout)
			defer cancel()
		}
	}

	// Create session
	userID := ""
	conversationID := ""
	if input.Context != nil {
		userID = input.Context.UserID
		conversationID = input.Context.ConversationID
	}
	session := NewSession(userID, conversationID)

	// Track cumulative token usage
	var totalTokens core.TokenUsage

	// Restore history
	session.RestoreHistory(input.History)

	// Add user message
	if input.UserMessage != "" {
		session.AddUserMessage(input.UserMessage)
	}

	// Get tools (filtered if AvailableTools is specified)
	var apiTools []anthropic.ToolUnionParam
	if len(input.AvailableTools) > 0 {
		apiTools = e.registry.ToAPIToolsFiltered(FilterByNames(input.AvailableTools...))
	} else {
		apiTools = e.registry.ToAPITools()
	}

	// Get agent name for audit logging
	agentName := input.AgentName
	if agentName == "" {
		agentName = "default"
	}

	// Get parent ID for audit chain
	var auditParentID *string
	if input.Context != nil && input.Context.AuditParentID != nil {
		auditParentID = input.Context.AuditParentID
	}

	for {
		// Check context cancellation
		if ctx.Err() != nil {
			return &Output{
				Type:       OutputError,
				Error:      fmt.Errorf("timed out: %w", ctx.Err()),
				TokensUsed: totalTokens,
			}, nil
		}

		// Check turn limit
		if session.TurnCount >= maxTurns {
			return &Output{
				Type:       OutputError,
				Error:      fmt.Errorf("exceeded maximum turns (%d)", maxTurns),
				TokensUsed: totalTokens,
			}, nil
		}

		session.IncrementTurnCount()

		// Build the message request
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: maxTokens,
			Messages:  session.Messages(),
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
		}

		if len(apiTools) > 0 {
			params.Tools = apiTools
		}

		// Call Claude API
		var resp *anthropic.Message
		var err error

		if input.StreamCallback != nil {
			resp, err = e.createMessageStreaming(ctx, params, input.StreamCallback)
		} else {
			resp, err = e.client.Messages.New(ctx, params)
		}

		if err != nil {
			return &Output{
				Type:       OutputError,
				Error:      fmt.Errorf("claude API error: %w", err),
				TokensUsed: totalTokens,
			}, err
		}

		// Accumulate token usage
		totalTokens.InputTokens += int(resp.Usage.InputTokens)
		totalTokens.OutputTokens += int(resp.Usage.OutputTokens)

		// Process response blocks
		var toolResults []anthropic.ContentBlockParamUnion
		var textResponse string
		var toolsUsed []core.ToolExecution
		var confirmationNeeded *core.PendingAction

		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				textResponse += block.Text

			case "tool_use":
				toolName := block.Name
				toolInput := block.Input

				// PHASE 1: THINK - Extract thought from tool input (type-safe)
				var baseInput struct {
					Thought string `json:"thought,omitempty"`
				}
				if err := json.Unmarshal(toolInput, &baseInput); err != nil {
					// JSON parsing error - shouldn't happen with Claude's output
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID,
						fmt.Sprintf("invalid tool input JSON: %s", err.Error()),
						true,
					))
					continue
				}

				thought := strings.TrimSpace(baseInput.Thought)

				tool, ok := e.registry.Get(toolName)
				if !ok {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID,
						fmt.Sprintf("unknown tool: %s", toolName),
						true,
					))
					continue
				}

				// PHASE 2: VALIDATE - Enforce thought presence for write operations
				if tool.RequiresConfirmation() && thought == "" {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID,
						`Error: Missing or empty "thought" field. Write operations require explicit reasoning.
Please explain:
1. What you've verified (e.g., "Balance is $500, sufficient for $100 transfer")
2. Why you're taking this action (e.g., "User requested transfer to Alice")
3. What you expect to happen (e.g., "This will complete the payment")`,
						true,
					))
					continue
				}

				// Create trace object for this action
				inputBytes, _ := json.Marshal(toolInput)
				trace := &core.Trace{
					ID:          uuid.New().String(),
					SessionID:   session.ID,
					TurnNumber:  session.TurnCount,
					Thought:     thought,
					Action:      toolName,
					ActionInput: inputBytes,
					Timestamp:   time.Now().Unix(),
					Metadata:    make(map[string]string),
				}

				// Check if write operation requiring confirmation
				if tool.RequiresConfirmation() {
					if !canConfirm {
						// Store trace for blocked confirmation
						trace.Success = false
						trace.Observation = "Operation blocked: confirmation not allowed in this context"
						trace.Metadata["error"] = "confirmation_disabled"
						session.AddTrace(trace)
						log.Printf("[REACT TRACE] %s", trace.String())

						toolResults = append(toolResults, anthropic.NewToolResultBlock(
							block.ID,
							"error: this operation requires user confirmation",
							true,
						))
						continue
					}

					// Generate pending confirmation
					confirmationNeeded = &core.PendingAction{
						ID:             uuid.New().String(),
						IdempotencyKey: GenerateIdempotencyKey(session.UserID, toolName, inputBytes),
						SessionID:      session.ID,
						UserID:         session.UserID,
						Tool:           toolName,
						Input:          inputBytes,
						Thought:        thought, // Store thought for ReAct trace on confirmation
						Summary:        tool.GetSummary(inputBytes),
						BlockID:        block.ID,
						CreatedAt:      time.Now().Unix(),
						ExpiresAt:      time.Now().Add(10 * time.Minute).Unix(),
					}

					// Store trace with pending status
					trace.Success = false
					trace.Observation = "Awaiting user confirmation"
					trace.Metadata["confirmation_id"] = confirmationNeeded.ID
					trace.Metadata["status"] = "pending_confirmation"
					session.AddTrace(trace)
					log.Printf("[REACT TRACE] %s", trace.String())
					break
				}

				// PHASE 3: ACT - Execute read-only tool
				startTime := time.Now()
				result, err := tool.Execute(ctx, &core.ToolParams{
					UserID:    session.UserID,
					Input:     inputBytes,
					RequestID: session.ID,
				})

				durationMs := time.Since(startTime).Milliseconds()
				execution := core.ToolExecution{
					Tool:       toolName,
					Input:      toolInput,
					DurationMs: durationMs,
				}

				// PHASE 4: OBSERVE - Format observation
				trace.Success = (err == nil && result != nil && result.Success)
				trace.Observation = formatObservation(tool, result, err)

				// Store failure context if applicable
				if !trace.Success {
					if err != nil {
						trace.Metadata["error"] = err.Error()
						execution.Error = err.Error()
					} else if result != nil && !result.Success {
						trace.Metadata["error"] = result.Error
						execution.Error = result.Error
					}

					// Categorize error for reflexion
					errorType := categorizeError(trace.Metadata["error"])
					trace.Metadata["error_type"] = errorType
					trace.Metadata["prevention"] = generatePrevention(toolName, errorType)
				}

				// Add trace to session
				session.AddTrace(trace)

				// Log the ReAct trace
				log.Printf("[REACT TRACE] %s", trace.String())

				// Log audit entry if configured
				if e.audit != nil {
					var outputBytes json.RawMessage
					var errStr *string
					if result != nil {
						outputBytes, _ = json.Marshal(result.Data)
						if result.Error != "" {
							errStr = &result.Error
						}
					}
					if err != nil {
						errMsg := err.Error()
						errStr = &errMsg
					}
					e.audit.Log(ctx, &AuditEntry{
						ID:         uuid.New().String(),
						UserID:     session.UserID,
						SessionID:  session.ID,
						RequestID:  session.ID,
						ParentID:   auditParentID,
						AgentName:  agentName,
						ToolName:   toolName,
						ToolInput:  inputBytes,
						ToolOutput: outputBytes,
						Error:      errStr,
						DurationMs: durationMs,
						IsWriteOp:  tool.RequiresConfirmation(),
						Timestamp:  startTime.Unix(),
					})
				}

				// Build tool result for Claude
				if err != nil {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID, err.Error(), true))
				} else if result != nil && !result.Success {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID, result.Error, true))
				} else {
					if result != nil {
						execution.Result = result.Data
					}
					resultBytes, _ := json.Marshal(result.Data)
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID, string(resultBytes), false))
				}

				toolsUsed = append(toolsUsed, execution)
			}

			if confirmationNeeded != nil {
				break
			}
		}

		// Build response blocks for persistence
		responseBlocks := responseToBlocks(resp)

		// If confirmation needed, return for user approval
		if confirmationNeeded != nil {
			session.AddAssistantResponse(resp)

			return &Output{
				Type:           OutputConfirmationNeeded,
				Text:           textResponse,
				PendingAction:  confirmationNeeded,
				ToolsUsed:      toolsUsed,
				ResponseBlocks: responseBlocks,
				TokensUsed:     totalTokens,
			}, nil
		}

		// If no tool calls, we're done
		if len(toolResults) == 0 {
			session.AddAssistantMessage(textResponse)

			if input.StreamCallback != nil {
				input.StreamCallback("", true)
			}

			// Record success with guardrails
			if e.guardrails != nil && input.Context != nil {
				e.guardrails.RecordSuccess(ctx, input.Context.UserID)
			}

			// === PHASE 5: RECORD TRACES ===
			if e.memory != nil && len(session.Traces) > 0 && input.Context != nil {
				log.Printf("[MEMORY] Recording %d traces", len(session.Traces))

				// Manager decides what to store and how
				traces := session.Traces
				userID := input.Context.UserID

				// Record traces (implementor can make async if desired)
				if err := e.memory.RecordTraces(ctx, userID, traces); err != nil {
					log.Printf("[MEMORY] Failed to record traces: %v", err)
				}
			}

			// === PHASE 5b: RECORD CONVERSATION ===
			if e.memory != nil && input.Context != nil && input.UserMessage != "" && textResponse != "" {
				if err := e.memory.RecordConversation(ctx, input.Context.UserID, input.UserMessage, textResponse); err != nil {
					log.Printf("[MEMORY] Failed to record conversation: %v", err)
				}
			}

			return &Output{
				Type:       OutputComplete,
				Text:       textResponse,
				ToolsUsed:  toolsUsed,
				TokensUsed: totalTokens,
			}, nil
		}

		// Continue loop with tool results
		session.AddAssistantResponse(resp)
		session.AddToolResults(toolResults)
	}
}

// ExecuteTool executes a confirmed write operation.
func (e *Engine) ExecuteTool(ctx context.Context, userID, toolName string, input json.RawMessage, confirmationID string) (*core.ToolResult, error) {
	tool, ok := e.registry.Get(toolName)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	return tool.Execute(ctx, &core.ToolParams{
		UserID:         userID,
		Input:          input,
		ConfirmationID: confirmationID,
		RequestID:      confirmationID,
	})
}


// RunConfirmedAction resumes the ReAct loop for a confirmed write operation.
// This ensures traces are created and Claude can respond to the result.
func (e *Engine) RunConfirmedAction(ctx context.Context, input *Input, action *core.PendingAction) (*Output, error) {
	// Create session from input
	userID := ""
	conversationID := ""
	if input.Context != nil {
		userID = input.Context.UserID
		conversationID = input.Context.ConversationID
	}
	session := NewSession(userID, conversationID)

	// Restore history - this includes the original tool_use block
	session.RestoreHistory(input.History)

	// Extract thought (already stored in action)
	thought := action.Thought

	// Get tool from registry
	tool, ok := e.registry.Get(action.Tool)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", action.Tool)
	}

	// Create trace object (THINK phase already done)
	trace := &core.Trace{
		ID:          uuid.New().String(),
		SessionID:   session.ID,
		TurnNumber:  session.TurnCount,
		Thought:     thought,
		Action:      action.Tool,
		ActionInput: action.Input,
		Timestamp:   time.Now().Unix(),
		Metadata:    make(map[string]string),
	}
	trace.Metadata["confirmed"] = "true"
	trace.Metadata["confirmation_id"] = action.ID

	// PHASE 3: ACT - Execute the confirmed tool
	// Note: Pass empty confirmationID since confirmation was already handled locally.
	// The HTTPExecutor will call ExecuteWrite() directly instead of trying to
	// confirm via the remote API (which doesn't know about our local confirmations).
	startTime := time.Now()
	result, toolErr := tool.Execute(ctx, &core.ToolParams{
		UserID:         action.UserID,
		Input:          action.Input,
		ConfirmationID: "", // Empty string = already confirmed, execute directly
		RequestID:      session.ID,
	})

	durationMs := time.Since(startTime).Milliseconds()

	// PHASE 4: OBSERVE - Format observation and complete trace
	trace.Success = (toolErr == nil && result != nil && result.Success)
	trace.Observation = formatObservation(tool, result, toolErr)

	if !trace.Success {
		if toolErr != nil {
			trace.Metadata["error"] = toolErr.Error()
		} else if result != nil && !result.Success {
			trace.Metadata["error"] = result.Error
		}

		errorType := categorizeError(trace.Metadata["error"])
		trace.Metadata["error_type"] = errorType
		trace.Metadata["prevention"] = generatePrevention(action.Tool, errorType)
	}

	// Add trace to session
	session.AddTrace(trace)
	log.Printf("[REACT TRACE] %s", trace.String())

	// Build tool result block for Claude
	var toolResult anthropic.ContentBlockParamUnion
	if toolErr != nil {
		log.Printf("[CONFIRMATION] Tool execution error, will send to Claude: %v", toolErr)
		toolResult = anthropic.NewToolResultBlock(action.BlockID, toolErr.Error(), true)
	} else if result != nil && !result.Success {
		log.Printf("[CONFIRMATION] Tool execution failed, will send to Claude: %s", result.Error)
		toolResult = anthropic.NewToolResultBlock(action.BlockID, result.Error, true)
	} else {
		log.Printf("[CONFIRMATION] Tool execution succeeded, sending result to Claude")
		resultBytes, _ := json.Marshal(result.Data)
		toolResult = anthropic.NewToolResultBlock(action.BlockID, string(resultBytes), false)
	}

	// Add tool result to session (the tool_use block is already in history from RestoreHistory)
	session.AddToolResults([]anthropic.ContentBlockParamUnion{toolResult})
	log.Printf("[CONFIRMATION] Calling Claude API to get contextual response...")

	// Continue the loop - call Claude with the result
	// Claude will see the tool result and generate a response
	model := input.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	maxTokens := input.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	systemPrompt := input.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  session.Messages(),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
	}

	resp, err := e.client.Messages.New(ctx, params)
	if err != nil {
		log.Printf("[CONFIRMATION] Claude API error: %v", err)
		return nil, fmt.Errorf("claude api error after confirmation: %w", err)
	}

	log.Printf("[CONFIRMATION] Claude responded successfully")

	// Extract text response
	var textResponse string
	for _, block := range resp.Content {
		if block.Type == "text" {
			textResponse += block.Text
		}
	}

	log.Printf("[CONFIRMATION] Claude's response: %s", textResponse)

	session.AddAssistantResponse(resp)

	// Build response blocks for persistence
	responseBlocks := responseToBlocks(resp)

	// Build tool execution record
	var toolInput interface{}
	json.Unmarshal(action.Input, &toolInput)
	execution := core.ToolExecution{
		Tool:       action.Tool,
		Input:      toolInput,
		DurationMs: durationMs,
	}
	if toolErr != nil {
		execution.Error = toolErr.Error()
	} else if result != nil {
		if !result.Success {
			execution.Error = result.Error
		} else {
			execution.Result = result.Data
		}
	}

	// === PHASE 5: RECORD TRACES ===
	if e.memory != nil && len(session.Traces) > 0 && input.Context != nil {
		log.Printf("[MEMORY] Recording %d traces from confirmed action", len(session.Traces))

		// Manager decides what to store and how
		traces := session.Traces
		userID := input.Context.UserID

		// Record traces (implementor can make async if desired)
		if err := e.memory.RecordTraces(ctx, userID, traces); err != nil {
			log.Printf("[MEMORY] Failed to record traces: %v", err)
		}
	}

	// === PHASE 5b: RECORD CONVERSATION ===
	if e.memory != nil && input.Context != nil && textResponse != "" {
		if err := e.memory.RecordConversation(ctx, input.Context.UserID, "", textResponse); err != nil {
			log.Printf("[MEMORY] Failed to record conversation: %v", err)
		}
	}

	return &Output{
		Type:           OutputComplete,
		Text:           textResponse,
		ToolsUsed:      []core.ToolExecution{execution},
		ResponseBlocks: responseBlocks,
	}, nil
}

// createMessageStreaming handles streaming API calls.
func (e *Engine) createMessageStreaming(ctx context.Context, params anthropic.MessageNewParams, callback func(string, bool)) (*anthropic.Message, error) {
	stream := e.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	// Accumulate the message from events
	message := anthropic.Message{}

	for stream.Next() {
		event := stream.Current()

		// Accumulate into the message
		if err := message.Accumulate(event); err != nil {
			// Log but continue - accumulation errors are non-fatal
		}

		// Handle different event types
		switch evt := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := evt.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				callback(delta.Text, false)
			}
		case anthropic.MessageStopEvent:
			// Stream complete
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &message, nil
}

// responseToBlocks converts a Claude response to core.ContentBlock slice.
func responseToBlocks(resp *anthropic.Message) []core.ContentBlock {
	blocks := make([]core.ContentBlock, 0, len(resp.Content))
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			blocks = append(blocks, core.NewTextBlock(block.Text))
		case "tool_use":
			inputBytes, _ := json.Marshal(block.Input)
			blocks = append(blocks, core.NewToolUseBlock(block.ID, block.Name, inputBytes))
		}
	}
	return blocks
}

// formatObservation handles observation formatting with fallback
func formatObservation(tool core.Tool, result *core.ToolResult, err error) string {
	// Try custom formatter first (optional interface)
	type ObservationFormatter interface {
		FormatObservation(result *core.ToolResult, err error) string
	}
	if formatter, ok := tool.(ObservationFormatter); ok {
		return formatter.FormatObservation(result, err)
	}

	// Default fallback formatting
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}
	if result == nil {
		return "No result returned"
	}
	if !result.Success {
		return fmt.Sprintf("Failed: %s", result.Error)
	}

	// Format success based on data type
	switch v := result.Data.(type) {
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			return msg
		}
		if status, ok := v["status"].(string); ok {
			return fmt.Sprintf("Success: %s", status)
		}
		bytes, _ := json.Marshal(v)
		return string(bytes)
	case string:
		return v
	default:
		return fmt.Sprintf("Success: %v", v)
	}
}

// categorizeError maps error messages to error types for reflexion
func categorizeError(errMsg string) string {
	if errMsg == "" {
		return "unknown"
	}

	errLower := strings.ToLower(errMsg)

	switch {
	case strings.Contains(errLower, "insufficient"), strings.Contains(errLower, "not enough"):
		return "insufficient_balance"
	case strings.Contains(errLower, "not found"), strings.Contains(errLower, "does not exist"):
		return "not_found"
	case strings.Contains(errLower, "invalid"), strings.Contains(errLower, "malformed"):
		return "invalid_input"
	case strings.Contains(errLower, "unauthorized"), strings.Contains(errLower, "forbidden"):
		return "permission_denied"
	case strings.Contains(errLower, "timeout"), strings.Contains(errLower, "deadline"):
		return "timeout"
	case strings.Contains(errLower, "rate limit"), strings.Contains(errLower, "too many"):
		return "rate_limit"
	case strings.Contains(errLower, "network"), strings.Contains(errLower, "connection"):
		return "network_error"
	default:
		return "unknown"
	}
}

// generatePrevention suggests how to avoid this error in the future
func generatePrevention(action, errorType string) string {
	preventionMap := map[string]string{
		"send_money:insufficient_balance":          "Check balance with get_balance before attempting transfer",
		"send_money:not_found":                     "Verify recipient exists with search_users before transfer",
		"send_money:invalid_input":                 "Validate amount is positive and recipient ID format is correct",
		"deposit_savings:insufficient_balance":     "Check wallet balance before depositing to savings",
		"withdraw_savings:insufficient_balance":    "Check savings balance with get_savings_balance before withdrawal",
	}

	key := action + ":" + errorType
	if prevention, ok := preventionMap[key]; ok {
		return prevention
	}

	// Generic prevention by error type
	switch errorType {
	case "insufficient_balance":
		return "Check balance before attempting operation"
	case "not_found":
		return "Verify the entity exists before referencing it"
	case "invalid_input":
		return "Validate input parameters before submission"
	case "rate_limit":
		return "Implement retry with backoff"
	case "timeout":
		return "Retry operation with timeout handling"
	default:
		return "Review error message and adjust approach accordingly"
	}
}

// RunAgent executes an Agent using the engine.
// This method uses the agent's Capabilities to configure the execution.
func (e *Engine) RunAgent(ctx context.Context, agent core.Agent, input *core.Input) (*core.Output, error) {
	caps := agent.Capabilities()

	// Build engine input from core input and agent capabilities
	engineInput := &Input{
		UserMessage:    input.UserMessage,
		Context:        input.Context,
		History:        input.History,
		SystemPrompt:   caps.SystemPrompt,
		Model:          caps.Model,
		MaxTokens:      caps.MaxTokens,
		AgentName:      agent.Name(),
		AvailableTools: caps.AvailableTools,
	}

	// Override context limits with agent capabilities if not already set
	if engineInput.Context != nil && engineInput.Context.Limits == nil {
		engineInput.Context.Limits = &core.ExecutionLimits{
			MaxTurns:   caps.MaxTurns,
			MaxTokens:  caps.MaxTokens,
			CanConfirm: caps.CanRequestConfirmation,
		}
	}

	// Set stream callback if provided
	if input.StreamCallback != nil {
		engineInput.StreamCallback = input.StreamCallback
	}

	// Run the engine
	output, err := e.Run(ctx, engineInput)
	if err != nil {
		return nil, err
	}

	// Convert to core output
	return &core.Output{
		Type:           core.OutputType(output.Type),
		Text:           output.Text,
		PendingAction:  output.PendingAction,
		ToolsUsed:      output.ToolsUsed,
		ResponseBlocks: output.ResponseBlocks,
		TokensUsed:     output.TokensUsed,
		Error:          output.Error,
	}, nil
}

// DefaultSystemPrompt is the default system prompt for the agent.
const DefaultSystemPrompt = `You are a helpful financial assistant.

GUIDELINES:
- Be conversational and helpful
- Ask clarifying questions when needed
- Use tools when you have enough information
- All money movements require user confirmation

REASONING PATTERN:
When using tools, include a "thought" field explaining your reasoning:
1. What you've verified (e.g., "User's balance is $500, sufficient for $100 transfer")
2. Why you're taking this action (e.g., "Need to check balance before attempting transfer")
3. What you expect to happen (e.g., "This will return the current account balance")

For write operations (transfers, payments, withdrawals), the thought field is REQUIRED.

Good thought examples:
- "User requested $50 to Alice. I've confirmed the amount and will check if balance is sufficient."
- "Balance is $200, sufficient for $50 transfer. Proceeding with send_money."

Bad thought examples:
- "Sending money" (too vague, doesn't explain reasoning)
- "User asked" (doesn't verify or explain decision)

AVAILABLE ACTIONS:
- Check balances and transactions
- Send money to other users
- Manage savings deposits and withdrawals
- Look up user profiles`
