package tools

import (
	"github.com/becomeliminal/nim-go-sdk/core"
)

// LiminalToolDefinitions returns the definitions for all Liminal tools.
// These are the standard tools available through the Liminal API.
func LiminalToolDefinitions() []core.ToolDefinition {
	return []core.ToolDefinition{
		// Read operations
		{
			ToolName:        "get_balance",
			ToolDescription: "Get the user's wallet balance.",
			InputSchema: ObjectSchema(map[string]interface{}{
				"currency": StringProperty("Optional: filter by currency (e.g., 'USD', 'EUR', 'LIL')"),
			}),
		},
		{
			ToolName:        "get_savings_balance",
			ToolDescription: "Get the user's savings positions and current APY.",
			InputSchema: ObjectSchema(map[string]interface{}{
				"vault": StringProperty("Optional: filter by vault name"),
			}),
		},
		{
			ToolName:        "get_vault_rates",
			ToolDescription: "Get current APY rates for available savings vaults.",
			InputSchema:     ObjectSchema(map[string]interface{}{}),
		},
		{
			ToolName:        "get_transactions",
			ToolDescription: "Get the user's recent transaction history.",
			InputSchema: ObjectSchema(map[string]interface{}{
				"limit": IntegerProperty("Number of transactions to return (default: 10)"),
				"type":  StringEnumProperty("Filter by transaction type", "send", "receive", "deposit", "withdraw"),
			}),
		},
		{
			ToolName:        "get_profile",
			ToolDescription: "Get the user's profile information.",
			InputSchema:     ObjectSchema(map[string]interface{}{}),
		},
		{
			ToolName:        "search_users",
			ToolDescription: "Search for users by display tag or name.",
			InputSchema: ObjectSchema(map[string]interface{}{
				"query": StringProperty("Search query (display tag like @alice or name)"),
			}, "query"),
		},

		// Write operations (require confirmation)
		{
			ToolName:                 "send_money",
			ToolDescription:          "Send money to another user. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Send {{.amount}} {{.currency}} to {{.recipient}}",
			InputSchema: ObjectSchema(map[string]interface{}{
				"recipient": StringProperty("Recipient's display tag (e.g., @alice) or user ID"),
				"amount":    StringProperty("Amount to send (e.g., '50.00')"),
				"currency":  StringProperty("Currency to send (e.g., 'USD', 'EUR', 'LIL')"),
				"note":      StringProperty("Optional payment note"),
			}, "recipient", "amount", "currency"),
		},
		{
			ToolName:                 "deposit_savings",
			ToolDescription:          "Deposit funds into savings. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Deposit {{.amount}} {{.currency}} into savings",
			InputSchema: ObjectSchema(map[string]interface{}{
				"amount":   StringProperty("Amount to deposit"),
				"currency": StringProperty("Currency to deposit (e.g., 'USD', 'EUR', 'LIL')"),
			}, "amount", "currency"),
		},
		{
			ToolName:                 "withdraw_savings",
			ToolDescription:          "Withdraw funds from savings. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Withdraw {{.amount}} {{.currency}} from savings",
			InputSchema: ObjectSchema(map[string]interface{}{
				"amount":   StringProperty("Amount to withdraw"),
				"currency": StringProperty("Currency to withdraw (e.g., 'USD', 'EUR', 'LIL')"),
			}, "amount", "currency"),
		},
		{
			ToolName:                 "execute_contract_call",
			ToolDescription:          "Execute an arbitrary smart contract call on any blockchain. Requires confirmation. You must provide pre-encoded calldata as hex.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Execute contract call on chain {{.chain_id}} to {{.to}}",
			InputSchema: ObjectSchema(map[string]interface{}{
				"chain_id": IntegerProperty("Chain ID (42161=Arbitrum, 8453=Base, 1=Ethereum)"),
				"to":       StringProperty("Contract address (0x...)"),
				"data":     StringProperty("Hex-encoded calldata (0x...). Must be pre-encoded."),
				"value":    StringProperty("Optional: ETH value to send in wei (default: 0)"),
				"gas_tier": StringEnumProperty("Optional: gas tier", "slow", "standard", "fast"),
			}, "chain_id", "to", "data"),
		},
	}
}

// LiminalTools creates Tool instances for all Liminal tools using the given executor.
func LiminalTools(executor core.ToolExecutor) []core.Tool {
	definitions := LiminalToolDefinitions()
	tools := make([]core.Tool, len(definitions))
	for i, def := range definitions {
		tools[i] = core.NewExecutorTool(def, executor)
	}
	return tools
}
