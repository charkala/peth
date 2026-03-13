package wallet

import (
	"strings"
	"testing"
)

func TestSolanaKeystoreCreate(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	key, err := ks.Create("sol-wallet")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if key.Name != "sol-wallet" {
		t.Errorf("Name = %q, want %q", key.Name, "sol-wallet")
	}

	// Solana addresses are base58-encoded, typically 32-44 chars
	if len(key.Address) < 32 || len(key.Address) > 44 {
		t.Errorf("Address length = %d, want 32-44", len(key.Address))
	}

	// Verify base58 characters only (no 0, O, I, l)
	for _, c := range key.Address {
		if strings.ContainsRune("0OIl", c) {
			t.Errorf("Address contains invalid base58 char: %c", c)
			break
		}
	}

	if len(key.PrivateKey) == 0 {
		t.Error("PrivateKey is empty")
	}
}

func TestSolanaKeystoreImport(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	// Generate a key first to get valid base58 private key
	created, err := ks.Create("temp")
	if err != nil {
		t.Fatalf("Create temp: %v", err)
	}

	// Export the private key as base58 and reimport
	privBase58 := Base58Encode(created.PrivateKey)

	dir2 := t.TempDir()
	ks2, err := NewSolanaKeystore(dir2)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	imported, err := ks2.Import("imported-sol", privBase58)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if imported.Name != "imported-sol" {
		t.Errorf("Name = %q, want %q", imported.Name, "imported-sol")
	}
	if imported.Address != created.Address {
		t.Errorf("Address = %q, want %q", imported.Address, created.Address)
	}
}

func TestSolanaKeystoreList(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	names := []string{"s1", "s2", "s3"}
	for _, n := range names {
		if _, err := ks.Create(n); err != nil {
			t.Fatalf("Create(%q): %v", n, err)
		}
	}

	keys, err := ks.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("List returned %d keys, want 3", len(keys))
	}

	found := map[string]bool{}
	for _, k := range keys {
		found[k.Name] = true
		if len(k.PrivateKey) != 0 {
			t.Errorf("List exposed PrivateKey for %q", k.Name)
		}
	}
	for _, n := range names {
		if !found[n] {
			t.Errorf("List missing wallet %q", n)
		}
	}
}

func TestSolanaKeystoreGet(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	created, err := ks.Create("get-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ks.Get("get-test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("Name = %q, want %q", got.Name, "get-test")
	}
	if got.Address != created.Address {
		t.Errorf("Address = %q, want %q", got.Address, created.Address)
	}
	if len(got.PrivateKey) == 0 {
		t.Error("PrivateKey should be populated on Get")
	}
}

func TestSolanaKeystoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewSolanaKeystore(dir)
	if err != nil {
		t.Fatalf("NewSolanaKeystore: %v", err)
	}

	_, err = ks.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent wallet")
	}
}
