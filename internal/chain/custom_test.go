package chain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddCustomChain(t *testing.T) {
	r := NewRegistry()

	opts := CustomChainOpts{
		Name:                   "my-chain",
		ShortName:              "myc",
		RPCURL:                 "https://rpc.mychain.io",
		ChainID:                12345,
		NativeCurrencyName:     "MyCoin",
		NativeCurrencySymbol:   "MYC",
		NativeCurrencyDecimals: 18,
		ExplorerURL:            "https://explorer.mychain.io",
	}

	c, err := r.AddCustom(opts)
	if err != nil {
		t.Fatalf("AddCustom() returned error: %v", err)
	}
	if c.ID != 12345 {
		t.Errorf("chain ID = %d, want 12345", c.ID)
	}
	if c.Name != "my-chain" {
		t.Errorf("chain Name = %q, want my-chain", c.Name)
	}
	if c.NativeCurrency.Symbol != "MYC" {
		t.Errorf("NativeCurrency.Symbol = %q, want MYC", c.NativeCurrency.Symbol)
	}
	if len(c.RPCURLs) != 1 || c.RPCURLs[0] != "https://rpc.mychain.io" {
		t.Errorf("RPCURLs = %v, want [https://rpc.mychain.io]", c.RPCURLs)
	}
	if len(c.ExplorerURLs) != 1 || c.ExplorerURLs[0] != "https://explorer.mychain.io" {
		t.Errorf("ExplorerURLs = %v, want [https://explorer.mychain.io]", c.ExplorerURLs)
	}

	// Should be retrievable by name.
	got, err := r.Get("my-chain")
	if err != nil {
		t.Fatalf("Get(my-chain) returned error: %v", err)
	}
	if got.ID != 12345 {
		t.Errorf("Get(my-chain).ID = %d, want 12345", got.ID)
	}
}

func TestAddCustomChainValidation(t *testing.T) {
	tests := []struct {
		name string
		opts CustomChainOpts
	}{
		{
			name: "missing name",
			opts: CustomChainOpts{ChainID: 100, RPCURL: "https://rpc.example.com"},
		},
		{
			name: "missing chain ID",
			opts: CustomChainOpts{Name: "test", RPCURL: "https://rpc.example.com"},
		},
		{
			name: "zero chain ID",
			opts: CustomChainOpts{Name: "test", ChainID: 0, RPCURL: "https://rpc.example.com"},
		},
		{
			name: "missing RPC URL",
			opts: CustomChainOpts{Name: "test", ChainID: 100},
		},
		{
			name: "invalid RPC URL",
			opts: CustomChainOpts{Name: "test", ChainID: 100, RPCURL: "not-a-url"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			_, err := r.AddCustom(tt.opts)
			if err == nil {
				t.Fatalf("AddCustom() expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestAddCustomChainDuplicateID(t *testing.T) {
	r := NewRegistry()

	opts := CustomChainOpts{
		Name:    "fake-ethereum",
		ChainID: 1, // ethereum already exists
		RPCURL:  "https://rpc.example.com",
	}

	_, err := r.AddCustom(opts)
	if err == nil {
		t.Fatal("AddCustom() with duplicate chain ID expected error, got nil")
	}
}

func TestSaveAndLoadCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-chains.json")

	// Create a registry with a custom chain and save.
	r1 := NewRegistry()
	opts := CustomChainOpts{
		Name:                   "saved-chain",
		ShortName:              "sc",
		RPCURL:                 "https://rpc.saved.io",
		ChainID:                77777,
		NativeCurrencyName:     "SaveCoin",
		NativeCurrencySymbol:   "SAV",
		NativeCurrencyDecimals: 18,
		ExplorerURL:            "https://explorer.saved.io",
	}
	if _, err := r1.AddCustom(opts); err != nil {
		t.Fatalf("AddCustom() returned error: %v", err)
	}
	if err := r1.SaveCustom(path); err != nil {
		t.Fatalf("SaveCustom() returned error: %v", err)
	}

	// Load into a fresh registry.
	r2 := NewRegistry()
	if err := r2.LoadCustom(path); err != nil {
		t.Fatalf("LoadCustom() returned error: %v", err)
	}

	c, err := r2.Get("saved-chain")
	if err != nil {
		t.Fatalf("Get(saved-chain) after load returned error: %v", err)
	}
	if c.ID != 77777 {
		t.Errorf("loaded chain ID = %d, want 77777", c.ID)
	}
	if c.NativeCurrency.Symbol != "SAV" {
		t.Errorf("loaded NativeCurrency.Symbol = %q, want SAV", c.NativeCurrency.Symbol)
	}
	if len(c.RPCURLs) != 1 || c.RPCURLs[0] != "https://rpc.saved.io" {
		t.Errorf("loaded RPCURLs = %v, want [https://rpc.saved.io]", c.RPCURLs)
	}
	if len(c.ExplorerURLs) != 1 || c.ExplorerURLs[0] != "https://explorer.saved.io" {
		t.Errorf("loaded ExplorerURLs = %v, want [https://explorer.saved.io]", c.ExplorerURLs)
	}
}

func TestLoadCustomNonexistent(t *testing.T) {
	r := NewRegistry()
	err := r.LoadCustom("/nonexistent/path/custom-chains.json")
	if err != nil {
		t.Fatalf("LoadCustom() on nonexistent file should be no-op, got error: %v", err)
	}
}

func TestLoadCustomInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	r := NewRegistry()
	err := r.LoadCustom(path)
	if err == nil {
		t.Fatal("LoadCustom() with invalid JSON expected error, got nil")
	}
}
