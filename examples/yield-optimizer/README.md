# Yield Optimizer Example

Cross-protocol DeFi yield optimizer that scans APYs across **Aave V3**, **Morpho**, and **Pendle** on Arbitrum — and executes deposits/withdrawals through Liminal custodial wallets.

## Features

- **scan_yields** — Compare real-time APYs across Aave V3, Morpho, and Pendle fixed-rate markets
- **get_defi_positions** — Show consolidated positions across all protocols + idle funds
- **suggest_allocation** — Optimal allocation recommendations (conservative / balanced / aggressive)
- **deposit_aave / withdraw_aave** — Execute Aave V3 deposits and withdrawals with user confirmation

## Architecture

```
yield-optimizer/
├── main.go              # Server setup, tool registration, config
├── .env.example         # Required environment variables
├── agent/
│   ├── prompt.go        # System prompt for the yield optimizer persona
│   └── tools.go         # 5 custom tools (3 read, 2 write)
└── defi/
    ├── contracts.go     # Arbitrum contract addresses & constants
    ├── rpc.go           # Minimal Ethereum JSON-RPC client
    ├── abi.go           # ABI encoding (no go-ethereum dependency)
    ├── aave.go          # Aave V3 on-chain reads (balance, allowance)
    ├── defillama.go     # DefiLlama API for reliable APY + TVL data
    ├── pendle.go        # Pendle API for fixed-rate stablecoin markets
    └── types.go         # Shared types
```

## Quick Start

```bash
cp .env.example .env
# Fill in ANTHROPIC_API_KEY and WALLET_ADDRESS

./run.sh
# Or: go run .

# In another terminal, start the frontend:
cd ../frontend && npm install && npm run dev
```

## Data Sources

| Protocol | APY Source | Position Source |
|----------|-----------|-----------------|
| Aave V3 | DefiLlama API | On-chain RPC (aUSDC.balanceOf) |
| Morpho | Liminal API (get_vault_rates) | Liminal API (get_savings_balance) |
| Pendle | Pendle API v2 | View only (no deposits yet) |
