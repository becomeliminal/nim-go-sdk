package engine

import (
	"encoding/json"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/google/uuid"
)

// Session represents a conversation session.
type Session struct {
	ID             string
	UserID         string
	ConversationID string
	messages       []anthropic.MessageParam
	TurnCount      int
	CreatedAt      time.Time
	Traces         []*core.Trace // Store traces for this session
}

// NewSession creates a new session.
func NewSession(userID, conversationID string) *Session {
	return &Session{
		ID:             uuid.New().String(),
		UserID:         userID,
		ConversationID: conversationID,
		messages:       make([]anthropic.MessageParam, 0),
		TurnCount:      0,
		CreatedAt:      time.Now(),
	}
}

// AddUserMessage adds a user message to the session.
func (s *Session) AddUserMessage(content string) {
	s.messages = append(s.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))
}

// AddAssistantMessage adds an assistant text message.
func (s *Session) AddAssistantMessage(content string) {
	s.messages = append(s.messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(content)))
}

// AddAssistantResponse adds a full Claude response including tool_use blocks.
func (s *Session) AddAssistantResponse(resp *anthropic.Message) {
	var content []anthropic.ContentBlockParamUnion
	for _, block := range resp.Content {
		content = append(content, block.ToParam())
	}

	s.messages = append(s.messages, anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleAssistant,
		Content: content,
	})
}

// AddAssistantBlocks adds filtered content blocks as an assistant message.
// Used when we need to store only a subset of Claude's response (e.g., filtering
// out unprocessed tool_use blocks during confirmation).
func (s *Session) AddAssistantBlocks(blocks []core.ContentBlock) {
	content := convertCoreBlocksToAPI(blocks)
	s.messages = append(s.messages, anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleAssistant,
		Content: content,
	})
}

// AddToolResults adds tool results to continue the conversation.
func (s *Session) AddToolResults(results []anthropic.ContentBlockParamUnion) {
	s.messages = append(s.messages, anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleUser,
		Content: results,
	})
}

// Messages returns the conversation history.
func (s *Session) Messages() []anthropic.MessageParam {
	return s.messages
}

// IncrementTurnCount increments and returns the turn count.
func (s *Session) IncrementTurnCount() int {
	s.TurnCount++
	return s.TurnCount
}

// AddTrace appends a trace to the session
func (s *Session) AddTrace(trace *core.Trace) {
	s.Traces = append(s.Traces, trace)
}

// RestoreHistory restores messages from core.Message history.
func (s *Session) RestoreHistory(history []core.Message) {
	for _, msg := range history {
		if len(msg.ContentBlocks) > 0 {
			blocks := convertCoreBlocksToAPI(msg.ContentBlocks)
			if len(blocks) > 0 {
				switch msg.Role {
				case core.RoleUser:
					s.messages = append(s.messages, anthropic.NewUserMessage(blocks...))
				case core.RoleAssistant:
					s.messages = append(s.messages, anthropic.MessageParam{
						Role:    anthropic.MessageParamRoleAssistant,
						Content: blocks,
					})
				}
			}
		} else if text := msg.GetText(); text != "" {
			if msg.Role == core.RoleUser {
				s.AddUserMessage(text)
			} else {
				s.AddAssistantMessage(text)
			}
		}
	}
}

// convertCoreBlocksToAPI converts core.ContentBlock slice to API-compatible content blocks.
func convertCoreBlocksToAPI(blocks []core.ContentBlock) []anthropic.ContentBlockParamUnion {
	result := make([]anthropic.ContentBlockParamUnion, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case core.TextBlockType:
			if block.Text != "" {
				result = append(result, anthropic.NewTextBlock(block.Text))
			}
		case core.ToolUseBlockType:
			if block.ToolUse != nil {
				var inputData interface{}
				if len(block.ToolUse.Input) > 0 {
					json.Unmarshal(block.ToolUse.Input, &inputData)
				}
				result = append(result, anthropic.NewToolUseBlock(block.ToolUse.ID, inputData, block.ToolUse.Name))
			}
		case core.ToolResultBlockType:
			if block.ToolResult != nil {
				content := block.ToolResult.Content
				if content == "" {
					content = "No output"
				}
				result = append(result, anthropic.NewToolResultBlock(block.ToolResult.ToolUseID, content, block.ToolResult.IsError))
			}
		}
	}
	return result
}
