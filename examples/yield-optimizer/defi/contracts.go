package defi

// Arbitrum Mainnet contract addresses and constants.
const (
	ChainIDArbitrum = 42161

	// Aave V3 on Arbitrum
	AaveV3Pool = "0x794a61358D6845594F94dc1DB02A252b5b4814aD"

	// Tokens on Arbitrum
	USDC = "0xaf88d065e77c8cC2239327C5EDb3A432268e5831" // Native USDC (Circle)

	// Aave aTokens (interest-bearing receipt tokens)
	AaveAUSDC = "0x724dc807b04555b71ed48a6896b6F41593b8C637" // aArbUSDCn

	// USDC has 6 decimals
	USDCDecimals = 6

	// Aave uses RAY units (1e27) for rates
	RayDecimals = 27

	// Public Arbitrum RPC endpoints
	ArbitrumRPC         = "https://arb1.arbitrum.io/rpc"
	ArbitrumRPCFallback = "https://rpc.ankr.com/arbitrum"
)
