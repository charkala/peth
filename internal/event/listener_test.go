package event

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockRPCHandler returns a handler that responds with the given logs.
func mockRPCHandler(logs []logEntry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, _ := json.Marshal(logs)
		resp := rpcResponse{
			JSONRPC: "2.0",
			Result:  result,
			ID:      1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// mockRPCErrorHandler returns a handler that responds with an RPC error.
func mockRPCErrorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32000, Message: "test error"},
			ID:      1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// mockRPCEmptyHandler returns no logs.
func mockRPCEmptyHandler() http.HandlerFunc {
	return mockRPCHandler(nil)
}

func TestSubscribe(t *testing.T) {
	logs := []logEntry{
		{
			Address:     "0xdead",
			Topics:      []string{"0xtopic0", "0xtopic1"},
			Data:        "0xabcd",
			BlockNumber: "0xa",
			TxHash:      "0xtxhash1",
		},
	}

	srv := httptest.NewServer(mockRPCHandler(logs))
	defer srv.Close()

	listener := NewListener(srv.URL)
	listener.pollInterval = 10 * time.Millisecond

	ch, cancel, err := listener.Subscribe(EventFilter{
		ContractAddress: "0xdead",
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}
	defer cancel()

	select {
	case evt := <-ch:
		if evt.Address != "0xdead" {
			t.Errorf("Address = %q, want %q", evt.Address, "0xdead")
		}
		if evt.TxHash != "0xtxhash1" {
			t.Errorf("TxHash = %q, want %q", evt.TxHash, "0xtxhash1")
		}
		if evt.BlockNumber != 10 {
			t.Errorf("BlockNumber = %d, want 10", evt.BlockNumber)
		}
		if len(evt.Topics) != 2 {
			t.Errorf("len(Topics) = %d, want 2", len(evt.Topics))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSubscribeCancel(t *testing.T) {
	srv := httptest.NewServer(mockRPCEmptyHandler())
	defer srv.Close()

	listener := NewListener(srv.URL)
	listener.pollInterval = 10 * time.Millisecond

	ch, cancel, err := listener.Subscribe(EventFilter{})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Cancel immediately.
	cancel()

	// Channel should be closed soon.
	select {
	case _, ok := <-ch:
		if ok {
			// Got an event before close, that's ok - just drain.
			select {
			case <-ch:
			case <-time.After(500 * time.Millisecond):
			}
		}
		// Channel closed, test passes.
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close after cancel")
	}
}

func TestWaitForEvent(t *testing.T) {
	logs := []logEntry{
		{
			Address:     "0xbeef",
			Topics:      []string{"0xtopic"},
			Data:        "0x",
			BlockNumber: "0x5",
			TxHash:      "0xtx",
		},
	}

	srv := httptest.NewServer(mockRPCHandler(logs))
	defer srv.Close()

	listener := NewListener(srv.URL)
	listener.pollInterval = 10 * time.Millisecond

	evt, err := listener.WaitForEvent(EventFilter{ContractAddress: "0xbeef"}, 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForEvent() error: %v", err)
	}

	if evt.Address != "0xbeef" {
		t.Errorf("Address = %q, want %q", evt.Address, "0xbeef")
	}
	if evt.TxHash != "0xtx" {
		t.Errorf("TxHash = %q", evt.TxHash)
	}
}

func TestWaitForEventTimeout(t *testing.T) {
	srv := httptest.NewServer(mockRPCEmptyHandler())
	defer srv.Close()

	listener := NewListener(srv.URL)
	listener.pollInterval = 10 * time.Millisecond

	_, err := listener.WaitForEvent(EventFilter{}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %q, should contain 'timeout'", err.Error())
	}
}

func TestEventSignatureHash(t *testing.T) {
	tests := []struct {
		sig  string
		want string // just check it's deterministic and properly formatted
	}{
		{"Transfer(address,address,uint256)", ""},
		{"Approval(address,address,uint256)", ""},
	}

	for _, tt := range tests {
		hash := EventSignatureHash(tt.sig)

		// Should be 0x-prefixed.
		if !strings.HasPrefix(hash, "0x") {
			t.Errorf("EventSignatureHash(%q) = %q, missing 0x prefix", tt.sig, hash)
		}

		// Should be 66 chars: "0x" + 64 hex chars (32 bytes).
		if len(hash) != 66 {
			t.Errorf("EventSignatureHash(%q) length = %d, want 66", tt.sig, len(hash))
		}

		// Should be deterministic.
		hash2 := EventSignatureHash(tt.sig)
		if hash != hash2 {
			t.Errorf("EventSignatureHash not deterministic for %q", tt.sig)
		}
	}

	// Different signatures should produce different hashes.
	h1 := EventSignatureHash("Transfer(address,address,uint256)")
	h2 := EventSignatureHash("Approval(address,address,uint256)")
	if h1 == h2 {
		t.Error("different signatures should produce different hashes")
	}
}

func TestSubscribeWithEventSignature(t *testing.T) {
	var receivedFilter map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Params) > 0 {
			if f, ok := req.Params[0].(map[string]any); ok {
				receivedFilter = f
			}
		}
		result, _ := json.Marshal([]logEntry{})
		resp := rpcResponse{JSONRPC: "2.0", Result: result, ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	listener := NewListener(srv.URL)
	listener.pollInterval = 10 * time.Millisecond

	ch, cancel, err := listener.Subscribe(EventFilter{
		ContractAddress: "0xcontract",
		EventSignature:  "Transfer(address,address,uint256)",
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for at least one poll.
	time.Sleep(50 * time.Millisecond)
	cancel()
	// Drain channel.
	for range ch {
	}

	if receivedFilter == nil {
		t.Fatal("no RPC request received")
	}

	if addr, ok := receivedFilter["address"]; !ok || addr != "0xcontract" {
		t.Errorf("filter address = %v", receivedFilter["address"])
	}
}

func TestGetLogsRPCError(t *testing.T) {
	srv := httptest.NewServer(mockRPCErrorHandler())
	defer srv.Close()

	listener := NewListener(srv.URL)
	_, _, err := listener.getLogs(EventFilter{}, "latest")
	if err == nil {
		t.Fatal("expected error from RPC")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("error = %q, should contain 'test error'", err.Error())
	}
}
