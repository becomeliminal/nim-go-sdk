# 💜 Liminal Vibe Banking Hackathon Starter

**Build AI-powered financial tools with real banking APIs in minutes.**

This starter project gives you everything you need to create intelligent financial agents powered by Claude AI and Liminal's banking platform. No complex setup, no mock data — just real banking tools and a beautiful chat interface.

---

## 🎯 What You'll Build

Create conversational AI agents that can:

- 💰 **Check balances** across wallet and savings accounts
- 📊 **Analyze spending** patterns and provide insights
- 💸 **Send money** to other users (with confirmation)
- 🏦 **Manage savings** deposits and withdrawals
- 📈 **Track transactions** and financial history
- 🤖 **Custom analytics** - the sky's the limit!

All through natural conversation in a beautiful chat interface.

---

## 🚀 5-Minute Quickstart

### Prerequisites

- **Go 1.21+** installed ([Download](https://go.dev/dl/))
- **Node.js 18+** installed ([Download](https://nodejs.org/))
- **Anthropic API key** ([Get one](https://console.anthropic.com/))

### Step 1: Clone and Setup

```bash
# Clone the repository
git clone https://github.com/becomeliminal/nim-go-sdk.git
cd nim-go-sdk/examples/hackathon-starter

# Copy environment template
cp .env.example .env
```

### Step 2: Add Your Anthropic API Key

Edit `.env` and add your Anthropic key:

```bash
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

That's it! Liminal authentication is automatic via the login flow in the chat interface.

### Step 3: Start the Backend

```bash
# Install Go dependencies
go mod tidy

# Run the server
go run main.go
```

You should see:
```
✅ Liminal API configured
✅ Added 9 Liminal banking tools
✅ Added custom spending analyzer tool
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 Hackathon Starter Server Running
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📡 WebSocket endpoint: ws://localhost:8080/ws
💚 Health check: http://localhost:8080/health
```

### Step 4: Start the Frontend

In a new terminal:

```bash
cd nim-go-sdk/examples/frontend

# Install dependencies
npm install

# Start dev server
npm run dev
```

Your browser will open to `http://localhost:5173` with a beautiful chat interface!

### Step 5: Login and Try It Out!

Click the chat bubble, then:

1. **Login** with your email (you'll get an OTP code)
2. Enter the code to authenticate
3. Start chatting! Try:
   - "What's my balance?"
   - "Show me my recent transactions"
   - "Analyze my spending over the last 30 days"

Authentication is automatic from this point forward - JWT tokens are managed under the hood.

---

## 🔑 Getting Your Anthropic API Key

1. Go to [console.anthropic.com](https://console.anthropic.com/)
2. Sign up or log in
3. Navigate to API Keys
4. Create a new key
5. Copy the key (starts with `sk-ant-`)
6. Add it to your `.env` file

### Liminal Authentication

**No API key needed!** Liminal authentication works automatically:

1. When you first use the chat, you'll see a login screen
2. Enter your email address
3. Check your email for a one-time code (OTP)
4. Enter the code to authenticate
5. That's it! Your session is authenticated via JWT tokens managed automatically

The SDK handles all JWT token management, extraction, and refreshing under the hood. You never need to manually manage Liminal credentials.

---

## 📁 Project Structure

```
hackathon-starter/
├── main.go              # Backend server with AI agent
│   ├── Configuration    # Environment variables
│   ├── Liminal Tools    # 9 banking tools (balance, transactions, etc.)
│   ├── Custom Tools     # Your hackathon project tools
│   └── System Prompt    # AI personality and behavior
│
├── frontend/            # React chat interface
│   ├── main.tsx         # App entry point with nim-chat widget
│   ├── index.html       # Beautiful landing page
│   ├── package.json     # Dependencies
│   └── vite.config.ts   # Build configuration
│
├── .env.example         # Environment template
├── .env                 # Your actual keys (don't commit!)
├── go.mod               # Go dependencies
└── README.md            # This file
```

---

## 🛠️ Built-in Liminal Banking Tools

Your AI agent has access to **9 core banking tools** out of the box:

### Read Operations (No Confirmation)

| Tool | Description | Example Query |
|------|-------------|---------------|
| `get_balance` | Check wallet balance | "What's my balance?" |
| `get_savings_balance` | Check savings positions & APY | "How much is in my savings?" |
| `get_vault_rates` | Get current savings rates | "What's the APY for savings?" |
| `get_transactions` | View transaction history | "Show my recent transactions" |
| `get_profile` | Get user profile info | "What's my display tag?" |
| `search_users` | Find users by display tag | "Search for @alice" |

### Write Operations (Require Confirmation)

| Tool | Description | Example Query |
|------|-------------|---------------|
| `send_money` | Send money to another user | "Send $50 to @alice" |
| `deposit_savings` | Deposit funds into savings | "Put $100 in savings" |
| `withdraw_savings` | Withdraw funds from savings | "Withdraw $50 from savings" |

All write operations require explicit user confirmation through the chat interface. The UI will show a countdown timer and summary before executing.

---

## 🎨 Adding Custom Tools

This is where your hackathon magic happens! The starter includes an example **spending analyzer** tool. Here's how to add your own:

### 1. Define Your Tool

In `main.go`, add a new tool after the existing ones:

```go
// Add your custom tool
srv.AddTool(createBudgetTrackerTool(liminalExecutor))
```

### 2. Implement the Tool

```go
func createBudgetTrackerTool(liminalExecutor core.ToolExecutor) core.Tool {
    return tools.New("track_budget").
        Description("Track spending against a monthly budget and alert when approaching limits").
        Schema(tools.ObjectSchema(map[string]interface{}{
            "budget_amount": tools.StringProperty("Monthly budget amount (e.g., '1000')"),
            "category": tools.StringProperty("Budget category (e.g., 'dining', 'entertainment')"),
        }, "budget_amount")).
        HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
            // 1. Parse input
            var params struct {
                BudgetAmount string `json:"budget_amount"`
                Category     string `json:"category"`
            }
            json.Unmarshal(input, &params)

            // 2. Fetch transaction data
            txRequest := map[string]interface{}{"limit": 100}
            txRequestJSON, _ := json.Marshal(txRequest)
            txResponse, _ := liminalExecutor.Execute(ctx, "get_transactions", txRequestJSON)

            // 3. Analyze and compare to budget
            spent := calculateCategorySpending(txResponse, params.Category)
            budgetAmount, _ := strconv.ParseFloat(params.Budget Amount, 64)
            percentUsed := (spent / budgetAmount) * 100

            // 4. Return insights
            return map[string]interface{}{
                "budget":        params.BudgetAmount,
                "spent":         fmt.Sprintf("%.2f", spent),
                "remaining":     fmt.Sprintf("%.2f", budgetAmount - spent),
                "percent_used":  fmt.Sprintf("%.1f%%", percentUsed),
                "status":        getBudgetStatus(percentUsed),
                "alert":         percentUsed > 80,
            }, nil
        }).
        Build()
}
```

### 3. Update the System Prompt

Add your new tool to the system prompt in `main.go`:

```go
const hackathonSystemPrompt = `You are Nim...

CUSTOM ANALYTICAL TOOLS:
- Analyze spending patterns (analyze_spending)
- Track budget goals (track_budget)  // <-- Add this
...`
```

### 4. Test It

Restart your backend and try:
- "Track my dining budget of $500"
- "Am I on track with my budget?"

---

## 💡 Hackathon Project Ideas

Here are some winning project ideas to inspire you:

### 🎯 Beginner-Friendly

1. **Savings Goal Tracker**
   - Set goals ("Save $5000 for vacation")
   - Track progress automatically
   - Suggest optimal deposit amounts
   - Celebrate milestones

2. **Spending Category Analyzer**
   - Automatically categorize transactions
   - Show spending breakdown by category
   - Compare month-over-month
   - Highlight unusual spending

3. **Bill Payment Reminder**
   - Detect recurring payments
   - Alert before bills are due
   - Ensure sufficient balance
   - Track payment history

### 🚀 Intermediate

4. **Smart Savings Advisor**
   - Analyze "spare cash" available
   - Recommend savings deposits
   - Calculate interest projections
   - Optimize for highest APY

5. **Cash Flow Forecaster**
   - Predict future balance based on patterns
   - Identify potential low-balance periods
   - Suggest when to save vs. spend
   - Warn before account goes negative

6. **Financial Health Score**
   - Calculate overall financial wellness
   - Track improvements over time
   - Compare to benchmarks
   - Provide actionable recommendations

### 🏆 Advanced

7. **AI Budget Coach**
   - Learn spending patterns with ML
   - Provide personalized recommendations
   - Automatically adjust budgets
   - Gamify financial goals

8. **Emergency Fund Builder**
   - Calculate needed emergency fund size
   - Create automated savings plan
   - Track progress with milestones
   - Adjust for income changes

9. **Tax Obligation Tracker**
   - Estimate tax liability on earnings
   - Suggest amounts to set aside
   - Generate tax reports
   - Track deductible expenses

10. **Peer Motivation System**
    - Compare savings rate to anonymized peers
    - Show percentile rankings
    - Friendly competition features
    - Social accountability

---

## 🎤 Example Queries to Try

### Balance & Account Info
- "What's my balance?"
- "How much do I have in savings?"
- "What's the current APY?"
- "Show me my profile"

### Transactions & History
- "Show my recent transactions"
- "What did I spend yesterday?"
- "Show me all payments to @alice"
- "What's my biggest transaction this month?"

### Money Movement
- "Send $50 to @alice"
- "Put $100 in savings"
- "Withdraw $25 from savings"
- "Pay @bob $30 for dinner"

### Analysis & Insights
- "Analyze my spending over the last 30 days"
- "How much do I spend per day on average?"
- "What's my spending velocity?"
- "Am I saving enough?"

### Custom Tool Examples
- "Track my dining budget of $500"
- "Set a savings goal for $10,000"
- "Predict my balance next week"
- "Calculate my financial health score"

---

## 🎨 Customizing the AI Personality

The `hackathonSystemPrompt` in `main.go` defines your AI agent's behavior. You can customize:

### Tone & Style
```go
const hackathonSystemPrompt = `You are Nim, a [YOUR PERSONALITY HERE].

// Examples:
- "a sassy financial advisor who uses Gen Z slang"
- "a professional wealth manager with 20 years experience"
- "a supportive friend helping you build better money habits"
- "a strict budget coach who holds you accountable"
```

### Expertise Focus
```go
// Focus on specific financial areas:
- "You specialize in helping young professionals save for their first home"
- "You're an expert in optimizing interest income from savings"
- "You help people eliminate debt and build emergency funds"
```

### Interaction Style
```go
CONVERSATIONAL STYLE:
- Use lots of emojis and be super friendly
- Be brief and to-the-point, like a text message
- Provide detailed explanations like a teacher
- Use analogies and metaphors to explain concepts
```

---

## 🐛 Troubleshooting

### Backend won't start

**Error:** `ANTHROPIC_API_KEY environment variable is required`
- **Fix:** Make sure you created `.env` and added your Anthropic API key

**Error:** `address already in use :8080`
- **Fix:** Another process is using port 8080. Change the PORT in `.env` or kill the other process

### Frontend won't connect

**Error:** `WebSocket connection failed`
- **Fix:** Make sure the backend is running on port 8080
- **Fix:** Check that the backend shows "WebSocket endpoint: ws://localhost:8080/ws"

**Error:** `Module not found: '@becomeliminal/nim-chat'`
- **Fix:** Run `npm install` in the frontend directory
- **Fix:** Make sure `nim-chat` is built: `cd ../../../nim-chat && npm install && npm run build`

### Authentication not working

**Can't login to Liminal:**
1. Make sure you entered a valid email address
2. Check your email (including spam folder) for the OTP code
3. The code expires after a few minutes - request a new one if needed
4. Contact a hackathon organizer if you're not receiving codes

**Anthropic API Key Invalid:**
1. Check the key starts with `sk-ant-`
2. Verify it's copied correctly with no extra spaces
3. Check your Anthropic console for API key status
4. Make sure your account has credits

---

## 🏆 Hackathon Tips

### What Makes a Great Project

1. **Solve a Real Problem** — Focus on actual financial pain points
2. **Use Real Data** — Leverage the Liminal tools to analyze actual transactions
3. **Be Conversational** — Make the AI feel natural and delightful
4. **Add Unique Insights** — Don't just show data, provide analysis and recommendations
5. **Polish the Experience** — Small UI/UX touches make a big difference

### Time Management (6-Hour Sprint)

- **Hour 1:** Get setup working and understand the tools
- **Hour 2-3:** Build your core custom tool
- **Hour 4:** Test and refine the conversational experience
- **Hour 5:** Polish UI and add delightful touches
- **Hour 6:** Prepare demo and documentation

### Demo Tips

- **Start with the problem:** "Have you ever struggled to track your budget?"
- **Show real interaction:** Live demo the conversation, don't just show code
- **Highlight the AI:** Show how it understands natural language and provides insights
- **Show confirmation flow:** Demo money movement with the countdown UI
- **End with impact:** "This helps people build better financial habits"

### Common Pitfalls to Avoid

- ❌ Building too many features — focus on one thing done really well
- ❌ Ignoring the system prompt — this is key to great UX
- ❌ Not testing with real data — use your actual Liminal account
- ❌ Over-engineering — simple solutions often win
- ❌ Forgetting error handling — graceful failures impress judges

---

## 📚 Additional Resources

### Documentation

- [nim-go-sdk GitHub](https://github.com/becomeliminal/nim-go-sdk)
- [nim-chat Widget](https://github.com/becomeliminal/nim-chat)
- [Anthropic Claude API](https://docs.anthropic.com/)
- [Liminal Platform](https://liminal.cash)

### Getting Help

- **Discord:** Join #hackathon-help channel
- **Office Hours:** Check schedule in main hackathon channel
- **Emergency:** Contact hackathon organizers directly

### Example Projects

Check out these examples in the repo:
- `examples/basic` — Minimal nim-go-sdk server
- `examples/full-agent` — Complete agent with all Liminal tools
- `examples/custom-tools` — More custom tool examples

---

## 🎉 Ready to Build!

You have everything you need:
- ✅ Real banking APIs with live data
- ✅ Claude AI for natural language understanding
- ✅ Beautiful chat interface
- ✅ Example custom tool to learn from
- ✅ Clear documentation

Now go build something amazing! 🚀

**Questions?** Ask in Discord or grab a hackathon organizer.

**Good luck!** 💜

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE) for details.

Built with 💜 by Liminal for the Vibe Banking Hackathon.
