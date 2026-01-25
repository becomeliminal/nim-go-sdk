// Example: Full Nim agent with Liminal tools and custom extensions.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/executor"
	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
)

func main() {
	// Configuration from environment
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	liminalAPIKey := os.Getenv("LIMINAL_API_KEY")
	liminalBaseURL := os.Getenv("LIMINAL_BASE_URL")
	if liminalBaseURL == "" {
		liminalBaseURL = "https://api.liminal.cash"
	}

	// Create HTTP executor for Liminal tools
	var liminalExecutor core.ToolExecutor
	if liminalAPIKey != "" {
		liminalExecutor = executor.NewHTTPExecutor(executor.HTTPExecutorConfig{
			BaseURL: liminalBaseURL,
			APIKey:  liminalAPIKey,
		})
		log.Println("Liminal API configured")
	} else {
		log.Println("Warning: LIMINAL_API_KEY not set, Liminal tools will not be available")
	}

	// Create server with authentication
	srv, err := server.New(server.Config{
		AnthropicKey: anthropicKey,
		SystemPrompt: nimSystemPrompt,
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    4096,
		AuthFunc:     authenticateRequest,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add Liminal tools if executor is configured
	if liminalExecutor != nil {
		srv.AddTools(tools.LiminalTools(liminalExecutor)...)
	}

	// Add custom tools
	srv.AddTool(createThinkTool())
	srv.AddTool(createCalculateTool())

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Nim agent on :%s", port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("Health check: http://localhost:%s/health", port)

	if err := srv.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// authenticateRequest validates the request and returns a user ID.
// In production, this would validate a JWT or session token.
func authenticateRequest(r *http.Request) (string, error) {
	// Check for token in query param or Authorization header
	token := r.URL.Query().Get("token")
	if token == "" {
		auth := r.Header.Get("Authorization")
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	// For demo purposes, use token as user ID
	// In production, validate the token and extract user ID
	if token == "" {
		token = "demo-user"
	}

	return token, nil
}

// createThinkTool creates a reasoning tool for the agent.
func createThinkTool() core.Tool {
	return tools.New("think").
		Description("Use this tool to think through a problem step by step before taking action. This helps ensure you fully understand the user's request.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"thought": tools.StringProperty("Your reasoning about the current situation"),
		}, "thought")).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Thought string `json:"thought"`
			}
			json.Unmarshal(input, &params)

			// The think tool doesn't execute anything - it just lets Claude reason
			return map[string]interface{}{
				"acknowledged": true,
				"thought":      params.Thought,
			}, nil
		}).
		Build()
}

// createCalculateTool creates a simple calculator tool.
func createCalculateTool() core.Tool {
	return tools.New("calculate").
		Description("Perform basic arithmetic calculations").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"expression": tools.StringProperty("Mathematical expression (e.g., '100 + 50 * 0.05')"),
		}, "expression")).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			json.Unmarshal(input, &params)

			// For safety, we'd use a proper expression parser in production
			// This is just a placeholder
			return map[string]interface{}{
				"expression": params.Expression,
				"note":       "Expression received - implement safe evaluation",
			}, nil
		}).
		Build()
}

const nimSystemPrompt = `You are Nim, a friendly financial assistant for Liminal.

CONVERSATIONAL GUIDELINES:
- Be conversational and helpful, not robotic
- Ask clarifying questions when the user's intent is unclear
- Don't rush to use tools - understand what the user wants first
- Build context naturally through conversation
- Remember details from earlier in the conversation

WHEN TO USE TOOLS:
- Use tools when you have enough information to complete an action
- For queries like "what's my balance", use tools immediately
- For actions like "send money", gather recipient and amount first

WHEN NOT TO USE TOOLS:
- General questions about how things work
- Clarifying what the user wants
- Explaining options or giving advice
- Casual conversation

CONFIRMATION REQUIRED:
- All money movements require explicit user confirmation
- Show clear summary before confirming: amount, recipient, source

PERSONALITY:
- Friendly but professional
- Concise but not terse
- Proactive with helpful suggestions
- Never make assumptions about amounts or recipients

AVAILABLE ACTIONS:
- Check wallet and savings balances
- View transaction history
- Send money to other users (requires confirmation)
- Deposit/withdraw from savings (requires confirmation)
- Search for users by display tag`
