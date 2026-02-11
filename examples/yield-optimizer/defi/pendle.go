package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const pendleAPIBase = "https://api-v2.pendle.finance/core/v1"

// PendleClient fetches market data from the Pendle API.
type PendleClient struct {
	httpClient *http.Client
}

// NewPendleClient creates a new Pendle API client.
func NewPendleClient() *PendleClient {
	return &PendleClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// PendleMarket represents a Pendle market with fixed yield.
type PendleMarket struct {
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	Expiry     string  `json:"expiry"`
	ImpliedAPY float64 `json:"implied_apy"` // As percentage (e.g., 7.42)
	PTAddress  string  `json:"pt_address"`
	Underlying string  `json:"underlying"`
}

type pendleAPIResponse struct {
	Results []pendleMarketRaw `json:"results"`
}

type pendleMarketRaw struct {
	ProName    string          `json:"proName"`
	Name       string          `json:"name"`
	Address    string          `json:"address"`
	Expiry     string          `json:"expiry"`
	ImpliedAPY float64         `json:"impliedApy"`
	PT         json.RawMessage `json:"pt"`
}

// GetStablecoinMarkets returns active Pendle markets for stablecoin-adjacent assets on Arbitrum.
func (c *PendleClient) GetStablecoinMarkets(ctx context.Context) ([]PendleMarket, error) {
	url := fmt.Sprintf("%s/%d/markets?order_by=name:1&skip=0&limit=100", pendleAPIBase, ChainIDArbitrum)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch markets: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp pendleAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	var markets []PendleMarket
	now := time.Now()

	for _, m := range apiResp.Results {
		// Skip expired or zero-APY markets
		if m.ImpliedAPY <= 0 {
			continue
		}

		expiry, err := time.Parse(time.RFC3339, m.Expiry)
		if err != nil {
			continue
		}
		if expiry.Before(now) {
			continue
		}

		name := m.ProName
		if name == "" {
			name = m.Name
		}

		// Filter to stablecoin-adjacent markets
		lowName := toLowerCase(name)
		if !containsAny(lowName, "usd", "dai", "gusdc", "usdc", "usdt", "usde", "fxusd") {
			continue
		}

		// Extract PT address
		var ptData struct {
			Address string `json:"address"`
		}
		json.Unmarshal(m.PT, &ptData)

		daysToExpiry := int(expiry.Sub(now).Hours() / 24)

		markets = append(markets, PendleMarket{
			Name:       fmt.Sprintf("%s (%dd)", name, daysToExpiry),
			Address:    m.Address,
			Expiry:     expiry.Format("2006-01-02"),
			ImpliedAPY: m.ImpliedAPY * 100, // Convert to percentage
			PTAddress:  ptData.Address,
			Underlying: name,
		})
	}

	return markets, nil
}

func toLowerCase(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
