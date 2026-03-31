package dapp

import (
	"fmt"
	"strings"
	"testing"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
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

// testPrivKey returns a deterministic secp256k1 private key for tests.
func testPrivKey() []byte {
	key, _ := secp256k1.GeneratePrivateKey()
	return key.Serialize()
}

func TestConnectFlow(t *testing.T) {
	mock := &mockBrowser{
		snapResult: `<button ref="connectWallet">Connect Wallet</button>`,
		// Eval sequence: IsConnected (empty), inject provider, signing bridge, set accounts.
		evalResults: []string{"", "ok", "ok", "ok"},
	}

	conn := NewConnector(mock, "0xabc123", testPrivKey())
	if err := conn.Connect(""); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Verify call sequence: Eval (IsConnected), Eval (inject), Eval (bridge), Eval (setAccounts), Snap, Click.
	if len(mock.calls) < 6 {
		t.Fatalf("expected at least 6 calls, got %d: %+v", len(mock.calls), mock.calls)
	}

	// First call must be IsConnected eval.
	if mock.calls[0].Method != "Eval" {
		t.Errorf("call[0]: expected Eval (IsConnected), got %s", mock.calls[0].Method)
	}

	// Last call must be Click.
	last := mock.calls[len(mock.calls)-1]
	if last.Method != "Click" {
		t.Errorf("last call: expected Click, got %s", last.Method)
	}
	if last.Args[0] != "connectWallet" {
		t.Errorf("Click ref: expected connectWallet, got %s", last.Args[0])
	}
}

func TestConnectWithExplicitRef(t *testing.T) {
	mock := &mockBrowser{
		// IsConnected returns empty (not connected), inject + bridge + setAccounts all succeed.
		evalResults: []string{"", "ok", "ok", "ok"},
	}

	conn := NewConnector(mock, "0xabc123", testPrivKey())
	if err := conn.Connect("e151"); err != nil {
		t.Fatalf("Connect(ref) error: %v", err)
	}

	// Snap should NOT be called when ref is provided.
	for _, c := range mock.calls {
		if c.Method == "Snap" {
			t.Error("Snap should not be called when buttonRef is provided")
		}
	}

	// Click should be called with the provided ref.
	var clicked string
	for _, c := range mock.calls {
		if c.Method == "Click" {
			clicked = c.Args[0]
		}
	}
	if clicked != "e151" {
		t.Errorf("expected Click(e151), got Click(%s)", clicked)
	}
}

func TestConnectAlreadyConnected(t *testing.T) {
	mock := &mockBrowser{
		// IsConnected returns an address, so Connect should be a no-op.
		evalResults: []string{"0xabc123"},
	}

	conn := NewConnector(mock, "0xabc123", testPrivKey())
	if err := conn.Connect(""); err != nil {
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
	conn := NewConnector(mock, "0xabc123", nil)

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
	conn := NewConnector(mock, "0xabc123", nil)

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
	conn := NewConnector(mock, "0xabc123", nil)

	if err := conn.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 Eval call, got %d", len(mock.calls))
	}
	if mock.calls[0].Method != "Eval" {
		t.Errorf("expected Eval, got %s", mock.calls[0].Method)
	}
	if !strings.Contains(mock.calls[0].Args[0], "_setAccounts([])") {
		t.Error("Disconnect eval should call _setAccounts([])")
	}
}

func TestSignInWithEthereum(t *testing.T) {
	privKey := testPrivKey()
	conn := NewConnector(&mockBrowser{}, "0x1234567890abcdef1234567890abcdef12345678", privKey)

	message := "example.com wants you to sign in with your Ethereum account"
	sig, err := conn.SignInWithEthereum(message)
	if err != nil {
		t.Fatalf("SignInWithEthereum() error: %v", err)
	}

	// Verify signature format: 0x-prefixed, 132 chars (65 bytes = 130 hex + "0x").
	if !strings.HasPrefix(sig, "0x") {
		t.Error("signature should be 0x-prefixed")
	}
	if len(sig) != 132 {
		t.Errorf("expected signature length 132, got %d: %s", len(sig), sig)
	}

	// Verify deterministic: same key + message produces same signature.
	sig2, err := conn.SignInWithEthereum(message)
	if err != nil {
		t.Fatalf("second SignInWithEthereum() error: %v", err)
	}
	if sig != sig2 {
		t.Error("expected deterministic signature for same message")
	}
}

func TestSignInWithEthereumNoKey(t *testing.T) {
	conn := NewConnector(&mockBrowser{}, "0xabc", nil)

	_, err := conn.SignInWithEthereum("test message")
	if err == nil {
		t.Fatal("expected error when no private key provided")
	}
	if !strings.Contains(err.Error(), "no private key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectNoButton(t *testing.T) {
	mock := &mockBrowser{
		snapResult:  `<div>No buttons here</div>`,
		evalResults: []string{"", "ok", "ok", "ok"},
	}
	conn := NewConnector(mock, "0xabc123", testPrivKey())

	err := conn.Connect("")
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
		evalResults: []string{"", "ok", "ok", "ok"},
	}
	conn := NewConnector(mock, "0xabc123", testPrivKey())

	err := conn.Connect("")
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

	conn := NewConnector(&mockBrowser{}, "0xabc", nil)
	for _, tt := range tests {
		got := conn.findConnectButton(tt.snapshot)
		if got != tt.want {
			t.Errorf("findConnectButton(%q) = %q, want %q", tt.snapshot, got, tt.want)
		}
	}
}
