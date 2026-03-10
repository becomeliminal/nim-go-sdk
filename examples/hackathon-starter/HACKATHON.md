# Scheduled Payments — Encode AI Series x Liminal Hackathon

## Event

This feature was built during **Encode AI Series: Session 1 — Vibe Banking with Liminal**, a BYOB (Build Your Own Bank) workshop hosted by [Encode Club](https://www.encode.club/) on **February 10, 2026** at Encode Hub, London.

The workshop focused on building and shipping real agentic banking tools using AI Agents in payments — from idea to working prototype.

## What Was Built

**Scheduled Payments** — a complete deferred payment system that lets users schedule future money transfers through natural conversation in the Nim chat interface.

### New Tools (Fluent API)

| Tool | Type | Description |
|------|------|-------------|
| `schedule_payment` | Write (confirmation required) | Schedule a future payment with recipient/balance validation |
| `list_scheduled_payments` | Read | List all pending scheduled payments |
| `cancel_scheduled_payment` | Read | Cancel a pending scheduled payment by ID |
| `check_balance` | Read | Wrapper around `get_balance` that shows available balance after scheduled payments |
| `send_money` | Write (confirmation required) | Wrapper around native `send_money` that guards against overspending reserved funds |

### Architecture

```
User: "Send 1 LIL to fifi next Monday at 9am"
  │
  ▼
Claude AI interprets intent, converts date to ISO 8601
  │
  ▼
schedule_payment tool (RequiresConfirmation)
  ├── Validates recipient via search_users
  ├── Checks available balance (balance - pending scheduled)
  └── Shows confirm card in chat UI
  │
  ▼
User clicks Confirm
  │
  ▼
Handler saves to SQLite (scheduled_payments.db)
  │
  ▼
Background Scheduler (every 30s)
  ├── Polls SQLite for due payments
  ├── Marks as "executing" (prevents double-execution)
  ├── Calls send_money via LiminalExecutor
  └── Updates status to "executed" or "failed"
```

### Key Implementation Details

- **SQLite persistence** via `modernc.org/sqlite` (pure Go, no CGO) — payments survive server restarts
- **Balance guard on `send_money`** — the native Liminal `send_money` tool is overwritten with a wrapper that subtracts pending scheduled payments from available balance before allowing transfers
- **`check_balance` wrapper** — enriches the standard balance response with scheduled payment totals and available amounts
- **Recipient validation** — `search_users` is called before saving to verify the recipient exists
- **Date awareness** — current UTC date/time is injected into the system prompt so Claude correctly interprets "tomorrow", "next Monday", etc.
- **LIL token support** — alongside USDC and EURC

### Bug Fixes During Development

1. **Balance parsing** — Liminal API returns `"amount"` field, not `"balance"`. Fixed `parseBalanceFromResponse` to check both.
2. **Recipient search with `@` prefix** — `search_users` API doesn't accept `@` prefix. Fixed by stripping it before searching + validating the response array isn't empty.
3. **Date in the past** — Claude didn't know the current date. Fixed by injecting `CURRENT DATE AND TIME` into the system prompt at startup.

### Files Changed

| File | Changes |
|------|---------|
| `examples/hackathon-starter/main.go` | PaymentStore (SQLite), 5 Fluent API tools, background scheduler, updated system prompt |
| `examples/hackathon-starter/go.mod` | Added `modernc.org/sqlite` dependency |
| `executor/http.go` | Fixed URL encoding (use `url.Values` instead of manual string concat) |

### How to Test

1. Start the backend: `cd examples/hackathon-starter && go run .`
2. Start the frontend: `cd examples/frontend && npm run dev`
3. Log in via email/OTP in the chat widget
4. Try these commands:
   - `"Schedule 1 LIL to fifi next Monday at 9am"`
   - `"What's my balance?"`  (shows available after scheduled)
   - `"Show my scheduled payments"`
   - `"Cancel scheduled payment <id>"`
   - `"Send 5 LIL to fifi"` (checks available balance accounting for scheduled)

### Tech Stack

- **Go 1.24** + nim-go-sdk
- **Claude claude-sonnet-4-20250514** (Anthropic)
- **SQLite** via modernc.org/sqlite
- **Liminal Banking API**
- **React + @liminalcash/nim-chat**
