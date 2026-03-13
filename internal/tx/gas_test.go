package tx

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func feeHistoryServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "eth_feeHistory":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"baseFeePerGas": []string{
						"0x3b9aca00", // 1 Gwei
						"0x3b9aca00",
						"0x3b9aca00",
						"0x3b9aca00",
						"0x3b9aca00", // next block
					},
					"reward": [][]string{
						{"0x3b9aca00", "0x77359400", "0xb2d05e00"}, // 1, 2, 3 Gwei
						{"0x3b9aca00", "0x77359400", "0xb2d05e00"},
						{"0x3b9aca00", "0x77359400", "0xb2d05e00"},
						{"0x3b9aca00", "0x77359400", "0xb2d05e00"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			t.Errorf("unexpected method: %s", req.Method)
		}
	}))
}

func TestEstimateFast(t *testing.T) {
	server := feeHistoryServer(t)
	defer server.Close()

	estimator := NewRPCGasEstimator()
	estimate, err := estimator.Estimate(server.URL, GasStrategyFast)
	if err != nil {
		t.Fatalf("Estimate(fast) returned error: %v", err)
	}

	if !estimate.IsEIP1559 {
		t.Error("IsEIP1559 = false, want true")
	}

	// Base fee = 1 Gwei, avg priority = 2 Gwei (50th percentile)
	// MaxFee = (1 + 2) * 1.5 = 4.5 Gwei = 4500000000
	// MaxPriorityFee = 2 * 1.5 = 3 Gwei = 3000000000
	maxFee := new(big.Int)
	maxFee.SetString(estimate.MaxFeePerGas, 10)
	expectedMaxFee := big.NewInt(4500000000)
	if maxFee.Cmp(expectedMaxFee) != 0 {
		t.Errorf("MaxFeePerGas = %s, want %s", maxFee.String(), expectedMaxFee.String())
	}

	maxPriority := new(big.Int)
	maxPriority.SetString(estimate.MaxPriorityFeePerGas, 10)
	expectedPriority := big.NewInt(3000000000)
	if maxPriority.Cmp(expectedPriority) != 0 {
		t.Errorf("MaxPriorityFeePerGas = %s, want %s", maxPriority.String(), expectedPriority.String())
	}
}

func TestEstimateNormal(t *testing.T) {
	server := feeHistoryServer(t)
	defer server.Close()

	estimator := NewRPCGasEstimator()
	estimate, err := estimator.Estimate(server.URL, GasStrategyNormal)
	if err != nil {
		t.Fatalf("Estimate(normal) returned error: %v", err)
	}

	if !estimate.IsEIP1559 {
		t.Error("IsEIP1559 = false, want true")
	}

	// MaxFee = (1 + 2) * 1.2 = 3.6 Gwei = 3600000000
	maxFee := new(big.Int)
	maxFee.SetString(estimate.MaxFeePerGas, 10)
	expectedMaxFee := big.NewInt(3600000000)
	if maxFee.Cmp(expectedMaxFee) != 0 {
		t.Errorf("MaxFeePerGas = %s, want %s", maxFee.String(), expectedMaxFee.String())
	}
}

func TestEstimateSlow(t *testing.T) {
	server := feeHistoryServer(t)
	defer server.Close()

	estimator := NewRPCGasEstimator()
	estimate, err := estimator.Estimate(server.URL, GasStrategySlow)
	if err != nil {
		t.Fatalf("Estimate(slow) returned error: %v", err)
	}

	if !estimate.IsEIP1559 {
		t.Error("IsEIP1559 = false, want true")
	}

	// MaxFee = (1 + 2) * 1.0 = 3.0 Gwei = 3000000000
	maxFee := new(big.Int)
	maxFee.SetString(estimate.MaxFeePerGas, 10)
	expectedMaxFee := big.NewInt(3000000000)
	if maxFee.Cmp(expectedMaxFee) != 0 {
		t.Errorf("MaxFeePerGas = %s, want %s", maxFee.String(), expectedMaxFee.String())
	}
}

func TestEstimateLegacyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "eth_feeHistory":
			// Return error to trigger legacy fallback.
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "method not supported",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "eth_gasPrice":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result":  "0x4a817c800", // 20 Gwei
			}
			json.NewEncoder(w).Encode(resp)
		default:
			t.Errorf("unexpected method: %s", req.Method)
		}
	}))
	defer server.Close()

	estimator := NewRPCGasEstimator()
	estimate, err := estimator.Estimate(server.URL, GasStrategyNormal)
	if err != nil {
		t.Fatalf("Estimate(normal) with fallback returned error: %v", err)
	}

	if estimate.IsEIP1559 {
		t.Error("IsEIP1559 = true, want false (legacy fallback)")
	}

	// 20 Gwei * 1.2 = 24 Gwei = 24000000000
	gasPrice := new(big.Int)
	gasPrice.SetString(estimate.GasPrice, 10)
	expectedPrice := big.NewInt(24000000000)
	if gasPrice.Cmp(expectedPrice) != 0 {
		t.Errorf("GasPrice = %s, want %s", gasPrice.String(), expectedPrice.String())
	}
}

func TestInvalidStrategy(t *testing.T) {
	estimator := NewRPCGasEstimator()
	_, err := estimator.Estimate("http://localhost:8545", GasStrategy("invalid"))
	if err == nil {
		t.Fatal("Estimate() with invalid strategy expected error, got nil")
	}
}
