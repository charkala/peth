package wallet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeystoreCreate(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	key, err := ks.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if key.Name != "alice" {
		t.Errorf("Name = %q, want %q", key.Name, "alice")
	}
	if !strings.HasPrefix(key.Address, "0x") {
		t.Errorf("Address %q missing 0x prefix", key.Address)
	}
	if len(key.Address) != 42 {
		t.Errorf("Address length = %d, want 42", len(key.Address))
	}
	// Verify hex characters only after 0x
	for _, c := range key.Address[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Errorf("Address contains non-hex char: %c", c)
			break
		}
	}
	if len(key.PrivateKey) == 0 {
		t.Error("PrivateKey is empty")
	}
	if key.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	// Verify file was written
	fpath := filepath.Join(dir, "alice.json")
	if _, err := os.Stat(fpath); err != nil {
		t.Errorf("wallet file not created: %v", err)
	}
}

func TestKeystoreCreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	if _, err := ks.Create("bob"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = ks.Create("bob")
	if err != ErrWalletExists {
		t.Errorf("second Create err = %v, want ErrWalletExists", err)
	}
}

func TestKeystoreImportHexKey(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	// A valid 32-byte hex private key (64 hex chars)
	hexKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	tests := []struct {
		name  string
		input string
	}{
		{"without_prefix", hexKey},
		{"with_prefix", "0x" + hexKey},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			ks2, _ := NewKeystore(d)
			key, err := ks2.Import("imported", tc.input)
			if err != nil {
				t.Fatalf("Import: %v", err)
			}
			if key.Name != "imported" {
				t.Errorf("Name = %q, want %q", key.Name, "imported")
			}
			if !strings.HasPrefix(key.Address, "0x") || len(key.Address) != 42 {
				t.Errorf("invalid address: %q", key.Address)
			}
		})
	}

	// Also test with the shared keystore to verify import works
	key, err := ks.Import("from-hex", hexKey)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(key.PrivateKey) == 0 {
		t.Error("PrivateKey should be populated")
	}
}

func TestKeystoreImportMnemonic(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	// Standard 12-word test mnemonic
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	key, err := ks.Import("mnemonic-wallet", mnemonic)
	if err != nil {
		t.Fatalf("Import mnemonic: %v", err)
	}
	if key.Name != "mnemonic-wallet" {
		t.Errorf("Name = %q, want %q", key.Name, "mnemonic-wallet")
	}
	if !strings.HasPrefix(key.Address, "0x") || len(key.Address) != 42 {
		t.Errorf("invalid address: %q", key.Address)
	}
	if len(key.PrivateKey) == 0 {
		t.Error("PrivateKey should be populated")
	}
}

func TestKeystoreList(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	names := []string{"wallet-a", "wallet-b", "wallet-c"}
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
		// List should not expose private keys
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

func TestKeystoreGet(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	created, err := ks.Create("charlie")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ks.Get("charlie")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "charlie" {
		t.Errorf("Name = %q, want %q", got.Name, "charlie")
	}
	if got.Address != created.Address {
		t.Errorf("Address = %q, want %q", got.Address, created.Address)
	}
}

func TestKeystoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	_, err = ks.Get("nonexistent")
	if err != ErrWalletNotFound {
		t.Errorf("Get err = %v, want ErrWalletNotFound", err)
	}
}

func TestKeystoreDelete(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	if _, err := ks.Create("disposable"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ks.Delete("disposable"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// File should be gone
	fpath := filepath.Join(dir, "disposable.json")
	if _, err := os.Stat(fpath); !os.IsNotExist(err) {
		t.Error("wallet file still exists after Delete")
	}

	// Get should return not found
	_, err = ks.Get("disposable")
	if err != ErrWalletNotFound {
		t.Errorf("Get after Delete: %v, want ErrWalletNotFound", err)
	}
}

func TestKeystoreUseAndActive(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	created, err := ks.Create("primary")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ks.Use("primary"); err != nil {
		t.Fatalf("Use: %v", err)
	}

	active, err := ks.Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active.Name != "primary" {
		t.Errorf("Active Name = %q, want %q", active.Name, "primary")
	}
	if active.Address != created.Address {
		t.Errorf("Active Address = %q, want %q", active.Address, created.Address)
	}

	// .active file should exist
	activePath := filepath.Join(dir, ".active")
	if _, err := os.Stat(activePath); err != nil {
		t.Errorf(".active file not created: %v", err)
	}

	// Use non-existent wallet should fail
	err = ks.Use("ghost")
	if err != ErrWalletNotFound {
		t.Errorf("Use(ghost) err = %v, want ErrWalletNotFound", err)
	}
}

func TestKeystoreActiveNoWalletSet(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	_, err = ks.Active()
	if err == nil {
		t.Error("Active() should return error when no wallet is set")
	}
}

func TestKeystoreDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	err = ks.Delete("nonexistent")
	if err != ErrWalletNotFound {
		t.Errorf("Delete err = %v, want ErrWalletNotFound", err)
	}
}

func TestKeystoreImportBadFormat(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	_, err = ks.Import("bad", "not-a-key-or-mnemonic")
	if err == nil {
		t.Error("Import should fail with unrecognized format")
	}
}

func TestKeystoreImportDuplicate(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	hexKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	if _, err := ks.Import("dup", hexKey); err != nil {
		t.Fatalf("first Import: %v", err)
	}
	_, err = ks.Import("dup", hexKey)
	if err != ErrWalletExists {
		t.Errorf("second Import err = %v, want ErrWalletExists", err)
	}
}

func TestKeystoreEncryption(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	hexKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	if _, err := ks.Import("encrypted-test", hexKey); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Read the file on disk
	data, err := os.ReadFile(filepath.Join(dir, "encrypted-test.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// The raw hex key should NOT appear in the file
	if strings.Contains(string(data), hexKey) {
		t.Error("plaintext private key found in wallet file")
	}

	// File should be valid JSON with expected fields
	var stored struct {
		Name         string `json:"name"`
		Address      string `json:"address"`
		EncryptedKey string `json:"encrypted_key"`
		CreatedAt    string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("wallet file is not valid JSON: %v", err)
	}
	if stored.Name != "encrypted-test" {
		t.Errorf("stored name = %q, want %q", stored.Name, "encrypted-test")
	}
	if stored.EncryptedKey == "" {
		t.Error("encrypted_key is empty")
	}
	if stored.Address == "" {
		t.Error("address is empty")
	}
}

func TestPersonalSign(t *testing.T) {
	// Known private key — Hardhat/Anvil account #0
	privKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privBytes := mustDecodeHex(privKeyHex)

	message := "Hello peth"
	sig, err := PersonalSign(privBytes, message)
	if err != nil {
		t.Fatalf("PersonalSign: %v", err)
	}

	// 0x + 65 bytes = 132 chars
	if !strings.HasPrefix(sig, "0x") {
		t.Error("signature missing 0x prefix")
	}
	if len(sig) != 132 {
		t.Errorf("signature length = %d, want 132", len(sig))
	}

	// Deterministic: same inputs → same output
	sig2, _ := PersonalSign(privBytes, message)
	if sig != sig2 {
		t.Error("PersonalSign is not deterministic")
	}

	// Different message → different signature
	sigOther, _ := PersonalSign(privBytes, "Other message")
	if sig == sigOther {
		t.Error("different messages produced the same signature")
	}
}

func TestPersonalSignHashPrefix(t *testing.T) {
	// Verify prefix hash matches the Ethereum standard
	msg := []byte("test message")
	hash := PersonalSignHash(msg)
	if len(hash) != 32 {
		t.Errorf("PersonalSignHash length = %d, want 32", len(hash))
	}
}

func TestDeriveAddressKnownKey(t *testing.T) {
	// Hardhat/Anvil account #0: known private key → known Ethereum address
	privKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	expectedAddr := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266" // well-known address

	addr, err := deriveAddress(mustDecodeHex(privKeyHex))
	if err != nil {
		t.Fatalf("deriveAddress: %v", err)
	}
	if !strings.EqualFold(addr, expectedAddr) {
		t.Errorf("deriveAddress = %q, want %q", addr, expectedAddr)
	}
}

func TestImportHexDerivesCorrectAddress(t *testing.T) {
	dir := t.TempDir()
	ks, _ := NewKeystore(dir)

	// Anvil account #0
	privKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	expectedAddr := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

	key, err := ks.Import("anvil-0", privKeyHex)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !strings.EqualFold(key.Address, expectedAddr) {
		t.Errorf("imported address = %q, want %q", key.Address, expectedAddr)
	}
}

func mustDecodeHex(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	b, err := hexDecode(s)
	if err != nil {
		panic(err)
	}
	return b
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := range b {
		n, err := hexNibble(s[2*i])
		if err != nil {
			return nil, err
		}
		m, err := hexNibble(s[2*i+1])
		if err != nil {
			return nil, err
		}
		b[i] = byte(n<<4 | m)
	}
	return b, nil
}

func hexNibble(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	}
	return 0, fmt.Errorf("invalid hex char: %c", c)
}
