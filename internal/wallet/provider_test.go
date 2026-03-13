package wallet

import (
	"strings"
	"testing"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	key, err := ks.Create("test-wallet")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ks.Use("test-wallet"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	_ = key
	return NewProvider(ks, 1)
}

func TestProviderGenerateJS(t *testing.T) {
	p := newTestProvider(t)

	js, err := p.GenerateJS()
	if err != nil {
		t.Fatalf("GenerateJS: %v", err)
	}

	checks := []string{
		"window.ethereum",
		"chainId",
		"_accounts",
		"isMetaMask",
	}
	for _, c := range checks {
		if !strings.Contains(js, c) {
			t.Errorf("GenerateJS output missing %q", c)
		}
	}
}

func TestProviderSetChainID(t *testing.T) {
	p := newTestProvider(t)

	p.SetChainID(137) // Polygon

	js, err := p.GenerateJS()
	if err != nil {
		t.Fatalf("GenerateJS: %v", err)
	}

	// Chain ID 137 = 0x89
	if !strings.Contains(js, "0x89") {
		t.Errorf("GenerateJS should contain chain ID 0x89 for Polygon, got:\n%s", js)
	}
}

func TestProviderSetAccounts(t *testing.T) {
	p := newTestProvider(t)

	addr := "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	p.SetAccounts([]string{addr})

	js, err := p.GenerateJS()
	if err != nil {
		t.Fatalf("GenerateJS: %v", err)
	}

	if !strings.Contains(js, addr) {
		t.Errorf("GenerateJS should contain account address %q", addr)
	}
}

func TestProviderInjectJS(t *testing.T) {
	p := newTestProvider(t)

	js := p.InjectJS()

	requiredMethods := []string{
		"eth_requestAccounts",
		"eth_accounts",
		"eth_chainId",
		"wallet_switchEthereumChain",
		"eth_sendTransaction",
		"personal_sign",
		"eth_signTypedData_v4",
	}
	for _, m := range requiredMethods {
		if !strings.Contains(js, m) {
			t.Errorf("InjectJS output missing method %q", m)
		}
	}

	// Check event support
	eventChecks := []string{
		"ethereum#initialized",
		"accountsChanged",
		"chainChanged",
		"connect",
		"disconnect",
	}
	for _, e := range eventChecks {
		if !strings.Contains(js, e) {
			t.Errorf("InjectJS output missing event %q", e)
		}
	}
}

func TestProviderEIP1193Methods(t *testing.T) {
	p := newTestProvider(t)

	js := p.InjectJS()

	// Verify core EIP-1193 structure
	checks := []string{
		"request",
		"on",
		"removeListener",
		"Promise.resolve",
		"Promise.reject",
		"selectedAddress",
	}
	for _, c := range checks {
		if !strings.Contains(js, c) {
			t.Errorf("InjectJS missing EIP-1193 element %q", c)
		}
	}
}

func TestProviderChainIDHexFormat(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	if _, err := ks.Create("w"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ks.Use("w"); err != nil {
		t.Fatalf("Use: %v", err)
	}

	tests := []struct {
		chainID uint64
		wantHex string
	}{
		{1, "0x1"},
		{10, "0xa"},
		{56, "0x38"},
		{137, "0x89"},
		{42161, "0xa4b1"},
	}

	for _, tc := range tests {
		p := NewProvider(ks, tc.chainID)
		js, err := p.GenerateJS()
		if err != nil {
			t.Fatalf("GenerateJS: %v", err)
		}
		if !strings.Contains(js, "'"+tc.wantHex+"'") {
			t.Errorf("chainID %d: expected hex %q in JS", tc.chainID, tc.wantHex)
		}
	}
}

func TestProviderNoActiveWallet(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	p := NewProvider(ks, 1)
	js, err := p.GenerateJS()
	if err != nil {
		t.Fatalf("GenerateJS should not error with no active wallet: %v", err)
	}

	// Should still produce valid JS with empty accounts
	if !strings.Contains(js, "window.ethereum") {
		t.Error("GenerateJS should still contain window.ethereum")
	}
}
