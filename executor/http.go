// Package executor provides ToolExecutor implementations.
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
)

// HTTPExecutor implements ToolExecutor by calling the agent_gateway over HTTP.
// This is the public implementation used by external developers.
type HTTPExecutor struct {
	baseURL    string
	apiKey     string  // Deprecated: use jwtToken
	jwtToken   string  // JWT for Bearer authentication
	httpClient *http.Client
}

// HTTPExecutorConfig configures the HTTP executor.
type HTTPExecutorConfig struct {
	// BaseURL is the agent_gateway URL (e.g., "https://api.liminal.cash").
	BaseURL string

	// Deprecated: Use JWTToken instead.
	// APIKey is the Liminal API key for authentication.
	APIKey string

	// JWTToken is the JWT token for Bearer authentication.
	JWTToken string

	// Timeout is the HTTP request timeout.
	Timeout time.Duration
}

// NewHTTPExecutor creates a new HTTP-based tool executor.
func NewHTTPExecutor(cfg HTTPExecutorConfig) *HTTPExecutor {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPExecutor{
		baseURL:  cfg.BaseURL,
		apiKey:   cfg.APIKey,   // Keep for backward compatibility
		jwtToken: cfg.JWTToken, // New JWT field
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Execute runs a read-only tool via HTTP.
func (e *HTTPExecutor) Execute(ctx context.Context, req *core.ExecuteRequest) (*core.ExecuteResponse, error) {
	fmt.Printf("[HTTP] Execute called for tool: %s, user: %s\n", req.Tool, req.UserID)
	endpoint := e.endpointForTool(req.Tool)
	return e.doRequest(ctx, "GET", endpoint, req, req.Tool)
}

// ExecuteWrite runs a write tool that may require confirmation.
func (e *HTTPExecutor) ExecuteWrite(ctx context.Context, req *core.ExecuteRequest) (*core.ExecuteResponse, error) {
	fmt.Printf("[HTTP] ExecuteWrite called for tool: %s, user: %s\n", req.Tool, req.UserID)
	endpoint := e.endpointForTool(req.Tool)
	return e.doRequest(ctx, "POST", endpoint, req, req.Tool)
}

// Confirm executes a previously confirmed write operation.
func (e *HTTPExecutor) Confirm(ctx context.Context, userID, confirmationID string) (*core.ExecuteResponse, error) {
	fmt.Printf("[HTTP] Confirm called for confirmationID: %s, user: %s\n", confirmationID, userID)
	endpoint := fmt.Sprintf("/nim/v1/agent/confirmations/%s/confirm", confirmationID)
	return e.doRequest(ctx, "POST", endpoint, nil, "")
}

// Cancel cancels a pending confirmation.
func (e *HTTPExecutor) Cancel(ctx context.Context, userID, confirmationID string) error {
	fmt.Printf("[HTTP] Cancel called for confirmationID: %s, user: %s\n", confirmationID, userID)
	endpoint := fmt.Sprintf("/nim/v1/agent/confirmations/%s/cancel", confirmationID)
	_, err := e.doRequest(ctx, "POST", endpoint, nil, "")
	return err
}

// endpointForTool maps tool names to HTTP endpoints.
func (e *HTTPExecutor) endpointForTool(tool string) string {
	// Map tool names to nim_gateway endpoints
	endpoints := map[string]string{
		"get_balance":            "/nim/v1/agent/wallet/balance",
		"get_savings_balance":    "/nim/v1/agent/savings/balance",
		"get_vault_rates":        "/nim/v1/agent/savings/vaults",
		"get_transactions":       "/nim/v1/agent/transactions",
		"get_profile":            "/nim/v1/agent/profile",
		"search_users":           "/nim/v1/agent/users/search",
		"send_money":             "/nim/v1/agent/payments/send",
		"deposit_savings":        "/nim/v1/agent/savings/deposit",
		"withdraw_savings":       "/nim/v1/agent/savings/withdraw",
		"execute_contract_call":  "/nim/v1/agent/wallet/execute",
	}

	if endpoint, ok := endpoints[tool]; ok {
		return endpoint
	}
	// Default: use tool name as endpoint
	return fmt.Sprintf("/nim/v1/agent/tools/%s", tool)
}

// doRequest performs an HTTP request to the agent_gateway.
func (e *HTTPExecutor) doRequest(ctx context.Context, method, endpoint string, body interface{}, toolName string) (*core.ExecuteResponse, error) {
	urlStr := e.baseURL + endpoint
	fmt.Printf("[HTTP] %s %s\n", method, urlStr)

	var bodyReader io.Reader

	// For GET requests, encode parameters as query string instead of body
	if method == "GET" && body != nil {
		if execReq, ok := body.(*core.ExecuteRequest); ok && len(execReq.Input) > 0 {
			// Parse Input JSON and add as query parameters
			var params map[string]interface{}
			if err := json.Unmarshal(execReq.Input, &params); err == nil {
				query := make([]string, 0, len(params))
				for k, v := range params {
					query = append(query, fmt.Sprintf("%s=%v", k, v))
				}
				if len(query) > 0 {
					urlStr += "?" + strings.Join(query, "&")
				}
			}
		}
		bodyReader = nil
	} else if body != nil {
		// For grpc-gateway endpoints, send only the tool parameters (Input field),
		// not the entire ExecuteRequest wrapper. User ID comes from JWT auth.
		var bodyToSend interface{}
		if execReq, ok := body.(*core.ExecuteRequest); ok {
			// Extract just the Input params from the wrapper
			var params map[string]interface{}
			if len(execReq.Input) > 0 {
				if err := json.Unmarshal(execReq.Input, &params); err != nil {
					return nil, fmt.Errorf("failed to unmarshal input params: %w", err)
				}
				bodyToSend = params
				fmt.Printf("[HTTP] Extracted params from ExecuteRequest: %+v\n", params)
			} else {
				bodyToSend = map[string]interface{}{}
				fmt.Printf("[HTTP] Empty input, sending empty params\n")
			}
		} else {
			bodyToSend = body // Non-ExecuteRequest bodies pass through
			fmt.Printf("[HTTP] Non-ExecuteRequest body: %+v\n", body)
		}

		bodyBytes, err := json.Marshal(bodyToSend)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		fmt.Printf("[HTTP] Request body: %s\n", string(bodyBytes))
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Prefer JWT over API key
	if e.jwtToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.jwtToken))
		fmt.Printf("[HTTP] Using JWT auth (token: %s...)\n", e.jwtToken[:20])
	} else if e.apiKey != "" {
		// Fallback to API key for backward compatibility
		req.Header.Set("X-API-Key", e.apiKey)
		fmt.Printf("[HTTP] Using API key auth\n")
	} else {
		fmt.Printf("[HTTP] WARNING: No authentication configured!\n")
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		fmt.Printf("[HTTP] Request failed: %v\n", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[HTTP] Failed to read response: %v\n", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("[HTTP] Response status: %d\n", resp.StatusCode)
	fmt.Printf("[HTTP] Response body: %s\n", string(respBody))

	if resp.StatusCode >= 400 {
		return &core.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Gateway returns raw proto response (not wrapped in ExecuteResponse)
	// Unmarshal into the proper type to validate the structure
	responseType := toolResponseType(toolName)
	if err := json.Unmarshal(respBody, responseType); err != nil {
		return nil, fmt.Errorf("failed to parse %s response: %w", toolName, err)
	}

	// Marshal back to JSON bytes for ExecuteResponse.Data
	dataBytes, err := json.Marshal(responseType)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s response: %w", toolName, err)
	}

	return &core.ExecuteResponse{
		Success: true,
		Data:    json.RawMessage(dataBytes),
	}, nil
}

// UpdateJWT updates the JWT token used for authentication.
// This should be called when the token is refreshed.
func (e *HTTPExecutor) UpdateJWT(jwt string) {
	e.jwtToken = jwt
}
