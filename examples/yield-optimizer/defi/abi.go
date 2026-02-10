package defi

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// Pre-computed function selectors (first 4 bytes of keccak256 of signature).
var (
	// Aave V3 Pool
	SelectorGetReserveData = mustDecodeHex("35ea6a75") // getReserveData(address)
	SelectorSupply         = mustDecodeHex("617ba037") // supply(address,uint256,address,uint16)
	SelectorWithdraw       = mustDecodeHex("69328dec") // withdraw(address,uint256,address)

	// ERC20
	SelectorBalanceOf = mustDecodeHex("70a08231") // balanceOf(address)
	SelectorApprove   = mustDecodeHex("095ea7b3") // approve(address,uint256)
	SelectorAllowance = mustDecodeHex("dd62ed3e") // allowance(address,address)

	// MaxUint256 for unlimited approval
	MaxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
)

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("invalid hex: %s", s))
	}
	return b
}

// encodeAddress pads a 20-byte Ethereum address to 32 bytes (left-padded with zeros).
func encodeAddress(addr string) []byte {
	addr = strings.TrimPrefix(addr, "0x")
	b, _ := hex.DecodeString(addr)
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}

// encodeUint256 encodes a big.Int as a 32-byte left-padded value.
func encodeUint256(n *big.Int) []byte {
	padded := make([]byte, 32)
	b := n.Bytes()
	copy(padded[32-len(b):], b)
	return padded
}

// encodeUint16 encodes a uint16 as a 32-byte left-padded value.
func encodeUint16(n uint16) []byte {
	padded := make([]byte, 32)
	padded[30] = byte(n >> 8)
	padded[31] = byte(n)
	return padded
}

// decodeUint256 decodes a 32-byte big-endian value into a big.Int.
func decodeUint256(data []byte) *big.Int {
	return new(big.Int).SetBytes(data)
}

// EncodeBalanceOf builds calldata for ERC20.balanceOf(address).
func EncodeBalanceOf(account string) []byte {
	data := make([]byte, 0, 4+32)
	data = append(data, SelectorBalanceOf...)
	data = append(data, encodeAddress(account)...)
	return data
}

// EncodeAllowance builds calldata for ERC20.allowance(owner, spender).
func EncodeAllowance(owner, spender string) []byte {
	data := make([]byte, 0, 4+64)
	data = append(data, SelectorAllowance...)
	data = append(data, encodeAddress(owner)...)
	data = append(data, encodeAddress(spender)...)
	return data
}

// EncodeApprove builds calldata for ERC20.approve(spender, amount).
func EncodeApprove(spender string, amount *big.Int) []byte {
	data := make([]byte, 0, 4+64)
	data = append(data, SelectorApprove...)
	data = append(data, encodeAddress(spender)...)
	data = append(data, encodeUint256(amount)...)
	return data
}

// EncodeGetReserveData builds calldata for Pool.getReserveData(address asset).
func EncodeGetReserveData(asset string) []byte {
	data := make([]byte, 0, 4+32)
	data = append(data, SelectorGetReserveData...)
	data = append(data, encodeAddress(asset)...)
	return data
}

// EncodeAaveSupply builds calldata for Pool.supply(asset, amount, onBehalfOf, referralCode).
func EncodeAaveSupply(asset string, amount *big.Int, onBehalfOf string) []byte {
	data := make([]byte, 0, 4+128)
	data = append(data, SelectorSupply...)
	data = append(data, encodeAddress(asset)...)
	data = append(data, encodeUint256(amount)...)
	data = append(data, encodeAddress(onBehalfOf)...)
	data = append(data, encodeUint16(0)...) // referralCode = 0
	return data
}

// EncodeAaveWithdraw builds calldata for Pool.withdraw(asset, amount, to).
func EncodeAaveWithdraw(asset string, amount *big.Int, to string) []byte {
	data := make([]byte, 0, 4+96)
	data = append(data, SelectorWithdraw...)
	data = append(data, encodeAddress(asset)...)
	data = append(data, encodeUint256(amount)...)
	data = append(data, encodeAddress(to)...)
	return data
}

// HexEncode returns 0x-prefixed hex encoding of data.
func HexEncode(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}

// ParseUSDCAmount converts a human-readable USDC amount (e.g., "100.50") to its
// on-chain representation (6 decimals). Returns the value in base units.
func ParseUSDCAmount(amount string) (*big.Int, error) {
	// Handle the decimal by splitting and reconstructing
	parts := strings.Split(amount, ".")
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}

	// Pad or truncate fractional part to 6 decimals
	for len(frac) < USDCDecimals {
		frac += "0"
	}
	frac = frac[:USDCDecimals]

	combined := whole + frac
	result, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("invalid USDC amount: %s", amount)
	}
	return result, nil
}

// FormatUSDCAmount converts on-chain USDC units (6 decimals) to human-readable format.
func FormatUSDCAmount(amount *big.Int) string {
	if amount == nil {
		return "0.00"
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(USDCDecimals), nil)
	whole := new(big.Int).Div(amount, divisor)
	remainder := new(big.Int).Mod(amount, divisor)

	// Pad remainder to 6 digits, then take first 2 for display
	fracStr := fmt.Sprintf("%06d", remainder.Int64())
	return fmt.Sprintf("%s.%s", whole.String(), fracStr[:2])
}
