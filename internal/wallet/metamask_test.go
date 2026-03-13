package wallet

import (
	"fmt"
	"testing"
)

// callRecord represents a single method call on the mock client.
type callRecord struct {
	Method string
	Args   []string
}

// mockPinchtabClient records all method calls in order for assertion.
type mockPinchtabClient struct {
	calls   []callRecord
	navErr  error
	clickErr error
	fillErr  error
	snapResult string
	snapErr    error
}

func (m *mockPinchtabClient) Nav(url string) error {
	m.calls = append(m.calls, callRecord{Method: "Nav", Args: []string{url}})
	return m.navErr
}

func (m *mockPinchtabClient) Click(ref string) error {
	m.calls = append(m.calls, callRecord{Method: "Click", Args: []string{ref}})
	return m.clickErr
}

func (m *mockPinchtabClient) Fill(ref, text string) error {
	m.calls = append(m.calls, callRecord{Method: "Fill", Args: []string{ref, text}})
	return m.fillErr
}

func (m *mockPinchtabClient) Snap() (string, error) {
	m.calls = append(m.calls, callRecord{Method: "Snap", Args: nil})
	return m.snapResult, m.snapErr
}

func TestMetaMaskSetup(t *testing.T) {
	mock := &mockPinchtabClient{}
	automator := NewMetaMaskAutomator(mock)

	seedPhrase := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	err := automator.Setup(seedPhrase)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one call to the client")
	}

	// First call should be Nav to the onboarding page
	if mock.calls[0].Method != "Nav" {
		t.Errorf("first call should be Nav, got %s", mock.calls[0].Method)
	}

	// Verify the sequence includes Nav, Click, Fill operations
	methodCounts := map[string]int{}
	for _, c := range mock.calls {
		methodCounts[c.Method]++
	}

	if methodCounts["Nav"] < 1 {
		t.Error("expected at least 1 Nav call")
	}
	if methodCounts["Click"] < 1 {
		t.Error("expected at least 1 Click call for setup steps")
	}
	if methodCounts["Fill"] < 1 {
		t.Error("expected at least 1 Fill call for seed phrase/password")
	}

	// Verify seed phrase was passed to a Fill call
	seedFilled := false
	for _, c := range mock.calls {
		if c.Method == "Fill" {
			for _, arg := range c.Args {
				if arg == seedPhrase {
					seedFilled = true
				}
			}
		}
	}
	if !seedFilled {
		t.Error("seed phrase should be passed to Fill")
	}
}

func TestMetaMaskSetupNavError(t *testing.T) {
	mock := &mockPinchtabClient{navErr: fmt.Errorf("connection refused")}
	automator := NewMetaMaskAutomator(mock)

	err := automator.Setup("test seed phrase with twelve words for the mnemonic here now ok done")
	if err == nil {
		t.Fatal("expected error when Nav fails")
	}
}

func TestMetaMaskApproveConnection(t *testing.T) {
	mock := &mockPinchtabClient{}
	automator := NewMetaMaskAutomator(mock)

	err := automator.ApproveConnection()
	if err != nil {
		t.Fatalf("ApproveConnection: %v", err)
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Should include Click calls for the approve button
	hasClick := false
	for _, c := range mock.calls {
		if c.Method == "Click" {
			hasClick = true
			break
		}
	}
	if !hasClick {
		t.Error("ApproveConnection should call Click")
	}
}

func TestMetaMaskApproveTransaction(t *testing.T) {
	mock := &mockPinchtabClient{}
	automator := NewMetaMaskAutomator(mock)

	err := automator.ApproveTransaction()
	if err != nil {
		t.Fatalf("ApproveTransaction: %v", err)
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Should include Click calls for the confirm button
	hasClick := false
	for _, c := range mock.calls {
		if c.Method == "Click" {
			hasClick = true
			break
		}
	}
	if !hasClick {
		t.Error("ApproveTransaction should call Click")
	}
}

func TestMetaMaskSwitchNetwork(t *testing.T) {
	mock := &mockPinchtabClient{}
	automator := NewMetaMaskAutomator(mock)

	err := automator.SwitchNetwork(137) // Polygon
	if err != nil {
		t.Fatalf("SwitchNetwork: %v", err)
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Should include Nav and Click calls
	methodCounts := map[string]int{}
	for _, c := range mock.calls {
		methodCounts[c.Method]++
	}

	if methodCounts["Click"] < 1 {
		t.Error("SwitchNetwork should call Click")
	}
}

func TestMetaMaskApproveConnectionError(t *testing.T) {
	mock := &mockPinchtabClient{clickErr: fmt.Errorf("element not found")}
	automator := NewMetaMaskAutomator(mock)

	err := automator.ApproveConnection()
	if err == nil {
		t.Fatal("expected error when Click fails")
	}
}

func TestMetaMaskApproveTransactionError(t *testing.T) {
	mock := &mockPinchtabClient{clickErr: fmt.Errorf("element not found")}
	automator := NewMetaMaskAutomator(mock)

	err := automator.ApproveTransaction()
	if err == nil {
		t.Fatal("expected error when Click fails")
	}
}

func TestMetaMaskSetupCallOrder(t *testing.T) {
	mock := &mockPinchtabClient{}
	automator := NewMetaMaskAutomator(mock)

	seed := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if err := automator.Setup(seed); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Verify Nav comes before any Click or Fill
	navIdx := -1
	for i, c := range mock.calls {
		if c.Method == "Nav" {
			navIdx = i
			break
		}
	}
	if navIdx == -1 {
		t.Fatal("no Nav call recorded")
	}

	for i, c := range mock.calls {
		if i < navIdx && (c.Method == "Click" || c.Method == "Fill") {
			t.Errorf("call %d (%s) came before Nav at index %d", i, c.Method, navIdx)
		}
	}
}
