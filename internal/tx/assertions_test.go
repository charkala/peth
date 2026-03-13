package tx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rpcHandler creates a test server that responds to JSON-RPC requests.
func rpcHandler(t *testing.T, method string, result interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		if req.Method != method {
			t.Errorf("rpc method = %q, want %q", req.Method, method)
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAssertBalanceGTE(t *testing.T) {
	tests := []struct {
		name     string
		balance  string // hex balance from RPC (in wei)
		operator string
		amount   string // in ETH
		wantPass bool
	}{
		{
			name:     "gte_pass_equal",
			balance:  "0xde0b6b3a7640000", // 1 ETH in wei
			operator: "gte",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "gte_pass_greater",
			balance:  "0x1bc16d674ec80000", // 2 ETH in wei
			operator: "gte",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "gte_fail_less",
			balance:  "0x6f05b59d3b20000", // 0.5 ETH in wei
			operator: "gte",
			amount:   "1.0",
			wantPass: false,
		},
		{
			name:     "eq_pass",
			balance:  "0xde0b6b3a7640000", // 1 ETH
			operator: "eq",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "eq_fail",
			balance:  "0x1bc16d674ec80000", // 2 ETH
			operator: "eq",
			amount:   "1.0",
			wantPass: false,
		},
		{
			name:     "lt_pass",
			balance:  "0x6f05b59d3b20000", // 0.5 ETH
			operator: "lt",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "lt_fail",
			balance:  "0xde0b6b3a7640000", // 1 ETH
			operator: "lt",
			amount:   "1.0",
			wantPass: false,
		},
		{
			name:     "gt_pass",
			balance:  "0x1bc16d674ec80000", // 2 ETH
			operator: "gt",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "gt_fail_equal",
			balance:  "0xde0b6b3a7640000", // 1 ETH
			operator: "gt",
			amount:   "1.0",
			wantPass: false,
		},
		{
			name:     "lte_pass_equal",
			balance:  "0xde0b6b3a7640000", // 1 ETH
			operator: "lte",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "lte_pass_less",
			balance:  "0x6f05b59d3b20000", // 0.5 ETH
			operator: "lte",
			amount:   "1.0",
			wantPass: true,
		},
		{
			name:     "lte_fail",
			balance:  "0x1bc16d674ec80000", // 2 ETH
			operator: "lte",
			amount:   "1.0",
			wantPass: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := rpcHandler(t, "eth_getBalance", tc.balance)

			result, err := AssertBalance(srv.URL, "0x1234567890abcdef1234567890abcdef12345678", tc.operator, tc.amount)
			if err != nil {
				t.Fatalf("AssertBalance: %v", err)
			}
			if result.Passed != tc.wantPass {
				t.Errorf("Passed = %v, want %v; Message: %s", result.Passed, tc.wantPass, result.Message)
			}
		})
	}
}

func TestAssertBalanceInvalidOperator(t *testing.T) {
	srv := rpcHandler(t, "eth_getBalance", "0xde0b6b3a7640000")

	_, err := AssertBalance(srv.URL, "0x1234567890abcdef1234567890abcdef12345678", "invalid", "1.0")
	if err == nil {
		t.Fatal("expected error for invalid operator")
	}
}

func TestAssertTxStatusSuccess(t *testing.T) {
	receipt := map[string]interface{}{
		"status": "0x1",
	}
	srv := rpcHandler(t, "eth_getTransactionReceipt", receipt)

	result, err := AssertTxStatus(srv.URL, "0xabcdef", "success")
	if err != nil {
		t.Fatalf("AssertTxStatus: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass for success status; Message: %s", result.Message)
	}
}

func TestAssertTxStatusFailed(t *testing.T) {
	receipt := map[string]interface{}{
		"status": "0x0",
	}
	srv := rpcHandler(t, "eth_getTransactionReceipt", receipt)

	result, err := AssertTxStatus(srv.URL, "0xabcdef", "failed")
	if err != nil {
		t.Fatalf("AssertTxStatus: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass for failed status; Message: %s", result.Message)
	}
}

func TestAssertTxStatusPending(t *testing.T) {
	// null result means no receipt yet (pending)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	result, err := AssertTxStatus(srv.URL, "0xabcdef", "success")
	if err != nil {
		t.Fatalf("AssertTxStatus: %v", err)
	}
	if result.Passed {
		t.Error("expected fail for pending transaction")
	}
	if result.Actual != "pending" {
		t.Errorf("Actual = %q, want %q", result.Actual, "pending")
	}
}

func TestAssertChainIDMatch(t *testing.T) {
	// eth_chainId returns hex chain ID
	srv := rpcHandler(t, "eth_chainId", "0x1")

	result, err := AssertChainID(srv.URL, 1)
	if err != nil {
		t.Fatalf("AssertChainID: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass for matching chain ID; Message: %s", result.Message)
	}
}

func TestAssertChainIDMismatch(t *testing.T) {
	srv := rpcHandler(t, "eth_chainId", "0x89") // 137 (polygon)

	result, err := AssertChainID(srv.URL, 1)
	if err != nil {
		t.Fatalf("AssertChainID: %v", err)
	}
	if result.Passed {
		t.Error("expected fail for mismatched chain ID")
	}
	if result.Expected != "1" {
		t.Errorf("Expected = %q, want %q", result.Expected, "1")
	}
}
