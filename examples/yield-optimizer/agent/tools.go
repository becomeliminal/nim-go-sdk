package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/tools"
	"github.com/becomeliminal/nim-go-sdk/examples/yield-optimizer/defi"
)

// ToolDeps holds shared dependencies for all custom tools.
type ToolDeps struct {
	Aave          *defi.AaveClient
	DefiLlama     *defi.DefiLlamaClient
	Pendle        *defi.PendleClient
	Executor      core.ToolExecutor
	WalletAddress string
}

// CreateTools returns all custom yield optimizer tools.
func CreateTools(deps *ToolDeps) []core.Tool {
	return []core.Tool{
		createScanYieldsTool(deps),
		createGetDefiPositionsTool(deps),
		createSuggestAllocationTool(deps),
		createDepositAaveTool(deps),
		createWithdrawAaveTool(deps),
	}
}

// ────────────────────────────────────────────────────────────────────────────
// scan_yields
// ────────────────────────────────────────────────────────────────────────────

func createScanYieldsTool(deps *ToolDeps) core.Tool {
	return tools.New("scan_yields").
		Description("Scan current USDC yield rates across Aave V3, Liminal/Morpho, and Pendle fixed-rate markets on Arbitrum.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"token": tools.StringEnumProperty("Token to scan yields for", "USDC"),
		})).
		Handler(func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
			protocols := []map[string]interface{}{}

			// 1. Aave V3 — use DefiLlama for reliable APY
			aaveAPY := 0.0
			aaveTVL := 0.0
			if deps.DefiLlama != nil {
				a, t, err := deps.DefiLlama.AaveArbitrumUSDCYield(ctx)
				if err == nil {
					aaveAPY = math.Round(a*100) / 100
					aaveTVL = t
				}
			}
			aaveEntry := map[string]interface{}{
				"name":      "Aave V3",
				"chain":     "Arbitrum",
				"apy":       fmt.Sprintf("%.2f", aaveAPY),
				"type":      "variable",
				"risk":      "low",
				"tvl":       formatTVL(aaveTVL),
				"actionable": true,
			}
			protocols = append(protocols, aaveEntry)

			// 2. Liminal/Morpho — via Liminal API
			vaultReq, _ := json.Marshal(map[string]interface{}{})
			vaultResp, err := deps.Executor.Execute(ctx, &core.ExecuteRequest{
				UserID:    params.UserID,
				Tool:      "get_vault_rates",
				Input:     vaultReq,
				RequestID: params.RequestID,
			})
			if err == nil && vaultResp.Success {
				var vaultData struct {
					Vaults []struct {
						Currency string `json:"currency"`
						APY      string `json:"apy"`
						TVL      string `json:"tvl"`
					} `json:"vaults"`
				}
				if json.Unmarshal(vaultResp.Data, &vaultData) == nil {
					for _, v := range vaultData.Vaults {
						if v.Currency == "USDC" || v.Currency == "usdc" {
							protocols = append(protocols, map[string]interface{}{
								"name":      "Morpho",
								"chain":     "Arbitrum",
								"apy":       v.APY,
								"tvl":       v.TVL,
								"type":      "variable",
								"risk":      "low",
								"actionable": true,
							})
						}
					}
				}
			}

			// 3. Pendle — fixed-rate markets
			if deps.Pendle != nil {
				markets, err := deps.Pendle.GetStablecoinMarkets(ctx)
				if err == nil {
					for _, m := range markets {
						protocols = append(protocols, map[string]interface{}{
							"name":      fmt.Sprintf("Pendle %s", m.Name),
							"chain":     "Arbitrum",
							"apy":       fmt.Sprintf("%.2f", m.ImpliedAPY),
							"type":      "fixed",
							"risk":      "medium",
							"expiry":    m.Expiry,
							"actionable": false,
						})
					}
				}
			}

			// Best yield
			bestYield := ""
			bestAPY := 0.0
			for _, p := range protocols {
				apyStr, _ := p["apy"].(string)
				apy, _ := strconv.ParseFloat(apyStr, 64)
				if apy > bestAPY {
					bestAPY = apy
					bestYield, _ = p["name"].(string)
				}
			}

			return &core.ToolResult{Success: true, Data: map[string]interface{}{
				"token":      "USDC",
				"protocols":  protocols,
				"best_yield": bestYield,
				"best_apy":   fmt.Sprintf("%.2f", bestAPY),
				"scanned_at": time.Now().Format(time.RFC3339),
			}}, nil
		}).
		Build()
}

// ────────────────────────────────────────────────────────────────────────────
// get_defi_positions
// ────────────────────────────────────────────────────────────────────────────

func createGetDefiPositionsTool(deps *ToolDeps) core.Tool {
	return tools.New("get_defi_positions").
		Description("Get user's USDC positions across all protocols: wallet balance, Aave V3, and Morpho savings.").
		Schema(tools.ObjectSchema(map[string]interface{}{})).
		Handler(func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
			positions := []map[string]interface{}{}
			walletUSDC := "0.00"

			// 1. Wallet balance
			balReq, _ := json.Marshal(map[string]interface{}{"currency": "USDC"})
			balResp, err := deps.Executor.Execute(ctx, &core.ExecuteRequest{
				UserID: params.UserID, Tool: "get_balance",
				Input: balReq, RequestID: params.RequestID,
			})
			if err == nil && balResp.Success {
				var balData struct {
					Balances []struct {
						Currency string `json:"currency"`
						Amount   string `json:"amount"`
					} `json:"balances"`
					TotalUSD string `json:"totalUsd"`
				}
				if json.Unmarshal(balResp.Data, &balData) == nil {
					for _, b := range balData.Balances {
						if b.Currency == "USDC" || b.Currency == "usdc" {
							walletUSDC = b.Amount
						}
					}
					if walletUSDC == "0.00" && balData.TotalUSD != "" {
						walletUSDC = balData.TotalUSD
					}
				}
			}

			// 2. Aave V3 (on-chain read)
			if deps.WalletAddress != "" {
				aaveBal, _, err := deps.Aave.GetUserBalance(ctx, deps.WalletAddress)
				if err == nil && aaveBal != "0.00" {
					// Get correct APY from DefiLlama
					aaveAPY := 0.0
					if deps.DefiLlama != nil {
						a, _, _ := deps.DefiLlama.AaveArbitrumUSDCYield(ctx)
						aaveAPY = math.Round(a*100) / 100
					}
					positions = append(positions, map[string]interface{}{
						"protocol": "Aave V3",
						"token":    "USDC",
						"balance":  aaveBal,
						"apy":      fmt.Sprintf("%.2f%%", aaveAPY),
						"type":     "variable",
					})
				}
			}

			// 3. Morpho savings
			savReq, _ := json.Marshal(map[string]interface{}{})
			savResp, err := deps.Executor.Execute(ctx, &core.ExecuteRequest{
				UserID: params.UserID, Tool: "get_savings_balance",
				Input: savReq, RequestID: params.RequestID,
			})
			if err == nil && savResp.Success {
				var savData struct {
					Positions []struct {
						Currency     string `json:"currency"`
						Deposited    string `json:"deposited"`
						CurrentValue string `json:"currentValue"`
						APY          string `json:"apy"`
						Earnings     string `json:"earnings"`
					} `json:"positions"`
				}
				if json.Unmarshal(savResp.Data, &savData) == nil {
					for _, p := range savData.Positions {
						pos := map[string]interface{}{
							"protocol": "Morpho",
							"token":    p.Currency,
							"balance":  p.CurrentValue,
							"apy":      p.APY + "%",
							"type":     "variable",
						}
						if p.Earnings != "" && p.Earnings != "0" {
							pos["earnings"] = p.Earnings
						}
						positions = append(positions, pos)
					}
				}
			}

			// Totals
			totalDeposited := 0.0
			for _, p := range positions {
				if b, ok := p["balance"].(string); ok {
					if v, err := strconv.ParseFloat(b, 64); err == nil {
						totalDeposited += v
					}
				}
			}
			walletVal, _ := strconv.ParseFloat(walletUSDC, 64)

			return &core.ToolResult{Success: true, Data: map[string]interface{}{
				"wallet_usdc":     walletUSDC,
				"positions":       positions,
				"total_deposited": fmt.Sprintf("%.2f", totalDeposited),
				"total_portfolio": fmt.Sprintf("%.2f", totalDeposited+walletVal),
				"idle_funds":      walletUSDC,
			}}, nil
		}).
		Build()
}

// ────────────────────────────────────────────────────────────────────────────
// suggest_allocation
// ────────────────────────────────────────────────────────────────────────────

func createSuggestAllocationTool(deps *ToolDeps) core.Tool {
	return tools.New("suggest_allocation").
		Description("Suggest optimal USDC allocation across protocols based on current rates and risk preference.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"amount":          tools.StringProperty("Total USDC amount to allocate (e.g., '1000'). If empty, analyzes existing positions."),
			"risk_preference": tools.StringEnumProperty("Risk tolerance", "conservative", "balanced", "aggressive"),
		})).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Amount         string `json:"amount"`
				RiskPreference string `json:"risk_preference"`
			}
			json.Unmarshal(input, &params)
			if params.RiskPreference == "" {
				params.RiskPreference = "balanced"
			}

			// Get rates from DefiLlama (reliable)
			aaveAPY := 0.0
			if deps.DefiLlama != nil {
				a, _, _ := deps.DefiLlama.AaveArbitrumUSDCYield(ctx)
				aaveAPY = math.Round(a*100) / 100
			}

			morphoAPY := 0.0
			vaultReq, _ := json.Marshal(map[string]interface{}{})
			vaultResp, _ := deps.Executor.Execute(ctx, &core.ExecuteRequest{
				Tool: "get_vault_rates", Input: vaultReq,
			})
			if vaultResp != nil && vaultResp.Success {
				var vaultData struct {
					Vaults []struct {
						Currency string `json:"currency"`
						APY      string `json:"apy"`
					} `json:"vaults"`
				}
				if json.Unmarshal(vaultResp.Data, &vaultData) == nil {
					for _, v := range vaultData.Vaults {
						if v.Currency == "USDC" || v.Currency == "usdc" {
							morphoAPY, _ = strconv.ParseFloat(v.APY, 64)
						}
					}
				}
			}

			// Get Pendle best fixed rate
			pendleAPY := 0.0
			pendleName := ""
			if deps.Pendle != nil {
				markets, err := deps.Pendle.GetStablecoinMarkets(ctx)
				if err == nil {
					for _, m := range markets {
						if m.ImpliedAPY > pendleAPY {
							pendleAPY = m.ImpliedAPY
							pendleName = m.Name
						}
					}
				}
			}

			totalAmount, _ := strconv.ParseFloat(params.Amount, 64)

			return buildAllocation(aaveAPY, morphoAPY, pendleAPY, pendleName, totalAmount, params.RiskPreference), nil
		}).
		Build()
}

func buildAllocation(aaveAPY, morphoAPY, pendleAPY float64, pendleName string, total float64, risk string) map[string]interface{} {
	type slot struct {
		name string
		apy  float64
		pct  float64
		kind string // "variable" or "fixed"
	}

	var slots []slot

	switch risk {
	case "conservative":
		// Split between Aave + Morpho, skip Pendle
		if morphoAPY > aaveAPY {
			slots = append(slots, slot{"Morpho", morphoAPY, 0.60, "variable"})
			slots = append(slots, slot{"Aave V3", aaveAPY, 0.40, "variable"})
		} else {
			slots = append(slots, slot{"Aave V3", aaveAPY, 0.60, "variable"})
			slots = append(slots, slot{"Morpho", morphoAPY, 0.40, "variable"})
		}
	case "aggressive":
		// All-in on highest yield (including Pendle)
		best := aaveAPY
		bestName := "Aave V3"
		bestKind := "variable"
		if morphoAPY > best {
			best = morphoAPY
			bestName = "Morpho"
		}
		if pendleAPY > best && pendleName != "" {
			best = pendleAPY
			bestName = "Pendle " + pendleName
			bestKind = "fixed"
		}
		slots = append(slots, slot{bestName, best, 1.0, bestKind})
	default: // balanced
		// Mix variable + fixed if Pendle offers significantly more
		if pendleAPY > aaveAPY*1.5 && pendleName != "" {
			slots = append(slots, slot{"Pendle " + pendleName, pendleAPY, 0.40, "fixed"})
			if morphoAPY > aaveAPY {
				slots = append(slots, slot{"Morpho", morphoAPY, 0.35, "variable"})
				slots = append(slots, slot{"Aave V3", aaveAPY, 0.25, "variable"})
			} else {
				slots = append(slots, slot{"Aave V3", aaveAPY, 0.35, "variable"})
				slots = append(slots, slot{"Morpho", morphoAPY, 0.25, "variable"})
			}
		} else if morphoAPY > aaveAPY {
			slots = append(slots, slot{"Morpho", morphoAPY, 0.65, "variable"})
			slots = append(slots, slot{"Aave V3", aaveAPY, 0.35, "variable"})
		} else {
			slots = append(slots, slot{"Aave V3", aaveAPY, 0.65, "variable"})
			slots = append(slots, slot{"Morpho", morphoAPY, 0.35, "variable"})
		}
	}

	suggestions := []map[string]interface{}{}
	blendedAPY := 0.0
	totalProjected := 0.0

	for _, s := range slots {
		entry := map[string]interface{}{
			"protocol":   s.name,
			"apy":        fmt.Sprintf("%.2f", s.apy),
			"allocation": fmt.Sprintf("%.0f%%", s.pct*100),
			"type":       s.kind,
		}
		if total > 0 {
			amt := total * s.pct
			yearly := amt * s.apy / 100
			entry["amount"] = fmt.Sprintf("%.2f", amt)
			entry["projected_yearly"] = fmt.Sprintf("%.2f", yearly)
			totalProjected += yearly
		}
		blendedAPY += s.apy * s.pct
		suggestions = append(suggestions, entry)
	}

	result := map[string]interface{}{
		"risk":        risk,
		"suggestions": suggestions,
		"blended_apy": fmt.Sprintf("%.2f", blendedAPY),
	}
	if total > 0 {
		result["total_amount"] = fmt.Sprintf("%.2f", total)
		result["projected_yearly"] = fmt.Sprintf("%.2f", totalProjected)
	}

	return result
}

// ────────────────────────────────────────────────────────────────────────────
// deposit_aave
// ────────────────────────────────────────────────────────────────────────────

func createDepositAaveTool(deps *ToolDeps) core.Tool {
	return tools.New("deposit_aave").
		Description("Deposit USDC into Aave V3 on Arbitrum. Handles USDC approval if needed. Requires confirmation.").
		Schema(tools.BuildSchemaWithThought(map[string]interface{}{
			"amount": tools.StringProperty("USDC amount to deposit (e.g., '100.00')"),
		}, true, "amount")).
		RequiresConfirmation().
		SummaryTemplate("Deposit {{.amount}} USDC into Aave V3").
		Handler(func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
			var input struct {
				Amount  string `json:"amount"`
				Thought string `json:"thought"`
			}
			if err := json.Unmarshal(params.Input, &input); err != nil {
				return &core.ToolResult{Success: false, Error: "invalid input"}, nil
			}

			amountWei, err := defi.ParseUSDCAmount(input.Amount)
			if err != nil {
				return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid amount: %v", err)}, nil
			}

			walletAddr := deps.WalletAddress
			if walletAddr == "" {
				return &core.ToolResult{Success: false, Error: "wallet address not configured"}, nil
			}

			// Check allowance, approve if needed
			allowance, err := deps.Aave.GetAllowance(ctx, walletAddr, defi.AaveV3Pool)
			if err == nil && allowance.Cmp(amountWei) < 0 {
				approveData := defi.EncodeApprove(defi.AaveV3Pool, defi.MaxUint256)
				approveReq, _ := json.Marshal(map[string]interface{}{
					"chain_id": defi.ChainIDArbitrum,
					"to":       defi.USDC,
					"data":     defi.HexEncode(approveData),
					"value":    "0",
					"gas_tier": "standard",
					"thought":  "Approving USDC for Aave V3 Pool",
				})
				resp, err := deps.Executor.ExecuteWrite(ctx, &core.ExecuteRequest{
					UserID: params.UserID, Tool: "execute_contract_call",
					Input: approveReq, RequestID: params.RequestID,
				})
				if err != nil || !resp.Success {
					return &core.ToolResult{Success: false, Error: "USDC approval failed"}, nil
				}
			}

			// Encode supply(USDC, amount, onBehalfOf, 0)
			supplyData := defi.EncodeAaveSupply(defi.USDC, amountWei, walletAddr)
			supplyReq, _ := json.Marshal(map[string]interface{}{
				"chain_id": defi.ChainIDArbitrum,
				"to":       defi.AaveV3Pool,
				"data":     defi.HexEncode(supplyData),
				"value":    "0",
				"gas_tier": "standard",
				"thought":  input.Thought,
			})

			resp, err := deps.Executor.ExecuteWrite(ctx, &core.ExecuteRequest{
				UserID: params.UserID, Tool: "execute_contract_call",
				Input: supplyReq, RequestID: params.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: err.Error()}, nil
			}
			if resp.RequiresConfirmation {
				return &core.ToolResult{Success: true, Data: map[string]interface{}{
					"status":  "pending_confirmation",
					"summary": fmt.Sprintf("Deposit %s USDC into Aave V3", input.Amount),
				}}, nil
			}
			return &core.ToolResult{Success: true, Data: map[string]interface{}{
				"status": "submitted",
			}}, nil
		}).
		Build()
}

// ────────────────────────────────────────────────────────────────────────────
// withdraw_aave
// ────────────────────────────────────────────────────────────────────────────

func createWithdrawAaveTool(deps *ToolDeps) core.Tool {
	return tools.New("withdraw_aave").
		Description("Withdraw USDC from Aave V3 on Arbitrum. Use 'max' to withdraw everything. Requires confirmation.").
		Schema(tools.BuildSchemaWithThought(map[string]interface{}{
			"amount": tools.StringProperty("USDC amount to withdraw (e.g., '100.00' or 'max')"),
		}, true, "amount")).
		RequiresConfirmation().
		SummaryTemplate("Withdraw {{.amount}} USDC from Aave V3").
		Handler(func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
			var input struct {
				Amount  string `json:"amount"`
				Thought string `json:"thought"`
			}
			if err := json.Unmarshal(params.Input, &input); err != nil {
				return &core.ToolResult{Success: false, Error: "invalid input"}, nil
			}

			walletAddr := deps.WalletAddress
			if walletAddr == "" {
				return &core.ToolResult{Success: false, Error: "wallet address not configured"}, nil
			}

			var amountWei *big.Int
			if input.Amount == "max" || input.Amount == "all" {
				amountWei = defi.MaxUint256
			} else {
				var err error
				amountWei, err = defi.ParseUSDCAmount(input.Amount)
				if err != nil {
					return &core.ToolResult{Success: false, Error: fmt.Sprintf("invalid amount: %v", err)}, nil
				}
			}

			withdrawData := defi.EncodeAaveWithdraw(defi.USDC, amountWei, walletAddr)
			withdrawReq, _ := json.Marshal(map[string]interface{}{
				"chain_id": defi.ChainIDArbitrum,
				"to":       defi.AaveV3Pool,
				"data":     defi.HexEncode(withdrawData),
				"value":    "0",
				"gas_tier": "standard",
				"thought":  input.Thought,
			})

			resp, err := deps.Executor.ExecuteWrite(ctx, &core.ExecuteRequest{
				UserID: params.UserID, Tool: "execute_contract_call",
				Input: withdrawReq, RequestID: params.RequestID,
			})
			if err != nil {
				return &core.ToolResult{Success: false, Error: err.Error()}, nil
			}
			if resp.RequiresConfirmation {
				return &core.ToolResult{Success: true, Data: map[string]interface{}{
					"status":  "pending_confirmation",
					"summary": fmt.Sprintf("Withdraw %s USDC from Aave V3", input.Amount),
				}}, nil
			}
			return &core.ToolResult{Success: true, Data: map[string]interface{}{
				"status": "submitted",
			}}, nil
		}).
		Build()
}

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

func formatTVL(tvl float64) string {
	if tvl >= 1e9 {
		return fmt.Sprintf("$%.1fB", tvl/1e9)
	}
	if tvl >= 1e6 {
		return fmt.Sprintf("$%.0fM", tvl/1e6)
	}
	if tvl > 0 {
		return fmt.Sprintf("$%.0fK", tvl/1e3)
	}
	return ""
}
