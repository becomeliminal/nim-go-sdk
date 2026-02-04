//go:build onnx

// Memory Example: AI Agent with Memory System
// Demonstrates how the agent learns from past interactions
package main

import (
	"log"
	"os"

	"github.com/becomeliminal/nim-go-sdk/executor"
	"github.com/becomeliminal/nim-go-sdk/memory"
	"github.com/becomeliminal/nim-go-sdk/memory/embedder/onnx"
	"github.com/becomeliminal/nim-go-sdk/memory/store/chromem"
	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
	"github.com/joho/godotenv"
)

func main() {
	// ============================================================================
	// CONFIGURATION
	// ============================================================================
	_ = godotenv.Load()

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("❌ ANTHROPIC_API_KEY environment variable is required")
	}

	liminalBaseURL := os.Getenv("LIMINAL_BASE_URL")
	if liminalBaseURL == "" {
		liminalBaseURL = "https://api.liminal.cash"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ============================================================================
	// LIMINAL EXECUTOR SETUP
	// ============================================================================
	liminalExecutor := executor.NewHTTPExecutor(executor.HTTPExecutorConfig{
		BaseURL: liminalBaseURL,
	})
	log.Println("✅ Liminal API configured")

	// ============================================================================
	// MEMORY SYSTEM SETUP
	// ============================================================================
	log.Println("📦 Setting up memory system...")

	// Create chromem-go store (in-memory vector database)
	store, err := chromem.New()
	if err != nil {
		log.Fatal(err)
	}

	// Create ONNX embedder with all-MiniLM-L6-v2 model
	embedder, err := onnx.New(onnx.Config{
		ModelPath:     "../../models/all-MiniLM-L6-v2/model.onnx",
		TokenizerPath: "../../models/all-MiniLM-L6-v2/tokenizer.json",
		Dimensions:    384,
	})
	if err != nil {
		log.Fatalf("❌ Failed to load ONNX embedder: %v\nRun: cd ../.. && ./scripts/download-model.sh", err)
	}
	defer embedder.Close()

	// Create memory manager
	memoryMgr := memory.NewSimpleManager(store, embedder, &memory.Config{
		Enabled:       true,
		MinSimilarity: 0.3, // Lower threshold for local embedder
	})
	log.Println("✅ Memory system configured (chromem-go + ONNX)")

	// ============================================================================
	// SERVER SETUP
	// ============================================================================
	srv, err := server.New(server.Config{
		AnthropicKey:    anthropicKey,
		SystemPrompt:    memorySystemPrompt,
		Model:           "claude-sonnet-4-20250514",
		MaxTokens:       4096,
		LiminalExecutor: liminalExecutor,
		Memory:          memoryMgr, // Enable memory system
	})
	if err != nil {
		log.Fatal(err)
	}

	// ============================================================================
	// ADD LIMINAL BANKING TOOLS
	// ============================================================================
	srv.AddTools(tools.LiminalTools(liminalExecutor)...)
	log.Println("✅ Added 9 Liminal banking tools")

	// ============================================================================
	// START SERVER
	// ============================================================================
	log.Println("=============================================================")
	log.Println("  Memory System Server Running")
	log.Println("=============================================================")
	log.Println()
	log.Printf("WebSocket: ws://localhost:%s/ws", port)
	log.Printf("Health:    http://localhost:%s/health", port)
	log.Println()
	log.Println("The agent will remember past interactions!")
	log.Println("Try: 'Send $50 to @alice' then in a new conversation")
	log.Println("     'Send $100 to @alice' - it will remember the user ID!")
	log.Println()
	log.Println("Press Ctrl+C to stop")
	log.Println("=============================================================")

	if err := srv.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

const memorySystemPrompt = `You are Nim, a friendly AI financial assistant with memory capabilities.

WHAT YOU DO:
You help users manage their money using Liminal's stablecoin banking platform. You can check balances, review transactions, send money, and manage savings - all through natural conversation.

MEMORY SYSTEM:
You have memory! You remember past interactions and can learn from them:
- If you've previously looked up a username, you may remember the user ID
- You learn from past successful and failed actions
- Use your memory to be more efficient and avoid redundant operations

CONVERSATIONAL STYLE:
- Be warm, friendly, and conversational
- Use casual language when appropriate, but stay professional about money
- Remember context from earlier conversations (not just this conversation!)
- Explain things simply

WHEN TO USE TOOLS:
- Check your memory first before making tool calls
- Use tools immediately for simple queries
- Always confirm before executing money movements

MONEY MOVEMENT RULES:
- ALL money movements require explicit user confirmation
- Show a clear summary before confirming
- Never assume amounts or recipients

AVAILABLE BANKING TOOLS:
- Check wallet balance (get_balance)
- Check savings balance and APY (get_savings_balance)
- View savings rates (get_vault_rates)
- View transaction history (get_transactions)
- Get profile info (get_profile)
- Search for users (search_users)
- Send money (send_money) - requires confirmation
- Deposit to savings (deposit_savings) - requires confirmation
- Withdraw from savings (withdraw_savings) - requires confirmation`
