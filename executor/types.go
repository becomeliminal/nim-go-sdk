package executor

// Response types that match nim/gateway proto definitions
// These use camelCase JSON tags to match the grpc-gateway JSON output

// Wallet types
type GetBalanceResponse struct {
	Balances []WalletBalance `json:"balances"`
	TotalUSD string          `json:"totalUsd"`
}

type WalletBalance struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
	USDValue string `json:"usdValue"`
}

type ExecuteContractCallResponse struct {
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transactionId,omitempty"`
	TxHash        string `json:"txHash,omitempty"`
	Status        string `json:"status,omitempty"`
}

// Savings types
type GetSavingsBalanceResponse struct {
	Positions []SavingsPosition `json:"positions"`
	TotalUSD  string            `json:"totalUsd"`
}

type SavingsPosition struct {
	Currency     string `json:"currency"`
	Deposited    string `json:"deposited"`
	CurrentValue string `json:"currentValue"`
	APY          string `json:"apy"`
	Earnings     string `json:"earnings"`
}

type GetVaultRatesResponse struct {
	Vaults []VaultRate `json:"vaults"`
}

type VaultRate struct {
	Currency string `json:"currency"`
	APY      string `json:"apy"`
	TVL      string `json:"tvl"`
}

type DepositResponse struct {
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transactionId,omitempty"`
	TxHash        string `json:"txHash,omitempty"`
}

type WithdrawResponse struct {
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transactionId,omitempty"`
	TxHash        string `json:"txHash,omitempty"`
}

// Payments types
type SendMoneyResponse struct {
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transactionId,omitempty"`
	TxHash        string `json:"txHash,omitempty"`
}

// Ledger types
type GetTransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
	NextCursor   string        `json:"nextCursor,omitempty"`
}

type Transaction struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	USDValue     string `json:"usdValue"`
	Counterparty string `json:"counterparty"`
	Note         string `json:"note"`
	Status       string `json:"status"`
	Direction    string `json:"direction"`
	CreatedAt    string `json:"createdAt"`
	TxHash       string `json:"txHash"`
}

// Users types
type GetProfileResponse struct {
	UserID     string `json:"userId"`
	DisplayTag string `json:"displayTag"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
}

type SearchUsersResponse struct {
	Users []UserResult `json:"users"`
}

type UserResult struct {
	UserID     string `json:"userId"`
	DisplayTag string `json:"displayTag"`
	Name       string `json:"name"`
}

// Chat types
type ListConversationsResponse struct {
	Conversations []ConversationSummary `json:"conversations"`
	NextCursor    string                `json:"nextCursor,omitempty"`
}

type ConversationSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	MessageCount int32  `json:"messageCount"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type GetConversationResponse struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Messages  []ChatMessage  `json:"messages"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

type ChatMessage struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	Content   string   `json:"content"`
	ToolsUsed []string `json:"toolsUsed,omitempty"`
	Timestamp int64    `json:"timestamp"`
}

type CreateConversationResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAt"`
}

type AppendMessageResponse struct {
	Message ChatMessage `json:"message"`
}

// toolResponseType maps tool names to their response types
func toolResponseType(toolName string) interface{} {
	switch toolName {
	case "get_balance":
		return &GetBalanceResponse{}
	case "get_savings_balance":
		return &GetSavingsBalanceResponse{}
	case "get_vault_rates":
		return &GetVaultRatesResponse{}
	case "deposit_savings":
		return &DepositResponse{}
	case "withdraw_savings":
		return &WithdrawResponse{}
	case "send_money":
		return &SendMoneyResponse{}
	case "get_transactions":
		return &GetTransactionsResponse{}
	case "get_profile":
		return &GetProfileResponse{}
	case "search_users":
		return &SearchUsersResponse{}
	case "execute_contract_call":
		return &ExecuteContractCallResponse{}
	default:
		// For unknown tools, use generic map
		return &map[string]interface{}{}
	}
}
