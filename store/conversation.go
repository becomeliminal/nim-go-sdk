package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryConversations is an in-memory implementation of Conversations.
// Suitable for development and testing. Not suitable for production
// as data is lost on restart and doesn't work across multiple instances.
type MemoryConversations struct {
	mu            sync.RWMutex
	conversations map[string]*ConversationWithMessages
	byUser        map[string][]string // userID -> []conversationID
}

// NewMemoryConversations creates a new in-memory conversation store.
func NewMemoryConversations() *MemoryConversations {
	return &MemoryConversations{
		conversations: make(map[string]*ConversationWithMessages),
		byUser:        make(map[string][]string),
	}
}

func (m *MemoryConversations) Create(ctx context.Context, userID string) (*Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	conv := &ConversationWithMessages{
		Conversation: Conversation{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "New conversation",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Messages: []StoredMessage{},
	}

	m.conversations[conv.ID] = conv
	m.byUser[userID] = append(m.byUser[userID], conv.ID)

	return &conv.Conversation, nil
}

func (m *MemoryConversations) Get(ctx context.Context, conversationID string) (*ConversationWithMessages, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return nil, fmt.Errorf("conversation not found: %s", conversationID)
	}
	return conv, nil
}

func (m *MemoryConversations) Append(ctx context.Context, msg *AppendMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[msg.ConversationID]
	if !ok {
		return fmt.Errorf("conversation not found: %s", msg.ConversationID)
	}

	msgID := msg.ID
	if msgID == "" {
		msgID = uuid.New().String()
	}
	stored := StoredMessage{
		ID:        msgID,
		Role:      msg.Role,
		Content:   msg.Content,
		Blocks:    msg.Blocks,
		Tools:     msg.Tools,
		CreatedAt: time.Now(),
	}

	conv.Messages = append(conv.Messages, stored)
	conv.UpdatedAt = time.Now()

	return nil
}

func (m *MemoryConversations) SetTitle(ctx context.Context, conversationID, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return fmt.Errorf("conversation not found: %s", conversationID)
	}

	conv.Title = title
	conv.UpdatedAt = time.Now()
	return nil
}

func (m *MemoryConversations) List(ctx context.Context, userID string, limit int) ([]*Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	convIDs, ok := m.byUser[userID]
	if !ok {
		return []*Conversation{}, nil
	}

	// Return most recent first
	result := make([]*Conversation, 0, limit)
	for i := len(convIDs) - 1; i >= 0 && len(result) < limit; i-- {
		if conv, ok := m.conversations[convIDs[i]]; ok {
			result = append(result, &conv.Conversation)
		}
	}

	return result, nil
}

func (m *MemoryConversations) Delete(ctx context.Context, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return fmt.Errorf("conversation not found: %s", conversationID)
	}

	// Remove from byUser index
	userConvs := m.byUser[conv.UserID]
	for i, id := range userConvs {
		if id == conversationID {
			m.byUser[conv.UserID] = append(userConvs[:i], userConvs[i+1:]...)
			break
		}
	}

	delete(m.conversations, conversationID)
	return nil
}

// Verify MemoryConversations implements Conversations.
var _ Conversations = (*MemoryConversations)(nil)
