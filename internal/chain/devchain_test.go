package chain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockProcess implements DevProcess for testing.
type mockProcess struct {
	running bool
	stopped bool
}

func (p *mockProcess) Stop() error {
	p.running = false
	p.stopped = true
	return nil
}

func (p *mockProcess) IsRunning() bool {
	return p.running
}

// mockCommander implements DevCommander for testing.
type mockCommander struct {
	lastCmd  string
	lastArgs []string
	proc     *mockProcess
}

func (c *mockCommander) Start(name string, args ...string) (DevProcess, error) {
	c.lastCmd = name
	c.lastArgs = args
	c.proc = &mockProcess{running: true}
	return c.proc, nil
}

func TestDevChainStartAnvil(t *testing.T) {
	cmd := &mockCommander{}
	dc := NewDevChain(DevChainOpts{
		Tool:      "anvil",
		Port:      8545,
		ChainID:   31337,
		Accounts:  10,
		BlockTime: 1,
	})
	dc.commander = cmd

	if err := dc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if cmd.lastCmd != "anvil" {
		t.Errorf("command = %q, want %q", cmd.lastCmd, "anvil")
	}

	// Verify args contain expected flags
	args := fmt.Sprintf("%v", cmd.lastArgs)
	if !containsArg(cmd.lastArgs, "--port", "8545") {
		t.Errorf("args missing --port 8545: %s", args)
	}
	if !containsArg(cmd.lastArgs, "--chain-id", "31337") {
		t.Errorf("args missing --chain-id 31337: %s", args)
	}
	if !containsArg(cmd.lastArgs, "--accounts", "10") {
		t.Errorf("args missing --accounts 10: %s", args)
	}
	if !containsArg(cmd.lastArgs, "--block-time", "1") {
		t.Errorf("args missing --block-time 1: %s", args)
	}
}

func TestDevChainStartHardhat(t *testing.T) {
	cmd := &mockCommander{}
	dc := NewDevChain(DevChainOpts{
		Tool:      "hardhat",
		Port:      8546,
		ChainID:   31337,
		Accounts:  5,
		BlockTime: 0,
	})
	dc.commander = cmd

	if err := dc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if cmd.lastCmd != "npx" {
		t.Errorf("command = %q, want %q", cmd.lastCmd, "npx")
	}
	if len(cmd.lastArgs) < 2 || cmd.lastArgs[0] != "hardhat" || cmd.lastArgs[1] != "node" {
		t.Errorf("args should start with [hardhat node], got %v", cmd.lastArgs)
	}
	if !containsArg(cmd.lastArgs, "--port", "8546") {
		t.Errorf("args missing --port 8546: %v", cmd.lastArgs)
	}
}

func TestDevChainStop(t *testing.T) {
	cmd := &mockCommander{}
	dc := NewDevChain(DevChainOpts{Tool: "anvil", Port: 8545})
	dc.commander = cmd

	if err := dc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !dc.IsRunning() {
		t.Error("expected IsRunning true after Start")
	}

	if err := dc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if dc.IsRunning() {
		t.Error("expected IsRunning false after Stop")
	}
	if !cmd.proc.stopped {
		t.Error("process was not stopped")
	}
}

func TestDevChainRPCURL(t *testing.T) {
	dc := NewDevChain(DevChainOpts{Tool: "anvil", Port: 8545})
	url := dc.RPCURL()
	if url != "http://localhost:8545" {
		t.Errorf("RPCURL() = %q, want %q", url, "http://localhost:8545")
	}

	dc2 := NewDevChain(DevChainOpts{Tool: "hardhat", Port: 9000})
	url2 := dc2.RPCURL()
	if url2 != "http://localhost:9000" {
		t.Errorf("RPCURL() = %q, want %q", url2, "http://localhost:9000")
	}
}

// rpcTestServer creates an httptest server that responds to JSON-RPC calls.
func rpcTestServer(t *testing.T, wantMethod string, result interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string        `json:"jsonrpc"`
			Method  string        `json:"method"`
			Params  []interface{} `json:"params"`
			ID      int           `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		if req.Method != wantMethod {
			t.Errorf("rpc method = %q, want %q", req.Method, wantMethod)
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

func TestDevChainSnapshot(t *testing.T) {
	srv := rpcTestServer(t, "evm_snapshot", "0x1")
	dc := NewDevChain(DevChainOpts{Tool: "anvil", Port: 8545})
	dc.rpcURLOverride = srv.URL

	id, err := dc.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if id != "0x1" {
		t.Errorf("snapshot ID = %q, want %q", id, "0x1")
	}
}

func TestDevChainRevert(t *testing.T) {
	srv := rpcTestServer(t, "evm_revert", true)
	dc := NewDevChain(DevChainOpts{Tool: "anvil", Port: 8545})
	dc.rpcURLOverride = srv.URL

	if err := dc.Revert("0x1"); err != nil {
		t.Fatalf("Revert: %v", err)
	}
}

func TestDevChainFundAccount(t *testing.T) {
	// Anvil uses anvil_setBalance
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string        `json:"jsonrpc"`
			Method  string        `json:"method"`
			Params  []interface{} `json:"params"`
			ID      int           `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "anvil_setBalance" {
			t.Errorf("method = %q, want %q", req.Method, "anvil_setBalance")
		}
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	dc := NewDevChain(DevChainOpts{Tool: "anvil", Port: 8545})
	dc.rpcURLOverride = srv.URL

	if err := dc.FundAccount("0x1234567890abcdef1234567890abcdef12345678", "100"); err != nil {
		t.Fatalf("FundAccount: %v", err)
	}
}

// containsArg checks if args contains flag followed by value.
func containsArg(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}
