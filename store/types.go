package store

import "time"

// Conversation represents conversation metadata.
type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationWithMessages includes the full message history.
type ConversationWithMessages struct {
	Conversation
	Messages []StoredMessage `json:"messages"`
}

// StoredMessage represents a persisted message.
type StoredMessage struct {
	ID        string        `json:"id"`
	Role      string        `json:"role"`
	Content   string        `json:"content"`
	Blocks    []interface{} `json:"blocks,omitempty"`
	Tools     []interface{} `json:"tools,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// AppendMessage contains data for adding a message to a conversation.
type AppendMessage struct {
	ID             string // Optional pre-generated message UUID.
	ConversationID string
	Role           string
	Content        string
	Blocks         []interface{}
	Tools          []interface{}
}
