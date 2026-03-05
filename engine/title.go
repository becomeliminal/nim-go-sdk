package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/becomeliminal/nim-go-sdk/core"
)

// TitleGenerationPrompt is the system prompt used for generating conversation titles.
const TitleGenerationPrompt = `Generate a 3-6 word title for this conversation.
Return ONLY the title, no quotes, no punctuation at the end.
The title should capture the main topic or intent of the conversation.
Examples of good titles:
- Check wallet balance
- Send money to Alice
- Savings deposit question
- Transaction history request`

// GenerateTitle creates a short title for a conversation based on its history.
// Uses a small, fast model call to generate a 3-6 word summary.
func (e *Engine) GenerateTitle(ctx context.Context, history []core.Message) (string, error) {
	if len(history) == 0 {
		return "New conversation", nil
	}

	// Convert history to API format
	messages := make([]anthropic.MessageParam, 0, len(history))
	for _, msg := range history {
		switch msg.Role {
		case core.RoleUser:
			if msg.Content != "" {
				messages = append(messages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		case core.RoleAssistant:
			if msg.Content != "" {
				messages = append(messages, anthropic.NewAssistantMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		}
	}

	if len(messages) == 0 {
		return "New conversation", nil
	}

	// Add the title request
	messages = append(messages, anthropic.NewUserMessage(
		anthropic.NewTextBlock("Based on this conversation, generate a short title (3-6 words):"),
	))

	// Use a smaller model for cost efficiency
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 50, // Titles are short
		Messages:  messages,
		System: []anthropic.TextBlockParam{
			{Text: TitleGenerationPrompt},
		},
	}

	resp, err := e.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	// Extract title from response
	for _, block := range resp.Content {
		if block.Type == "text" {
			title := strings.TrimSpace(block.Text)
			// Remove any quotes or trailing punctuation
			title = strings.Trim(title, `"'`)
			title = strings.TrimRight(title, ".!?")
			if title != "" {
				return title, nil
			}
		}
	}

	return "New conversation", nil
}

// GenerateTitleFromFirstMessage creates a title based on just the first user message.
// This is useful when you want to generate a title early in the conversation.
func (e *Engine) GenerateTitleFromFirstMessage(ctx context.Context, message string) (string, error) {
	history := []core.Message{
		{Role: core.RoleUser, Content: message},
	}
	return e.GenerateTitle(ctx, history)
}
