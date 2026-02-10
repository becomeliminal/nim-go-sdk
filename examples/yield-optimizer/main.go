// Yield Optimizer: Cross-Protocol DeFi Yield Agent
// Scans yields across Aave V3 and Liminal/Morpho, suggests optimal allocation,
// and executes deposits/withdrawals with user confirmation.
package main

import (
	"log"
	"os"

	"github.com/becomeliminal/nim-go-sdk/executor"
	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
	"github.com/becomeliminal/nim-go-sdk/examples/yield-optimizer/agent"
	"github.com/becomeliminal/nim-go-sdk/examples/yield-optimizer/defi"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	liminalBaseURL := os.Getenv("LIMINAL_BASE_URL")
	if liminalBaseURL == "" {
		liminalBaseURL = "https://api.liminal.cash"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Wallet address for on-chain reads and Aave interactions
	// This is the Liminal-managed wallet that sends transactions
	walletAddress := os.Getenv("WALLET_ADDRESS")
	if walletAddress == "" {
		log.Println("WARNING: WALLET_ADDRESS not set — Aave position reads and deposits will be limited")
	}

	// Liminal executor for banking API calls (JWT auth handled automatically)
	liminalExecutor := executor.NewHTTPExecutor(executor.HTTPExecutorConfig{
		BaseURL: liminalBaseURL,
	})

	// Arbitrum RPC client for on-chain reads
	rpcClient := defi.NewRPCClient(defi.ArbitrumRPC, defi.ArbitrumRPCFallback)

	// Aave V3 client for reading supply rates and balances
	aaveClient := defi.NewAaveClient(rpcClient)

	// DefiLlama client for yield enrichment (TVL, metadata)
	defiLlamaClient := defi.NewDefiLlamaClient()

	// Pendle client for fixed-rate stablecoin markets
	pendleClient := defi.NewPendleClient()

	// Create server with Claude
	srv, err := server.New(server.Config{
		AnthropicKey:    anthropicKey,
		SystemPrompt:    agent.SystemPrompt,
		Model:           "claude-sonnet-4-20250514",
		MaxTokens:       4096,
		LiminalExecutor: liminalExecutor,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Register Liminal banking tools (balance, savings, send, etc.)
	srv.AddTools(tools.LiminalTools(liminalExecutor)...)
	log.Println("Added 10 Liminal banking tools")

	// Register custom yield optimizer tools
	deps := &agent.ToolDeps{
		Aave:          aaveClient,
		DefiLlama:     defiLlamaClient,
		Pendle:        pendleClient,
		Executor:      liminalExecutor,
		WalletAddress: walletAddress,
	}
	srv.AddTools(agent.CreateTools(deps)...)
	log.Println("Added 5 yield optimizer tools")

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("DeFi Yield Optimizer Agent Running")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("Health check: http://localhost:%s/health", port)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("Protocols: Aave V3 + Morpho + Pendle on Arbitrum")
	log.Printf("Arbitrum RPC: %s", defi.ArbitrumRPC)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("Start the frontend with: cd frontend && npm run dev")

	if err := srv.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
