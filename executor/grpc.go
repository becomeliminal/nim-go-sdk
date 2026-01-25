package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/store"
	"github.com/google/uuid"
)

// GRPCExecutor implements ToolExecutor using gRPC clients.
// This executor is used internally within the Liminal infrastructure
// where direct service access is available.
type GRPCExecutor struct {
	wallets  WalletService
	payments PaymentService
	savings  SavingsService
	users    UserService
	ledger   LedgerService

	// confirmations stores pending actions awaiting user approval
	confirmations store.Confirmations
}

// WalletService defines the interface for wallet operations.
type WalletService interface {
	GetBalance(ctx context.Context, userID string, currency *string) (json.RawMessage, error)
}

// PaymentService defines the interface for payment operations.
type PaymentService interface {
	Send(ctx context.Context, userID, recipient, amount, currency string, note *string) (json.RawMessage, error)
}

// SavingsService defines the interface for savings operations.
type SavingsService interface {
	GetBalance(ctx context.Context, userID string, vault *string) (json.RawMessage, error)
	GetVaultRates(ctx context.Context) (json.RawMessage, error)
	Deposit(ctx context.Context, userID, amount, currency string) (json.RawMessage, error)
	Withdraw(ctx context.Context, userID, amount, currency string) (json.RawMessage, error)
}

// UserService defines the interface for user operations.
type UserService interface {
	GetProfile(ctx context.Context, userID string) (json.RawMessage, error)
	Search(ctx context.Context, query string) (json.RawMessage, error)
}

// LedgerService defines the interface for ledger operations.
type LedgerService interface {
	GetTransactions(ctx context.Context, userID string, limit int, txType *string) (json.RawMessage, error)
}

// GRPCExecutorConfig configures the gRPC executor.
type GRPCExecutorConfig struct {
	Wallets       WalletService
	Payments      PaymentService
	Savings       SavingsService
	Users         UserService
	Ledger        LedgerService
	Confirmations store.Confirmations
}

// NewGRPCExecutor creates a new gRPC-based tool executor.
func NewGRPCExecutor(cfg GRPCExecutorConfig) *GRPCExecutor {
	return &GRPCExecutor{
		wallets:       cfg.Wallets,
		payments:      cfg.Payments,
		savings:       cfg.Savings,
		users:         cfg.Users,
		ledger:        cfg.Ledger,
		confirmations: cfg.Confirmations,
	}
}

// Execute runs a read-only tool.
func (e *GRPCExecutor) Execute(ctx context.Context, req *core.ExecuteRequest) (*core.ExecuteResponse, error) {
	var data json.RawMessage
	var err error

	switch req.Tool {
	case "get_balance":
		data, err = e.executeGetBalance(ctx, req)
	case "get_savings_balance":
		data, err = e.executeGetSavingsBalance(ctx, req)
	case "get_vault_rates":
		data, err = e.executeGetVaultRates(ctx, req)
	case "get_transactions":
		data, err = e.executeGetTransactions(ctx, req)
	case "get_profile":
		data, err = e.executeGetProfile(ctx, req)
	case "search_users":
		data, err = e.executeSearchUsers(ctx, req)
	default:
		return &core.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}

	if err != nil {
		return &core.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &core.ExecuteResponse{
		Success: true,
		Data:    data,
	}, nil
}

// ExecuteWrite runs a write tool that may require confirmation.
func (e *GRPCExecutor) ExecuteWrite(ctx context.Context, req *core.ExecuteRequest) (*core.ExecuteResponse, error) {
	// Generate confirmation for write operations
	confirmationID := uuid.New().String()

	var summary string
	switch req.Tool {
	case "send_money":
		summary = e.generateSendMoneySummary(req.Input)
	case "deposit_savings":
		summary = e.generateDepositSummary(req.Input)
	case "withdraw_savings":
		summary = e.generateWithdrawSummary(req.Input)
	default:
		return &core.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown write tool: %s", req.Tool),
		}, nil
	}

	action := &core.PendingAction{
		ID:        confirmationID,
		UserID:    req.UserID,
		Tool:      req.Tool,
		Input:     req.Input,
		Summary:   summary,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	}

	if e.confirmations != nil {
		if err := e.confirmations.Store(ctx, action); err != nil {
			return &core.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to store confirmation: %v", err),
			}, nil
		}
	}

	return &core.ExecuteResponse{
		Success:              true,
		RequiresConfirmation: true,
		Confirmation: &core.ConfirmationDetails{
			ID:        confirmationID,
			Summary:   summary,
			ExpiresAt: action.ExpiresAt,
		},
	}, nil
}

// Confirm executes a previously confirmed write operation.
func (e *GRPCExecutor) Confirm(ctx context.Context, userID, confirmationID string) (*core.ExecuteResponse, error) {
	if e.confirmations == nil {
		return &core.ExecuteResponse{
			Success: false,
			Error:   "confirmation store not configured",
		}, nil
	}

	// Confirm retrieves and removes the pending action atomically
	action, err := e.confirmations.Confirm(ctx, userID, confirmationID)
	if err != nil {
		return &core.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Execute the confirmed operation
	var data json.RawMessage
	switch action.Tool {
	case "send_money":
		data, err = e.executeSendMoney(ctx, action.UserID, action.Input)
	case "deposit_savings":
		data, err = e.executeDepositSavings(ctx, action.UserID, action.Input)
	case "withdraw_savings":
		data, err = e.executeWithdrawSavings(ctx, action.UserID, action.Input)
	default:
		return &core.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", action.Tool),
		}, nil
	}

	if err != nil {
		return &core.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &core.ExecuteResponse{
		Success: true,
		Data:    data,
	}, nil
}

// Cancel cancels a pending confirmation.
func (e *GRPCExecutor) Cancel(ctx context.Context, userID, confirmationID string) error {
	if e.confirmations == nil {
		return fmt.Errorf("confirmation store not configured")
	}

	return e.confirmations.Cancel(ctx, userID, confirmationID)
}

// Read operation implementations

func (e *GRPCExecutor) executeGetBalance(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.wallets == nil {
		return nil, fmt.Errorf("wallet service not configured")
	}

	var input struct {
		Currency *string `json:"currency"`
	}
	json.Unmarshal(req.Input, &input)

	return e.wallets.GetBalance(ctx, req.UserID, input.Currency)
}

func (e *GRPCExecutor) executeGetSavingsBalance(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.savings == nil {
		return nil, fmt.Errorf("savings service not configured")
	}

	var input struct {
		Vault *string `json:"vault"`
	}
	json.Unmarshal(req.Input, &input)

	return e.savings.GetBalance(ctx, req.UserID, input.Vault)
}

func (e *GRPCExecutor) executeGetVaultRates(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.savings == nil {
		return nil, fmt.Errorf("savings service not configured")
	}

	return e.savings.GetVaultRates(ctx)
}

func (e *GRPCExecutor) executeGetTransactions(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.ledger == nil {
		return nil, fmt.Errorf("ledger service not configured")
	}

	var input struct {
		Limit int     `json:"limit"`
		Type  *string `json:"type"`
	}
	json.Unmarshal(req.Input, &input)

	limit := input.Limit
	if limit == 0 {
		limit = 10
	}

	return e.ledger.GetTransactions(ctx, req.UserID, limit, input.Type)
}

func (e *GRPCExecutor) executeGetProfile(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.users == nil {
		return nil, fmt.Errorf("user service not configured")
	}

	return e.users.GetProfile(ctx, req.UserID)
}

func (e *GRPCExecutor) executeSearchUsers(ctx context.Context, req *core.ExecuteRequest) (json.RawMessage, error) {
	if e.users == nil {
		return nil, fmt.Errorf("user service not configured")
	}

	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return nil, err
	}

	return e.users.Search(ctx, input.Query)
}

// Write operation implementations

func (e *GRPCExecutor) executeSendMoney(ctx context.Context, userID string, input json.RawMessage) (json.RawMessage, error) {
	if e.payments == nil {
		return nil, fmt.Errorf("payment service not configured")
	}

	var params struct {
		Recipient string  `json:"recipient"`
		Amount    string  `json:"amount"`
		Currency  string  `json:"currency"`
		Note      *string `json:"note"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, err
	}

	return e.payments.Send(ctx, userID, params.Recipient, params.Amount, params.Currency, params.Note)
}

func (e *GRPCExecutor) executeDepositSavings(ctx context.Context, userID string, input json.RawMessage) (json.RawMessage, error) {
	if e.savings == nil {
		return nil, fmt.Errorf("savings service not configured")
	}

	var params struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, err
	}

	return e.savings.Deposit(ctx, userID, params.Amount, params.Currency)
}

func (e *GRPCExecutor) executeWithdrawSavings(ctx context.Context, userID string, input json.RawMessage) (json.RawMessage, error) {
	if e.savings == nil {
		return nil, fmt.Errorf("savings service not configured")
	}

	var params struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, err
	}

	return e.savings.Withdraw(ctx, userID, params.Amount, params.Currency)
}

// Summary generation helpers

func (e *GRPCExecutor) generateSendMoneySummary(input json.RawMessage) string {
	var params struct {
		Recipient string `json:"recipient"`
		Amount    string `json:"amount"`
		Currency  string `json:"currency"`
	}
	json.Unmarshal(input, &params)

	return fmt.Sprintf("Send %s %s to %s", params.Amount, params.Currency, params.Recipient)
}

func (e *GRPCExecutor) generateDepositSummary(input json.RawMessage) string {
	var params struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	json.Unmarshal(input, &params)

	return fmt.Sprintf("Deposit %s %s into savings", params.Amount, params.Currency)
}

func (e *GRPCExecutor) generateWithdrawSummary(input json.RawMessage) string {
	var params struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	json.Unmarshal(input, &params)

	return fmt.Sprintf("Withdraw %s %s from savings", params.Amount, params.Currency)
}
