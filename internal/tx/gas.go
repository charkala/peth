package tx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
)

// GasStrategy determines the speed/cost tradeoff for gas estimation.
type GasStrategy string

const (
	// GasStrategyFast uses 1.5x multiplier for faster inclusion.
	GasStrategyFast GasStrategy = "fast"
	// GasStrategyNormal uses 1.2x multiplier for standard inclusion.
	GasStrategyNormal GasStrategy = "normal"
	// GasStrategySlow uses 1.0x multiplier for cheapest inclusion.
	GasStrategySlow GasStrategy = "slow"
)

// GasEstimate holds the estimated gas parameters.
type GasEstimate struct {
	MaxFeePerGas         string
	MaxPriorityFeePerGas string
	GasPrice             string // legacy fallback
	IsEIP1559            bool
}

// GasEstimator estimates gas parameters for transactions.
type GasEstimator interface {
	Estimate(rpcURL string, strategy GasStrategy) (*GasEstimate, error)
}

// RPCGasEstimator estimates gas via JSON-RPC.
type RPCGasEstimator struct {
	client *http.Client
}

// NewRPCGasEstimator creates a new RPCGasEstimator.
func NewRPCGasEstimator() *RPCGasEstimator {
	return &RPCGasEstimator{client: &http.Client{}}
}

// Estimate fetches gas parameters from the network.
// It tries eth_feeHistory for EIP-1559 chains, falling back to eth_gasPrice for legacy.
func (e *RPCGasEstimator) Estimate(rpcURL string, strategy GasStrategy) (*GasEstimate, error) {
	m, err := strategyMultiplier(strategy)
	if err != nil {
		return nil, err
	}

	// Try EIP-1559 first via eth_feeHistory.
	estimate, err := e.estimateEIP1559(rpcURL, m)
	if err == nil {
		return estimate, nil
	}

	// Fall back to legacy eth_gasPrice.
	return e.estimateLegacy(rpcURL, m)
}

// estimateEIP1559 fetches fee history and computes EIP-1559 gas params.
func (e *RPCGasEstimator) estimateEIP1559(rpcURL string, multiplier *multiplierRat) (*GasEstimate, error) {
	// eth_feeHistory(blockCount, newestBlock, rewardPercentiles)
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_feeHistory",
		Params:  []interface{}{4, "latest", []int{25, 50, 75}},
		ID:      1,
	}

	resp, err := e.doRPC(rpcURL, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("eth_feeHistory: %s", resp.Error.Message)
	}

	var feeHistory struct {
		BaseFeePerGas []string   `json:"baseFeePerGas"`
		Reward        [][]string `json:"reward"`
	}
	if err := json.Unmarshal(resp.Result, &feeHistory); err != nil {
		return nil, fmt.Errorf("parse fee history: %w", err)
	}

	if len(feeHistory.BaseFeePerGas) == 0 {
		return nil, fmt.Errorf("empty fee history")
	}

	// Use the latest base fee (last element includes next block's base fee).
	latestBaseFee := parseHexBigInt(feeHistory.BaseFeePerGas[len(feeHistory.BaseFeePerGas)-1])

	// Average the median (50th percentile) priority fees.
	var totalPriorityFee big.Int
	count := 0
	for _, rewards := range feeHistory.Reward {
		if len(rewards) >= 2 {
			totalPriorityFee.Add(&totalPriorityFee, parseHexBigInt(rewards[1])) // 50th percentile
			count++
		}
	}

	avgPriorityFee := new(big.Int)
	if count > 0 {
		avgPriorityFee.Div(&totalPriorityFee, big.NewInt(int64(count)))
	}

	// Apply multiplier to both base fee and priority fee.
	maxPriorityFee := applyMultiplier(avgPriorityFee, multiplier)
	maxFee := applyMultiplier(new(big.Int).Add(latestBaseFee, avgPriorityFee), multiplier)

	return &GasEstimate{
		MaxFeePerGas:         maxFee.String(),
		MaxPriorityFeePerGas: maxPriorityFee.String(),
		IsEIP1559:            true,
	}, nil
}

// estimateLegacy fetches gas price via eth_gasPrice.
func (e *RPCGasEstimator) estimateLegacy(rpcURL string, multiplier *multiplierRat) (*GasEstimate, error) {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_gasPrice",
		Params:  []interface{}{},
		ID:      1,
	}

	resp, err := e.doRPC(rpcURL, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("eth_gasPrice: %s", resp.Error.Message)
	}

	var priceHex string
	if err := json.Unmarshal(resp.Result, &priceHex); err != nil {
		return nil, fmt.Errorf("parse gas price: %w", err)
	}

	gasPrice := parseHexBigInt(priceHex)
	adjusted := applyMultiplier(gasPrice, multiplier)

	return &GasEstimate{
		GasPrice:  adjusted.String(),
		IsEIP1559: false,
	}, nil
}

// doRPC sends a JSON-RPC request and returns the parsed response.
func (e *RPCGasEstimator) doRPC(rpcURL string, req rpcRequest) (*rpcResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := e.client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &rpcResp, nil
}

// multiplierRat holds a rational multiplier (numerator/denominator).
type multiplierRat struct {
	num   int64
	denom int64
}

// strategyMultiplier returns the fee multiplier for a gas strategy as a rational number.
func strategyMultiplier(s GasStrategy) (*multiplierRat, error) {
	switch s {
	case GasStrategyFast:
		return &multiplierRat{3, 2}, nil // 1.5x
	case GasStrategyNormal:
		return &multiplierRat{6, 5}, nil // 1.2x
	case GasStrategySlow:
		return &multiplierRat{1, 1}, nil // 1.0x
	default:
		return nil, fmt.Errorf("invalid gas strategy: %q", s)
	}
}

// parseHexBigInt parses a 0x-prefixed hex string to *big.Int.
func parseHexBigInt(s string) *big.Int {
	s = strings.TrimPrefix(s, "0x")
	n := new(big.Int)
	n.SetString(s, 16)
	return n
}

// applyMultiplier multiplies an integer value by a rational multiplier, returning the integer result.
func applyMultiplier(value *big.Int, m *multiplierRat) *big.Int {
	result := new(big.Int).Mul(value, big.NewInt(m.num))
	result.Div(result, big.NewInt(m.denom))
	return result
}
