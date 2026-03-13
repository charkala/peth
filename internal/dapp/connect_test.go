package dapp

import (
	"fmt"
	"strings"
	"testing"
)

// call records a single method invocation on the mock.
type call struct {
	Method string
	Args   []string
}

// mockBrowser is a recording mock that tracks call sequence.
type mockBrowser struct {
	calls      []call
	snapResult string
	snapErr    error
	evalResult string
	evalErr    error
	clickErr   error
	// evalResults allows returning different results for sequential Eval calls.
	evalResults []string
	evalIndex   int
}

func (m *mockBrowser) Nav(url string) error {
	m.calls = append(m.calls, call{Method: "Nav", Args: []string{url}})
	return nil
}

func (m *mockBrowser) Snap() (string, error) {
	m.calls = append(m.calls, call{Method: "Snap"})
	return m.snapResult, m.snapErr
}

func (m *mockBrowser) Click(ref string) error {
	m.calls = append(m.calls, call{Method: "Click", Args: []string{ref}})
	return m.clickErr
}

func (m *mockBrowser) Fill(ref, text string) error {
	m.calls = append(m.calls, call{Method: "Fill", Args: []string{ref, text}})
	return nil
}

func (m *mockBrowser) Eval(js string) (string, error) {
	m.calls = append(m.calls, call{Method: "Eval", Args: []string{js}})
	if m.evalErr != nil {
		return "", m.evalErr
	}
	if m.evalResults != nil && m.evalIndex < len(m.evalResults) {
		result := m.evalResults[m.evalIndex]
		m.evalIndex++
		return result, nil
	}
	return m.evalResult, nil
}

func TestConnectFlow(t *testing.T) {
	mock := &mockBrowser{
		snapResult: `<button ref="connectWallet">Connect Wallet</button>`,
		// First Eval: IsConnected check returns empty (not connected).
		// Second Eval: inject provider.
		// Third Eval: set accounts.
		evalResults: []string{"", "ok", "ok"},
	}

	conn := NewConnector(mock, "0xabc123")
	if err := conn.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Verify call sequence: Eval (IsConnected), Snap, Eval (inject), Eval (setAccounts), Click.
	if len(mock.calls) < 5 {
		t.Fatalf("expected at least 5 calls, got %d: %+v", len(mock.calls), mock.calls)
	}

	expected := []string{"Eval", "Snap", "Eval", "Eval", "Click"}
	for i, exp := range expected {
		if mock.calls[i].Method != exp {
			t.Errorf("call[%d]: expected %s, got %s", i, exp, mock.calls[i].Method)
		}
	}

	// Verify Click was called with the connect button ref.
	clickCall := mock.calls[4]
	if clickCall.Args[0] != "connectWallet" {
		t.Errorf("Click ref: expected connectWallet, got %s", clickCall.Args[0])
	}
}

func TestConnectAlreadyConnected(t *testing.T) {
	mock := &mockBrowser{
		// IsConnected returns an address, so Connect should be a no-op.
		evalResults: []string{"0xabc123"},
	}

	conn := NewConnector(mock, "0xabc123")
	if err := conn.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Only one call should have been made: the IsConnected Eval.
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call (IsConnected check only), got %d: %+v", len(mock.calls), mock.calls)
	}
	if mock.calls[0].Method != "Eval" {
		t.Errorf("expected Eval call, got %s", mock.calls[0].Method)
	}
}

func TestIsConnected(t *testing.T) {
	mock := &mockBrowser{evalResult: "0xabc123"}
	conn := NewConnector(mock, "0xabc123")

	connected, err := conn.IsConnected()
	if err != nil {
		t.Fatalf("IsConnected() error: %v", err)
	}
	if !connected {
		t.Error("expected IsConnected to return true")
	}
}

func TestIsConnectedNot(t *testing.T) {
	mock := &mockBrowser{evalResult: ""}
	conn := NewConnector(mock, "0xabc123")

	connected, err := conn.IsConnected()
	if err != nil {
		t.Fatalf("IsConnected() error: %v", err)
	}
	if connected {
		t.Error("expected IsConnected to return false")
	}
}

func TestDisconnect(t *testing.T) {
	mock := &mockBrowser{evalResult: "ok"}
	conn := NewConnector(mock, "0xabc123")

	if err := conn.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 Eval call, got %d", len(mock.calls))
	}
	if mock.calls[0].Method != "Eval" {
		t.Errorf("expected Eval, got %s", mock.calls[0].Method)
	}
	// Verify the eval clears state.
	if !strings.Contains(mock.calls[0].Args[0], "_setAccounts([])") {
		t.Error("Disconnect eval should call _setAccounts([])")
	}
}

func TestSignInWithEthereum(t *testing.T) {
	conn := NewConnector(&mockBrowser{}, "0x1234567890abcdef1234567890abcdef12345678")

	message := "example.com wants you to sign in with your Ethereum account"
	sig, err := conn.SignInWithEthereum(message)
	if err != nil {
		t.Fatalf("SignInWithEthereum() error: %v", err)
	}

	// Verify signature format: 0x-prefixed, 130 hex chars (65 bytes).
	if !strings.HasPrefix(sig, "0x") {
		t.Error("signature should be 0x-prefixed")
	}
	if len(sig) != 132 { // "0x" + 130 hex chars
		t.Errorf("expected signature length 132, got %d", len(sig))
	}

	// Verify deterministic: same message produces same signature.
	sig2, err := conn.SignInWithEthereum(message)
	if err != nil {
		t.Fatalf("second SignInWithEthereum() error: %v", err)
	}
	if sig != sig2 {
		t.Error("expected deterministic signature for same message")
	}
}

func TestSignInWithEthereumNoAddress(t *testing.T) {
	conn := NewConnector(&mockBrowser{}, "")

	_, err := conn.SignInWithEthereum("test message")
	if err == nil {
		t.Fatal("expected error for empty wallet address")
	}
}

func TestConnectNoButton(t *testing.T) {
	mock := &mockBrowser{
		snapResult:  `<div>No buttons here</div>`,
		evalResults: []string{""},
	}
	conn := NewConnector(mock, "0xabc123")

	err := conn.Connect()
	if err == nil {
		t.Fatal("expected error when no connect button found")
	}
	if !strings.Contains(err.Error(), "no connect-wallet button") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectSnapError(t *testing.T) {
	mock := &mockBrowser{
		snapErr:     fmt.Errorf("snap failed"),
		evalResults: []string{""},
	}
	conn := NewConnector(mock, "0xabc123")

	err := conn.Connect()
	if err == nil {
		t.Fatal("expected error on snap failure")
	}
}

func TestFindConnectButtonVariants(t *testing.T) {
	tests := []struct {
		snapshot string
		want     string
	}{
		{`<button ref="connectWallet">Connect</button>`, "connectWallet"},
		{`<button ref="connect-wallet">Connect</button>`, "connect-wallet"},
		{`<button ref="btnConnect">Connect</button>`, "btnConnect"},
		{`<button ref="connect_wallet">Connect</button>`, "connect_wallet"},
		{`<button ref="walletConnect">WalletConnect</button>`, "walletConnect"},
		{`<div>no button</div>`, ""},
	}

	conn := NewConnector(&mockBrowser{}, "0xabc")
	for _, tt := range tests {
		got := conn.findConnectButton(tt.snapshot)
		if got != tt.want {
			t.Errorf("findConnectButton(%q) = %q, want %q", tt.snapshot, got, tt.want)
		}
	}
}
