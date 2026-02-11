// Hackathon Starter: Complete AI Financial Agent
// Build intelligent financial tools with nim-go-sdk + Liminal banking APIs
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/executor"
	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func main() {
	// ============================================================================
	// CONFIGURATION
	// ============================================================================
	// Load .env file if it exists (optional - will use system env vars if not found)
	_ = godotenv.Load()

	// Load configuration from environment variables
	// Create a .env file or export these in your shell

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
	// The HTTPExecutor handles all API calls to Liminal banking services.
	// Authentication is handled automatically via JWT tokens passed from the
	// frontend login flow (email/OTP). No API key needed!

	liminalExecutor := executor.NewHTTPExecutor(executor.HTTPExecutorConfig{
		BaseURL: liminalBaseURL,
	})
	log.Println("✅ Liminal API configured")

	// ============================================================================
	// SERVER SETUP
	// ============================================================================
	// Create the nim-go-sdk server with Claude AI
	// The server handles WebSocket connections and manages conversations
	// Authentication is automatic: JWT tokens from the login flow are extracted
	// from WebSocket connections and forwarded to Liminal API calls

	// Inject current date into system prompt so Claude can calculate future dates
	systemPrompt := fmt.Sprintf("%s\n\nCURRENT DATE AND TIME: %s", hackathonSystemPrompt, time.Now().UTC().Format("2006-01-02T15:04:05Z (Monday, January 2, 2006)"))

	srv, err := server.New(server.Config{
		AnthropicKey:    anthropicKey,
		SystemPrompt:    systemPrompt,
		Model:           "claude-sonnet-4-20250514",
		MaxTokens:       4096,
		LiminalExecutor: liminalExecutor, // SDK automatically handles JWT extraction and forwarding
	})
	if err != nil {
		log.Fatal(err)
	}

	// ============================================================================
	// ADD LIMINAL BANKING TOOLS
	// ============================================================================
	// These are the 9 core Liminal tools that give your AI access to real banking:
	//
	// READ OPERATIONS (no confirmation needed):
	//   1. get_balance - Check wallet balance
	//   2. get_savings_balance - Check savings positions and APY
	//   3. get_vault_rates - Get current savings rates
	//   4. get_transactions - View transaction history
	//   5. get_profile - Get user profile info
	//   6. search_users - Find users by display tag
	//
	// WRITE OPERATIONS (require user confirmation):
	//   7. send_money - Send money to another user
	//   8. deposit_savings - Deposit funds into savings
	//   9. withdraw_savings - Withdraw funds from savings

	srv.AddTools(tools.LiminalTools(liminalExecutor)...)
	log.Println("✅ Added 9 Liminal banking tools")

	// ============================================================================
	// ADD CUSTOM TOOLS
	// ============================================================================
	// This is where you'll add your hackathon project's custom tools!
	// Below is an example spending analyzer tool to get you started.

	srv.AddTool(createSpendingAnalyzerTool(liminalExecutor))
	log.Println("✅ Added custom spending analyzer tool")

	// ============================================================================
	// SCHEDULED PAYMENTS
	// ============================================================================
	// SQLite-backed scheduled payments with validation and background execution.

	paymentStore, err := NewPaymentStore("scheduled_payments.db")
	if err != nil {
		log.Fatalf("❌ Failed to initialize payment store: %v", err)
	}
	defer paymentStore.Close()

	srv.AddTool(createSchedulePaymentTool(paymentStore, liminalExecutor))
	srv.AddTool(createListScheduledPaymentsTool(paymentStore))
	srv.AddTool(createCancelScheduledPaymentTool(paymentStore))
	srv.AddTool(createCheckBalanceTool(paymentStore, liminalExecutor))
	srv.AddTool(createSendMoneyWrapper(paymentStore, liminalExecutor)) // overwrites native send_money
	log.Println("✅ Added scheduled payment tools + balance checker + send_money guard")

	startScheduler(paymentStore, liminalExecutor)
	log.Println("✅ Started payment scheduler (checks every 30s)")

	// ============================================================================
	// START SERVER
	// ============================================================================

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 Hackathon Starter Server Running")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📡 WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("💚 Health check: http://localhost:%s/health", port)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("Ready for connections! Start your frontend with: cd ../frontend && npm run dev")
	log.Println()

	if err := srv.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// SYSTEM PROMPT
// ============================================================================
// This prompt defines your AI agent's personality and behavior
// Customize this to match your hackathon project's focus!

const hackathonSystemPrompt = `You are Nim, a friendly AI financial assistant built for the Liminal Vibe Banking Hackathon.

WHAT YOU DO:
You help users manage their money using Liminal's banking platform. You can check balances, review transactions, send money, manage savings, and schedule future payments - all through natural conversation.

CONVERSATIONAL STYLE:
- Be warm, friendly, and conversational - not robotic
- Use casual language when appropriate, but stay professional about money
- Ask clarifying questions when something is unclear
- Remember context from earlier in the conversation
- Explain things simply without being condescending
- Use everyday financial language: "$50.00" not "50 USDC", "euros" not "EURC", "LIL" is fine as-is
- Never mention blockchain names, chain IDs, or crypto jargon unless the user asks
- You are a financial co-pilot, not a crypto assistant

WHEN TO USE TOOLS:
- Use tools immediately for simple queries ("what's my balance?")
- For actions, gather all required info first ("send $50 to @alice")
- Always confirm before executing money movements
- Don't use tools for general questions about how things work

MONEY MOVEMENT RULES (IMPORTANT):
- ALL money movements require explicit user confirmation
- Show a clear summary before confirming:
  * send_money: "Send $50 USD to @alice"
  * deposit_savings: "Deposit $100 USD into savings"
  * withdraw_savings: "Withdraw $50 USD from savings"
  * schedule_payment: "Schedule $50 USD to @alice on Monday Feb 16"
- Never assume amounts or recipients
- Always use the exact currency the user specified

AVAILABLE BANKING TOOLS:
- Check wallet balance with scheduled payment info (check_balance) -- ALWAYS USE THIS instead of get_balance
- Check savings balance and APY (get_savings_balance)
- View savings rates (get_vault_rates)
- View transaction history (get_transactions)
- Get profile info (get_profile)
- Search for users (search_users)
- Send money (send_money) - requires confirmation
- Deposit to savings (deposit_savings) - requires confirmation
- Withdraw from savings (withdraw_savings) - requires confirmation

SCHEDULED PAYMENT TOOLS:
- Schedule a future payment (schedule_payment) - validates recipient and balance, requires confirmation
- List upcoming scheduled payments (list_scheduled_payments)
- Cancel a scheduled payment (cancel_scheduled_payment)

CUSTOM ANALYTICAL TOOLS:
- Analyze spending patterns (analyze_spending)

BALANCE DISPLAY RULES:
- ALWAYS use check_balance instead of get_balance when the user asks about their balance
- check_balance shows both the total balance AND the available balance after scheduled payments
- Format example: "$500.00 ($380.00 available after 3 scheduled payments)"
- If there are no scheduled payments, just show the balance normally

SCHEDULED PAYMENT RULES:
- When users want to send money at a future date, use schedule_payment
- Convert relative dates to ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ):
  * "tomorrow" = next day at 09:00 UTC
  * "next Monday" = coming Monday at 09:00 UTC
  * "in 3 days" = 3 days from now at 09:00 UTC
  * "March 1st" = March 1 at 09:00 UTC
- Always show the interpreted date in your response so the user can verify
- The system validates that the recipient exists and that sufficient funds are available
- Scheduled payments are automatically executed when the time comes

TIPS FOR GREAT INTERACTIONS:
- Proactively suggest relevant actions ("Want me to move some to savings?")
- Explain the "why" behind suggestions
- Celebrate financial wins ("Nice! Your savings earned $5 this month!")
- Be encouraging about savings goals
- Make finance feel less intimidating
- If a user has scheduled payments, mention them when relevant

Remember: You're here to make banking delightful and help users build better financial habits!`

// ============================================================================
// CUSTOM TOOL: SPENDING ANALYZER
// ============================================================================
// This is an example custom tool that demonstrates how to:
// 1. Define tool parameters with JSON schema
// 2. Call other Liminal tools from within your tool
// 3. Process and analyze the data
// 4. Return useful insights
//
// Use this as a template for your own hackathon tools!

func createSpendingAnalyzerTool(liminalExecutor core.ToolExecutor) core.Tool {
	return tools.New("analyze_spending").
		Description("Analyze the user's spending patterns over a specified time period. Returns insights about spending velocity, categories, and trends.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"days": tools.IntegerProperty("Number of days to analyze (default: 30)"),
		})).
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			// Parse input parameters
			var params struct {
				Days int `json:"days"`
			}
			if err := json.Unmarshal(toolParams.Input, &params); err != nil {
				return &core.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("invalid input: %v", err),
				}, nil
			}

			// Default to 30 days if not specified
			if params.Days == 0 {
				params.Days = 30
			}

			// STEP 1: Fetch transaction history
			// We'll call the Liminal get_transactions tool through the executor
			txRequest := map[string]interface{}{
				"limit": 100, // Get up to 100 transactions
			}
			txRequestJSON, _ := json.Marshal(txRequest)

			txResponse, err := liminalExecutor.Execute(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "get_transactions",
				Input:     txRequestJSON,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("failed to fetch transactions: %v", err),
				}, nil
			}

			if !txResponse.Success {
				return &core.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("transaction fetch failed: %s", txResponse.Error),
				}, nil
			}

			// STEP 2: Parse transaction data
			// In a real implementation, you'd parse the actual response structure
			// For now, we'll create a structured analysis

			var transactions []map[string]interface{}
			var txData map[string]interface{}
			if err := json.Unmarshal(txResponse.Data, &txData); err == nil {
				if txArray, ok := txData["transactions"].([]interface{}); ok {
					for _, tx := range txArray {
						if txMap, ok := tx.(map[string]interface{}); ok {
							transactions = append(transactions, txMap)
						}
					}
				}
			}

			// STEP 3: Analyze the data
			analysis := analyzeTransactions(transactions, params.Days)

			// STEP 4: Return insights
			result := map[string]interface{}{
				"period_days":        params.Days,
				"total_transactions": len(transactions),
				"analysis":           analysis,
				"generated_at":       time.Now().Format(time.RFC3339),
			}

			return &core.ToolResult{
				Success: true,
				Data:    result,
			}, nil
		}).
		Build()
}

// analyzeTransactions processes transaction data and returns insights
func analyzeTransactions(transactions []map[string]interface{}, days int) map[string]interface{} {
	if len(transactions) == 0 {
		return map[string]interface{}{
			"summary": "No transactions found in the specified period",
		}
	}

	// Calculate basic metrics
	var totalSpent, totalReceived float64
	var spendCount, receiveCount int

	// This is a simplified example - you'd do real analysis here:
	// - Group by category/merchant
	// - Calculate daily/weekly averages
	// - Identify spending spikes
	// - Compare to previous periods
	// - Detect recurring payments

	for _, tx := range transactions {
		// Example analysis logic
		txType, _ := tx["type"].(string)
		amount, _ := tx["amount"].(float64)

		switch txType {
		case "send":
			totalSpent += amount
			spendCount++
		case "receive":
			totalReceived += amount
			receiveCount++
		}
	}

	avgDailySpend := totalSpent / float64(days)

	return map[string]interface{}{
		"total_spent":     fmt.Sprintf("%.2f", totalSpent),
		"total_received":  fmt.Sprintf("%.2f", totalReceived),
		"spend_count":     spendCount,
		"receive_count":   receiveCount,
		"avg_daily_spend": fmt.Sprintf("%.2f", avgDailySpend),
		"velocity":        calculateVelocity(spendCount, days),
		"insights": []string{
			fmt.Sprintf("You made %d spending transactions over %d days", spendCount, days),
			fmt.Sprintf("Average daily spend: $%.2f", avgDailySpend),
			"Consider setting up savings goals to build financial cushion",
		},
	}
}

// calculateVelocity determines spending frequency
func calculateVelocity(transactionCount, days int) string {
	txPerWeek := float64(transactionCount) / float64(days) * 7

	switch {
	case txPerWeek < 2:
		return "low"
	case txPerWeek < 7:
		return "moderate"
	default:
		return "high"
	}
}

// ============================================================================
// SCHEDULED PAYMENTS: SQLite STORE
// ============================================================================

// ScheduledPayment represents a payment scheduled for future execution.
type ScheduledPayment struct {
	ID          string    `json:"id"`
	Recipient   string    `json:"recipient"`
	Amount      string    `json:"amount"`
	Currency    string    `json:"currency"`
	Note        string    `json:"note,omitempty"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"` // pending, executing, executed, failed, cancelled
	Error       string    `json:"error,omitempty"`
}

// PaymentStore provides SQLite-backed storage for scheduled payments.
type PaymentStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewPaymentStore opens (or creates) a SQLite database and initializes the schema.
func NewPaymentStore(path string) (*PaymentStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Create table if it doesn't exist
	createSQL := `CREATE TABLE IF NOT EXISTS scheduled_payments (
		id TEXT PRIMARY KEY,
		recipient TEXT NOT NULL,
		amount TEXT NOT NULL,
		currency TEXT NOT NULL,
		note TEXT DEFAULT '',
		scheduled_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		error TEXT DEFAULT ''
	)`
	if _, err := db.Exec(createSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &PaymentStore{db: db}, nil
}

// Add inserts a new scheduled payment.
func (s *PaymentStore) Add(p *ScheduledPayment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO scheduled_payments (id, recipient, amount, currency, note, scheduled_at, created_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Recipient, p.Amount, p.Currency, p.Note,
		p.ScheduledAt.UTC().Format(time.RFC3339),
		p.CreatedAt.UTC().Format(time.RFC3339),
		p.Status,
	)
	return err
}

// GetPending returns all payments with status "pending", ordered by scheduled time.
func (s *PaymentStore) GetPending() ([]*ScheduledPayment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, recipient, amount, currency, note, scheduled_at, created_at, status, error
		 FROM scheduled_payments WHERE status = 'pending' ORDER BY scheduled_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPayments(rows)
}

// GetDue returns pending payments whose scheduled time has passed.
func (s *PaymentStore) GetDue() ([]*ScheduledPayment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.Query(
		`SELECT id, recipient, amount, currency, note, scheduled_at, created_at, status, error
		 FROM scheduled_payments WHERE status = 'pending' AND scheduled_at <= ? ORDER BY scheduled_at ASC`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPayments(rows)
}

// GetPendingTotalByCurrency returns the total pending amount per currency.
func (s *PaymentStore) GetPendingTotalByCurrency() (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT currency, SUM(CAST(amount AS REAL)) as total
		 FROM scheduled_payments WHERE status = 'pending' GROUP BY currency`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make(map[string]float64)
	for rows.Next() {
		var currency string
		var total float64
		if err := rows.Scan(&currency, &total); err != nil {
			return nil, err
		}
		totals[currency] = total
	}
	return totals, rows.Err()
}

// UpdateStatus updates the status (and optional error message) of a payment.
func (s *PaymentStore) UpdateStatus(id, status, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`UPDATE scheduled_payments SET status = ?, error = ? WHERE id = ?`,
		status, errMsg, id,
	)
	return err
}

// Cancel sets a pending payment's status to "cancelled".
func (s *PaymentStore) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		`UPDATE scheduled_payments SET status = 'cancelled' WHERE id = ? AND status = 'pending'`,
		id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("payment %s not found or not in pending status", id)
	}
	return nil
}

// Close closes the database connection.
func (s *PaymentStore) Close() error {
	return s.db.Close()
}

// scanPayments reads rows into ScheduledPayment slices.
func scanPayments(rows *sql.Rows) ([]*ScheduledPayment, error) {
	var payments []*ScheduledPayment
	for rows.Next() {
		p := &ScheduledPayment{}
		var scheduledAt, createdAt string
		if err := rows.Scan(&p.ID, &p.Recipient, &p.Amount, &p.Currency, &p.Note,
			&scheduledAt, &createdAt, &p.Status, &p.Error); err != nil {
			return nil, err
		}
		p.ScheduledAt, _ = time.Parse(time.RFC3339, scheduledAt)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// ============================================================================
// TOOL: schedule_payment (with confirmation + validation)
// ============================================================================

func createSchedulePaymentTool(store *PaymentStore, liminalExecutor core.ToolExecutor) core.Tool {
	return tools.New("schedule_payment").
		Description("Schedule a payment for a future date/time. The payment will be automatically sent when the time comes. Validates that the recipient exists and sufficient funds are available. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. 'LIL' stays as 'LIL'.").
		Schema(tools.BuildSchemaWithThought(map[string]interface{}{
			"recipient":    tools.StringProperty("Recipient's display tag (e.g., @alice) or user ID"),
			"amount":       tools.StringProperty("Amount to send (e.g., '50.00')"),
			"currency":     tools.StringProperty("Currency: 'USDC' for dollars, 'EURC' for euros, 'LIL' for LIL"),
			"scheduled_at": tools.StringProperty("ISO 8601 datetime for when to send (e.g., '2026-02-16T09:00:00Z')"),
			"note":         tools.StringProperty("Optional payment note"),
		}, true, "recipient", "amount", "currency", "scheduled_at")).
		RequiresConfirmation().
		SummaryTemplate("Schedule {{.amount}} {{.currency}} to {{.recipient}} on {{.scheduled_at}}").
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			// Parse input
			var params struct {
				Recipient   string `json:"recipient"`
				Amount      string `json:"amount"`
				Currency    string `json:"currency"`
				ScheduledAt string `json:"scheduled_at"`
				Note        string `json:"note"`
			}
			if err := json.Unmarshal(toolParams.Input, &params); err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid input: %v", err)}, nil
			}

			// Validate amount is a positive number
			amountFloat, err := strconv.ParseFloat(params.Amount, 64)
			if err != nil || amountFloat <= 0 {
				return &core.ToolResult{Success: false, Error: "amount must be a positive number"}, nil
			}

			// Validate scheduled_at is in the future
			scheduledAt, err := time.Parse(time.RFC3339, params.ScheduledAt)
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid date format, expected ISO 8601: %v", err)}, nil
			}
			if scheduledAt.Before(time.Now()) {
				return &core.ToolResult{Success: false, Error: "scheduled date must be in the future"}, nil
			}

			// VALIDATION 1: Check recipient exists via search_users
			// Strip @ prefix for search (API expects plain name)
			searchQuery := strings.TrimPrefix(params.Recipient, "@")
			searchInput, _ := json.Marshal(map[string]interface{}{"query": searchQuery})
			searchResp, err := liminalExecutor.Execute(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "search_users",
				Input:     searchInput,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to validate recipient: %v", err)}, nil
			}
			if !searchResp.Success {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("recipient not found: %s", params.Recipient)}, nil
			}
			// Check that search actually returned results
			var searchData map[string]interface{}
			if err := json.Unmarshal(searchResp.Data, &searchData); err == nil {
				if users, ok := searchData["users"].([]interface{}); ok && len(users) == 0 {
					return &core.ToolResult{Success: false, Error: fmt.Sprintf("recipient not found: %s", params.Recipient)}, nil
				}
			}

			// VALIDATION 2: Check available balance
			balanceInput, _ := json.Marshal(map[string]interface{}{"currency": params.Currency})
			balanceResp, err := liminalExecutor.Execute(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "get_balance",
				Input:     balanceInput,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to check balance: %v", err)}, nil
			}
			if !balanceResp.Success {
				return &core.ToolResult{Success: false, Error: "failed to retrieve balance"}, nil
			}

			// Parse balance from response
			currentBalance := parseBalanceFromResponse(balanceResp.Data, params.Currency)

			// Get pending scheduled totals
			pendingTotals, err := store.GetPendingTotalByCurrency()
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to check pending payments: %v", err)}, nil
			}

			available := currentBalance - pendingTotals[params.Currency]
			if available < amountFloat {
				return &core.ToolResult{
					Success: false,
					Error: fmt.Sprintf("insufficient available balance: %.2f %s (%.2f reserved in scheduled payments)",
						currentBalance, params.Currency, pendingTotals[params.Currency]),
				}, nil
			}

			// All validations passed -- save to SQLite
			payment := &ScheduledPayment{
				ID:          uuid.New().String(),
				Recipient:   params.Recipient,
				Amount:      params.Amount,
				Currency:    params.Currency,
				Note:        params.Note,
				ScheduledAt: scheduledAt,
				CreatedAt:   time.Now(),
				Status:      "pending",
			}

			if err := store.Add(payment); err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to save payment: %v", err)}, nil
			}

			return &core.ToolResult{
				Success: true,
				Data: map[string]interface{}{
					"payment_id":   payment.ID,
					"recipient":    payment.Recipient,
					"amount":       payment.Amount,
					"currency":     payment.Currency,
					"scheduled_at": payment.ScheduledAt.Format(time.RFC3339),
					"status":       "pending",
					"message":      fmt.Sprintf("Payment of %s %s to %s scheduled for %s", payment.Amount, payment.Currency, payment.Recipient, payment.ScheduledAt.Format("Mon Jan 2, 2006 at 3:04 PM UTC")),
				},
			}, nil
		}).
		Build()
}

// parseBalanceFromResponse extracts the balance for a given currency from the get_balance response.
func parseBalanceFromResponse(data json.RawMessage, currency string) float64 {
	// Try to parse the balance response -- structure may vary
	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		return 0
	}

	// Try common response formats
	// Format 1: {"balances": [{"currency": "USDC", "amount": "10", ...}, ...]}
	if balances, ok := response["balances"].([]interface{}); ok {
		for _, b := range balances {
			if bMap, ok := b.(map[string]interface{}); ok {
				if cur, _ := bMap["currency"].(string); cur == currency {
					// Try "amount" field (Liminal API format)
					for _, field := range []string{"amount", "balance"} {
						if bal, ok := bMap[field].(string); ok {
							f, _ := strconv.ParseFloat(bal, 64)
							if f > 0 {
								return f
							}
						}
						if bal, ok := bMap[field].(float64); ok {
							if bal > 0 {
								return bal
							}
						}
					}
				}
			}
		}
	}

	// Format 2: {"balance": "500.00"} or {"balance": 500.00}
	if bal, ok := response["balance"].(string); ok {
		f, _ := strconv.ParseFloat(bal, 64)
		return f
	}
	if bal, ok := response["balance"].(float64); ok {
		return bal
	}

	// Format 3: {"USDC": "500.00"} or {"USDC": 500.00}
	if bal, ok := response[currency].(string); ok {
		f, _ := strconv.ParseFloat(bal, 64)
		return f
	}
	if bal, ok := response[currency].(float64); ok {
		return bal
	}

	return 0
}

// ============================================================================
// TOOL: list_scheduled_payments (read-only)
// ============================================================================

func createListScheduledPaymentsTool(store *PaymentStore) core.Tool {
	return tools.New("list_scheduled_payments").
		Description("List all pending scheduled payments that haven't been sent yet. Shows payment ID, recipient, amount, currency, and scheduled date.").
		Schema(tools.ObjectSchema(map[string]interface{}{})).
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			payments, err := store.GetPending()
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to list payments: %v", err)}, nil
			}

			if len(payments) == 0 {
				return &core.ToolResult{
					Success: true,
					Data: map[string]interface{}{
						"payments": []interface{}{},
						"count":    0,
						"message":  "No scheduled payments found.",
					},
				}, nil
			}

			var paymentList []map[string]interface{}
			for _, p := range payments {
				paymentList = append(paymentList, map[string]interface{}{
					"id":           p.ID,
					"recipient":    p.Recipient,
					"amount":       p.Amount,
					"currency":     p.Currency,
					"note":         p.Note,
					"scheduled_at": p.ScheduledAt.Format(time.RFC3339),
					"scheduled_display": p.ScheduledAt.Format("Mon Jan 2, 2006 at 3:04 PM UTC"),
					"created_at":   p.CreatedAt.Format(time.RFC3339),
				})
			}

			return &core.ToolResult{
				Success: true,
				Data: map[string]interface{}{
					"payments": paymentList,
					"count":    len(paymentList),
				},
			}, nil
		}).
		Build()
}

// ============================================================================
// TOOL: cancel_scheduled_payment
// ============================================================================

func createCancelScheduledPaymentTool(store *PaymentStore) core.Tool {
	return tools.New("cancel_scheduled_payment").
		Description("Cancel a scheduled payment that hasn't been sent yet. Requires the payment ID.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"payment_id": tools.StringProperty("ID of the scheduled payment to cancel (from list_scheduled_payments)"),
		}, "payment_id")).
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			var params struct {
				PaymentID string `json:"payment_id"`
			}
			if err := json.Unmarshal(toolParams.Input, &params); err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid input: %v", err)}, nil
			}

			if err := store.Cancel(params.PaymentID); err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to cancel: %v", err)}, nil
			}

			return &core.ToolResult{
				Success: true,
				Data: map[string]interface{}{
					"payment_id": params.PaymentID,
					"status":     "cancelled",
					"message":    "Scheduled payment has been cancelled.",
				},
			}, nil
		}).
		Build()
}

// ============================================================================
// TOOL: check_balance (wrapper with scheduled payment info)
// ============================================================================

func createCheckBalanceTool(store *PaymentStore, liminalExecutor core.ToolExecutor) core.Tool {
	return tools.New("check_balance").
		Description("Check wallet balance with available amounts after scheduled payments. Shows total balance and available balance (total minus pending scheduled payments). ALWAYS use this instead of get_balance. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. 'LIL' stays as 'LIL'.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"currency": tools.StringProperty("Optional: filter by currency (e.g., 'USDC' for dollars, 'EURC' for euros, 'LIL' for LIL)"),
		})).
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			// Step 1: Call the real get_balance via executor
			balanceResp, err := liminalExecutor.Execute(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "get_balance",
				Input:     toolParams.Input,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to fetch balance: %v", err)}, nil
			}
			if !balanceResp.Success {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("balance fetch failed: %s", balanceResp.Error)}, nil
			}

			// Step 2: Get pending scheduled totals
			pendingTotals, err := store.GetPendingTotalByCurrency()
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to check pending payments: %v", err)}, nil
			}

			// Step 3: Get pending payment count
			pendingPayments, err := store.GetPending()
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to list pending payments: %v", err)}, nil
			}

			// Step 4: Build enriched response
			// Pass through the original balance data and add scheduled payment info
			var originalData interface{}
			if err := json.Unmarshal(balanceResp.Data, &originalData); err != nil {
				originalData = string(balanceResp.Data)
			}

			result := map[string]interface{}{
				"balance_data":            originalData,
				"pending_scheduled_count": len(pendingPayments),
			}

			if len(pendingTotals) > 0 {
				result["pending_scheduled_totals"] = pendingTotals

				// Calculate available per currency
				available := make(map[string]string)
				for currency, pending := range pendingTotals {
					balance := parseBalanceFromResponse(balanceResp.Data, currency)
					avail := balance - pending
					if avail < 0 {
						avail = 0
					}
					available[currency] = fmt.Sprintf("%.2f", avail)
				}
				result["available_after_scheduled"] = available
			}

			return &core.ToolResult{
				Success: true,
				Data:    result,
			}, nil
		}).
		Build()
}

// ============================================================================
// TOOL: send_money wrapper (balance guard with scheduled payments)
// ============================================================================
// Overwrites the native Liminal send_money tool to check available balance
// (accounting for pending scheduled payments) before allowing a transfer.

func createSendMoneyWrapper(store *PaymentStore, liminalExecutor core.ToolExecutor) core.Tool {
	return tools.New("send_money").
		Description("Send money to another user. Checks available balance accounting for scheduled payments. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. 'LIL' stays as 'LIL'. Requires confirmation.").
		Schema(tools.BuildSchemaWithThought(map[string]interface{}{
			"recipient": tools.StringProperty("Recipient's display tag (e.g., @alice) or user ID"),
			"amount":    tools.StringProperty("Amount to send (e.g., '50.00')"),
			"currency":  tools.StringProperty("Currency to send. Use 'USDC' for dollars, 'EURC' for euros, 'LIL' for LIL"),
			"note":      tools.StringProperty("Optional payment note"),
		}, true, "recipient", "amount", "currency")).
		RequiresConfirmation().
		SummaryTemplate("Send {{.amount}} {{.currency}} to {{.recipient}}").
		Handler(func(ctx context.Context, toolParams *core.ToolParams) (*core.ToolResult, error) {
			// Parse input to get amount and currency for validation
			var params struct {
				Recipient string `json:"recipient"`
				Amount    string `json:"amount"`
				Currency  string `json:"currency"`
				Note      string `json:"note"`
			}
			if err := json.Unmarshal(toolParams.Input, &params); err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid input: %v", err)}, nil
			}

			amountFloat, err := strconv.ParseFloat(params.Amount, 64)
			if err != nil || amountFloat <= 0 {
				return &core.ToolResult{Success: false, Error: "amount must be a positive number"}, nil
			}

			// Check balance via executor
			balanceInput, _ := json.Marshal(map[string]interface{}{"currency": params.Currency})
			balanceResp, err := liminalExecutor.Execute(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "get_balance",
				Input:     balanceInput,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to check balance: %v", err)}, nil
			}
			if !balanceResp.Success {
				return &core.ToolResult{Success: false, Error: "failed to retrieve balance"}, nil
			}

			currentBalance := parseBalanceFromResponse(balanceResp.Data, params.Currency)

			// Subtract pending scheduled payments
			pendingTotals, err := store.GetPendingTotalByCurrency()
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("failed to check pending payments: %v", err)}, nil
			}

			available := currentBalance - pendingTotals[params.Currency]
			if available < amountFloat {
				return &core.ToolResult{
					Success: false,
					Error: fmt.Sprintf(
						"insufficient available balance: you have %.2f %s but %.2f is reserved for scheduled payments (available: %.2f %s)",
						currentBalance, params.Currency, pendingTotals[params.Currency], available, params.Currency,
					),
				}, nil
			}

			// Balance OK -- delegate to the real Liminal send_money
			resp, err := liminalExecutor.ExecuteWrite(ctx, &core.ExecuteRequest{
				UserID:    toolParams.UserID,
				Tool:      "send_money",
				Input:     toolParams.Input,
				RequestID: toolParams.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("send failed: %v", err)}, nil
			}
			if !resp.Success {
				return &core.ToolResult{Success: false, Error: resp.Error}, nil
			}

			// Return the original Liminal response
			return &core.ToolResult{
				Success: true,
				Data:    resp.Data,
			}, nil
		}).
		Build()
}

// ============================================================================
// BACKGROUND SCHEDULER
// ============================================================================
// Runs every 30 seconds, finds due payments, and executes them via send_money.

func startScheduler(store *PaymentStore, liminalExecutor core.ToolExecutor) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			due, err := store.GetDue()
			if err != nil {
				log.Printf("⚠️  Scheduler: failed to get due payments: %v", err)
				continue
			}

			for _, p := range due {
				log.Printf("📤 Scheduler: executing payment %s (%s %s to %s)", p.ID, p.Amount, p.Currency, p.Recipient)

				// Mark as executing to prevent double-execution
				if err := store.UpdateStatus(p.ID, "executing", ""); err != nil {
					log.Printf("⚠️  Scheduler: failed to mark %s as executing: %v", p.ID, err)
					continue
				}

				// Build send_money request
				sendInput, _ := json.Marshal(map[string]interface{}{
					"recipient": p.Recipient,
					"amount":    p.Amount,
					"currency":  p.Currency,
					"note":      p.Note,
					"thought":   fmt.Sprintf("Executing scheduled payment %s", p.ID),
				})

				resp, err := liminalExecutor.ExecuteWrite(context.Background(), &core.ExecuteRequest{
					Tool:  "send_money",
					Input: sendInput,
				})

				if err != nil {
					errMsg := fmt.Sprintf("execution error: %v", err)
					log.Printf("❌ Scheduler: payment %s failed: %s", p.ID, errMsg)
					store.UpdateStatus(p.ID, "failed", errMsg)
					continue
				}

				if !resp.Success {
					errMsg := fmt.Sprintf("send_money failed: %s", resp.Error)
					log.Printf("❌ Scheduler: payment %s failed: %s", p.ID, errMsg)
					store.UpdateStatus(p.ID, "failed", errMsg)
					continue
				}

				store.UpdateStatus(p.ID, "executed", "")
				log.Printf("✅ Scheduler: payment %s executed successfully", p.ID)
			}
		}
	}()
}
