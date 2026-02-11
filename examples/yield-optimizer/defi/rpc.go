package defi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// RPCClient is a minimal Ethereum JSON-RPC client for eth_call operations.
type RPCClient struct {
	urls       []string
	httpClient *http.Client
	requestID  atomic.Int64
}

// NewRPCClient creates a new RPC client with the given endpoint URLs.
// The first URL is primary; others are fallbacks.
func NewRPCClient(urls ...string) *RPCClient {
	return &RPCClient{
		urls: urls,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int64         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// EthCall executes a read-only contract call (eth_call) and returns the raw result bytes.
func (c *RPCClient) EthCall(ctx context.Context, to string, calldata []byte) ([]byte, error) {
	params := []interface{}{
		map[string]string{
			"to":   to,
			"data": "0x" + hex.EncodeToString(calldata),
		},
		"latest",
	}

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params:  params,
		ID:      c.requestID.Add(1),
	}

	var lastErr error
	for _, url := range c.urls {
		result, err := c.doRequest(ctx, url, req)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("all RPC endpoints failed: %w", lastErr)
}

func (c *RPCClient) doRequest(ctx context.Context, url string, req rpcRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Result is a hex string like "0x..."
	var hexResult string
	if err := json.Unmarshal(rpcResp.Result, &hexResult); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	hexResult = strings.TrimPrefix(hexResult, "0x")
	return hex.DecodeString(hexResult)
}
