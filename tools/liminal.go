package tools

import (
	"github.com/becomeliminal/nim-go-sdk/core"
)

// LiminalToolDefinitions returns the definitions for all Liminal tools.
// These are the standard tools available through the Liminal API.
func LiminalToolDefinitions() []core.ToolDefinition {
	return []core.ToolDefinition{
		// Read operations (thought optional)
		{
			ToolName:        "get_balance",
			ToolDescription: "Get the user's wallet balance across all supported currencies. Returns balances in USDC (dollars) and EURC (euros). When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'.",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"currency": StringProperty("Optional: filter by currency (e.g., 'USDC' for dollars, 'EURC' for euros)"),
			}, false),
		},
		{
			ToolName:        "get_savings_balance",
			ToolDescription: "Get the user's savings positions and current APY.",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"vault": StringProperty("Optional: filter by vault name"),
			}, false),
		},
		{
			ToolName:        "get_vault_rates",
			ToolDescription: "Get current APY rates for available savings vaults.",
			InputSchema:     BuildSchemaWithThought(map[string]interface{}{}, false),
		},
		{
			ToolName:        "get_transactions",
			ToolDescription: "Get the user's recent transaction history.",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"limit": IntegerProperty("Number of transactions to return (default: 10)"),
				"type":  StringEnumProperty("Filter by transaction type", "send", "receive", "deposit", "withdraw"),
			}, false),
		},
		{
			ToolName:        "get_profile",
			ToolDescription: "Get the user's profile information.",
			InputSchema:     BuildSchemaWithThought(map[string]interface{}{}, false),
		},
		{
			ToolName:        "search_users",
			ToolDescription: "Search for users by display tag or name.",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"query": StringProperty("Search query (display tag like @alice or name)"),
			}, false, "query"),
		},

		// Write operations (thought required)
		{
			ToolName:                 "send_money",
			ToolDescription:          "Send money to another user. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Send {{.amount}} {{.currency}} to {{.recipient}}",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"recipient": StringProperty("Recipient's display tag (e.g., @alice) or user ID"),
				"amount":    StringProperty("Amount to send (e.g., '50.00')"),
				"currency":  StringProperty("Currency to send. Use 'USDC' for dollars, 'EURC' for euros"),
				"note":      StringProperty("Optional payment note"),
			}, true, "recipient", "amount", "currency"),
		},
		{
			ToolName:                 "deposit_savings",
			ToolDescription:          "Deposit funds into savings to earn yield. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Deposit {{.amount}} {{.currency}} into savings",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"amount":   StringProperty("Amount to deposit"),
				"currency": StringProperty("Currency to deposit. Use 'USDC' for dollars, 'EURC' for euros"),
			}, true, "amount", "currency"),
		},
		{
			ToolName:                 "withdraw_savings",
			ToolDescription:          "Withdraw funds from savings back to your wallet. When users say 'USD' or 'dollars', use 'USDC'. When users say 'EUR' or 'euros', use 'EURC'. Requires confirmation.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Withdraw {{.amount}} {{.currency}} from savings",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"amount":   StringProperty("Amount to withdraw"),
				"currency": StringProperty("Currency to withdraw. Use 'USDC' for dollars, 'EURC' for euros"),
			}, true, "amount", "currency"),
		},
		{
			ToolName:                 "execute_contract_call",
			ToolDescription:          "Execute an arbitrary smart contract call on any blockchain. Requires confirmation. You must provide pre-encoded calldata as hex.",
			RequiresUserConfirmation: true,
			SummaryTemplate:          "Execute contract call on chain {{.chain_id}} to {{.to}}",
			InputSchema: BuildSchemaWithThought(map[string]interface{}{
				"chain_id": IntegerProperty("Chain ID (42161=Arbitrum, 8453=Base, 1=Ethereum)"),
				"to":       StringProperty("Contract address (0x...)"),
				"data":     StringProperty("Hex-encoded calldata (0x...). Must be pre-encoded."),
				"value":    StringProperty("Optional: ETH value to send in wei (default: 0)"),
				"gas_tier": StringEnumProperty("Optional: gas tier", "slow", "standard", "fast"),
			}, true, "chain_id", "to", "data"),
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
