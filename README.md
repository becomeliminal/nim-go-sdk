# Nim Go SDK

**Build AI agents that think about money.**

The Nim Go SDK is a production-grade framework for building AI-powered financial applications using Claude as the intelligent decision-making layer. It provides a complete infrastructure for creating AI agents that can understand financial intent, execute banking operations, and safely manage money on behalf of users.

## Overview

This SDK implements Claude as a **financial brain** - an intelligent agent that autonomously orchestrates financial operations while keeping humans in control of critical decisions. Instead of building rigid rule-based systems, developers can create conversational financial assistants that understand natural language, reason about user intent, and execute complex multi-step financial workflows.

**Key Capabilities:**
- Natural language financial operations ("Send $50 to mom" → search user → check balance → transfer funds)
- Multi-turn reasoning with Claude as the orchestration engine
- Built-in safety: automatic confirmations for all write operations
- WebSocket streaming for real-time conversational experiences
- Extensible tool system for custom business logic
- Native integration with Liminal's banking APIs

## Why This Exists

Traditional financial software requires users to navigate complex UIs and understand the underlying system architecture. The nim-go-sdk inverts this model: **Claude understands what you want to do, and the SDK handles how to do it safely.**

This enables a new class of financial applications:
- **Consumer**: Conversational banking ("Split $200 between my savings and checking")
- **Business**: Autonomous treasury management ("Optimize yield across our vaults")
- **Agent-to-Agent**: AI systems coordinating financial operations at scale

---

## The Financial Brain Architecture

                    ┌─────────────────────────────────────────────┐
                    │          WebSocket Server                   │
                    │         (server/ package)                   │
                    │                                             │
                    │  • Handles client connections               │
                    │  • JWT authentication                       │
                    │  • Protocol message routing                 │
                    └──────────────┬──────────────────────────────┘
                                   │
                                   │ User messages
                                   │
                                   ▼
        ┌──────────────────────────────────────────────────────────────┐
        │                                                              │
        │              ENGINE - The Financial Brain                    │
        │                  (engine/ package)                           │
        │                                                              │
        │  ┌────────────────────────────────────────────────┐         │
        │  │  Agentic Loop:                                 │         │
        │  │  1. Add user message to conversation           │         │
        │  │  2. Call Claude API with tools + history       │         │
        │  │  3. Claude decides: text or tool calls         │         │
        │  │  4. Execute tools, add results to history      │         │
        │  │  5. Loop until Claude returns final response   │         │
        │  └────────────────────────────────────────────────┘         │
        │                                                              │
        └───┬──────────────┬──────────────────┬────────────────┬──────┘
            │              │                  │                │
            │              │                  │                │
    ┌───────▼──────┐  ┌────▼─────┐  ┌────────▼──────┐  ┌──────▼────────┐
    │              │  │          │  │               │  │               │
    │   Session    │  │ Registry │  │  Claude API   │  │ ExecutionCtx  │
    │              │  │          │  │               │  │               │
    │ • History    │  │ • Lookup │  │ • Streaming   │  │ • Limits      │
    │ • State      │  │ • Tools  │  │ • Reasoning   │  │ • User info   │
    │ • Tokens     │  │          │  │ • Tool use    │  │ • Audit log   │
    │              │  │          │  │               │  │               │
    └──────────────┘  └────┬─────┘  └───────────────┘  └───────────────┘
                           │
                           │ Tools registered
                           │
          ┌────────────────┴─────────────────┐
          │                                  │
          ▼                                  ▼
    ┌──────────────────┐            ┌─────────────────────┐
    │                  │            │                     │
    │  Custom Tools    │            │   Liminal Tools     │
    │  (tools.Builder) │            │   (tools.Liminal)   │
    │                  │            │                     │
    │  • Your logic    │            │  • Via Executor     │
    │  • Direct exec   │            │  • Banking APIs     │
    │  • Confirmations │            │  • Confirmations    │
    │                  │            │                     │
    └──────────────────┘            └──────┬──────────────┘
                                           │
                                           │ HTTP calls
                                           │
                                    ┌──────▼───────────────┐
                                    │                      │
                                    │   HTTPExecutor       │
                                    │  (executor/ pkg)     │
                                    │                      │
                                    │  • Liminal API       │
                                    │  • JWT auth          │
                                    │  • Confirmations     │
                                    │                      │
                                    └──────┬───────────────┘
                                           │
                                           ▼
                                    ┌──────────────┐
                                    │              │
                                    │ Liminal API  │
                                    │ (Banking)    │
                                    │              │
                                    └──────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Key Insight: The Engine + Claude API = "The Brain"            │
│                                                                 │
│  Instead of hardcoded business logic, Claude reasons about     │
│  what to do based on conversation context. The Engine manages  │
│  the loop, tools, and safety guardrails.                       │
└─────────────────────────────────────────────────────────────────┘

### How the Brain Works

The **Engine** (`engine/` package) implements Claude as an autonomous financial brain:

**Intelligence Layer (Claude API)**
- Understands natural language: "Send $50 to mom" → sequence of operations
- Reasons about tool selection: search user → check balance → transfer
- Handles edge cases: insufficient funds, invalid recipients, confirmation needed

**Orchestration Layer (Engine)**
- Manages the agentic loop: conversation → Claude → tools → results → repeat
- Enforces safety: execution limits, timeouts, confirmation requirements
- Maintains state: conversation history, pending actions, token usage

**Execution Layer (Tools & Executor)**
- Custom tools: Your business logic executed directly
- Liminal tools: Banking operations via HTTPExecutor → Liminal API
- Confirmations: Write operations pause for user approval before execution

This separation means developers write simple tool definitions, and Claude handles all the cognitive work—understanding intent, orchestrating multi-step flows, and keeping humans in the loop for critical decisions.

---

## Core Features

### Intelligent Orchestration
Claude serves as the cognitive engine, reasoning about user intent and coordinating multi-step financial operations. The SDK manages the agentic loop, conversation state, and execution limits.

### Safety-First Architecture
- **Automatic confirmations** for all write operations (transfers, withdrawals, deposits)
- **Human-in-the-loop** for consequential financial decisions
- **Audit logging** of all tool executions with input/output tracking
- **Execution guardrails**: configurable turn limits, timeouts, and rate limiting

### Production-Ready WebSocket Server
Handle real-time conversations with built-in support for:
- Streaming text responses from Claude
- Message chunking and conversation resumption
- JWT-based authentication
- Protocol-level error handling

### Extensible Tool System
- **Fluent builder API** for defining custom tools with JSON Schema
- **Pre-built Liminal tools**: 9 banking operations (balance, transactions, transfers, savings)
- **Two-tier execution model**: Direct execution for custom logic, HTTP executor for Liminal integration
- **Template-based summaries** for readable confirmation prompts

### Developer Experience
- Type-safe Go APIs with comprehensive error handling
- Minimal boilerplate: working server in <20 lines of code
- Complete examples from basic to full-featured banking agents
- Detailed architecture documentation

## Quick Start

### Installation

```bash
go get github.com/becomeliminal/nim-go-sdk
```

### Basic Server

Create a conversational AI server in minutes:

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/becomeliminal/nim-go-sdk/server"
    "github.com/becomeliminal/nim-go-sdk/tools"
)

func main() {
    // Initialize server with your Anthropic API key
    srv, err := server.New(server.Config{
        AnthropicKey: "sk-ant-...",
        SystemPrompt: "You are a helpful financial assistant.",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Add a custom tool that Claude can use
    srv.AddTool(tools.New("get_account_summary").
        Description("Get a summary of the user's financial accounts").
        Schema(tools.ObjectSchema(map[string]interface{}{
            "include_pending": tools.BooleanProperty("Include pending transactions"),
        }, "include_pending")).
        HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
            // Your business logic here
            return map[string]interface{}{
                "total_balance": "5,230.50",
                "accounts": []map[string]string{
                    {"name": "Checking", "balance": "1,230.50"},
                    {"name": "Savings", "balance": "4,000.00"},
                },
            }, nil
        }).
        Build())

    log.Println("Server starting on :8080")
    srv.Run(":8080")
}
```

**Usage:**
1. Connect a WebSocket client to `ws://localhost:8080/ws`
2. Send: `{"type": "new_conversation"}`
3. Send: `{"type": "message", "content": "What's my account summary?"}`
4. Receive streaming responses as Claude thinks and executes tools

## How It Works

### The Agentic Loop

The SDK implements a turn-based execution model where Claude acts as the orchestrator:

```
1. User sends message → "Send $50 to mom"
2. Engine adds to conversation history
3. Claude API called with available tools and conversation context
4. Claude reasons and decides:
   - Return text response only
   - Call one or more tools
   - Request confirmation for write operations
5. If tool calls made:
   - Engine executes each tool
   - Results added to conversation history
   - Loop back to step 3 for Claude to continue reasoning
6. Process continues until Claude provides final response
```

**Example multi-turn flow:**
```
User: "Send $50 to mom"
  ↓
Claude calls search_users("mom") → Results: @mom_account
  ↓
Claude calls get_balance() → Results: $200 available
  ↓
Claude calls send_money({recipient: "@mom_account", amount: "50", currency: "USD"})
  ↓
SDK detects write operation → Return confirmation request to user
  ↓
User confirms → Execute transaction via Liminal API
  ↓
Claude: "Done! Sent $50 to mom. Your new balance is $150."
```

### Architecture

The SDK is organized into focused packages with clear responsibilities:

```
nim-go-sdk/
├── core/          # Shared types and interfaces
│   ├── types.go       # Message, Context, ExecutionLimits, PendingAction
│   ├── tool.go        # Tool interface definition
│   └── executor.go    # ToolExecutor interface for Liminal integration
│
├── engine/        # Claude orchestration layer
│   ├── engine.go      # Main agentic loop with Claude API
│   ├── registry.go    # Tool registration and lookup
│   └── session.go     # Conversation history management
│
├── server/        # WebSocket server implementation
│   ├── server.go      # HTTP/WebSocket handling, authentication
│   ├── protocol.go    # Client/server message types
│   └── streaming.go   # Real-time response streaming
│
├── executor/      # Tool execution implementations
│   └── http.go        # HTTPExecutor for Liminal banking APIs
│
├── tools/         # Tool building utilities
│   ├── builder.go     # Fluent API for custom tools
│   ├── liminal.go     # Pre-built Liminal tool definitions
│   └── schema.go      # JSON Schema helpers
│
└── store/         # State management
    ├── conversation.go # Conversation persistence
    └── confirmation.go # Pending action tracking
```

## Package Reference

### `core/` - Foundation Types

The core package defines the fundamental types and interfaces used throughout the SDK:

- **`Tool`** - Interface that all tools must implement (name, description, schema, execution logic)
- **`ToolExecutor`** - Interface for external tool execution systems (used for Liminal integration)
- **`Message`, `ContentBlock`** - Conversation message types compatible with Claude API
- **`Context`** - Execution context with user info, preferences, and audit metadata
- **`ExecutionLimits`** - Configurable guardrails (max turns, timeout, max tool calls)
- **`PendingAction`** - Represents a write operation awaiting user confirmation

### `engine/` - Orchestration Layer

The engine package implements the core agentic loop with Claude:

- **`Engine`** - Main orchestrator that manages the conversation loop with Claude, handles tool execution, and enforces execution limits
- **`ToolRegistry`** - Central registry for all available tools (both custom and Liminal), provides lookup by name
- **`Session`** - Manages conversation history, token usage tracking, and state persistence across messages
- **`StreamHandler`** - Callbacks for handling streaming events (text chunks, tool calls, completions)

### `server/` - WebSocket Server

Production-ready WebSocket server with authentication and streaming support:

- **`Server`** - HTTP server with WebSocket upgrade handling, supports concurrent conversations
- **`Config`** - Configuration for API keys, system prompts, execution limits, and CORS
- **Protocol types** - Well-defined message schemas for client-server communication
- **Authentication** - JWT token validation and user identification
- **Error handling** - Graceful error recovery and client-friendly error messages

### `executor/` - External Integration

Implementations of the ToolExecutor interface for connecting to external systems:

- **`HTTPExecutor`** - Production executor for Liminal banking APIs
  - Handles authentication, request signing, and API communication
  - Manages confirmation lifecycle (create, confirm, cancel)
  - Supports both read (immediate) and write (confirmation-based) operations

### `tools/` - Tool Development

Utilities for building and registering tools:

- **`Builder`** - Fluent API for constructing tools with chainable methods
- **`LiminalTools()`** - Factory function returning all 9 pre-built Liminal banking tools
- **Schema helpers** - Type-safe functions for building JSON Schema (StringProperty, NumberProperty, ObjectSchema, etc.)
- **Template engine** - Renders human-readable summaries for confirmation prompts using Go templates

## WebSocket Protocol

The server uses a JSON-based protocol over WebSockets for real-time bidirectional communication.

### Client → Server Messages

**Start new conversation:**
```json
{"type": "new_conversation"}
```

**Resume existing conversation:**
```json
{
  "type": "resume_conversation",
  "conversationId": "conv_abc123"
}
```

**Send user message:**
```json
{
  "type": "message",
  "content": "What's my balance?"
}
```

**Confirm pending write operation:**
```json
{
  "type": "confirm",
  "actionId": "action_xyz789"
}
```

**Cancel pending write operation:**
```json
{
  "type": "cancel",
  "actionId": "action_xyz789"
}
```

### Server → Client Messages

**Conversation initialized:**
```json
{
  "type": "conversation_started",
  "conversationId": "conv_abc123"
}
```

**Streaming text chunk (real-time):**
```json
{
  "type": "text_chunk",
  "content": "Let me check your balance..."
}
```

**Complete text message:**
```json
{
  "type": "text",
  "content": "Your current balance is $1,230.50"
}
```

**Confirmation request for write operation:**
```json
{
  "type": "confirm_request",
  "actionId": "action_xyz789",
  "tool": "send_money",
  "summary": "Send 50 USD to @alice",
  "input": {
    "recipient": "@alice",
    "amount": "50",
    "currency": "USD"
  },
  "expiresAt": "2024-01-15T10:30:00Z"
}
```

**Turn complete:**
```json
{
  "type": "complete",
  "tokenUsage": {
    "inputTokens": 1250,
    "outputTokens": 420
  }
}
```

**Error occurred:**
```json
{
  "type": "error",
  "content": "Rate limit exceeded. Please try again in 60 seconds."
}
```

## Building Custom Tools

### Basic Tool with Fluent Builder

The SDK provides a fluent builder API for creating custom tools that Claude can use:

```go
import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/becomeliminal/nim-go-sdk/tools"
)

tool := tools.New("analyze_spending").
    Description("Analyze user spending patterns and provide insights").
    Schema(tools.ObjectSchema(map[string]interface{}{
        "time_period": tools.StringProperty("Time period to analyze (e.g., 'last_month', 'last_quarter')"),
        "category":    tools.StringProperty("Optional category to filter by"),
    }, "time_period")).  // "time_period" is required
    HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
        // Parse input parameters
        var params struct {
            TimePeriod string `json:"time_period"`
            Category   string `json:"category,omitempty"`
        }
        if err := json.Unmarshal(input, &params); err != nil {
            return nil, fmt.Errorf("invalid input: %w", err)
        }

        // Your business logic here
        analysis := performSpendingAnalysis(params.TimePeriod, params.Category)

        // Return structured data
        return map[string]interface{}{
            "total_spent":      analysis.Total,
            "top_categories":   analysis.TopCategories,
            "average_per_day":  analysis.AvgPerDay,
            "insights":         analysis.Insights,
        }, nil
    }).
    Build()

// Register with server
srv.AddTool(tool)
```

### Write Operations with Confirmation

Tools that perform write operations (transfers, deletions, updates) should require user confirmation:

```go
tool := tools.New("cancel_subscription").
    Description("Cancel a recurring subscription for the user").
    RequiresConfirmation().  // Marks this as a write operation
    SummaryTemplate("Cancel subscription to {{.service_name}} ({{.amount}}/{{.frequency}})").
    Schema(tools.ObjectSchema(map[string]interface{}{
        "subscription_id": tools.StringProperty("ID of the subscription to cancel"),
        "service_name":    tools.StringProperty("Name of the service"),
        "amount":          tools.StringProperty("Subscription amount"),
        "frequency":       tools.StringProperty("Billing frequency"),
    }, "subscription_id", "service_name", "amount", "frequency")).
    HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
        var params struct {
            SubscriptionID string `json:"subscription_id"`
            ServiceName    string `json:"service_name"`
            Amount         string `json:"amount"`
            Frequency      string `json:"frequency"`
        }
        json.Unmarshal(input, &params)

        // This will only execute AFTER user confirms
        err := cancelSubscription(params.SubscriptionID)
        if err != nil {
            return nil, err
        }

        return map[string]interface{}{
            "success": true,
            "message": fmt.Sprintf("Successfully cancelled %s subscription", params.ServiceName),
        }, nil
    }).
    Build()
```

**How confirmation works:**
1. Claude calls `cancel_subscription`
2. SDK generates a `PendingAction` with summary: "Cancel subscription to Netflix ($15.99/month)"
3. Server sends `confirm_request` message to client
4. Client shows confirmation UI to user
5. User approves → SDK executes the tool's handler function
6. Result returned to Claude to continue conversation

### Advanced: Schema with Nested Objects

```go
tool := tools.New("create_budget").
    Description("Create a monthly budget with category limits").
    Schema(tools.ObjectSchema(map[string]interface{}{
        "month": tools.StringProperty("Month in YYYY-MM format"),
        "categories": map[string]interface{}{
            "type": "array",
            "description": "Budget categories with limits",
            "items": tools.ObjectSchema(map[string]interface{}{
                "name":  tools.StringProperty("Category name"),
                "limit": tools.NumberProperty("Monthly spending limit"),
            }, "name", "limit"),
        },
        "rollover": tools.BooleanProperty("Allow unused budget to roll over to next month"),
    }, "month", "categories")).
    HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
        // Implementation
        return map[string]interface{}{"budget_id": "budget_123"}, nil
    }).
    Build()
```

## Using Liminal Banking Tools

The SDK includes pre-built integrations with Liminal's banking APIs, providing 9 production-ready financial operations.

### Setup

```go
import (
    "log"

    "github.com/becomeliminal/nim-go-sdk/executor"
    "github.com/becomeliminal/nim-go-sdk/server"
    "github.com/becomeliminal/nim-go-sdk/tools"
)

func main() {
    // Create Liminal API executor
    exec := executor.NewHTTPExecutor(executor.HTTPExecutorConfig{
        BaseURL: "https://api.liminal.cash",
        // Authentication is handled automatically via JWT tokens from client
    })

    // Initialize server
    srv, err := server.New(server.Config{
        AnthropicKey: "sk-ant-...",
        SystemPrompt: "You are a helpful banking assistant powered by Liminal.",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Add all Liminal tools at once
    srv.AddTools(tools.LiminalTools(exec)...)

    srv.Run(":8080")
}
```

### Available Tools

#### Read Operations (No Confirmation)

**`get_balance`** - Retrieve current wallet balance
- Returns: USD (USDC), EUR (EURC), and LIL token balances
- Networks: Arbitrum and Base

**`get_savings_balance`** - Get savings/vault positions
- Returns: All active savings deposits with amounts and APY

**`get_vault_rates`** - Query current savings yield rates
- Returns: Available vaults with current APY rates

**`get_transactions`** - Fetch transaction history
- Parameters: Optional filters for time range, type, limit
- Returns: Paginated list of transactions with amounts, recipients, timestamps

**`get_profile`** - Get user's profile information
- Returns: Username, display name, verification status

**`search_users`** - Find other users by username or display name
- Parameters: Search query string
- Returns: List of matching users with usernames and profiles

#### Write Operations (Confirmation Required)

**`send_money`** - Send payment to another user
- Parameters: `recipient` (username or user ID), `amount`, `currency`
- Confirmation: "Send {amount} {currency} to {recipient}"
- Networks: Both Arbitrum and Base supported

**`deposit_savings`** - Deposit funds into savings vault
- Parameters: `amount`, `currency`, `vault_id`
- Confirmation: "Deposit {amount} {currency} to savings (earning {apy}% APY)"

**`withdraw_savings`** - Withdraw from savings vault
- Parameters: `amount`, `currency`, `vault_id`
- Confirmation: "Withdraw {amount} {currency} from savings"

### Multi-Currency Support

All monetary operations support:
- **USD** (USDC stablecoin)
- **EUR** (EURC stablecoin)
- **LIL** (Liminal platform token)

### Example Conversation Flow

```
User: "How much money do I have?"
  → Claude calls get_balance()
  → Response: "You have $1,230.50 in your wallet and $4,000 in savings."

User: "Send $50 to @alice"
  → Claude calls search_users("alice") to confirm recipient
  → Claude calls get_balance() to verify sufficient funds
  → Claude calls send_money({recipient: "@alice", amount: "50", currency: "USD"})
  → SDK returns confirmation request to user
  → User approves
  → Transaction executes
  → Response: "Sent $50 to @alice. Your new balance is $1,180.50."
```

## Examples

The `examples/` directory contains complete reference implementations demonstrating different use cases:

### `basic/`
Minimal server with a single custom tool.

**Best for:** Learning the fundamentals, simple proof-of-concepts

### `custom-tools/`
Task management system with multiple custom tools (create, list, update, delete tasks).

**Best for:** Understanding how to build tool-based applications

### `full-agent/`
Complete integration showing:
- Liminal banking tools
- Custom business logic
- Confirmation flows
- Error handling

**Best for:** Production implementation reference

## Configuration

### Environment Variables

**Required:**
- `ANTHROPIC_API_KEY` - Your Anthropic API key (get one at https://platform.claude.com)

**Optional:**
- `LIMINAL_BASE_URL` - Liminal API endpoint (default: `https://api.liminal.cash`)
- `PORT` - Server port (default: `8080`)

### Authentication

**Liminal API Authentication:**
Authentication to Liminal's banking APIs is handled automatically via JWT tokens passed from the client. The SDK extracts the token from WebSocket messages and includes it in all Liminal API requests. No separate API key configuration needed.

**Client Authentication:**
The server optionally supports JWT-based client authentication. Configure via `server.Config.JWTSecret` to enable token validation.

## Production Considerations

### Rate Limiting
Configure execution limits to prevent runaway costs:

```go
srv, _ := server.New(server.Config{
    AnthropicKey: "sk-ant-...",
    ExecutionLimits: core.ExecutionLimits{
        MaxTurns:     15,  // Limit conversation turns
        MaxToolCalls: 30,  // Limit total tool executions
        Timeout:      time.Minute * 2,
    },
})
```

### Error Handling
The SDK includes comprehensive error handling:
- API failures are logged and returned to clients with user-friendly messages
- Confirmation timeouts automatically cancel pending actions
- Network errors trigger automatic retries (configurable)

### Monitoring
Implement custom logging by wrapping tools:

```go
originalTool := tools.New("my_tool").
    HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
        start := time.Now()
        result, err := actualImplementation(ctx, input)

        log.Printf("Tool executed: duration=%v error=%v", time.Since(start), err)

        return result, err
    }).
    Build()
```

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

MIT License

## Support

- **Issues**: [GitHub Issues](https://github.com/becomeliminal/nim-go-sdk/issues)
- **Discussions**: [GitHub Discussions](https://github.com/becomeliminal/nim-go-sdk/discussions)
