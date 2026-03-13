package tx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSimulateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "eth_call" {
			t.Errorf("method = %q, want eth_call", req.Method)
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x0000000000000000000000000000000000000000000000000000000000000001",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sim := NewRPCSimulator()
	tx := &Transaction{
		From:  "0x1234567890abcdef1234567890abcdef12345678",
		To:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value: "0",
	}

	result, err := sim.Simulate(server.URL, tx)
	if err != nil {
		t.Fatalf("Simulate() returned error: %v", err)
	}

	if !result.Success {
		t.Error("Simulate() Success = false, want true")
	}
	if len(result.ReturnData) != 32 {
		t.Errorf("ReturnData length = %d, want 32", len(result.ReturnData))
	}
}

func TestSimulateRevert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    3,
				"message": "execution reverted: Insufficient balance",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sim := NewRPCSimulator()
	tx := &Transaction{
		From:  "0x1234567890abcdef1234567890abcdef12345678",
		To:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value: "1000000000000000000",
	}

	result, err := sim.Simulate(server.URL, tx)
	if err != nil {
		t.Fatalf("Simulate() returned error: %v", err)
	}

	if result.Success {
		t.Error("Simulate() Success = true, want false")
	}
	if result.RevertReason != "Insufficient balance" {
		t.Errorf("RevertReason = %q, want %q", result.RevertReason, "Insufficient balance")
	}
}

func TestEstimateGas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "eth_estimateGas" {
			t.Errorf("method = %q, want eth_estimateGas", req.Method)
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x5208", // 21000
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sim := NewRPCSimulator()
	tx := &Transaction{
		From: "0x1234567890abcdef1234567890abcdef12345678",
		To:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	gas, err := sim.EstimateGas(server.URL, tx)
	if err != nil {
		t.Fatalf("EstimateGas() returned error: %v", err)
	}

	if gas != 21000 {
		t.Errorf("EstimateGas() = %d, want 21000", gas)
	}
}

func TestEstimateGasError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "gas required exceeds allowance",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sim := NewRPCSimulator()
	tx := &Transaction{
		From: "0x1234567890abcdef1234567890abcdef12345678",
		To:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	_, err := sim.EstimateGas(server.URL, tx)
	if err == nil {
		t.Fatal("EstimateGas() expected error, got nil")
	}
}

func TestSimulateCallObject(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sim := NewRPCSimulator()
	tx := &Transaction{
		From:     "0x1234567890abcdef1234567890abcdef12345678",
		To:       "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value:    "1000",
		Data:     []byte{0x01, 0x02},
		GasLimit: 21000,
	}

	sim.Simulate(server.URL, tx)

	params, ok := capturedBody["params"].([]interface{})
	if !ok || len(params) < 2 {
		t.Fatal("expected params with 2 elements")
	}

	callObj, ok := params[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected call object as first param")
	}

	if callObj["from"] != tx.From {
		t.Errorf("from = %q, want %q", callObj["from"], tx.From)
	}
	if callObj["to"] != tx.To {
		t.Errorf("to = %q, want %q", callObj["to"], tx.To)
	}
}
