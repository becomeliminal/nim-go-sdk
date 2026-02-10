package defi

// YieldRate represents a yield opportunity from a DeFi protocol.
type YieldRate struct {
	Protocol string `json:"protocol"`
	Chain    string `json:"chain"`
	Token    string `json:"token"`
	APY      string `json:"apy"`
	TVL      string `json:"tvl"`
	Type     string `json:"type"` // "lending", "vault", etc.
	Risk     string `json:"risk"` // "low", "medium", "high"
}

// Position represents a user's deposit in a DeFi protocol.
type Position struct {
	Protocol     string `json:"protocol"`
	Token        string `json:"token"`
	Deposited    string `json:"deposited"`
	CurrentValue string `json:"current_value"`
	APY          string `json:"apy"`
	Earnings     string `json:"earnings"`
}

// AllocationSuggestion represents an optimized allocation recommendation.
type AllocationSuggestion struct {
	Protocol       string `json:"protocol"`
	Amount         string `json:"amount"`
	APY            string `json:"apy"`
	ProjectedYearly string `json:"projected_yearly"`
}
