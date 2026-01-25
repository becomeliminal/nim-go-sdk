package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gorilla/websocket"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/engine"
	"github.com/becomeliminal/nim-go-sdk/store"
)

// Config configures the server.
type Config struct {
	// AnthropicKey is the Anthropic API key.
	AnthropicKey string

	// BaseURL is the Anthropic API base URL.
	// If empty, uses the default Anthropic API URL.
	// Useful for testing with mock servers.
	BaseURL string

	// SystemPrompt is the system prompt for the agent.
	SystemPrompt string

	// Model is the Claude model to use.
	Model string

	// MaxTokens is the maximum response tokens.
	MaxTokens int64

	// AuthFunc validates requests and returns a user ID.
	// If nil, a default user ID is used (not recommended for production).
	AuthFunc func(r *http.Request) (userID string, err error)

	// Conversations persists conversations.
	// If nil, an in-memory store is used.
	Conversations store.Conversations

	// Confirmations stores pending actions awaiting user approval.
	// If nil, an in-memory store is used.
	Confirmations store.Confirmations

	// Guardrails provides rate limiting and circuit breaker functionality.
	// If nil, no guardrails are applied.
	Guardrails engine.Guardrails

	// AuditLogger logs agent actions for compliance.
	// If nil, no audit logging is performed.
	AuditLogger engine.AuditLogger

	// AnthropicOptions are additional options for the Anthropic client.
	// This can be used to customize the HTTP client for testing.
	AnthropicOptions []option.RequestOption
}

// Server is a WebSocket server for the Nim agent.
type Server struct {
	config   Config
	engine   *engine.Engine
	registry *engine.ToolRegistry
	upgrader websocket.Upgrader

	conversations store.Conversations
	confirmations store.Confirmations
	sessions      sync.Map // *websocket.Conn -> *session
}

type session struct {
	ID             string
	UserID         string
	ConversationID string
	History        []core.Message
	TurnCount      int
}

// New creates a new server with the given configuration.
// Returns an error if AnthropicKey is not provided.
func New(cfg Config) (*Server, error) {
	if cfg.AnthropicKey == "" {
		return nil, fmt.Errorf("AnthropicKey is required")
	}

	// Build Anthropic client options
	opts := make([]option.RequestOption, 0, len(cfg.AnthropicOptions)+2)
	opts = append(opts, cfg.AnthropicOptions...)
	opts = append(opts, option.WithAPIKey(cfg.AnthropicKey))

	// Add base URL if provided
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	// Create Anthropic client
	client := anthropic.NewClient(opts...)

	// Create registry
	registry := engine.NewToolRegistry()

	// Build engine options
	var engineOpts []engine.Option
	if cfg.Guardrails != nil {
		engineOpts = append(engineOpts, engine.WithGuardrails(cfg.Guardrails))
	}
	if cfg.AuditLogger != nil {
		engineOpts = append(engineOpts, engine.WithAudit(cfg.AuditLogger))
	}

	// Create engine
	eng := engine.NewEngine(&client, registry, engineOpts...)

	// Default to in-memory stores if not provided
	conversations := cfg.Conversations
	if conversations == nil {
		conversations = store.NewMemoryConversations()
	}

	confirmations := cfg.Confirmations
	if confirmations == nil {
		confirmations = store.NewMemoryConfirmations()
	}

	return &Server{
		config:        cfg,
		engine:        eng,
		registry:      registry,
		conversations: conversations,
		confirmations: confirmations,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}, nil
}

// AddTool registers a custom tool with the server.
func (s *Server) AddTool(tool core.Tool) {
	s.registry.Register(tool)
}

// AddTools registers multiple tools with the server.
func (s *Server) AddTools(tools ...core.Tool) {
	s.registry.RegisterAll(tools...)
}

// ToolCount returns the number of registered tools.
func (s *Server) ToolCount() int {
	return s.registry.Count()
}

// Handler returns an HTTP handler for WebSocket connections.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handleWebSocket)
}

// Run starts the server on the given address.
func (s *Server) Run(addr string) error {
	http.Handle("/ws", s.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Starting Nim agent server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate
	userID := "default-user"
	if s.config.AuthFunc != nil {
		var err error
		userID, err = s.config.AuthFunc(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connected for user %s", userID)

	var currentSession *session

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			s.sendError(conn, "Invalid message format")
			continue
		}

		log.Printf("Received message type=%s from user=%s", msg.Type, userID)

		switch msg.Type {
		case "new_conversation":
			currentSession = s.handleNewConversation(r.Context(), conn, userID)

		case "resume_conversation":
			currentSession = s.handleResumeConversation(r.Context(), conn, userID, msg.ConversationID)

		case "message":
			if currentSession == nil {
				s.sendError(conn, "No active conversation. Send 'new_conversation' first.")
				continue
			}
			s.handleMessage(r.Context(), conn, currentSession, msg.Content)

		case "confirm":
			if currentSession == nil {
				s.sendError(conn, "No active conversation")
				continue
			}
			s.handleConfirm(r.Context(), conn, currentSession, userID, msg.ActionID)

		case "cancel":
			if currentSession == nil {
				s.sendError(conn, "No active conversation")
				continue
			}
			s.handleCancel(r.Context(), conn, currentSession, userID, msg.ActionID)

		default:
			s.sendError(conn, fmt.Sprintf("Unknown message type: %s", msg.Type))
		}
	}
}

func (s *Server) handleNewConversation(ctx context.Context, conn *websocket.Conn, userID string) *session {
	conv, err := s.conversations.Create(ctx, userID)
	if err != nil {
		s.sendError(conn, fmt.Sprintf("Failed to create conversation: %v", err))
		return nil
	}

	sess := &session{
		ID:             conv.ID,
		UserID:         userID,
		ConversationID: conv.ID,
		History:        []core.Message{},
	}
	s.sessions.Store(conn, sess)

	s.send(conn, ServerMessage{
		Type:           "conversation_started",
		ConversationID: conv.ID,
	})

	log.Printf("Started conversation %s for user %s", conv.ID, userID)
	return sess
}

func (s *Server) handleResumeConversation(ctx context.Context, conn *websocket.Conn, userID, conversationID string) *session {
	conv, err := s.conversations.Get(ctx, conversationID)
	if err != nil {
		s.sendError(conn, "Conversation not found")
		return nil
	}

	// Convert stored messages to core.Message
	history := make([]core.Message, 0, len(conv.Messages))
	for _, m := range conv.Messages {
		history = append(history, core.Message{
			Role:    core.Role(m.Role),
			Content: m.Content,
		})
	}

	sess := &session{
		ID:             conversationID,
		UserID:         userID,
		ConversationID: conversationID,
		History:        history,
	}
	s.sessions.Store(conn, sess)

	s.send(conn, ServerMessage{
		Type:           "conversation_resumed",
		ConversationID: conversationID,
		Messages:       conv.Messages,
	})

	log.Printf("Resumed conversation %s for user %s", conversationID, userID)
	return sess
}

func (s *Server) handleMessage(ctx context.Context, conn *websocket.Conn, sess *session, content string) {
	if content == "" {
		return
	}

	log.Printf("[CONVERSATION %s] USER: %s", sess.ConversationID, truncate(content, 50))

	// Add to history
	sess.History = append(sess.History, core.NewUserMessage(content))
	sess.TurnCount++

	// Persist user message
	s.persistMessage(ctx, sess.ConversationID, "user", content)

	// Build input
	agentCtx := core.NewContext(sess.UserID, sess.ID, sess.ConversationID, sess.ID)

	input := &engine.Input{
		UserMessage:  content,
		Context:      agentCtx,
		History:      sess.History[:len(sess.History)-1],
		SystemPrompt: s.config.SystemPrompt,
		Model:        s.config.Model,
		MaxTokens:    s.config.MaxTokens,
		StreamCallback: func(chunk string, done bool) {
			if !done && chunk != "" {
				s.send(conn, ServerMessage{Type: "text_chunk", Content: chunk})
			}
		},
	}

	// Run agent
	output, err := s.engine.Run(ctx, input)
	if err != nil {
		log.Printf("Agent error: %v", err)
		s.sendError(conn, fmt.Sprintf("Agent error: %v", err))
		return
	}

	s.handleOutput(ctx, conn, sess, output)
}

func (s *Server) handleOutput(ctx context.Context, conn *websocket.Conn, sess *session, output *engine.Output) {
	switch output.Type {
	case engine.OutputComplete:
		log.Printf("[CONVERSATION %s] ASSISTANT: %s", sess.ConversationID, truncate(output.Text, 200))

		sess.History = append(sess.History, core.NewAssistantMessage(output.Text))

		s.persistMessage(ctx, sess.ConversationID, "assistant", output.Text)

		s.send(conn, ServerMessage{Type: "text", Content: output.Text})
		s.send(conn, ServerMessage{
			Type: "complete",
			TokenUsage: &TokenUsage{
				InputTokens:  output.TokensUsed.InputTokens,
				OutputTokens: output.TokensUsed.OutputTokens,
				TotalTokens:  output.TokensUsed.TotalTokens(),
			},
		})

	case engine.OutputConfirmationNeeded:
		pending := output.PendingAction

		// Store confirmation
		if err := s.confirmations.Store(ctx, pending); err != nil {
			log.Printf("Failed to store confirmation: %v", err)
		}

		sess.History = append(sess.History, core.NewAssistantMessageWithBlocks(output.ResponseBlocks))

		s.send(conn, ServerMessage{
			Type:      "confirm_request",
			ActionID:  pending.ID,
			Tool:      pending.Tool,
			Summary:   pending.Summary,
			Content:   output.Text,
			ExpiresAt: time.Unix(pending.ExpiresAt, 0).Format(time.RFC3339),
		})

	case engine.OutputError:
		log.Printf("Agent error: %v", output.Error)
		s.sendError(conn, output.Error.Error())
	}
}

func (s *Server) handleConfirm(ctx context.Context, conn *websocket.Conn, sess *session, userID, actionID string) {
	log.Printf("Processing confirmation for action=%s, user=%s", actionID, userID)

	// Get and remove confirmation
	action, err := s.confirmations.Confirm(ctx, userID, actionID)
	if err != nil {
		s.send(conn, ServerMessage{
			Type:    "text",
			Content: "That action expired. Would you like me to set it up again?",
		})
		s.send(conn, ServerMessage{Type: "complete"})
		return
	}

	// Execute the confirmed tool
	result, err := s.engine.ExecuteTool(ctx, userID, action.Tool, action.Input, action.ID)

	var resultContent string
	var isError bool
	if err != nil {
		resultContent = fmt.Sprintf("Error: %v", err)
		isError = true
	} else if !result.Success {
		resultContent = result.Error
		isError = true
	} else {
		resultBytes, _ := json.Marshal(result.Data)
		resultContent = string(resultBytes)
	}

	// Add tool result to history
	sess.History = append(sess.History, core.NewToolResultMessage([]core.ToolResultContent{
		{ToolUseID: action.BlockID, Content: resultContent, IsError: isError},
	}))

	if isError {
		s.send(conn, ServerMessage{
			Type:    "text",
			Content: fmt.Sprintf("Sorry, that action failed: %s", resultContent),
		})
		s.send(conn, ServerMessage{Type: "complete"})
		return
	}

	// Format success message
	resultMsg := formatToolResult(action.Tool, result.Data)
	sess.History = append(sess.History, core.NewAssistantMessage(resultMsg))

	s.persistMessage(ctx, sess.ConversationID, "assistant", resultMsg)

	s.send(conn, ServerMessage{Type: "text", Content: resultMsg})
	s.send(conn, ServerMessage{Type: "complete"})
}

func (s *Server) handleCancel(ctx context.Context, conn *websocket.Conn, sess *session, userID, actionID string) {
	// Get action first to have the BlockID for history
	action, err := s.confirmations.Get(ctx, userID, actionID)
	if err != nil {
		s.sendError(conn, "Action not found")
		return
	}

	// Cancel the action
	if err := s.confirmations.Cancel(ctx, userID, actionID); err != nil {
		s.sendError(conn, "Failed to cancel action")
		return
	}

	// Add cancelled tool result to history
	sess.History = append(sess.History, core.NewToolResultMessage([]core.ToolResultContent{
		{ToolUseID: action.BlockID, Content: "Cancelled by user", IsError: true},
	}))

	s.send(conn, ServerMessage{Type: "text", Content: "Action cancelled."})
	s.send(conn, ServerMessage{Type: "complete"})
}

func (s *Server) persistMessage(ctx context.Context, conversationID string, role, content string) {
	err := s.conversations.Append(ctx, &store.AppendMessage{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
	})
	if err != nil {
		log.Printf("Failed to persist message: %v", err)
	}
}

func (s *Server) send(conn *websocket.Conn, msg ServerMessage) {
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (s *Server) sendError(conn *websocket.Conn, content string) {
	log.Printf("Sending error: %s", content)
	s.send(conn, ServerMessage{Type: "error", Content: content})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatToolResult(tool string, result interface{}) string {
	switch r := result.(type) {
	case map[string]interface{}:
		if msg, ok := r["message"].(string); ok {
			return msg
		}
		if success, ok := r["success"].(bool); ok && success {
			return "Done! " + tool + " completed successfully."
		}
	}
	return "Action completed."
}
