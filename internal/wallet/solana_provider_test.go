package wallet

import (
	"strings"
	"testing"
)

func TestSolanaProviderGenerateJS(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}
	if _, err := ks.Create("test-sol"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sp := NewSolanaProvider(ks, "test-sol")
	js := sp.GenerateJS()

	if !strings.Contains(js, "window.solana") {
		t.Error("JS does not contain window.solana")
	}
}

func TestSolanaProviderIsPhantom(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}
	if _, err := ks.Create("phantom-test"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sp := NewSolanaProvider(ks, "phantom-test")
	js := sp.GenerateJS()

	if !strings.Contains(js, "isPhantom: true") {
		t.Error("JS does not contain isPhantom: true")
	}
}

func TestSolanaProviderMethods(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}
	if _, err := ks.Create("methods-test"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sp := NewSolanaProvider(ks, "methods-test")
	js := sp.GenerateJS()

	methods := []string{"connect", "disconnect", "signTransaction", "signMessage"}
	for _, m := range methods {
		if !strings.Contains(js, m) {
			t.Errorf("JS does not contain method %q", m)
		}
	}
}
