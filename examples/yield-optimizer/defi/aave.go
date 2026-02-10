package defi

import (
	"context"
	"fmt"
	"math"
	"math/big"
)

// AaveClient reads Aave V3 on-chain data via RPC.
type AaveClient struct {
	rpc *RPCClient
}

// NewAaveClient creates a new Aave V3 client using the given RPC client.
func NewAaveClient(rpc *RPCClient) *AaveClient {
	return &AaveClient{rpc: rpc}
}

// GetSupplyAPY returns the current USDC supply APY on Aave V3 as a percentage (e.g., 4.23).
func (a *AaveClient) GetSupplyAPY(ctx context.Context) (float64, error) {
	calldata := EncodeGetReserveData(USDC)
	result, err := a.rpc.EthCall(ctx, AaveV3Pool, calldata)
	if err != nil {
		return 0, fmt.Errorf("getReserveData call failed: %w", err)
	}

	// getReserveData returns a ReserveData struct. The fields are ABI-encoded as:
	// [0]  (32 bytes) configuration (ReserveConfigurationMap)
	// [1]  (32 bytes) liquidityIndex (uint128)
	// [2]  (32 bytes) currentLiquidityRate (uint128) ← this is the supply rate in RAY
	// [3]  (32 bytes) variableBorrowIndex (uint128)
	// [4]  (32 bytes) currentVariableBorrowRate (uint128)
	// ... more fields follow
	//
	// Each field occupies 32 bytes in the ABI encoding.
	// currentLiquidityRate is at offset 2*32 = 64, spanning bytes [64:96].

	if len(result) < 96 {
		return 0, fmt.Errorf("unexpected response length: %d bytes (need at least 96)", len(result))
	}

	liquidityRate := decodeUint256(result[64:96])
	return rayToAPY(liquidityRate), nil
}

// GetUserBalance returns the user's aUSDC balance (current value including interest)
// as a formatted string (e.g., "1234.56") and the raw big.Int value.
func (a *AaveClient) GetUserBalance(ctx context.Context, userAddress string) (string, *big.Int, error) {
	calldata := EncodeBalanceOf(userAddress)
	result, err := a.rpc.EthCall(ctx, AaveAUSDC, calldata)
	if err != nil {
		return "0.00", big.NewInt(0), fmt.Errorf("balanceOf call failed: %w", err)
	}

	if len(result) < 32 {
		return "0.00", big.NewInt(0), nil
	}

	balance := decodeUint256(result[:32])
	return FormatUSDCAmount(balance), balance, nil
}

// GetAllowance returns the USDC allowance granted by owner to spender.
func (a *AaveClient) GetAllowance(ctx context.Context, owner, spender string) (*big.Int, error) {
	calldata := EncodeAllowance(owner, spender)
	result, err := a.rpc.EthCall(ctx, USDC, calldata)
	if err != nil {
		return big.NewInt(0), fmt.Errorf("allowance call failed: %w", err)
	}

	if len(result) < 32 {
		return big.NewInt(0), nil
	}

	return decodeUint256(result[:32]), nil
}

// rayToAPY converts an Aave RAY rate (1e27) to an annual percentage yield.
// The liquidityRate is a per-second rate in RAY, compounded over a year.
// For simplicity we use the linear approximation: APY ≈ rate * SECONDS_PER_YEAR / 1e27 * 100
func rayToAPY(rayRate *big.Int) float64 {
	if rayRate == nil || rayRate.Sign() == 0 {
		return 0
	}

	const secondsPerYear = 365.25 * 24 * 3600

	// Convert to float for the calculation
	rateFloat := new(big.Float).SetInt(rayRate)
	rayDivisor := new(big.Float).SetFloat64(math.Pow10(RayDecimals))

	ratePerSecond, _ := new(big.Float).Quo(rateFloat, rayDivisor).Float64()

	// Linear approximation of APY (good enough for display purposes)
	apy := ratePerSecond * secondsPerYear * 100

	// Round to 2 decimal places
	return math.Round(apy*100) / 100
}
