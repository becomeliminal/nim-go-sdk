package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defiLlamaYieldsURL = "https://yields.llama.fi/pools"

// DefiLlamaClient fetches yield data from the DefiLlama Yields API.
type DefiLlamaClient struct {
	httpClient *http.Client
}

// NewDefiLlamaClient creates a new DefiLlama client.
func NewDefiLlamaClient() *DefiLlamaClient {
	return &DefiLlamaClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type defiLlamaResponse struct {
	Status string          `json:"status"`
	Data   []defiLlamaPool `json:"data"`
}

type defiLlamaPool struct {
	Pool       string  `json:"pool"`
	Chain      string  `json:"chain"`
	Project    string  `json:"project"`
	Symbol     string  `json:"symbol"`
	TVLUsd     float64 `json:"tvlUsd"`
	APY        float64 `json:"apy"`
	APYBase    float64 `json:"apyBase"`
	APYReward  float64 `json:"apyReward"`
	StableCoin bool    `json:"stablecoin"`
}

// AaveArbitrumUSDCYield fetches the current Aave V3 USDC yield on Arbitrum from DefiLlama.
// Returns APY and TVL. This serves as enrichment data alongside direct RPC reads.
func (c *DefiLlamaClient) AaveArbitrumUSDCYield(ctx context.Context) (apy float64, tvl float64, err error) {
	pool, err := c.findPool(ctx, "aave-v3", "Arbitrum", "USDC")
	if err != nil {
		return 0, 0, err
	}
	return pool.APY, pool.TVLUsd, nil
}

// MorphoArbitrumUSDCYield fetches Morpho USDC yield data from DefiLlama if available.
func (c *DefiLlamaClient) MorphoArbitrumUSDCYield(ctx context.Context) (apy float64, tvl float64, err error) {
	pool, err := c.findPool(ctx, "morpho", "Arbitrum", "USDC")
	if err != nil {
		// Morpho may not have an Arbitrum pool on DefiLlama — not an error
		return 0, 0, nil
	}
	return pool.APY, pool.TVLUsd, nil
}

func (c *DefiLlamaClient) findPool(ctx context.Context, project, chain, symbol string) (*defiLlamaPool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", defiLlamaYieldsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch yields: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result defiLlamaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	for _, pool := range result.Data {
		if pool.Project == project && pool.Chain == chain {
			// Match symbol — DefiLlama uses compound symbols like "USDC" or "USDC.e"
			if pool.Symbol == symbol || pool.Symbol == symbol+".e" {
				return &pool, nil
			}
		}
	}

	return nil, fmt.Errorf("pool not found: %s/%s/%s", project, chain, symbol)
}
